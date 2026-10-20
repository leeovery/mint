package engine_test

import (
	"errors"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// failingChunkNotes is a chunk-notes store whose every save fails with err.
type failingChunkNotes struct{ err error }

func (s failingChunkNotes) Save(string, string, string) (string, error) { return "", s.err }

func unresolvableUserCacheDir(err error) *notescache.ChunkNotes {
	return notescache.NewUserChunkNotes(func() (string, error) { return "", err })
}

var errChunkNotesWrite = errors.New("writing chunk notes: open /cache/mint/chunk-notes/v1.2.4.md: no space left on device")

func unsavedChunkNotesWarning(output string) presenter.Warning {
	return presenter.Warning{Label: "notes", Message: "the finished chunk notes could not be saved", Output: output}
}

func warnings(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, e := range rec.Events {
		if e.Kind == presentertest.KindWarn {
			warns = append(warns, e.Warn)
		}
	}
	return warns
}

func assertOnlyWarning(t *testing.T, rec *presentertest.RecordingPresenter, want presenter.Warning) {
	t.Helper()
	if got := warnings(rec); len(got) != 1 || got[0] != want {
		t.Errorf("warnings = %+v, want only %+v", got, want)
	}
}

// assertWarnedAfterTheFailure fails unless the run's one warning comes after its ✗
// line and before its unwind.
func assertWarnedAfterTheFailure(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	failed, warned, unwound := -1, -1, len(rec.Events)
	for i, e := range rec.Events {
		switch e.Kind {
		case presentertest.KindStageFailed:
			failed = i
		case presentertest.KindWarn:
			warned = i
		case presentertest.KindUnwound:
			unwound = i
		}
	}
	if failed < 0 || warned < failed || warned > unwound {
		t.Errorf("event kinds = %v, want the warning after StageFailed and before any Unwound", rec.Kinds())
	}
}

func TestRelease_ChunkNotesCannotBeSaved_Abort_KeepsTheAIsFailureAndWarnsWithTheErrorVerbatim(t *testing.T) {
	t.Parallel()

	unresolved := errors.New("neither $XDG_CACHE_HOME nor $HOME are defined")
	cases := []struct {
		name   string
		store  engine.ChunkNotesSaver
		output string
	}{
		{name: "the write fails", store: failingChunkNotes{err: errChunkNotesWrite}, output: errChunkNotesWrite.Error()},
		{name: "the user cache dir cannot be resolved", store: unresolvableUserCacheDir(unresolved), output: unresolved.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))
			run.deps.ChunkNotes = tc.store

			assertAbortNonZero(t, run.release(t, t.Context()))

			assertNoMutation(t, run.f)
			sf := onlyStageFailureEvent(t, run.rec)
			if sf.Message != "AI call failed (part 2 of 2)" || sf.Output != usageLimitOutput {
				t.Errorf("StageFailure = %q with output %q, want the AI's failure unchanged", sf.Message, sf.Output)
			}
			if sf.SavedNotes != "" {
				t.Errorf("StageFailure.SavedNotes = %q, want no path named", sf.SavedNotes)
			}
			assertOnlyWarning(t, run.rec, unsavedChunkNotesWarning(tc.output))
			assertWarnedAfterTheFailure(t, run.rec)
		})
	}
}

func TestRelease_ChunkNotesCannotBeWritten_Fallback_WarnsAndShipsTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	run := newChunkNotesRun(t, "fallback", twoPartTransport(failsSecondPart))
	run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}
	run.f.SeedSequence("git", ScriptedOut(subjects)) // log --format=%s
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)

	if err := run.release(t, t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertOnlyWarning(t, run.rec, unsavedChunkNotesWarning(errChunkNotesWrite.Error()))
	if body := tagAnnotationBody(t, run.f, nextTag); body != subjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", body, subjects)
	}
}

func TestRelease_NoChunkFinished_WarnsNothingAboutSavingChunkNotes(t *testing.T) {
	t.Parallel()

	failsEveryPart := func(prompt string) error {
		if isChunkCallPrompt(prompt) {
			return &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
		}
		return nil
	}
	cases := []struct {
		name  string
		store func(run *chunkNotesRun) engine.ChunkNotesSaver
	}{
		{name: "the user cache dir resolves", store: func(run *chunkNotesRun) engine.ChunkNotesSaver {
			return notescache.NewUserChunkNotes(func() (string, error) { return run.userCache, nil })
		}},
		{name: "the user cache dir cannot be resolved", store: func(*chunkNotesRun) engine.ChunkNotesSaver {
			return unresolvableUserCacheDir(errors.New("neither $XDG_CACHE_HOME nor $HOME are defined"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newChunkNotesRun(t, "abort", twoPartTransport(failsEveryPart))
			run.deps.ChunkNotes = tc.store(run)

			assertAbortNonZero(t, run.release(t, t.Context()))

			if got := warnings(run.rec); len(got) != 0 {
				t.Errorf("warnings = %+v, want none", got)
			}
			assertNoSavedChunkNotes(t, run.chunkNotesBase())
		})
	}
}
