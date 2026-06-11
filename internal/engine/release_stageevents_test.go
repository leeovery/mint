package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// indexOfStage returns the position of the first recorded event of the given kind
// whose stage Name matches, or -1. It reads the Name off whichever stage payload
// the kind carries (StageStarted / StageSucceeded), so a test can locate a
// specific stage's narration in the recorded timeline.
func indexOfStage(rec *presentertest.RecordingPresenter, kind presentertest.EventKind, name string) int {
	for i, ev := range rec.Events {
		if ev.Kind != kind {
			continue
		}
		switch kind {
		case presentertest.KindStageStarted:
			if ev.StageStarted.Name == name {
				return i
			}
		case presentertest.KindStageSucceeded:
			if ev.StageSucceeded.Name == name {
				return i
			}
		}
	}
	return -1
}

// stageStarted returns the recorded StageStart payload for the named stage and
// whether one was found.
func stageStarted(rec *presentertest.RecordingPresenter, name string) (presenter.StageStart, bool) {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageStarted && ev.StageStarted.Name == name {
			return ev.StageStarted, true
		}
	}
	return presenter.StageStart{}, false
}

// stageSucceeded returns the recorded StageSuccess payload for the named stage and
// whether one was found.
func stageSucceeded(rec *presentertest.RecordingPresenter, name string) (presenter.StageSuccess, bool) {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Name == name {
			return ev.StageSucceeded, true
		}
	}
	return presenter.StageSuccess{}, false
}

// TestRelease_EmitsBlockingStageEvents proves the release orchestrator narrates the
// blocking stages — pre_tag hook, notes generation, and push — with a
// StageStarted(Blocking:true) before each and a StageSucceeded(Blocking:true,
// engine-measured Elapsed) after each, in stage order. The read-only gates (version,
// preflight) emit completion narration too. Existing events are unchanged.
func TestRelease_EmitsBlockingStageEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedPreTagClean(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // pre_tag hook exits zero
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Each blocking stage emits a StageStarted(Blocking:true) and a
	// StageSucceeded(Blocking:true) with engine-measured Elapsed (>= 0, never a
	// sentinel), in StageStarted-before-StageSucceeded order.
	for _, name := range []string{"pre_tag", "notes", "push"} {
		start, ok := stageStarted(rec, name)
		if !ok {
			t.Fatalf("no StageStarted for blocking stage %q; kinds = %v", name, rec.Kinds())
		}
		if !start.Blocking {
			t.Errorf("StageStarted(%q).Blocking = false, want true", name)
		}
		done, ok := stageSucceeded(rec, name)
		if !ok {
			t.Fatalf("no StageSucceeded for blocking stage %q; kinds = %v", name, rec.Kinds())
		}
		if !done.Blocking {
			t.Errorf("StageSucceeded(%q).Blocking = false, want true (mirrors StageStart)", name)
		}
		if done.Elapsed < 0 {
			t.Errorf("StageSucceeded(%q).Elapsed = %v, want a non-negative engine-measured duration", name, done.Elapsed)
		}
		startAt := indexOfStage(rec, presentertest.KindStageStarted, name)
		doneAt := indexOfStage(rec, presentertest.KindStageSucceeded, name)
		if startAt >= doneAt {
			t.Errorf("StageStarted(%q) at %d must precede StageSucceeded(%q) at %d", name, startAt, name, doneAt)
		}
	}

	// The blocking stages narrate in spine order: pre_tag (Stage 3) before notes
	// (Stage 4) before push (Stage 6).
	preTagAt := indexOfStage(rec, presentertest.KindStageStarted, "pre_tag")
	notesAt := indexOfStage(rec, presentertest.KindStageStarted, "notes")
	pushAt := indexOfStage(rec, presentertest.KindStageStarted, "push")
	if preTagAt >= notesAt || notesAt >= pushAt {
		t.Errorf("blocking stage order = pre_tag:%d notes:%d push:%d, want pre_tag < notes < push", preTagAt, notesAt, pushAt)
	}

	// The read-only gates emit completion narration (non-blocking is acceptable).
	for _, name := range []string{"version", "preflight"} {
		if _, ok := stageSucceeded(rec, name); !ok {
			t.Errorf("no StageSucceeded completion narration for read-only gate %q; kinds = %v", name, rec.Kinds())
		}
	}

	// RunStarted OPENS the block: it must precede the first stage narration event
	// (the version gate completion at Stage 1). The brand header renders first, with
	// every stage line — version/preflight/pre_tag/notes — beneath it (the spec worked
	// example, and the presenter golden transcript). This pins "RunStarted first" at
	// the engine layer.
	runStartedAt := indexOfKind(rec, presentertest.KindRunStarted)
	if runStartedAt == -1 {
		t.Fatalf("no RunStarted recorded; kinds = %v", rec.Kinds())
	}
	if firstStageAt := firstStageEventIndex(rec); runStartedAt >= firstStageAt {
		t.Errorf("RunStarted at %d must precede the first stage event at %d (RunStarted opens the block)", runStartedAt, firstStageAt)
	}
}

