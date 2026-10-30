package engine

import (
	"context"
	"time"

	"mint/internal/presenter"
)

// answerWaits sums the time a stage spends waiting on the person at the terminal: a
// question's answer, a typed line, the editor. A stage's time leaves it out, so it
// counts only mint's work, as a release's notes node does, its review asked after it.
type answerWaits struct {
	now   func() time.Time
	total time.Duration
}

// timed runs wait, adding how long it took to the waits.
func (w *answerWaits) timed(wait func()) {
	start := w.now()
	wait()
	w.total += w.now().Sub(start)
}

// waitingPresenter is a presenter whose questions are timed as waits.
type waitingPresenter struct {
	presenter.Presenter
	waits *answerWaits
}

func (p waitingPresenter) Prompt(g presenter.Gate) (c presenter.Choice, err error) {
	p.waits.timed(func() { c, err = p.Presenter.Prompt(g) })
	return c, err
}

func (p waitingPresenter) AskLine(prompt string) (line string, err error) {
	p.waits.timed(func() { line, err = p.Presenter.AskLine(prompt) })
	return line, err
}

// waitingEditor is an editor whose edits are timed as waits.
type waitingEditor struct {
	Editor
	waits *answerWaits
}

func (e waitingEditor) Edit(ctx context.Context, current string) (body string, err error) {
	e.waits.timed(func() { body, err = e.Editor.Edit(ctx, current) })
	return body, err
}

// awaiting is deps whose questions and editor are timed as waits the stage s leaves
// out of its time, for the questions asked while it goes on: a --all version's
// review, which its node goes on past to write the release.
func (s *blockingStage) awaiting(deps ReleaseDeps) ReleaseDeps {
	if s.waits == nil {
		s.waits = &answerWaits{now: s.now}
	}
	deps.Presenter = waitingPresenter{Presenter: deps.Presenter, waits: s.waits}
	if deps.Editor != nil {
		deps.Editor = waitingEditor{Editor: deps.Editor, waits: s.waits}
	}
	return deps
}
