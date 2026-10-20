package engine

import (
	"context"
	"errors"
	"strings"

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

// notesFloorMoves are the ways an unattended release that stopped at the floor can
// still go out. savedNotes is the path the failed run saved its chunk notes to, or "".
func notesFloorMoves(savedNotes string) presenter.NextMoves {
	notesFile := "write the notes to a file and release with mint release --notes-file <file>"
	if savedNotes != "" {
		notesFile = "tidy the saved notes at " + savedNotes + ", filling in any files its marker lists, and release with mint release --notes-file " + savedNotes
	}
	return presenter.NextMoves{
		Stage: "notes",
		Moves: []string{
			notesFile,
			"re-run with --no-ai to release with the commit-subject list",
			`set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`,
		},
	}
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
