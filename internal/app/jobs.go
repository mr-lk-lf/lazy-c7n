package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/runner"
	"github.com/vstrofago/lazy-c7n/internal/store"
)

// maxJobLines is how much output a job keeps in memory; the full output
// is always in the run's stdout.log / stderr.log.
const maxJobLines = 5000

type jobView struct {
	id       int
	kind     string // "dry-run", "live", "validate"
	title    string
	run      store.Run
	job      *runner.Job
	lines    []runner.Line
	started  bool
	done     bool
	canceled bool
	result   runner.Result
	err      string          // could not start
	matched  int             // resources matched, after a run
	req      *LiveRunRequest // what the gate confirmed, for live runs
}

func (j jobView) running() bool { return !j.done && j.err == "" }

// label is the job's kind and title, for status messages.
func (j jobView) label() string {
	switch j.kind {
	case "live":
		return "LIVE run of " + j.title
	case "dry-run":
		return "dry-run " + j.title
	}
	return j.kind + " " + j.title
}

type jobsState struct {
	list   []jobView
	nextID int
	cursor int
	scroll int  // right pane, from the top
	follow bool // right pane shows the newest lines
}

func (m Model) runningJobs() int {
	n := 0
	for _, j := range m.jobs.list {
		if j.running() {
			n++
		}
	}
	return n
}

func (m *Model) jobIndex(id int) int {
	for i, j := range m.jobs.list {
		if j.id == id {
			return i
		}
	}
	return -1
}

// Messages from job commands.
type (
	jobStartedMsg struct {
		id  int
		run store.Run
		job *runner.Job
	}
	jobFailedMsg struct {
		id  int
		run store.Run
		err string
	}
	jobOutputMsg struct {
		id    int
		lines []runner.Line
	}
	jobDoneMsg struct {
		id     int
		result runner.Result
	}
	jobSavedMsg struct {
		id      int
		run     store.Run
		matched int
		pruned  int
	}
	versionMsg struct {
		version string
		err     string
	}
)

// runSpec is the custodian run for the chosen policies. OutDir is filled
// in when the run directory exists.
func (m Model) runSpec(sel c7n.Selection, dryRun bool) runner.Spec {
	var regions []string
	if m.cfg.Defaults.Region != "" {
		regions = []string{m.cfg.Defaults.Region}
	}
	return runner.Spec{
		Subcommand:  "run",
		DryRun:      dryRun,
		Cache:       m.store.CachePath(),
		CachePeriod: m.cfg.Defaults.CachePeriod,
		Regions:     regions,
		Policies:    sel.Names,
		Files:       sel.Files,
	}
}

// previewArgv is the command line a run would use, with a placeholder for
// the run directory that does not exist yet.
func (m Model) previewArgv(spec runner.Spec) []string {
	spec.OutDir = m.store.Root + "/runs/<new-run>/out"
	argv, err := runner.Argv(m.cfg.Runner, spec, m.host)
	if err != nil {
		return nil
	}
	return argv
}

// startDryRun runs the selection with --dryrun. No confirmation: dry runs
// never change resources (SPEC §6.1).
func (m Model) startDryRun() (tea.Model, tea.Cmd) {
	chosen := m.selectedPolicies()
	sel, err := c7n.Select(m.policies.files, chosen)
	if err != nil {
		m.setError(err.Error())
		return m, nil
	}
	return m.addJob("dry-run", describePolicies(sel.Names), m.runSpec(sel, true))
}

// startValidate validates the files of the selection.
func (m Model) startValidate() (tea.Model, tea.Cmd) {
	chosen := m.selectedPolicies()
	var files []string
	seen := map[string]bool{}
	for _, p := range chosen {
		if !seen[p.File] {
			seen[p.File] = true
			files = append(files, p.File)
		}
	}
	if row, ok := m.cursorRow(); ok && len(files) == 0 {
		files = []string{m.policies.files[row.file].Path}
	}
	if len(files) == 0 {
		m.setError("select a policy or file first")
		return m, nil
	}
	return m.addJob("validate", strings.Join(shortPaths(files), " "), runner.Spec{Subcommand: "validate", Files: files})
}

