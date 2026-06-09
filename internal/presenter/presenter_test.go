package presenter_test

import (
	"testing"
	"time"

	"mint/internal/presenter"
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
