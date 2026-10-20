package engine_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
)

const (
	raiseTimeoutMove   = "raise [release].timeout"
	installAIToolMove  = "install the AI tool, or set ai_command"
	abortConfig        = "[release]\non_notes_failure = \"abort\"\n"
	timeoutPhrase      = "AI timed out"
	notInstalledPhrase = "AI tool not installed"
)

var (
	errTimedOut  = fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)
	errNotOnPath = fmt.Errorf("%w: claude", ai.ErrCommandMissing)
	alwaysMoves  = []string{writeNotesMove, noAIMove, fallbackMove}
)

func TestRelease_NotesFloorCauseMoves_SinglePassTimeoutOrMissingToolAddsItsMove(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		failure    error
		wantPhrase string
		wantMove   string
	}{
		{name: "timeout", failure: errTimedOut, wantPhrase: timeoutPhrase, wantMove: raiseTimeoutMove},
		{name: "tool not installed", failure: errNotOnPath, wantPhrase: notInstalledPhrase, wantMove: installAIToolMove},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, abortConfig)
			_, rec, deps := singlePassFailingRun(t, root, tc.failure)
			deps.Yes = true

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			if sf := onlyStageFailureEvent(t, rec); sf.Message != tc.wantPhrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase)
			}
			assertStopsWithNextMoves(t, rec, err, append(alwaysMoves, tc.wantMove))
		})
	}
}

func TestRelease_NotesFloorCauseMoves_ChunkOrMergeTimeoutAddsRaiseTheTimeout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		fails      func(prompt string) bool
		wantPhrase string
	}{
		{name: "a chunk call times out", fails: isSecondPartPrompt, wantPhrase: timeoutPhrase + " (part 2 of 2)"},
		{name: "the merge call times out", fails: isMergeCallPrompt, wantPhrase: timeoutPhrase + " (merging)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fails := tc.fails
			run := newChunkNotesRun(t, "abort", twoPartTransport(func(prompt string) error {
				if fails(prompt) {
					return errTimedOut
				}
				return nil
			}))
			run.deps.Yes = true

			err := run.release(t, t.Context())

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != tc.wantPhrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase)
			}
			path := run.savedPath(nextTag)
			assertStopsWithNextMoves(t, run.rec, err, []string{tidySavedNotesMove(path), noAIMove, fallbackMove, raiseTimeoutMove})
		})
	}
}

// otherFailureCases are failures that take the OTHER path, with output or without.
func otherFailureCases() []struct {
	name       string
	failure    error
	wantPhrase string
} {
	return []struct {
		name       string
		failure    error
		wantPhrase string
	}{
		{name: "with output", failure: &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}, wantPhrase: "AI call failed"},
		{name: "with empty output", failure: &ai.GenerationError{Stderr: " \n", ExitCode: 1}, wantPhrase: "AI returned empty notes"},
	}
}

func TestRelease_NotesFloorCauseMoves_OtherFailureWithTheShippedCommandTakesOnlyTheAlwaysMoves(t *testing.T) {
	t.Parallel()

	for _, config := range []struct {
		name string
		toml string
	}{
		{name: "no ai_command set", toml: abortConfig},
		{name: "only [commit].ai_command set", toml: abortConfig + "\n[commit]\nai_command = \"llm -m gpt-4o\"\n"},
		{name: "blank ai_command at both levels", toml: "ai_command = \"  \"\n\n" + abortConfig + "ai_command = \"\"\n"},
	} {
		for _, tc := range otherFailureCases() {
			t.Run(config.name+", "+tc.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				writeConfig(t, root, config.toml)
				_, rec, deps := singlePassFailingRun(t, root, tc.failure)
				deps.Yes = true

				err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

				sf := onlyStageFailureEvent(t, rec)
				if sf.Message != tc.wantPhrase {
					t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase)
				}
				var genErr *ai.GenerationError
				if errors.As(tc.failure, &genErr) && sf.Output != genErr.Output() {
					t.Errorf("StageFailure.Output = %q, want the AI's output %q verbatim", sf.Output, genErr.Output())
				}
				assertStopsWithNextMoves(t, rec, err, alwaysMoves)
			})
		}
	}
}

func TestRelease_NotesFloorCauseMoves_OtherFailureWithTheRepoOwnCommandAddsTheLeanCommandPointer(t *testing.T) {
	t.Parallel()

	for _, config := range []struct {
		name string
		toml string
	}{
		{name: "[release].ai_command set", toml: abortConfig + "ai_command = \"llm -m gpt-4o\"\n"},
		{name: "shared ai_command set, none under [release]", toml: "ai_command = \"llm -m gpt-4o\"\n\n" + abortConfig},
		{name: "shared ai_command set, blank under [release]", toml: "ai_command = \"llm -m gpt-4o\"\n\n" + abortConfig + "ai_command = \" \"\n"},
	} {
		for _, tc := range otherFailureCases() {
			t.Run(config.name+", "+tc.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				writeConfig(t, root, config.toml)
				_, rec, deps := singlePassFailingRun(t, root, tc.failure)
				deps.Yes = true

				err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

				if sf := onlyStageFailureEvent(t, rec); sf.Message != tc.wantPhrase {
					t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase)
				}
				assertStopsWithNextMoves(t, rec, err, append(alwaysMoves, leanCommandMove))
				assertNoClaudeFlags(t, rec)
			})
		}
	}
}

func TestRelease_NotesFloorCauseMoves_TimeoutWithTheRepoOwnCommandAddsNoLeanCommandPointer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, abortConfig+"ai_command = \"llm -m gpt-4o\"\n")
	_, rec, deps := singlePassFailingRun(t, root, errTimedOut)
	deps.Yes = true

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertStopsWithNextMoves(t, rec, err, append(alwaysMoves, raiseTimeoutMove))
}

func TestRelease_NotesFloorCauseMoves_MergeTooLargeWithTheRepoOwnCommandTakesOnlyTheAlwaysMoves(t *testing.T) {
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
			writeConfig(t, run.root, "ai_command = \"llm -m gpt-4o\"\nmax_prompt_bytes = 20000\n\n"+abortConfig)
			run.deps.Yes = true

			err := run.release(t, t.Context())

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != mergeTooLargePhrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, mergeTooLargePhrase)
			}
			assertStopsWithNextMoves(t, run.rec, err, []string{tidySavedNotesMove(run.savedPath(nextTag)), noAIMove, fallbackMove})
		})
	}
}

func assertNoClaudeFlags(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	for _, moves := range nextMovesEvents(rec) {
		for _, move := range moves.Moves {
			for _, flag := range []string{"--safe-mode", "--tools", "--strict-mcp-config", "--no-session-persistence", "--system-prompt", "--model"} {
				if strings.Contains(move, flag) {
					t.Errorf("move %q names the claude flag %s", move, flag)
				}
			}
		}
	}
}
