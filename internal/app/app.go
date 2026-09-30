// Package app is the Bubble Tea model: application state, Update and View
// (SPEC §7, "The Elm Architecture").
//
// Update never performs I/O itself. Anything that touches the outside world
// (running custodian, reading files) is returned as a tea.Cmd, so the state
// machine, and above all the live-run gate, can be tested by feeding
// messages into Update.
package app

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
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
	LiveRun key.Binding
	Next    key.Binding
	Prev    key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		LiveRun: key.NewBinding(key.WithKeys("R", "shift+r"), key.WithHelp("R", "run live")),
		Next:    key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab/l", "next screen")),
		Prev:    key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab/h", "prev screen")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more keys")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding { return []key.Binding{k.Next, k.Help, k.Quit} }

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Next, k.Prev}, {k.LiveRun}, {k.Help, k.Quit}}
}

type Model struct {
	cfg    config.Config
	screen Screen
	width  int
	height int
	styles ui.Styles
	keys   keyMap
	help   help.Model

	// selected are the policies R would run. Empty until the policy browser
	// (M1) fills it.
	selected []c7n.Policy
	gate     liveGate
	status   string // one-line message in the footer
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
	case tea.PasteMsg:
		if m.gate.open() {
			m.gate = m.gate.paste(msg.Content)
		}
	case liveRunStartedMsg:
		m.status = fmt.Sprintf("live run of %d %s confirmed (running arrives in M3)",
			len(msg.req.Policies), plural(len(msg.req.Policies), "policy", "policies"))
	case tea.KeyPressMsg:
		if msg.Mod == tea.ModCtrl && msg.Code == 'c' {
			return m, tea.Quit // quitting never runs anything, so it is always allowed
		}
		if m.gate.open() {
			// While the gate is open every key goes to it: no screen changes,
			// no quitting with q, a second R is just a typed letter.
			req := m.gate.req
			var approved bool
			m.gate, approved = m.gate.update(msg)
			if approved {
				return m, m.startLiveRun(req)
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.LiveRun):
			if len(m.selected) == 0 {
				m.status = "select a policy first"
				return m, nil
			}
			m.status = ""
			m.gate = openGate(LiveRunRequest{Policies: m.selected})
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

// liveRunStartedMsg reports that a confirmed live run was handed to the
// runner.
type liveRunStartedMsg struct{ req LiveRunRequest }

// startLiveRun is the only place a live run starts, and it is only called
// when the gate approves. Until the runner exists (M3) it just reports back.
func (m Model) startLiveRun(req LiveRunRequest) tea.Cmd {
	return func() tea.Msg { return liveRunStartedMsg{req: req} }
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
