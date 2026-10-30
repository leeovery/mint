package engine

import (
	"context"
	"errors"
	"strings"

	"mint/internal/cmdline"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/presenter"
)

// errFloorAborted is the cause for an abort at the notes floor's choice.
var errFloorAborted = errors.New("aborted at the notes failure")

// attended reports a run at a terminal without -y.
func (d ReleaseDeps) attended() bool {
	return !d.Yes && d.StdinInteractive
}

// asksAtTerminal reports whether the run is offered the notes floor's choice and the
// exclusion check's pauses: it is attended, and a dry run asks nothing.
func (d ReleaseDeps) asksAtTerminal(opts ReleaseOptions) bool {
	return d.attended() && !opts.DryRun
}

// reachesNotesFloor reports whether a notes failure is the notes generation failing,
// which an attended real run answers at the floor's choice and any other run with the
// next moves. A cancel is not a notes failure, and an error notes.ClassifyFailure does
// not know came from outside the generation calls.
func reachesNotesFloor(failure error) bool {
	if errors.Is(failure, context.Canceled) {
		return false
	}
	_, generation := notes.ClassifyFailure(failure)
	return generation
}

// notesFloorMoves are the ways a release that stopped at the floor on failure can
// still go out, then the moves that clear failure's cause. The fallback move shows
// only unattended: at a terminal without -y the real run offers the choice instead of
// stopping. savedNotes is the path the failed run saved its chunk notes to, or "".
func (d ReleaseDeps) notesFloorMoves(failure error, cfg config.Config, savedNotes string) presenter.NextMoves {
	moves := []presenter.NextMove{
		notesFileMove("mint release", savedNotes),
		{Lead: "re-run with --no-ai to release with the commit-subject list"},
	}
	if !d.attended() {
		moves = append(moves, presenter.NextMove{Lead: `set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`})
	}
	return presenter.NextMoves{Stage: "notes", Moves: append(moves, causeMoves(failure, cfg)...)}
}

// regenerateNotesFloorMoves are the ways to put notes on version, already out, after an
// unattended regenerate stopped at the floor on failure, then the moves that clear
// failure's cause. Neither --no-ai nor on_notes_failure exists for a regenerate.
func regenerateNotesFloorMoves(failure error, cfg config.Config, version, savedNotes string) presenter.NextMoves {
	moves := []presenter.NextMove{notesFileMove("mint release regenerate "+version, savedNotes)}
	return presenter.NextMoves{Stage: "notes", Moves: append(moves, causeMoves(failure, cfg)...)}
}

// notesFileMove is the --notes-file route through command: the chunk notes saved at
// savedNotes, once tidied, or with none saved, notes written from scratch. Its
// command ends the move, so it stands apart from the lead.
func notesFileMove(command, savedNotes string) presenter.NextMove {
	if savedNotes == "" {
		return presenter.NextMove{Lead: "write the notes to a file and release with", Command: command + " --notes-file <file>"}
	}
	return presenter.NextMove{
		Lead:    "tidy the saved notes at " + savedNotes + ", filling in any files its marker lists, and release with",
		Command: notesFileCommand(command, savedNotes),
	}
}

// notesFileCommand is base run with --notes-file path. The reader pastes the command
// into a shell, so path is shell-quoted where a byte in it needs it; a saved path shown
// in prose rather than passed stays raw.
func notesFileCommand(base, path string) string {
	return base + " --notes-file " + cmdline.Quote(path)
}

const leanCommandMove = `if ai_command carries a large overhead of its own, use the lean command documented in the README under "The shipped default"`

// causeMoves are the moves that clear failure's cause, each a lead alone: none ends
// in a command. A merge too large gets none: a
// lower cap cuts the release into more parts and does not shrink what the merge holds.
// An OTHER failure leaves the AI's own output to name its cause, adding the lean
// command only where the release runs a command of the repo's own, whose overhead
// alone can overflow the window.
func causeMoves(failure error, cfg config.Config) []presenter.NextMove {
	leads := causeLeads(failure, cfg)
	moves := make([]presenter.NextMove, len(leads))
	for i, lead := range leads {
		moves[i] = presenter.NextMove{Lead: lead}
	}
	return moves
}

