package engine_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const loginOutput = "Not logged in · Please run /login"

const commitSubjects = "feat: add the auth package\nfix: keep the session on token refresh\n"

// attendedDeps is newDeps for a run at a terminal without -y.
func attendedDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.StdinInteractive = true
	return deps
}

// floorGates returns every recorded gate that offers the commit-subject list.
func floorGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, g := range promptGates(rec) {
		if g.Has(presenter.ChoiceCommitSubjects) {
			gates = append(gates, g)
		}
	}
	return gates
}

func indexOfKindFrom(rec *presentertest.RecordingPresenter, kind presentertest.EventKind, from int) int {
	for i := from; i < len(rec.Events); i++ {
		if rec.Events[i].Kind == kind {
			return i
		}
	}
	return -1
}

// assertFloorFollowsFailure checks the floor's choice is the next gate after the
// notes ✗ line, with c, e and a offered, a the default, and no spinner started between.
func assertFloorFollowsFailure(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	failedAt := indexOfKind(rec, presentertest.KindStageFailed)
	if failedAt == -1 {
		t.Fatalf("no StageFailed recorded; kinds = %v", rec.Kinds())
	}
	promptAt := indexOfKindFrom(rec, presentertest.KindPrompt, failedAt)
	if promptAt == -1 {
		t.Fatalf("no gate after the notes failure; kinds = %v", rec.Kinds())
	}
	gate := rec.Events[promptAt].Prompt
	if got, want := gate.Keys(), []presenter.Choice{presenter.ChoiceCommitSubjects, presenter.ChoiceEdit, presenter.ChoiceAbort}; !slices.Equal(got, want) {
		t.Errorf("floor keys = %v, want %v", got, want)
	}
	if gate.Default != presenter.ChoiceAbort {
		t.Errorf("floor default = %q, want %q", gate.Default, presenter.ChoiceAbort)
	}
	for _, ev := range rec.Events[failedAt+1 : promptAt] {
		if ev.Kind == presentertest.KindStageStarted || ev.Kind == presentertest.KindResumeSpinner {
			t.Errorf("a spinner restarted between the failure and the choice; kinds = %v", rec.Kinds())
		}
	}
}

func assertNoTag(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	if firstIndexWithPrefix(f, "git tag -a") != -1 {
		t.Errorf("a tag was created; got %v", commandLines(f.Invocations()))
	}
}

func singlePassFailingRun(t *testing.T, root string, failure error) (*runner.FakeRunner, *presentertest.RecordingPresenter, engine.ReleaseDeps) {
	t.Helper()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	rec := &presentertest.RecordingPresenter{}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{err: failure}
	return f, rec, deps
}

func TestRelease_NotesFloor_AttendedAbortOffersTheChoiceBeneathTheFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config string
	}{
		{name: "on_notes_failure unset"},
		{name: "on_notes_failure abort", config: "[release]\non_notes_failure = \"abort\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tc.config != "" {
				writeConfig(t, root, tc.config)
			}
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput + "\n", ExitCode: 1})

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != loginOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", loginOutput)
			}
			assertFloorFollowsFailure(t, rec)
			assertNoTag(t, f)
		})
	}
}

func TestRelease_NotesFloor_SavedChunkNotesPathPrecedesTheChoice(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "", twoPartTransport(failsMerge))
	run.deps.StdinInteractive = true

	err := run.release(t, t.Context())

	assertAbortNonZero(t, err)
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != run.savedPath(nextTag) {
		t.Errorf("StageFailure.SavedNotes = %q, want %q", sf.SavedNotes, run.savedPath(nextTag))
	}
	assertFloorFollowsFailure(t, run.rec)
}

func TestRelease_NotesFloor_TimeoutAndMissingToolOfferTheChoice(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		failure     error
		wantMessage string
	}{
		{name: "timeout", failure: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), wantMessage: "AI timed out"},
		{name: "tool not installed", failure: fmt.Errorf("%w: claude", ai.ErrCommandMissing), wantMessage: "AI tool not installed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, rec, deps := singlePassFailingRun(t, t.TempDir(), tc.failure)

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			if sf := onlyStageFailureEvent(t, rec); sf.Message != tc.wantMessage {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantMessage)
			}
			assertFloorFollowsFailure(t, rec)
		})
	}
}

