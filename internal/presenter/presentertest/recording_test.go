package presentertest_test

import (
	"testing"
	"time"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// Compile-time proof that the recording presenter satisfies the contract it is
// meant to record. If the interface or the recorder drifts, the build breaks.
var _ presenter.Presenter = (*presentertest.RecordingPresenter)(nil)

func TestRecordingPresenter_RecordsSingleStageSucceededPayload(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	success := presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Elapsed:  1100 * time.Millisecond,
		Blocking: true,
	}
	rec.StageSucceeded(success)

	if got := len(rec.Events); got != 1 {
		t.Fatalf("len(Events) = %d, want 1", got)
	}

	ev := rec.Events[0]
	if ev.Kind != presentertest.KindStageSucceeded {
		t.Errorf("Kind = %v, want %v", ev.Kind, presentertest.KindStageSucceeded)
	}
	if ev.StageSucceeded != success {
		t.Errorf("StageSucceeded = %+v, want %+v", ev.StageSucceeded, success)
	}
	// Every field of the payload must be retrievable for assertions.
	if ev.StageSucceeded.Name != "notes" {
		t.Errorf("Name = %q, want %q", ev.StageSucceeded.Name, "notes")
	}
	if ev.StageSucceeded.Detail != "generated" {
		t.Errorf("Detail = %q, want %q", ev.StageSucceeded.Detail, "generated")
	}
	if ev.StageSucceeded.Elapsed != 1100*time.Millisecond {
		t.Errorf("Elapsed = %v, want %v", ev.StageSucceeded.Elapsed, 1100*time.Millisecond)
	}
	if !ev.StageSucceeded.Blocking {
		t.Error("Blocking = false, want true")
	}
}

func TestRecordingPresenter_RecordsEventsInCallOrderAcrossKinds(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.RunStarted(presenter.RunInfo{Project: "acme", Version: "1.4.0", Action: "releasing"})
	rec.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	rec.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated"})
	rec.RunFinished(presenter.RunResult{Project: "acme", Version: "1.4.0"})

	want := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindStageStarted,
		presentertest.KindStageSucceeded,
		presentertest.KindRunFinished,
	}

	got := rec.Kinds()
	if len(got) != len(want) {
		t.Fatalf("Kinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Kinds()[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// Spot-check that a payload deep in the sequence is fully retrievable.
	if rec.Events[0].RunStarted.Action != "releasing" {
		t.Errorf("RunStarted.Action = %q, want %q", rec.Events[0].RunStarted.Action, "releasing")
	}
}

func TestRecordingPresenter_RecordsMultipleStagesInIssueOrder(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	// Repeated stage cycles interleaved with run-level events must be recorded
	// in issue order with no collapsing or de-duplication.
	rec.RunStarted(presenter.RunInfo{Project: "acme"})
	rec.StageStarted(presenter.StageStart{Name: "build"})
	rec.StageSucceeded(presenter.StageSuccess{Name: "build"})
	rec.StageStarted(presenter.StageStart{Name: "notes"})
	rec.StageSucceeded(presenter.StageSuccess{Name: "notes"})
	rec.RunFinished(presenter.RunResult{Project: "acme"})

	want := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindStageStarted,
		presentertest.KindStageSucceeded,
		presentertest.KindStageStarted,
		presentertest.KindStageSucceeded,
		presentertest.KindRunFinished,
	}

	got := rec.Kinds()
	if len(got) != len(want) {
		t.Fatalf("Kinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Kinds()[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// The two StageStarted entries must keep their distinct payloads — no
	// de-duplication or collapsing of repeated stage cycles.
	if rec.Events[1].StageStarted.Name != "build" {
		t.Errorf("Events[1].StageStarted.Name = %q, want %q", rec.Events[1].StageStarted.Name, "build")
	}
	if rec.Events[3].StageStarted.Name != "notes" {
		t.Errorf("Events[3].StageStarted.Name = %q, want %q", rec.Events[3].StageStarted.Name, "notes")
	}
}

func TestRecordingPresenter_ReportsZeroEventsBeforeAnyCall(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	if got := len(rec.Events); got != 0 {
		t.Errorf("len(Events) = %d, want 0", got)
	}
	if got := rec.Kinds(); len(got) != 0 {
		t.Errorf("Kinds() = %v, want empty", got)
	}

	// Accessors must not panic on an empty recorder.
	if ev, ok := rec.At(0); ok {
		t.Errorf("At(0) = %+v, ok=true, want ok=false", ev)
	}
}

func TestRecordingPresenter_AtFetchesNthEvent(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.StageStarted(presenter.StageStart{Name: "build"})
	rec.StageFailed(presenter.StageFailure{Name: "build", Message: "boom"})

	ev, ok := rec.At(1)
	if !ok {
		t.Fatal("At(1) ok = false, want true")
	}
	if ev.Kind != presentertest.KindStageFailed {
		t.Errorf("At(1).Kind = %v, want %v", ev.Kind, presentertest.KindStageFailed)
	}
	if ev.StageFailed.Message != "boom" {
		t.Errorf("At(1).StageFailed.Message = %q, want %q", ev.StageFailed.Message, "boom")
	}

	// Out-of-range index must report not-found, never panic.
	if _, ok := rec.At(2); ok {
		t.Error("At(2) ok = true, want false")
	}
	if _, ok := rec.At(-1); ok {
		t.Error("At(-1) ok = true, want false")
	}
}
