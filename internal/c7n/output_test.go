package c7n

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOutputDirSingleRegion(t *testing.T) {
	runs, err := ReadOutputDir(filepath.Join(realFixtures, "dryrun", "out"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, r := range runs {
		got[r.Policy] = r.ResourceCount
		if r.Region != "us-east-1" || !r.DryRun || r.Version != "0.9.52" || r.Status() != "ok" {
			t.Errorf("%s: %+v", r.Policy, r)
		}
		if r.Start.IsZero() || r.End.Before(r.Start) {
			t.Errorf("%s: times %v %v", r.Policy, r.Start, r.End)
		}
	}
	want := map[string]int{"ec2-mark-stop": 2, "ec2-none-match": 0, "s3-periodic": 1, "s3-untagged-owner": 1}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %d resources, want %d", k, got[k], v)
		}
	}
}

func TestReadOutputDirMultiRegion(t *testing.T) {
	runs, err := ReadOutputDir(filepath.Join(realFixtures, "dryrun-multiregion", "out"))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, r := range runs {
		keys = append(keys, r.Policy+"@"+r.Region)
	}
	if strings.Join(keys, " ") != "ec2-mark-stop@eu-west-1 ec2-mark-stop@us-east-1 ec2-none-match@eu-west-1 ec2-none-match@us-east-1" {
		t.Fatalf("got %v", keys)
	}
}

func TestReadOutputDirStatuses(t *testing.T) {
	runs, err := ReadOutputDir(filepath.Join(realFixtures, "dryrun-api-error", "out"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	if r := runs[0]; r.ResourceCount != -1 || r.Status() != "error" {
		t.Errorf("api error run: count=%d status=%s", r.ResourceCount, r.Status())
	}

	runs, err = ReadOutputDir(filepath.Join(realFixtures, "live-periodic", "out"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	if r := runs[0]; r.Status() != "deployed" || r.DryRun || r.APICalls["lambda.CreateFunction"] != 1 {
		t.Errorf("periodic live run: %+v status=%s", r, r.Status())
	}

	// moto rejects the Lambda role: the same scenario is an error there.
	runs, _ = ReadOutputDir("../../tests/fixtures/real/c7n-0.9.52-moto/live-periodic/out")
	if len(runs) != 1 || runs[0].Status() != "error" {
		t.Errorf("moto periodic live run: %+v", runs)
	}
}

func TestReadOutputDirEmpty(t *testing.T) {
	if _, err := ReadOutputDir(t.TempDir()); !errors.Is(err, ErrNoOutput) {
		t.Fatalf("err = %v", err)
	}
	if _, err := ReadOutputDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing dir: no error")
	}
}

func TestReadResources(t *testing.T) {
	res, err := ReadResources(filepath.Join(realFixtures, "dryrun", "out", "ec2-mark-stop", "resources.json"), "aws.ec2")
	if err != nil || len(res) != 2 {
		t.Fatalf("res=%d err=%v", len(res), err)
	}
	if !strings.HasPrefix(res[0].ID, "i-") {
		t.Errorf("id %q", res[0].ID)
	}
	if !strings.Contains(res[0].Pretty(), "\n  \"InstanceId\"") {
		t.Errorf("pretty:\n%s", res[0].Pretty())
	}

	res, err = ReadResources(filepath.Join(realFixtures, "dryrun", "out", "s3-untagged-owner", "resources.json"), "s3")
	if err != nil || len(res) != 1 || res[0].ID != "lc7n-public-logs" {
		t.Fatalf("s3: %+v err=%v", res, err)
	}
}

func TestResourceIDFallbacks(t *testing.T) {
	cases := []struct {
		typ    string
		fields map[string]any
		want   string
	}{
		{"azure.vm", map[string]any{"id": "/subscriptions/x/vm1", "name": "vm1"}, "/subscriptions/x/vm1"},
		{"gcp.instance", map[string]any{"name": "inst-1", "selfLink": "https://..."}, "inst-1"},
		{"aws.unknown", map[string]any{"ZoneName": "z", "WidgetId": "w-1"}, "w-1"},
		{"aws.ec2", map[string]any{"Name": "n"}, "n"},
		{"", nil, "?"},
	}
	for _, c := range cases {
		if got := resourceID(c.typ, c.fields); got != c.want {
			t.Errorf("%s %v: %q, want %q", c.typ, c.fields, got, c.want)
		}
	}
}

func TestResourceTags(t *testing.T) {
	tags := resourceTags(map[string]any{
		"Tags":   []any{map[string]any{"Key": "owner", "Value": "a"}},
		"labels": map[string]any{"env": "dev"},
	})
	if len(tags) != 2 || tags[0] != (Tag{"env", "dev"}) || tags[1] != (Tag{"owner", "a"}) {
		t.Fatalf("tags = %v", tags)
	}
}
