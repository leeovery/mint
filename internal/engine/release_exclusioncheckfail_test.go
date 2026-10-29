package engine_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const checkFailedMessage = "couldn't check for files that don't belong in the notes"

// checkFailedWarnings returns the recorded warnings that the exclusion check failed.
func checkFailedWarnings(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var out []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Message == checkFailedMessage {
			out = append(out, ev.Warn)
		}
	}
	return out
}

// assertCheckFailedWarnedOnce fails unless the run's one notes stage, after the check's
// call, opens with exactly one check-failed warning carrying output, the spinner
// suspended around it, before the stage completes.
func assertCheckFailedWarnedOnce(t *testing.T, rec *presentertest.RecordingPresenter, output string) {
	t.Helper()
	warnings := checkFailedWarnings(rec)
	if len(warnings) != 1 {
		t.Fatalf("check-failed warnings = %d, want one; kinds = %v", len(warnings), rec.Kinds())
	}
	if want := (presenter.Warning{Label: "notes", Message: checkFailedMessage, Key: "check", Output: output}); warnings[0] != want {
		t.Errorf("warning = %+v, want %+v", warnings[0], want)
	}
	stage := releaseNotesStage(t, rec)
	want := []presentertest.EventKind{presentertest.KindSuspendSpinner, presentertest.KindWarn, presentertest.KindResumeSpinner}
	if len(stage) < len(want)+1 || !slices.Equal(kindsOf(stage[:len(want)]), want) {
		t.Errorf("notes stage after the check = %v, want the suspended warning first, then the notes", kindsOf(stage))
	}
}

// checkFailingTransport fails the exclusion check with checkErr and answers every other
// call with aiBody, recording the other calls' prompts.
type checkFailingTransport struct {
	checkErr error

	mu      sync.Mutex
	prompts []string
}

func (ct *checkFailingTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "", ct.checkErr
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.prompts = append(ct.prompts, prompt)
	return aiBody, nil
}

// seededCheckRelease is a prior-tag release over priorTagDiff seeded through the push,
// claude scripted with calls.
func seededCheckRelease(t *testing.T, calls ...runner.ScriptedCall) (*runner.FakeRunner, engine.ReleaseDeps, *presentertest.RecordingPresenter) {
	t.Helper()
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.SeedSequence("claude", calls...)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	return f, newDeps(rec, f), rec
}

func TestRelease_CheckCallFailsAfterTheRetry_WarnsWithItsOutputAndMakesNoClassifyCall(t *testing.T) {
	t.Parallel()

	notLoggedInExit := runner.ScriptedCall{
		Result: runner.Result{Stdout: notLoggedIn + "\n", ExitCode: 1},
		Err:    errors.New("exit status 1"),
	}
	f, deps, rec := seededCheckRelease(t, notLoggedInExit, notLoggedInExit, ScriptedOut(aiBody))

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(checkInvocations(f, "claude")); got != 2 {
		t.Errorf("check calls = %d, want the first attempt and the transport's retry", got)
	}
	notesCalls := notesInvocations(f, "claude")
	for _, inv := range notesCalls {
		if isClassifyPrompt(inv.Stdin) {
			t.Errorf("a classify call was made for the check:\n%s", inv.Stdin)
		}
	}
	if len(notesCalls) != 1 {
		t.Errorf("claude calls after the check = %d, want the notes call alone", len(notesCalls))
	}
	assertCheckFailedWarnedOnce(t, rec, notLoggedIn)
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
	}
}

func TestRelease_CheckTimesOutOrFindsNoTool_WarnsAndCarriesOnToItsNotes(t *testing.T) {
	t.Parallel()

	for _, failure := range []error{
		fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout),
		fmt.Errorf("%w: claude", ai.ErrCommandMissing),
	} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Parallel()

			f, deps, rec := seededCheckRelease(t)
			tr := &checkFailingTransport{checkErr: failure}
			deps.Transport = tr

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertCheckFailedWarnedOnce(t, rec, "")
			if len(tr.prompts) != 1 || isClassifyPrompt(tr.prompts[0]) {
				t.Errorf("calls after the check = %d, want the notes call alone", len(tr.prompts))
			}
			if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
				t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
			}
		})
	}
}

