package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestNotesReviewGateDeclaresFourChoices proves the notes-review gate declares
// exactly y/n/e/r in that order with default y and the spec's action labels.
func TestNotesReviewGateDeclaresFourChoices(t *testing.T) {
	gate := presenter.NotesReviewGate()

	wantKeys := []presenter.Choice{
		presenter.ChoiceYes,
		presenter.ChoiceNo,
		presenter.ChoiceEdit,
		presenter.ChoiceRegen,
	}
	if got := gate.Keys(); !equalChoices(got, wantKeys) {
		t.Errorf("Keys() = %v, want %v", got, wantKeys)
	}
	if gate.Default != presenter.ChoiceYes {
		t.Errorf("Default = %q, want %q", gate.Default, presenter.ChoiceYes)
	}

	wantActions := map[presenter.Choice]string{
		presenter.ChoiceYes:   "accept & proceed",
		presenter.ChoiceNo:    "abort",
		presenter.ChoiceEdit:  "edit in $EDITOR",
		presenter.ChoiceRegen: "regenerate",
	}
	for _, choice := range gate.Choices {
		if want := wantActions[choice.Key]; choice.Action != want {
			t.Errorf("action for %q = %q, want %q", choice.Key, choice.Action, want)
		}
	}
}

// TestReuseConfirmGateDeclaresTwoChoices proves the reuse confirm declares only
// y/n with default y and no e/r.
func TestReuseConfirmGateDeclaresTwoChoices(t *testing.T) {
	gate := presenter.ReuseConfirmGate()

	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}
	if got := gate.Keys(); !equalChoices(got, wantKeys) {
		t.Errorf("Keys() = %v, want %v", got, wantKeys)
	}
	if gate.Default != presenter.ChoiceYes {
		t.Errorf("Default = %q, want %q", gate.Default, presenter.ChoiceYes)
	}
	if gate.Has(presenter.ChoiceEdit) {
		t.Error("Has(ChoiceEdit) = true, want false (reuse confirm has no e)")
	}
	if gate.Has(presenter.ChoiceRegen) {
		t.Error("Has(ChoiceRegen) = true, want false (reuse confirm has no r)")
	}
}

// TestGateCanDeclareNonYesDefault proves the model does not assume a yes-default:
// a gate may declare any member of its choice set as the default.
func TestGateCanDeclareNonYesDefault(t *testing.T) {
	gate := presenter.Gate{
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept & proceed"},
			{Key: presenter.ChoiceNo, Action: "abort"},
		},
		Default: presenter.ChoiceNo,
	}

	if gate.Default != presenter.ChoiceNo {
		t.Errorf("Default = %q, want %q", gate.Default, presenter.ChoiceNo)
	}
	if !gate.Has(presenter.ChoiceNo) {
		t.Error("Has(ChoiceNo) = false, want true")
	}
}

// TestHasRejectsChoiceOutsideDeclaredSet proves Has operates over the declared
// set, not a hardcoded list: a choice the gate does not declare is rejected.
func TestHasRejectsChoiceOutsideDeclaredSet(t *testing.T) {
	if presenter.ReuseConfirmGate().Has(presenter.ChoiceEdit) {
		t.Error("ReuseConfirmGate().Has(ChoiceEdit) = true, want false")
	}
	if presenter.NotesReviewGate().Has(presenter.Choice("x")) {
		t.Error(`NotesReviewGate().Has("x") = true, want false`)
	}
}

// TestPromptIsOnInterfaceAndRecorderCapturesGate drives Prompt through the
// RecordingPresenter and asserts the gate is captured AND the canned choice is
// returned — proving Prompt is on the interface and recorded.
func TestPromptIsOnInterfaceAndRecorderCapturesGate(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	gate := presenter.NotesReviewGate()

	var p presenter.Presenter = rec
	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}

	// Default canned behaviour returns the gate's Default.
	if choice != gate.Default {
		t.Errorf("Prompt returned %q, want gate default %q", choice, gate.Default)
	}

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.Kind != presentertest.KindPrompt {
		t.Fatalf("Kind = %v, want %v", ev.Kind, presentertest.KindPrompt)
	}
	if got := ev.Prompt.Keys(); !equalChoices(got, gate.Keys()) {
		t.Errorf("recorded gate Keys() = %v, want %v", got, gate.Keys())
	}
	if ev.Prompt.Default != gate.Default {
		t.Errorf("recorded gate Default = %q, want %q", ev.Prompt.Default, gate.Default)
	}
}

// TestRecorderReturnsScriptedChoice proves the recorder can be scripted with a
// canned answer so engine-driven tests control the gate response.
func TestRecorderReturnsScriptedChoice(t *testing.T) {
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}

	choice, err := rec.Prompt(presenter.NotesReviewGate())
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if choice != presenter.ChoiceNo {
		t.Errorf("Prompt returned %q, want scripted %q", choice, presenter.ChoiceNo)
	}
}

// TestPlainPromptReadsDefaultOnEmptyEnter proves the plain Prompt now drives the
// real line-read loop (the stub that returned gate.Default with no input is
// replaced in task 3-3): an injected empty-Enter line selects the gate's Default.
// The full input matrix lives in prompt_test.go; this guards the constructor wiring
// from gate_test's vantage.
func TestPlainPromptReadsDefaultOnEmptyEnter(t *testing.T) {
	gate := presenter.ReuseConfirmGate()
	p := presenter.NewPlainPresenterWithInput(&bytes.Buffer{}, &bytes.Buffer{}, strings.NewReader("\n"))

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if choice != gate.Default {
		t.Errorf("Prompt returned %q, want gate default %q", choice, gate.Default)
	}
}

// TestPrettyPromptReadsDefaultOnEmptyEnter proves the same for the pretty Prompt:
// the stub is replaced by the shared line-read loop, and an injected empty-Enter
// line selects the gate's Default (the full vertical menu is task 3-4).
func TestPrettyPromptReadsDefaultOnEmptyEnter(t *testing.T) {
	gate := presenter.NotesReviewGate()
	p := presenter.NewPrettyPresenterWithInput(&bytes.Buffer{}, termenv.Ascii, strings.NewReader("\n"))

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if choice != gate.Default {
		t.Errorf("Prompt returned %q, want gate default %q", choice, gate.Default)
	}
}

// equalChoices compares two ordered choice slices element-by-element.
func equalChoices(a, b []presenter.Choice) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
