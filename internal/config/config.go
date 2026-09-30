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

type ConfirmLive string

const (
	ConfirmTypeName ConfirmLive = "type-name"
	ConfirmYesNo    ConfirmLive = "yes-no"
)

type Config struct {
	PolicyDirs []string `toml:"policy_dirs"`
	Runner     Runner   `toml:"runner"`
	Defaults   Defaults `toml:"defaults"`
	Safety     Safety   `toml:"safety"`
}

type Runner struct {
	Kind      RunnerKind `toml:"kind"`
	Custodian string     `toml:"custodian"`
	Command   []string   `toml:"command"`
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
		Runner:     Runner{Kind: RunnerBinary, Custodian: "custodian"},
		Safety:     Safety{DefaultDryRun: true, ConfirmLive: ConfirmTypeName},
	}
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
	case ConfirmTypeName, ConfirmYesNo:
	default:
		errs = append(errs, fmt.Errorf("safety.confirm_live: unknown value %q (want type-name or yes-no)", c.Safety.ConfirmLive))
	}
	return errors.Join(errs...)
}