func TestRelease_NotesFloor_CommitSubjectsReleaseAsUsual(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s v1.2.3..HEAD
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceCommitSubjects, presenter.ChoiceYes}

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "log", "--format=%s", priorTag+"..HEAD") {
		t.Errorf("no `git log --format=%%s %s..HEAD`; got %v", priorTag, commandLines(f.Invocations()))
	}
	gates := promptGates(rec)
	if len(gates) != 3 {
		t.Fatalf("gates = %d, want the version gate, the floor and the review gate", len(gates))
	}
	review := gates[2]
	if review.Question != "Release with these notes?" {
		t.Errorf("review gate Question = %q, want %q", review.Question, "Release with these notes?")
	}
	if got, want := review.Keys(), []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}; !slices.Equal(got, want) {
		t.Errorf("review gate keys = %v, want %v", got, want)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != commitSubjects {
		t.Errorf("tag annotation body = %q, want %q", got, commitSubjects)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != commitSubjects {
		t.Errorf("CHANGELOG body = %q, want %q", got, commitSubjects)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != commitSubjects {
		t.Errorf("provider release body = %q, want %q", got, commitSubjects)
	}
	if sf := onlyStageFailureEvent(t, rec); !sf.Recoverable {
		t.Errorf("the notes failure ahead of the choice is not Recoverable, so the success line would be suppressed")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("last event = %v, want RunFinished", fin.Kind)
	}
}

func TestRelease_NotesFloor_EmptyCommitSubjectsGiveTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		log  string
	}{
		{name: "empty", log: ""},
		{name: "whitespace only", log: "  \n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
			f.SeedSequence("git", ScriptedOut(tc.log)) // log --format=%s v1.2.3..HEAD
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceCommitSubjects, presenter.ChoiceYes}

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(notes.StubBody()) {
				t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, notes.StubBody())
			}
		})
	}
}

// seedPreTagArtifactThenNotes scripts a prior-tag run whose pre_tag hook leaves an
// artifact commit, then the notes assembly, then the unwind's reset.
func seedPreTagArtifactThenNotes(f *runner.FakeRunner, root string) {
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
	)
	seedNormalAINotes(f)
	f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startingSHA
}

func TestRelease_NotesFloor_AbortEndsBeforeAnyTagAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		answer func(presenter.Gate) (presenter.Choice, error)
	}{
		{name: "a", answer: func(presenter.Gate) (presenter.Choice, error) { return presenter.ChoiceAbort, nil }},
		{name: "bare Enter", answer: func(g presenter.Gate) (presenter.Choice, error) { return g.Default, nil }},
		{name: "stdin closed", answer: func(presenter.Gate) (presenter.Choice, error) { return "", presenter.ErrInputClosed }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
			f := runner.NewFakeRunner()
			seedPreTagArtifactThenNotes(f, root)
			f.Seed("sh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
				if g.Has(presenter.ChoiceCommitSubjects) {
					return tc.answer(g)
				}
				return g.Default, nil
			}}
			deps := attendedDeps(rec, f)
			deps.Transport = &freshTransport{err: &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}}

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			if len(floorGates(rec)) != 1 {
				t.Fatalf("floor gates = %d, want 1", len(floorGates(rec)))
			}
			assertNoTag(t, f)
			if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
				t.Errorf("the pre_tag artifact commit was not unwound; got %v", commandLines(f.Invocations()))
			}
			if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
				t.Errorf("Unwound.Text() = %q, want %q", got, want)
			}
			if countCmd(f, "git", "log", "--format=%s", priorTag+"..HEAD") != 0 {
				t.Errorf("an abort built the commit-subject list")
			}
		})
	}
}

func TestRelease_NotesFloor_UnattendedRunGetsNoChoice(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	deps.Yes = true

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("an unattended run was offered the choice: %+v", gates)
	}
	if indexOfKind(rec, presentertest.KindAskLine) != -1 {
		t.Errorf("an unattended run read input; kinds = %v", rec.Kinds())
	}
	if sf := onlyStageFailureEvent(t, rec); sf.Recoverable {
		t.Errorf("an unattended run's notes failure is Recoverable, want it to end the run")
	}
	assertNoTag(t, f)
}

func TestRelease_NotesFloor_FailedRegenerateAbortsWithoutTheChoice(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen}}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{body: aiBody}
	deps.Regenerator = &fakeRegenerator{err: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)}

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if !errors.Is(err, ai.ErrTimeout) {
		t.Errorf("err = %v, want the failed regenerate's cause", err)
	}
	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("a failed r was offered the choice: %+v", gates)
	}
	assertNoTag(t, f)
}

func TestRelease_NotesFloor_CancelDuringNotesAbortsWithoutTheChoice(t *testing.T) {
	t.Parallel()

	f, rec, deps := singlePassFailingRun(t, t.TempDir(), context.Canceled)

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("a cancel was offered the choice: %+v", gates)
	}
	assertNoTag(t, f)
}
