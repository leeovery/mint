package engine

import (
	"strings"

	"mint/internal/presenter"
)

// manualStep is a recovery step a failed unwind leaves the user: what it does, the
// commands that do it in order (none where mint can name no command), and why, where
// that needs saying.
type manualStep struct {
	does     string
	commands []string
	why      string
}

// text is the step as plain's sentence has always read it: what it does, its commands
// in backticks, then why.
func (m manualStep) text() string {
	text := m.does
	if len(m.commands) > 0 {
		text += " (`" + strings.Join(m.commands, "` then `") + "`)"
	}
	if m.why != "" {
		text += " " + m.why
	}
	return text
}

// move is the step as a next move: what it does as a lead and, where it has any, its
// commands joined into the one line a user pastes.
func (m manualStep) move() presenter.NextMove {
	lead := strings.ToUpper(m.does[:1]) + m.does[1:]
	if m.why != "" {
		lead += " " + m.why
	}
	if len(m.commands) == 0 {
		return presenter.NextMove{Lead: lead}
	}
	return presenter.NextMove{Lead: lead + ":", Command: strings.Join(m.commands, " && ")}
}

// warnUnwindIncomplete emits the manual-cleanup notice when a recovery step failed
// mid-unwind: the repo is NOT clean, so the user must finish the recovery by hand. It
// is a Failed warning on the unwind's node, the failing step's own output riding its
// Output verbatim. Plain's line names steps, the manual recovery in order; the rail's
// row says only that recovery failed, and steps reach it as Quiet next moves.
func warnUnwindIncomplete(p presenter.Presenter, output string, steps ...manualStep) {
	texts := make([]string, len(steps))
	moves := make([]presenter.NextMove, len(steps))
	for i, step := range steps {
		texts[i], moves[i] = step.text(), step.move()
	}
	const failed = "automatic recovery failed"
	p.Warn(presenter.Warning{
		Key:     stageUnwind,
		Failed:  true,
		Label:   "unwind incomplete",
		Message: failed + "; manually " + strings.Join(texts, ", then "),
		Row:     failed,
		Output:  output,
	})
	p.NextMoves(presenter.NextMoves{Stage: stageUnwind, Quiet: true, Moves: moves})
}