// firstStageEventIndex returns the position of the FIRST StageStarted or
// StageSucceeded event in the recorded timeline, or len(events) when none exists,
// so a test can assert RunStarted precedes the whole stage block.
func firstStageEventIndex(rec *presentertest.RecordingPresenter) int {
	for i, k := range rec.Kinds() {
		if k == presentertest.KindStageStarted || k == presentertest.KindStageSucceeded {
			return i
		}
	}
	return len(rec.Events)
}

// TestRelease_NoPreTagHook_OmitsPreTagStageEvents proves the pre_tag stage narration
// fires ONLY when a pre_tag hook is configured: a run with no hook emits no pre_tag
// StageStarted/StageSucceeded (nothing ran to narrate), while notes and push still do.
func TestRelease_NoPreTagHook_OmitsPreTagStageEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if _, ok := stageStarted(rec, "pre_tag"); ok {
		t.Errorf("pre_tag StageStarted emitted with no hook configured; kinds = %v", rec.Kinds())
	}
	if _, ok := stageSucceeded(rec, "pre_tag"); ok {
		t.Errorf("pre_tag StageSucceeded emitted with no hook configured; kinds = %v", rec.Kinds())
	}
	// notes + push still narrate.
	if _, ok := stageStarted(rec, "notes"); !ok {
		t.Errorf("notes StageStarted missing; kinds = %v", rec.Kinds())
	}
	if _, ok := stageStarted(rec, "push"); !ok {
		t.Errorf("push StageStarted missing; kinds = %v", rec.Kinds())
	}
}

// TestRelease_EditChoice_SuspendSpinnerWrapsLiveSpinner proves the editor's
// suspend/resume bracket is no longer a permanent no-op: when the user chooses `e`,
// the engine emits a BLOCKING StageStarted around the editor invocation, so the real
// EditorLauncher's SuspendSpinner suspends a GENUINELY ACTIVE stage spinner and its
// ResumeSpinner resumes it — both falling between the edit StageStarted and its
// StageSucceeded. The editor is the REAL launcher (via a write-back runner that
// simulates a saved edit), so SuspendSpinner/ResumeSpinner fire from production code.
func TestRelease_EditChoice_SuspendSpinnerWrapsLiveSpinner(t *testing.T) {
	t.Setenv("VISUAL", "myedit") // resolve a launchable editor for the real launcher

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	// The real EditorLauncher drives the production SuspendSpinner/ResumeSpinner
	// bracket; the write-back runner simulates the editor saving an edit.
	editor := engine.NewEditorLauncher(rec, &writeBackRunner{content: editedBody})
	deps := newDeps(rec, f)
	deps.Editor = editor

	if err := engine.Release(t.Context(), deps, patchOptionsWithBody(phase2Body)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// A blocking edit stage brackets the editor invocation.
	editStart, ok := stageStarted(rec, "edit")
	if !ok {
		t.Fatalf("no StageStarted for the edit stage; kinds = %v", rec.Kinds())
	}
	if !editStart.Blocking {
		t.Errorf("edit StageStarted.Blocking = false, want true (the spinner must be live for the editor to suspend it)")
	}
	editStartAt := indexOfStage(rec, presentertest.KindStageStarted, "edit")
	editDoneAt := indexOfStage(rec, presentertest.KindStageSucceeded, "edit")
	if editDoneAt == -1 {
		t.Fatalf("no StageSucceeded for the edit stage; kinds = %v", rec.Kinds())
	}

	// SuspendSpinner and ResumeSpinner fired (from the real launcher) and both fall
	// INSIDE the live edit stage — proving the bracket suspended an active spinner.
	suspendAt := indexOfKind(rec, presentertest.KindSuspendSpinner)
	resumeAt := indexOfKind(rec, presentertest.KindResumeSpinner)
	if suspendAt == -1 {
		t.Fatalf("no SuspendSpinner recorded; the bracket is still a no-op; kinds = %v", rec.Kinds())
	}
	if resumeAt == -1 {
		t.Fatalf("no ResumeSpinner recorded; kinds = %v", rec.Kinds())
	}
	if editStartAt >= suspendAt || suspendAt >= resumeAt || resumeAt >= editDoneAt {
		t.Errorf("suspend/resume not bracketed by a live edit spinner: editStart=%d suspend=%d resume=%d editDone=%d",
			editStartAt, suspendAt, resumeAt, editDoneAt)
	}
}

// indexOfKind returns the position of the first recorded event of the given kind,
// or -1 if none was recorded.
func indexOfKind(rec *presentertest.RecordingPresenter, kind presentertest.EventKind) int {
	for i, k := range rec.Kinds() {
		if k == kind {
			return i
		}
	}
	return -1
}
