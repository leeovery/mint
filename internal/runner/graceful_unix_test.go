//go:build unix

package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"mint/internal/runner"
)

// helperLife is the longest a hook helper lives when nothing stops it, so a failing
// test leaves no process behind for long.
const helperLife = time.Minute

// TestHookHelper is not a test: the tests here run the test binary again as a hook,
// MINT_HOOK_HELPER saying how it behaves, and it leaves its marks in its working
// directory.
func TestHookHelper(t *testing.T) {
	mode := os.Getenv("MINT_HOOK_HELPER")
	if mode == "" {
		return
	}
	os.Exit(hookHelper(mode))
}

// hookHelper is a hook that, once marked ready, behaves on SIGTERM as mode says:
// trap stops in its own time and marks it stopped, refuse marks the SIGTERM and runs
// on, ignore never sees it. pgid prints its process group and exits.
func hookHelper(mode string) int {
	if mode == "pgid" {
		fmt.Print(syscall.Getpgrp())
		return 0
	}
	terms := make(chan os.Signal, 1)
	if mode == "ignore" {
		signal.Ignore(syscall.SIGTERM)
	} else {
		signal.Notify(terms, syscall.SIGTERM)
	}
	mark("ready")
	select {
	case <-terms:
	case <-time.After(helperLife):
		return 1
	}
	if mode == "trap" {
		time.Sleep(200 * time.Millisecond)
		mark("stopped")
		return 0
	}
	mark("sigterm")
	time.Sleep(helperLife)
	return 1
}

func mark(name string) { _ = os.WriteFile(name, nil, 0o600) }

// outcome is a hook run's end: its result, and when it returned.
type outcome struct {
	res runner.Result
	err error
	at  time.Time
}

// startHook runs the hook helper behaving as mode through r, from dir.
func startHook(t *testing.T, ctx context.Context, r *runner.ExecRunner, dir, mode string) <-chan outcome {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := r.RunInDir(ctx, dir, []string{"MINT_HOOK_HELPER=" + mode}, self, "-test.run=^TestHookHelper$")
		done <- outcome{res: res, err: err, at: time.Now()}
	}()
	return done
}

// waitForMark waits for the hook helper to leave the mark name in dir.
func waitForMark(t *testing.T, dir, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the hook never marked %s", name)
}

// awaitHook waits for the hook's run to return, for no longer than within.
func awaitHook(t *testing.T, done <-chan outcome, within time.Duration) outcome {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(within):
		t.Fatalf("the hook's run had not returned after %v", within)
		return outcome{}
	}
}

func marked(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// A hook that stops on SIGTERM gets the time it takes: mint waits for it to exit
// rather than killing it.
func TestGracefulExecRunner_AHookThatTrapsSIGTERMStopsInItsOwnTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := startHook(t, ctx, runner.NewGracefulExecRunner(helperLife), dir, "trap")
	waitForMark(t, dir, "ready")

	cancel()
	got := awaitHook(t, done, 30*time.Second)

	if !marked(dir, "stopped") {
		t.Error("the run returned before the hook had finished stopping on its SIGTERM")
	}
	if got.res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0: the hook exited on its own, not killed", got.res.ExitCode)
	}
}

// A hook that ignores SIGTERM is killed once the bound has passed, and not before.
func TestGracefulExecRunner_AHookThatIgnoresSIGTERMIsKilledAfterTheBound(t *testing.T) {
	t.Parallel()

	const grace = 300 * time.Millisecond
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := startHook(t, ctx, runner.NewGracefulExecRunner(grace), dir, "ignore")
	waitForMark(t, dir, "ready")

	cancelled := time.Now()
	cancel()
	got := awaitHook(t, done, 30*time.Second)

	if waited := got.at.Sub(cancelled); waited < grace {
		t.Errorf("the hook was killed %v after the cancel, want no sooner than the %v bound", waited, grace)
	}
	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a hook killed by a signal", got.res.ExitCode)
	}
}

