package app

// State-machine tests for the live-run gate (SPEC §6, §11). Each test drives
// Update with messages, exactly as the terminal would, and checks whether a
// live run was started. Most of them try to get past the gate without typing
// the expected text.

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/config"
)

var (
	ec2Stop  = c7n.Policy{Name: "ec2-stop", Resource: "aws.ec2", Actions: []string{"stop"}, File: "p.yml", Line: 2}
	s3Tag    = c7n.Policy{Name: "s3-tag", Resource: "aws.s3", Actions: []string{"tag"}, File: "p.yml", Line: 8}
	offhours = c7n.Policy{Name: "ec2-offhours", Resource: "aws.ec2", Mode: "periodic", Actions: []string{"stop"}, File: "p.yml", Line: 14}
)

var (
	enter     = tea.KeyPressMsg{Code: tea.KeyEnter}
	esc       = tea.KeyPressMsg{Code: tea.KeyEscape}
	backspace = tea.KeyPressMsg{Code: tea.KeyBackspace}
	tab       = tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTab  = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	ctrlU     = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	liveKey   = press('R', "R")
)

// typed turns text into one key press per character.
func typed(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, press(r, string(r)))
	}
	return msgs
}

// seq flattens messages and message lists into one sequence.
func seq(parts ...any) []tea.Msg {
	var out []tea.Msg
	for _, p := range parts {
		switch p := p.(type) {
		case []tea.Msg:
			out = append(out, p...)
		case tea.Msg:
			out = append(out, p)
		}
	}
	return out
}

// withSelection loads policies as one file and selects all of them.
// Policies without a file or line get distinct ones.
func withSelection(cfg config.Config, policies ...c7n.Policy) Model {
	m := New(cfg, Options{})
	m.store.Root = filepath.Join(os.TempDir(), "lazyc7n-gate-tests") // never the real state dir
	m.policies.loading = false
	for i := range policies {
		if policies[i].File == "" {
			policies[i].File = "p.yml"
		}
		if policies[i].Line == 0 {
			policies[i].Line = 100 + i
		}
		m.policies.marked[policyKey(policies[i])] = true
	}
	m.policies.files = []c7n.PolicyFile{{Path: "p.yml", Policies: policies}}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return next.(Model)
}

// drive feeds msgs into Update one by one and returns the live runs that
// were started: a live job appearing in the model. Commands are never
// executed, so no process is ever started by these tests.
func drive(m Model, msgs []tea.Msg) (Model, []LiveRunRequest) {
	var runs []LiveRunRequest
	for _, msg := range msgs {
		before := len(m.jobs.list)
		next, _ := m.Update(msg)
		m = next.(Model)
		for _, j := range m.jobs.list[before:] {
			if j.kind == "live" {
				runs = append(runs, *j.req)
			}
		}
	}
	return m, runs
}

