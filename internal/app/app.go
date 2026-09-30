// Package app is the Bubble Tea model: application state, Update and View
// (SPEC §7, "The Elm Architecture").
//
// Update never performs I/O itself. Anything that touches the outside world
// (running custodian, reading files) is returned as a tea.Cmd, so the state
// machine, and above all the live-run gate, can be tested by feeding
// messages into Update.
package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/config"
	"github.com/vstrofago/lazy-c7n/internal/ui"
)

type Screen int

const (
	ScreenPolicies Screen = iota
	ScreenRuns
	ScreenResources
	ScreenSchema
	ScreenJobs
)

// Screens is the tab order.
var Screens = []Screen{ScreenPolicies, ScreenRuns, ScreenResources, ScreenSchema, ScreenJobs}

func (s Screen) Title() string {
	switch s {
	case ScreenPolicies:
		return "Policies"
	case ScreenRuns:
		return "Runs"
	case ScreenResources:
		return "Resources"
	case ScreenSchema:
		return "Schema"
	case ScreenJobs:
		return "Jobs"
	}
	return "?"
}

func (s Screen) offset(delta int) Screen {
	n := len(Screens)
	return Screens[((int(s)+delta)%n+n)%n]
}

type keyMap struct {
	Next key.Binding
	Prev key.Binding
	Help key.Binding
	Quit key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Next: key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab/l", "next screen")),
		Prev: key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab/h", "prev screen")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding { return []key.Binding{k.Next, k.Help, k.Quit} }

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Next, k.Prev}, {k.Help, k.Quit}}
}

type Model struct {
	cfg    config.Config
	screen Screen
	width  int
	height int
	styles ui.Styles
	keys   keyMap
	help   help.Model
}

func New(cfg config.Config) Model {
	m := Model{cfg: cfg, screen: ScreenPolicies, keys: defaultKeys(), help: help.New()}
	m.setTheme(true) // until the terminal tells us its background
	return m
}

func (m *Model) setTheme(dark bool) {
	m.styles = ui.NewStyles(dark)
	m.help.Styles = help.DefaultStyles(dark)
}

// Screen is the active screen.
func (m Model) Screen() Screen { return m.screen }

func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.setTheme(msg.IsDark())
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Next):
			m.screen = m.screen.offset(1)
		case key.Matches(msg, m.keys.Prev):
			m.screen = m.screen.offset(-1)
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}
	return m, nil
}
