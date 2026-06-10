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
		presenter.ChoiceYes:  "accept & proceed",
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

// TestPhase1Events_RecordedInOrder proves the Phase 1 event->method mapping is
// real: the engine's thin emit helpers push ShowPlan+RunStarted, StageFailed,
// ShowNotes and Warn through the presenter in the order called, with their full
// payloads intact.
func TestPhase1Events_RecordedInOrder(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}

	plan := presenter.Plan{Steps: []presenter.PlanStep{
		{Verb: "commit", Target: "v1.0.0"},
		{Verb: "tag", Target: "v1.0.0"},
		{Verb: "push", Target: "--atomic → origin"},
	}}
	info := presenter.RunInfo{Project: "mint", Version: "1.0.0", Action: "releasing"}
	engine.EmitPlan(rec, plan, info)

	failure := presenter.StageFailure{Name: "pre_tag", Message: "hook failed", Output: "boom\n"}
	engine.EmitStageFailed(rec, failure)

	notes := presenter.Notes{Version: "1.0.0", Body: "Initial release.\n"}
	engine.EmitNotes(rec, notes)

	warning := presenter.Warning{Label: "post_release", Message: "hook failed; tag is already published"}
	engine.EmitWarning(rec, warning)

	wantOrder := []presentertest.EventKind{
		presentertest.KindShowPlan,
		presentertest.KindRunStarted,
		presentertest.KindStageFailed,
		presentertest.KindShowNotes,
		presentertest.KindWarn,
	}
	got := rec.Kinds()
	if len(got) != len(wantOrder) {
		t.Fatalf("kinds = %v, want %v", got, wantOrder)
	}
	for i, want := range wantOrder {
		if got[i] != want {
			t.Errorf("kind[%d] = %v, want %v", i, got[i], want)
		}
	}

	planEv, _ := rec.At(0)
	if len(planEv.ShowPlan.Steps) != 3 || planEv.ShowPlan.Steps[0].Verb != "commit" {
		t.Errorf("recorded plan = %+v, want the 3-step plan", planEv.ShowPlan)
	}
	startEv, _ := rec.At(1)
	if startEv.RunStarted != info {
		t.Errorf("recorded run info = %+v, want %+v", startEv.RunStarted, info)
	}
	failEv, _ := rec.At(2)
	if failEv.StageFailed != failure {
		t.Errorf("recorded failure = %+v, want %+v", failEv.StageFailed, failure)
	}
	notesEv, _ := rec.At(3)
	if notesEv.ShowNotes != notes {
		t.Errorf("recorded notes = %+v, want %+v", notesEv.ShowNotes, notes)
	}
}

// TestEmitWarning_StructuredLabelAndMessage records a post-PONR warn-only event
// and asserts Label and Message survive as SEPARATE structured fields — never a
// single combined string.
func TestEmitWarning_StructuredLabelAndMessage(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}

	warning := presenter.Warning{
		Label:   "post_release",
		Message: "post_release hook failed; tag is already published",
		Output:  "exit status 1\n",
	}
	engine.EmitWarning(rec, warning)

	if got := rec.Kinds(); len(got) != 1 || got[0] != presentertest.KindWarn {
		t.Fatalf("kinds = %v, want [Warn]", got)
	}
	ev, _ := rec.At(0)
	if ev.Warn.Label != warning.Label {
		t.Errorf("warn Label = %q, want %q", ev.Warn.Label, warning.Label)
	}
	if ev.Warn.Message != warning.Message {
		t.Errorf("warn Message = %q, want %q", ev.Warn.Message, warning.Message)
	}
	if ev.Warn.Output != warning.Output {
		t.Errorf("warn Output = %q, want %q", ev.Warn.Output, warning.Output)
	}
	if ev.Warn.Label == ev.Warn.Message {
		t.Errorf("Label and Message are the same value; they must be distinct fields")
	}
}
