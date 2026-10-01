package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/runner"
	"github.com/vstrofago/lazy-c7n/internal/store"
)

// The Resources screen: a table of the matched resources on top, with the
// columns c7n itself picks for the resource type (`custodian report`), and
// a card (or the raw JSON) of the selected resource below.

type resourcesState struct {
	pr       c7n.PolicyRun // whose resources are shown
	runTitle string
	loading  bool
	list     []c7n.Resource
	total    int // resources in resources.json (list may hold fewer)
	err      error
	cursor   int
	scroll   int // detail pane
	showJSON bool

	report        *c7n.Report
	rowOf         []int // report row of each resource in list (-1: none)
	reportLoading bool
	reportErr     string
}

// indexReport matches resources to report rows once both have loaded.
func (st *resourcesState) indexReport() {
	st.rowOf = nil
	if st.report != nil && st.list != nil {
		st.rowOf = st.report.Index(st.list)
	}
}

type reportLoadedMsg struct {
	dir    string
	report *c7n.Report
	err    string
}

func (m Model) openResources(run runEntry, pr c7n.PolicyRun) (tea.Model, tea.Cmd) {
	m.res = resourcesState{pr: pr, runTitle: runTitle(run), loading: true}
	delete(m.filters, ScreenResources)
	m.screen, m.focus = ScreenResources, paneLeft
	if pr.ResourceCount <= 0 {
		return m, loadResources(pr)
	}
	m.res.reportLoading = true
	return m, tea.Batch(loadResources(pr), loadReport(m.cfgRunnerArgv(), m.store, pr))
}

