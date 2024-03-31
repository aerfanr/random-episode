package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func closeDB() {
	db.Close()
}

type Stage int
const (
	Menu Stage = iota
	AddSeries
	Result
)

type Episode struct {
	season int
	episode int
	number int
}

type model struct {
	stage   Stage
	message string
	inputSeries Series
	series []Series
	choices []string
	cursor  int
	selected int
	episode Episode
	textInput textinput.Model
}

type Series struct {
	name string
	episodeCount int
	seasonCount int
	seasonLengths []int
}

func newSeries() Series {
	return Series{}
}

func (s Series) save() error {
	if db == nil {
		return fmt.Errorf("No db connection")
	}

	sl := ""
	for _, l := range s.seasonLengths {
		sl += fmt.Sprintf("%d,", l)
	}

	stmt := `INSERT INTO series (name, season_lengths) VALUES (?, ?);`
	fmt.Println(stmt)
	_, err := db.Exec(stmt, s.name, sl)
	if err != nil {
		return err
	}

	return nil
}

func (s Series) getEpisode() (Episode, error) {
	for i := 0; i < 5; i++ {
		n := rand.Intn(s.episodeCount)

		stmt := `SELECT number FROM episodes WHERE series = ? AND number = ? AND watched = 1;`
		rows := db.QueryRow(stmt, s.name, n)
		if rows.Scan() == sql.ErrNoRows {
			m := n
			for j, l := range s.seasonLengths {
				if m < l {
					return Episode{
						season: j + 1,
						episode: m + 1,
						number: n,
					}, nil
				}
				m -= l
			}
		}
	}

	return Episode{}, fmt.Errorf("No episode found")
}

func (s Series) watchEpisode(e Episode) error {
	stmt := `INSERT INTO episodes (series, number, watched) VALUES (?, ?, 1);`
	_, err := db.Exec(stmt, s.name, e.number)
	return err
}

func seriesFromRow(rows *sql.Rows) (Series, error) {
	var name string
	var seasonLengths string
	err := rows.Scan(&name, &seasonLengths)
	if err != nil {
		return Series{}, err
	}

	s := newSeries()
	s.name = name
	for _, l := range strings.Split(seasonLengths, ",") {
		n, err := strconv.Atoi(l)
		if err == nil {
			s.seasonLengths = append(s.seasonLengths, n)
			s.episodeCount += n
		}
	}

	return s, nil
}

type dbLoadMsg struct {
	series []Series
}

func connectDB() tea.Msg {
	dbDir := os.Getenv("XDG_DATA_HOME")
	if dbDir == "" {
		dbDir = os.Getenv("HOME") + "/.local/share"
	}
	dbDir = dbDir + "/random-episode"

	os.MkdirAll(dbDir, 0755)

	// Connect sqlite db
	var err error
	db, err = sql.Open("sqlite3", dbDir + "/data.db")
	if err != nil {
		log.Fatal(err)
	}

	stmt := `CREATE TABLE IF NOT EXISTS series
	(name text, season_lengths text);`
	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `CREATE TABLE IF NOT EXISTS episodes
	(series text, number int, watched int);`
	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `SELECT name, season_lengths FROM series;`
	rows, err := db.Query(stmt)
	if err != nil {
		log.Fatal(err)
	}

	series := []Series{}
	for rows.Next() {
		s, err := seriesFromRow(rows)
		if err != nil {
			log.Print(err)
		}
		series = append(series, s)
	}

	return dbLoadMsg{
		series: series,
	}
}

func initialModel() model {
	ti := textinput.New()
	ti.Focus()

	return model{
		message: "What do you want to do?",
		stage:   Menu,
		choices: []string{"Add series"},
		cursor:  0,
		textInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	return connectDB
}

func handleSeriesInput(m *model) {
	var err error

	if m.textInput.Value() == "" {
		return
	}

	if m.inputSeries.name == "" {
		m.inputSeries.name = m.textInput.Value()
		m.message = fmt.Sprintf("Season count: ")
	} else if m.inputSeries.seasonCount == 0 {
		m.inputSeries.seasonCount, err = strconv.Atoi(m.textInput.Value())
		if err != nil {
			return
		}

		m.message = fmt.Sprintf("Season 1 length: ")
	} else if len(m.inputSeries.seasonLengths) < m.inputSeries.seasonCount {
		l, err := strconv.Atoi(m.textInput.Value())
		if err != nil {
			return
		}

		m.inputSeries.seasonLengths = append(m.inputSeries.seasonLengths, l)
		if len(m.inputSeries.seasonLengths) == m.inputSeries.seasonCount {
			err = m.inputSeries.save()
			if err != nil {
				log.Fatal(err)
				return
			}
			m.message = fmt.Sprintf("Series %s added!", m.inputSeries.name)
			m.stage = Menu
		} else {
			m.message = fmt.Sprintf("Season %d length: ", len(m.inputSeries.seasonLengths) + 1)
		}
	}

	m.textInput.Reset()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var err error

	switch msg := msg.(type) {
	case dbLoadMsg:
		m.series = msg.series
		for _, series := range msg.series {
			m.choices = append(m.choices, series.name)
		}
		if (len(m.choices) > 1) {
			m.cursor = 1
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.stage {
		case Menu:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "j", "down":
				m.cursor++
				if m.cursor >= len(m.choices) {
					m.cursor = 0
				}
			case "k", "up":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.choices) - 1
				}
			case "enter":
				if m.cursor == 0 {
					m.message = "Show name: "
					m.stage = AddSeries
				} else {
					m.selected = m.cursor - 1
					m.episode, err = m.series[m.selected].getEpisode()
					if err != nil {
						log.Println(err)
					}
					m.message = fmt.Sprintf(
						"Season %d, episode %d",
						m.episode.season,
						m.episode.episode,
					)
					m.stage = Result
					m.cursor = 0
				}
			}

		case AddSeries:
			switch msg.String() {
			case "enter":
				handleSeriesInput(&m)
			}

		case Result:
			switch msg.String() {
			case "j", "down":
				m.cursor = (m.cursor + 1) % 2
			case "k", "up":
				m.cursor = (m.cursor + 1) % 2
			case "q":
				return m, tea.Quit
			case "enter":
				if m.cursor == 0 {
					m.series[m.selected].watchEpisode(m.episode)
				}
				m.message = "What do you want to do?"
				m.stage = Menu
			}
		}

	}

	if m.stage == AddSeries {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	s := m.message + "\n\n"

	switch m.stage {
	case Menu:
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			s += fmt.Sprintf("%s %s\n", cursor, choice)
		}

		s += "\n(Press q to quit)"

	case AddSeries:
		s += m.textInput.View()

	case Result:
		for i, choice := range []string{"Watched", "Later"} {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			s += fmt.Sprintf("%s %s\n", cursor, choice)
		}
	}

	return s
}

func main() {
	defer closeDB()

	tea.LogToFile("/tmp/random-episode.log", "")
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
