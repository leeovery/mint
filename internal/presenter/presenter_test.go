package presenter_test

import (
	"testing"
	"time"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// nopPresenter is a trivial no-op implementation used to prove that an ordinary
// value can satisfy the Presenter interface. Task 1-2 owns the full recording
// presenter; this is just enough to lock the contract here.
type nopPresenter struct {
	lastRun presenter.RunInfo
}

func (p *nopPresenter) RunStarted(info presenter.RunInfo)     { p.lastRun = info }
func (p *nopPresenter) StageStarted(presenter.StageStart)     {}
func (p *nopPresenter) StageSucceeded(presenter.StageSuccess) {}
func (p *nopPresenter) StageFailed(presenter.StageFailure)    {}
func (p *nopPresenter) RunFinished(presenter.RunResult)       {}

// Compile-time proof that the no-op value satisfies the interface.
var _ presenter.Presenter = (*nopPresenter)(nil)

func TestNopPresenterSatisfiesInterface(t *testing.T) {
	// Assigning to the interface variable exercises the compile-time contract;
	// driving every method confirms the no-op value is usable as a Presenter.
	var p presenter.Presenter = &nopPresenter{}

	p.RunStarted(presenter.RunInfo{})
	p.StageStarted(presenter.StageStart{})
	p.StageSucceeded(presenter.StageSuccess{})
	p.StageFailed(presenter.StageFailure{})
	p.RunFinished(presenter.RunResult{})
}

func TestStageStartCarriesBlockingFlag(t *testing.T) {
	s := presenter.StageStart{Name: "notes", Blocking: true}

	if s.Name != "notes" {
		t.Errorf("Name = %q, want %q", s.Name, "notes")
	}
	if !s.Blocking {
		t.Error("Blocking = false, want true")
	}
}

func TestStageSuccessCarriesEngineSuppliedElapsedAndBlocking(t *testing.T) {
	elapsed := 1100 * time.Millisecond
	s := presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Elapsed:  elapsed,
		Blocking: true,
	}

	if s.Detail != "generated" {
		t.Errorf("Detail = %q, want %q", s.Detail, "generated")
	}
	if s.Elapsed != elapsed {
		t.Errorf("Elapsed = %v, want %v", s.Elapsed, elapsed)
	}
	if !s.Blocking {
		t.Error("Blocking = false, want true")
	}
}

func TestStageFailureCarriesMessageAndCapturedOutput(t *testing.T) {
	s := presenter.StageFailure{
		Name:    "tag/push",
		Message: "push rejected: remote moved",
		Output:  "fatal: failed to push some refs",
	}

	if s.Message != "push rejected: remote moved" {
		t.Errorf("Message = %q, want %q", s.Message, "push rejected: remote moved")
	}
	if s.Output != "fatal: failed to push some refs" {
		t.Errorf("Output = %q, want %q", s.Output, "fatal: failed to push some refs")
	}
}

func TestRunInfoActionRoundTripsThroughPresenter(t *testing.T) {
	p := &nopPresenter{}
	info := presenter.RunInfo{
		Project: "acme",
		Version: "1.4.0",
		Action:  "regenerating",
	}

	p.RunStarted(info)

	if p.lastRun.Action != "regenerating" {
		t.Errorf("Action = %q, want %q", p.lastRun.Action, "regenerating")
	}
	if p.lastRun.Project != "acme" {
		t.Errorf("Project = %q, want %q", p.lastRun.Project, "acme")
	}
	if p.lastRun.Version != "1.4.0" {
		t.Errorf("Version = %q, want %q", p.lastRun.Version, "1.4.0")
	}
}

func TestRunResultCarriesProjectVersionAndOptionalURL(t *testing.T) {
	r := presenter.RunResult{
		Project: "acme",
		Version: "1.4.0",
		URL:     "https://github.com/acme/acme/releases/tag/v1.4.0",
	}

	if r.Project != "acme" {
		t.Errorf("Project = %q, want %q", r.Project, "acme")
	}
	if r.Version != "1.4.0" {
		t.Errorf("Version = %q, want %q", r.Version, "1.4.0")
	}
	if r.URL != "https://github.com/acme/acme/releases/tag/v1.4.0" {
		t.Errorf("URL = %q, want %q", r.URL, "https://github.com/acme/acme/releases/tag/v1.4.0")
	}
}

// TestRunInfoCarriesBrandLeaf proves the start-of-run payload carries the
// engine-supplied brand leaf so the presenter renders it rather than hardcoding
// a glyph. The leaf ties to the engine's commit_prefix brand.
func TestRunInfoCarriesBrandLeaf(t *testing.T) {
	info := presenter.RunInfo{Leaf: "🌱"}

	if info.Leaf != "🌱" {
		t.Errorf("Leaf = %q, want %q", info.Leaf, "🌱")
	}
}

