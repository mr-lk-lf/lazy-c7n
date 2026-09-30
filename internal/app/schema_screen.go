package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/runner"
	"github.com/vstrofago/lazy-c7n/internal/store"
)

type schemaState struct {
	loading  bool
	schema   *c7n.Schema
	err      string
	resource string // "" = the list of resource types
	cursor   int    // resource types
	item     int    // actions/filters of resource
	scroll   int    // right pane
	help     map[string]string
}

// schemaItem is one action or filter of a resource type.
type schemaItem struct {
	category string // "actions" or "filters"
	name     string
}

func (it schemaItem) path(resource string) string {
	return resource + "." + it.category + "." + it.name
}

type (
	schemaLoadedMsg struct {
		schema *c7n.Schema
		err    string
	}
	schemaHelpMsg struct {
		path string
		text string
	}
)

// loadSchema reads the cached `custodian schema --json` for this custodian
// version, or runs it (a few seconds) and caches the result.
func loadSchema(argvFor func(runner.Spec) ([]string, error), st store.Store, version string, refresh bool) tea.Cmd {
	return func() tea.Msg {
		cache := ""
		if version != "" {
			cache = st.SchemaCachePath(version)
		}
		var data []byte
		if cache != "" && !refresh {
			data, _ = os.ReadFile(cache)
		}
		if data == nil {
			argv, err := argvFor(runner.Spec{Subcommand: "schema", Args: []string{"--json"}})
			if err != nil {
				return schemaLoadedMsg{err: err.Error()}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			data, err = runner.Output(ctx, argv)
			if err != nil {
				return schemaLoadedMsg{err: "custodian schema --json: " + err.Error()}
			}
			if cache != "" && os.MkdirAll(filepath.Dir(cache), 0o700) == nil {
				_ = os.WriteFile(cache, data, 0o600)
			}
		}
		s, err := c7n.ParseSchema(data)
		if err != nil {
			return schemaLoadedMsg{err: err.Error()}
		}
		return schemaLoadedMsg{schema: &s}
	}
}

// loadSchemaHelp runs `custodian schema <resource>.<category>.<name>`,
// which prints the docstring of an action or filter.
func loadSchemaHelp(argvFor func(runner.Spec) ([]string, error), path string) tea.Cmd {
	return func() tea.Msg {
		argv, err := argvFor(runner.Spec{Subcommand: "schema", Args: []string{path}})
		if err != nil {
			return schemaHelpMsg{path: path, text: err.Error()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := runner.Output(ctx, argv)
		if err != nil {
			return schemaHelpMsg{path: path, text: "could not load help: " + err.Error()}
		}
		text := strings.TrimSpace(string(out))
		text = strings.TrimSpace(strings.TrimPrefix(text, "Help\n----"))
		return schemaHelpMsg{path: path, text: text}
	}
}

// ensureSchema starts loading the schema the first time it is needed.
func (m *Model) ensureSchema() tea.Cmd {
	if m.screen != ScreenSchema || m.schema.schema != nil || m.schema.loading {
		return nil
	}
	m.schema.loading, m.schema.err = true, ""
	return loadSchema(m.cfgRunnerArgv(), m.store, m.version, false)
}

func (m Model) schemaTypes() []string {
	if m.schema.schema == nil {
		return nil
	}
	var out []string
	for _, t := range m.schema.schema.ResourceTypes() {
		if matchesFilter(t, m.filters[ScreenSchema]) {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) schemaItems() []schemaItem {
	if m.schema.schema == nil {
		return nil
	}
	r := m.schema.schema.Resources[m.schema.resource]
	var out []schemaItem
	for _, a := range c7n.Names(r.Actions) {
		if matchesFilter("action "+a, m.filters[ScreenSchema]) {
			out = append(out, schemaItem{"actions", a})
		}
	}
	for _, f := range c7n.Names(r.Filters) {
		if matchesFilter("filter "+f, m.filters[ScreenSchema]) {
			out = append(out, schemaItem{"filters", f})
		}
	}
	return out
}

func (m Model) currentSchemaItem() (schemaItem, bool) {
	items := m.schemaItems()
	if m.schema.item < 0 || m.schema.item >= len(items) {
		return schemaItem{}, false
	}
	return items[m.schema.item], true
}

func (m Model) updateSchema(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.schema
	page := paneRows(m.bodyHeight())

	if key.Matches(msg, k.Reload) {
		st.schema, st.loading, st.err = nil, true, ""
		return m, loadSchema(m.cfgRunnerArgv(), m.store, m.version, true)
	}

	if m.focus == paneRight {
		n := len(m.schemaDetail())
		switch {
		case key.Matches(msg, k.Up):
			st.scroll = scrollBy(st.scroll, -1, n, page)
		case key.Matches(msg, k.Down):
			st.scroll = scrollBy(st.scroll, 1, n, page)
		case key.Matches(msg, k.PageUp):
			st.scroll = scrollBy(st.scroll, -page, n, page)
		case key.Matches(msg, k.PageDown):
			st.scroll = scrollBy(st.scroll, page, n, page)
		case key.Matches(msg, k.Top):
			st.scroll = 0
		case key.Matches(msg, k.Bottom):
			st.scroll = scrollBy(0, n, n, page)
		case key.Matches(msg, k.Back):
			m.focus = paneLeft
		}
		return m, nil
	}

	// Left pane: resource types, or the items of one resource.
	cursor := &st.cursor
	n := len(m.schemaTypes())
	if st.resource != "" {
		cursor, n = &st.item, len(m.schemaItems())
	}
	move := func(to int) {
		*cursor = min(max(to, 0), max(n-1, 0))
		st.scroll = 0
	}
	switch {
	case key.Matches(msg, k.Up):
		move(*cursor - 1)
	case key.Matches(msg, k.Down):
		move(*cursor + 1)
	case key.Matches(msg, k.PageUp):
		move(*cursor - page)
	case key.Matches(msg, k.PageDown):
		move(*cursor + page)
	case key.Matches(msg, k.Top):
		move(0)
	case key.Matches(msg, k.Bottom):
		move(n - 1)
	case key.Matches(msg, k.Back):
		if st.resource != "" {
			st.resource, st.item, st.scroll = "", 0, 0
		}
	case key.Matches(msg, k.Enter):
		if st.resource == "" {
			types := m.schemaTypes()
			if st.cursor < len(types) {
				st.resource, st.item, st.scroll = types[st.cursor], 0, 0
				delete(m.filters, ScreenSchema)
			}
			return m, nil
		}
		if it, ok := m.currentSchemaItem(); ok {
			path := it.path(st.resource)
			if _, done := st.help[path]; !done {
				if st.help == nil {
					st.help = map[string]string{}
				}
				st.help[path] = "" // loading
				return m, loadSchemaHelp(m.cfgRunnerArgv(), path)
			}
		}
	}
	return m, nil
}

func (m Model) viewSchema(width, height int) string {
	s := m.styles
	st := m.schema
	visible := paneRows(height)

	var title string
	var list []string
	cursor := st.cursor
	switch {
	case st.loading:
		title = "Schema"
		list = []string{s.Muted.Render("running custodian schema --json…"), s.Muted.Render("(cached per c7n version after the first time)")}
	case st.err != "":
		title = "Schema"
		list = []string{s.Danger.Render(st.err), "", s.Muted.Render("r to retry")}
	case st.schema == nil:
		title = "Schema"
	case st.resource == "":
		types := m.schemaTypes()
		title = fmt.Sprintf("Resource types · %d", len(types))
		list = types
	default:
		title = st.resource + " · esc back"
		cursor = st.item
		for _, it := range m.schemaItems() {
			if it.category == "actions" {
				list = append(list, m.actionLabel(it.name))
			} else {
				list = append(list, s.Muted.Render("filter ")+it.name)
			}
		}
	}
	list = m.listLines(list, cursor, visible, leftWidth(width), m.focus == paneLeft)
	detailTitle := "Details"
	if st.resource != "" {
		if it, ok := m.currentSchemaItem(); ok {
			detailTitle = it.path(st.resource)
		}
	} else if types := m.schemaTypes(); st.cursor < len(types) {
		detailTitle = types[st.cursor]
	}
	return m.twoPanes(title, list, detailTitle, scrolled(m.schemaDetail(), st.scroll, visible), width, height)
}

// actionLabel is "action <name>" coloured by safety class.
func (m Model) actionLabel(name string) string {
	s := m.styles
	prefix := s.Muted.Render("action ")
	switch c7n.ClassifyAction(name) {
	case c7n.ActionDestructive:
		return prefix + s.Danger.Render(name)
	case c7n.ActionMutating:
		return prefix + s.Warn.Render(name)
	case c7n.ActionNotify:
		return prefix + s.OK.Render(name)
	}
	return prefix + name
}

func (m Model) schemaDetail() []string {
	s := m.styles
	st := m.schema
	if st.schema == nil {
		return nil
	}
	width := m.rightInner()
	if st.resource == "" {
		types := m.schemaTypes()
		if st.cursor >= len(types) {
			return nil
		}
		r := st.schema.Resources[types[st.cursor]]
		actions := c7n.Names(r.Actions)
		var colored []string
		for _, a := range actions {
			colored = append(colored, strings.TrimPrefix(m.actionLabel(a), s.Muted.Render("action ")))
		}
		out := []string{s.Muted.Render(fmt.Sprintf("%d actions · %d filters · enter to browse", len(actions), len(r.Filters))), ""}
		out = append(out, s.Bold.Render("actions"))
		out = append(out, wrapped(s.Item, strings.Join(colored, "  "), width)...)
		out = append(out, "", s.Bold.Render("filters"))
		out = append(out, wrapped(s.Item, strings.Join(c7n.Names(r.Filters), "  "), width)...)
		return out
	}

	it, ok := m.currentSchemaItem()
	if !ok {
		return nil
	}
	r := st.schema.Resources[st.resource]
	var raw []byte
	var out []string
	if it.category == "actions" {
		raw = r.Actions[it.name]
		out = append(out, s.Bold.Render("class ")+strings.TrimPrefix(m.actionLabel(it.name), s.Muted.Render("action ")))
		out = append(out, s.Muted.Render(fmt.Sprintf("(%s)", c7n.ClassifyAction(it.name))), "")
	} else {
		raw = r.Filters[it.name]
	}
	help, asked := st.help[it.path(st.resource)]
	switch {
	case !asked:
		out = append(out, s.Muted.Render("enter: show help (custodian schema "+it.path(st.resource)+")"))
	case help == "":
		out = append(out, s.Muted.Render("loading help…"))
	default:
		for _, l := range strings.Split(help, "\n") {
			out = append(out, wrapped(s.Item, l, width)...)
		}
	}
	out = append(out, "", s.Bold.Render("JSON schema"))
	for _, l := range strings.Split(c7n.PrettyJSON(raw), "\n") {
		out = append(out, highlightJSON(s, l))
	}
	return out
}
