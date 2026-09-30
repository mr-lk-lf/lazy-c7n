package app

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestSchemaBrowser(t *testing.T) {
	m := withFakeCustodian(t)
	m.version = "0.9.52"
	m, cmd := send(m, key1('4'))
	if !m.schema.loading || cmd == nil {
		t.Fatal("schema not loading")
	}
	if out := plain(m); !strings.Contains(out, "custodian schema --json") {
		t.Errorf("no loading hint:\n%s", out)
	}
	m = settle(t, m, cmd)
	out := plain(m)
	for _, want := range []string{"Resource types · 3", "aws.ebs", "aws.ec2", "aws.s3", "enter to browse"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Cached for this version.
	if _, err := os.Stat(m.store.SchemaCachePath("0.9.52")); err != nil {
		t.Errorf("schema not cached: %v", err)
	}

	// Into aws.ec2, filter to "stop", load its help.
	m, _ = send(m, key1('j'), enter)
	if m.schema.resource != "aws.ec2" {
		t.Fatalf("resource %q", m.schema.resource)
	}
	m, _ = send(m, seq(key1('/'), typed("action stop"), enter)...)
	it, ok := m.currentSchemaItem()
	if !ok || it.name != "stop" || it.category != "actions" {
		t.Fatalf("item %+v", it)
	}
	if out := plain(m); !strings.Contains(out, "(destructive)") || !strings.Contains(out, `"hibernate"`) {
		t.Errorf("action detail:\n%s", out)
	}
	m, cmd = send(m, enter)
	m = settle(t, m, cmd)
	if out := plain(m); !strings.Contains(out, "Docs for aws.ec2.actions.stop") {
		t.Errorf("no help:\n%s", out)
	}

	// esc clears the filter, then goes back to the types.
	m, _ = send(m, esc, esc)
	if m.schema.resource != "" {
		t.Fatalf("still in %s", m.schema.resource)
	}
}

func TestEditorCommand(t *testing.T) {
	cases := []struct {
		visual, editor string
		want           []string
	}{
		{"", "vim", []string{"vim", "+12", "p.yml"}},
		{"nvim", "vim", []string{"nvim", "+12", "p.yml"}},
		{"", "code -w", []string{"code", "-w", "-g", "p.yml:12"}},
		{"", "/usr/bin/hx", []string{"/usr/bin/hx", "p.yml:12"}},
	}
	for _, c := range cases {
		cmd, err := editorCommand(c.visual, c.editor, "p.yml", 12)
		if err != nil || !slices.Equal(cmd.Args, c.want) {
			t.Errorf("%q/%q: %q (err %v)", c.visual, c.editor, cmd.Args, err)
		}
	}
}
