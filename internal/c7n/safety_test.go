package c7n

import "testing"

func TestClassifyAction(t *testing.T) {
	cases := map[string]ActionClass{
		"notify":                 ActionNotify,
		"post-finding":           ActionNotify,
		"no-op":                  ActionNotify,
		"tag":                    ActionMutating,
		"mark-for-op":            ActionMutating,
		"remove-tag":             ActionMutating,
		"untag":                  ActionMutating,
		"modify-security-groups": ActionMutating,
		"set-public-block":       ActionMutating,
		"invoke-lambda":          ActionMutating,
		"terminate":              ActionDestructive,
		"delete":                 ActionDestructive,
		"stop":                   ActionDestructive,
		"delete-inline-policies": ActionDestructive,
		"remove-permissions":     ActionDestructive,
		"revoke-access":          ActionDestructive,
		" DELETE ":               ActionDestructive,
		// Unknown types fail closed: never shown as harmless.
		"some-future-action": ActionMutating,
		"":                   ActionMutating,
	}
	for in, want := range cases {
		if got := ClassifyAction(in); got != want {
			t.Errorf("ClassifyAction(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsPull(t *testing.T) {
	cases := map[string]bool{
		"":            true,
		"pull":        true,
		"periodic":    false,
		"cloudtrail":  false,
		"config-rule": false,
		"Pull":        false, // unknown spelling fails closed
		"brand-new":   false,
	}
	for mode, want := range cases {
		if got := (Policy{Mode: mode}).IsPull(); got != want {
			t.Errorf("IsPull(%q) = %v, want %v", mode, got, want)
		}
	}
}

func TestDestructiveActions(t *testing.T) {
	p := Policy{Actions: []string{"tag", "stop", "notify", "delete"}}
	got := p.DestructiveActions()
	if len(got) != 2 || got[0] != "stop" || got[1] != "delete" {
		t.Fatalf("got %v", got)
	}
}