func TestRelease_CheckAnswerYieldsNoUsableSuggestion_WarnsWithTheAnswerAndNeitherPausesNorAsksAgain(t *testing.T) {
	t.Parallel()

	// priorTagDiff changes auth/login.go alone, so no glob below matches it.
	const answer = "vendor/: vendored code\ndocs/design/: design documents"
	f, deps, rec := seededCheckRelease(t, ScriptedOut(answer+"\n"), ScriptedOut(aiBody))
	deps.StdinInteractive = true

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertCheckFailedWarnedOnce(t, rec, answer)
	if got := pauseGates(rec); len(got) != 0 {
		t.Errorf("pauses = %v, want none", pauseQuestions(got))
	}
	if got := len(checkInvocations(f, "claude")); got != 1 {
		t.Errorf("check calls = %d, want one", got)
	}
	if got := len(notesInvocations(f, "claude")); got != 1 {
		t.Errorf("notes calls = %d, want one", got)
	}
}

func TestRelease_AFailedCheck_NeitherPausesNorPrintsSuggestionsAfterTheNotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yes      bool
		attended bool
	}{
		{name: "attended", attended: true},
		{name: "-y", yes: true, attended: true},
		{name: "non-TTY stdin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, deps, rec := seededCheckRelease(t)
			deps.Yes = tt.yes
			deps.StdinInteractive = tt.attended
			deps.Transport = &checkFailingTransport{checkErr: &ai.GenerationError{Stdout: notLoggedIn + "\n", ExitCode: 1}}

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertCheckFailedWarnedOnce(t, rec, notLoggedIn)
			if got := pauseGates(rec); len(got) != 0 {
				t.Errorf("pauses = %v, want none", pauseQuestions(got))
			}
			if got := countKind(rec, presentertest.KindExclusionSuggestions); got != 0 {
				t.Errorf("suggestion blocks = %d, want none", got)
			}
			if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
				t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
			}
		})
	}
}

func TestRelease_CheckFindsNoTool_TheNotesCallFailsThroughItsOwnPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{}, fmt.Errorf("%w: claude", runner.ErrCommandNotFound))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if got := len(checkFailedWarnings(rec)); got != 1 {
		t.Errorf("check-failed warnings = %d, want one", got)
	}
	if got := len(notesInvocations(f, "claude")); got != 1 {
		t.Errorf("notes calls = %d, want one, with no classify call", got)
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != "AI tool not installed" {
		t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, "AI tool not installed")
	}
	warned := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
		return ev.Kind == presentertest.KindWarn && ev.Warn.Message == checkFailedMessage
	})
	if failed := indexOfKind(rec, presentertest.KindStageFailed); warned < 0 || warned > failed {
		t.Errorf("check-failed warning at event %d, notes failure at %d; want the warning first", warned, failed)
	}
}

func TestRelease_CtrlCDuringTheCheck_CancelsBeforeAnyNotesCallWithNoCheckWarning(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, deps, rec := seededCheckRelease(t)
	var notesPrompts []string
	deps.Transport = transportFunc(func(ctx context.Context, prompt string) (string, error) {
		if isCheckPrompt(prompt) {
			cancel()
			return "", ctx.Err()
		}
		notesPrompts = append(notesPrompts, prompt)
		return aiBody, nil
	})

	err := engine.Release(ctx, deps, priorTagNormalAIOptions())

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if len(notesPrompts) != 0 {
		t.Errorf("calls after the Ctrl-C = %d, want none", len(notesPrompts))
	}
	if got := checkFailedWarnings(rec); len(got) != 0 {
		t.Errorf("check-failed warnings = %v, want none", got)
	}
}
