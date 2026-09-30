package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/config"
)

// TestFakeCustodian is not a real test: jobs tests run the test binary as
// a fake `custodian` (LC7N_FAKE_CUSTODIAN=1). It copies real fixture output
// for the policies asked for and records its arguments in argv.txt next to
// the -s dir.
func TestFakeCustodian(t *testing.T) {
	if os.Getenv("LC7N_FAKE_CUSTODIAN") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	os.Exit(fakeCustodian(args))
}

func fakeCustodian(args []string) int {
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Println("0.9.52")
		return 0
	case "schema":
		if len(args) > 1 && args[1] == "--json" {
			data, err := os.ReadFile(os.Getenv("LC7N_SCHEMA"))
			if err != nil {
				return 2
			}
			_, _ = os.Stdout.Write(data)
			return 0
		}
		fmt.Printf("Help\n----\n\nDocs for %s\n", args[1])
		return 0
	case "validate":
		for _, f := range args[1:] {
			if strings.Contains(f, "invalid") {
				fmt.Fprintln(os.Stderr, "2026-09-30 22:15:10,281: custodian.commands:ERROR Configuration invalid: "+f)
				return 1
			}
			fmt.Fprintln(os.Stderr, "2026-09-30 22:15:10,281: custodian.commands:INFO Configuration valid: "+f)
		}
		return 0
	case "run":
		var out string
		var names []string
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-s":
				out = args[i+1]
			case "-p":
				names = append(names, args[i+1])
			}
		}
		_ = os.MkdirAll(out, 0o755)
		_ = os.WriteFile(filepath.Join(filepath.Dir(out), "argv.txt"), []byte(strings.Join(args, "\n")), 0o644)
		src := os.Getenv("LC7N_FIXTURE_OUT")
		for _, n := range names {
			if err := os.CopyFS(filepath.Join(out, n), os.DirFS(filepath.Join(src, n))); err != nil {
				fmt.Fprintln(os.Stderr, "fake: "+err.Error())
				return 2
			}
			fmt.Fprintf(os.Stderr, "2026-09-30 22:15:11,130: custodian.policy:INFO policy:%s resource:aws.x region:us-east-1 count:1 time:0.21\n", n)
		}
		return 0
	}
	return 2
}

// withFakeCustodian returns a model whose runner is the fake custodian,
// with the fixture policies loaded and a temporary state dir.
func withFakeCustodian(t *testing.T) Model {
	t.Helper()
	fixtureOut, err := filepath.Abs(fixtures + "/dryrun/out")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LC7N_FAKE_CUSTODIAN", "1")
	t.Setenv("LC7N_FIXTURE_OUT", fixtureOut)
	schema, err := filepath.Abs("../../tests/fixtures/real/c7n-0.9.52-schema-small.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LC7N_SCHEMA", schema)

	cfg := config.Default()
	cfg.StateDir = t.TempDir()
	cfg.Runner.Kind = config.RunnerCommand
	cfg.Runner.Command = []string{os.Args[0], "-test.run=^TestFakeCustodian$", "--"}
	m := New(cfg, Options{PolicyPaths: []string{fixtures + "/policies.yml", fixtures + "/invalid.yml"}})
	m, _ = send(m, tea.WindowSizeMsg{Width: 120, Height: 30}, loadPolicies(m.policyPaths())())
	return m
}

// settle runs cmd and every command that follows from it, feeding each
// message into Update, like the Bubble Tea runtime does.
func settle(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("commands did not settle")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		default:
			next, nc := m.Update(msg)
			m = next.(Model)
			queue = append(queue, nc)
		}
	}
	return m
}

func key1(r rune) tea.KeyPressMsg { return press(r, string(r)) }

// policyCursor moves the cursor to the named policy.
func policyCursor(t *testing.T, m Model, name string) Model {
	t.Helper()
	for i, r := range m.policyRows() {
		if r.policy >= 0 && m.policies.files[r.file].Policies[r.policy].Name == name {
			m.policies.cursor = i
			return m
		}
	}
	t.Fatalf("policy %s not found", name)
	return m
}

