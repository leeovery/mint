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
// Once the context is cancelled, Ctrl-C and SIGTERM go back to their default, so
// another terminates mint at once: the escape hatch for any read the cancellation
// cannot reach (the cooked-mode gate/AskLine stdin reads are not ctx-aware), without
// which every later Ctrl-C is swallowed, observed live as an unkillable gate. While
// mint waits for a hook to stop, the hooks runner catches that second signal itself,
// kills the hook and lets the unwind run.
//
// SIGHUP and SIGPIPE stay caught for the whole run, so a closed terminal stops the run
// the same way and doesn't kill mint mid-cleanup: a second SIGHUP (a closing terminal
// can send two) is dropped, and a write to a closed stdout or stderr fails with EPIPE,
// which the presenter discards, rather than raising SIGPIPE. They are caught, never
// ignored, because an ignored signal stays ignored in a hook across exec, where a
// caught one is reset to its default. A SIGHUP ignored when mint started (nohup) is
// left ignored, for mint and its hooks alike.
func runContext() (ctx context.Context, stopCatching func()) {
	ending := []os.Signal{os.Interrupt, syscall.SIGTERM}
	kept := []os.Signal{syscall.SIGPIPE}
	if !signal.Ignored(syscall.SIGHUP) {
		ending = append(ending, syscall.SIGHUP)
		kept = append(kept, syscall.SIGHUP)
	}
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, kept...)
	first := make(chan os.Signal, 1)
	signal.Notify(first, ending...)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-first:
		case <-ctx.Done():
		}
		// Stopped before the cancel, so whatever sees the context end sees the signals
		// back at their default already.
		signal.Stop(first)
		cancel()
	}()
	return ctx, func() {
		signal.Stop(first)
		signal.Stop(caught)
		cancel()
	}
}
