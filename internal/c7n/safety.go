package c7n

import "strings"

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
// "mark-for-op". Case and surrounding spaces are ignored.
func ClassifyAction(actionType string) ActionClass {
	t := strings.ToLower(strings.TrimSpace(actionType))
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
