// Command lazyc7n is a terminal UI for the Cloud Custodian (custodian) CLI.
//
// Independent project; not affiliated with Cloud Custodian or the CNCF.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

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
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(),
			"lazyc7n: a terminal UI for the Cloud Custodian (custodian) CLI.\n"+
				"Independent project; not affiliated with Cloud Custodian or the CNCF.\n\n"+
				"Usage: lazyc7n [flags]\n\n")
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
	_, err = tea.NewProgram(app.New(cfg)).Run()
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