func (m Model) addJob(kind, title string, spec runner.Spec) (Model, tea.Cmd) {
	m.jobs.nextID++
	id := m.jobs.nextID
	j := jobView{id: id, kind: kind, title: title}
	m.jobs.list = append(m.jobs.list, j)
	m.jobs.cursor = len(m.jobs.list) - 1
	m.jobs.follow, m.jobs.scroll = true, 0
	m.screen, m.focus = ScreenJobs, paneRight
	m.setStatus(j.label() + " started")
	return m, startJob(id, kind, spec, string(m.cfg.Runner.Kind), m.store, m.cfgRunnerArgv(), m.version)
}

// cfgRunnerArgv returns a function that builds argv with the current
// config, so the command does not capture the whole model.
func (m Model) cfgRunnerArgv() func(runner.Spec) ([]string, error) {
	cfg, host := m.cfg.Runner, m.host
	return func(s runner.Spec) ([]string, error) { return runner.Argv(cfg, s, host) }
}

func describePolicies(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%d policies", len(names))
}

func shortPaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = shortPath(p)
	}
	return out
}

// startJob creates the run directory, records run.json and starts
// custodian.
func startJob(id int, kind string, spec runner.Spec, backend string, st store.Store, argvFor func(runner.Spec) ([]string, error), version string) tea.Cmd {
	return func() tea.Msg {
		run, err := st.Create(kind, time.Now())
		if err != nil {
			return jobFailedMsg{id: id, err: "cannot create run dir: " + err.Error()}
		}
		if spec.Subcommand == "run" {
			spec.OutDir = run.OutDir()
		}
		run.Backend = backend
		run.Policies, run.Files = spec.Policies, spec.Files
		run.CustodianVersion = version
		argv, err := argvFor(spec)
		if err != nil {
			return failRun(st, id, run, err)
		}
		run.Argv = argv
		if err := st.Save(run); err != nil {
			return jobFailedMsg{id: id, run: run, err: "cannot write run.json: " + err.Error()}
		}
		job, err := runner.Start(argv, run.StdoutPath(), run.StderrPath())
		if err != nil {
			return failRun(st, id, run, err)
		}
		return jobStartedMsg{id: id, run: run, job: job}
	}
}

func failRun(st store.Store, id int, run store.Run, err error) tea.Msg {
	run.Finished, run.ExitCode, run.Error, run.Ended = true, -1, err.Error(), time.Now()
	_ = st.Save(run)
	return jobFailedMsg{id: id, run: run, err: err.Error()}
}

// waitJob delivers the next batch of output, or the end of the job.
func waitJob(id int, j *runner.Job) tea.Cmd {
	return func() tea.Msg {
		l, ok := <-j.Lines
		if !ok {
			return jobDoneMsg{id: id, result: <-j.Done}
		}
		batch := []runner.Line{l}
		for len(batch) < 500 {
			select {
			case l, ok := <-j.Lines:
				if !ok {
					return jobOutputMsg{id: id, lines: batch}
				}
				batch = append(batch, l)
			default:
				return jobOutputMsg{id: id, lines: batch}
			}
		}
		return jobOutputMsg{id: id, lines: batch}
	}
}

// finishJob records the end of the run and applies retention.
func finishJob(id int, run store.Run, res runner.Result, st store.Store, keep int) tea.Cmd {
	return func() tea.Msg {
		run.Finished, run.ExitCode, run.Ended = true, res.ExitCode, time.Now()
		if res.Err != nil {
			run.Error = res.Err.Error()
		}
		_ = st.Save(run)
		matched := 0
		if pols, err := c7n.ReadOutputDir(run.OutDir()); err == nil {
			for _, p := range pols {
				matched += max(p.ResourceCount, 0)
			}
		}
		removed, _ := st.Prune(keep, 0, time.Now())
		return jobSavedMsg{id: id, run: run, matched: matched, pruned: len(removed)}
	}
}

