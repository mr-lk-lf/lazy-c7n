package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

type runView int

const (
	runViewPolicies runView = iota
	runViewLog
)

type runsState struct {
	list      []runEntry
	loading   bool
	cursor    int // left: runs
	polCursor int // right: policies of the run
	view      runView
	log       logLoadedMsg
	scroll    int // log scroll
}

// runRows are indexes into runs.list after the "/" filter.
func (m Model) runRows() []int {
	filter := m.filters[ScreenRuns]
	var rows []int
	for i, r := range m.runs.list {
		text := r.Kind + " " + r.ID
		for _, p := range r.Policies {
			text += " " + p.Policy
		}
		if matchesFilter(text, filter) {
			rows = append(rows, i)
		}
	}
	return rows
}

func (m Model) currentRun() (runEntry, bool) {
	rows := m.runRows()
	if m.runs.cursor < 0 || m.runs.cursor >= len(rows) {
		return runEntry{}, false
	}
	return m.runs.list[rows[m.runs.cursor]], true
}

func (m Model) currentPolicyRun() (c7n.PolicyRun, bool) {
	r, ok := m.currentRun()
	if !ok || m.runs.polCursor >= len(r.Policies) {
		return c7n.PolicyRun{}, false
	}
	return r.Policies[m.runs.polCursor], true
}

// logPath is what the log view shows: the selected policy's
// custodian-run.log, or custodian's stderr when there is no policy output.
func (m Model) logPath() string {
	if pr, ok := m.currentPolicyRun(); ok {
		return pr.LogPath()
	}
	if r, ok := m.currentRun(); ok {
		return r.LogPath
	}
	return ""
}

func (m Model) updateRuns(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.runs
	run, hasRun := m.currentRun()
	page := paneRows(m.bodyHeight())

	switch {
	case key.Matches(msg, k.Reload):
		st.loading = true
		return m, m.reloadRuns()
	case key.Matches(msg, k.Copy):
		if hasRun && len(run.Argv) > 0 {
			m.setStatus("copied: " + strings.Join(run.Argv, " "))
			return m, tea.SetClipboard(strings.Join(run.Argv, " "))
		}
		m.setError("no command line recorded for this run")
		return m, nil
	case key.Matches(msg, k.Toggle):
		if st.view == runViewLog {
			st.view = runViewPolicies
			return m, nil
		}
		st.view, st.scroll = runViewLog, 0
		if p := m.logPath(); p != "" {
			return m, loadLog(p)
		}
		return m, nil
	}

	if m.focus == paneLeft {
		n := len(m.runRows())
		move := func(to int) {
			st.cursor = min(max(to, 0), max(n-1, 0))
			st.polCursor, st.scroll = 0, 0
			st.view = runViewPolicies
		}
		switch {
		case key.Matches(msg, k.Up):
			move(st.cursor - 1)
		case key.Matches(msg, k.Down):
			move(st.cursor + 1)
		case key.Matches(msg, k.PageUp):
			move(st.cursor - page)
		case key.Matches(msg, k.PageDown):
			move(st.cursor + page)
		case key.Matches(msg, k.Top):
			move(0)
		case key.Matches(msg, k.Bottom):
			move(n - 1)
		case key.Matches(msg, k.Enter):
			m.focus = paneRight
		}
		return m, nil
	}

	// Right pane.
	if st.view == runViewLog {
		w, _ := m.size()
		n := len(m.logLines(w - leftWidth(w) - 4))
		switch {
		case key.Matches(msg, k.Up):
			st.scroll = scrollBy(st.scroll, -1, n, page)
		case key.Matches(msg, k.Down):
			st.scroll = scrollBy(st.scroll, 1, n, page)
		case key.Matches(msg, k.PageUp):
			st.scroll = scrollBy(st.scroll, -page, n, page)
		case key.Matches(msg, k.PageDown):
			st.scroll = scrollBy(st.scroll, page, n, page)
		case key.Matches(msg, k.Top):
			st.scroll = 0
		case key.Matches(msg, k.Bottom):
			st.scroll = scrollBy(0, n, n, page)
		case key.Matches(msg, k.Back):
			st.view = runViewPolicies
		}
		return m, nil
	}
	n := len(run.Policies)
	switch {
	case key.Matches(msg, k.Up):
		st.polCursor = max(st.polCursor-1, 0)
	case key.Matches(msg, k.Down):
		st.polCursor = min(st.polCursor+1, max(n-1, 0))
	case key.Matches(msg, k.Top):
		st.polCursor = 0
	case key.Matches(msg, k.Bottom):
		st.polCursor = max(n-1, 0)
	case key.Matches(msg, k.Back):
		m.focus = paneLeft
	case key.Matches(msg, k.Enter):
		if pr, ok := m.currentPolicyRun(); ok {
			return m.openResources(run, pr)
		}
	}
	return m, nil
}

func (m Model) reloadRuns() tea.Cmd {
	return loadRuns(m.opts.OutputDirs)
}

func (m Model) viewRuns(width, height int) string {
	s := m.styles
	st := m.runs
	visible := paneRows(height)

	var list []string
	for _, i := range m.runRows() {
		list = append(list, m.runRowText(st.list[i]))
	}
	title := "Runs"
	switch {
	case st.loading:
		title += " (loading…)"
	case len(st.list) == 0:
		list = []string{s.Muted.Render("no runs yet"), "", s.Muted.Render("dry-run a policy with d, or open an"), s.Muted.Render("existing c7n output dir: lazyc7n -output <dir>")}
	case len(list) == 0:
		list = []string{s.Muted.Render("nothing matches the filter")}
	}
	list = m.listLines(list, st.cursor, visible, leftWidth(width), m.focus == paneLeft)

	run, ok := m.currentRun()
	if !ok {
		return m.twoPanes(title, list, "Run", nil, width, height)
	}
	if st.view == runViewLog {
		logTitle := "Log · " + shortPath(m.logPath()) + " · t back"
		logs := m.logLines(width - leftWidth(width) - 4)
		return m.twoPanes(title, list, logTitle, scrolled(logs, st.scroll, visible), width, height)
	}
	return m.twoPanes(title, list, "Run · "+runTitle(run), m.runDetail(run, visible, width-leftWidth(width)), width, height)
}