func names(ps []c7n.Policy) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestGate(t *testing.T) {
	cases := []struct {
		name     string
		policies []c7n.Policy
		msgs     []tea.Msg
		wantRun  []string // policy names of the single run expected; nil = no run
		wantOpen bool     // gate still open at the end
	}{
		// The happy paths.
		{"one pull policy, name typed", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("ec2-stop"), enter), []string{"ec2-stop"}, false},
		{"surrounding spaces are ignored", []c7n.Policy{ec2Stop},
			seq(liveKey, typed(" ec2-stop "), enter), []string{"ec2-stop"}, false},
		{"several policies, count typed", []c7n.Policy{ec2Stop, s3Tag},
			seq(liveKey, typed("2"), enter), []string{"ec2-stop", "s3-tag"}, false},
		{"retry after a mismatch", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("nope"), enter, typed("ec2-stop"), enter), []string{"ec2-stop"}, false},
		{"typo fixed with backspace", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("ec2-stopp"), backspace, enter), []string{"ec2-stop"}, false},
		{"non-pull needs name then DEPLOY", []c7n.Policy{offhours},
			seq(liveKey, typed("ec2-offhours"), enter, typed("DEPLOY"), enter), []string{"ec2-offhours"}, false},
		{"mixed selection needs count then DEPLOY", []c7n.Policy{ec2Stop, offhours},
			seq(liveKey, typed("2"), enter, typed("DEPLOY"), enter), []string{"ec2-stop", "ec2-offhours"}, false},

		// Wrong text.
		{"nothing typed", []c7n.Policy{ec2Stop}, seq(liveKey, enter), nil, true},
		{"wrong name", []c7n.Policy{ec2Stop}, seq(liveKey, typed("s3-tag"), enter), nil, true},
		{"prefix of the name", []c7n.Policy{ec2Stop}, seq(liveKey, typed("ec2-sto"), enter), nil, true},
		{"name plus extra", []c7n.Policy{ec2Stop}, seq(liveKey, typed("ec2-stopx"), enter), nil, true},
		{"name in another case", []c7n.Policy{ec2Stop}, seq(liveKey, typed("EC2-STOP"), enter), nil, true},
		{"yes instead of the name", []c7n.Policy{ec2Stop}, seq(liveKey, typed("y"), enter), nil, true},
		{"ALL for several policies", []c7n.Policy{ec2Stop, s3Tag}, seq(liveKey, typed("ALL"), enter), nil, true},
		{"wrong count", []c7n.Policy{ec2Stop, s3Tag}, seq(liveKey, typed("3"), enter), nil, true},
		{"one name for several policies", []c7n.Policy{ec2Stop, s3Tag}, seq(liveKey, typed("ec2-stop"), enter), nil, true},
		{"cleared with ctrl+u", []c7n.Policy{ec2Stop}, seq(liveKey, typed("ec2-stop"), ctrlU, enter), nil, true},
		{"a failed attempt does not keep the text", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("ec2-"), enter, typed("stop"), enter), nil, true},

		// Leaving or poking around.
		{"esc before typing", []c7n.Policy{ec2Stop}, seq(liveKey, esc, enter), nil, false},
		{"esc after typing the name", []c7n.Policy{ec2Stop}, seq(liveKey, typed("ec2-stop"), esc, enter), nil, false},
		{"reopen after esc starts empty", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("ec2-stop"), esc, liveKey, enter), nil, true},
		{"second R is just a letter", []c7n.Policy{ec2Stop}, seq(liveKey, liveKey, enter), nil, true},
		{"second R after the name", []c7n.Policy{ec2Stop}, seq(liveKey, typed("ec2-stop"), liveKey, enter), nil, true},
		{"tab and shift+tab", []c7n.Policy{ec2Stop}, seq(liveKey, tab, shiftTab, enter), nil, true},
		{"focus and resize events", []c7n.Policy{ec2Stop},
			seq(liveKey, tea.BlurMsg{}, tea.FocusMsg{}, tea.WindowSizeMsg{Width: 50, Height: 10}, enter), nil, true},
		{"paste with a newline is not Enter", []c7n.Policy{ec2Stop},
			seq(liveKey, tea.PasteMsg{Content: "ec2-stop\n"}), nil, true},
		{"enter with the gate closed", []c7n.Policy{ec2Stop}, seq(typed("ec2-stop"), enter), nil, false},
		{"approval does not repeat", []c7n.Policy{ec2Stop},
			seq(liveKey, typed("ec2-stop"), enter, typed("ec2-stop"), enter, enter), []string{"ec2-stop"}, false},

		// The deploy step.
		{"non-pull stops after the name", []c7n.Policy{offhours},
			seq(liveKey, typed("ec2-offhours"), enter), nil, true},
		{"DEPLOY instead of the name", []c7n.Policy{offhours}, seq(liveKey, typed("DEPLOY"), enter), nil, true},
		{"deploy in lower case", []c7n.Policy{offhours},
			seq(liveKey, typed("ec2-offhours"), enter, typed("deploy"), enter), nil, true},
		{"name twice", []c7n.Policy{offhours},
			seq(liveKey, typed("ec2-offhours"), enter, typed("ec2-offhours"), enter), nil, true},
		{"esc on the deploy step", []c7n.Policy{offhours},
			seq(liveKey, typed("ec2-offhours"), enter, esc, typed("DEPLOY"), enter), nil, false},
		{"unknown mode needs DEPLOY too", []c7n.Policy{{Name: "x", Mode: "brand-new"}},
			seq(liveKey, typed("x"), enter), nil, true},
		{"policy with an empty name is refused before the gate", []c7n.Policy{{Name: ""}},
			seq(liveKey, enter, typed(" "), enter), nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, runs := drive(withSelection(config.Default(), tc.policies...), tc.msgs)
			if tc.wantRun == nil && len(runs) > 0 {
				t.Fatalf("live run started: %v", names(runs[0].Policies))
			}
			if tc.wantRun != nil {
				if len(runs) != 1 {
					t.Fatalf("want exactly 1 run, got %d", len(runs))
				}
				if got := names(runs[0].Policies); !slices.Equal(got, tc.wantRun) {
					t.Fatalf("ran %v, want %v", got, tc.wantRun)
				}
			}
			if m.gate.open() != tc.wantOpen {
				t.Fatalf("gate open = %v, want %v", m.gate.open(), tc.wantOpen)
			}
		})
	}
}

