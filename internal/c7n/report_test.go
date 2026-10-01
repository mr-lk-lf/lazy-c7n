package c7n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseRealReportCSV(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(realFixtures, "report-csv", "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseReportCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"CustodianDate", "InstanceId", "tag:Name", "InstanceType", "LaunchTime", "VpcId", "PrivateIpAddress"}
	if !slices.Equal(r.Columns, want) || len(r.Rows) != 2 {
		t.Fatalf("columns %v rows %d", r.Columns, len(r.Rows))
	}

	res, _ := ReadResources(filepath.Join(realFixtures, "dryrun", "out", "ec2-mark-stop", "resources.json"), "aws.ec2")
	// Rows are matched by id, whatever their order.
	row, ok := r.RowFor(res[1], 0)
	if !ok || row[1] != res[1].ID {
		t.Fatalf("row for %s = %v", res[1].ID, row)
	}
	if _, ok := r.RowFor(Resource{ID: "nope"}, 9); ok {
		t.Fatal("found a row for an unknown resource")
	}
	if _, err := ParseReportCSV(nil); err == nil {
		t.Fatal("empty report accepted")
	}
}

func TestWriteReportPolicy(t *testing.T) {
	runs, err := ReadOutputDir(filepath.Join(realFixtures, "dryrun", "out"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := WriteReportPolicy(runs[0], t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var f struct {
		Policies []struct{ Name, Resource string } `json:"policies"`
	}
	if err := json.Unmarshal(data, &f); err != nil || len(f.Policies) != 1 || f.Policies[0].Name != runs[0].Policy {
		t.Fatalf("policy file %s: %v", data, err)
	}
	if _, err := WriteReportPolicy(PolicyRun{}, t.TempDir()); err == nil {
		t.Fatal("no spec accepted")
	}
}