func runTitle(r runEntry) string {
	if r.Kind == "dir" {
		return shortPath(r.OutDir)
	}
	return r.ID
}

func (m Model) kindBadge(kind string) string {
	s := m.styles
	switch kind {
	case "dry-run":
		return s.OK.Render("DRY ")
	case "live":
		return s.Danger.Render("LIVE")
	case "validate":
		return s.Muted.Render("VAL ")
	}
	return s.Muted.Render("DIR ")
}

func (m Model) runRowText(r runEntry) string {
	s := m.styles
	when := "--------"
	if !r.Started.IsZero() {
		when = r.Started.Local().Format("01-02 15:04")
	}
	status := s.OK.Render("✓")
	switch {
	case !r.Finished:
		status = s.Warn.Render("…")
	case !r.ok():
		status = s.Danger.Render("✗")
	}
	summary := fmt.Sprintf("%d pol · %d res", len(r.Policies), r.matched())
	switch r.Kind {
	case "validate":
		summary = ""
	case "dir":
		summary += " " + s.Muted.Render(lastElems(r.OutDir, 2))
	}
	return fmt.Sprintf("%s %s %s %s", s.Muted.Render(when), m.kindBadge(r.Kind), status, summary)
}

// runDetail is the right pane of Runs: facts about the run and a table of
// its policies.
func (m Model) runDetail(r runEntry, visible, width int) []string {
	s := m.styles
	var out []string

	facts := []string{r.Kind}
	if !r.Started.IsZero() {
		facts = append(facts, "started "+r.Started.Local().Format("2006-01-02 15:04:05"))
	}
	if !r.Ended.IsZero() && !r.Started.IsZero() {
		facts = append(facts, "took "+r.Ended.Sub(r.Started).Round(10*time.Millisecond).String())
	}
	switch {
	case !r.Finished:
		facts = append(facts, s.Warn.Render("running"))
	case r.Kind != "dir":
		exit := fmt.Sprintf("exit %d", r.ExitCode)
		if r.ExitCode != 0 {
			exit = s.Danger.Render(exit)
		}
		facts = append(facts, exit)
	}
	out = append(out, s.Muted.Render(strings.Join(facts, " · ")))
	if len(r.Argv) > 0 {
		out = append(out, s.Item.Render("$ "+strings.Join(r.Argv, " ")))
	}
	if r.OutDir != "" {
		out = append(out, s.Muted.Render("output: "+shortPath(r.OutDir)))
	}
	if r.Err != "" {
		out = append(out, s.Danger.Render(r.Err))
	}
	if len(r.Policies) == 0 {
		if r.LogPath != "" {
			out = append(out, "", s.Muted.Render("no policy output; press t for custodian's log"))
		}
		return out
	}

	out = append(out, "", s.Bold.Render(fmt.Sprintf("%-28s %-10s %-8s %5s %7s", "POLICY", "REGION", "STATUS", "RES", "TIME")))
	var rows []string
	for _, p := range r.Policies {
		status := p.Status()
		statusText := fmt.Sprintf("%-8s", status)
		switch status {
		case "ok":
			statusText = s.OK.Render(statusText)
		case "deployed":
			statusText = s.Warn.Render(statusText)
		default:
			statusText = s.Danger.Render(statusText)
		}
		res := "-"
		if p.ResourceCount >= 0 {
			res = itoa(p.ResourceCount)
		}
		rows = append(rows, fmt.Sprintf("%-28s %-10s %s %5s %6.2fs", truncate(p.Policy, 28), truncate(p.Region, 10), statusText, res, p.Duration))
	}
	rows = m.listLines(rows, m.runs.polCursor, max(visible-len(out), 1), width, m.focus == paneRight)
	out = append(out, rows...)
	if m.focus == paneRight {
		out = append(out, "", s.Muted.Render("enter resources · t log · esc back"))
	}
	return out
}

// logLines are the log view's lines, wrapped to width.
func (m Model) logLines(width int) []string {
	s := m.styles
	st := m.runs
	if st.log.path != m.logPath() {
		return []string{s.Muted.Render("loading…")}
	}
	if st.log.err != nil {
		return []string{s.Danger.Render(st.log.err.Error())}
	}
	out := make([]string, 0, len(st.log.lines))
	for _, raw := range st.log.lines {
		for _, l := range strings.Split(ansi.Hardwrap(raw, max(width, 10), true), "\n") {
			out = append(out, m.logLine(l))
		}
	}
	return out
}

func (m Model) logLine(l string) string {
	s := m.styles
	switch {
	case strings.Contains(l, ":ERROR") || strings.Contains(l, " - ERROR - ") || strings.HasPrefix(l, "Traceback"):
		return s.Danger.Render(l)
	case strings.Contains(l, ":WARNING") || strings.Contains(l, " - WARNING - "):
		return s.Warn.Render(l)
	}
	return l
}

// lastElems is the last n elements of a path, for short labels.
func lastElems(p string, n int) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(p)), "/")
	if len(parts) > n {
		parts = parts[len(parts)-n:]
	}
	return strings.Join(parts, "/")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
