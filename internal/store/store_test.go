package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCreateSaveList(t *testing.T) {
	s := Store{Root: t.TempDir()}
	t0 := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	a, err := s.Create("dry-run", t0)
	if err != nil {
		t.Fatal(err)
	}
	a.Argv = []string{"env", "AWS_SECRET_ACCESS_KEY=abc", "custodian", "run"}
	a.Policies = []string{"p1"}
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Create("live", t0.Add(time.Minute))
	b.Finished, b.ExitCode = true, 2
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("validate", t0.Add(2*time.Minute)); err != nil { // never saved: ignored
		t.Fatal(err)
	}

	runs, err := s.List()
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	if runs[0].ID != b.ID || runs[0].ExitCode != 2 || !runs[0].Finished {
		t.Errorf("newest first: %+v", runs[0])
	}
	if runs[1].Argv[1] != "AWS_SECRET_ACCESS_KEY=<redacted>" {
		t.Errorf("secret stored: %v", runs[1].Argv)
	}
	data, _ := os.ReadFile(filepath.Join(a.Dir, "run.json"))
	if strings.Contains(string(data), "abc") {
		t.Errorf("secret in run.json:\n%s", data)
	}
	if runs[1].OutDir() != filepath.Join(a.Dir, "out") {
		t.Errorf("out dir %s", runs[1].OutDir())
	}
}

func TestListWithoutState(t *testing.T) {
	runs, err := Store{Root: filepath.Join(t.TempDir(), "nope")}.List()
	if err != nil || runs != nil {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
}

func TestPrune(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	var ids []string
	for i := range 5 {
		r, _ := s.Create("dry-run", now.Add(-time.Duration(i)*24*time.Hour))
		r.Finished = i != 4 // the oldest is still "running": never pruned
		if err := s.Save(r); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.ID)
	}

	removed, err := s.Prune(2, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(removed, []string{ids[2], ids[3]}) {
		t.Fatalf("removed %v, want %v", removed, ids[2:4])
	}
	removed, _ = s.Prune(0, 12*time.Hour, now)
	if !slices.Equal(removed, []string{ids[1]}) {
		t.Fatalf("by age removed %v", removed)
	}
	runs, _ := s.List()
	if len(runs) != 2 {
		t.Fatalf("left %d runs", len(runs))
	}
}

func TestRedactArgv(t *testing.T) {
	got := RedactArgv([]string{"custodian", "--profile=prod", "MY_TOKEN=x", "a=b", "Password=p"})
	want := []string{"custodian", "--profile=prod", "MY_TOKEN=<redacted>", "a=b", "Password=<redacted>"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}
