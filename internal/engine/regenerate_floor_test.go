package engine_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	regenFloorTag      = "v1.2.0"
	regenFloorRange    = "v1.1.0..v1.2.0"
	regenFloorSubjects = "feat: add export\nfix: handle empty input\n"
)

// newAttendedRegenFloor is a fresh single-version regenerate at a terminal without -y,
// its editor ed and its target the provider release.
func newAttendedRegenFloor(t *testing.T, ed engine.Editor) *regenChunkNotes {
	t.Helper()
	run := newRegenChunkNotes(t)
	run.deps.StdinInteractive = true
	run.deps.Editor = ed
	return run
}

func (run *regenChunkNotes) regenerateAttended(ctx context.Context, produce freshProducer) error {
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.Tag = regenFloorTag
	req.DiffRange = regenFloorRange
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return produce(ctx, version.Resolution{Tag: regenFloorTag, PreviousTag: "v1.1.0"}, sinks)
	}
	return engine.RegenerateRun(ctx, run.deps, run.pub, run.root, req)
}

// singlePassFailsUnder fails the one notes call of a fresh run whose
// on_notes_failure is onFailure.
func singlePassFailsUnder(onFailure string) freshProducer {
	return func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		cfg := freshCfg()
		cfg.Release.OnNotesFailure = onFailure
		notesGit := seedFreshGit(twoFileDiff(1_000), "M\tapi/a.go\nM\tcore/b.go\n", "16\t0\tapi/a.go\n16\t0\tcore/b.go\n")
		failsNotesCalls := func(_ context.Context, prompt string) (string, error) {
			if isClassifyPrompt(prompt) {
				return "OTHER", nil
			}
			return "", &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
		}
		return engine.RegenerateFreshBody(ctx, notesGit, transportFunc(failsNotesCalls), "", cfg, res, sinks)
	}
}

func freshNotes(context.Context, version.Resolution, engine.NotesSinks) (string, error) {
	return regenRunFreshBody, nil
}

func (run *regenChunkNotes) seedSubjects() {
	run.deps.Runner.(*runner.FakeRunner).SeedSequence("git", ScriptedOut(regenFloorSubjects))
}

func assertNothingWritten(t *testing.T, run *regenChunkNotes) {
	t.Helper()
	if len(run.pub.dispatched) != 0 {
		t.Errorf("provider writes = %d, want none", len(run.pub.dispatched))
	}
	if got := countCmd(run.deps.Runner.(*runner.FakeRunner), "git", "push", "origin", "HEAD"); got != 0 {
		t.Errorf("changelog pushes = %d, want none", got)
	}
}

func assertExitsOne(t *testing.T, err error) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", err)
	}
}

func assertOnlyBodyWritten(t *testing.T, run *regenChunkNotes, want string) {
	t.Helper()
	if len(run.pub.dispatched) != 1 {
		t.Fatalf("provider writes = %d, want 1", len(run.pub.dispatched))
	}
	if got := run.pub.dispatched[0].body; got != want {
		t.Errorf("provider release body = %q, want %q", got, want)
	}
}

func assertReviewGateWithoutRegen(t *testing.T, run *regenChunkNotes) {
	t.Helper()
	gates := reviewGates(run.rec)
	if len(gates) != 1 {
		t.Fatalf("review gates = %d, want 1", len(gates))
	}
	if got, want := gates[0].Keys(), []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}; !slices.Equal(got, want) {
		t.Errorf("review gate keys = %v, want %v", got, want)
	}
}

func TestRegenerateRun_NotesFloor_AttendedFailureOffersTheChoiceWhateverOnNotesFailureHolds(t *testing.T) {
	t.Parallel()

	for _, onFailure := range []string{"", "abort", "fallback"} {
		t.Run(fmt.Sprintf("on_notes_failure %q", onFailure), func(t *testing.T) {
			t.Parallel()

			run := newAttendedRegenFloor(t, &fakeEditor{})

			err := run.regenerateAttended(t.Context(), singlePassFailsUnder(onFailure))

			assertExitsOne(t, err)
			sf := onlyStageFailureEvent(t, run.rec)
			if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != usageLimitOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", usageLimitOutput)
			}
			if sf.SavedNotes != "" {
				t.Errorf("StageFailure.SavedNotes = %q, want none named", sf.SavedNotes)
			}
			assertFloorFollowsFailure(t, run.rec)
			if got := floorGates(run.rec)[0].Question; got != "Regenerate without AI notes?" {
				t.Errorf("floor Question = %q, want %q", got, "Regenerate without AI notes?")
			}
			assertNothingWritten(t, run)
		})
	}
}

func TestRegenerateRun_NotesFloor_SavedChunkNotesPathPrecedesTheChoice(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})

	assertExitsOne(t, run.regenerateAttended(t.Context(), overCapFailing(failsMerge)))

	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != run.savedPath(regenFloorTag) {
		t.Errorf("StageFailure.SavedNotes = %q, want %q", sf.SavedNotes, run.savedPath(regenFloorTag))
	}
	assertFloorFollowsFailure(t, run.rec)
}

