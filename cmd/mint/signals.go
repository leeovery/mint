package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// hookStopGrace is how long a hook the run's interrupt stops has from its SIGTERM to
// its SIGKILL: time for `press prepare` to stop its builds and notarisation, release
// its lock and say so.
const hookStopGrace = 60 * time.Second

// foldWindow is how soon after the first press mint folds a Ctrl-C or SIGTERM into it.
// A hook that signals its own process group as it stops (`trap 'kill 0' TERM`) reaches
// mint, in that group, within a millisecond of the first press; a person pressing
// Ctrl-C again takes far longer.
const foldWindow = 500 * time.Millisecond

// runContext is the ONE signal-cancellable context for the whole run, and
// stopCatching, which stops catching the signals. The first Ctrl-C (os.Interrupt),
// SIGTERM or SIGHUP (the terminal closing, an SSH session dropping) cancels it, which
// threads down through every command's external-command seam so a long pre-PONR step
// (the AI call, a hook, the gap before the atomic push) is interrupted. The engine
// treats a pre-PONR cancellation as a failure routed through its surgical unwind
// (resets, tag delete, autostash pop), so the repo is left clean; a post-PONR
// cancellation keeps the existing warn-only contract. A hook it interrupts is given
// time to stop (the hooks runner, hookStopGrace).
//
// Ctrl-C and SIGTERM stay caught for the whole run, so no second press ever meets
// their default by chance. One within fold of the first (mint's is foldWindow) is
// folded into it. The press after that is the second: where a hook is being stopped,
// killStopping kills it and the run's unwind goes on; where none is, and for any press
// after it, mint ends at once (die), the escape hatch for any wait the cancellation
// cannot reach.
//
// SIGHUP and SIGPIPE stay caught too, so a closed terminal stops the run the same way
// and doesn't kill mint mid-cleanup: a later SIGHUP (a closing terminal can send two)
// is dropped, and a write to a closed stdout or stderr fails with EPIPE, which the
// presenter discards, rather than raising SIGPIPE. Every signal is caught, never
// ignored, because an ignored signal stays ignored in a hook across exec, where a
// caught one is reset to its default. A SIGHUP ignored when mint started (nohup) is
// left ignored, for mint and its hooks alike.
func runContext(fold time.Duration, killStopping func() bool) (ctx context.Context, stopCatching func()) {
	caught := []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGPIPE}
	if !signal.Ignored(syscall.SIGHUP) {
		caught = append(caught, syscall.SIGHUP)
	}
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, caught...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go watchSignals(signals, done, cancel, fold, killStopping)
	return ctx, func() {
		signal.Stop(signals)
		close(done)
		cancel()
	}
}

// watchSignals acts on the run's signals until done, as runContext says.
func watchSignals(signals <-chan os.Signal, done <-chan struct{}, cancel func(), fold time.Duration, killStopping func() bool) {
	var first time.Time
	secondPressed := false
	for {
		var sig os.Signal
		select {
		case sig = <-signals:
		case <-done:
			return
		}
		switch {
		case sig == syscall.SIGPIPE:
		case first.IsZero():
			first = time.Now()
			cancel()
		case sig == syscall.SIGHUP, time.Since(first) < fold:
		case !secondPressed && killStopping():
			secondPressed = true
		default:
			die(sig)
		}
	}
}

// die ends mint at once on sig, as sig's default would, its unwind and close skipped.
func die(sig os.Signal) {
	signal.Reset(sig)
	if self, err := os.FindProcess(os.Getpid()); err == nil {
		_ = self.Signal(sig)
	}
	// sig ends mint within a moment, unless mint started with it ignored or it can't be
	// sent here; then mint exits as a failed run does.
	time.Sleep(time.Second)
	os.Exit(1)
}
