package engine_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// attendedChunkNotesRun is a chunked run at a terminal without -y, its editor ed.
func attendedChunkNotesRun(t *testing.T, transport func(prompt string) error, ed engine.Editor) *chunkNotesRun {
	t.Helper()
	run := newChunkNotesRun(t, "abort", twoPartTransport(transport))
	run.deps.StdinInteractive = true
	run.deps.Editor = ed
	return run
}

// answerFloor answers the floor's choice with each of floor in turn, then a, and
// every other gate with its default.
func answerFloor(floor ...presenter.Choice) func(presenter.Gate) (presenter.Choice, error) {
	return func(g presenter.Gate) (presenter.Choice, error) {
		if !g.Has(presenter.ChoiceCommitSubjects) {
			return g.Default, nil
		}
		if len(floor) == 0 {
			return presenter.ChoiceAbort, nil
		}
		next := floor[0]
		floor = floor[1:]
		return next, nil
	}
}

// reviewGates returns every recorded notes review gate.
func reviewGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, g := range promptGates(rec) {
		if g.Subject == "notes" && g.Has(presenter.ChoiceYes) {
			gates = append(gates, g)
		}
	}
	return gates
}

// resolvedEditorName is the binary the editor launcher runs for this process's env.
func resolvedEditorName() string {
	return strings.Fields(engine.ResolveEditor())[0]
}

func TestRelease_NotesFloorEdit_OpensOnTheChunkNotesThisRunSaved(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		transport func(prompt string) error
		want      string
	}{
		{name: "a part failed", transport: failsSecondPart, want: partOneSaved},
		{name: "the merge failed", transport: failsMerge, want: apiPartNotes + "\n\n" + corePartNotes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ed := &fakeEditor{}
			run := attendedChunkNotesRun(t, tc.transport, ed)
			run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

			assertAbortNonZero(t, run.release(t, t.Context()))

			if ed.calls != 1 {
				t.Fatalf("editor Edit calls = %d, want 1", ed.calls)
			}
			if ed.gotCurrent != tc.want {
				t.Errorf("editor opened on %q, want the saved chunk notes %q", ed.gotCurrent, tc.want)
			}
			if got := readSaved(t, run.savedPath(nextTag)); got != ed.gotCurrent {
				t.Errorf("editor opened on %q, want the saved file's contents %q", ed.gotCurrent, got)
			}
		})
	}
}

func TestRelease_NotesFloorEdit_SinglePassFailureOpensEmpty(t *testing.T) {
	t.Parallel()

	_, rec, deps := singlePassFailingRun(t, t.TempDir(), &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	ed := &fakeEditor{}
	deps.Editor = ed
	rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	assertAbortNonZero(t, engine.Release(t.Context(), deps, priorTagNormalAIOptions()))

	if ed.calls != 1 {
		t.Fatalf("editor Edit calls = %d, want 1", ed.calls)
	}
	if ed.gotCurrent != "" {
		t.Errorf("editor opened on %q, want it empty", ed.gotCurrent)
	}
}

func TestRelease_NotesFloorEdit_UnsavedChunkNotesOpenEmptyOverAnEarlierFile(t *testing.T) {
	t.Parallel()

	const earlier = "- an earlier failure's notes\n"
	ed := &fakeEditor{}
	run := attendedChunkNotesRun(t, failsSecondPart, ed)
	path, err := notescache.NewChunkNotes(run.chunkNotesBase()).Save(run.root, nextTag, earlier)
	if err != nil {
		t.Fatalf("pre-saving chunk notes: %v", err)
	}
	makeReadOnly(t, filepath.Dir(path))
	run.rec.PromptResult = answerFloor(presenter.ChoiceEdit)

	assertAbortNonZero(t, run.release(t, t.Context()))

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

// makeReadOnly leaves no file creatable in dir until the test ends.
func makeReadOnly(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("making %s read-only: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestRelease_NotesFloorEdit_SavedBodyReleasesByteIdenticalAfterTheReviewGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	deps.Editor = &fakeEditor{edited: editedBody}
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceYes}

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
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
	if got := tagAnnotationBody(t, f, nextTag); got != editedBody {
		t.Errorf("tag annotation body = %q, want %q", got, editedBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != editedBody {
		t.Errorf("CHANGELOG body = %q, want %q", got, editedBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != editedBody {
		t.Errorf("provider release body = %q, want %q", got, editedBody)
	}
}

func TestRelease_NotesFloorEdit_LeavesTheSavedChunkNotesAsTheWaveSavedThem(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		review presenter.Choice
	}{
		{name: "released", review: presenter.ChoiceYes},
		{name: "aborted at the review gate", review: presenter.ChoiceNo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := attendedChunkNotesRun(t, failsSecondPart, &fakeEditor{edited: editedBody})
			if tc.review == presenter.ChoiceYes {
				seedRecordTagPush(run.f, run.root)
				run.f.Seed("gh", runner.Result{}, nil)
			}
			run.rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, tc.review}

			err := run.release(t, t.Context())

			if tc.review == presenter.ChoiceYes && err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}
			if tc.review == presenter.ChoiceNo {
				assertAbortNonZero(t, err)
			}
			if got := readSaved(t, run.savedPath(nextTag)); got != partOneSaved {
				t.Errorf("saved chunk notes = %q, want them as the wave saved them %q", got, partOneSaved)
			}
		})
	}
}

func TestRelease_NotesFloorEdit_EmptyBodyReturnsToTheChoice(t *testing.T) {
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

			root := t.TempDir()
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
			deps.Editor = &fakeEditor{edited: tc.edited}
			f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s v1.2.3..HEAD
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceCommitSubjects, presenter.ChoiceYes}

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			gates := promptGates(rec)
			if len(gates) != 4 {
				t.Fatalf("gates = %d, want the version gate, the floor twice and the review gate", len(gates))
			}
			for i, g := range gates[1:3] {
				if !g.Has(presenter.ChoiceCommitSubjects) {
					t.Errorf("gate %d = %q, want the floor's choice", i+1, g.Question)
				}
			}
			if len(reviewGates(rec)) != 1 {
				t.Errorf("review gates = %d, want only the one after c", len(reviewGates(rec)))
			}
			if got := tagAnnotationBody(t, f, nextTag); got != commitSubjects {
				t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, commitSubjects)
			}
		})
	}
}

