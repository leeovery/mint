package engine_test

import (
	"context"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 7-3's regenerate half: the regenerate orchestrators emit the
// equivalent StageStarted/StageSucceeded narration for their sequenced blocking
// stages — body production (fresh re-diff + AI) on the interactive run, and the
// changelog push (the regenerate point of no return) on the write path.

// TestRegenerateRun_EmitsBlockingNotesStage proves the interactive run narrates body
// PRODUCTION as a blocking stage: a StageStarted(Blocking:true) before ProduceBody
// and a StageSucceeded(Blocking:true, engine-measured Elapsed) after it, both BEFORE
// the start-of-run header (the body is produced before the plan/confirm).
func TestRegenerateRun_EmitsBlockingNotesStage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	start, ok := stageStarted(rec, "notes")
	if !ok {
		t.Fatalf("no StageStarted for the regenerate notes stage; kinds = %v", rec.Kinds())
	}
	if !start.Blocking {
		t.Errorf("regenerate notes StageStarted.Blocking = false, want true")
	}
	done, ok := stageSucceeded(rec, "notes")
	if !ok {
		t.Fatalf("no StageSucceeded for the regenerate notes stage; kinds = %v", rec.Kinds())
	}
	if !done.Blocking {
		t.Errorf("regenerate notes StageSucceeded.Blocking = false, want true")
	}
	if done.Elapsed < 0 {
		t.Errorf("regenerate notes StageSucceeded.Elapsed = %v, want non-negative", done.Elapsed)
	}
	notesStartAt := indexOfStage(rec, presentertest.KindStageStarted, "notes")
	notesDoneAt := indexOfStage(rec, presentertest.KindStageSucceeded, "notes")
	if notesStartAt >= notesDoneAt {
		t.Errorf("notes StageStarted (%d) must precede StageSucceeded (%d)", notesStartAt, notesDoneAt)
	}

	// RunStarted OPENS the block: it must precede the first stage narration event
	// (the blocking notes stage), mirroring the batch processOneVersion ordering
	// (RunStarted → plan → notes stage). This pins "RunStarted first" for the
	// interactive regenerate run at the engine layer.
	runStartedAt := indexOfKind(rec, presentertest.KindRunStarted)
	if runStartedAt == -1 {
		t.Fatalf("no RunStarted recorded; kinds = %v", rec.Kinds())
	}
	if firstStageAt := firstStageEventIndex(rec); runStartedAt >= firstStageAt {
		t.Errorf("RunStarted at %d must precede the first stage event at %d (RunStarted opens the block)", runStartedAt, firstStageAt)
	}
}

// TestRegenerateRun_BodyProductionFailure_NoNotesSuccess proves a body-production
// failure surfaces the StageFailed (record it) and emits NO StageSucceeded for the
// notes stage — the blocking-stage success narration is for the success path only.
func TestRegenerateRun_BodyProductionFailure_NoNotesSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
	req.ProduceBody = func(context.Context, engine.RegenerateSource) (string, error) {
		return "", context.DeadlineExceeded
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err == nil {
		t.Fatal("RegenerateRun returned nil error for a body-production failure, want an abort")
	}

	if _, ok := stageSucceeded(rec, "notes"); ok {
		t.Errorf("StageSucceeded(notes) emitted on a body-production FAILURE; success narration is success-only; kinds = %v", rec.Kinds())
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("no StageFailed for the failed body production; kinds = %v", rec.Kinds())
	}
}

// TestRegenerateWrite_EmitsBlockingPushStage proves the changelog write path narrates
// the PUSH (the regenerate point of no return) as a blocking stage: a
// StageStarted(Blocking:true) before `git push origin HEAD` and a
// StageSucceeded(Blocking:true) after it crosses the PONR.
func TestRegenerateWrite_EmitsBlockingPushStage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD (capture clean start)
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short (historical date)
		ScriptedOut(""),                   // -C dir add CHANGELOG.md
		ScriptedOut(""),                   // -C dir commit -m docs(changelog): ...
		ScriptedOut(""),                   // push origin HEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	start, ok := stageStarted(rec, "push")
	if !ok {
		t.Fatalf("no StageStarted for the regenerate push stage; kinds = %v", rec.Kinds())
	}
	if !start.Blocking {
		t.Errorf("regenerate push StageStarted.Blocking = false, want true")
	}
	done, ok := stageSucceeded(rec, "push")
	if !ok {
		t.Fatalf("no StageSucceeded for the regenerate push stage; kinds = %v", rec.Kinds())
	}
	if !done.Blocking {
		t.Errorf("regenerate push StageSucceeded.Blocking = false, want true")
	}
	pushStartAt := indexOfStage(rec, presentertest.KindStageStarted, "push")
	pushDoneAt := indexOfStage(rec, presentertest.KindStageSucceeded, "push")
	if pushStartAt >= pushDoneAt {
		t.Errorf("push StageStarted (%d) must precede StageSucceeded (%d)", pushStartAt, pushDoneAt)
	}
}

// TestRegenerateWrite_NoChangelogTarget_OmitsPushStage proves the push stage narrates
// ONLY when a commit is pushed: a release-only run (no changelog commit, no push)
// emits no push StageStarted/StageSucceeded.
func TestRegenerateWrite_NoChangelogTarget_OmitsPushStage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, freshWriteReq(engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	if _, ok := stageStarted(rec, "push"); ok {
		t.Errorf("push StageStarted emitted on a release-only run with no changelog push; kinds = %v", rec.Kinds())
	}
}
