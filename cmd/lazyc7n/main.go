// Command lazyc7n is a terminal UI for the Cloud Custodian (custodian) CLI.
//
// Independent project; not affiliated with Cloud Custodian or the CNCF.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/vstrofago/lazy-c7n/internal/app"
	"github.com/vstrofago/lazy-c7n/internal/config"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = ""

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyc7n:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "config file to use instead of the user config + "+config.ProjectFile)
	showVersion := flag.Bool("version", false, "print version and exit")
	var outputs listFlag
	flag.Var(&outputs, "output", "existing c7n output dir (custodian -s) to browse in Runs; repeatable")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(),
			"lazyc7n: a terminal UI for the Cloud Custodian (custodian) CLI.\n"+
				"Independent project; not affiliated with Cloud Custodian or the CNCF.\n\n"+
				"Usage: lazyc7n [flags] [policy files or dirs...]\n\n"+
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
	if err != nil {
		return err
	}

	// Bubble Tea restores the terminal on exit and on panics inside the program.
	opts := app.Options{PolicyPaths: flag.Args(), OutputDirs: outputs}
	_, err = tea.NewProgram(app.New(cfg, opts)).Run()
	return err
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
