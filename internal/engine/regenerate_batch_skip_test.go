package engine_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 5-12: batch `--all` SKIP-AND-CONTINUE + END SUMMARY. A
// per-version FAILURE (notes/diff-too-large; `--reuse --all` against a body-less tag)
// is CAUGHT, RECORDED with a human reason, and the loop CONTINUES — consciously
// OVERRIDING the single-version on_notes_failure=abort default. A CONFIG-level fact
// (a changelog/both target with changelog=false) aborts the WHOLE batch UP FRONT,
// before any version is processed. The run closes with one RunFinished carrying a
// VerbRegenerate, URL-less, engine-computed Summary that lists the regenerated count
// and each skipped version with its reason (so the user re-runs stragglers); the
// presenter renders that Summary VERBATIM.

// freshBodyOrDiffTooLarge returns a ProduceBody that yields the version's fresh body
// EXCEPT for failTag, where it returns a wrapped notes.ErrDiffTooLarge — the
// per-version notes failure the batch must SKIP rather than abort on.
func freshBodyOrDiffTooLarge(failTag string) func(context.Context, engine.RegenerateSource, version.Resolution) (string, error) {
	return func(_ context.Context, _ engine.RegenerateSource, res version.Resolution) (string, error) {
		if res.Tag == failTag {
			return "", fmt.Errorf("%w: diff exceeds max_diff_lines (9000 > 800)", notes.ErrDiffTooLarge)
		}
		return "## fresh " + res.Tag + "\n", nil
	}
}

// reuseBatchReq builds a reuse `--all` request whose ProduceBody is the canned
// per-version reuse body — the body that flows AFTER the engine's ReadTagBody
// has-body check passes.
func reuseBatchReq(versions []version.Resolution, yes bool) engine.BatchRegenerateRequest {
	return batchReq(engine.RegenerateSourceReuse, versions, yes)
}

// finishEvent returns the single RunFinished the recorder captured, failing if none
// (or more than one) fired — a batch closes with exactly one end summary.
func finishEvent(t *testing.T, rec *presentertest.RecordingPresenter) presenter.RunResult {
	t.Helper()
	var results []presenter.RunResult
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindRunFinished {
			results = append(results, ev.RunFinished)
		}
	}
	if len(results) != 1 {
		t.Fatalf("recorded %d RunFinished events, want exactly 1; kinds = %v", len(results), rec.Kinds())
	}
	return results[0]
}

// TestRegenerateAll_DiffTooLargeSkippedBatchContinues proves a per-version notes
// failure (diff too large) is SKIPPED and the batch CONTINUES: the other versions
// still dispatch, only the failing one is absent.
func TestRegenerateAll_DiffTooLargeSkippedBatchContinues(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = freshBodyOrDiffTooLarge(batchV2Tag)

	bodies, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req)
	if err != nil {
		t.Fatalf("a per-version failure must not abort the batch; got error: %v", err)
	}

	// The failing version is skipped; the other two still dispatch, in order.
	wantTags := []string{batchV1Tag, batchV3Tag}
	if got := dispatchedTags(pub); !slices.Equal(got, wantTags) {
		t.Errorf("dispatched %v, want the batch to skip %s and continue with %v", got, batchV2Tag, wantTags)
	}
	// Only the surviving versions are collected for the 5-13 rebuild.
	if len(bodies) != 2 {
		t.Fatalf("collected %d bodies, want 2 (the skipped version is not collected)", len(bodies))
	}
}

// TestRegenerateAll_DiffTooLargeSummaryReports proves the skipped version appears in
// the end summary with its reason, and the regenerated count reflects only the
// dispatched versions.
func TestRegenerateAll_DiffTooLargeSummaryReports(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = freshBodyOrDiffTooLarge(batchV2Tag)

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	fin := finishEvent(t, rec)
	if fin.Verb != presenter.VerbRegenerate {
		t.Errorf("RunFinished.Verb = %v, want VerbRegenerate", fin.Verb)
	}
	if fin.Project != batchProject {
		t.Errorf("RunFinished.Project = %q, want %q", fin.Project, batchProject)
	}
	if fin.URL != "" {
		t.Errorf("RunFinished.URL = %q, want empty (regenerate omits the URL entirely)", fin.URL)
	}
	want := "2 regenerated, 1 skipped: " + batchV2Tag + " (diff too large)"
	if fin.Summary != want {
		t.Errorf("RunFinished.Summary = %q, want %q", fin.Summary, want)
	}
}

// TestRegenerateAll_ReuseNoAnnotationBodySkipped proves `--reuse --all` against a tag
// with NO annotation body is SKIPPED + reported (NOT the single-mode fail-loud
// error): the batch continues and the body-less version lands in the skipped list.
func TestRegenerateAll_ReuseNoAnnotationBodySkipped(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	// The reuse path reads each tag's annotation body via for-each-ref, oldest →
	// newest. v1 and v3 carry a body; v2 is body-less (lightweight / empty), so it is
	// skipped rather than written as an empty release.
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "## v1 body\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "   \n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "## v3 body\n"}},
	)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		reuseBatchReq(threeVersions(), true)); err != nil {
		t.Fatalf("a body-less tag must be skipped, not error the batch; got: %v", err)
	}

	// The batch continues: v1 and v3 dispatch; v2 is skipped.
	wantTags := []string{batchV1Tag, batchV3Tag}
	if got := dispatchedTags(pub); !slices.Equal(got, wantTags) {
		t.Errorf("dispatched %v, want %v (the body-less tag skipped)", got, wantTags)
	}

	// The end summary names the skipped version with the --all reason variant.
	fin := finishEvent(t, rec)
	want := "2 regenerated, 1 skipped: " + batchV2Tag + " (no annotation body — use --fresh)"
	if fin.Summary != want {
		t.Errorf("RunFinished.Summary = %q, want %q", fin.Summary, want)
	}
}

