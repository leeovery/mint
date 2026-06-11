package main

import (
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// TestResolveBatchAxes_NoFlags_PromptsSourceThenTarget proves a bare `--all` run (no
// source flag, no --target, no -y) ASKS for the source THEN the target before the batch
// runs — the same interactive idiom the single-version path uses. It must NOT silently
// default to fresh+release.
func TestResolveBatchAxes_NoFlags_PromptsSourceThenTarget(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			switch g.Subject {
			case "source":
				return presenter.Choice("fresh"), nil
			case "target":
				return presenter.Choice("both"), nil
			default:
				return presenter.ChoiceYes, nil
			}
		},
	}

	req := regenerateRequest{Source: sourceFresh, SourceSet: false, Target: targetUnset, All: true}
	source, target, err := resolveBatchAxes(rec, req, true)
	if err != nil {
		t.Fatalf("resolveBatchAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh", source)
	}
	if target != engine.RegenerateTargetBoth {
		t.Errorf("target = %v, want both", target)
	}
	if got, want := batchGateSubjects(rec), []string{"source", "target"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (a bare --all must ask source then target)", got, want)
	}
}

// TestResolveBatchAxes_ReuseAll_ForcesReleaseWithoutAsking proves `--reuse --all` forces
// target=release WITHOUT asking the target question (the 5-2 axis contract). The reuse
// flag also skips the source question. validateRegenerateRequest has already resolved the
// reuse-implied release target before this point, so the request arrives with both axes
// supplied — no gate should fire.
func TestResolveBatchAxes_ReuseAll_ForcesReleaseWithoutAsking(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}

	// As produced by parseRegenerateFlags + validateRegenerateRequest for `--reuse --all`:
	// SourceSet=true reuse, and the implied target release resolved up front.
	req := regenerateRequest{Source: sourceReuse, SourceSet: true, Target: targetRelease, All: true}
	source, target, err := resolveBatchAxes(rec, req, true)
	if err != nil {
		t.Fatalf("resolveBatchAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceReuse {
		t.Errorf("source = %v, want reuse", source)
	}
	if target != engine.RegenerateTargetRelease {
		t.Errorf("target = %v, want release (reuse forces release)", target)
	}
	if got := batchGateSubjects(rec); len(got) != 0 {
		t.Errorf("gate subjects = %v, want none (--reuse --all forces release without asking)", got)
	}
}

// TestResolveBatchAxes_SuppliedTarget_SkipsTargetPrompt proves a supplied --target (e.g.
// `--target both --all`, fresh source unset) skips the target prompt: only the source
// question fires.
func TestResolveBatchAxes_SuppliedTarget_SkipsTargetPrompt(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			return presenter.Choice("fresh"), nil
		},
	}

	req := regenerateRequest{Source: sourceFresh, SourceSet: false, Target: targetBoth, All: true}
	source, target, err := resolveBatchAxes(rec, req, true)
	if err != nil {
		t.Fatalf("resolveBatchAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh", source)
	}
	if target != engine.RegenerateTargetBoth {
		t.Errorf("target = %v, want both (supplied --target unchanged)", target)
	}
	if got, want := batchGateSubjects(rec), []string{"source"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (a supplied --target skips the target prompt)", got, want)
	}
}

// TestResolveBatchAxes_Yes_StillPromptsEveryAxis proves that under -y the resolver still
// calls Prompt at every applicable axis (the presenter models the skip+echo by returning
// the gate default), mirroring how the single-version path handles -y. The fresh -y +
// no --target combination is rejected earlier by validateRegenerateRequest, so here -y
// arrives with a supplied target; the source still prompts (auto-answered to fresh).
func TestResolveBatchAxes_Yes_StillPromptsEveryAxis(t *testing.T) {
	t.Parallel()

	// No PromptResult / NextChoices: each Prompt falls back to the gate default — exactly
	// how the recorder models the presenter-internal -y skip+echo.
	rec := &presentertest.RecordingPresenter{}

	req := regenerateRequest{Source: sourceFresh, SourceSet: false, Target: targetRelease, All: true, Yes: true}
	source, target, err := resolveBatchAxes(rec, req, true)
	if err != nil {
		t.Fatalf("resolveBatchAxes returned unexpected error: %v", err)
	}
	if source != engine.RegenerateSourceFresh {
		t.Errorf("source = %v, want fresh (the source gate default)", source)
	}
	if target != engine.RegenerateTargetRelease {
		t.Errorf("target = %v, want release (supplied)", target)
	}
	if got, want := batchGateSubjects(rec), []string{"source"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (-y still prompts the unset source axis)", got, want)
	}
}

