package app

// Small line-based highlighters for the YAML and JSON views. They only
// colour what they recognise and never change the text itself.

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/ui"
)

// "  - key: value" / "key:" (quoted keys too).
var yamlKeyRe = regexp.MustCompile(`^(\s*(?:-\s+)?)("[^"]*"|'[^']*'|[^\s#:"'][^:#]*?)(:)(\s|$)(.*)$`)

// "  - value" (a plain list item).
var yamlItemRe = regexp.MustCompile(`^(\s*-\s+)(.*)$`)

// highlightYAML colours one line of policy YAML. Values that name one of
// actions are coloured by their safety class, so `- delete` stands out.
func highlightYAML(s ui.Styles, line string, actions map[string]bool) string {
	code, comment := splitComment(line)
	var out string
	if m := yamlKeyRe.FindStringSubmatch(code); m != nil {
		out = m[1] + s.Key.Render(m[2]) + m[3] + m[4] + yamlValue(s, m[5], m[2], actions)
	} else if m := yamlItemRe.FindStringSubmatch(code); m != nil {
		out = m[1] + yamlValue(s, m[2], "", actions)
	} else {
		out = code
	}
	if comment != "" {
		out += s.Comment.Render(comment)
	}
	return out
}

func yamlValue(s ui.Styles, v, key string, actions map[string]bool) string {
	t := strings.TrimSpace(v)
	if t == "" {
		return v
	}
	if actions[t] && (key == "" || key == "type") {
		switch c7n.ClassifyAction(t) {
		case c7n.ActionDestructive:
			return s.Danger.Render(v)
		case c7n.ActionMutating:
			return s.Warn.Render(v)
		case c7n.ActionNotify:
			return s.OK.Render(v)
		}
	}
	return scalarStyle(s, t).Render(v)
}

func scalarStyle(s ui.Styles, t string) lipgloss.Style {
	switch {
	case strings.HasPrefix(t, `"`), strings.HasPrefix(t, `'`):
		return s.String
	case t == "true", t == "false", t == "null", t == "~", isNumber(t):
		return s.Number
	}
	return s.Item
}

func isNumber(t string) bool {
	var f float64
	_, err := fmt.Sscan(t, &f)
	return err == nil && !strings.ContainsAny(t, " :")
}

// splitComment separates a trailing "# comment" that is not inside quotes.
func splitComment(line string) (code, comment string) {
	inSingle, inDouble := false, false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
				return line[:i], line[i:]
			}
		}
	}
	return line, ""
}

// `  "key": value,` as written by json.Indent.
var jsonKeyRe = regexp.MustCompile(`^(\s*)("(?:[^"\\]|\\.)*")(:\s*)(.*)$`)

// highlightJSON colours one line of indented JSON.
func highlightJSON(s ui.Styles, line string) string {
	if m := jsonKeyRe.FindStringSubmatch(line); m != nil {
		return m[1] + s.Key.Render(m[2]) + m[3] + jsonValue(s, m[4])
	}
	indent := len(line) - len(strings.TrimLeft(line, " "))
	return line[:indent] + jsonValue(s, line[indent:])
}

func jsonValue(s ui.Styles, v string) string {
	body := strings.TrimSuffix(v, ",")
	tail := v[len(body):]
	switch {
	case strings.HasPrefix(body, `"`):
		return s.String.Render(body) + tail
	case body == "true", body == "false", body == "null", isNumber(body):
		return s.Number.Render(body) + tail
	}
	return v
}
