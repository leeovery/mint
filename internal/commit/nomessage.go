package commit

import (
	"errors"
	"fmt"
	"strings"

	"mint/internal/ai"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// noAIMessage is why a run has no AI message to review, which the editor fallback
// stands in for: why, as the fallback's failure says it, empty under --no-ai, which
// asked for none, and a failed AI call's captured output.
type noAIMessage struct {
	why    string
	output string
}

// The runs with no AI message that no AI call failed.
var (
	noAIAsked       = noAIMessage{}
	diffTooLarge    = noAIMessage{why: "the diff is too large for the AI"}
	changesExcluded = noAIMessage{why: "every change is excluded from the AI"}
)

// aiFailed is why a failed AI call left no message.
func aiFailed(err error) noAIMessage {
	return noAIMessage{
		why:    "the AI could not write the message (" + aiFailurePhrase(err) + ")",
		output: aiFailureOutput(err),
	}
}

// aiFailurePhrase is a failed AI call's cause in a few words.
func aiFailurePhrase(err error) string {
	switch {
	case errors.Is(err, ai.ErrTimeout):
		return "it timed out"
	case errors.Is(err, ai.ErrCommandMissing):
		return "its command is not installed"
	case aiFailureOutput(err) != "":
		return "its call failed"
	default:
		return "it returned an empty message"
	}
}

// failEditorUnopenable fails the message node where the editor the fallback opens
// could not be opened: why the run has no AI message, why it has no editor, and the
// ways on from there. A failed AI call's output is not repeated: the warn that said
// the editor was opening showed it. mint commit has no -m, so the ways on are an
// editor mint can run, or git's own commit.
func failEditorUnopenable(p presenter.Presenter, deps Deps, missing noAIMessage, err error) error {
	editor, output := unopenedEditor(err)
	message := editor
	if missing.why != "" {
		message = missing.why + ", and " + editor
	}
	cause := surfaceFailure(p, presenter.StageFailure{Name: "editor", Message: message, Output: output}, fmt.Errorf("%s: %w", message, err))
	p.NextMoves(presenter.NextMoves{Stage: "editor", Moves: []presenter.NextMove{
		{Lead: "Set an editor mint can run, in GIT_EDITOR, core.editor, VISUAL or EDITOR (git falls back to vi), then run it again:", Command: commitRerun(deps)},
		{Lead: "Or commit with a message of your own:", Command: ownMessageCommit(deps)},
	}})
	return cause
}

// unopenedEditor says why err left no editor open, and what the launch said where it
// never started. mint runs the editor itself with no shell, so a command that leans
// on one is told so.
func unopenedEditor(err error) (string, string) {
	var unopened *editorOpenError
	if !errors.As(err, &unopened) {
		return "git names no editor to open", ""
	}
	if unopened.words == nil {
		return fmt.Sprintf("the editor command %q could not be read (%v)", unopened.command, unopened.err), ""
	}
	why, output := "not found", ""
	if errors.Is(err, runner.ErrNotStarted) {
		why, output = "it failed to start", unopened.err.Error()
	}
	if unopened.shellOnly() {
		why += "; mint runs it without a shell, so ~ and $VARS are not expanded"
	}
	return fmt.Sprintf("the editor %q could not be opened (%s)", unopened.words[0], why), output
}

// commitRerun is the command that runs this commit again.
func commitRerun(deps Deps) string {
	command := []string{"mint", "commit"}
	switch deps.Staging {
	case All:
		command = append(command, "-a")
	case AddAll:
		command = append(command, "-A")
	}
	if deps.Push {
		command = append(command, "-p")
	}
	if deps.NoAI {
		command = append(command, "--no-ai")
	}
	return strings.Join(command, " ")
}

// ownMessageCommit is git's commit of what this one would carry, with a message the
// user writes in place of <message>, pushed where this one would be.
func ownMessageCommit(deps Deps) string {
	commit := `git commit -m "<message>"`
	switch deps.Staging {
	case All:
		commit = `git commit -a -m "<message>"`
	case AddAll:
		commit = "git add -A && " + commit
	}
	if deps.Push {
		commit += " && git push"
	}
	return commit
}
