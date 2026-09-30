package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

type resourcesState struct {
	pr       c7n.PolicyRun // whose resources are shown
	runTitle string
	loading  bool
	list     []c7n.Resource
	err      error
	cursor   int
	scroll   int // right pane
}

func (m Model) openResources(run runEntry, pr c7n.PolicyRun) (tea.Model, tea.Cmd) {
	m.res = resourcesState{pr: pr, runTitle: runTitle(run), loading: true}
	delete(m.filters, ScreenResources)
	m.screen, m.focus = ScreenResources, paneLeft
	return m, loadResources(pr)
}

// resourceRows are indexes into res.list after the "/" filter.
func (m Model) resourceRows() []int {
	filter := m.filters[ScreenResources]
	var rows []int
	for i, r := range m.res.list {
		text := r.ID
		for _, t := range r.Tags {
			text += " " + t.Key + "=" + t.Value
		}
		if matchesFilter(text, filter) {
			rows = append(rows, i)
		}
	}
	return rows
}

func (m Model) currentResource() (c7n.Resource, bool) {
	rows := m.resourceRows()
	if m.res.cursor < 0 || m.res.cursor >= len(rows) {
		return c7n.Resource{}, false
	}
	return m.res.list[rows[m.res.cursor]], true
}

func (m Model) updateResources(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.res
	page := paneRows(m.bodyHeight())

	if key.Matches(msg, k.Copy) {
		if r, ok := m.currentResource(); ok {
			m.setStatus("copied resource id " + r.ID)
			return m, tea.SetClipboard(r.ID)
		}
		return m, nil
	}

	if m.focus == paneRight {
		n := len(m.resourceDetail())
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

	n := len(m.resourceRows())
	move := func(to int) {
		st.cursor = min(max(to, 0), max(n-1, 0))
		st.scroll = 0
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
	case key.Matches(msg, k.Back):
		m.screen = ScreenRuns
		m.focus = paneRight
	}
	return m, nil
}

func (m Model) viewResources(width, height int) string {
	s := m.styles
	st := m.res
	visible := paneRows(height)

	if st.pr.Dir == "" {
		return m.twoPanes("Resources", []string{
			s.Muted.Render("pick a run in Runs, then a policy,"),
			s.Muted.Render("and press enter"),
		}, "Resource", nil, width, height)
	}

	title := fmt.Sprintf("%s · %s", st.pr.Policy, st.pr.Region)
	var list []string
	switch {
	case st.loading:
		list = []string{s.Muted.Render("loading…")}
	case st.err != nil:
		list = []string{s.Danger.Render(st.err.Error())}
	case len(st.list) == 0:
		list = []string{s.Muted.Render("no resources matched")}
	default:
		title += fmt.Sprintf(" · %d", len(st.list))
		for _, i := range m.resourceRows() {
			list = append(list, st.list[i].ID)
		}
		if len(list) == 0 {
			list = []string{s.Muted.Render("nothing matches the filter")}
		}
	}
	list = m.listLines(list, st.cursor, visible, leftWidth(width), m.focus == paneLeft)

	detailTitle := "Resource"
	if r, ok := m.currentResource(); ok {
		detailTitle = r.ID
	}
	return m.twoPanes(title, list, detailTitle, scrolled(m.resourceDetail(), st.scroll, visible), width, height)
}

// resourceDetail is the right pane: tags, then the full JSON.
func (m Model) resourceDetail() []string {
	s := m.styles
	r, ok := m.currentResource()
	if !ok {
		return nil
	}
	var out []string
	out = append(out, s.Muted.Render(m.res.runTitle+" · "+m.res.pr.Resource))
	if len(r.Tags) > 0 {
		var tags []string
		for _, t := range r.Tags {
			tags = append(tags, s.Key.Render(t.Key)+"="+t.Value)
		}
		out = append(out, s.Bold.Render("tags ")+strings.Join(tags, "  "))
	}
	out = append(out, "")
	for _, l := range strings.Split(r.Pretty(), "\n") {
		out = append(out, highlightJSON(s, l))
	}
	return out
}
