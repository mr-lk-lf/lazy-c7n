// Package runner builds and runs custodian command lines for the three
// backends (SPEC §3): binary, command (a prefix such as uvx or aws-vault)
// and docker. It never decides *what* to run; the app does.
package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vstrofago/lazy-c7n/internal/config"
)

// Spec is one custodian invocation. Paths are host paths; the docker
// backend maps them into the container.
type Spec struct {
	Subcommand  string // "run", "validate", "version" or "schema"
	DryRun      bool
	OutDir      string // run: -s
	Cache       string // run: -f
	CachePeriod string // run: --cache-period (minutes)
	Regions     []string
	Policies    []string // run: -p (names)
	Files       []string // policy files
	Args        []string // extra trailing arguments (schema)
}

// Host is what the docker backend needs from the machine: the user to run
// as and the environment names to pass through. Tests use a fixed Host.
type Host struct {
	UID, GID int // -1 when not available (Windows)
	Environ  []string
}

// CurrentHost describes this process.
func CurrentHost() Host {
	return Host{UID: os.Getuid(), GID: os.Getgid(), Environ: os.Environ()}
}

// Container paths used by the docker backend. They are outside
// /home/custodian, which is not readable when running as the host user.
const (
	containerPolicies = "/lazyc7n/policies"
	containerRun      = "/lazyc7n/run"
	containerCache    = "/lazyc7n/cache"
)

// Argv returns the full command line for spec with the configured backend.
func Argv(cfg config.Runner, spec Spec, host Host) ([]string, error) {
	switch cfg.Kind {
	case config.RunnerBinary:
		if cfg.Custodian == "" {
			return nil, errors.New("runner.custodian is empty")
		}
		return append([]string{cfg.Custodian}, custodianArgs(spec, identity)...), nil
	case config.RunnerCommand:
		if len(cfg.Command) == 0 {
			return nil, errors.New("runner.command is empty")
		}
		return append(append([]string{}, cfg.Command...), custodianArgs(spec, identity)...), nil
	case config.RunnerDocker:
		return dockerArgv(cfg, spec, host)
	}
	return nil, fmt.Errorf("unknown runner kind %q", cfg.Kind)
}

func identity(p string) string { return p }

// custodianArgs is everything after the custodian executable. path maps a
// host path to the path custodian will see.
func custodianArgs(s Spec, path func(string) string) []string {
	args := []string{s.Subcommand}
	if s.Subcommand == "run" {
		args = append(args, "-s", path(s.OutDir))
		if s.Cache != "" {
			args = append(args, "-f", path(s.Cache))
		}
		if s.CachePeriod != "" {
			args = append(args, "--cache-period", s.CachePeriod)
		}
		if s.DryRun {
			args = append(args, "--dryrun")
		}
		for _, r := range s.Regions {
			args = append(args, "-r", r)
		}
		for _, p := range s.Policies {
			args = append(args, "-p", p)
		}
	}
	for _, f := range s.Files {
		args = append(args, path(f))
	}
	return append(args, s.Args...)
}

// passthroughPrefixes are the environment variables handed to the
// container. Only names are put on the command line (`-e NAME`); docker
// reads the values from its own environment, so secrets never appear in
// argv or run.json.
var passthroughPrefixes = []string{"AWS_", "AZURE_", "GOOGLE_", "CLOUDSDK_", "ARM_", "OCI_", "TENCENTCLOUD_", "OS_", "KUBECONFIG"}

func dockerArgv(cfg config.Runner, s Spec, host Host) ([]string, error) {
	argv := []string{cfg.Docker, "run", "--rm"}
	if host.UID >= 0 && host.GID >= 0 {
		argv = append(argv, "--user", fmt.Sprintf("%d:%d", host.UID, host.GID))
	}
	argv = append(argv, cfg.DockerArgs...)

	var names []string
	for _, kv := range host.Environ {
		name, _, _ := strings.Cut(kv, "=")
		for _, p := range passthroughPrefixes {
			if strings.HasPrefix(name, p) {
				names = append(names, name)
				break
			}
		}
	}
	for _, n := range names {
		argv = append(argv, "-e", n)
	}

	// Mount each policy file's directory read-only, the run dir (parent of
	// -s) and the cache directory.
	mounts := map[string]string{} // host dir -> container dir
	mount := func(hostDir, containerDir string, ro bool) {
		if _, ok := mounts[hostDir]; ok {
			return
		}
		mounts[hostDir] = containerDir
		opt := hostDir + ":" + containerDir
		if ro {
			opt += ":ro"
		}
		argv = append(argv, "-v", opt)
	}
	abs := func(p string) (string, error) {
		a, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("docker backend: %w", err)
		}
		return a, nil
	}

	fileMap := map[string]string{}
	for i, f := range s.Files {
		a, err := abs(f)
		if err != nil {
			return nil, err
		}
		dir := filepath.Dir(a)
		mount(dir, fmt.Sprintf("%s/%d", containerPolicies, i), true)
		fileMap[f] = mounts[dir] + "/" + filepath.Base(a)
	}
	if s.Subcommand == "run" {
		out, err := abs(s.OutDir)
		if err != nil {
			return nil, err
		}
		mount(filepath.Dir(out), containerRun, false)
		fileMap[s.OutDir] = mounts[filepath.Dir(out)] + "/" + filepath.Base(out)
		if s.Cache != "" {
			c, err := abs(s.Cache)
			if err != nil {
				return nil, err
			}
			mount(filepath.Dir(c), containerCache, false)
			fileMap[s.Cache] = mounts[filepath.Dir(c)] + "/" + filepath.Base(c)
		}
	}

	argv = append(argv, cfg.Image)
	return append(argv, custodianArgs(s, func(p string) string {
		if c, ok := fileMap[p]; ok {
			return c
		}
		return p
	})...), nil
}
