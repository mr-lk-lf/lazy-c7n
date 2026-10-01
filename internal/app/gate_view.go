package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

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
		lines = append(lines, s.Bold.Render("target: "+m.cloud))
		if len(req.Argv) > 0 {
			cmd := wrapped(s.Muted, "$ "+strings.Join(req.Argv, " "), width-4)
			if len(cmd) > 3 {
				cmd = append(cmd[:2], s.Muted.Render("… (the full command is in Jobs once it starts)"))
			}
			lines = append(lines, cmd...)
		}
		lines = append(lines, s.Muted.Render("backend: "+string(m.cfg.Runner.Kind)+" · "+m.cacheNote()), "")
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
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, max(width-4, 1), "…")
	}
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

// cacheNote explains c7n's resource cache, shared by dry and live runs
// (SPEC §3): a live run soon after a dry-run acts on the same list.
func (m Model) cacheNote() string {
	period := m.cfg.Defaults.CachePeriod
	if period == "" {
		period = "15"
	}
	return "c7n reuses resource lists up to " + period + " min old (e.g. from your dry-run)"
}
