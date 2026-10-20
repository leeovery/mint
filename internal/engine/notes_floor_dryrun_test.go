package engine_test

import (
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

func TestRelease_DryRunNotesFloor_AttendedAsksNothingAndPrintsTheMovesWithoutFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\non_notes_failure = \"abort\"\n")
	f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})

	err := engine.Release(t.Context(), deps, dryRunOptions())

	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != loginOutput {
		t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", loginOutput)
	}
	if sf.Recoverable {
		t.Errorf("StageFailure.Recoverable = true, want a dry run's failure to offer no choice")
	}
	if got := promptGates(rec); len(got) != 0 {
		t.Errorf("a dry run asked: %+v", got)
	}
	assertStopsWithNextMoves(t, rec, err, []string{writeNotesMove, noAIMove})
	assertNoTag(t, f)
}

func TestRelease_DryRunNotesFloor_UnattendedAddsTheFallbackMove(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		yes              bool
		stdinInteractive bool
	}{
		{name: "-y at a terminal", yes: true, stdinInteractive: true},
		{name: "stdin not a terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release]\non_notes_failure = \"abort\"\n")
			_, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
			deps.Yes = tc.yes
			deps.StdinInteractive = tc.stdinInteractive

			err := engine.Release(t.Context(), deps, dryRunOptions())

			assertStopsWithNextMoves(t, rec, err, []string{writeNotesMove, noAIMove, fallbackMove})
		})
	}
}

func TestRelease_DryRunNotesFloor_ChunkedFailureSavesTheChunkNotesAndNamesThem(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))
	run.deps.StdinInteractive = true

	err := engine.Release(t.Context(), run.deps, dryRunOptions())

	path := run.savedPath(nextTag)
	if got := readSaved(t, path); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want part 1's notes %q", got, partOneSaved)
	}
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
	}
	assertStopsWithNextMoves(t, run.rec, err, []string{tidySavedNotesMove(path), noAIMove})
}

func TestRelease_DryRunNotesFloor_TimeoutAddsRaiseTheTimeout(t *testing.T) {
	t.Parallel()

	_, rec, deps := singlePassFailingRun(t, t.TempDir(), errTimedOut)

	err := engine.Release(t.Context(), deps, dryRunOptions())

	if sf := onlyStageFailureEvent(t, rec); sf.Message != timeoutPhrase {
		t.Errorf("StageFailure.Message = %q, want %q", sf.Message, timeoutPhrase)
	}
	assertStopsWithNextMoves(t, rec, err, []string{writeNotesMove, noAIMove, raiseTimeoutMove})
}

func TestRelease_DryRunNotesFloor_HalvingRunsOutAddsTheSizeMoves(t *testing.T) {
	t.Parallel()

	tc := halvingOutCases()[0]
	root := t.TempDir()
	f := runner.NewFakeRunner()
	tc.seedRelease(t, root, f, "abort")
	rec := &presentertest.RecordingPresenter{}
	tr := &firstPartOverflowTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr
	deps.Yes = true

	err := engine.Release(t.Context(), deps, dryRunOptions())

	if sf := onlyStageFailureEvent(t, rec); sf.Message != tc.wantPhrase(t, tr) {
		t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase(t, tr))
	}
	assertStopsWithNextMoves(t, rec, err, withSizeMoves(writeNotesMove, noAIMove, fallbackMove))
}

func TestRelease_DryRunNotesFloor_UnsavedChunkNotesWarnAndNameNoPath(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))
	run.deps.StdinInteractive = true
	run.deps.ChunkNotes = failingChunkNotesSaver{}

	err := engine.Release(t.Context(), run.deps, dryRunOptions())

	assertStopsWithNextMoves(t, run.rec, err, []string{writeNotesMove, noAIMove})
	got := warnings(run.rec)
	if len(got) != 1 || got[0].Label != "notes" || got[0].Message != "the finished chunk notes could not be saved" || got[0].Output != "mkdir /cache: permission denied" {
		t.Errorf("warnings = %+v, want the one saying the chunk notes could not be saved, with the write error verbatim", got)
	}
}
