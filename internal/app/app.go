// Package app is the Bubble Tea model: application state, Update and View
// (SPEC §7, "The Elm Architecture").
//
// Update never performs I/O itself. Anything that touches the outside world
// (running custodian, reading files) is returned as a tea.Cmd, so the state
// machine, and above all the live-run gate, can be tested by feeding
// messages into Update.
package app

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/config"
	"github.com/vstrofago/lazy-c7n/internal/runner"
	"github.com/vstrofago/lazy-c7n/internal/store"
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

// pane is which half of a screen has the keyboard.
type pane int

const (
	paneLeft pane = iota
	paneRight
)

// Options come from the command line.
type Options struct {
	// PolicyPaths replaces config policy_dirs when not empty.
	PolicyPaths []string
	// OutputDirs are existing c7n output dirs (-s) to show in Runs.
	OutputDirs []string
}

type Model struct {
	cfg    config.Config
	opts   Options
	screen Screen
	focus  pane
	width  int
	height int
	styles ui.Styles
	keys   keyMap
	help   help.Model

	// "/" filter of the list on the left, per screen.
	filtering bool
	filters   map[Screen]string

	store store.Store
	host  runner.Host

	version    string // custodian version, once known
	versionErr string

	policies policiesState
	runs     runsState
	res      resourcesState
	jobs     jobsState
	schema   schemaState

	quitArmed bool // q pressed once while jobs are running

	gate      liveGate
	status    string // one-line message in the footer
	statusErr bool
}

func New(cfg config.Config, opts Options) Model {
	m := Model{
		cfg:     cfg,
		opts:    opts,
		screen:  ScreenPolicies,
		keys:    defaultKeys(),
		help:    help.New(),
		filters: map[Screen]string{},
	}
	m.store = store.Store{Root: cfg.StatePath()}
	m.host = runner.CurrentHost()
	m.policies.collapsed = map[string]bool{}
	m.policies.marked = map[string]bool{}
	m.policies.loading = true
	m.runs.loading = true
	switch cfg.Theme {
	case config.ThemeLight:
		m.setTheme(false)
	case config.ThemeDark, config.ThemeAuto:
		m.setTheme(true) // auto: until the terminal tells us its background
	}
	return m
}

func (m *Model) setTheme(dark bool) {
	m.styles = ui.NewStyles(dark)
	m.help.Styles = help.DefaultStyles(dark)
}

// Screen is the active screen.
func (m Model) Screen() Screen { return m.screen }

func (m Model) policyPaths() []string {
	if len(m.opts.PolicyPaths) > 0 {
		return m.opts.PolicyPaths
	}
	return m.cfg.PolicyDirs
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		loadPolicies(m.policyPaths()),
		m.reloadRuns(),
		loadVersion(m.cfgRunnerArgv()),
	)
}

func (m *Model) setStatus(msg string) { m.status, m.statusErr = msg, false }
func (m *Model) setError(msg string)  { m.status, m.statusErr = msg, true }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if m.cfg.Theme == config.ThemeAuto {
			m.setTheme(msg.IsDark())
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
	case tea.PasteMsg:
		switch {
		case m.gate.open():
			m.gate = m.gate.paste(msg.Content)
		case m.filtering:
			m.filters[m.screen] += strings.ReplaceAll(msg.Content, "\n", "")
		}

	case policiesLoadedMsg:
		m.policies.loading = false
		m.policies.files = msg.files
		m.policies.cursor = min(m.policies.cursor, max(len(m.policyRows())-1, 0))
	case runsLoadedMsg:
		m.runs.loading = false
		m.runs.list = msg.runs
		m.runs.cursor = min(m.runs.cursor, max(len(m.runRows())-1, 0))
		if m.runs.selectID != "" {
			for i, idx := range m.runRows() {
				if m.runs.list[idx].ID == m.runs.selectID {
					m.runs.cursor, m.runs.polCursor = i, 0
				}
			}
			m.runs.selectID = ""
		}
		if msg.err != "" {
			m.setError(msg.err)
		}
	case resourcesLoadedMsg:
		if msg.dir == m.res.pr.Dir {
			m.res.loading = false
			m.res.list, m.res.err = msg.resources, msg.err
		}
	case logLoadedMsg:
		m.runs.log = msg

	case versionMsg:
		m.version, m.versionErr = msg.version, msg.err
		if msg.err != "" {
			m.setError(msg.err)
		}
	case jobStartedMsg, jobFailedMsg, jobOutputMsg, jobDoneMsg, jobSavedMsg:
		return m.updateJobMsg(msg)
	case schemaLoadedMsg:
		m.schema.loading = false
		m.schema.schema, m.schema.err = msg.schema, msg.err
	case schemaHelpMsg:
		if m.schema.help == nil {
			m.schema.help = map[string]string{}
		}
		if msg.text == "" {
			msg.text = "(no help text)"
		}
		m.schema.help[msg.path] = msg.text
	case editorDoneMsg:
		if msg.err != nil {
			m.setError("editor: " + msg.err.Error())
		}
		m.policies.loading = true
		return m, loadPolicies(m.policyPaths())

	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Mod == tea.ModCtrl && msg.Code == 'c' {
		return m, m.quit() // quitting never runs anything, so it is always allowed
	}
	if m.gate.open() {
		// While the gate is open every key goes to it: no screen changes,
		// no quitting with q, a second R is just a typed letter.
		req := m.gate.req
		var approved bool
		m.gate, approved = m.gate.update(msg)
		if approved {
			return m.startLiveRun(req)
		}
		return m, nil
	}
	if m.filtering {
		return m.updateFilter(msg), nil
	}

	if !key.Matches(msg, m.keys.Quit) {
		m.quitArmed = false
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		if n := m.runningJobs(); n > 0 && !m.quitArmed {
			m.quitArmed = true
			m.setError(itoa(n) + " job(s) still running: press q again to cancel and quit")
			return m, nil
		}
		return m, m.quit()
	case key.Matches(msg, m.keys.NextScreen):
		m.switchScreen(m.screen.offset(1))
		return m, m.ensureSchema()
	case key.Matches(msg, m.keys.PrevScreen):
		m.switchScreen(m.screen.offset(-1))
		return m, m.ensureSchema()
	case msg.Text >= "1" && msg.Text <= "5" && len(msg.Text) == 1:
		m.switchScreen(Screens[msg.Text[0]-'1'])
		return m, m.ensureSchema()
	case key.Matches(msg, m.keys.Left):
		m.focus = paneLeft
		return m, nil
	case key.Matches(msg, m.keys.Right):
		m.focus = paneRight
		return m, nil
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Filter) && m.focus == paneLeft:
		m.filtering = true
		return m, nil
	case key.Matches(msg, m.keys.Back) && m.filters[m.screen] != "":
		delete(m.filters, m.screen)
		return m, nil
	case key.Matches(msg, m.keys.LiveRun):
		if m.screen != ScreenPolicies {
			m.setError("live runs start from the Policies screen (1)")
			return m, nil
		}
		m.openLiveGate()
		return m, nil
	}

	switch m.screen {
	case ScreenPolicies:
		return m.updatePolicies(msg)
	case ScreenRuns:
		return m.updateRuns(msg)
	case ScreenResources:
		return m.updateResources(msg)
	case ScreenJobs:
		return m.updateJobs(msg)
	case ScreenSchema:
		return m.updateSchema(msg)
	}
	return m, nil
}

