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
	"sync"
	"sync/atomic"
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

// outcome is a run's end: its result, and when it returned.
type outcome struct {
	res runner.Result
	err error
	at  time.Time
}

// startRun runs name with args through r, from dir with env.
func startRun(ctx context.Context, r *runner.ExecRunner, dir string, env []string, name string, args ...string) <-chan outcome {
	done := make(chan outcome, 1)
	go func() {
		res, err := r.RunInDir(ctx, dir, env, name, args...)
		done <- outcome{res: res, err: err, at: time.Now()}
	}()
	return done
}

// startHook runs the hook helper behaving as mode through r, from dir.
func startHook(t *testing.T, ctx context.Context, r *runner.ExecRunner, dir, mode string) <-chan outcome {
	t.Helper()
	return startRun(ctx, r, dir, []string{"MINT_HOOK_HELPER=" + mode}, self(t), "-test.run=^TestHookHelper$")
}

func self(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return path
}

// noticing is ctx carrying a stop notice, and what the notice closes.
func noticing(ctx context.Context) (context.Context, <-chan struct{}) {
	noticed := make(chan struct{})
	return runner.WithStopNotice(ctx, func() { close(noticed) }), noticed
}

// killNoticing is ctx carrying a kill notice, and how many times it has been called.
func killNoticing(ctx context.Context) (context.Context, *atomic.Int32) {
	var calls atomic.Int32
	return runner.WithKillNotice(ctx, func() { calls.Add(1) }), &calls
}

// waitForMark waits for the hook helper to leave the mark name in dir.
func waitForMark(t *testing.T, dir, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if marked(dir, name) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the hook never marked %s", name)
}

func marked(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// await waits for what done says, for no longer than within.
func await[T any](t *testing.T, done <-chan T, within time.Duration, what string) T {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(within):
		t.Fatalf("%s had not happened after %v", what, within)
		var zero T
		return zero
	}
}

func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// A hook that stops on SIGTERM gets the time it takes: mint says so and waits for it to
// exit rather than killing it.
func TestGracefulExecRunner_AHookThatTrapsSIGTERMStopsInItsOwnTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	done := startHook(t, ctx, runner.NewGracefulExecRunner(helperLife), dir, "trap")
	waitForMark(t, dir, "ready")

	cancel()
	got := await(t, done, 30*time.Second, "the hook's run returning")

	if !isClosed(noticed) {
		t.Error("the run returned without its stop notice, though the hook was sent SIGTERM")
	}
	if !marked(dir, "stopped") {
		t.Error("the run returned before the hook had finished stopping on its SIGTERM")
	}
	if got.res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0: the hook exited on its own, not killed", got.res.ExitCode)
	}
}

// sh execs a hook entry that is a single command, so the command itself gets the
// SIGTERM and stops in its own time.
func TestGracefulExecRunner_ASingleCommandEntryGetsTheSIGTERMItself(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	entry := "'" + self(t) + "' -test.run=^TestHookHelper$"
	done := startRun(ctx, runner.NewGracefulExecRunner(helperLife), dir, []string{"MINT_HOOK_HELPER=trap"}, "sh", "-c", entry)
	waitForMark(t, dir, "ready")

	cancel()
	got := await(t, done, 30*time.Second, "the hook's run returning")

	if !marked(dir, "stopped") {
		t.Error("the command sh ran never stopped on a SIGTERM of its own")
	}
	if got.res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0: the command exited on its own, not killed", got.res.ExitCode)
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
	got := await(t, done, 30*time.Second, "the hook's run returning")

	if waited := got.at.Sub(cancelled); waited < grace {
		t.Errorf("the hook was killed %v after the cancel, want no sooner than the %v bound", waited, grace)
	}
	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a hook killed by a signal", got.res.ExitCode)
	}
}

// KillStopping, the second press, kills a hook mint waits on to stop at once rather
// than waiting out the bound.
func TestGracefulExecRunner_KillStoppingKillsTheHookAtOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := runner.NewGracefulExecRunner(helperLife)
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	done := startHook(t, ctx, r, dir, "refuse")
	waitForMark(t, dir, "ready")

	cancel()
	await(t, noticed, 30*time.Second, "the stop notice")
	if !r.KillStopping() {
		t.Error("KillStopping reported no command stopping, with the hook sent SIGTERM")
	}
	got := await(t, done, time.Second, "the hook's run returning")

	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a hook killed by a signal", got.res.ExitCode)
	}
}

