package engine_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const fixturesDiff = "diff --git a/test/fixtures/export.json b/test/fixtures/export.json\n" +
	"new file mode 100644\n" +
	"index 0000000..4444444\n" +
	"--- /dev/null\n" +
	"+++ b/test/fixtures/export.json\n" +
	"@@ -0,0 +1 @@\n" +
	"+{}\n"

const twoSuggestions = "docs/design/: design documents, not shipped behaviour\ntest/fixtures/: test data\n"

// pausedRelease runs an attended prior-tag release under config whose notes diff is
// the export source, the design doc and the fixtures, claude answering the check with
// checkAnswer and the notes with aiBody. rec answers the gates, and git the answers
// commit unless checkAnswer is none.
func pausedRelease(t *testing.T, rec *presentertest.RecordingPresenter, config, checkAnswer string) (*runner.FakeRunner, error) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(exportGoDiff+designDocDiff+fixturesDiff))
	if checkAnswer != "none" {
		f.SeedSequence("git", answersCommitCalls...)
	}
	f.SeedSequence("git",
		ScriptedOut("M\tsrc/export.go\nA\tdocs/design/export.md\nA\ttest/fixtures/export.json\n"),
		ScriptedOut("1\t1\tsrc/export.go\n1\t0\tdocs/design/export.md\n1\t0\ttest/fixtures/export.json\n"),
	)
	f.SeedSequence("claude", ScriptedOut(checkAnswer), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	return f, engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())
}

func isPauseGate(g presenter.Gate) bool {
	return g.Has(presenter.ChoiceKeep)
}

// pauseGates returns every recorded exclusion pause.
func pauseGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, g := range promptGates(rec) {
		if isPauseGate(g) {
			gates = append(gates, g)
		}
	}
	return gates
}

func pauseQuestions(gates []presenter.Gate) []string {
	questions := make([]string, len(gates))
	for i, g := range gates {
		questions[i] = g.Question
	}
	return questions
}

func TestRelease_Attended_PausesOnEachSuggestionShowingItsGlobAndReason(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", twoSuggestions); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{
		"Exclude docs/design/ from the notes? (design documents, not shipped behaviour; 1 file, 176 B)",
		"Exclude test/fixtures/ from the notes? (test data; 1 file, 175 B)",
	}
	if got := pauseQuestions(pauseGates(rec)); !slices.Equal(got, want) {
		t.Errorf("pauses = %q, want %q", got, want)
	}
}

func TestRelease_APause_OffersExcludeAndKeepWithKeepTheDefault(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", "docs/design/: design documents\n"); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	gates := pauseGates(rec)
	if len(gates) != 1 {
		t.Fatalf("pauses = %d, want one", len(gates))
	}
	want := []presenter.GateChoice{
		{Key: "e", Action: "exclude", Answered: "excluded"},
		{Key: "k", Action: "keep"},
	}
	if !slices.Equal(gates[0].Choices, want) {
		t.Errorf("pause choices = %v, want %v", gates[0].Choices, want)
	}
	if gates[0].Default != "k" {
		t.Errorf("pause default = %q, want k", gates[0].Default)
	}
}

func TestRelease_ABareEnterAtAPause_PicksKeep(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", "docs/design/: design documents\n"); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	gates := pauseGates(rec)
	if len(gates) != 1 {
		t.Fatalf("pauses = %d, want one", len(gates))
	}
	p := presenter.NewPlainPresenterWithInput(&strings.Builder{}, &strings.Builder{}, strings.NewReader("\n"))

	choice, err := p.Prompt(gates[0])
	if err != nil {
		t.Fatalf("Prompt returned unexpected error: %v", err)
	}
	if choice != presenter.ChoiceKeep {
		t.Errorf("bare Enter picked %q, want keep", choice)
	}
}

func TestRelease_CheckAnswersNone_NoPauseShows(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", "none"); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if gates := pauseGates(rec); len(gates) != 0 {
		t.Errorf("pauses = %q, want none", pauseQuestions(gates))
	}
}

func TestRelease_SuggestionsMatchingNoUncoveredPath_GetNoPauseWhileTheOthersDo(t *testing.T) {
	t.Parallel()

	// git leaves specs/ out of the notes diff, so only the keep list's path, the source
	// and the fixtures remain.
	const config = "[release]\ndiff_exclude = [\"specs/\"]\ndiff_keep = [\"docs/api/\"]\n"
	keptDiff := "diff --git a/docs/api/export.md b/docs/api/export.md\n@@ -0,0 +1 @@\n+# Export API\n"
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", withAnswersCommit(
		ScriptedOut(keptDiff+exportGoDiff+fixturesDiff),
		ScriptedOut("A\tdocs/api/export.md\nM\tsrc/export.go\nA\ttest/fixtures/export.json\n"),
		ScriptedOut("1\t0\tdocs/api/export.md\n1\t1\tsrc/export.go\n1\t0\ttest/fixtures/export.json\n"),
	)...)
	f.SeedSequence("claude",
		ScriptedOut("build/: build output\nspecs/: specs\ndocs/api/: API docs\ntest/fixtures/: test data\n"),
		ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{"Exclude test/fixtures/ from the notes? (test data; 1 file, 175 B)"}
	if got := pauseQuestions(pauseGates(rec)); !slices.Equal(got, want) {
		t.Errorf("pauses = %q, want only %q", got, want)
	}
}

