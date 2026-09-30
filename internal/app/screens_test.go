package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/config"
)

const fixtures = "../../tests/fixtures/real/c7n-0.9.52-floci"

// loaded returns a model with the fixture policies and output dirs loaded,
// running the load commands the way the Bubble Tea runtime would.
func loaded(t *testing.T) Model {
	t.Helper()
	cfg := config.Default()
	cfg.StateDir = t.TempDir()
	m := New(cfg, Options{
		PolicyPaths: []string{fixtures + "/policies.yml"},
		OutputDirs:  []string{fixtures + "/dryrun/out", fixtures + "/live-periodic/out"},
	})
	m, _ = send(m,
		tea.WindowSizeMsg{Width: 120, Height: 30},
		loadPolicies(m.policyPaths())(),
		m.reloadRuns()(),
	)
	return m
}

func TestPoliciesScreen(t *testing.T) {
	m := loaded(t)
	out := plain(m)
	for _, want := range []string{"policies.yml", "s3-untagged-owner", "ec2-mark-stop", "s3-periodic", "periodic", "report"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// Cursor on the first policy shows its summary and YAML.
	m, _ = send(m, press('j', "j"))
	out = plain(m)
	for _, want := range []string{"aws.s3 · mode pull · ", "policies.yml:2", "filters tag:owner: absent", "actions tag", "2   - name: s3-untagged-owner"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// The periodic policy warns about deploying.
	m, _ = send(m, press('G', "G"))
	if out := plain(m); !strings.Contains(out, "a live run deploys a Lambda") {
		t.Errorf("no deploy warning:\n%s", out)
	}
}

func TestPolicySelectionAndFolding(t *testing.T) {
	m := loaded(t)
	space := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}

	// Nothing marked: the cursor decides. On the file row, that is every policy.
	if n := len(m.selectedPolicies()); n != 4 {
		t.Fatalf("file row selects %d policies", n)
	}
	m, _ = send(m, press('j', "j"))
	if sel := m.selectedPolicies(); len(sel) != 1 || sel[0].Name != "s3-untagged-owner" {
		t.Fatalf("cursor selection = %v", sel)
	}

	// space marks and moves down; marked policies win over the cursor.
	m, _ = send(m, space, press('j', "j"), space)
	sel := m.selectedPolicies()
	if len(sel) != 2 || sel[0].Name != "s3-untagged-owner" || sel[1].Name != "ec2-mark-stop" {
		t.Fatalf("marked = %v", names(sel))
	}
	if out := plain(m); !strings.Contains(out, "2 selected") {
		t.Errorf("no selected count:\n%s", out)
	}
	m, _ = send(m, esc)
	if m.markedCount() != 0 {
		t.Fatal("esc did not clear the selection")
	}

	// enter on the file row folds it.
	m, _ = send(m, press('g', "g"), enter)
	if len(m.policyRows()) != 1 || !strings.Contains(plain(m), "(4)") {
		t.Fatalf("not folded: %v", m.policyRows())
	}
}

func TestPolicyFilter(t *testing.T) {
	m := loaded(t)
	m, _ = send(m, seq(press('/', "/"), typed("ec2-mark"), enter)...)
	rows := m.policyRows()
	if len(rows) != 2 || m.policies.files[0].Policies[rows[1].policy].Name != "ec2-mark-stop" {
		t.Fatalf("rows = %v", rows)
	}
	if m.filtering {
		t.Fatal("still typing the filter after enter")
	}
	m, _ = send(m, esc)
	if len(m.policyRows()) != 5 {
		t.Fatalf("esc did not clear the filter: %v", m.policyRows())
	}
}

func TestRunsAndResources(t *testing.T) {
	m := loaded(t)
	m, _ = send(m, press('2', "2"))
	out := plain(m)
	for _, want := range []string{"DIR", "4 pol · 4 res", "1 pol · 0 res"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// Pick the dry-run dir (4 policies) and open ec2-mark-stop's resources.
	for i, r := range m.runs.list {
		if len(r.Policies) == 4 {
			m.runs.cursor = i
		}
	}
	m, _ = send(m, enter)
	out = plain(m)
	for _, want := range []string{"ec2-mark-stop", "us-east-1", "ok", "POLICY"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	m, cmd := send(m, enter) // ec2-mark-stop is first in the table
	if m.Screen() != ScreenResources || cmd == nil {
		t.Fatalf("screen=%v cmd=%v", m.Screen(), cmd)
	}
	m, _ = send(m, cmd())
	out = plain(m)
	for _, want := range []string{"ec2-mark-stop · us-east-1 · 2", "i-", `"Architecture": "x86_64"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// esc goes back to the run.
	m, _ = send(m, esc)
	if m.Screen() != ScreenRuns {
		t.Fatalf("screen = %v", m.Screen())
	}
}

func TestRunLogView(t *testing.T) {
	m := loaded(t)
	m, _ = send(m, press('2', "2"))
	m, cmd := send(m, press('t', "t"))
	if cmd == nil {
		t.Fatal("t did not load the log")
	}
	m, _ = send(m, cmd())
	if out := plain(m); !strings.Contains(out, "custodian") || !strings.Contains(out, "Log ·") {
		t.Fatalf("log not shown:\n%s", out)
	}
}

func TestNoPoliciesFound(t *testing.T) {
	m := New(config.Default(), Options{PolicyPaths: []string{t.TempDir()}})
	m, _ = send(m, loadPolicies(m.policyPaths())())
	if out := plain(m); !strings.Contains(out, "no policy files found") {
		t.Fatalf("no hint:\n%s", out)
	}
}
