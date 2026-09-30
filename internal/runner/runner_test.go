package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vstrofago/lazy-c7n/internal/config"
)

func TestArgvBinaryAndCommand(t *testing.T) {
	spec := Spec{
		Subcommand: "run", DryRun: true, OutDir: "/s/runs/1/out", Cache: "/s/c7n.cache",
		CachePeriod: "5", Regions: []string{"us-east-1"}, Policies: []string{"a", "b"}, Files: []string{"p.yml"},
	}
	cfg := config.Default().Runner
	got, err := Argv(cfg, spec, Host{})
	want := []string{"custodian", "run", "-s", "/s/runs/1/out", "-f", "/s/c7n.cache", "--cache-period", "5",
		"--dryrun", "-r", "us-east-1", "-p", "a", "-p", "b", "p.yml"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("binary:\n got %q\nwant %q (err %v)", got, want, err)
	}

	cfg.Kind, cfg.Command = config.RunnerCommand, []string{"uvx", "--from", "c7n", "custodian"}
	got, _ = Argv(cfg, Spec{Subcommand: "validate", Files: []string{"a.yml", "b.yml"}}, Host{})
	if !slices.Equal(got, []string{"uvx", "--from", "c7n", "custodian", "validate", "a.yml", "b.yml"}) {
		t.Fatalf("command: %q", got)
	}

	// Live run: no --dryrun anywhere.
	spec.DryRun = false
	cfg = config.Default().Runner
	got, _ = Argv(cfg, spec, Host{})
	if slices.Contains(got, "--dryrun") {
		t.Fatalf("live argv has --dryrun: %q", got)
	}
}

func TestArgvDocker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix paths")
	}
	cfg := config.Default().Runner
	cfg.Kind = config.RunnerDocker
	cfg.DockerArgs = []string{"--network", "host"}
	host := Host{UID: 1000, GID: 1000, Environ: []string{
		"AWS_ACCESS_KEY_ID=AKIA...", "AWS_SECRET_ACCESS_KEY=supersecret", "HOME=/home/u", "AWS_ENDPOINT_URL=http://x",
	}}
	spec := Spec{
		Subcommand: "run", DryRun: true, OutDir: "/state/runs/1/out", Cache: "/state/c7n.cache",
		Policies: []string{"p"}, Files: []string{"/pol/a.yml", "/pol/b.yml", "/other/c.yml"},
	}
	got, err := Argv(cfg, spec, host)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "run", "--rm", "--user", "1000:1000", "--network", "host",
		"-e", "AWS_ACCESS_KEY_ID", "-e", "AWS_SECRET_ACCESS_KEY", "-e", "AWS_ENDPOINT_URL",
		"-v", "/pol:/lazyc7n/policies/0:ro", "-v", "/other:/lazyc7n/policies/2:ro",
		"-v", "/state/runs/1:/lazyc7n/run", "-v", "/state:/lazyc7n/cache",
		"cloudcustodian/c7n",
		"run", "-s", "/lazyc7n/run/out", "-f", "/lazyc7n/cache/c7n.cache", "--dryrun", "-p", "p",
		"/lazyc7n/policies/0/a.yml", "/lazyc7n/policies/0/b.yml", "/lazyc7n/policies/2/c.yml"}
	if !slices.Equal(got, want) {
		t.Fatalf("docker:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(strings.Join(got, " "), "supersecret") {
		t.Fatal("secret value on the command line")
	}

	// Windows-like host: no --user.
	got, _ = Argv(cfg, Spec{Subcommand: "version"}, Host{UID: -1, GID: -1})
	if slices.Contains(got, "--user") || got[len(got)-1] != "version" {
		t.Fatalf("no-uid docker: %q", got)
	}
}

// TestHelperProcess is not a real test: the tests below run the test binary
// itself as a fake custodian.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("LC7N_HELPER") != "1" {
		return
	}
	switch os.Getenv("LC7N_MODE") {
	case "lines":
		fmt.Println("out 1")
		fmt.Fprintln(os.Stderr, "err 1")
		fmt.Println("out 2")
		os.Exit(3)
	case "sleep":
		fmt.Println("started")
		time.Sleep(time.Minute)
	case "version":
		fmt.Println("0.9.52")
	}
	os.Exit(0)
}

func helper(t *testing.T, mode string) []string {
	t.Helper()
	t.Setenv("LC7N_HELPER", "1")
	t.Setenv("LC7N_MODE", mode)
	return []string{os.Args[0], "-test.run=TestHelperProcess"}
}

func collect(t *testing.T, j *Job) ([]Line, Result) {
	t.Helper()
	var lines []Line
	timeout := time.After(20 * time.Second)
	for {
		select {
		case l, ok := <-j.Lines:
			if !ok {
				return lines, <-j.Done
			}
			lines = append(lines, l)
		case <-timeout:
			t.Fatal("job did not finish")
		}
	}
}

func TestJobStreamsAndLogs(t *testing.T) {
	dir := t.TempDir()
	j, err := Start(helper(t, "lines"), filepath.Join(dir, "stdout.log"), filepath.Join(dir, "stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines, res := collect(t, j)
	if res.ExitCode != 3 || res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	var out, errs []string
	for _, l := range lines {
		if l.Stderr {
			errs = append(errs, l.Text)
		} else {
			out = append(out, l.Text)
		}
	}
	if !slices.Equal(out, []string{"out 1", "out 2"}) || !slices.Equal(errs, []string{"err 1"}) {
		t.Fatalf("out=%q err=%q", out, errs)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "stderr.log"))
	if string(data) != "err 1\n" {
		t.Fatalf("stderr.log = %q", data)
	}
}

func TestJobCancel(t *testing.T) {
	j, err := Start(helper(t, "sleep"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if l := <-j.Lines; l.Text != "started" {
		t.Fatalf("first line %q", l.Text)
	}
	start := time.Now()
	j.Cancel()
	j.Cancel() // twice is fine
	_, res := collect(t, j)
	if res.ExitCode == 0 {
		t.Fatalf("canceled job exited 0")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("cancel took too long")
	}
}

func TestStartMissingBinary(t *testing.T) {
	if _, err := Start([]string{"/no/such/custodian"}, "", ""); err == nil {
		t.Fatal("no error")
	}
}

func TestOutput(t *testing.T) {
	out, err := Output(context.Background(), helper(t, "version"))
	if err != nil || strings.TrimSpace(string(out)) != "0.9.52" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
