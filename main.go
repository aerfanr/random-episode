package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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

type dbLoadMsg struct {
	series []Series
}

func connectDB() tea.Msg {
	return dbLoadMsg{
		series: []Series{
			{
				name: "The Office",
			},
			{
				name: "Friends",
			},
		},
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
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
