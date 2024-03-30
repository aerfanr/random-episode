package main

import (
	"database/sql"
	"fmt"
	"log"
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
	Episode
)

type model struct {
	stage   Stage
	message string
	inputSeries Series
	choices []string
	cursor  int
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
	switch msg := msg.(type) {
	case dbLoadMsg:
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
					return m, tea.Quit
				}
			}

		case AddSeries:
			switch msg.String() {
			case "enter":
				handleSeriesInput(&m)
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
	}

	return s
}

func main() {
	defer closeDB()

	tea.LogToFile("/tmp/random-series.log", "")
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
