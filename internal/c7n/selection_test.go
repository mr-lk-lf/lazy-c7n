package c7n

import (
	"slices"
	"strings"
	"testing"
)

func TestSelect(t *testing.T) {
	a := PolicyFile{Path: "a.yml", Policies: []Policy{
		{Name: "s3-tag", File: "a.yml", Line: 2},
		{Name: "s3-tag-all", File: "a.yml", Line: 8},
		{Name: "ec2-stop", File: "a.yml", Line: 14},
	}}
	b := PolicyFile{Path: "b.yml", Policies: []Policy{
		{Name: "ec2-stop", File: "b.yml", Line: 2},
		{Name: "rds-x", File: "b.yml", Line: 9},
	}}
	dup := PolicyFile{Path: "dup.yml", Policies: []Policy{
		{Name: "x", File: "dup.yml", Line: 2},
		{Name: "x", File: "dup.yml", Line: 5},
	}}
	glob := PolicyFile{Path: "g.yml", Policies: []Policy{
		{Name: "s3-*", File: "g.yml", Line: 2},
		{Name: "s3-a", File: "g.yml", Line: 5},
	}}
	all := []PolicyFile{a, b, dup, glob}

	sel, err := Select(all, []Policy{a.Policies[0], a.Policies[1]})
	if err != nil || !slices.Equal(sel.Files, []string{"a.yml"}) || !slices.Equal(sel.Names, []string{"s3-tag", "s3-tag-all"}) {
		t.Fatalf("sel=%+v err=%v", sel, err)
	}
	if _, err := Select(all, []Policy{a.Policies[2], b.Policies[1]}); err == nil || !strings.Contains(err.Error(), "ec2-stop (b.yml:2)") {
		t.Fatalf("name repeated across files not refused: %v", err)
	}
	if _, err := Select(all, []Policy{a.Policies[2]}); err != nil {
		t.Fatalf("b.yml is not passed, so its ec2-stop does not run: %v", err)
	}
	if _, err := Select(all, []Policy{dup.Policies[0]}); err == nil {
		t.Fatal("duplicate name in one file not refused")
	}
	if _, err := Select(all, []Policy{glob.Policies[0]}); err == nil || !strings.Contains(err.Error(), "s3-a") {
		t.Fatalf("glob-like name not refused: %v", err)
	}
	if _, err := Select(all, nil); err == nil {
		t.Fatal("empty selection accepted")
	}
}