// TestRunResultCarriesBrandLeaf proves the end-of-run payload carries the same
// engine-supplied brand leaf so the closing brand line is rendered, not hardcoded.
func TestRunResultCarriesBrandLeaf(t *testing.T) {
	r := presenter.RunResult{Leaf: "🌱"}

	if r.Leaf != "🌱" {
		t.Errorf("Leaf = %q, want %q", r.Leaf, "🌱")
	}
}

// TestStageStartPayloadRoundTripsThroughRecorder proves the recorder captures the
// full StageStart payload — name and the engine-supplied blocking flag — so an
// engine-driven test can assert the flag the renderers depend on.
func TestStageStartPayloadRoundTripsThroughRecorder(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.Kind != presentertest.KindStageStarted {
		t.Fatalf("Kind = %v, want %v", ev.Kind, presentertest.KindStageStarted)
	}
	if ev.StageStarted.Name != "notes" {
		t.Errorf("Name = %q, want %q", ev.StageStarted.Name, "notes")
	}
	if !ev.StageStarted.Blocking {
		t.Error("Blocking = false, want true")
	}
}

// TestStageSuccessPayloadRoundTripsThroughRecorder proves the recorder captures the
// full extended StageSuccess payload — name, detail, engine-supplied elapsed, and
// the blocking flag — with no field dropped.
func TestStageSuccessPayloadRoundTripsThroughRecorder(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	elapsed := 1100 * time.Millisecond

	rec.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Elapsed:  elapsed,
		Blocking: true,
	})

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.Kind != presentertest.KindStageSucceeded {
		t.Fatalf("Kind = %v, want %v", ev.Kind, presentertest.KindStageSucceeded)
	}
	if ev.StageSucceeded.Name != "notes" {
		t.Errorf("Name = %q, want %q", ev.StageSucceeded.Name, "notes")
	}
	if ev.StageSucceeded.Detail != "generated" {
		t.Errorf("Detail = %q, want %q", ev.StageSucceeded.Detail, "generated")
	}
	if ev.StageSucceeded.Elapsed != elapsed {
		t.Errorf("Elapsed = %v, want %v", ev.StageSucceeded.Elapsed, elapsed)
	}
	if !ev.StageSucceeded.Blocking {
		t.Error("Blocking = false, want true")
	}
}

// TestShortStageSuccessCarriesNoMeaningfulElapsed locks the first zero-value
// semantic: a short stage (Blocking==false) carries no meaningful elapsed. The
// payload does not enforce a zero — renderers honour the flag and must not print
// elapsed regardless of the Elapsed value. This contract test asserts the field
// shape only; rendering lives in the pretty/plain tasks.
func TestShortStageSuccessCarriesNoMeaningfulElapsed(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "ok", Blocking: false})

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.StageSucceeded.Blocking {
		t.Error("Blocking = true, want false for a short stage")
	}
	// The blocking flag — not the Elapsed value — is what gates elapsed rendering.
	if ev.StageSucceeded.Detail != "ok" {
		t.Errorf("Detail = %q, want %q", ev.StageSucceeded.Detail, "ok")
	}
}

// TestZeroElapsedIsLegalOnBlockingStage locks the second zero-value semantic:
// Elapsed==0 is legal even when Blocking==true and must NOT be treated as "no
// elapsed". Constructing and recording such a payload is valid — no panic, no
// special-casing.
func TestZeroElapsedIsLegalOnBlockingStage(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.StageSucceeded(presenter.StageSuccess{Name: "notes", Blocking: true, Elapsed: 0})

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if !ev.StageSucceeded.Blocking {
		t.Error("Blocking = false, want true")
	}
	if ev.StageSucceeded.Elapsed != 0 {
		t.Errorf("Elapsed = %v, want 0", ev.StageSucceeded.Elapsed)
	}
}

// TestEmptyDetailIsLegal locks the third zero-value semantic: Detail=="" is legal;
// the payload supplies no default. Renderers fall back to the ok/detail-less form.
func TestEmptyDetailIsLegal(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	rec.StageSucceeded(presenter.StageSuccess{Name: "record", Detail: ""})

	ev, ok := rec.At(0)
	if !ok {
		t.Fatal("expected one recorded event, got none")
	}
	if ev.StageSucceeded.Detail != "" {
		t.Errorf("Detail = %q, want empty", ev.StageSucceeded.Detail)
	}
}