func causeLeads(failure error, cfg config.Config) []string {
	cause, known := notes.ClassifyFailure(failure)
	if !known {
		return nil
	}
	switch cause {
	case notes.CauseNoRoomForDiff:
		return []string{noRoomMove(cfg.Release)}
	case notes.CauseMergeTooLarge:
		return nil
	case notes.CauseTooLargeForModel:
		return []string{
			"lower max_prompt_bytes, since the model accepts less than the cap",
			leanCommandMove,
			"exclude generated paths with [release].diff_exclude",
		}
	case notes.CauseTimeout:
		return []string{"raise [release].timeout"}
	case notes.CauseCommandMissing:
		return []string{"install the AI tool, or set ai_command"}
	case notes.CauseGenerationFailed:
		if cfg.AICommandFor(config.VerbRelease) != config.DefaultAICommand {
			return []string{leanCommandMove}
		}
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

// notesFloorGate is the choice a failed notes generation leaves an attended run.
func notesFloorGate(question string) presenter.Gate {
	return presenter.Gate{
		Question:   question,
		Subject:    "notes",
		AcceptEcho: "aborted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceCommitSubjects, Action: "use the commit subjects"},
			{Key: presenter.ChoiceEdit, Action: "write in $EDITOR"},
			{Key: presenter.ChoiceAbort, Action: "abort"},
		},
		Default: presenter.ChoiceAbort,
		Stage:   stageNotes,
	}
}

// notesFloor returns the body chosen at gate, the floor's choice, re-presenting it
// when the editor wrote nothing or could not be launched. subjects builds the
// commit-subject list; savedNotes is what the editor opens on: the chunk notes this
// run saved, or "". A way out with no body is an abort, its cause returned for the
// caller to unwind or abort on.
func notesFloor(ctx context.Context, deps ReleaseDeps, gate presenter.Gate, subjects func(context.Context) (string, error), savedNotes string) (string, error) {
	for {
		choice, err := deps.Presenter.Prompt(gate)
		if err != nil {
			return "", err
		}
		switch choice {
		case presenter.ChoiceCommitSubjects:
			body, err := subjects(ctx)
			if err != nil {
				surfaceStageFailure(deps.Presenter, "notes", err)
				return "", err
			}
			return body, nil
		case presenter.ChoiceEdit:
			body, err := editBody(ctx, deps.Editor, savedNotes)
			switch {
			case errors.Is(err, ErrEditorReturnToGate):
				continue
			case err != nil:
				surfaceStageFailure(deps.Presenter, "edit", err)
				return "", err
			case strings.TrimSpace(body) == "":
				// Closing the editor empty backs out of it; accepting an empty body would
				// ship an empty tag message, changelog entry and provider release.
				continue
			}
			return body, nil
		case presenter.ChoiceAbort:
			return "", errFloorAborted
		default:
			err := errUnexpectedChoice(choice)
			surfaceStageFailure(deps.Presenter, "notes", err)
			return "", err
		}
	}
}

// releaseNotesFloor is the floor's choice on a release, its commit subjects those
// since lastTag.
func releaseNotesFloor(ctx context.Context, deps ReleaseDeps, lastTag, savedNotes string) (string, notes.Kind, error) {
	subjects := func(ctx context.Context) (string, error) { return notes.NoAIBody(ctx, deps.Runner, lastTag) }
	body, err := notesFloor(ctx, deps, notesFloorGate("Release without AI notes?"), subjects, savedNotes)
	return body, notes.KindFloor, err
}

// regenerateNotesFloor is the floor's choice on a single-version regenerate, its
// commit subjects those VersionNoAIBody lists over the version's own range.
func regenerateNotesFloor(ctx context.Context, deps ReleaseDeps, req RegenerateRunRequest, savedNotes string) (string, error) {
	subjects := func(ctx context.Context) (string, error) {
		return notes.VersionNoAIBody(ctx, deps.Runner, req.DiffRange, req.Tag)
	}
	body, err := notesFloor(ctx, deps, notesFloorGate("Regenerate without AI notes?"), subjects, savedNotes)
	if err != nil {
		return "", abort(err)
	}
	return body, nil
}
