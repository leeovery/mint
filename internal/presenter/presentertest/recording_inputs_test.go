package presentertest_test

import (
	"errors"
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestRecorderAskLineRecordsPromptAndScriptsNextLines proves AskLine records the
// engine-supplied prompt label and pops scripted answers from the NextLines FIFO
// in order, falling back to the empty "no input" answer once the queue drains.
func TestRecorderAskLineRecordsPromptAndScriptsNextLines(t *testing.T) {
	rec := &presentertest.RecordingPresenter{NextLines: []string{"first context", "second context"}}

	for i, want := range []string{"first context", "second context", ""} {
		got, err := rec.AskLine("context")
		if err != nil {
			t.Fatalf("AskLine call %d returned error: %v", i, err)
		}
		if got != want {
			t.Errorf("AskLine call %d = %q, want %q", i, got, want)
		}
	}
	ev, ok := rec.At(0)
	if !ok || ev.Kind != presentertest.KindAskLine {
		t.Fatalf("event 0 = %v, want a recorded KindAskLine", ev.Kind)
	}
	if ev.AskLine != "context" {
		t.Errorf("recorded prompt = %q, want %q", ev.AskLine, "context")
	}
}

// TestRecorderAskLineResultTakesPrecedenceAndScriptsErrors proves the
// AskLineResult hook overrides the NextLines queue entirely — the error-injection
// path for ErrInputClosed/ErrNotInteractive engine branches.
func TestRecorderAskLineResultTakesPrecedenceAndScriptsErrors(t *testing.T) {
	rec := &presentertest.RecordingPresenter{
		NextLines:     []string{"queued answer that must not be used"},
		AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
	}

	_, err := rec.AskLine("context")
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("AskLine with AskLineResult = %v, want the scripted ErrInputClosed", err)
	}
	if len(rec.NextLines) != 1 {
		t.Errorf("NextLines was consumed despite AskLineResult precedence")
	}
}

// TestRecorderShowMessageRecordsFullPayload proves the titled-message event
// round-trips through the recorder — title and verbatim body — independent of any
// rendering.
func TestRecorderShowMessageRecordsFullPayload(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	m := presenter.Message{Title: "commit message", Body: "feat: x\n\ndetail"}

	rec.ShowMessage(m)

	ev, ok := rec.At(0)
	if !ok || ev.Kind != presentertest.KindShowMessage {
		t.Fatalf("event 0 = %v, want a recorded KindShowMessage", ev.Kind)
	}
	if ev.ShowMessage != m {
		t.Errorf("recorded message = %+v, want %+v", ev.ShowMessage, m)
	}
}

// TestRecorderPromptAnswerPrecedence proves Prompt's documented answer
// resolution order directly: a non-nil PromptResult (choice AND error) overrides
// the NextChoices queue, the queue pops FIFO with nil errors, and the gate's own
// Default is the unscripted fallback.
func TestRecorderPromptAnswerPrecedence(t *testing.T) {
	gate := presenter.NotesReviewGate()

	t.Run("prompt result overrides queue and scripts an error", func(t *testing.T) {
		rec := &presentertest.RecordingPresenter{
			NextChoices:  []presenter.Choice{presenter.ChoiceNo},
			PromptResult: func(presenter.Gate) (presenter.Choice, error) { return "", presenter.ErrNotInteractive },
		}
		_, err := rec.Prompt(gate)
		if !errors.Is(err, presenter.ErrNotInteractive) {
			t.Fatalf("Prompt with PromptResult = %v, want the scripted ErrNotInteractive", err)
		}
		if len(rec.NextChoices) != 1 {
			t.Errorf("NextChoices was consumed despite PromptResult precedence")
		}
	})

	t.Run("next choices pop in order then fall back to the gate default", func(t *testing.T) {
		rec := &presentertest.RecordingPresenter{
			NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceEdit},
		}
		for i, want := range []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceEdit, gate.Default} {
			got, err := rec.Prompt(gate)
			if err != nil {
				t.Fatalf("Prompt call %d returned error: %v", i, err)
			}
			if got != want {
				t.Errorf("Prompt call %d = %q, want %q", i, got, want)
			}
		}
	})
}

// TestRecorderSuspendResumeRecordOrderedKinds proves the payload-less spinner
// control hooks are recorded in call order so an engine-driven test can assert
// the $EDITOR hand-off wrapping (suspend before, resume after).
func TestRecorderSuspendResumeRecordOrderedKinds(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.SuspendSpinner()
	rec.ResumeSpinner()

	want := []presentertest.EventKind{presentertest.KindSuspendSpinner, presentertest.KindResumeSpinner}
	got := rec.Kinds()
	if len(got) != len(want) {
		t.Fatalf("Kinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Kinds()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