func TestDryRunEndToEnd(t *testing.T) {
	m := policyCursor(t, withFakeCustodian(t), "ec2-mark-stop")
	m, cmd := send(m, key1('d'))
	if m.Screen() != ScreenJobs || len(m.jobs.list) != 1 {
		t.Fatalf("screen=%v jobs=%d", m.Screen(), len(m.jobs.list))
	}
	m = settle(t, m, cmd)

	j := m.jobs.list[0]
	if !j.done || j.result.ExitCode != 0 || j.matched != 2 {
		t.Fatalf("job = %+v", j)
	}
	if !strings.Contains(m.status, "finished · 2 resources matched") {
		t.Errorf("status %q", m.status)
	}
	argv, err := os.ReadFile(filepath.Join(j.run.Dir, "argv.txt"))
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(string(argv), "\n")
	if !slices.Contains(args, "--dryrun") || !slices.Contains(args, "ec2-mark-stop") {
		t.Fatalf("custodian args %q", args)
	}
	if !slices.Contains(args, "-f") || !slices.Contains(args, m.store.CachePath()) {
		t.Errorf("no private cache: %q", args)
	}
	out := plain(m)
	for _, want := range []string{"DRY  ec2-mark-stop", "policy:ec2-mark-stop", "exit 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// The run is in the history, recorded and selected.
	m, _ = send(m, key1('2'))
	if r, ok := m.currentRun(); !ok || r.ID != j.run.ID || r.Kind != "dry-run" || r.matched() != 2 {
		t.Fatalf("current run = %+v", r)
	}
	if out := plain(m); !strings.Contains(out, "DRY") || !strings.Contains(out, "1 pol · 2 res") {
		t.Errorf("run not shown:\n%s", out)
	}
}

func TestValidate(t *testing.T) {
	m := withFakeCustodian(t)
	m, cmd := send(m, key1('v')) // cursor on the first file: invalid.yml
	m = settle(t, m, cmd)
	if !m.statusErr || !strings.Contains(m.status, "INVALID") {
		t.Fatalf("status %q", m.status)
	}

	m.screen = ScreenPolicies // focus stays on the right pane: v must still work
	m = policyCursor(t, m, "s3-untagged-owner")
	m, cmd = send(m, key1('v'))
	m = settle(t, m, cmd)
	if m.statusErr || !strings.HasSuffix(m.status, ": valid") {
		t.Fatalf("status %q", m.status)
	}
}

func TestStartFailureIsReported(t *testing.T) {
	m := withFakeCustodian(t)
	m.cfg.Runner.Command = []string{"/no/such/custodian"}
	m = policyCursor(t, m, "ec2-mark-stop")
	m, cmd := send(m, key1('d'))
	m = settle(t, m, cmd)
	if !m.statusErr || !strings.Contains(m.status, "could not start custodian") {
		t.Fatalf("status %q", m.status)
	}
	runs, _ := m.store.List()
	if len(runs) != 1 || runs[0].Error == "" || !runs[0].Finished {
		t.Fatalf("failed run not recorded: %+v", runs)
	}
}

func TestVersion(t *testing.T) {
	m := withFakeCustodian(t)
	m = settle(t, m, loadVersion(m.cfgRunnerArgv()))
	if m.version != "0.9.52" || !strings.Contains(plain(m), "c7n 0.9.52") {
		t.Fatalf("version %q", m.version)
	}
}

func TestQuitWithRunningJobAsksFirst(t *testing.T) {
	m := withFakeCustodian(t)
	m.jobs.list = []jobView{{id: 1, title: "dry-run x", started: true}}
	m, cmd := send(m, key1('q'))
	if cmd != nil || !strings.Contains(m.status, "press q again") {
		t.Fatalf("first q: cmd=%v status=%q", cmd, m.status)
	}
	_, cmd = send(m, key1('q'))
	if cmd == nil {
		t.Fatal("second q did not quit")
	}
}

func fakeArgs(t *testing.T, j jobView) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(j.run.Dir, "argv.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(data), "\n")
}

func TestLiveRunEndToEnd(t *testing.T) {
	m := policyCursor(t, withFakeCustodian(t), "ec2-mark-stop")

	// Without the name nothing starts.
	m, _ = send(m, seq(key1('R'), typed("ec2-mark"), enter)...)
	if len(m.jobs.list) != 0 || !m.gate.open() {
		t.Fatalf("jobs=%d gate=%v", len(m.jobs.list), m.gate.open())
	}
	m, cmd := send(m, seq(typed("ec2-mark-stop"), enter)...)
	m = settle(t, m, cmd)

	j := m.jobs.list[0]
	if j.kind != "live" || !j.done || j.result.ExitCode != 0 || j.run.Kind != "live" {
		t.Fatalf("job = %+v", j)
	}
	args := fakeArgs(t, j)
	if slices.Contains(args, "--dryrun") || !slices.Contains(args, "ec2-mark-stop") || args[0] != "run" {
		t.Fatalf("live custodian args %q", args)
	}
	runs, _ := m.store.List()
	if len(runs) != 1 || runs[0].Kind != "live" || !runs[0].Finished {
		t.Fatalf("history = %+v", runs)
	}
}

func TestLiveRunNonPullNeedsDeploy(t *testing.T) {
	m := policyCursor(t, withFakeCustodian(t), "s3-periodic")
	m, _ = send(m, seq(key1('R'), typed("s3-periodic"), enter)...)
	if !m.gate.open() || m.gate.step != gateDeploy || len(m.jobs.list) != 0 {
		t.Fatalf("step=%v jobs=%d", m.gate.step, len(m.jobs.list))
	}
	m, cmd := send(m, seq(typed("DEPLOY"), enter)...)
	m = settle(t, m, cmd)
	if len(m.jobs.list) != 1 || slices.Contains(fakeArgs(t, m.jobs.list[0]), "--dryrun") {
		t.Fatalf("jobs = %+v", m.jobs.list)
	}
}
