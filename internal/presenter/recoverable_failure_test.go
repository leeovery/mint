package presenter_test

import (
	"io"
	"strings"
	"testing"

	"mint/internal/presenter"
)

func floorLikeGate() presenter.Gate {
	return presenter.Gate{
		Question: "Release without AI notes?",
		Subject:  "notes",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceCommitSubjects, Action: "use the commit subjects"},
			{Key: presenter.ChoiceAbort, Action: "abort"},
		},
		Default: presenter.ChoiceAbort,
	}
}

func recoverableNotesFailure() presenter.StageFailure {
	return presenter.StageFailure{Name: "notes", Message: "AI call failed", Output: "Not logged in", Recoverable: true}
}

func TestRecoverableFailure_LeavesTheSuccessLineStanding(t *testing.T) {
	t.Parallel()

	finished := presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0"}

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		out, _ := drive(func(p *presenter.PlainPresenter) {
			p.StageFailed(recoverableNotesFailure())
			p.RunFinished(finished)
		})

		want := "notes: FAILED - AI call failed\n--- output ---\nNot logged in\n--- end output ---\ndone: acme v1.4.0\n"
		if got := out.String(); got != want {
			t.Errorf("plain output = %q, want %q", got, want)
		}
	})
}

func TestRecoverableFailure_StillEndsTheRunWhenItUnwinds(t *testing.T) {
	t.Parallel()

	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.StageFailed(recoverableNotesFailure())
		p.Unwound(presenter.Unwind{Undone: "reset 1 commit; repo clean"})
		p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0"})
	})

	if strings.Contains(out.String(), "done:") {
		t.Errorf("plain output = %q, want no success line after the unwind", out.String())
	}
}

// readSnapshot calls at before each read of r, so a test sees the presenter's state
// while a gate waits for input.
type readSnapshot struct {
	r  io.Reader
	at func()
}

func (s *readSnapshot) Read(p []byte) (int, error) {
	s.at()
	return s.r.Read(p)
}

func TestRailRecoverableFailure_NoSpinnerAnimatesWhileTheNextGateWaits(t *testing.T) {
	t.Parallel()

	var r *railTerm
	tickingAtRead := true
	in := &readSnapshot{r: strings.NewReader("a\n"), at: func() { tickingAtRead = r.ticks.running() }}
	r = newRailTerm(t, 80, 24, in)

	r.StageStarted(presenter.StageStart{Name: "notes", Key: "notes", Blocking: true})
	r.StageFailed(recoverableNotesFailure())
	choice, err := r.Prompt(floorLikeGate())

	if err != nil || choice != presenter.ChoiceAbort {
		t.Fatalf("Prompt = %q, %v, want %q", choice, err, presenter.ChoiceAbort)
	}
	if tickingAtRead {
		t.Error("the spinner's ticker ran while the gate waited")
	}
}