// TestRegenerateAll_SkipOverridesOnNotesFailureAbort proves per-version
// skip-and-continue OVERRIDES the single-version on_notes_failure=abort default: a
// per-version notes failure does NOT abort the batch — RegenerateAll returns no
// error and the run still closes with a success RunFinished.
func TestRegenerateAll_SkipOverridesOnNotesFailureAbort(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	// The OLDEST version fails — under the single-version abort default this would kill
	// the run before any later version. Skip-and-continue overrides that.
	req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
	req.ProduceBody = freshBodyOrDiffTooLarge(batchV1Tag)

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, req); err != nil {
		t.Fatalf("skip-and-continue must override on_notes_failure=abort; got error: %v", err)
	}

	// The two later versions still dispatch despite the first one failing.
	wantTags := []string{batchV2Tag, batchV3Tag}
	if got := dispatchedTags(pub); !slices.Equal(got, wantTags) {
		t.Errorf("dispatched %v, want %v (the oldest failure must not abort the rest)", got, wantTags)
	}
	// The run still closes successfully.
	if fin := finishEvent(t, rec); fin.Verb != presenter.VerbRegenerate {
		t.Errorf("RunFinished.Verb = %v, want VerbRegenerate", fin.Verb)
	}
}

// TestRegenerateAll_AllRegeneratedNoneSkippedSummary proves the all-success summary
// reads as a plain count with NO skipped clause.
func TestRegenerateAll_AllRegeneratedNoneSkippedSummary(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "## body\n"}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub,
		reuseBatchReq(threeVersions(), true)); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	fin := finishEvent(t, rec)
	if want := "3 regenerated"; fin.Summary != want {
		t.Errorf("RunFinished.Summary = %q, want %q (no skipped clause when all succeed)", fin.Summary, want)
	}
}

// TestRegenerateAllValidated_ConfigErrorAbortsUpFront proves a CONFIG-level error (a
// changelog/both target with changelog=false) aborts the WHOLE batch UP FRONT —
// before any version is processed: no version dispatches and NO end summary fires.
func TestRegenerateAllValidated_ConfigErrorAbortsUpFront(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	// changelog disabled + a changelog-touching target is a static config fact.
	configReq := reuseBatchReq(threeVersions(), true)
	configReq.Target = engine.RegenerateTargetBoth
	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, t.TempDir(), configReq, false)

	assertAbortNonZero(t, err)
	// The abort carries the single owned engine.ErrChangelogDisabled sentinel — the
	// same one the cmd-layer validator returns — so the changelog-disabled wording is
	// pinned to one symbol across the single and batch paths.
	if !errors.Is(err, engine.ErrChangelogDisabled) {
		t.Errorf("config abort error = %v, want errors.Is(err, engine.ErrChangelogDisabled)", err)
	}
	if len(pub.dispatched) != 0 {
		t.Errorf("a config-level error dispatched %+v; the batch must abort before any version", pub.dispatched)
	}
	if len(runStartedVersions(rec)) != 0 {
		t.Errorf("a config-level error opened %d narration blocks; the batch must abort before any version", len(runStartedVersions(rec)))
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindRunFinished {
			t.Errorf("a config-level abort still emitted an end summary; an aborted batch emits no success line")
		}
	}
}

// TestRegenerateAllValidated_ValidConfigRunsBatch proves the up-front validation is a
// pass-through when the target is config-valid: the batch runs normally and closes
// with the end summary.
func TestRegenerateAllValidated_ValidConfigRunsBatch(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "## body\n"}, nil)
	f.Seed("gh", runner.Result{}, nil) // entry-point preflight gh-auth (release target)
	pub := newFakePublisher()
	pub.seedExists(batchV1Tag, true, nil)
	pub.seedExists(batchV2Tag, true, nil)
	pub.seedExists(batchV3Tag, true, nil)
	rec := &presentertest.RecordingPresenter{}

	// reuse forces a release-only target, which is config-valid even with changelog off.
	validReq := reuseBatchReq(threeVersions(), true)
	validReq.Target = engine.RegenerateTargetRelease
	if err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), pub, t.TempDir(), validReq, false); err != nil {
		t.Fatalf("a config-valid target must run the batch; got error: %v", err)
	}

	if len(pub.dispatched) != 3 {
		t.Errorf("dispatched %d, want 3 (the batch runs when config is valid)", len(pub.dispatched))
	}
	if fin := finishEvent(t, rec); fin.Summary != "3 regenerated" {
		t.Errorf("RunFinished.Summary = %q, want %q", fin.Summary, "3 regenerated")
	}
}