func TestRelease_NotesFloorEdit_NoEditorInstalledWarnsAndReturnsToTheChoice(t *testing.T) {
	t.Parallel()

	f, rec, deps := singlePassFailingRun(t, t.TempDir(), &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
	editorRunner := runner.NewFakeRunner()
	name := resolvedEditorName()
	editorRunner.SeedNotFound(name)
	deps.Editor = engine.NewEditorLauncher(rec, editorRunner)
	rec.PromptResult = answerFloor(presenter.ChoiceEdit, presenter.ChoiceAbort)

	assertAbortNonZero(t, engine.Release(t.Context(), deps, priorTagNormalAIOptions()))

	want := presenter.Warning{Label: "editor", Message: fmt.Sprintf("could not launch editor %q: not found", name)}
	assertOnlyWarning(t, rec, want)
	if got := len(floorGates(rec)); got != 2 {
		t.Errorf("floor gates = %d, want the choice shown again after the warning", got)
	}
	warned := indexOfKind(rec, presentertest.KindWarn)
	if warned == -1 {
		t.Fatalf("no warning recorded; kinds = %v", rec.Kinds())
	}
	if again := indexOfKindFrom(rec, presentertest.KindPrompt, warned); again == -1 || !rec.Events[again].Prompt.Has(presenter.ChoiceCommitSubjects) {
		t.Errorf("the choice did not follow the warning; kinds = %v", rec.Kinds())
	}
	if len(reviewGates(rec)) != 0 {
		t.Errorf("a review gate showed with no notes written")
	}
	assertNoTag(t, f)
}

func TestRelease_NotesFloorEdit_FailedEditorAbortsAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	f := runner.NewFakeRunner()
	seedPreTagArtifactThenNotes(f, root)
	f.Seed("sh", runner.Result{}, nil)
	name := resolvedEditorName()
	editorRunner := runner.NewFakeRunner()
	editorRunner.Seed(name, runner.Result{}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{PromptResult: answerFloor(presenter.ChoiceEdit)}
	deps := attendedDeps(rec, f)
	deps.Transport = &freshTransport{err: &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}}
	deps.Editor = engine.NewEditorLauncher(rec, editorRunner)

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", err)
	}
	if got, want := stageFailures(rec), []string{"notes", "edit"}; !slices.Equal(got, want) {
		t.Errorf("failed stages = %v, want %v", got, want)
	}
	if got, want := lastStageFailure(rec).Message, fmt.Sprintf("launching editor %q: exit status 1", name); got != want {
		t.Errorf("edit StageFailure.Message = %q, want %q", got, want)
	}
	if got := len(floorGates(rec)); got != 1 {
		t.Errorf("floor gates = %d, want the run to end at the editor failure", got)
	}
	assertNoTag(t, f)
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("the pre_tag artifact commit was not unwound; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundText(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
}

func lastStageFailure(rec *presentertest.RecordingPresenter) presenter.StageFailure {
	var last presenter.StageFailure
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			last = ev.StageFailed
		}
	}
	return last
}

func TestRelease_NotesFloorEdit_TheChoiceNamesWritingInTheEditor(t *testing.T) {
	t.Parallel()

	_, rec, deps := singlePassFailingRun(t, t.TempDir(), &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})

	assertAbortNonZero(t, engine.Release(t.Context(), deps, priorTagNormalAIOptions()))

	gates := floorGates(rec)
	if len(gates) != 1 {
		t.Fatalf("floor gates = %d, want 1", len(gates))
	}
	want := []presenter.GateChoice{
		{Key: presenter.ChoiceCommitSubjects, Action: "use the commit subjects"},
		{Key: presenter.ChoiceEdit, Action: "write in $EDITOR"},
		{Key: presenter.ChoiceAbort, Action: "abort"},
	}
	if !slices.Equal(gates[0].Choices, want) {
		t.Errorf("floor choices = %+v, want %+v", gates[0].Choices, want)
	}
}
