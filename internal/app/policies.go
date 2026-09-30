package app

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

type policiesState struct {
	files     []c7n.PolicyFile
	loading   bool
	collapsed map[string]bool // file path -> folded
	marked    map[string]bool // policy key (file:line) -> selected
	cursor    int
	scroll    int // right pane
}

// polRow is one row of the policy tree: a file (policy == -1) or a policy.
type polRow struct{ file, policy int }

func policyKey(p c7n.Policy) string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// policyRows is the visible tree, after folding and the "/" filter.
func (m Model) policyRows() []polRow {
	filter := m.filters[ScreenPolicies]
	var rows []polRow
	for fi, f := range m.policies.files {
		var matches []int
		for pi, p := range f.Policies {
			if matchesFilter(p.Name+" "+p.Resource, filter) {
				matches = append(matches, pi)
			}
		}
		if filter != "" && len(matches) == 0 && !matchesFilter(f.Path, filter) {
			continue
		}
		rows = append(rows, polRow{fi, -1})
		if m.policies.collapsed[f.Path] && filter == "" {
			continue
		}
		for _, pi := range matches {
			rows = append(rows, polRow{fi, pi})
		}
	}
	return rows
}

func (m Model) cursorRow() (polRow, bool) {
	rows := m.policyRows()
	if m.policies.cursor < 0 || m.policies.cursor >= len(rows) {
		return polRow{}, false
	}
	return rows[m.policies.cursor], true
}

// selectedPolicies is what d / R act on: the marked policies, or else the
// policy under the cursor, or else every policy of the file under it.
func (m Model) selectedPolicies() []c7n.Policy {
	var out []c7n.Policy
	for _, f := range m.policies.files {
		for _, p := range f.Policies {
			if m.policies.marked[policyKey(p)] {
				out = append(out, p)
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	row, ok := m.cursorRow()
	if !ok {
		return nil
	}
	f := m.policies.files[row.file]
	if row.policy >= 0 {
		return []c7n.Policy{f.Policies[row.policy]}
	}
	return append(out, f.Policies...)
}

func (m Model) updatePolicies(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.policies
	rows := m.policyRows()

	// Actions work from either pane.
	switch {
	case key.Matches(msg, k.DryRun):
		return m.startDryRun()
	case key.Matches(msg, k.Validate):
		return m.startValidate()
	case key.Matches(msg, k.Copy):
		sel, err := c7n.Select(st.files, m.selectedPolicies())
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		cmdline := strings.Join(m.previewArgv(m.runSpec(sel, true)), " ")
		m.setStatus("copied: " + cmdline)
		return m, tea.SetClipboard(cmdline)
	case key.Matches(msg, k.Reload):
		st.loading = true
		return m, loadPolicies(m.policyPaths())
	case key.Matches(msg, k.Edit):
		row, ok := m.cursorRow()
		if !ok {
			return m, nil
		}
		f := st.files[row.file]
		line := 1
		if row.policy >= 0 {
			line = f.Policies[row.policy].Line
		}
		cmd, err := editorCommand(os.Getenv("VISUAL"), os.Getenv("EDITOR"), f.Path, line)
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{err} })
	}

	if m.focus == paneRight {
		n := len(m.policyDetail())
		page := paneRows(m.bodyHeight())
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
			m.focus = paneLeft
		}
		return m, nil
	}

	move := func(to int) {
		st.cursor = min(max(to, 0), max(len(rows)-1, 0))
		st.scroll = 0
	}
	switch {
	case key.Matches(msg, k.Up):
		move(st.cursor - 1)
	case key.Matches(msg, k.Down):
		move(st.cursor + 1)
	case key.Matches(msg, k.PageUp):
		move(st.cursor - paneRows(m.bodyHeight()))
	case key.Matches(msg, k.PageDown):
		move(st.cursor + paneRows(m.bodyHeight()))
	case key.Matches(msg, k.Top):
		move(0)
	case key.Matches(msg, k.Bottom):
		move(len(rows) - 1)
	case key.Matches(msg, k.Enter):
		if row, ok := m.cursorRow(); ok {
			if row.policy < 0 {
				path := st.files[row.file].Path
				st.collapsed[path] = !st.collapsed[path]
			} else {
				m.focus = paneRight
			}
		}
	case key.Matches(msg, k.Mark):
		m.toggleMark()
		move(st.cursor + 1)
	case key.Matches(msg, k.Back):
		clear(st.marked)
	}
	return m, nil
}

// toggleMark selects/unselects the policy under the cursor, or all
// policies of the file under it.
func (m *Model) toggleMark() {
	row, ok := m.cursorRow()
	if !ok {
		return
	}
	f := m.policies.files[row.file]
	if row.policy >= 0 {
		k := policyKey(f.Policies[row.policy])
		m.policies.marked[k] = !m.policies.marked[k]
		return
	}
	all := true
	for _, p := range f.Policies {
		all = all && m.policies.marked[policyKey(p)]
	}
	for _, p := range f.Policies {
		m.policies.marked[policyKey(p)] = !all
	}
}

func (m Model) markedCount() int {
	n := 0
	for _, v := range m.policies.marked {
		if v {
			n++
		}
	}
	return n
}

// bodyHeight approximates the body height for paging keys.
func (m Model) bodyHeight() int {
	_, h := m.size()
	return max(h-3, 3)
}

