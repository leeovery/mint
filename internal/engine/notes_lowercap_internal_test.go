package engine

import (
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

func TestNotesStage_HalvedRun_HintsTheLargestCallsExactByteCount(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	stage := &notesStage{p: rec}
	stage.CallSucceeded(1_259_999)
	stage.Halved(3, 2)
	stage.CallSucceeded(1_260_001)
	stage.CallSucceeded(630_000)
	stage.Merged(5)
	rec.Events = nil

	hintLowerCap(rec, stage.lowerCap())

	want := presenter.Warning{
		Label:   "max_prompt_bytes",
		Message: "set higher than this model accepts; set max_prompt_bytes = 1260001, the largest call that succeeded",
	}
	if len(rec.Events) != 1 || rec.Events[0].Kind != presentertest.KindWarn || rec.Events[0].Warn != want {
		t.Fatalf("events = %+v, want only the hint %+v", rec.Events, want)
	}
}

func TestNotesStage_RunWithNoHalving_NoHint(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	stage := &notesStage{p: rec}
	stage.CallSucceeded(1_260_001)

	hintLowerCap(rec, stage.lowerCap())

	if len(rec.Events) != 0 {
		t.Fatalf("events = %+v, want none", rec.Events)
	}
}

func TestNotesStage_HalvedRunWithNoMerge_NoCap(t *testing.T) {
	t.Parallel()

	stage := &notesStage{p: &presentertest.RecordingPresenter{}}
	stage.Halved(0, 2)
	stage.CallSucceeded(630_000)

	if got := stage.lowerCap(); got != 0 {
		t.Errorf("lowerCap() = %d, want 0 for a halved run whose notes were not written", got)
	}
}
