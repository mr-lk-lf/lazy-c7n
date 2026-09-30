// Package config loads lazyc7n's configuration (SPEC §5).
//
// The user config ($XDG_CONFIG_HOME/lazyc7n/config.toml) is loaded first and
// the project-local .lazyc7n.toml is decoded on top of it, so keys present in
// the project file win and everything else is kept. Unknown keys are ignored.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// ProjectFile is the project-local override file, looked up in the cwd.
const ProjectFile = ".lazyc7n.toml"

type RunnerKind string

const (
	RunnerBinary  RunnerKind = "binary"
	RunnerDocker  RunnerKind = "docker"
	RunnerCommand RunnerKind = "command"
)

type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

type ConfirmLive string

// ConfirmTypeName is the only confirmation mode in v0: the user types the
// policy name (or the number of policies). A weaker "yes-no" mode was
// considered and rejected for v0.
const ConfirmTypeName ConfirmLive = "type-name"

type Config struct {
	PolicyDirs []string `toml:"policy_dirs"`
	// StateDir holds run history and caches; empty = $XDG_STATE_HOME/lazyc7n.
	StateDir string `toml:"state_dir"`
	// KeepRuns is how many runs to keep in the history (0 = all).
	KeepRuns int `toml:"keep_runs"`
	// Theme is "auto" (follow the terminal background), "dark" or "light".
	Theme    Theme    `toml:"theme"`
	Runner   Runner   `toml:"runner"`
	Defaults Defaults `toml:"defaults"`
	Safety   Safety   `toml:"safety"`
}

type Runner struct {
	Kind      RunnerKind `toml:"kind"`
	Custodian string     `toml:"custodian"` // binary: name or path
	Command   []string   `toml:"command"`   // command: prefix, e.g. ["uvx", "--from", "c7n", "custodian"]

	// docker backend (SPEC §3).
	Docker     string   `toml:"docker"`      // docker (or podman) executable
	Image      string   `toml:"image"`       // c7n image
	DockerArgs []string `toml:"docker_args"` // extra `docker run` args, e.g. ["--network", "host"]
}

type Defaults struct {
	// Region empty = let custodian decide.
	Region      string `toml:"region"`
	CachePeriod string `toml:"cache_period"`
}

type Safety struct {
	DefaultDryRun bool        `toml:"default_dry_run"`
	ConfirmLive   ConfirmLive `toml:"confirm_live"`
}

// Default returns the built-in configuration. It is the safe baseline:
// dry-run by default and typed confirmation for live runs.
func Default() Config {
	return Config{
		PolicyDirs: []string{"./policies"},
		KeepRuns:   200,
		Theme:      ThemeAuto,
		Runner: Runner{
			Kind:      RunnerBinary,
			Custodian: "custodian",
			Docker:    "docker",
			Image:     "cloudcustodian/c7n",
		},
		Safety: Safety{DefaultDryRun: true, ConfirmLive: ConfirmTypeName},
	}
}

// StatePath is the state directory: StateDir, or $XDG_STATE_HOME/lazyc7n.
func (c Config) StatePath() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(xdg.StateHome, "lazyc7n")
}

// UserConfigPath is $XDG_CONFIG_HOME/lazyc7n/config.toml (platform
// equivalent elsewhere).
func UserConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "lazyc7n", "config.toml")
}

// Load reads explicit if it is non-empty (it must exist). Otherwise it layers
// the user config and cwd/.lazyc7n.toml over Default(); missing implicit files
// are not an error.
func Load(explicit, cwd string) (Config, error) {
	var files []string
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return Config{}, fmt.Errorf("config file not found: %s", explicit)
		}
		files = []string{explicit}
	} else {
		for _, f := range []string{UserConfigPath(), filepath.Join(cwd, ProjectFile)} {
			if _, err := os.Stat(f); err == nil {
				files = append(files, f)
			}
		}
	}

	cfg := Default()
	for _, f := range files {
		if _, err := toml.DecodeFile(f, &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", f, err)
		}
	}
	return cfg, cfg.Validate()
}

// Validate rejects unknown enum values instead of silently falling back, so
// a typo in a safety setting is never ignored.
func (c Config) Validate() error {
	var errs []error
	switch c.Runner.Kind {
	case RunnerBinary, RunnerDocker, RunnerCommand:
	default:
		errs = append(errs, fmt.Errorf("runner.kind: unknown value %q (want binary, docker or command)", c.Runner.Kind))
	}
	switch c.Safety.ConfirmLive {
	case ConfirmTypeName:
	case "yes-no":
		errs = append(errs, errors.New(`safety.confirm_live: "yes-no" is not supported in this version; remove the key or set it to "type-name"`))
	default:
		errs = append(errs, fmt.Errorf("safety.confirm_live: unknown value %q (want type-name)", c.Safety.ConfirmLive))
	}
	switch c.Theme {
	case ThemeAuto, ThemeDark, ThemeLight:
	default:
		errs = append(errs, fmt.Errorf("theme: unknown value %q (want auto, dark or light)", c.Theme))
	}
	if c.Runner.Kind == RunnerCommand && len(c.Runner.Command) == 0 {
		errs = append(errs, errors.New(`runner.command: required when runner.kind = "command"`))
	}
	if c.KeepRuns < 0 {
		errs = append(errs, errors.New("keep_runs: must be 0 (keep all) or more"))
	}
	return errors.Join(errs...)
}