func TestGateBlocksNavigationAndQuit(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), ec2Stop), seq(liveKey))
	for _, msg := range seq(tab, shiftTab, typed("lhq?")) {
		next, cmd := m.Update(msg)
		m = next.(Model)
		if cmd != nil {
			t.Fatalf("%v returned a command while the gate is open", msg)
		}
	}
	if m.Screen() != ScreenPolicies || !m.gate.open() || m.help.ShowAll {
		t.Fatalf("screen=%v gate=%v help=%v", m.Screen(), m.gate.open(), m.help.ShowAll)
	}
	if m.gate.typed != "lhq?" {
		t.Fatalf("typed = %q", m.gate.typed)
	}
}

func TestCtrlCQuitsWithoutRunning(t *testing.T) {
	_, cmd := send(withSelection(config.Default(), ec2Stop), seq(liveKey, typed("ec2-stop"))...)
	if cmd != nil {
		t.Fatal("unexpected command before ctrl+c")
	}
	m, _ := drive(withSelection(config.Default(), ec2Stop), seq(liveKey, typed("ec2-stop")))
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestRWithoutSelectionOpensNothing(t *testing.T) {
	m, runs := drive(withSelection(config.Default()), seq(liveKey, enter))
	if m.gate.open() || len(runs) > 0 {
		t.Fatalf("gate=%v runs=%d", m.gate.open(), len(runs))
	}
	if !strings.Contains(plain(m), "select a policy first") {
		t.Fatalf("no hint:\n%s", plain(m))
	}
}

// Changing the selection while the gate is open must not change what runs.
func TestGateConfirmsWhatItShowed(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), ec2Stop, s3Tag), seq(liveKey, typed("x"), esc))
	m.policies.marked[policyKey(s3Tag)] = false
	m, _ = drive(m, seq(liveKey)) // gate for ec2-stop only
	m.policies.files[0].Policies[0].Name = "changed"
	m.policies.marked[policyKey(ec2Stop)] = false
	m.policies.marked[policyKey(s3Tag)] = true

	_, runs := drive(m, seq(typed("s3-tag"), enter))
	if len(runs) > 0 {
		t.Fatal("confirmed with the new selection's name")
	}
	m, _ = drive(m, seq(typed("changed"), enter))
	_, runs = drive(m, seq(typed("ec2-stop"), enter))
	if len(runs) != 1 || runs[0].Policies[0].Name != "ec2-stop" {
		t.Fatalf("runs = %+v", runs)
	}
}

// Weakening the config must never weaken the gate: the gate does not read
// the config at all. (A config with yes-no does not load; see the config
// tests. Here we force it past validation anyway.)
func TestConfigCannotWeakenTheGate(t *testing.T) {
	cfg := config.Default()
	cfg.Safety.DefaultDryRun = false
	cfg.Safety.ConfirmLive = "yes-no"

	for _, msgs := range [][]tea.Msg{
		seq(liveKey, enter),
		seq(liveKey, typed("y"), enter),
		seq(liveKey, typed("yes"), enter),
	} {
		if _, runs := drive(withSelection(cfg, ec2Stop), msgs); len(runs) > 0 {
			t.Fatalf("ran with weakened config after %d msgs", len(msgs))
		}
	}
	if _, runs := drive(withSelection(cfg, ec2Stop), seq(liveKey, typed("ec2-stop"), enter)); len(runs) != 1 {
		t.Fatal("correct name did not run")
	}
}

// Random key mashing never starts a run: first without Enter at all, then
// with Enter but without one letter the name needs.
func TestRandomKeysNeverRun(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	noEnter := seq(liveKey, esc, backspace, tab, shiftTab, ctrlU, typed("ec2-stopDEPLOY2 q"))
	noP := seq(liveKey, esc, backspace, tab, ctrlU, enter, typed("ec2-sto DELOY2q"))

	for _, keys := range [][]tea.Msg{noEnter, noP} {
		for range 2000 {
			msgs := seq(liveKey)
			for range rng.IntN(40) {
				msgs = append(msgs, keys[rng.IntN(len(keys))])
			}
			for _, pol := range []c7n.Policy{ec2Stop, offhours} {
				if _, runs := drive(withSelection(config.Default(), pol), msgs); len(runs) > 0 {
					t.Fatalf("run started by %v", msgs)
				}
			}
		}
	}
}

