package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Fallback size before the first WindowSizeMsg (and in tests).
const (
	defaultWidth  = 80
	defaultHeight = 20
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "lazyc7n"
	return v
}

func (m Model) render() string {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		w, h = defaultWidth, defaultHeight
	}

	header := m.header()
	footer := m.footer(w)
	bodyHeight := max(h-lipgloss.Height(header)-lipgloss.Height(footer), 3)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.body(w, bodyHeight), footer)
}

func (m Model) header() string {
	parts := []string{m.styles.Brand.Render("lazyc7n"), " "}
	for _, s := range Screens {
		style := m.styles.Tab
		if s == m.screen {
			style = m.styles.ActiveTab
		}
		parts = append(parts, style.Render(s.Title()))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) body(width, height int) string {
	var lines []string
	switch m.screen {
	case ScreenPolicies:
		lines = append(lines, m.styles.Muted.Render("policy dirs"))
		for _, d := range m.cfg.PolicyDirs {
			lines = append(lines, m.styles.Item.Render("  "+d))
		}
	case ScreenRuns, ScreenResources, ScreenSchema, ScreenJobs:
		lines = append(lines, m.styles.Muted.Render("not implemented yet"))
	}
	content := m.styles.PaneTitle.Render(m.screen.Title()) + "\n\n" + strings.Join(lines, "\n")
	return m.styles.Pane.Width(width).Height(height).Render(content)
}

func (m Model) footer(width int) string {
	badge := m.styles.DryBadge.Render("DRY")
	if !m.cfg.Safety.DefaultDryRun {
		badge = m.styles.LiveBadge.Render("LIVE")
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top,
		badge,
		m.styles.Status.Render(" runner: "+m.cfg.Runner.Custodian+"  "),
	)
	m.help.SetWidth(max(width-lipgloss.Width(left), 0))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, m.help.View(m.keys))
}