// quit cancels running jobs (custodian gets an interrupt) and exits.
func (m Model) quit() tea.Cmd {
	var cmds []tea.Cmd
	for _, j := range m.jobs.list {
		if j.running() && j.job != nil {
			cmds = append(cmds, cancelJob(j.job))
		}
	}
	if len(cmds) == 0 {
		return tea.Quit
	}
	return tea.Sequence(tea.Batch(cmds...), tea.Quit)
}

func (m *Model) switchScreen(s Screen) {
	m.screen = s
	m.focus = paneLeft
	m.status = ""
}

// updateFilter edits the "/" filter of the current screen.
func (m Model) updateFilter(msg tea.KeyPressMsg) Model {
	f := m.filters[m.screen]
	switch msg.Code {
	case tea.KeyEscape:
		delete(m.filters, m.screen)
		m.filtering = false
		return m
	case tea.KeyEnter:
		m.filtering = false
		return m
	case tea.KeyBackspace:
		if r := []rune(f); len(r) > 0 {
			f = string(r[:len(r)-1])
		}
	default:
		f += msg.Text
	}
	m.filters[m.screen] = f
	m.resetCursor()
	return m
}

func (m *Model) resetCursor() {
	switch m.screen {
	case ScreenPolicies:
		m.policies.cursor, m.policies.scroll = 0, 0
	case ScreenRuns:
		m.runs.cursor, m.runs.polCursor = 0, 0
	case ScreenResources:
		m.res.cursor, m.res.scroll = 0, 0
	case ScreenJobs:
		m.jobs.cursor = 0
	case ScreenSchema:
		m.schema.cursor, m.schema.item, m.schema.scroll = 0, 0, 0
	}
}

// matchesFilter is a simple fuzzy match: the filter's characters appear in
// s in order, ignoring case.
func matchesFilter(s, filter string) bool {
	if filter == "" {
		return true
	}
	s, filter = strings.ToLower(s), strings.ToLower(filter)
	i := 0
	for _, r := range s {
		if i < len(filter) && r == rune(filter[i]) {
			i++
		}
	}
	return i == len(filter)
}

// openLiveGate starts the confirmation for the current selection, after
// checking that custodian would run exactly the selected policies.
func (m *Model) openLiveGate() {
	chosen := m.selectedPolicies()
	if len(chosen) == 0 {
		m.setError("select a policy first")
		return
	}
	sel, err := c7n.Select(m.policies.files, chosen)
	if err != nil {
		m.setError(err.Error())
		return
	}
	spec := m.runSpec(sel, false)
	m.status = ""
	m.gate = openGate(LiveRunRequest{Policies: chosen, Spec: spec, Argv: m.previewArgv(spec)})
}

// startLiveRun is the only place a live run starts, and it is only called
// when the gate approves (see gate.go).
func (m Model) startLiveRun(req LiveRunRequest) (tea.Model, tea.Cmd) {
	spec := req.Spec
	spec.DryRun = false
	m, cmd := m.addJob("live", describeSelection(c7n.Selection{Files: spec.Files, Names: spec.Policies}), spec)
	m.jobs.list[len(m.jobs.list)-1].req = &req
	return m, cmd
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
