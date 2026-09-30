package c7n

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParseSchema(t *testing.T) {
	data, err := os.ReadFile("../../tests/fixtures/real/c7n-0.9.52-schema-small.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseSchema(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ResourceTypes(); !slices.Equal(got, []string{"aws.ebs", "aws.ec2", "aws.s3"}) {
		t.Fatalf("types %v", got)
	}
	ec2 := s.Resources["aws.ec2"]
	if !slices.Contains(Names(ec2.Actions), "stop") || !slices.Contains(Names(ec2.Filters), "instance-age") {
		t.Fatalf("ec2 actions %v filters %v", Names(ec2.Actions), Names(ec2.Filters))
	}
	if !strings.Contains(PrettyJSON(ec2.Actions["stop"]), `"hibernate"`) {
		t.Errorf("stop schema:\n%s", PrettyJSON(ec2.Actions["stop"]))
	}
	for _, mode := range []string{"pull", "periodic", "cloudtrail"} {
		if !slices.Contains(s.Modes, mode) {
			t.Errorf("mode %s missing from %v", mode, s.Modes)
		}
	}
	if _, err := ParseSchema([]byte(`{"definitions": {}}`)); err == nil {
		t.Error("empty schema accepted")
	}
}