func TestGateView(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), ec2Stop, s3Tag, offhours), seq(liveKey))
	out := plain(m)
	for _, want := range []string{
		"LIVE RUN · step 1/2", "ec2-stop", "s3-tag", "[destructive]", "deploys Lambda",
		"2 policies have DESTRUCTIVE actions: stop", "Type 3 (the number of policies)",
		"esc cancel", "LIVE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "q quit") {
		t.Errorf("normal key help shown inside the gate:\n%s", out)
	}

	m, _ = drive(m, seq(typed("x"), enter))
	if out := plain(m); !strings.Contains(out, "does not match") {
		t.Errorf("no mismatch message:\n%s", out)
	}

	m, _ = drive(m, seq(typed("3"), enter))
	out = plain(m)
	for _, want := range []string{"DEPLOYS INFRASTRUCTURE · step 2/2", "ec2-offhours  mode: periodic", "Type DEPLOY"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGateViewSinglePolicy(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), s3Tag), seq(liveKey, typed("s3")))
	out := plain(m)
	for _, want := range []string{"LIVE RUN · step 1/1", "Type the policy name to run it live: s3-tag", "> s3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "DESTRUCTIVE") {
		t.Errorf("destructive warning for a tag-only policy:\n%s", out)
	}
}

func TestGateViewFitsSmallWindowWithManyPolicies(t *testing.T) {
	var many []c7n.Policy
	for i := range 40 {
		p := ec2Stop
		p.Name = "policy-" + strings.Repeat("x", i%7) + itoa(i)
		p.Line = 0
		many = append(many, p)
	}
	m, _ := drive(withSelection(config.Default(), many...), seq(liveKey, tea.WindowSizeMsg{Width: 80, Height: 24}))
	out := plain(m)
	lines := strings.Split(out, "\n")
	if len(lines) > 24 {
		t.Errorf("height %d > 24:\n%s", len(lines), out)
	}
	for _, want := range []string{"more (all 40 will run)", "Type 40"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestConfirmedRunIsALiveJob(t *testing.T) {
	m, runs := drive(withSelection(config.Default(), ec2Stop), seq(liveKey, typed("ec2-stop"), enter))
	if len(runs) != 1 {
		t.Fatalf("runs = %d", len(runs))
	}
	j := m.jobs.list[0]
	if j.kind != "live" || m.Screen() != ScreenJobs || !strings.Contains(plain(m), "LIVE ec2-stop") {
		t.Fatalf("job = %+v\n%s", j, plain(m))
	}
	if runs[0].Spec.DryRun || !slices.Equal(runs[0].Spec.Policies, []string{"ec2-stop"}) || !slices.Equal(runs[0].Spec.Files, []string{"p.yml"}) {
		t.Fatalf("spec = %+v", runs[0].Spec)
	}
	if slices.Contains(runs[0].Argv, "--dryrun") || !slices.Contains(runs[0].Argv, "ec2-stop") {
		t.Fatalf("argv = %q", runs[0].Argv)
	}
}

func TestMismatchMessageClearsWhenTypingAgain(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), ec2Stop), seq(liveKey, typed("x"), enter))
	if !m.gate.mismatch {
		t.Fatal("no mismatch after a wrong answer")
	}
	m, _ = drive(m, seq(typed("e")))
	if m.gate.mismatch || strings.Contains(plain(m), "does not match") {
		t.Fatal("mismatch message still shown while retyping")
	}
}

func TestLiveRunOnlyFromPolicies(t *testing.T) {
	m, _ := drive(withSelection(config.Default(), ec2Stop), seq(press('2', "2"), liveKey))
	if m.gate.open() || !strings.Contains(m.status, "Policies screen") {
		t.Fatalf("gate=%v status=%q", m.gate.open(), m.status)
	}
	if _, runs := drive(m, seq(typed("ec2-stop"), enter)); len(runs) > 0 {
		t.Fatal("ran from the Runs screen")
	}
}

// Actions made destructive in the config are flagged by the gate; the
// confirmation itself does not change.
func TestGateUsesConfiguredActionClasses(t *testing.T) {
	if err := c7n.SetActionOverrides(c7n.ActionOverrides{Destructive: []string{"tag"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c7n.SetActionOverrides(c7n.ActionOverrides{}) })

	m, _ := drive(withSelection(config.Default(), s3Tag), seq(liveKey))
	if out := plain(m); !strings.Contains(out, "DESTRUCTIVE actions: tag") || !strings.Contains(out, "[destructive]") {
		t.Fatalf("tag not flagged:\n%s", out)
	}
	if _, runs := drive(m, seq(enter)); len(runs) > 0 {
		t.Fatal("ran without the name")
	}
	if _, runs := drive(m, seq(typed("s3-tag"), enter)); len(runs) != 1 {
		t.Fatal("name did not run")
	}
}

func TestGateShowsTarget(t *testing.T) {
	m := withSelection(config.Default(), ec2Stop)
	m.cloud = cloudContext([]string{"AWS_PROFILE=prod", "AWS_REGION=eu-west-1"}, "")
	m, _ = drive(m, seq(liveKey))
	if out := plain(m); !strings.Contains(out, "target: aws profile prod · eu-west-1") {
		t.Fatalf("no target:\n%s", out)
	}
}