// A hook KillStopping killed was forced: its run calls the kill notice once, on its
// own goroutine rather than KillStopping's, before it returns, and its error says the
// run was cancelled and the hook forced.
func TestGracefulExecRunner_AHookKillStoppingKilledIsForced(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := runner.NewGracefulExecRunner(helperLife)
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	var calls atomic.Int32
	killNoticed, release := make(chan struct{}), make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseOnce)
	ctx = runner.WithKillNotice(ctx, func() {
		if calls.Add(1) == 1 {
			close(killNoticed)
		}
		<-release
	})
	done := startHook(t, ctx, r, dir, "refuse")
	waitForMark(t, dir, "ready")

	cancel()
	await(t, noticed, 30*time.Second, "the stop notice")
	killed := make(chan bool, 1)
	go func() { killed <- r.KillStopping() }()
	if !await(t, killed, time.Second, "KillStopping returning while the kill notice is held") {
		t.Error("KillStopping reported no command stopping, with the hook sent SIGTERM")
	}
	await(t, killNoticed, time.Second, "the kill notice")
	select {
	case <-done:
		t.Error("the run returned before its kill notice had")
	default:
	}
	releaseOnce()
	got := await(t, done, time.Second, "the hook's run returning")

	if n := calls.Load(); n != 1 {
		t.Errorf("kill notice calls = %d, want 1", n)
	}
	if !errors.Is(got.err, context.Canceled) {
		t.Errorf("error = %v, want it to match context.Canceled", got.err)
	}
	if !errors.Is(got.err, runner.ErrForced) {
		t.Errorf("error = %v, want it to match ErrForced", got.err)
	}
}

// A hook that stopped on its SIGTERM was not forced: no kill notice, and its error
// says only that the run was cancelled.
func TestGracefulExecRunner_AHookThatStoppedOnItsSIGTERMIsNotForced(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	ctx, calls := killNoticing(ctx)
	done := startHook(t, ctx, runner.NewGracefulExecRunner(helperLife), dir, "trap")
	waitForMark(t, dir, "ready")

	cancel()
	got := await(t, done, 30*time.Second, "the hook's run returning")

	if n := calls.Load(); n != 0 {
		t.Errorf("kill notice calls = %d, want none for a hook that stopped on its SIGTERM", n)
	}
	if !errors.Is(got.err, context.Canceled) {
		t.Errorf("error = %v, want it to match context.Canceled", got.err)
	}
	if errors.Is(got.err, runner.ErrForced) {
		t.Errorf("error = %v, a hook that stopped on its SIGTERM must NOT match ErrForced", got.err)
	}
}

// KillStopping ends the wait at once even where something the hook started, which
// ignores SIGTERM, still holds the hook's output open.
func TestGracefulExecRunner_KillStoppingEndsTheWaitThoughAGrandchildHoldsTheOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := runner.NewGracefulExecRunner(helperLife)
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	done := startRun(ctx, r, dir, nil, "sh", "-c", `trap "" TERM; sleep 60 & echo $! > grandchild; : > ready; wait`)
	waitForMark(t, dir, "ready")
	t.Cleanup(func() { killGrandchild(dir) })

	cancel()
	await(t, noticed, 30*time.Second, "the stop notice")
	r.KillStopping()
	got := await(t, done, time.Second, "the hook's run returning")

	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a hook killed by a signal", got.res.ExitCode)
	}
}

// killGrandchild kills the process whose pid the hook left in dir.
func killGrandchild(dir string) {
	pid, err := os.ReadFile(filepath.Join(dir, "grandchild"))
	if err != nil {
		return
	}
	if n, err := strconv.Atoi(strings.TrimSpace(string(pid))); err == nil {
		_ = syscall.Kill(n, syscall.SIGKILL)
	}
}

