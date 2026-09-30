package runner

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Line is one line of output from the process.
type Line struct {
	Text   string
	Stderr bool
}

// Result is how the process ended.
type Result struct {
	ExitCode int   // -1 if it could not run or was killed by a signal
	Err      error // start/wait error, nil for a normal exit (even non-zero)
}

// Job is a running custodian process. Read Lines until it is closed, then
// read Done once.
type Job struct {
	Lines <-chan Line
	Done  <-chan Result

	cmd      *exec.Cmd
	exited   chan struct{} // closed when the process has been waited for
	mu       sync.Mutex
	canceled bool
}

// cancelGrace is how long a canceled process gets after the interrupt
// before it is killed.
const cancelGrace = 5 * time.Second

// Start runs argv in its own process group. Every line of stdout and
// stderr is written to stdoutPath / stderrPath (when not empty) and sent on
// Lines.
func Start(argv []string, stdoutPath, stderrPath string) (*Job, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	setProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	outFile, err := createLog(stdoutPath)
	if err != nil {
		return nil, err
	}
	errFile, err := createLog(stderrPath)
	if err != nil {
		closeLog(outFile)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		closeLog(outFile)
		closeLog(errFile)
		return nil, err
	}

	lines := make(chan Line, 1024)
	done := make(chan Result, 1)
	j := &Job{Lines: lines, Done: done, cmd: cmd, exited: make(chan struct{})}

	var wg sync.WaitGroup
	wg.Add(2)
	go pump(&wg, stdout, outFile, false, lines)
	go pump(&wg, stderr, errFile, true, lines)
	go func() {
		wg.Wait() // read everything before Wait closes the pipes
		err := cmd.Wait()
		close(j.exited)
		closeLog(outFile)
		closeLog(errFile)
		close(lines)
		done <- result(err)
	}()
	return j, nil
}

func pump(wg *sync.WaitGroup, r io.Reader, log *os.File, isErr bool, out chan<- Line) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		text := sc.Text()
		if log != nil {
			_, _ = log.WriteString(text + "\n")
		}
		out <- Line{Text: text, Stderr: isErr}
	}
	// Keep draining after a scanner error so the process never blocks.
	_, _ = io.Copy(io.Discard, r)
}

func result(err error) Result {
	if err == nil {
		return Result{ExitCode: 0}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return Result{ExitCode: exit.ExitCode()}
	}
	return Result{ExitCode: -1, Err: err}
}

func createLog(path string) (*os.File, error) {
	if path == "" {
		return nil, nil
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
}

func closeLog(f *os.File) {
	if f != nil {
		_ = f.Close()
	}
}

// Cancel asks the process (and its children) to stop: an interrupt first,
// then a kill after cancelGrace if it is still running.
func (j *Job) Cancel() {
	j.mu.Lock()
	if j.canceled {
		j.mu.Unlock()
		return
	}
	j.canceled = true
	j.mu.Unlock()

	interrupt(j.cmd)
	go func() {
		select {
		case <-j.exited:
		case <-time.After(cancelGrace):
			kill(j.cmd)
		}
	}()
}

// Output runs argv and returns its stdout (for short commands such as
// `custodian version`). stderr is returned in the error on failure.
func Output(ctx context.Context, argv []string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return out, errors.New(lastLine(string(exit.Stderr)))
	}
	return out, err
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
