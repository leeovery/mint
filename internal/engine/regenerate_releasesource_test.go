package engine_test

import (
	"context"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins the provider-release SOURCE (--source release): reading the EXISTING
// provider release body back through the Publisher seam and re-projecting it onto a
// surface. Like reuse it is deterministic — the simple confirm, no review gate — and it
// composes with every target (the orthogonal axes). A body-less release is a per-version
// SKIP in --all and a fail-loud in single mode; the source is impossible without a
// resolved provider, so a nil publisher aborts up front.

// echoPreReadBody returns a ProduceBody that yields the loop's pre-read body unchanged —
// the production wiring for a deterministic source (the batch pre-reads the body for its
// skip check and threads it through, so the producer consumes it rather than re-reading).
func echoPreReadBody() func(context.Context, engine.RegenerateSource, version.Resolution, string) (string, error) {
	return func(_ context.Context, _ engine.RegenerateSource, _ version.Resolution, preReadBody string) (string, error) {
		return preReadBody, nil
	}
}

// TestRegenerateAll_ReleaseSource_ReadsBodyAndSkipsBodyless proves the --all
// provider-release source reads each version's published release body through the
// Publisher seam and dispatches it, while a version whose release has NO body is SKIPPED
// (the --all skip, not a fail-loud) with the release-body reason — the batch continues.
func TestRegenerateAll_ReleaseSource_ReadsBodyAndSkipsBodyless(t *testing.T) {
	t.Parallel()

	pub := newFakePublisher()
	// v1 and v3 carry a published release body; v2's release is body-less (absent/empty).
	pub.seedReleaseBody(batchV1Tag, "## release v1\n", true, nil)
	pub.seedReleaseBody(batchV2Tag, "", false, nil)
	pub.seedReleaseBody(batchV3Tag, "## release v3\n", true, nil)
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceRelease, threeVersions(), true)
	req.ProduceBody = echoPreReadBody() // consume the pre-read release body, as production does

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req); err != nil {
		t.Fatalf("a body-less release must be skipped, not error the batch; got: %v", err)
	}

	// v1 and v3 dispatch with the READ release body; v2 is skipped.
	if got, want := dispatchedTags(pub), []string{batchV1Tag, batchV3Tag}; !slices.Equal(got, want) {
		t.Errorf("dispatched %v, want %v (the body-less release skipped)", got, want)
	}
	if pub.dispatched[0].body != "## release v1\n" {
		t.Errorf("dispatched body = %q, want the read release body flowed through", pub.dispatched[0].body)
	}
	fin := finishEvent(t, rec)
	want := "2 regenerated, 1 skipped: " + batchV2Tag + " (no release body — use --source fresh)"
	if fin.Summary != want {
		t.Errorf("RunFinished.Summary = %q, want %q", fin.Summary, want)
	}
}

// TestRegenerateAllValidated_ReleaseSource_NilPublisherAbortsUpFront proves a --all
// provider-release source on a DOWNGRADED run (nil publisher) aborts the WHOLE batch UP
// FRONT — before any version — surfacing a "publish" StageFailed, since reading the
// release is impossible without a resolved provider.
func TestRegenerateAllValidated_ReleaseSource_NilPublisherAbortsUpFront(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceRelease, threeVersions(), true)
	req.Target = engine.RegenerateTargetRelease
	req.ReleaseBranch = regenReleaseBranch

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), nil, t.TempDir(), req, true)

	assertAbortNonZero(t, err)
	if stageFailedName(t, rec) != "publish" {
		t.Errorf("StageFailed = %q, want \"publish\"", stageFailedName(t, rec))
	}
	if len(runStartedVersions(rec)) != 0 {
		t.Errorf("the release-source nil-publisher abort opened %d narration blocks; it must abort before any version", len(runStartedVersions(rec)))
	}
}

// TestRegenerateRun_ReleaseSource_SimpleConfirmAndDispatch proves the single-version
// provider-release source runs the simple two-choice confirm (no e/r review gate, like
// reuse) and dispatches the provider release.
func TestRegenerateRun_ReleaseSource_SimpleConfirmAndDispatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("gh", runner.Result{}, nil) // resolved target is the provider release → gh-auth preflight gate
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceRelease), engine.TargetOf(engine.RegenerateTargetRelease), false))
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	confirm := confirmGate(t, rec)
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}
	if !slices.Equal(confirm.Keys(), wantKeys) {
		t.Errorf("release-source confirm keys = %v, want the simple confirm %v (no e/r)", confirm.Keys(), wantKeys)
	}
	if len(pub.dispatched) != 1 {
		t.Errorf("dispatched %d, want 1 (the release source writes the provider)", len(pub.dispatched))
	}
}

// TestRegenerateRun_ReleaseSource_NilPublisherFailsLoud proves the single-version
// provider-release source on a DOWNGRADED run (nil publisher) fails loud — a "publish"
// StageFailed and a non-zero abort — rather than nil-dereferencing the seam.
func TestRegenerateRun_ReleaseSource_NilPublisherFailsLoud(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), nil, dir,
		runReq(engine.SourceOf(engine.RegenerateSourceRelease), engine.TargetOf(engine.RegenerateTargetRelease), false))

	assertAbortNonZero(t, err)
	if stageFailedName(t, rec) != "publish" {
		t.Errorf("StageFailed = %q, want \"publish\"", stageFailedName(t, rec))
	}
}
