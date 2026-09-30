package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
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
	if m.gate.open() {
		return m.gateView(width, height)
	}
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
	if !m.cfg.Safety.DefaultDryRun || m.gate.open() {
		badge = m.styles.LiveBadge.Render("LIVE")
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top,
		badge,
		m.styles.Status.Render(" runner: "+m.cfg.Runner.Custodian+"  "),
	)
	if m.gate.open() {
		// The gate shows its own keys; the normal ones do not apply.
		return left
	}
	m.help.SetWidth(max(width-lipgloss.Width(left), 0))
	footer := lipgloss.JoinHorizontal(lipgloss.Top, left, m.help.View(m.keys))
	if m.status != "" {
		footer = m.styles.Status.Render(m.status) + "\n" + footer
	}
	return footer
}

// gateView renders the live-run confirmation in place of the screen body.
func (m Model) gateView(width, height int) string {
	g := m.gate
	s := m.styles
	req := g.req
	var lines []string

	title := "LIVE RUN"
	if g.step == gateDeploy {
		title = "DEPLOYS INFRASTRUCTURE"
	}
	lines = append(lines, s.Danger.Render(fmt.Sprintf("%s · step %d/%d", title, stepNumber(g.step), g.totalSteps())))

	var body []c7n.Policy
	var tail []string
	switch g.step {
	case gateClosed:
		return ""
	case gateConfirm:
		lines = append(lines, s.Item.Render("This will change real resources, with your current credentials."))
		if len(req.Argv) > 0 {
			lines = append(lines, s.Muted.Render("$ "+strings.Join(req.Argv, " ")))
		}
		lines = append(lines, s.Muted.Render("backend: "+string(m.cfg.Runner.Kind)), "")
		body = req.Policies

		var destructive []string
		n := 0
		for _, p := range req.Policies {
			if d := p.DestructiveActions(); len(d) > 0 {
				destructive = append(destructive, d...)
				n++
			}
		}
		if n > 0 {
			tail = append(tail, "", s.Danger.Render(fmt.Sprintf("!! %d %s DESTRUCTIVE actions: %s",
				n, plural(n, "policy has", "policies have"), strings.Join(uniq(destructive), ", "))))
		}
		if len(req.Policies) == 1 {
			tail = append(tail, "", s.Item.Render("Type the policy name to run it live: "+g.expected()))
		} else {
			tail = append(tail, "", s.Item.Render(fmt.Sprintf("Type %s (the number of policies) to run them live:", g.expected())))
		}
	case gateDeploy:
		lines = append(lines,
			s.Item.Render("These policies are not mode: pull. A live run does NOT evaluate resources now:"),
			s.Item.Render("it creates or updates a Lambda function and its trigger in your account."),
			"")
		body = req.NonPull()
		tail = append(tail, "", s.Item.Render("Type "+deployWord+" to continue:"))
	}

	tail = append(tail, s.Input.Render("> "+g.typed+"█"))
	if g.mismatch {
		tail = append(tail, s.Danger.Render("does not match, nothing was run; try again"))
	}
	tail = append(tail, s.Muted.Render("enter confirm · esc cancel"))

	// Room left for the policy list: body height minus the border (2), the
	// lines above and below, and one spare line for the mismatch message.
	room := height - 2 - len(lines) - len(tail)
	if !g.mismatch {
		room--
	}
	lines = append(lines, m.gatePolicyRows(g.step, body, room)...)
	lines = append(lines, tail...)
	return s.GatePane.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

// gatePolicyRows lists policies in at most maxRows lines, the last one
// saying how many were left out.
func (m Model) gatePolicyRows(step gateStep, policies []c7n.Policy, maxRows int) []string {
	maxRows = max(maxRows, 1)
	shown := policies
	if len(policies) > maxRows {
		shown = policies[:maxRows-1]
	}
	nameW, resW := 0, 0
	for _, p := range shown {
		nameW = max(nameW, len(p.Name))
		resW = max(resW, len(p.Resource))
	}
	var rows []string
	for _, p := range shown {
		rows = append(rows, m.gatePolicyRow(step, p, nameW, resW))
	}
	if len(shown) < len(policies) {
		rows = append(rows, m.styles.Muted.Render(fmt.Sprintf("  … and %d more (all %d will run)",
			len(policies)-len(shown), len(policies))))
	}
	return rows
}

func (m Model) gatePolicyRow(step gateStep, p c7n.Policy, nameW, resW int) string {
	switch step {
	case gateClosed, gateConfirm:
	case gateDeploy:
		return m.styles.Warn.Render(fmt.Sprintf("  %-*s  mode: %s", nameW, p.Name, p.Mode))
	}
	actions := "no actions (report only)"
	if len(p.Actions) > 0 {
		actions = "actions: " + strings.Join(p.Actions, ", ")
	}
	row := fmt.Sprintf("  %-*s  %-*s  %s", nameW, p.Name, resW, p.Resource, actions)
	switch {
	case !p.IsPull():
		return m.styles.Warn.Render(row + "  [mode " + p.Mode + ": deploys Lambda]")
	case len(p.DestructiveActions()) > 0:
		return m.styles.Danger.Render(row + "  [destructive]")
	}
	return m.styles.Item.Render(row)
}

func stepNumber(s gateStep) int {
	switch s {
	case gateClosed, gateConfirm:
		return 1
	case gateDeploy:
		return 2
	}
	return 0
}

func uniq(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
