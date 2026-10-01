package c7n

import (
	"errors"
	"fmt"
	"strings"
)

// ActionClass says how much an action can hurt (SPEC §6.3).
type ActionClass int

const (
	// ActionNotify only reports (notify, post-finding, ...). Resources are not
	// changed.
	ActionNotify ActionClass = iota
	// ActionMutating changes resources in a way that is usually reversible
	// (tags, settings). Also used for every action type we do not know.
	ActionMutating
	// ActionDestructive can lose data or stop service (delete, terminate,
	// stop, revoke, ...).
	ActionDestructive
)

func (c ActionClass) String() string {
	switch c {
	case ActionNotify:
		return "notify"
	case ActionMutating:
		return "mutating"
	case ActionDestructive:
		return "destructive"
	}
	return "?"
}

// The lists below were built from the action names of c7n 0.9.52 (AWS).
// Anything not listed is mutating, so a new or unknown action is never shown
// as harmless.

var notifyActions = map[string]bool{
	"notify":       true,
	"post-finding": true,
	"post-item":    true,
	"put-metric":   true,
	"webhook":      true,
	"no-op":        true,
}

var destructiveActions = map[string]bool{
	"terminate":         true,
	"delete":            true,
	"stop":              true,
	"detach":            true,
	"release":           true,
	"deregister":        true,
	"disable":           true,
	"disassociate":      true,
	"suspend":           true,
	"pause":             true,
	"reboot":            true,
	"cancel":            true,
	"revoke-access":     true,
	"schedule-deletion": true,
	"trim-versions":     true,
}

// Prefixes of destructive action families (delete-inline-policies,
// remove-permissions, ...).
var destructivePrefixes = []string{"delete-", "remove-"}

// Removing tags is not destructive even though the name starts with remove-.
var tagActions = map[string]bool{
	"remove-tag": true,
}

// ClassifyAction classifies a c7n action type such as "delete" or
// "mark-for-op". Case and surrounding spaces are ignored. Classes set in the
// config (SetActionOverrides) win over the built-in lists.
func ClassifyAction(actionType string) ActionClass {
	t := normalizeAction(actionType)
	if c, ok := overrides[t]; ok {
		return c
	}
	return builtinClass(t)
}

func normalizeAction(t string) string { return strings.ToLower(strings.TrimSpace(t)) }

func builtinClass(t string) ActionClass {
	switch {
	case notifyActions[t]:
		return ActionNotify
	case tagActions[t]:
		return ActionMutating
	case destructiveActions[t]:
		return ActionDestructive
	}
	for _, p := range destructivePrefixes {
		if strings.HasPrefix(t, p) {
			return ActionDestructive
		}
	}
	return ActionMutating
}

// overrides are the classes from the config ([safety.actions]). Set once
// at startup.
var overrides = map[string]ActionClass{}

// ActionOverrides lists action types per class, as written in the config.
type ActionOverrides struct {
	Destructive []string `toml:"destructive"`
	Mutating    []string `toml:"mutating"`
	Notify      []string `toml:"notify"`
}

// Check validates overrides: an action may appear in one list only, and a
// built-in destructive action can never be made less severe (that would
// hide the destructive warnings of the live-run gate).
func (o ActionOverrides) Check() error {
	_, err := o.resolve()
	return err
}

func (o ActionOverrides) resolve() (map[string]ActionClass, error) {
	out := map[string]ActionClass{}
	var errs []error
	add := func(list []string, class ActionClass) {
		for _, raw := range list {
			t := normalizeAction(raw)
			if t == "" {
				errs = append(errs, fmt.Errorf("safety.actions.%s: empty action name", class))
				continue
			}
			if prev, dup := out[t]; dup {
				errs = append(errs, fmt.Errorf("safety.actions: %q is listed as both %s and %s", t, prev, class))
				continue
			}
			if builtinClass(t) == ActionDestructive && class != ActionDestructive {
				errs = append(errs, fmt.Errorf("safety.actions.%s: %q is destructive and cannot be made less severe", class, t))
				continue
			}
			out[t] = class
		}
	}
	add(o.Destructive, ActionDestructive)
	add(o.Mutating, ActionMutating)
	add(o.Notify, ActionNotify)
	return out, errors.Join(errs...)
}

// SetActionOverrides installs the config's classes (after Check). Passing
// an empty value restores the built-in classification.
func SetActionOverrides(o ActionOverrides) error {
	resolved, err := o.resolve()
	if err != nil {
		return err
	}
	overrides = resolved
	return nil
}
