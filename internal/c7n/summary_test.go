package c7n

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestSummarizeRealDryRun(t *testing.T) {
	runs, err := ReadOutputDir(filepath.Join(realFixtures, "dryrun", "out"))
	if err != nil {
		t.Fatal(err)
	}
	s := Summarize(runs)
	// s3-untagged-owner 1 (tag), ec2-none-match 0, ec2-mark-stop 2
	// (mark-for-op), s3-periodic 1 (no actions).
	if s.Policies != 4 || s.WithMatches != 3 || s.Resources != 4 || !s.DryRun {
		t.Fatalf("summary %+v", s)
	}
	if s.MutatingResources != 3 || s.DestructiveResources != 0 {
		t.Fatalf("mutating %d destructive %d", s.MutatingResources, s.DestructiveResources)
	}
	if !slices.Equal(s.MutatingActions, []Count{{"mark-for-op", 2}, {"tag", 1}}) {
		t.Fatalf("mutating actions %v", s.MutatingActions)
	}
	if !slices.Equal(s.ByType, []Count{{"aws.ec2", 2}, {"aws.s3", 2}}) {
		t.Fatalf("by type %v", s.ByType)
	}
	if !slices.Equal(s.Regions, []string{"us-east-1"}) || len(s.Errors) != 0 {
		t.Fatalf("regions %v errors %v", s.Regions, s.Errors)
	}
}

func TestSummarizeDestructiveErrorsDeployed(t *testing.T) {
	runs := []PolicyRun{
		{Policy: "ec2-kill", Resource: "aws.ec2", ResourceCount: 3, Actions: []string{"notify", "terminate"}, DryRun: true},
		{Policy: "ebs-del", Resource: "aws.ebs", ResourceCount: 2, Actions: []string{"tag", "delete"}, DryRun: true},
		{Policy: "broken", Resource: "aws.s3", ResourceCount: -1, DryRun: true},
	}
	s := Summarize(runs)
	if s.DestructiveResources != 5 || s.MutatingResources != 0 {
		t.Fatalf("destructive %d mutating %d", s.DestructiveResources, s.MutatingResources)
	}
	if !slices.Equal(s.DestructiveActions, []Count{{"terminate", 3}, {"delete", 2}}) {
		t.Fatalf("destructive actions %v", s.DestructiveActions)
	}
	if !slices.Equal(s.Errors, []string{"broken"}) {
		t.Fatalf("errors %v", s.Errors)
	}

	live, _ := ReadOutputDir(filepath.Join(realFixtures, "live-periodic", "out"))
	if s := Summarize(live); !slices.Equal(s.Deployed, []string{"s3-periodic"}) || s.DryRun {
		t.Fatalf("live periodic: %+v", s)
	}
	if s := Summarize(nil); s.DryRun || s.Policies != 0 {
		t.Fatalf("empty: %+v", s)
	}
}

func TestSpecActions(t *testing.T) {
	got := specActions([]byte(`{"actions": ["delete", {"type": "tag", "key": "a"}, {"nope": 1}]}`))
	if !slices.Equal(got, []string{"delete", "tag", "?"}) {
		t.Fatalf("got %v", got)
	}
}