func (m Model) viewPolicies(width, height int) string {
	s := m.styles
	st := m.policies
	rows := m.policyRows()

	nameWidth := 0
	for _, r := range rows {
		if r.policy >= 0 {
			nameWidth = max(nameWidth, len([]rune(st.files[r.file].Policies[r.policy].Name)))
		}
	}
	nameWidth = min(nameWidth, 32)

	var list []string
	for _, r := range rows {
		f := st.files[r.file]
		if r.policy < 0 {
			fold := "▾ "
			if st.collapsed[f.Path] && m.filters[ScreenPolicies] == "" {
				fold = "▸ "
			}
			line := s.Bold.Render(fold + f.Rel)
			switch {
			case f.Err != nil:
				line = s.Danger.Render("✗ "+f.Rel) + s.Muted.Render("  "+f.Err.Error())
			case st.collapsed[f.Path]:
				line += s.Muted.Render(fmt.Sprintf("  (%d)", len(f.Policies)))
			}
			list = append(list, line)
			continue
		}
		p := f.Policies[r.policy]
		mark := "  "
		if st.marked[policyKey(p)] {
			mark = s.Marked.Render("● ")
		}
		name := fmt.Sprintf("%-*s", nameWidth, truncate(p.Name, nameWidth))
		list = append(list, "  "+mark+name+"  "+m.policyBadge(p))
	}

	title := "Policies"
	switch {
	case st.loading:
		title += " (loading…)"
	case len(rows) == 0 && m.filters[ScreenPolicies] != "":
		list = []string{s.Muted.Render("nothing matches the filter")}
	case len(rows) == 0:
		list = []string{
			s.Muted.Render("no policy files found in:"),
		}
		for _, p := range m.policyPaths() {
			list = append(list, "  "+p)
		}
		list = append(list, "", s.Muted.Render("pass paths as arguments or set policy_dirs"))
	}
	if n := m.markedCount(); n > 0 {
		title += fmt.Sprintf(" · %d selected", n)
	}
	visible := paneRows(height)
	list = m.listLines(list, st.cursor, visible, leftWidth(width), m.focus == paneLeft)

	detail := scrolled(m.policyDetail(), st.scroll, visible)
	return m.twoPanes(title, list, m.policyDetailTitle(), detail, width, height)
}

// policyBadge is the short, coloured summary after a policy name.
func (m Model) policyBadge(p c7n.Policy) string {
	s := m.styles
	res := s.Muted.Render(strings.TrimPrefix(p.Resource, "aws."))
	switch {
	case !p.IsPull():
		return res + " " + s.Warn.Render(p.Mode)
	case len(p.DestructiveActions()) > 0:
		return res + " " + s.Danger.Render(strings.Join(p.DestructiveActions(), ","))
	case len(p.Actions) == 0:
		return res + " " + s.Muted.Render("report")
	}
	return res
}

func (m Model) policyDetailTitle() string {
	row, ok := m.cursorRow()
	if !ok {
		return "Details"
	}
	if row.policy < 0 {
		return m.policies.files[row.file].Rel
	}
	return m.policies.files[row.file].Policies[row.policy].Name
}

// policyDetail is the right pane: a summary and the YAML.
func (m Model) policyDetail() []string {
	s := m.styles
	row, ok := m.cursorRow()
	if !ok {
		return nil
	}
	f := m.policies.files[row.file]
	if row.policy < 0 {
		if f.Err != nil {
			return []string{s.Danger.Render(f.Err.Error())}
		}
		n := len(f.Policies)
		out := []string{s.Muted.Render(fmt.Sprintf("%d %s · enter fold · space select all · v validate · d dry-run", n, plural(n, "policy", "policies"))), ""}
		actions := map[string]bool{}
		for _, p := range f.Policies {
			for _, a := range p.Actions {
				actions[a] = true
			}
		}
		return append(out, m.yamlLines(f.Lines, 1, actions)...)
	}

	p := f.Policies[row.policy]
	mode := "pull"
	if !p.IsPull() {
		mode = p.Mode
	}
	var out []string
	out = append(out, s.Muted.Render(fmt.Sprintf("%s · mode %s · %s:%d", p.Resource, mode, f.Rel, p.Line)))
	if p.Description != "" {
		out = append(out, s.Item.Render(strings.SplitN(p.Description, "\n", 2)[0]))
	}
	if !p.IsPull() {
		out = append(out, s.Warn.Render("⚠ a live run deploys a Lambda + trigger; dry-run evaluates it as pull"))
	}
	filters := "none"
	if len(p.Filters) > 0 {
		filters = strings.Join(p.Filters, " · ")
	}
	out = append(out, s.Bold.Render("filters ")+filters)
	actionText := s.Muted.Render("none (report only)")
	if len(p.Actions) > 0 {
		var parts []string
		for _, a := range p.Actions {
			switch c7n.ClassifyAction(a) {
			case c7n.ActionDestructive:
				parts = append(parts, s.Danger.Render(a+" (destructive)"))
			case c7n.ActionMutating:
				parts = append(parts, s.Warn.Render(a))
			case c7n.ActionNotify:
				parts = append(parts, s.OK.Render(a))
			}
		}
		actionText = strings.Join(parts, s.Muted.Render(" · "))
	}
	out = append(out, s.Bold.Render("actions ")+actionText, "")

	actions := map[string]bool{}
	for _, a := range p.Actions {
		actions[a] = true
	}
	return append(out, m.yamlLines(f.PolicyText(p), p.Line, actions)...)
}

// yamlLines highlights YAML with line numbers starting at first.
func (m Model) yamlLines(lines []string, first int, actions map[string]bool) []string {
	width := len(itoa(first + len(lines)))
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		num := fmt.Sprintf("%*d ", width, first+i)
		out = append(out, m.styles.Muted.Render(num)+highlightYAML(m.styles, strings.ReplaceAll(l, "\t", "  "), actions))
	}
	return out
}
