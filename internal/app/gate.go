package app

// The live-run gate (SPEC §6). This is the most safety-critical code in
// lazyc7n: a live run can only start from gate.submit, after the user has
// typed the expected text at every step. Any change here needs a test in
// gate_test.go that drives Update with messages.
//
// Steps:
//
//	gateClosed --R--> gateConfirm --ok--> gateDeploy --ok--> run
//	                       |    (only if a policy is not pull)
//	                       +--ok (all pull)---------------> run
//
// Esc at any step closes the gate without running anything. A wrong answer
// keeps the gate on the same step and clears what was typed.

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

type gateStep int

const (
	gateClosed  gateStep = iota
	gateConfirm          // type the policy name, or the number of policies
	gateDeploy           // type DEPLOY: some policy is not pull mode
)

// deployWord is what the user types to accept that serverless policies
// deploy infrastructure.
const deployWord = "DEPLOY"

// LiveRunRequest is what the gate shows and, once confirmed, what runs.
type LiveRunRequest struct {
	Policies []c7n.Policy
	// Argv is the exact custodian command line. Filled by the runner (M2);
	// shown only when present.
	Argv []string
}

// NonPull returns the policies whose live run deploys infrastructure.
func (r LiveRunRequest) NonPull() []c7n.Policy {
	var out []c7n.Policy
	for _, p := range r.Policies {
		if !p.IsPull() {
			out = append(out, p)
		}
	}
	return out
}

// liveGate is the state of the confirmation dialog.
type liveGate struct {
	step     gateStep
	req      LiveRunRequest // frozen copy taken when the gate opened
	typed    string
	mismatch bool // the last Enter did not match
}

func (g liveGate) open() bool { return g.step != gateClosed }

// openGate starts a confirmation for req. It takes its own copy, so nothing
// that changes the selection later can change what gets confirmed.
func openGate(req LiveRunRequest) liveGate {
	req.Policies = slices.Clone(req.Policies)
	req.Argv = slices.Clone(req.Argv)
	return liveGate{step: gateConfirm, req: req}
}

// expected is the exact text the user must type at the current step.
func (g liveGate) expected() string {
	switch g.step {
	case gateClosed:
		return ""
	case gateConfirm:
		if len(g.req.Policies) == 1 {
			return g.req.Policies[0].Name
		}
		return strconv.Itoa(len(g.req.Policies))
	case gateDeploy:
		return deployWord
	}
	return ""
}

// update handles a key while the gate is open. It returns the new gate and,
// only when the last step was confirmed, approved = true.
func (g liveGate) update(msg tea.KeyPressMsg) (next liveGate, approved bool) {
	switch msg.Code {
	case tea.KeyEscape:
		return liveGate{}, false
	case tea.KeyEnter:
		return g.submit()
	}

	g.mismatch = false // the user is trying again
	switch {
	case msg.Code == tea.KeyBackspace:
		if r := []rune(g.typed); len(r) > 0 {
			g.typed = string(r[:len(r)-1])
		}
	case msg.Code == 'u' && msg.Mod == tea.ModCtrl:
		g.typed = ""
	default:
		g.typed += msg.Text // empty for non-printable keys such as tab or arrows
	}
	return g, false
}

// paste appends pasted text; line breaks are dropped so a paste can never
// act as Enter.
func (g liveGate) paste(text string) liveGate {
	g.typed += strings.NewReplacer("\r", "", "\n", "").Replace(text)
	return g
}

func (g liveGate) submit() (liveGate, bool) {
	want := g.expected()
	if want == "" || strings.TrimSpace(g.typed) != want {
		g.typed = ""
		g.mismatch = true
		return g, false
	}
	g.typed, g.mismatch = "", false
	switch g.step {
	case gateClosed:
		return liveGate{}, false
	case gateConfirm:
		if len(g.req.NonPull()) > 0 {
			g.step = gateDeploy
			return g, false
		}
		return liveGate{}, true
	case gateDeploy:
		return liveGate{}, true
	}
	return liveGate{}, false
}

// totalSteps is 2 when a deploy confirmation will follow, else 1.
func (g liveGate) totalSteps() int {
	if len(g.req.NonPull()) > 0 {
		return 2
	}
	return 1
}
