package presenter_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/muesli/termenv"

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

	t.Run("pretty", func(t *testing.T) {
		t.Parallel()

		out := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) {
			p.StageFailed(recoverableNotesFailure())
			p.RunFinished(finished)
		})

		if !strings.HasSuffix(out.String(), "\n🌿 released acme v1.4.0\n") {
			t.Errorf("pretty output = %q, want it to end with the release's success line", out.String())
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

func TestPrettyRecoverableFailure_NoSpinnerAnimatesWhileTheNextGateWaits(t *testing.T) {
	t.Parallel()

	tr := &spyTracker{}
	activeAtRead := -1
	in := &readSnapshot{r: strings.NewReader("a\n"), at: func() { activeAtRead = tr.active }}
	p := presenter.NewPrettyPresenter(&bytes.Buffer{}, presenter.WithProfile(termenv.Ascii), presenter.WithErr(&bytes.Buffer{}), presenter.WithInput(in)).
		WithSpinnerFactory(tr.factory()).
		WithInteractiveStdin(true)

	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	p.StageFailed(recoverableNotesFailure())
	choice, err := p.Prompt(floorLikeGate())

	if err != nil || choice != presenter.ChoiceAbort {
		t.Fatalf("Prompt = %q, %v, want %q", choice, err, presenter.ChoiceAbort)
	}
	if activeAtRead != 0 {
		t.Errorf("spinners animating while the gate waited = %d, want 0", activeAtRead)
	}
}