func TestRelease_AStarGlobMatchingANestedChange_GetsAPause(t *testing.T) {
	t.Parallel()

	const guideDiff = "diff --git a/docs/guide/intro.md b/docs/guide/intro.md\n@@ -1 +1 @@\n-old\n+new\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", withAnswersCommit(
		ScriptedOut(exportGoDiff+guideDiff),
		ScriptedOut("M\tsrc/export.go\nM\tdocs/guide/intro.md\n"),
		ScriptedOut("1\t1\tsrc/export.go\n1\t1\tdocs/guide/intro.md\n"),
	)...)
	f.SeedSequence("claude", ScriptedOut("*.md: documentation\n"), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{"Exclude *.md from the notes? (documentation; 1 file, 77 B)"}
	if got := pauseQuestions(pauseGates(rec)); !slices.Equal(got, want) {
		t.Errorf("pauses = %q, want %q", got, want)
	}
}

func TestRelease_EveryPauseAnsweredKeep_WritesTheNotesFromTheSameDiff(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceKeep, presenter.ChoiceKeep, presenter.ChoiceYes},
	}
	f, err := pausedRelease(t, rec, "", twoSuggestions)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(pauseGates(rec)); got != 2 {
		t.Fatalf("pauses = %d, want two", got)
	}
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one", len(notesCalls))
	}
	if diff := exportGoDiff + designDocDiff + fixturesDiff; !strings.Contains(notesCalls[0].Stdin, "\n\n"+diff+"\n\n") {
		t.Errorf("the notes call does not carry the whole diff unchanged:\n%s", notesCalls[0].Stdin)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
	}
}

func TestRelease_ThePauses_SuspendTheSpinnerWhileEachWaits(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", twoSuggestions); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	pauses := 0
	for i, ev := range rec.Events {
		if ev.Kind != presentertest.KindPrompt || !isPauseGate(ev.Prompt) {
			continue
		}
		pauses++
		if i == 0 || rec.Events[i-1].Kind != presentertest.KindSuspendSpinner {
			t.Errorf("pause %d is not preceded by SuspendSpinner; kinds = %v", pauses, rec.Kinds())
		}
		if i+1 == len(rec.Events) || rec.Events[i+1].Kind != presentertest.KindResumeSpinner {
			t.Errorf("pause %d is not followed by ResumeSpinner; kinds = %v", pauses, rec.Kinds())
		}
	}
	if pauses != 2 {
		t.Errorf("pauses = %d, want two", pauses)
	}
}

// answeringPauses answers every pause with pause and every other gate with its default.
func answeringPauses(pause func() (presenter.Choice, error)) *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if isPauseGate(g) {
				return pause()
			}
			return g.Default, nil
		},
	}
}

func TestRelease_CtrlCAtAPause_CancelsTheRunBeforeAnyNotesCall(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	f.SeedSequence("claude", ScriptedOut("auth/: auth code\n"), ScriptedOut(aiBody))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	rec := answeringPauses(func() (presenter.Choice, error) {
		cancel()
		return presenter.ChoiceKeep, nil
	})

	err := engine.Release(ctx, attendedDeps(rec, f), priorTagNormalAIOptions())

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if got := len(pauseGates(rec)); got != 1 {
		t.Errorf("pauses = %d, want the one the Ctrl-C landed at", got)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made after the Ctrl-C")
	}
	assertStageFailedThenUnwound(t, rec)
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_StdinClosedAtAPause_AbortsWithNoNotesCallAndUnwindsThePreTagCommit(t *testing.T) {
	t.Parallel()

	_, f := gateAbortRelease(t, preTagHookConfig, seedNormalAINotes)
	f.SeedSequence("claude", ScriptedOut("auth/: auth code\n"), ScriptedOut(aiBody))
	rec := answeringPauses(func() (presenter.Choice, error) { return "", presenter.ErrInputClosed })

	err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())

	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made; the closed input was taken as keep")
	}
	assertStageFailedThenUnwound(t, rec)
	assertUnwoundToStart(t, rec, f, err)
}

func TestRelease_Unattended_RaisesNoPause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		yes, interactive bool
	}{
		{name: "-y at a terminal", yes: true, interactive: true},
		{name: "non-TTY stdin", yes: false, interactive: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedNormalAINotes(f)
			f.SeedSequence("claude", ScriptedOut("auth/: auth code\n"), ScriptedOut(aiBody))
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			deps.Yes, deps.StdinInteractive = tt.yes, tt.interactive

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if gates := pauseGates(rec); len(gates) != 0 {
				t.Errorf("pauses = %q, want none", pauseQuestions(gates))
			}
		})
	}
}
