//go:build unix

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"mint/internal/runner"
)

// TestSignalHelper is not a test: the signal tests run the test binary again as mint
// runs, MINT_SIGNAL_HELPER saying what it does once runContext holds the signals. It
// reports to the test on fd 3 and, in the run mode, waits on fd 4 for the go-ahead.
func TestSignalHelper(t *testing.T) {
	mode := os.Getenv("MINT_SIGNAL_HELPER")
	if mode == "" {
		return
	}
	os.Exit(signalHelper(mode))
}

func signalHelper(mode string) int {
	ctx, stopCatching := runContext()
	defer stopCatching()
	report := os.NewFile(3, "report")
	if mode == "hook" {
		reportHookDispositions(report)
		return 0
	}
	_, _ = fmt.Fprintln(report, "ready")
	<-ctx.Done()
	_, _ = fmt.Fprintln(report, "stopped")
	_, _ = io.Copy(io.Discard, os.NewFile(4, "go-ahead"))
	// The unwind's narration, as the presenter writes it: errors discarded.
	for range 3 {
		_, _ = fmt.Fprintln(os.Stdout, "unwind: reset 1 commit")
		_, _ = fmt.Fprintln(os.Stderr, "unwind: reset 1 commit")
	}
	_, _ = fmt.Fprintln(report, "cleaned up")
	return 0
}

// reportHookDispositions runs a hook through the hooks runner per signal, which sends
// the signal to itself, and reports whether it was killed or survived.
func reportHookDispositions(report io.Writer) {
	r := runner.NewGracefulExecRunner(time.Minute)
	for _, sig := range []string{"PIPE", "HUP"} {
		res, _ := r.RunInDir(context.Background(), ".", nil, "sh", "-c", "kill -"+sig+" $$; echo survived")
		fate := "killed"
		if strings.TrimSpace(res.Stdout) == "survived" {
			fate = "survived"
		}
		_, _ = fmt.Fprintln(report, sig, fate)
	}
}

// signalHelperProc is the helper running as mint, the reports it writes and the
// go-ahead it waits on.
type signalHelperProc struct {
	cmd     *exec.Cmd
	report  *os.File
	reports *bufio.Scanner
	goAhead *os.File
}

// startSignalHelper starts the helper in mode, through wrap, a command that execs it,
// where given, and set up further by setup where given.
func startSignalHelper(t *testing.T, mode string, wrap []string, setup func(cmd *exec.Cmd)) *signalHelperProc {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	argv := append(slices.Clone(wrap), self, "-test.run=^TestSignalHelper$")
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "MINT_SIGNAL_HELPER="+mode)
	reportR, reportW := pipe(t)
	goAheadR, goAheadW := pipe(t)
	cmd.ExtraFiles = []*os.File{reportW, goAheadR}
	if setup != nil {
		setup(cmd)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	_ = reportW.Close()
	_ = goAheadR.Close()
	return &signalHelperProc{cmd: cmd, report: reportR, reports: bufio.NewScanner(reportR), goAhead: goAheadW}
}

func pipe(t *testing.T) (r, w *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	return r, w
}

// expect reads the helper's next report, failing unless it is want within half a
// minute.
func (h *signalHelperProc) expect(t *testing.T, want string) {
	t.Helper()
	_ = h.report.SetReadDeadline(time.Now().Add(30 * time.Second))
	if !h.reports.Scan() {
		if err := h.reports.Err(); err != nil {
			t.Fatalf("no report %q from the helper: %v", want, err)
		}
		t.Fatalf("the helper ended before reporting %q: %v", want, h.wait(t))
	}
	if got := h.reports.Text(); got != want {
		t.Fatalf("the helper reported %q, want %q", got, want)
	}
}

// signal sends the helper sig.
func (h *signalHelperProc) signal(t *testing.T, sig os.Signal) {
	t.Helper()
	if err := h.cmd.Process.Signal(sig); err != nil {
		t.Fatalf("signalling the helper: %v", err)
	}
}

// wait is how the helper exited, for no longer than half a minute.
func (h *signalHelperProc) wait(t *testing.T) error {
	t.Helper()
	exited := make(chan error, 1)
	go func() { exited <- h.cmd.Wait() }()
	select {
	case err := <-exited:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("the helper had not exited after 30s")
		return nil
	}
}

// Ctrl-C, SIGTERM and SIGHUP (the terminal closing, an SSH session dropping) each
// stop the run the same way: the context ends, and mint lives on to clean up.
func TestRunContext_EachEndingSignalStopsTheRun(t *testing.T) {
	t.Parallel()

	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()

			h := startSignalHelper(t, "run", nil, nil)
			h.expect(t, "ready")

			h.signal(t, sig)
			h.expect(t, "stopped")
			_ = h.goAhead.Close()
			h.expect(t, "cleaned up")

			if err := h.wait(t); err != nil {
				t.Errorf("the helper exited with %v, want a clean exit after its cleanup", err)
			}
		})
	}
}

// A closed terminal doesn't kill mint mid-cleanup: a second hang-up is dropped, and
// writes to the stdout and stderr nothing reads any more fail without raising SIGPIPE.
func TestRunContext_AClosedTerminalDoesNotKillMintMidCleanup(t *testing.T) {
	t.Parallel()

	stdoutR, stdoutW := pipe(t)
	stderrR, stderrW := pipe(t)
	h := startSignalHelper(t, "run", nil, func(cmd *exec.Cmd) {
		cmd.Stdout, cmd.Stderr = stdoutW, stderrW
	})
	_ = stdoutW.Close()
	_ = stderrW.Close()
	h.expect(t, "ready")
	_ = stdoutR.Close()
	_ = stderrR.Close()

	h.signal(t, syscall.SIGHUP)
	h.expect(t, "stopped")
	h.signal(t, syscall.SIGHUP)
	_ = h.goAhead.Close()
	h.expect(t, "cleaned up")

	if err := h.wait(t); err != nil {
		t.Errorf("the helper exited with %v, want a clean exit after its cleanup", err)
	}
}

// Mint catches SIGPIPE and SIGHUP rather than ignoring them, so a hook gets them at
// their default; a SIGHUP mint started with ignored (nohup) it leaves ignored, for
// itself and its hooks.
func TestRunContext_AHookGetsTheSignalsAsMintStartedWithThem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wrap []string
		want []string
	}{
		{name: "started as usual", want: []string{"PIPE killed", "HUP killed"}},
		{
			name: "started with SIGHUP ignored",
			wrap: []string{"sh", "-c", `trap "" HUP; exec "$@"`, "sh"},
			want: []string{"PIPE killed", "HUP survived"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := startSignalHelper(t, "hook", tt.wrap, nil)
			for _, want := range tt.want {
				h.expect(t, want)
			}

			if err := h.wait(t); err != nil {
				t.Errorf("the helper exited with %v, want a clean exit", err)
			}
		})
	}
}

// Once the run is stopping, a second Ctrl-C or SIGTERM ends mint at once: the escape
// hatch from a read the cancel can't reach.
func TestRunContext_ASecondInterruptEndsMintAtOnce(t *testing.T) {
	t.Parallel()

	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()

			h := startSignalHelper(t, "run", nil, nil)
			h.expect(t, "ready")
			h.signal(t, sig)
			h.expect(t, "stopped")

			h.signal(t, sig)
			err := h.wait(t)

			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != sig {
				t.Errorf("the helper exited with %v, want it killed by the second %v", err, sig)
			}
		})
	}
}
