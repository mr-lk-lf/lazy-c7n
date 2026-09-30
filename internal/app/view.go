package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/config"
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

func (m Model) size() (int, int) {
	if m.width <= 0 || m.height <= 0 {
		return defaultWidth, defaultHeight
	}
	return m.width, m.height
}

func (m Model) render() string {
	w, h := m.size()
	header := m.header(w)
	footer := m.footer(w)
	bodyHeight := max(h-lipgloss.Height(header)-lipgloss.Height(footer), 3)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.body(w, bodyHeight), footer)
}

func (m Model) header(width int) string {
	parts := []string{m.styles.Brand.Render("lazyc7n"), " "}
	for i, s := range Screens {
		style := m.styles.Tab
		if s == m.screen {
			style = m.styles.ActiveTab
		}
		parts = append(parts, style.Render(itoa(i+1)+" "+s.Title()))
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	var info []string
	if n := m.runningJobs(); n > 0 {
		info = append(info, m.styles.Warn.Render("⟳ "+itoa(n)+" running"))
	}
	switch {
	case m.version != "":
		info = append(info, m.styles.Muted.Render("c7n "+m.version))
	case m.versionErr != "":
		info = append(info, m.styles.Danger.Render("custodian not found"))
	}
	right := strings.Join(info, "  ")
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if right == "" || gap < 1 {
		return ansi.Truncate(left, width, "")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) body(width, height int) string {
	if m.gate.open() {
		return m.gateView(width, height)
	}
	switch m.screen {
	case ScreenPolicies:
		return m.viewPolicies(width, height)
	case ScreenRuns:
		return m.viewRuns(width, height)
	case ScreenResources:
		return m.viewResources(width, height)
	case ScreenJobs:
		return m.viewJobs(width, height)
	case ScreenSchema:
		return m.viewSchema(width, height)
	}
	return m.pane(m.screen.Title(), []string{m.styles.Muted.Render("not implemented yet")}, width, height, true)
}

func (m Model) footer(width int) string {
	badge := m.styles.DryBadge.Render("DRY")
	if !m.cfg.Safety.DefaultDryRun || m.gate.open() {
		badge = m.styles.LiveBadge.Render("LIVE")
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top,
		badge,
		m.styles.Status.Render(" "+m.runnerLabel()+"  "),
	)
	if m.gate.open() {
		// The gate shows its own keys; the normal ones do not apply.
		return left
	}
	m.help.SetWidth(max(width-lipgloss.Width(left), 0))
	footer := lipgloss.JoinHorizontal(lipgloss.Top, left, m.help.View(screenHelp{m.keys, m.screen}))

	var top string
	switch {
	case m.filtering:
		top = m.styles.Input.Render("/" + m.filters[m.screen] + "█")
	case m.status != "" && m.statusErr:
		top = m.styles.StatusError.Render(m.status)
	case m.status != "":
		top = m.styles.Status.Render(m.status)
	}
	if top != "" {
		footer = ansi.Truncate(top, width, "…") + "\n" + footer
	}
	return footer
}

func (m Model) runnerLabel() string {
	switch m.cfg.Runner.Kind {
	case config.RunnerBinary:
		return "binary: " + m.cfg.Runner.Custodian
	case config.RunnerCommand:
		return "command: " + strings.Join(m.cfg.Runner.Command, " ")
	case config.RunnerDocker:
		return "docker: " + m.cfg.Runner.Image
	}
	return string(m.cfg.Runner.Kind)
}

// twoPanes lays out a list on the left and details on the right.
func (m Model) twoPanes(leftTitle string, left []string, rightTitle string, right []string, width, height int) string {
	lw := leftWidth(width)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.pane(leftTitle, left, lw, height, m.focus == paneLeft),
		m.pane(rightTitle, right, width-lw, height, m.focus == paneRight),
	)
}

// pane draws a bordered box of exactly width x height with a title line.
// Lines longer than the box are cut.
func (m Model) pane(title string, lines []string, width, height int, focused bool) string {
	style := m.styles.Pane
	if focused {
		style = m.styles.FocusedPane
	}
	inner := max(width-4, 1)
	rows := max(height-2, 1)
	out := make([]string, 0, rows)
	out = append(out, ansi.Truncate(m.styles.PaneTitle.Render(title), inner, "…"))
	for _, l := range lines {
		if len(out) == rows {
			break
		}
		out = append(out, ansi.Truncate(l, inner, "…"))
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return style.Width(width).Height(height).Render(strings.Join(out, "\n"))
}

// paneRows is how many content lines fit in a pane of the given height
// (border and title removed).
func paneRows(height int) int { return max(height-3, 1) }

// windowStart keeps the cursor in view, roughly centred.
func windowStart(cursor, n, rows int) int {
	if n <= rows {
		return 0
	}
	return min(max(cursor-rows/2, 0), n-rows)
}

// listLines renders the visible part of a list with the cursor row
// highlighted across the pane width.
func (m Model) listLines(rows []string, cursor, visible, width int, focused bool) []string {
	start := windowStart(cursor, len(rows), visible)
	end := min(start+visible, len(rows))
	inner := max(width-4, 1)
	out := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if i != cursor {
			out = append(out, rows[i])
			continue
		}
		text := ansi.Truncate(ansi.Strip(rows[i]), inner, "…")
		text += strings.Repeat(" ", max(inner-ansi.StringWidth(text), 0))
		if focused {
			out = append(out, m.styles.Cursor.Render(text))
		} else {
			out = append(out, m.styles.CursorDim.Render(text))
		}
	}
	return out
}

// leftWidth mirrors twoPanes, for lists that need their pane width.
func leftWidth(width int) int {
	lw := min(max(width*2/5, 24), 64)
	if width-lw < 20 {
		lw = width / 2
	}
	return lw
}

// scrolled returns lines[scroll : scroll+rows], with scroll clamped.
func scrolled(lines []string, scroll, rows int) []string {
	scroll = min(max(scroll, 0), max(len(lines)-rows, 0))
	return lines[scroll:min(scroll+rows, len(lines))]
}

// scrollBy moves a scroll offset, keeping it within content of n lines.
func scrollBy(scroll, delta, n, rows int) int {
	return min(max(scroll+delta, 0), max(n-rows, 0))
}

func itoa(n int) string { return strconv.Itoa(n) }

// rightInner is the text width of the right pane on two-pane screens.
func (m Model) rightInner() int {
	w, _ := m.size()
	return max(w-leftWidth(w)-4, 10)
}

// wrapped breaks text into lines of at most width cells, each styled.
func wrapped(style lipgloss.Style, text string, width int) []string {
	var out []string
	for _, l := range strings.Split(ansi.Hardwrap(text, max(width, 10), true), "\n") {
		out = append(out, style.Render(l))
	}
	return out
}
