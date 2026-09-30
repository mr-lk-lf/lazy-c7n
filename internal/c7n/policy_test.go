package c7n

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const realFixtures = "../../tests/fixtures/real/c7n-0.9.52-floci"

func TestParseRealPolicyFile(t *testing.T) {
	f, ok := ReadPolicyFile(filepath.Join(realFixtures, "policies.yml"))
	if !ok || f.Err != nil {
		t.Fatalf("ok=%v err=%v", ok, f.Err)
	}
	var names []string
	for _, p := range f.Policies {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"s3-untagged-owner", "ec2-none-match", "ec2-mark-stop", "s3-periodic"}) {
		t.Fatalf("names = %v", names)
	}
	p := f.Policies[0]
	if p.Resource != "aws.s3" || !slices.Equal(p.Actions, []string{"tag"}) || !slices.Equal(p.Filters, []string{"tag:owner: absent"}) {
		t.Fatalf("policy 0 = %+v", p)
	}
	if p.Line != 2 || p.EndLine != 9 {
		t.Fatalf("lines %d-%d", p.Line, p.EndLine)
	}
	if got := f.PolicyText(p); len(got) != 8 || !strings.Contains(got[0], "name: s3-untagged-owner") {
		t.Fatalf("text = %q", got)
	}
	if f.Policies[3].Mode != "periodic" || f.Policies[3].IsPull() {
		t.Fatalf("policy 3 = %+v", f.Policies[3])
	}
	if f.Policies[1].Actions != nil {
		t.Fatalf("report-only policy has actions %v", f.Policies[1].Actions)
	}
}

func TestParseIsLenient(t *testing.T) {
	src := `# comment
vars:
  owner-filter: &owner
    "tag:owner": absent
policies:
  - name: a
    resource: ec2
    unknown-key: {whatever: 1}
    description: |
      Stops things.
    filters:
      - *owner
      - type: value
        key: State.Name
        value: running
      - or:
          - "tag:a": present
          - "tag:b": present
    actions:
      - stop
      - {type: notify, to: [x]}
      - type: delete


  - name: b
    resource: aws.s3
    mode: {type: cloudtrail, events: []}
`
	f, ok := ParsePolicies("x.yml", []byte(src))
	if !ok || f.Err != nil || len(f.Policies) != 2 {
		t.Fatalf("ok=%v err=%v n=%d", ok, f.Err, len(f.Policies))
	}
	a := f.Policies[0]
	if a.Description != "Stops things." {
		t.Errorf("description %q", a.Description)
	}
	if want := []string{"tag:owner: absent", "value State.Name = running", "or (2)"}; !slices.Equal(a.Filters, want) {
		t.Errorf("filters %q", a.Filters)
	}
	if want := []string{"stop", "notify", "delete"}; !slices.Equal(a.Actions, want) {
		t.Errorf("actions %q", a.Actions)
	}
	if !slices.Equal(a.DestructiveActions(), []string{"stop", "delete"}) {
		t.Errorf("destructive %q", a.DestructiveActions())
	}
	if a.EndLine != 22 {
		t.Errorf("a ends at %d (trailing blank lines must not count)", a.EndLine)
	}
	if f.Policies[1].Mode != "cloudtrail" || f.Policies[1].Line != 25 {
		t.Errorf("b = %+v", f.Policies[1])
	}
}

func TestNotPolicyFiles(t *testing.T) {
	for _, src := range []string{"", "foo: bar\n", "- a\n- b\n", "key: [unclosed\n"} {
		if _, ok := ParsePolicies("x.yml", []byte(src)); ok {
			t.Errorf("%q taken as a policy file", src)
		}
	}
	f, ok := ParsePolicies("x.yml", []byte("policies:\n  - name: [broken\n"))
	if !ok || f.Err == nil {
		t.Errorf("broken policy file not reported: ok=%v err=%v", ok, f.Err)
	}
	f, ok = ParsePolicies("x.yml", []byte("policies: 3\n"))
	if !ok || f.Err == nil {
		t.Errorf("non-list policies not reported")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pol := "policies:\n  - name: x\n    resource: aws.s3\n"
	write("a.yml", pol)
	write("sub/b.yaml", pol)
	write("sub/deeper/c.YML", pol)
	write("other.yml", "foo: bar\n")
	write(".git/hidden.yml", pol)
	write("node_modules/n.yml", pol)
	write("notes.txt", pol)

	files := Discover([]string{dir, filepath.Join(dir, "a.yml"), filepath.Join(dir, "missing")})
	var got []string
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f.Path)
		if f.Err != nil {
			rel += " (err)"
		}
		got = append(got, filepath.ToSlash(rel))
	}
	want := []string{"a.yml", "missing (err)", "sub/b.yaml", "sub/deeper/c.YML"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