// Interrupted again while it waits, mint kills the hook at once rather than waiting
// out the bound.
func TestGracefulExecRunner_AFurtherInterruptKillsTheHookAtOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	again := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(t.Context())
	done := startHook(t, ctx, runner.NewGracefulExecRunnerWatching(helperLife, again), dir, "refuse")
	waitForMark(t, dir, "ready")

	cancel()
	waitForMark(t, dir, "sigterm")
	again <- os.Interrupt
	got := awaitHook(t, done, helperLife/2)

	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a hook killed by a signal", got.res.ExitCode)
	}
}

// However the hook ends once the run's context does, on its SIGTERM or killed past
// the bound, the error says why the context ended, so a cancel or a timeout never
// reads as the hook's verdict.
func TestGracefulExecRunner_AStoppedHookReportsWhyTheRunStopped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
		// deadline leaves the run's context to time out, where it is otherwise cancelled
		// once the hook is ready.
		deadline bool
		want     error
	}{
		{name: "cancelled, the hook exits on its SIGTERM", mode: "trap", want: context.Canceled},
		{name: "cancelled, the hook killed past the bound", mode: "ignore", want: context.Canceled},
		{name: "timed out, the hook exits on its SIGTERM", mode: "trap", deadline: true, want: context.DeadlineExceeded},
		{name: "timed out, the hook killed past the bound", mode: "ignore", deadline: true, want: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.deadline {
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
			}
			done := startHook(t, ctx, runner.NewGracefulExecRunner(200*time.Millisecond), dir, tt.mode)
			if !tt.deadline {
				waitForMark(t, dir, "ready")
				cancel()
			}
			got := awaitHook(t, done, 30*time.Second)

			if !errors.Is(got.err, tt.want) {
				t.Errorf("error = %v, want it to match %v", got.err, tt.want)
			}
			if errors.Is(got.err, runner.ErrCommandNotFound) {
				t.Errorf("error = %v, a stopped hook must NOT match ErrCommandNotFound", got.err)
			}
		})
	}
}

// The hook runs in mint's own process group, so the terminal's Ctrl-C reaches it
// directly and a hook that reads the terminal keeps working.
func TestGracefulExecRunner_TheHookStaysInMintsProcessGroup(t *testing.T) {
	t.Parallel()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	r := runner.NewGracefulExecRunner(helperLife)

	res, err := r.RunInDir(t.Context(), t.TempDir(), []string{"MINT_HOOK_HELPER=pgid"}, self, "-test.run=^TestHookHelper$")

	if err != nil {
		t.Fatalf("RunInDir returned unexpected error: %v", err)
	}
	if got, want := strings.TrimSpace(res.Stdout), strconv.Itoa(syscall.Getpgrp()); got != want {
		t.Errorf("the hook's process group = %s, want mint's own, %s", got, want)
	}
}

// A hook that exits 0 but leaves something running with its output open has its
// verdict at the bound: the run succeeds with what it wrote, rather than waiting on
// what it left behind.
func TestGracefulExecRunner_AHookThatLeavesItsOutputOpenSucceedsAtTheBound(t *testing.T) {
	t.Parallel()

	r := runner.NewGracefulExecRunner(200 * time.Millisecond)
	started := time.Now()

	res, err := r.RunInDir(t.Context(), t.TempDir(), nil, "sh", "-c", "sleep 5 & printf done")

	if err != nil {
		t.Fatalf("RunInDir returned %v, want nil: the hook exited 0", err)
	}
	if res.Stdout != "done" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "done")
	}
	if took := time.Since(started); took >= 5*time.Second {
		t.Errorf("the run took %v, waiting on what the hook left running", took)
	}
}

// A short command keeps exec's stop: killed at once when its context ends, with no
// SIGTERM first.
func TestExecRunner_ACommandIsKilledAtOnceWithNoSIGTERM(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := startHook(t, ctx, runner.NewExecRunner(), dir, "refuse")
	waitForMark(t, dir, "ready")

	cancel()
	got := awaitHook(t, done, 30*time.Second)

	if marked(dir, "sigterm") {
		t.Error("the command was sent SIGTERM, want it killed at once")
	}
	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a command killed by a signal", got.res.ExitCode)
	}
}