func TestRegenerateRun_NotesFloor_CommitSubjectsGoThroughTheReviewGateWithoutRegen(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})
	run.seedSubjects()
	run.rec.PromptResult = answerFloor(presenter.ChoiceCommitSubjects)

	if err := run.regenerateAttended(t.Context(), singlePassFailsUnder("")); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if !invokedWith(run.deps.Runner.(*runner.FakeRunner), "git", "log", "--format=%s", regenFloorRange) {
		t.Errorf("no `git log --format=%%s %s`; got %v", regenFloorRange, commandLines(run.deps.Runner.(*runner.FakeRunner).Invocations()))
	}
	assertReviewGateWithoutRegen(t, run)
	assertOnlyBodyWritten(t, run, regenFloorSubjects)
}

func TestRegenerateRun_NotesFloor_CommitSubjectsLeaveOutTheVersionsBookkeepingCommit(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})
	run.deps.Runner.(*runner.FakeRunner).SeedSequence("git", ScriptedOut("🌿 Release "+regenFloorTag+"\n"+regenFloorSubjects))
	run.rec.PromptResult = answerFloor(presenter.ChoiceCommitSubjects)

	if err := run.regenerateAttended(t.Context(), singlePassFailsUnder("")); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	assertOnlyBodyWritten(t, run, regenFloorSubjects)
}

func TestRegenerateRun_NotesFloorEdit_OpensOnTheChunkNotesThisRunSaved(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		fail func(prompt string) error
		want string
	}{
		{name: "a part failed", fail: failsSecondPart, want: partOneSaved},
		{name: "the merge failed", fail: failsMerge, want: apiPartNotes + "\n\n" + corePartNotes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ed := &fakeEditor{}
			run := newAttendedRegenFloor(t, ed)
			run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

			assertExitsOne(t, run.regenerateAttended(t.Context(), overCapFailing(tc.fail)))

			if ed.calls != 1 {
				t.Fatalf("editor Edit calls = %d, want 1", ed.calls)
			}
			if ed.gotCurrent != tc.want {
				t.Errorf("editor opened on %q, want the saved chunk notes %q", ed.gotCurrent, tc.want)
			}
			if got := readSaved(t, run.savedPath(regenFloorTag)); got != ed.gotCurrent {
				t.Errorf("editor opened on %q, want the saved file's contents %q", ed.gotCurrent, got)
			}
		})
	}
}

func TestRegenerateRun_NotesFloorEdit_SinglePassFailureOpensEmpty(t *testing.T) {
	t.Parallel()

	ed := &fakeEditor{}
	run := newAttendedRegenFloor(t, ed)
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	assertExitsOne(t, run.regenerateAttended(t.Context(), singlePassFailsUnder("")))

	if ed.calls != 1 {
		t.Fatalf("editor Edit calls = %d, want 1", ed.calls)
	}
	if ed.gotCurrent != "" {
		t.Errorf("editor opened on %q, want it empty", ed.gotCurrent)
	}
}

func TestRegenerateRun_NotesFloorEdit_UnsavedChunkNotesOpenEmptyOverAnEarlierFile(t *testing.T) {
	t.Parallel()

	const earlier = "- an earlier failure's notes\n"
	ed := &fakeEditor{}
	run := newAttendedRegenFloor(t, ed)
	path, err := notescache.NewChunkNotes(run.base()).Save(run.root, regenFloorTag, earlier)
	if err != nil {
		t.Fatalf("pre-saving chunk notes: %v", err)
	}
	makeReadOnly(t, filepath.Dir(path))
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	assertExitsOne(t, run.regenerateAttended(t.Context(), overCapFailing(failsSecondPart)))

	if got := len(warnings(run.rec)); got != 1 {
		t.Fatalf("warnings = %d, want the one saying the chunk notes could not be saved", got)
	}
	if got := readSaved(t, path); got != earlier {
		t.Fatalf("file at the path = %q, want the earlier failure's %q", got, earlier)
	}
	if ed.calls != 1 {
		t.Fatalf("editor Edit calls = %d, want 1", ed.calls)
	}
	if ed.gotCurrent != "" {
		t.Errorf("editor opened on %q, want it empty", ed.gotCurrent)
	}
}

func TestRegenerateRun_NotesFloorEdit_SavedBodyGoesThroughTheReviewGateAndLeavesTheFile(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{edited: editedBody})
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	if err := run.regenerateAttended(t.Context(), overCapFailing(failsSecondPart)); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	assertReviewGateWithoutRegen(t, run)
	assertOnlyBodyWritten(t, run, editedBody)
	if got := readSaved(t, run.savedPath(regenFloorTag)); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want them as the wave saved them %q", got, partOneSaved)
	}
}