func cancelJob(j *runner.Job) tea.Cmd {
	return func() tea.Msg {
		j.Cancel()
		return nil
	}
}

// loadVersion asks custodian for its version (also shows whether the
// backend works at all). Docker may need to pull the image, hence the
// long timeout.
func loadVersion(argvFor func(runner.Spec) ([]string, error)) tea.Cmd {
	return func() tea.Msg {
		argv, err := argvFor(runner.Spec{Subcommand: "version"})
		if err != nil {
			return versionMsg{err: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		out, err := runner.Output(ctx, argv)
		if err != nil {
			return versionMsg{err: fmt.Sprintf("custodian not available (%s): %v", argv[0], err)}
		}
		return versionMsg{version: strings.TrimSpace(string(out))}
	}
}

// updateJobMsg handles messages coming back from job commands.
func (m Model) updateJobMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case jobStartedMsg:
		if i := m.jobIndex(msg.id); i >= 0 {
			m.jobs.list[i].run, m.jobs.list[i].job, m.jobs.list[i].started = msg.run, msg.job, true
		}
		return m, tea.Batch(waitJob(msg.id, msg.job), m.reloadRuns())
	case jobFailedMsg:
		if i := m.jobIndex(msg.id); i >= 0 {
			m.jobs.list[i].run, m.jobs.list[i].err = msg.run, msg.err
		}
		m.setError("could not start custodian: " + msg.err)
		return m, m.reloadRuns()
	case jobOutputMsg:
		i := m.jobIndex(msg.id)
		if i < 0 {
			return m, nil
		}
		j := &m.jobs.list[i]
		j.lines = append(j.lines, msg.lines...)
		if over := len(j.lines) - maxJobLines; over > 0 {
			j.lines = append([]runner.Line(nil), j.lines[over:]...)
		}
		return m, waitJob(j.id, j.job)
	case jobDoneMsg:
		i := m.jobIndex(msg.id)
		if i < 0 {
			return m, nil
		}
		j := &m.jobs.list[i]
		j.done, j.result = true, msg.result
		return m, finishJob(j.id, j.run, msg.result, m.store, m.cfg.KeepRuns)
	case jobSavedMsg:
		i := m.jobIndex(msg.id)
		if i < 0 {
			return m, nil
		}
		j := &m.jobs.list[i]
		j.run, j.matched = msg.run, msg.matched
		m.runs.selectID = msg.run.ID
		m.setJobStatus(*j)
		return m, m.reloadRuns()
	}
	return m, nil
}

func (m *Model) setJobStatus(j jobView) {
	code := j.result.ExitCode
	switch {
	case j.canceled:
		m.setError(j.label() + " canceled")
	case j.kind == "validate" && code == 0:
		m.setStatus(j.label() + ": valid")
	case j.kind == "validate":
		m.setError(j.label() + ": INVALID (see the output)")
	case code != 0:
		m.setError(fmt.Sprintf("%s failed (exit %d) · %d resources matched · 2 Runs for details", j.label(), code, j.matched))
	default:
		m.setStatus(fmt.Sprintf("%s finished · %d resources matched · 2 Runs for details", j.label(), j.matched))
	}
}

