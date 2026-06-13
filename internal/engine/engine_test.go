package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// staticPresenter satisfies presenter.Presenter via the embedded
// RecordingPresenter; it is the in-test proof the engine reports through the
// AS-BUILT presenter seam — engine functions accept presenter.Presenter, so this
// recorder (a presenter.Presenter) is a legal argument with no engine-defined
// interface or fake in sight.
var _ presenter.Presenter = (*presentertest.RecordingPresenter)(nil)

// TestFirstReleaseReviewGate_IsYNEOnly pins the hand-built Phase 1 first-release
// gate: it offers y/n/e ONLY (no r — no AI to regenerate in the no-AI path),
// carries Subject/AcceptEcho for the -y echo, defaults to yes, and pairs each
// choice with the spec's action label.
func TestFirstReleaseReviewGate_IsYNEOnly(t *testing.T) {
	t.Parallel()

	gate := engine.FirstReleaseReviewGate()

	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}
	gotKeys := gate.Keys()
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("gate keys = %v, want %v", gotKeys, wantKeys)
	}
	for i, want := range wantKeys {
		if gotKeys[i] != want {
			t.Errorf("gate key[%d] = %q, want %q", i, gotKeys[i], want)
		}
	}

	if gate.Has(presenter.ChoiceRegen) {
		t.Errorf("gate offers ChoiceRegen (r); first-release no-AI gate must omit it")
	}
	if gate.Default != presenter.ChoiceYes {
		t.Errorf("gate Default = %q, want %q", gate.Default, presenter.ChoiceYes)
	}
	if gate.Subject != "notes" {
		t.Errorf("gate Subject = %q, want %q", gate.Subject, "notes")
	}
	if gate.AcceptEcho != "accepted" {
		t.Errorf("gate AcceptEcho = %q, want %q", gate.AcceptEcho, "accepted")
	}

	wantActions := map[presenter.Choice]string{
		presenter.ChoiceYes:  "release",
		presenter.ChoiceNo:   "abort",
		presenter.ChoiceEdit: "edit in $EDITOR",
	}
	for _, gc := range gate.Choices {
		if want := wantActions[gc.Key]; gc.Action != want {
			t.Errorf("action for %q = %q, want %q", gc.Key, gc.Action, want)
		}
	}
}

// TestReviewDecision_ReturnsScriptedChoice drives the decision seam against the
// RecordingPresenter: the scripted NextChoices answer is returned, and the gate
// is recorded as a single KindPrompt event carrying the declared gate.
func TestReviewDecision_ReturnsScriptedChoice(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}
	gate := engine.FirstReleaseReviewGate()

	choice, err := engine.ReviewDecision(rec, gate)
	if err != nil {
		t.Fatalf("ReviewDecision returned unexpected error: %v", err)
	}
	if choice != presenter.ChoiceNo {
		t.Errorf("choice = %q, want %q", choice, presenter.ChoiceNo)
	}

	if got := rec.Kinds(); len(got) != 1 || got[0] != presentertest.KindPrompt {
		t.Fatalf("kinds = %v, want [Prompt]", got)
	}
	ev, ok := rec.At(0)
	if !ok {
		t.Fatalf("no event recorded at 0")
	}
	if ev.Prompt.Subject != "notes" || ev.Prompt.Default != presenter.ChoiceYes {
		t.Errorf("recorded gate = %+v, want first-release review gate", ev.Prompt)
	}
}

// TestReviewDecision_PromptError_AbortsNonZero proves both EOF/non-TTY input
// errors map to an engine abort carrying a non-zero exit code, and that the
// engine's abort preserves the underlying sentinel for errors.Is branching.
func TestReviewDecision_PromptError_AbortsNonZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		injected error
	}{
		{name: "not a TTY without -y", injected: presenter.ErrNotInteractive},
		{name: "input closed mid-gate", injected: presenter.ErrInputClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{
				PromptResult: func(presenter.Gate) (presenter.Choice, error) {
					return "", tt.injected
				},
			}

			_, err := engine.ReviewDecision(rec, engine.FirstReleaseReviewGate())
			if err == nil {
				t.Fatalf("ReviewDecision returned nil error, want an abort")
			}
			if !errors.Is(err, tt.injected) {
				t.Errorf("err does not wrap injected sentinel %v: %v", tt.injected, err)
			}

			var abort *engine.AbortError
			if !errors.As(err, &abort) {
				t.Fatalf("err is not an *engine.AbortError: %v", err)
			}
			if abort.ExitCode == 0 {
				t.Errorf("abort ExitCode = 0, want non-zero")
			}
		})
	}
}
