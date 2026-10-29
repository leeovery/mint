package engine_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	lowerCapMove         = "lower max_prompt_bytes, since the model accepts less than the cap"
	leanCommandMove      = `if ai_command carries a large overhead of its own, use the lean command documented in the README under "The shipped default"`
	excludeGeneratedMove = "exclude generated paths with [release].diff_exclude"
	shortenPromptMove    = "shorten [release].prompt, which every call carries in full, to leave room for the diff under max_prompt_bytes"
	shortenContextMove   = "shorten [release].context, which every call carries in full, to leave room for the diff under max_prompt_bytes"
	raiseCapMove         = "raise max_prompt_bytes, since it is below the fixed part of mint's own instructions"
)

var sizeMoves = []string{lowerCapMove, leanCommandMove, excludeGeneratedMove}

func withSizeMoves(always ...string) []string {
	return append(always, sizeMoves...)
}

// partContext is operator text that leaves a chunk call under partContextCap room
// for diff, but none at half that call's size.
var partContext = strings.Repeat("Project context the operator wrote at length.\n", 260)

// partContextCap scales with the default prompt so a chunk call carrying partContext
// keeps that room as the prompt's wording changes.
var partContextCap = len(notes.DefaultPrompt) + 13_400

func TestRelease_NotesFloorSizeMoves_HalvingRunsOutAddsTheSizeMoves(t *testing.T) {
	t.Parallel()

	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seedRelease(t, root, f, "abort")
			rec := &presentertest.RecordingPresenter{}
			tr := &firstPartOverflowTransport{}
			deps := newDeps(rec, f)
			deps.Transport = tr
			deps.Yes = true

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			if sf := onlyStageFailureEvent(t, rec); sf.Message != tc.wantPhrase(t, tr) {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase(t, tr))
			}
			assertStopsWithNextMoves(t, rec, err, withSizeMoves(writeNotesMove, noAIMove, fallbackMove))
		})
	}
}

func TestRelease_NotesFloorSizeMoves_PartHalvingRoundWithNoRoomAddsTheSizeMoves(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	writeFile(t, root, "context.md", partContext)
	writeConfig(t, root, fmt.Sprintf("max_prompt_bytes = %d\n\n[release]\non_notes_failure = \"abort\"\ncontext = \"context.md\"\n", partContextCap))
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
	rec := &presentertest.RecordingPresenter{}
	tr := &firstPartOverflowTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr
	deps.Yes = true

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	if tr.overflowed != 1 {
		t.Errorf("overflowed calls = %d, want the one chunk call whose halving has no room", tr.overflowed)
	}
	sf := onlyStageFailureEvent(t, rec)
	if !strings.HasPrefix(sf.Message, tooLargePhrase+" (part 1 of ") {
		t.Errorf("StageFailure.Message = %q, want %q on part 1", sf.Message, tooLargePhrase)
	}
	assertStopsWithNextMoves(t, rec, err, withSizeMoves(writeNotesMove, noAIMove, fallbackMove))
}

func TestRelease_NotesFloorSizeMoves_LeanCommandMovePointsAtTheREADMEWithoutClaudeFlags(t *testing.T) {
	t.Parallel()

	tc := halvingOutCases()[0]
	root := t.TempDir()
	f := runner.NewFakeRunner()
	tc.seedRelease(t, root, f, "abort")
	rec := &presentertest.RecordingPresenter{}
	deps := newDeps(rec, f)
	deps.Transport = &firstPartOverflowTransport{}
	deps.Yes = true

	_ = engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	moves := nextMovesEvents(rec)
	if len(moves) != 1 || !slices.Contains(moveTexts(moves[0].Moves), leanCommandMove) {
		t.Fatalf("NextMoves = %+v, want the lean-command move", moves)
	}
	for _, move := range moveTexts(moves[0].Moves) {
		for _, flag := range []string{"--safe-mode", "--tools", "--strict-mcp-config", "--no-session-persistence", "--system-prompt", "--model"} {
			if strings.Contains(move, flag) {
				t.Errorf("move %q names the claude flag %s", move, flag)
			}
		}
	}
}

func TestRelease_NotesFloorSizeMoves_NoRoomNamesTheTextToShortenOrRaisesTheCap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		config  string
		context bool
		want    string
	}{
		{name: "prompt set", config: "max_prompt_bytes = 20000\n\n[release]\nprompt = \"context.md\"\n", context: true, want: shortenPromptMove},
		{
			name:    "prompt and context set",
			config:  "max_prompt_bytes = 20000\n\n[release]\nprompt = \"context.md\"\ncontext = \"context.md\"\n",
			context: true,
			want:    shortenPromptMove,
		},
		{name: "context set", config: "max_prompt_bytes = 20000\n\n[release]\ncontext = \"context.md\"\n", context: true, want: shortenContextMove},
		{name: "neither set, cap below mint's own instructions", config: "max_prompt_bytes = 1000\n\n[release]\n", want: raiseCapMove},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			uncallable := uncallableCase{config: tc.config}
			if tc.context {
				uncallable.context = outsizedContext
			}
			uncallable.seedRelease(t, root, f, "abort")
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			deps.Yes = true

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			if sf := onlyStageFailureEvent(t, rec); sf.Message != noRoomPhrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, noRoomPhrase)
			}
			assertStopsWithNextMoves(t, rec, err, []string{writeNotesMove, noAIMove, fallbackMove, tc.want})
		})
	}
}

// mergeTooLargeTransport writes a two-part run's part notes, then fails its merge as
// too large: by the cap, when the part notes overflow it, or by the model, which
// rejects the merge call and has its failure classified SIZE.
func mergeTooLargeTransport(partNotes string) func(context.Context, string) (string, error) {
	return func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "SIZE", nil
		case isMergeCallPrompt(prompt):
			return "", &ai.GenerationError{Stdout: modelRejectsSize, ExitCode: 1}
		default:
			return partNotes, nil
		}
	}
}

func TestRelease_NotesFloorSizeMoves_MergeTooLargeTakesOnlyTheAlwaysMoves(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		partNotes string
	}{
		{name: "merge prompt over the cap", partNotes: bigPartNotes},
		{name: "merge call classified SIZE", partNotes: apiPartNotes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newChunkNotesRun(t, "abort", mergeTooLargeTransport(tc.partNotes))
			run.deps.Yes = true

			err := run.release(t, t.Context())

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != mergeTooLargePhrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, mergeTooLargePhrase)
			}
			path := run.savedPath(nextTag)
			assertStopsWithNextMoves(t, run.rec, err, []string{tidySavedNotesMove(path), noAIMove, fallbackMove})
		})
	}
}