// TestNewBatchBodyProducer_Reuse proves the batch body producer reads the tag
// annotation body verbatim for the reuse source — the per-version body the batch
// dispatches and collects — when no pre-read body is threaded (the single-version
// delegation), issuing exactly ONE read. The reuse read is git-only, so it is
// exercisable with the FakeRunner without an AI transport.
func TestNewBatchBodyProducer_Reuse(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "## reuse body\n"}, nil)

	produce := newBatchBodyProducer(f, config.Config{}, t.TempDir())
	body, err := produce(t.Context(), engine.RegenerateSourceReuse, version.Resolution{Tag: "v1.0.0"}, "")
	if err != nil {
		t.Fatalf("produce returned unexpected error: %v", err)
	}
	if body != "## reuse body\n" {
		t.Errorf("reuse body = %q, want the verbatim tag annotation body", body)
	}
	if got := len(f.Invocations()); got != 1 {
		t.Errorf("git invocations = %d, want exactly 1 annotation read", got)
	}
}

// TestNewBatchBodyProducer_Reuse_ConsumesPreReadBody proves the producer consumes the
// batch loop's pre-read annotation body for the reuse source WITHOUT reading the tag
// again — each tag is read once, by the loop's skip check.
func TestNewBatchBodyProducer_Reuse_ConsumesPreReadBody(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()

	produce := newBatchBodyProducer(f, config.Config{}, t.TempDir())
	body, err := produce(t.Context(), engine.RegenerateSourceReuse, version.Resolution{Tag: "v1.0.0"}, "## pre-read body\n")
	if err != nil {
		t.Fatalf("produce returned unexpected error: %v", err)
	}
	if body != "## pre-read body\n" {
		t.Errorf("reuse body = %q, want the threaded pre-read body verbatim", body)
	}
	if got := len(f.Invocations()); got != 0 {
		t.Errorf("git invocations = %d, want 0 (the pre-read body must not be re-read)", got)
	}
}

// TestNewBatchRegeneratorProducer_Reuse proves the batch regenerator producer returns
// NO regenerator for a reuse source: reuse runs the simple confirm (no review gate), so
// the `r` choice never applies.
func TestNewBatchRegeneratorProducer_Reuse(t *testing.T) {
	t.Parallel()

	produce := newBatchRegeneratorProducer(runner.NewFakeRunner(), config.Config{}, t.TempDir())
	if got := produce(engine.RegenerateSourceReuse, version.Resolution{Tag: "v1.0.0", PreviousTag: "v0.9.0"}); got != nil {
		t.Errorf("reuse regenerator = %v, want nil (reuse has no review gate)", got)
	}
}

// TestNewBatchRegeneratorProducer_Fresh proves the batch regenerator producer returns a
// non-nil regenerator for a fresh source bound to that version's resolved range, so the
// per-version `r` choice re-runs the fresh AI path (never errRegeneratorUnavailable).
func TestNewBatchRegeneratorProducer_Fresh(t *testing.T) {
	t.Parallel()

	produce := newBatchRegeneratorProducer(runner.NewFakeRunner(), config.Config{MaxDiffLines: 50000}, t.TempDir())
	if got := produce(engine.RegenerateSourceFresh, version.Resolution{Tag: "v1.1.0", PreviousTag: "v1.0.0"}); got == nil {
		t.Error("fresh regenerator = nil, want a non-nil per-version regenerator for the `r` choice")
	}
}

// batchGateSubjects returns the Subject of each recorded gate in order — the load-bearing
// way to assert the batch axis prompt sequence.
func batchGateSubjects(rec *presentertest.RecordingPresenter) []string {
	var subjects []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			subjects = append(subjects, ev.Prompt.Subject)
		}
	}
	return subjects
}
