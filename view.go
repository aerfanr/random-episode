package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

var normalStyle lipgloss.Style
var focusStyle lipgloss.Style
var deleteStyle lipgloss.Style
var inputStyle lipgloss.Style

type stageModel interface {
	Update(*globalModel, tea.Msg) tea.Cmd
	View() string
}

type globalModel struct {
	stageModel stageModel
	menu       *menuModel
	shows      []Show
}

type showChoice struct {
	name string
	deleted bool
}

type menuModel struct {
	choices []showChoice
	cursor  int
}

func newMenu(s []Show) menuModel {
	choices := []showChoice{{name: "Add show"}}
	
	for _, show := range s {
		choices = append(choices, showChoice{name: show.name})
	}

	cursor := 1
	if len(s) == 0 {
		cursor = 0
	}

	return menuModel{
		choices: choices,
		cursor:  cursor,
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
		case "d":
			if m.cursor > 0 {
				err := g.shows[m.cursor-1].delete()
				if err != nil {
					log.Fatal(err)
				}
				m.choices[m.cursor].deleted = true
			}
		case "u":
			if m.cursor > 0 {
				err := g.shows[m.cursor-1].undelete()
				if err != nil {
					log.Fatal(err)
				}
				m.choices[m.cursor].deleted = false
			}
		}
	}

	return nil
}

func (m *menuModel) View() string {
	s := "What do you want to do?\n\n"
	for i, choice := range m.choices {
		style := normalStyle
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			style = focusStyle
		}

		if choice.deleted {
			s += fmt.Sprintf(
				deleteStyle.Render("%s (deleted: %s, press u to undo)"),
				cursor, choice.name,
			)
		} else {
			s += fmt.Sprintf(
				style.Render("%s %s"),
				cursor, choice.name,
			)
		}

		s += "\n"
	}

	s += "\n(Press q to quit, d to delete)"

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
			cmd = m.handleShowInput(g)
		}
	}

	return cmd
}

func (m *addModel) handleShowInput(g *globalModel) tea.Cmd {
	var err error

	if m.textInput.Value() == "" {
		return nil
	}

	if m.show.name == "" {
		m.show.name = m.textInput.Value()
		m.message = fmt.Sprintf("Season count: ")

	} else if m.show.seasonCount == 0 {
		m.show.seasonCount, err = strconv.Atoi(m.textInput.Value())
		if err != nil {
			return nil
		}
		m.message = fmt.Sprintf("Season 1 length: ")

	} else if len(m.show.seasonLengths) < m.show.seasonCount {
		l, err := strconv.Atoi(m.textInput.Value())
		if err != nil {
			return nil
		}

		m.show.seasonLengths = append(m.show.seasonLengths, l)
		if len(m.show.seasonLengths) == m.show.seasonCount {
			err = m.show.save()
			if err != nil {
				log.Fatal(err)
				return nil
			}
			g.stageModel = g.menu
			return readShows
		} else {
			m.message = fmt.Sprintf("Season %d length: ", len(m.show.seasonLengths)+1)
		}
	}

	m.textInput.Reset()

	return nil
}

func (m *addModel) View() string {
	s := m.message + "\n\n"
	return s + inputStyle.Render(m.textInput.View())
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
		style := normalStyle
		cursor := " "
		if m.cursor == i {
			style = focusStyle
			cursor = ">"
		}
		s += fmt.Sprintf(style.Render("%s %s"), cursor, choice)
		s += "\n"
	}

	return s
}

type showsLoaded struct {
	shows []Show
}

func initialModel() globalModel {
	normalStyle = lipgloss.NewStyle().Width(24).AlignHorizontal(lipgloss.Center)
	focusStyle = normalStyle.Copy().Bold(true).Foreground(lipgloss.Color("#CF3476"))
	deleteStyle = normalStyle.Copy().Italic(true)
	inputStyle = focusStyle.Copy().AlignHorizontal(lipgloss.Left)

	menu := newMenu([]Show{})

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
	case showsLoaded:
		m.shows = msg.shows
		menu := newMenu(m.shows)
		m.menu = &menu
		m.stageModel = m.menu

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