// loadReport runs `custodian report --format csv` for one policy result.
// The policy comes from metadata.json, so this works for any output dir.
func loadReport(argvFor func(runner.Spec) ([]string, error), st store.Store, pr c7n.PolicyRun) tea.Cmd {
	return func() tea.Msg {
		fail := func(err error) tea.Msg { return reportLoadedMsg{dir: pr.Dir, err: err.Error()} }
		file, err := c7n.WriteReportPolicy(pr, filepath.Join(st.Root, "tmp"))
		if err != nil {
			return fail(err)
		}
		defer func() { _ = os.Remove(file) }()
		argv, err := argvFor(runner.Spec{
			Subcommand: "report",
			OutDir:     filepath.Dir(pr.Dir), // c7n reads <dir>/<policy>/resources.json
			Policies:   []string{pr.Policy},
			Files:      []string{file},
		})
		if err != nil {
			return fail(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := runner.Output(ctx, argv)
		if err != nil {
			return fail(err)
		}
		r, err := c7n.ParseReportCSV(out)
		if err != nil {
			return fail(err)
		}
		return reportLoadedMsg{dir: pr.Dir, report: &r}
	}
}

// reportColumns are the report columns shown in the table, without the
// report timestamp (the same on every row).
func (m Model) reportColumns() []int {
	if m.res.report == nil {
		return nil
	}
	var cols []int
	for i, c := range m.res.report.Columns {
		if c != "CustodianDate" {
			cols = append(cols, i)
		}
	}
	return cols
}

// cells are a resource's table cells: its report row, or just its id.
func (m Model) cells(i int) []string {
	r := m.res.list[i]
	cols := m.reportColumns()
	if len(cols) == 0 {
		return []string{r.ID, itoa(len(r.Tags))}
	}
	var row []string
	if i < len(m.res.rowOf) && m.res.rowOf[i] >= 0 {
		row = m.res.report.Rows[m.res.rowOf[i]]
	}
	out := make([]string, len(cols))
	for j, c := range cols {
		if c < len(row) {
			out[j] = row[c]
		}
	}
	return out
}

func (m Model) headers() []string {
	cols := m.reportColumns()
	if len(cols) == 0 {
		return []string{"id", "tags"}
	}
	out := make([]string, len(cols))
	for j, c := range cols {
		out[j] = m.res.report.Columns[c]
	}
	return out
}

// resourceRows are indexes into res.list after the "/" filter, which
// matches the whole row and the tags.
func (m Model) resourceRows() []int {
	filter := m.filters[ScreenResources]
	rows := make([]int, 0, len(m.res.list))
	if filter == "" {
		for i := range m.res.list {
			rows = append(rows, i)
		}
		return rows
	}
	for i, r := range m.res.list {
		text := strings.Join(m.cells(i), " ")
		for _, t := range r.Tags {
			text += " " + t.Key + "=" + t.Value
		}
		if matchesFilter(text, filter) {
			rows = append(rows, i)
		}
	}
	return rows
}

func (m Model) currentResource() (c7n.Resource, bool) {
	rows := m.resourceRows()
	if m.res.cursor < 0 || m.res.cursor >= len(rows) {
		return c7n.Resource{}, false
	}
	return m.res.list[rows[m.res.cursor]], true
}

// resourceLayout splits the body: table on top, detail below.
func (m Model) resourceLayout(height int) (top, bottom int) {
	want := len(m.resourceRows()) + 4 // border, title, header
	top = min(max(want, 6), height*45/100)
	return top, max(height-top, 5)
}

func (m Model) updateResources(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	st := &m.res
	_, bottom := m.resourceLayout(m.bodyHeight())
	page := paneRows(bottom)

	switch {
	case key.Matches(msg, k.Copy):
		if r, ok := m.currentResource(); ok {
			m.setStatus("copied resource id " + r.ID)
			return m, tea.SetClipboard(r.ID)
		}
		return m, nil
	case key.Matches(msg, k.Toggle):
		st.showJSON, st.scroll = !st.showJSON, 0
		return m, nil
	}

	if m.focus == paneRight {
		n := len(m.resourceDetail())
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

	n := len(m.resourceRows())
	move := func(to int) {
		st.cursor = min(max(to, 0), max(n-1, 0))
		st.scroll = 0
	}
	switch {
	case key.Matches(msg, k.Up):
		move(st.cursor - 1)
	case key.Matches(msg, k.Down):
		move(st.cursor + 1)
	case key.Matches(msg, k.PageUp):
		move(st.cursor - page)
	case key.Matches(msg, k.PageDown):
		move(st.cursor + page)
	case key.Matches(msg, k.Top):
		move(0)
	case key.Matches(msg, k.Bottom):
		move(n - 1)
	case key.Matches(msg, k.Enter):
		m.focus = paneRight
	case key.Matches(msg, k.Back):
		m.screen = ScreenRuns
		m.focus = paneRight
	}
	return m, nil
}

func (m Model) viewResources(width, height int) string {
	s := m.styles
	st := m.res
	if st.pr.Dir == "" {
		return m.twoPanes("Resources", []string{
			s.Muted.Render("pick a run in Runs, then a policy,"),
			s.Muted.Render("and press enter"),
		}, "Resource", nil, width, height)
	}

	top, bottom := m.resourceLayout(height)
	title := fmt.Sprintf("%s · %s · %s", st.pr.Policy, st.pr.Region, st.pr.Resource)
	var lines []string
	switch {
	case st.loading:
		lines = []string{s.Muted.Render("loading…")}
	case st.err != nil:
		lines = []string{s.Danger.Render(st.err.Error())}
	case len(st.list) == 0:
		lines = []string{s.Muted.Render("no resources matched")}
	default:
		if st.total > len(st.list) {
			title += fmt.Sprintf(" · showing the first %d of %d", len(st.list), st.total)
		} else {
			title += fmt.Sprintf(" · %d", len(st.list))
		}
		switch {
		case st.reportLoading:
			title += " · loading report…"
		case st.reportErr != "":
			title += " · no report"
		}
		lines = m.resourceTable(width-4, paneRows(top))
	}
	table := m.pane(title, lines, width, top, m.focus == paneLeft)

	detailTitle := "Resource"
	if r, ok := m.currentResource(); ok {
		mode := "t json"
		if st.showJSON {
			mode = "t card"
		}
		detailTitle = r.ID + " · " + mode
	}
	detail := m.pane(detailTitle, scrolled(m.resourceDetail(), st.scroll, paneRows(bottom)), width, bottom, m.focus == paneRight)
	return table + "\n" + detail
}

// resourceTable renders the header and the visible rows, with columns
// sized to their content and dropped from the right when they do not fit.
func (m Model) resourceTable(inner, visible int) []string {
	s := m.styles
	headers := m.headers()
	rows := m.resourceRows()
	// Only the visible rows are built. Column widths come from them and the
	// first rows of the list, so they stay stable while scrolling.
	page := max(visible-1, 1)
	start := windowStart(m.res.cursor, len(rows), page)
	end := min(start+page, len(rows))
	cellsOf := func(idx int) []string {
		cells := m.cells(idx)
		for j, c := range cells {
			cells[j] = shortValue(c)
		}
		return cells
	}
	widths := make([]int, len(headers))
	for j, h := range headers {
		widths[j] = ansi.StringWidth(h)
	}
	measure := func(cells []string) {
		for j, c := range cells {
			if j < len(widths) {
				widths[j] = max(widths[j], ansi.StringWidth(c))
			}
		}
	}
	for _, idx := range rows[:min(len(rows), 200)] {
		measure(cellsOf(idx))
	}
	all := make([][]string, 0, end-start)
	for _, idx := range rows[start:end] {
		cells := cellsOf(idx)
		measure(cells)
		all = append(all, cells)
	}
	const gap = 2
	shown, used := 0, 0
	for j := range widths {
		widths[j] = min(widths[j], 40)
		if j > 0 && used+gap+widths[j] > inner {
			break
		}
		used += widths[j] + gap
		shown++
	}
	shown = max(shown, 1)
	widths[0] = min(widths[0], inner)

	line := func(cells []string, style func(j int, text string) string) string {
		var b strings.Builder
		for j := 0; j < shown && j < len(cells); j++ {
			text := truncate(cells[j], widths[j])
			drawn := max(ansi.StringWidth(text), 1) // an empty cell is drawn as "—"
			b.WriteString(style(j, text))
			b.WriteString(strings.Repeat(" ", max(widths[j]-drawn, 0)+gap))
		}
		return b.String()
	}
	header := line(headers, func(_ int, t string) string { return s.Key.Render(t) })
	if hidden := len(headers) - shown; hidden > 0 {
		header += s.Muted.Render(fmt.Sprintf("+%d", hidden))
	}

	var body []string
	for _, cells := range all {
		body = append(body, line(cells, func(j int, t string) string {
			switch {
			case t == "":
				return s.Muted.Render("—")
			case j == 0:
				return s.Bold.Render(t)
			}
			return s.Item.Render(t)
		}))
	}
	if len(body) == 0 {
		body = []string{s.Muted.Render("nothing matches the filter")}
	}
	return append([]string{header}, m.listLines(body, m.res.cursor-start, page, inner+4, m.focus == paneLeft)...)
}

// timeLayouts are the timestamp shapes found in c7n reports.
var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02 15:04:05.999999"}

func parseTime(v string) (time.Time, bool) {
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// shortValue shortens timestamps for the table.
func shortValue(v string) string {
	if t, ok := parseTime(v); ok {
		return t.Local().Format("2006-01-02 15:04")
	}
	return v
}

// age is a rough "3h ago".
func age(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// resourceDetail is the lower pane: a card of the resource, or its JSON.
func (m Model) resourceDetail() []string {
	s := m.styles
	r, ok := m.currentResource()
	if !ok {
		return nil
	}
	if m.res.showJSON {
		var out []string
		for _, l := range strings.Split(r.Pretty(), "\n") {
			out = append(out, highlightJSON(s, l))
		}
		return out
	}

	width := m.innerWidth()
	out := []string{s.Muted.Render(m.res.pr.Resource + " · " + m.res.pr.Region + " · " + m.res.runTitle), ""}

	// Key fields: the report row, or else the resource's top-level scalars.
	var keys, values []string
	if cols := m.reportColumns(); len(cols) > 0 {
		row := m.cells(m.indexOf(r))
		for j, c := range cols {
			if j >= len(row) {
				break
			}
			if row[j] != r.ID {
				keys = append(keys, m.res.report.Columns[c])
				values = append(values, row[j])
			}
		}
	} else {
		keys, values = scalarFields(r.Raw, 12)
	}
	keyWidth := 0
	for _, k := range keys {
		keyWidth = max(keyWidth, ansi.StringWidth(k))
	}
	for i, k := range keys {
		v := values[i]
		text := s.Item.Render(v)
		switch t, ok := parseTime(v); {
		case v == "":
			text = s.Muted.Render("—")
		case ok:
			text = s.Item.Render(t.Local().Format("2006-01-02 15:04:05")) + s.Muted.Render("  "+age(t))
		}
		out = append(out, s.Muted.Render(fmt.Sprintf("%-*s  ", keyWidth, k))+text)
	}

	if len(r.Tags) > 0 {
		out = append(out, "", s.Bold.Render("tags"))
		out = append(out, m.chips(r.Tags, width)...)
	}
	if matched := matchedFilters(r.Raw); len(matched) > 0 {
		out = append(out, "", s.Bold.Render("matched by"))
		for _, f := range matched {
			out = append(out, wrapped(s.Item, "• "+f, width)...)
		}
	}
	return append(out, "", s.Muted.Render("t raw JSON · y copy id · / filter"))
}

func (m Model) indexOf(r c7n.Resource) int {
	for i, x := range m.res.list {
		if x.ID == r.ID {
			return i
		}
	}
	return -1
}

// innerWidth is the text width of a full-width pane.
func (m Model) innerWidth() int {
	w, _ := m.size()
	return max(w-4, 10)
}

// chips lays tags out as small labels, as many per line as fit.
func (m Model) chips(tags []c7n.Tag, width int) []string {
	var lines []string
	line, used := "", 0
	for _, t := range tags {
		chip := m.styles.Chip.Render(truncate(t.Key+"="+t.Value, max(width-2, 4)))
		w := ansi.StringWidth(chip)
		if used > 0 && used+1+w > width {
			lines = append(lines, line)
			line, used = "", 0
		}
		if used > 0 {
			line += " "
			used++
		}
		line += chip
		used += w
	}
	return append(lines, line)
}

// scalarFields lists the first top-level string/number/bool fields of a
// resource, for when there is no report.
func scalarFields(raw json.RawMessage, limit int) (keys, values []string) {
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil {
		return nil, nil
	}
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if len(keys) == limit || strings.HasPrefix(k, "c7n") {
			continue
		}
		switch v := fields[k].(type) {
		case string:
			keys, values = append(keys, k), append(values, v)
		case float64, bool:
			keys, values = append(keys, k), append(values, fmt.Sprint(v))
		}
	}
	return keys, values
}

// matchedFilters reads c7n:MatchedFilters, which c7n adds to resources that
// matched value filters.
func matchedFilters(raw json.RawMessage) []string {
	var fields struct {
		Matched []string `json:"c7n:MatchedFilters"`
	}
	_ = json.Unmarshal(raw, &fields)
	return fields.Matched
}
