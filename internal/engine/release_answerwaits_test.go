package engine_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// timedTransport answers the exclusion check with check after checkTime and every
// notes call with aiBody after notesTime, on clock.
type timedTransport struct {
	clock                *manualClock
	check                string
	checkTime, notesTime time.Duration
}

func (tr timedTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		tr.clock.advance(tr.checkTime)
		return tr.check, nil
	}
	tr.clock.advance(tr.notesTime)
	return aiBody, nil
}

// answeringSlowly answers every gate with its default, a minute after asking.
func answeringSlowly(rec *presentertest.RecordingPresenter, clock *manualClock) {
	rec.PromptResult = func(g presenter.Gate) (presenter.Choice, error) {
		clock.advance(time.Minute)
		return g.Default, nil
	}
}

// notesSuccess is the run's one StageSucceeded for the notes.
func notesSuccess(t *testing.T, rec *presentertest.RecordingPresenter) presenter.StageSuccess {
	t.Helper()
	at := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
		return ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Key == "notes"
	})
	if at < 0 {
		t.Fatalf("no notes success; kinds = %v", rec.Kinds())
	}
	return rec.Events[at].StageSucceeded
}

// TestRelease_TheNotesAreTimedFromTheCheckDone_NeverItsCallOrItsAnswers pins that the
// notes start again, quietly, only once the check's last question is answered and the
// answers recorded, and that their time is their own work from then: neither the
// check's call nor a minute spent on each of its questions.
func TestRelease_TheNotesAreTimedFromTheCheckDone_NeverItsCallOrItsAnswers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(exportGoDiff+designDocDiff+fixturesDiff))
	f.SeedSequence("git", answersCommitCalls...)
	f.SeedSequence("git",
		ScriptedOut("M\tsrc/export.go\nA\tdocs/design/export.md\nA\ttest/fixtures/export.json\n"),
		ScriptedOut("1\t1\tsrc/export.go\n1\t0\tdocs/design/export.md\n1\t0\ttest/fixtures/export.json\n"),
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	clock := &manualClock{now: railEpoch}
	rec := &presentertest.RecordingPresenter{}
	answeringSlowly(rec, clock)
	deps := attendedDeps(rec, f)
	deps.Clock = clock.Now
	deps.Transport = timedTransport{clock: clock, check: twoSuggestions, checkTime: 3 * time.Second, notesTime: 5 * time.Second}

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := rec.Events[indexOfKind(rec, presentertest.KindNotesChecked)].NotesCheck.Elapsed; got != 3*time.Second {
		t.Errorf("check elapsed = %v, want its call's 3s", got)
	}
	if got := notesSuccess(t, rec).Elapsed; got != 5*time.Second {
		t.Errorf("notes elapsed = %v, want their call's 5s alone", got)
	}
	restart := slices.IndexFunc(rec.Events, isNotesRestart)
	recorded := indexOfKind(rec, presentertest.KindExclusionAnswersRecorded)
	lastPause := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
		return ev.Kind == presentertest.KindPrompt && isPauseGate(ev.Prompt) && ev.Prompt.Suggestion.Glob == "test/fixtures/"
	})
	if restart < 0 || recorded < 0 || lastPause < 0 || restart < recorded || restart < lastPause {
		t.Errorf("notes started again at event %d, the last pause at %d, the answers recorded at %d; want the notes after both: %v", restart, lastPause, recorded, rec.Kinds())
	}
}

// TestRelease_TheNotesLeaveTheCachedNotesQuestionOutOfTheirTime pins that a question
// asked while the notes run, the cached notes', is a wait their time leaves out.
func TestRelease_TheNotesLeaveTheCachedNotesQuestionOutOfTheirTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)
	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	clock := &manualClock{now: railEpoch}
	rec := &presentertest.RecordingPresenter{}
	answeringSlowly(rec, clock)
	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	deps.Clock = clock.Now

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !reuseConfirmPrompted(rec) {
		t.Fatal("the cached notes were never offered")
	}
	if got := notesSuccess(t, rec).Elapsed; got != 0 {
		t.Errorf("notes elapsed = %v, want none of the minute the question waited", got)
	}
}
