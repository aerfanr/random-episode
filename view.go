package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type stageModel interface {
	Update(*globalModel, tea.Msg) tea.Cmd
	View() string
}

type globalModel struct {
	stageModel stageModel
	menu       *menuModel
	shows      []Show
}

type menuModel struct {
	choices []string
	cursor  int
}

func newMenu() menuModel {
	return menuModel{
		choices: []string{"Add show"},
		cursor:  0,
	}
}

func (m *menuModel) Update(g *globalModel, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return tea.Quit
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
				model := newAddModel()
				g.stageModel = &model
			} else {
				model := newResultModel(&g.shows[m.cursor-1])
				g.stageModel = &model
			}
		}
	}

	return nil
}

func (m *menuModel) View() string {
	s := "What do you want to do?\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\n(Press q to quit)"

	return s
}

type addModel struct {
	textInput textinput.Model
	message   string
	show      Show
}

func newAddModel() addModel {
	ti := textinput.New()
	ti.Focus()
	return addModel{
		textInput: ti,
		message:   "Show name: ",
	}
}

func (m *addModel) Update(g *globalModel, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd = nil
	m.textInput, cmd = m.textInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.handleShowInput(g)
		}
	}

	return cmd
}

func (m *addModel) handleShowInput(g *globalModel) {
	var err error

	if m.textInput.Value() == "" {
		return
	}

	if m.show.name == "" {
		m.show.name = m.textInput.Value()
		m.message = fmt.Sprintf("Season count: ")

	} else if m.show.seasonCount == 0 {
		m.show.seasonCount, err = strconv.Atoi(m.textInput.Value())
		if err != nil {
			return
		}
		m.message = fmt.Sprintf("Season 1 length: ")

	} else if len(m.show.seasonLengths) < m.show.seasonCount {
		l, err := strconv.Atoi(m.textInput.Value())
		if err != nil {
			return
		}

		m.show.seasonLengths = append(m.show.seasonLengths, l)
		if len(m.show.seasonLengths) == m.show.seasonCount {
			err = m.show.save()
			if err != nil {
				log.Fatal(err)
				return
			}
			g.stageModel = g.menu
		} else {
			m.message = fmt.Sprintf("Season %d length: ", len(m.show.seasonLengths)+1)
		}
	}

	m.textInput.Reset()
}

func (m *addModel) View() string {
	s := m.message + "\n\n"
	return s + m.textInput.View()
}

type resultModel struct {
	cursor  int
	show    *Show
	episode Episode
	error   error
}

func newResultModel(s *Show) resultModel {
	episode, err := s.getEpisode()
	return resultModel{
		cursor:  0,
		show:    s,
		episode: episode,
		error:   err,
	}
}

func (m *resultModel) Update(g *globalModel, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.cursor = (m.cursor + 1) % 2
		case "k", "up":
			m.cursor = (m.cursor + 1) % 2
		case "q":
			return tea.Quit
		case "enter":
			if m.cursor == 0 {
				m.show.watchEpisode(m.episode)
			}
			g.stageModel = g.menu
		}
	}

	return nil
}

func (m *resultModel) View() string {
	if m.error != nil {
		return "Error: \n\n" + m.error.Error()
	}

	s := fmt.Sprintf(
		"Season %d, episode %d\n\n",
		m.episode.season,
		m.episode.episode,
	)

	for i, choice := range []string{"Watched", "Later"} {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	return s
}

type dbLoadMsg struct {
	shows []Show
}

func initialModel() globalModel {
	menu := newMenu()

	return globalModel{
		stageModel: &menu,
		menu:       &menu,
	}
}

func (m globalModel) Init() tea.Cmd {
	return connectDB
}

func (m globalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd = nil

	switch msg := msg.(type) {
	case dbLoadMsg:
		m.shows = msg.shows
		for _, show := range msg.shows {
			m.menu.choices = append(m.menu.choices, show.name)
		}
		if len(m.menu.choices) > 1 {
			m.menu.cursor = 1
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	cmd = m.stageModel.Update(&m, msg)

	return m, cmd
}

func (m globalModel) View() string {
	return m.stageModel.View()
}
