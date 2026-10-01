package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
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
	useFakeCustodian(t, &m)
	m, cmd := send(m, enter) // ec2-mark-stop is first in the table
	if m.Screen() != ScreenResources || cmd == nil {
		t.Fatalf("screen=%v cmd=%v", m.Screen(), cmd)
	}
	m = settle(t, m, cmd)
	out = plain(m)
	// A table with c7n's report columns, and a card of the first resource.
	for _, want := range []string{"ec2-mark-stop · us-east-1 · aws.ec2 · 2", "InstanceId", "InstanceType", "t3.micro", "i-", "vpc-default-us-east-1", "t json"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if m.res.report == nil || len(m.res.report.Rows) != 2 {
		t.Fatalf("report = %+v (err %q)", m.res.report, m.res.reportErr)
	}
	m, _ = send(m, press('t', "t"))
	if out := plain(m); !strings.Contains(out, `"Architecture": "x86_64"`) || !strings.Contains(out, "t card") {
		t.Errorf("no JSON view:\n%s", out)
	}
	m, _ = send(m, press('t', "t"))

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

func TestResourcesWithoutReport(t *testing.T) {
	m := loaded(t)
	m.cfg.Runner.Custodian = "/no/such/custodian"
	m, _ = send(m, press('2', "2"))
	for i, r := range m.runs.list {
		if len(r.Policies) == 4 {
			m.runs.cursor = i
		}
	}
	m, _ = send(m, enter)
	m, cmd := send(m, enter)
	m = settle(t, m, cmd)
	if m.res.reportErr == "" {
		t.Fatal("report did not fail")
	}
	out := plain(m)
	for _, want := range []string{"no report", "id", "tags", "i-", "Architecture", "x86_64"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunSummary(t *testing.T) {
	m := loaded(t)
	m, _ = send(m, press('2', "2"))
	for i, r := range m.runs.list {
		if len(r.Policies) == 4 {
			m.runs.cursor = i
		}
	}
	out := plain(m)
	for _, want := range []string{
		"4 matches in 3 of 4 policies", "ec2 2", "s3 2", "us-east-1",
		"changing actions would hit 3 matches: mark-for-op 2 · tag 1",
		"ACTIONS", "mark-for-op", "report",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "DESTRUCTIVE") {
		t.Errorf("destructive warning without destructive actions:\n%s", out)
	}

	for i, r := range m.runs.list {
		if len(r.Policies) == 1 {
			m.runs.cursor = i
		}
	}
	if out := plain(m); !strings.Contains(out, "deployed as Lambda: s3-periodic") {
		t.Errorf("no deployed line:\n%s", out)
	}
}

func TestRunSummaryDestructive(t *testing.T) {
	m := loaded(t)
	m.runs.list = []runEntry{{ID: "x", Kind: "live", Finished: true, Policies: []c7n.PolicyRun{
		{Policy: "ec2-kill", Region: "us-east-1", Resource: "aws.ec2", ResourceCount: 3, Actions: []string{"terminate"}},
	}}}
	m.runs.cursor = 0
	m, _ = send(m, press('2', "2"))
	out := plain(m)
	for _, want := range []string{"⚠ DESTRUCTIVE actions hit 3 matches: terminate 3", "⚠"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// Thousands of resources must keep the screen responsive: one full render
// (table + card) well under a frame budget on a slow CI machine.
func TestResourcesScreenScales(t *testing.T) {
	m := loaded(t)
	const n = c7n.MaxResources
	rep := &c7n.Report{Columns: []string{"CustodianDate", "InstanceId", "InstanceType"}}
	for i := range n {
		id := fmt.Sprintf("i-%06d", i)
		m.res.list = append(m.res.list, c7n.Resource{ID: id, Raw: []byte(`{"InstanceId":"` + id + `"}`)})
		rep.Rows = append(rep.Rows, []string{"2026-10-01 00:00:00.000000", id, "t3.micro"})
	}
	m.res.pr = c7n.PolicyRun{Policy: "p", Dir: "/x", Resource: "aws.ec2"}
	m.res.total, m.res.report = 25000, rep
	m.res.indexReport()
	m.screen = ScreenResources

	start := time.Now()
	out := plain(m)
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("render took %v", d)
	}
	if !strings.Contains(out, "showing the first 10000 of 25000") || !strings.Contains(out, "i-000000") {
		t.Fatalf("view:\n%s", out)
	}
	t.Logf("render of %d resources: %v", n, time.Since(start))
}
