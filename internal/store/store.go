// Package store keeps lazyc7n's own record of runs (SPEC §5): one
// directory per run under <state>/runs/, with run.json, custodian's output
// dir (out/) and the raw stdout/stderr. Plain files only.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Run is run.json: what lazyc7n started and how it ended.
type Run struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"` // "dry-run", "live" or "validate"
	Argv             []string  `json:"argv"` // secrets redacted
	Backend          string    `json:"backend"`
	Policies         []string  `json:"policies,omitempty"`
	Files            []string  `json:"files,omitempty"`
	Started          time.Time `json:"started"`
	Ended            time.Time `json:"ended,omitzero"`
	Finished         bool      `json:"finished"`
	ExitCode         int       `json:"exit_code"`
	Error            string    `json:"error,omitempty"` // custodian could not be started
	CustodianVersion string    `json:"custodian_version,omitempty"`

	Dir string `json:"-"` // the run's directory
}

func (r Run) OutDir() string     { return filepath.Join(r.Dir, "out") }
func (r Run) StdoutPath() string { return filepath.Join(r.Dir, "stdout.log") }
func (r Run) StderrPath() string { return filepath.Join(r.Dir, "stderr.log") }
func (r Run) recordPath() string { return filepath.Join(r.Dir, "run.json") }

// Store is the state directory.
type Store struct{ Root string }

func (s Store) runsDir() string { return filepath.Join(s.Root, "runs") }

// CachePath is the c7n resource cache (-f) lazyc7n passes to custodian, so
// it does not share ~/.cache/cloud-custodian.cache with other tools.
func (s Store) CachePath() string { return filepath.Join(s.Root, "c7n.cache") }

// SchemaCachePath is where `custodian schema --json` is kept per version.
func (s Store) SchemaCachePath(version string) string {
	safe := regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(version, "_")
	return filepath.Join(s.Root, "schema-cache", safe+".json")
}

// Create makes a new, empty run directory.
func (s Store) Create(kind string, now time.Time) (Run, error) {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return Run{}, err
	}
	id := now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix)
	r := Run{ID: id, Kind: kind, Started: now, Dir: filepath.Join(s.runsDir(), id)}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return Run{}, err
	}
	return r, nil
}

// Save writes run.json (via a temp file, so a crash never leaves half a file).
func (s Store) Save(r Run) error {
	r.Argv = RedactArgv(r.Argv)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.recordPath() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.recordPath())
}

// List returns every readable run, newest first.
func (s Store) List() ([]Run, error) {
	entries, err := os.ReadDir(s.runsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var runs []Run
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.runsDir(), e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "run.json"))
		if err != nil {
			continue
		}
		var r Run
		if json.Unmarshal(data, &r) != nil {
			continue
		}
		r.Dir = dir
		runs = append(runs, r)
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })
	return runs, nil
}

// Prune deletes the oldest runs beyond keep (0 = no limit) and runs older
// than maxAge (0 = no limit). Unfinished runs are never deleted. It returns
// the ids it removed.
func (s Store) Prune(keep int, maxAge time.Duration, now time.Time) ([]string, error) {
	runs, err := s.List()
	if err != nil {
		return nil, err
	}
	var removed []string
	var errs []error
	for i, r := range runs {
		tooMany := keep > 0 && i >= keep
		tooOld := maxAge > 0 && now.Sub(r.Started) > maxAge
		if !r.Finished || (!tooMany && !tooOld) {
			continue
		}
		// Only ever delete directories that are really inside runs/.
		if filepath.Dir(r.Dir) != s.runsDir() {
			continue
		}
		if err := os.RemoveAll(r.Dir); err != nil {
			errs = append(errs, err)
			continue
		}
		removed = append(removed, r.ID)
	}
	return removed, errors.Join(errs...)
}

// secretName matches variable/flag names whose values must never be shown
// or stored (SPEC §6.6).
var secretName = regexp.MustCompile(`(?i)(KEY|SECRET|TOKEN|PASSWORD|CREDENTIAL)`)

// RedactArgv hides values of NAME=value arguments whose name looks secret
// (e.g. `env AWS_SECRET_ACCESS_KEY=... custodian`).
func RedactArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		name, _, found := strings.Cut(a, "=")
		if found && secretName.MatchString(name) {
			a = name + "=" + "<redacted>"
		}
		out[i] = a
	}
	return out
}

// String is a one-line description for logs and errors.
func (r Run) String() string {
	return fmt.Sprintf("%s %s (%d policies)", r.Kind, r.ID, len(r.Policies))
}