func (m Model) updateJobs(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.jobs
	if key.Matches(msg, k.Cancel) {
		if st.cursor < len(st.list) {
			j := &st.list[st.cursor]
			if j.running() && j.job != nil {
				j.canceled = true
				m.setStatus("canceling " + j.label() + "…")
				return m, cancelJob(j.job)
			}
		}
		m.setError("no running job selected")
		return m, nil
	}

	if m.focus == paneLeft {
		move := func(to int) {
			st.cursor = min(max(to, 0), max(len(st.list)-1, 0))
			st.follow, st.scroll = true, 0
		}
		switch {
		case key.Matches(msg, k.Up):
			move(st.cursor - 1)
		case key.Matches(msg, k.Down):
			move(st.cursor + 1)
		case key.Matches(msg, k.Top):
			move(0)
		case key.Matches(msg, k.Bottom):
			move(len(st.list) - 1)
		case key.Matches(msg, k.Enter):
			m.focus = paneRight
		}
		return m, nil
	}

	lines := m.jobOutput()
	page := paneRows(m.bodyHeight())
	n := len(lines)
	if st.follow {
		st.scroll = max(n-page, 0)
	}
	switch {
	case key.Matches(msg, k.Up):
		st.follow, st.scroll = false, scrollBy(st.scroll, -1, n, page)
	case key.Matches(msg, k.Down):
		st.scroll = scrollBy(st.scroll, 1, n, page)
	case key.Matches(msg, k.PageUp):
		st.follow, st.scroll = false, scrollBy(st.scroll, -page, n, page)
	case key.Matches(msg, k.PageDown):
		st.scroll = scrollBy(st.scroll, page, n, page)
	case key.Matches(msg, k.Top):
		st.follow, st.scroll = false, 0
	case key.Matches(msg, k.Bottom):
		st.follow = true
	case key.Matches(msg, k.Back):
		m.focus = paneLeft
	}
	if st.scroll >= max(n-page, 0) {
		st.follow = true
	}
	return m, nil
}

func (m Model) currentJob() (jobView, bool) {
	if m.jobs.cursor < 0 || m.jobs.cursor >= len(m.jobs.list) {
		return jobView{}, false
	}
	return m.jobs.list[m.jobs.cursor], true
}

// jobOutput is the right pane of Jobs: the command and its output.
func (m Model) jobOutput() []string {
	s := m.styles
	j, ok := m.currentJob()
	if !ok {
		return nil
	}
	var out []string
	if len(j.run.Argv) > 0 {
		out = append(out, wrapped(s.Muted, "$ "+strings.Join(store.RedactArgv(j.run.Argv), " "), m.rightInner())...)
	}
	for _, l := range j.lines {
		for _, w := range strings.Split(ansi.Hardwrap(l.Text, m.rightInner(), true), "\n") {
			if l.Stderr {
				out = append(out, m.logLine(w))
			} else {
				out = append(out, s.Item.Render(w))
			}
		}
	}
	switch {
	case j.err != "":
		out = append(out, s.Danger.Render("could not start: "+j.err))
	case !j.started:
		out = append(out, s.Muted.Render("starting…"))
	case j.done:
		end := fmt.Sprintf("── exit %d", j.result.ExitCode)
		if j.canceled {
			end += " (canceled)"
		}
		if j.result.ExitCode == 0 {
			out = append(out, s.OK.Render(end))
		} else {
			out = append(out, s.Danger.Render(end))
		}
	}
	return out
}

func (m Model) viewJobs(width, height int) string {
	s := m.styles
	st := m.jobs
	visible := paneRows(height)

	var list []string
	for _, j := range st.list {
		var state string
		switch {
		case j.err != "":
			state = s.Danger.Render("✗ failed")
		case j.running():
			state = s.Warn.Render("… running")
		case j.canceled:
			state = s.Danger.Render("✗ canceled")
		case j.result.ExitCode == 0:
			state = s.OK.Render("✓")
		default:
			state = s.Danger.Render(fmt.Sprintf("✗ exit %d", j.result.ExitCode))
		}
		list = append(list, fmt.Sprintf("%s %s %s", m.kindBadge(j.kind), j.title, state))
	}
	if len(list) == 0 {
		list = []string{s.Muted.Render("no jobs yet"), "", s.Muted.Render("v validate · d dry-run · R live run"), s.Muted.Render("(from the Policies screen)")}
	}
	list = m.listLines(list, st.cursor, visible, leftWidth(width), m.focus == paneLeft)

	out := m.jobOutput()
	scroll := st.scroll
	if st.follow {
		scroll = len(out)
	}
	title := "Output"
	if j, ok := m.currentJob(); ok {
		title = j.title
		if j.running() {
			title += " · x cancel"
		}
	}
	return m.twoPanes("Jobs", list, title, scrolled(out, scroll, visible), width, height)
}