// waitForExit waits for the process whose pid the hook left in dir's file name to be
// gone, its exit reaped, so the run has had the hook's exit.
func waitForExit(t *testing.T, dir, name string) {
	t.Helper()
	waitForMark(t, dir, name)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pid, err := os.ReadFile(filepath.Join(dir, name))
		n, convErr := strconv.Atoi(strings.TrimSpace(string(pid)))
		if err == nil && convErr == nil && errors.Is(syscall.Kill(n, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the hook never exited")
}

// With nothing stopping, KillStopping has nothing to kill and says so: a command that
// has returned is not stopping, nor is one running whose run's context goes on.
func TestExecRunner_KillStoppingWithNothingStoppingReportsNone(t *testing.T) {
	t.Parallel()

	r := runner.NewGracefulExecRunner(helperLife)
	if _, err := r.Run(t.Context(), "true"); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := startHook(t, ctx, r, dir, "trap")
	waitForMark(t, dir, "ready")

	if r.KillStopping() {
		t.Error("a graceful runner with nothing stopping reported a command killed")
	}
	if runner.NewExecRunner().KillStopping() {
		t.Error("a runner that kills at once reported a command killed")
	}
	cancel()
	await(t, done, 30*time.Second, "the hook's run returning")
	if !marked(dir, "stopped") {
		t.Error("KillStopping killed a hook whose run had not been interrupted")
	}
}

// The stop notice is only for a hook sent SIGTERM: a hook that never started, or that
// had exited when the run's context ended, gets none. A cancel while mint still reads
// the output of a hook that has exited leaves that read to go on; KillStopping ends it,
// forcing no hook, so no kill notice either.
func TestGracefulExecRunner_NoStopNoticeForAHookNotRunning(t *testing.T) {
	t.Parallel()

	t.Run("never started", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		ctx, noticed := noticing(ctx)
		cancel()

		_, err := runner.NewGracefulExecRunner(helperLife).RunInDir(ctx, t.TempDir(), nil, "true")

		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want it to match context.Canceled", err)
		}
		if isClosed(noticed) {
			t.Error("a hook that never started got a stop notice")
		}
	})
	for _, exit := range []int{0, 3} {
		t.Run(fmt.Sprintf("exited %d, its output still held", exit), func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			ctx, cancel := context.WithCancel(t.Context())
			ctx, noticed := noticing(ctx)
			ctx, killCalls := killNoticing(ctx)
			r := runner.NewGracefulExecRunner(helperLife)
			done := startRun(ctx, r, dir, nil, "sh", "-c", fmt.Sprintf("sleep 60 & echo $! > grandchild; echo $$ > hook; exit %d", exit))
			t.Cleanup(func() { killGrandchild(dir) })
			waitForExit(t, dir, "hook")

			cancel()
			if !r.KillStopping() {
				t.Error("KillStopping found nothing stopping while mint read the hook's output")
			}
			got := await(t, done, time.Second, "the hook's run returning")

			if exit == 0 && got.err != nil {
				t.Errorf("error = %v, want nil: the hook exited 0 before the cancel", got.err)
			}
			if errors.Is(got.err, runner.ErrForced) {
				t.Errorf("error = %v, a hook that had exited must NOT match ErrForced", got.err)
			}
			if isClosed(noticed) {
				t.Error("a hook that had exited got a stop notice")
			}
			if n := killCalls.Load(); n != 0 {
				t.Errorf("kill notice calls = %d, want none: the kill reached no hook, only its output", n)
			}
		})
	}
}

// A hook the terminal's Ctrl-C killed before mint acted on it got no SIGTERM, and its
// output is read to the end: what is still in the pipe, and what it left running
// prints, reach mint.
func TestGracefulExecRunner_AHookThatDiedBeforeItsSIGTERMHasItsOutputReadToTheEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	done := startRun(ctx, runner.NewGracefulExecRunner(helperLife), dir, nil, "sh", "-c",
		`(while [ ! -e go ]; do sleep 0.01; done; echo late) & echo $! > grandchild; echo early; echo $$ > hook; kill -INT $$`)
	t.Cleanup(func() { killGrandchild(dir) })
	waitForExit(t, dir, "hook")

	cancel()
	// Time for a read the cancel cut short to have ended before the late output.
	time.Sleep(100 * time.Millisecond)
	mark(filepath.Join(dir, "go"))
	got := await(t, done, 30*time.Second, "the hook's run returning")

	if got.res.Stdout != "early\nlate\n" {
		t.Errorf("Stdout = %q, want %q: all the hook's output", got.res.Stdout, "early\nlate\n")
	}
	if !errors.Is(got.err, context.Canceled) {
		t.Errorf("error = %v, want it to match context.Canceled", got.err)
	}
	if isClosed(noticed) {
		t.Error("a hook sent no SIGTERM got a stop notice")
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
			got := await(t, done, 30*time.Second, "the hook's run returning")

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

	r := runner.NewGracefulExecRunner(helperLife)

	res, err := r.RunInDir(t.Context(), t.TempDir(), []string{"MINT_HOOK_HELPER=pgid"}, self(t), "-test.run=^TestHookHelper$")

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

// A short command keeps exec's stop: its runner gives it no Cancel of its own, so it is
// killed at once when its context ends, with no SIGTERM first.
func TestExecRunner_ACommandIsKilledAtOnceWithNoSIGTERM(t *testing.T) {
	t.Parallel()

	r := runner.NewExecRunner()
	if runner.StopsGracefully(r) {
		t.Fatal("the runner for short commands gives them a Cancel of its own, want exec's kill")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	ctx, noticed := noticing(ctx)
	done := startHook(t, ctx, r, dir, "refuse")
	waitForMark(t, dir, "ready")

	cancel()
	got := await(t, done, 30*time.Second, "the command's run returning")

	if marked(dir, "sigterm") || isClosed(noticed) {
		t.Error("the command was sent SIGTERM, want it killed at once")
	}
	if got.res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for a command killed by a signal", got.res.ExitCode)
	}
}
