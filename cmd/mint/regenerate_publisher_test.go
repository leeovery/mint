package main

import (
	"errors"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins the cmd-layer publisher resolution for the regenerate paths (task
// 10-2): both `mint release regenerate <ver>` and `--all` must resolve the publisher
// through engine.ResolvePublisher and ACT on the result rather than discarding the error
// (`publisher, _ := …`) and passing a nil interface down — the discard that crashed the
// downstream DispatchRelease with a nil-pointer dereference on a non-github / no-remote
// origin.

// regenDeps builds the minimal ReleaseDeps the cmd publisher resolution consumes — the
// recording presenter (for the downgrade warn) and the FakeRunner (for the remote read).
func regenDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	return engine.ReleaseDeps{Presenter: rec, Runner: f}
}

// TestResolveRegeneratePublisher_Unresolved_DowngradesProceeds proves that an
// unresolvable provider (no remote / non-github origin) downgrades CLEANLY: the cmd helper
// reports proceed=true with a NIL publisher (so the engine write skips the provider
// surface) and exitCode 0 — never a crash, never an abort.
func TestResolveRegeneratePublisher_Unresolved_DowngradesProceeds(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	// `git remote get-url origin` fails → empty remote → ErrProviderUnresolved.
	f.Seed("git", runner.Result{ExitCode: 1}, errors.New("no origin remote"))
	rec := &presentertest.RecordingPresenter{}

	publisher, code, proceed := resolveRegeneratePublisher(t.Context(), regenDeps(rec, f), config.Config{})

	if !proceed {
		t.Fatalf("proceed = false (code %d), want true — an unresolved provider downgrades, it does not abort", code)
	}
	if publisher != nil {
		t.Errorf("publisher = %v, want nil (downgrade skips the provider write)", publisher)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 on a clean downgrade", code)
	}
}

// TestResolveRegeneratePublisher_Resolved_ReturnsDriver proves a github origin resolves a
// real driver and proceeds with it — the normal path is unaffected by the guard.
func TestResolveRegeneratePublisher_Resolved_ReturnsDriver(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "https://github.com/acme/widget.git\n"}, nil)
	rec := &presentertest.RecordingPresenter{}

	publisher, code, proceed := resolveRegeneratePublisher(t.Context(), regenDeps(rec, f), config.Config{})

	if !proceed || code != 0 {
		t.Fatalf("proceed=%v code=%d, want proceed=true code=0 for a resolvable github origin", proceed, code)
	}
	if publisher == nil {
		t.Error("publisher = nil, want a resolved GitHub driver for a github.com origin")
	}
}