func TestRegenerateRun_NotesFloorEdit_EmptyBodyReturnsToTheChoice(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		edited string
	}{
		{name: "empty", edited: ""},
		{name: "whitespace only", edited: "  \n\t\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newAttendedRegenFloor(t, &fakeEditor{edited: tc.edited})
			run.seedSubjects()
			run.rec.PromptResult = answerFloor(presenter.ChoiceEdit, presenter.ChoiceCommitSubjects)

			if err := run.regenerateAttended(t.Context(), singlePassFailsUnder("")); err != nil {
				t.Fatalf("RegenerateRun returned unexpected error: %v", err)
			}

			gates := promptGates(run.rec)
			if len(gates) != 3 {
				t.Fatalf("gates = %d, want the floor twice and the review gate", len(gates))
			}
			for i, g := range gates[:2] {
				if !g.Has(presenter.ChoiceCommitSubjects) {
					t.Errorf("gate %d = %q, want the floor's choice", i, g.Question)
				}
			}
			assertReviewGateWithoutRegen(t, run)
			assertOnlyBodyWritten(t, run, regenFloorSubjects)
		})
	}
}

func TestRegenerateRun_NotesFloorEdit_NoEditorInstalledWarnsAndReturnsToTheChoice(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, nil)
	editorRunner := runner.NewFakeRunner()
	name := resolvedEditorName()
	editorRunner.SeedNotFound(name)
	run.deps.Editor = engine.NewEditorLauncher(run.rec, editorRunner)
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit, presenter.ChoiceAbort)

	assertExitsOne(t, run.regenerateAttended(t.Context(), singlePassFailsUnder("")))

	assertOnlyWarning(t, run.rec, presenter.Warning{Label: "editor", Message: fmt.Sprintf("could not launch editor %q: not found", name)})
	if got := len(floorGates(run.rec)); got != 2 {
		t.Errorf("floor gates = %d, want the choice shown again after the warning", got)
	}
	if len(reviewGates(run.rec)) != 0 {
		t.Errorf("a review gate showed with no notes written")
	}
	assertNothingWritten(t, run)
}

func TestRegenerateRun_NotesFloorEdit_FailedEditorIsSurfacedAndAborts(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, nil)
	name := resolvedEditorName()
	editorRunner := runner.NewFakeRunner()
	editorRunner.Seed(name, runner.Result{}, errors.New("exit status 1"))
	run.deps.Editor = engine.NewEditorLauncher(run.rec, editorRunner)
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	assertExitsOne(t, run.regenerateAttended(t.Context(), singlePassFailsUnder("")))

	if got, want := stageFailures(run.rec), []string{"notes", "edit"}; !slices.Equal(got, want) {
		t.Errorf("failed stages = %v, want %v", got, want)
	}
	if got, want := lastStageFailure(run.rec).Message, fmt.Sprintf("launching editor %q: exit status 1", name); got != want {
		t.Errorf("edit StageFailure.Message = %q, want %q", got, want)
	}
	if got := len(floorGates(run.rec)); got != 1 {
		t.Errorf("floor gates = %d, want the run to end at the editor failure", got)
	}
	assertNothingWritten(t, run)
}

func TestRegenerateRun_NotesFloor_AbortOrBareEnterWritesNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		answer func(presenter.Gate) (presenter.Choice, error)
	}{
		{name: "a", answer: answerFloor(presenter.ChoiceAbort)},
		{name: "bare Enter", answer: func(g presenter.Gate) (presenter.Choice, error) { return g.Default, nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newAttendedRegenFloor(t, &fakeEditor{})
			run.rec.PromptResult = tc.answer

			assertExitsOne(t, run.regenerateAttended(t.Context(), singlePassFailsUnder("")))

			if got := len(floorGates(run.rec)); got != 1 {
				t.Errorf("floor gates = %d, want 1", got)
			}
			if len(reviewGates(run.rec)) != 0 {
				t.Errorf("a review gate showed after the abort")
			}
			assertNothingWritten(t, run)
		})
	}
}

func TestRegenerateRun_NotesFloor_FailedRegenAtTheReviewGateAbortsWithoutTheChoice(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})
	run.deps.Regenerator = &fakeRegenerator{err: fmt.Errorf("regenerating: %w", &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1})}
	run.rec.NextChoices = []presenter.Choice{presenter.ChoiceRegen}

	assertExitsOne(t, run.regenerateAttended(t.Context(), freshNotes))

	if got := len(floorGates(run.rec)); got != 0 {
		t.Errorf("floor gates = %d, want none after a failed r", got)
	}
	if got, want := stageFailures(run.rec), []string{"regenerate"}; !slices.Equal(got, want) {
		t.Errorf("failed stages = %v, want %v", got, want)
	}
	assertNothingWritten(t, run)
}

func TestRegenerateRun_NotesFloor_CtrlCDuringTheNotesCallAbortsWithoutTheChoice(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})
	ctx, cancel := context.WithCancel(t.Context())
	cancelled := func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		return underCapFresh(ctx, func(ctx context.Context, _ string) (string, error) {
			cancel()
			return "", ctx.Err()
		}, res, sinks)
	}

	err := run.regenerateAttended(ctx, cancelled)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want a cancel", err)
	}
	if got := len(floorGates(run.rec)); got != 0 {
		t.Errorf("floor gates = %d, want none after a cancel", got)
	}
	assertNothingWritten(t, run)
}
