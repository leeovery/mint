package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

const (
	writeNotesMove = "write the notes to a file and release with mint release --notes-file <file>"
	noAIMove       = "re-run with --no-ai to release with the commit-subject list"
	fallbackMove   = `set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`
)

func tidySavedNotesMove(path string) string {
	return "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with mint release --notes-file " + path
}

func nextMovesEvents(rec *presentertest.RecordingPresenter) []presenter.NextMoves {
	var moves []presenter.NextMoves
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNextMoves {
			moves = append(moves, ev.NextMoves)
		}
	}
	return moves
}

// assertStopsWithNextMoves checks the run printed want as the notes stage's only next
// moves, after its ✗ line and followed by nothing but the unwind, without the floor's
// choice, and ended with exit 1.
func assertStopsWithNextMoves(t *testing.T, rec *presentertest.RecordingPresenter, err error, want []string) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort with exit 1", err)
	}
	got := nextMovesEvents(rec)
	if len(got) != 1 {
		t.Fatalf("NextMoves events = %+v, want one; kinds = %v", got, rec.Kinds())
	}
	if got[0].Stage != "notes" || !slices.Equal(got[0].Moves, want) {
		t.Errorf("NextMoves = %q %q, want notes %q", got[0].Stage, got[0].Moves, want)
	}
	failedAt := indexOfKind(rec, presentertest.KindStageFailed)
	movesAt := indexOfKind(rec, presentertest.KindNextMoves)
	if failedAt == -1 || failedAt >= movesAt {
		t.Errorf("want the next moves after the ✗ line; kinds = %v", rec.Kinds())
	}
	for _, ev := range rec.Events[movesAt+1:] {
		if ev.Kind != presentertest.KindUnwound {
			t.Errorf("want nothing but the unwind after the next moves; kinds = %v", rec.Kinds())
		}
	}
	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("an unattended run was offered the choice: %+v", gates)
	}
}

func TestRelease_NotesFloorNextMoves_UnattendedSinglePassFailurePrintsThemBeneathTheOutput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		stdinInteractive bool
	}{
		{name: "stdin a terminal", stdinInteractive: true},
		{name: "stdin not a terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release]\non_notes_failure = \"abort\"\n")
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
			deps.Yes = true
			deps.StdinInteractive = tc.stdinInteractive

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != loginOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", loginOutput)
			}
			assertStopsWithNextMoves(t, rec, err, []string{writeNotesMove, noAIMove, fallbackMove})
			assertNoTag(t, f)
		})
	}
}

func TestRelease_NotesFloorNextMoves_SavedChunkNotesAreTheNotesFileRoute(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsMerge))
	run.deps.Yes = true

	err := run.release(t, t.Context())

	path := run.savedPath(nextTag)
	assertStopsWithNextMoves(t, run.rec, err, []string{tidySavedNotesMove(path), noAIMove, fallbackMove})
}

type failingChunkNotesSaver struct{}

func (failingChunkNotesSaver) Save(string, string, string) (string, error) {
	return "", errors.New("mkdir /cache: permission denied")
}

func TestRelease_NotesFloorNextMoves_UnsavedChunkNotesNameNoPath(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))
	run.deps.Yes = true
	run.deps.ChunkNotes = failingChunkNotesSaver{}

	err := run.release(t, t.Context())

	assertStopsWithNextMoves(t, run.rec, err, []string{writeNotesMove, noAIMove, fallbackMove})
	if got := len(warnings(run.rec)); got != 1 {
		t.Errorf("warnings = %d, want the one saying the chunk notes could not be saved", got)
	}
}

func TestRelease_NotesFloorNextMoves_AttendedRunGetsTheChoiceInstead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})

	assertAbortNonZero(t, engine.Release(t.Context(), deps, priorTagNormalAIOptions()))

	if got := nextMovesEvents(rec); len(got) != 0 {
		t.Errorf("an attended run printed next moves: %+v", got)
	}
	assertFloorFollowsFailure(t, rec)
}

func TestRelease_NotesFloorNextMoves_UnattendedCancelPrintsNone(t *testing.T) {
	t.Parallel()

	_, rec, deps := singlePassFailingRun(t, t.TempDir(), context.Canceled)
	deps.Yes = true

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if got := nextMovesEvents(rec); len(got) != 0 {
		t.Errorf("a cancel printed next moves: %+v", got)
	}
}
