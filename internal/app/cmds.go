package app

// Commands: the only place where the app reads files. Each returns a
// message that Update stores in the model.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/c7n"
)

type policiesLoadedMsg struct{ files []c7n.PolicyFile }

func loadPolicies(paths []string) tea.Cmd {
	return func() tea.Msg { return policiesLoadedMsg{files: c7n.Discover(paths)} }
}

// runEntry is one run in the Runs list: a run lazyc7n started, or an
// existing c7n output dir opened with -output.
type runEntry struct {
	ID       string
	Kind     string // "dry-run", "live", "validate" or "dir"
	Started  time.Time
	Ended    time.Time
	OutDir   string   // c7n's -s dir
	LogPath  string   // custodian's stderr, for runs lazyc7n started
	Argv     []string // command line, for runs lazyc7n started
	Backend  string
	Finished bool
	ExitCode int
	Policies []c7n.PolicyRun
	Err      string // why the output could not be read
}

func (r runEntry) matched() int {
	n := 0
	for _, p := range r.Policies {
		n += max(p.ResourceCount, 0)
	}
	return n
}

// ok is false when custodian failed or any policy errored.
func (r runEntry) ok() bool {
	if r.Err != "" || (r.Finished && r.ExitCode != 0) {
		return false
	}
	for _, p := range r.Policies {
		if p.Status() == "error" {
			return false
		}
	}
	return true
}

type runsLoadedMsg struct {
	runs []runEntry
	err  string
}

func loadRuns(outputDirs []string) tea.Cmd {
	return func() tea.Msg {
		var runs []runEntry
		var errs []string
		for _, dir := range outputDirs {
			e := runEntry{ID: dir, Kind: "dir", OutDir: dir, Finished: true}
			pols, err := c7n.ReadOutputDir(dir)
			if err != nil {
				errs = append(errs, dir+": "+err.Error())
				e.Err = err.Error()
			}
			e.Policies = pols
			for _, p := range pols {
				if e.Started.IsZero() || (!p.Start.IsZero() && p.Start.Before(e.Started)) {
					e.Started = p.Start
				}
				if p.End.After(e.Ended) {
					e.Ended = p.End
				}
			}
			runs = append(runs, e)
		}
		sort.SliceStable(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })
		return runsLoadedMsg{runs: runs, err: strings.Join(errs, "; ")}
	}
}

type resourcesLoadedMsg struct {
	dir       string
	resources []c7n.Resource
	err       error
}

func loadResources(pr c7n.PolicyRun) tea.Cmd {
	return func() tea.Msg {
		if pr.ResourceCount < 0 {
			return resourcesLoadedMsg{dir: pr.Dir, err: errors.New("no resources.json: " + noResourcesReason(pr))}
		}
		res, err := c7n.ReadResources(pr.ResourcesPath(), pr.Resource)
		return resourcesLoadedMsg{dir: pr.Dir, resources: res, err: err}
	}
}

func noResourcesReason(pr c7n.PolicyRun) string {
	if pr.Status() == "deployed" {
		return "a live run of a " + pr.Mode + " policy deploys a Lambda and does not evaluate resources"
	}
	return "the policy failed; see its log"
}

type logLoadedMsg struct {
	path  string
	lines []string
	err   error
}

// maxLogBytes keeps the log view responsive; the end of the log is kept.
const maxLogBytes = 1 << 20

func loadLog(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return logLoadedMsg{path: path, err: err}
		}
		if len(data) > maxLogBytes {
			data = data[len(data)-maxLogBytes:]
		}
		text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		return logLoadedMsg{path: path, lines: strings.Split(text, "\n")}
	}
}

// shortPath shows p relative to the working directory when it is inside it.
func shortPath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}
