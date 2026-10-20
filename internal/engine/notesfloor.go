package engine

import (
	"context"
	"errors"
	"strings"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/presenter"
)

// errFloorAborted is the cause for an abort at the notes floor's choice.
var errFloorAborted = errors.New("release aborted at the notes failure")

// attended reports a run at a terminal without -y.
func (d ReleaseDeps) attended() bool {
	return !d.Yes && d.StdinInteractive
}

// reachesNotesFloor reports whether a notes failure is a real run's notes generation
// failing, which an attended run answers at the floor's choice and an unattended one
// with the next moves. A cancel is not a notes failure, and an error notes.CauseText
// does not know came from outside the generation calls.
func reachesNotesFloor(opts ReleaseOptions, failure error) bool {
	if opts.DryRun || errors.Is(failure, context.Canceled) {
		return false
	}
	_, generation := notes.CauseText(failure)
	return generation
}

// notesFloorMoves are the ways an unattended release that stopped at the floor on
// failure can still go out, then the moves that clear failure's cause. savedNotes is
// the path the failed run saved its chunk notes to, or "".
func notesFloorMoves(failure error, cfg config.Config, savedNotes string) presenter.NextMoves {
	notesFile := "write the notes to a file and release with mint release --notes-file <file>"
	if savedNotes != "" {
		notesFile = "tidy the saved notes at " + savedNotes + ", filling in any files its marker lists, and release with mint release --notes-file " + savedNotes
	}
	moves := []string{
		notesFile,
		"re-run with --no-ai to release with the commit-subject list",
		`set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`,
	}
	return presenter.NextMoves{Stage: "notes", Moves: append(moves, causeMoves(failure, cfg)...)}
}

const leanCommandMove = `if ai_command carries a large overhead of its own, use the lean command documented in the README under "The shipped default"`

// causeMoves are the moves that clear failure's cause. A merge too large gets none: a
// lower cap cuts the release into more parts and does not shrink what the merge holds.
// An OTHER failure leaves the AI's own output to name its cause, adding the lean
// command only where the release runs a command of the repo's own, whose overhead
// alone can overflow the window.
func causeMoves(failure error, cfg config.Config) []string {
	switch {
	case errors.Is(failure, notes.ErrNoRoomForDiff):
		return []string{noRoomMove(cfg.Release)}
	case errors.Is(failure, notes.ErrMergeTooLarge):
		return nil
	case errors.Is(failure, notes.ErrTooLargeForModel):
		return []string{
			"lower max_prompt_bytes, since the model accepts less than the cap",
			leanCommandMove,
			"exclude generated paths with [release].diff_exclude",
		}
	case errors.Is(failure, ai.ErrTimeout):
		return []string{"raise [release].timeout"}
	case errors.Is(failure, ai.ErrCommandMissing):
		return []string{"install the AI tool, or set ai_command"}
	case errors.Is(failure, ai.ErrGenerationFailed) && cfg.AICommandFor(config.VerbRelease) != config.DefaultAICommand:
		return []string{leanCommandMove}
	}
	return nil
}

// noRoomMove names the operator text that leaves a call no room for diff, prompt over
// context since a set prompt replaces context, or with neither set a cap too small for
// mint's own instructions.
func noRoomMove(rel config.Release) string {
	const leaveRoom = ", which every call carries in full, to leave room for the diff under max_prompt_bytes"
	switch {
	case rel.Prompt != "":
		return "shorten [release].prompt" + leaveRoom
	case rel.Context != "":
		return "shorten [release].context" + leaveRoom
	}
	return "raise max_prompt_bytes, since it is below the fixed part of mint's own instructions"
}

// notesFloorGate is the choice a failed notes generation leaves an attended release.
func notesFloorGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Release without AI notes?",
		Subject:    "notes",
		AcceptEcho: "aborted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceCommitSubjects, Action: "use the commit subjects"},
			{Key: presenter.ChoiceEdit, Action: "write in $EDITOR"},
			{Key: presenter.ChoiceAbort, Action: "abort"},
		},
		Default: presenter.ChoiceAbort,
	}
}

// notesFloor returns the body chosen at the floor's choice, re-presenting the choice
// when the editor wrote nothing or could not be launched. savedNotes is what the
// editor opens on: the chunk notes this run saved, or "". A way out with no body is
// an abort, its cause returned for the caller's unwind.
func notesFloor(ctx context.Context, deps ReleaseDeps, lastTag, savedNotes string) (string, notes.Kind, error) {
	for {
		choice, err := deps.Presenter.Prompt(notesFloorGate())
		if err != nil {
			return "", notes.KindFloor, err
		}
		switch choice {
		case presenter.ChoiceCommitSubjects:
			body, err := notes.NoAIBody(ctx, deps.Runner, lastTag)
			if err != nil {
				surfaceStageFailure(deps.Presenter, "notes", err)
				return "", notes.KindFloor, err
			}
			return body, notes.KindFloor, nil
		case presenter.ChoiceEdit:
			body, err := editBody(ctx, deps.Editor, savedNotes)
			switch {
			case errors.Is(err, ErrEditorReturnToGate):
				continue
			case err != nil:
				surfaceStageFailure(deps.Presenter, "edit", err)
				return "", notes.KindFloor, err
			case strings.TrimSpace(body) == "":
				// Closing the editor empty backs out of it; accepting an empty body would
				// ship an empty tag message, changelog entry and provider release.
				continue
			}
			return body, notes.KindFloor, nil
		case presenter.ChoiceAbort:
			return "", notes.KindFloor, errFloorAborted
		default:
			err := errUnexpectedChoice(choice)
			surfaceStageFailure(deps.Presenter, "notes", err)
			return "", notes.KindFloor, err
		}
	}
}
