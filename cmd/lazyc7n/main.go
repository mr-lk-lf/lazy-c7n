// Command lazyc7n is a terminal UI for the Cloud Custodian (custodian) CLI.
//
// Independent project; not affiliated with Cloud Custodian or the CNCF.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/app"
	"github.com/vstrofago/lazy-c7n/internal/c7n"
	"github.com/vstrofago/lazy-c7n/internal/config"
	"github.com/vstrofago/lazy-c7n/internal/runner"
	"github.com/vstrofago/lazy-c7n/internal/store"
)

// Set at release time with -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = ""
	commit  = ""
	date    = ""
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyc7n:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "config file to use instead of the user config + "+config.ProjectFile)
	showVersion := flag.Bool("version", false, "print version and exit")
	prune := flag.Bool("prune", false, "delete old runs from the history (keep_runs newest, and -prune-days) and exit")
	pruneDays := flag.Int("prune-days", 0, "with -prune: also delete runs older than this many days")
	var outputs listFlag
	flag.Var(&outputs, "output", "existing c7n output dir (custodian -s) to browse in Runs; repeatable")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(),
			"lazyc7n: a terminal UI for the Cloud Custodian (custodian) CLI.\n"+
				"Independent project; not affiliated with Cloud Custodian or the CNCF.\n\n"+
				"Usage: lazyc7n [flags] [policy files or dirs...]\n"+
				"       lazyc7n version   print versions (lazyc7n, custodian) and exit\n\n"+
				"Policy paths given as arguments replace policy_dirs from the config.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("lazyc7n", buildVersion())
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(*configPath, cwd)

	if flag.NArg() == 1 && flag.Arg(0) == "version" {
		printVersions(cfg, err)
		return nil
	}
	if err != nil {
		return err
	}

	if err := c7n.SetActionOverrides(cfg.Safety.Actions); err != nil {
		return err // already checked by config.Load; kept for safety
	}

	if *prune {
		st := store.Store{Root: cfg.StatePath()}
		removed, err := st.Prune(cfg.KeepRuns, time.Duration(*pruneDays)*24*time.Hour, time.Now())
		fmt.Printf("removed %d run(s) from %s\n", len(removed), st.Root)
		return err
	}

	opts := app.Options{PolicyPaths: flag.Args(), OutputDirs: outputs, Environ: os.Environ()}
	// Bubble Tea restores the terminal on exit and on panics inside the program.
	_, err = tea.NewProgram(app.New(cfg, opts)).Run()
	return err
}

// printVersions is `lazyc7n version`: everything a bug report needs.
func printVersions(cfg config.Config, cfgErr error) {
	fmt.Println("lazyc7n", buildVersion())
	if c := buildCommit(); c != "" {
		fmt.Println("  commit", c)
	}
	if date != "" {
		fmt.Println("  built ", date)
	}
	fmt.Printf("  %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if cfgErr != nil {
		fmt.Println("config: error:", cfgErr)
		return
	}
	argv, err := runner.Argv(cfg.Runner, runner.Spec{Subcommand: "version"}, runner.CurrentHost())
	if err != nil {
		fmt.Println("custodian: error:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := runner.Output(ctx, argv)
	if err != nil {
		fmt.Printf("custodian: not available (%s backend, %s): %v\n", cfg.Runner.Kind, argv[0], err)
		return
	}
	fmt.Printf("custodian %s (%s backend: %s)\n", strings.TrimSpace(string(out)), cfg.Runner.Kind, argv[0])
}

func buildCommit() string {
	if commit != "" {
		return commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return ""
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

// listFlag is a flag that can be given several times.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	*l = append(*l, v)
	return nil
}
