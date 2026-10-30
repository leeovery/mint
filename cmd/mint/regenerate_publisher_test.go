package main

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/notescache"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins the cmd-layer publisher resolution for the regenerate paths (task
// 10-2): `mint release regenerate <ver>` resolves the publisher through
// engine.ResolvePublisher and ACTS on the result rather than discarding the error and
// passing a nil interface down — the discard that crashed the downstream
// DispatchRelease on a non-github / no-remote origin. It resolves inside the run, so
// a downgrade's warning is drawn in the run it belongs to.

func TestRegenerate_UnresolvedProvider_WarnsInsideTheRunAndProceedsDowngraded(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	// `git remote get-url origin` fails → empty remote → ErrProviderUnresolved.
	seedRegenStartup(f, t.TempDir(), runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("no origin remote")})
	rec := &presentertest.RecordingPresenter{}
	// The release source can't be read without a provider, so the downgraded run stops
	// once its station is drawn, past the warning.
	req := parseRegenArgs(t, "1.4.0", "--source", "release", "--target", "release", "-y")

	if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	opened := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool { return ev.Kind == presentertest.KindRunStarted })
	warned := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
		return ev.Kind == presentertest.KindWarn && ev.Warn.Label == "publish skipped"
	})
	if opened != 0 || warned <= opened {
		t.Errorf("kinds = %v, want the run opened first, then the downgrade's warning (at %d)", rec.Kinds(), warned)
	}
	if got := stageFailedNames(rec); !slices.Equal(got, []string{"publish"}) {
		t.Errorf("failed stages = %v, want the release source's publish, the downgraded run having gone on", got)
	}
}

// stageFailedNames returns the Name of every StageFailed rec holds, in order.
func stageFailedNames(rec *presentertest.RecordingPresenter) []string {
	var names []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			names = append(names, ev.StageFailed.Name)
		}
	}
	return names
}
