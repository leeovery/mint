// Package engine is mint's release-side orchestration core. This file establishes
// and VERIFIES the adoption of the presenter seam — it is the engine's single point
// of contact with mint's output and interactive-gate machinery.
//
// CROSS-SPEC OWNERSHIP BOUNDARY (load-bearing):
//
// The presenter seam — the Presenter interface, its event payload structs, the
// Gate data model, the four-choice gate constructors, the ErrNotInteractive /
// ErrInputClosed sentinels, and the RecordingPresenter test double — is OWNED by
// the CLI Presentation specification and has already SHIPPED in full under
// mint/internal/presenter (+ presentertest). The engine is the CONSUMER. It
// depends ONLY on the as-built presenter.Presenter interface and defines NO
// parallel presenter interface and NO parallel fake: production engine code calls
// the shipped methods, and engine tests drive the shipped presentertest recorder.
//
// What this file owns (the engine's half of the seam):
//   - FirstReleaseReviewGate: the Phase 1 first-release y/n/e review gate, built as
//     a hand-built presenter.Gate literal (NOT NotesReviewGate, which is the
//     four-choice y/n/e/r variant) — there is no AI to regenerate on the no-AI
//     first-release path, so r is omitted.
//   - ReviewDecision: the decision seam — it calls Prompt(gate) and maps the
//     presenter's error contract (ErrNotInteractive / ErrInputClosed) to an engine
//     abort carrying a non-zero exit code (*AbortError).
package engine

import (
	"fmt"
	"time"

	"mint/internal/presenter"
)

// abortExitCode is the non-zero process exit code an engine abort carries. mint's
// gate/abort failures must exit non-zero so a script or CI run sees a release that
// did not complete; this is the single source of that code for the gate-decision
// abort path.
const abortExitCode = 1

// AbortError is the engine's typed abort: a failure that terminates the run and
// carries the process ExitCode the entry point should surface. It wraps the
// underlying cause so callers can still branch on the original sentinel via
// errors.Is (e.g. presenter.ErrNotInteractive / presenter.ErrInputClosed) while
// also reading the non-zero exit code via errors.As.
//
// The presenter never sets an exit code (it only renders); owning the exit code
// here keeps that policy in the engine where the run's lifecycle — and thus its
// terminal status — is decided.
type AbortError struct {
	// ExitCode is the non-zero process exit code this abort should produce.
	ExitCode int
	// cause is the underlying failure, preserved for errors.Is/errors.As inspection.
	cause error
}

// Error renders the abort for logs and test output, including the wrapped cause.
func (e *AbortError) Error() string {
	return fmt.Sprintf("aborting (exit %d): %v", e.ExitCode, e.cause)
}

// Unwrap exposes the underlying cause so errors.Is/errors.As can walk the chain to
// the original sentinel (ErrNotInteractive / ErrInputClosed).
func (e *AbortError) Unwrap() error {
	return e.cause
}

// abort builds an *AbortError carrying the standard non-zero exit code and the
// given cause.
func abort(cause error) *AbortError {
	return &AbortError{ExitCode: abortExitCode, cause: cause}
}

// FirstReleaseReviewGate is the Phase 1 first-release notes-review gate: a
// HAND-BUILT presenter.Gate offering y/n/e ONLY. It is deliberately NOT
// presenter.NotesReviewGate() — that constructor declares the four-choice
// y/n/e/r set, and the first-release path runs WITHOUT AI (fixed-body notes), so
// there is nothing for r to regenerate; offering it would be meaningless here.
//
// Subject ("notes") and AcceptEcho ("accepted") MUST be set: under -y the
// presenter renders the auto-accept echo "notes: accepted (-y)" from these two
// fields, so omitting them would render the skip echo wrong. Default is ChoiceYes
// (a bare Enter accepts — the 99% path), and each choice pairs with the spec's
// action label.
func FirstReleaseReviewGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Continue?",
		Subject:    "notes",
		AcceptEcho: "accepted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept & proceed"},
			{Key: presenter.ChoiceNo, Action: "abort"},
			{Key: presenter.ChoiceEdit, Action: "edit in $EDITOR"},
		},
		Default: presenter.ChoiceYes,
	}
}

// ReviewDecision is the decision seam between the engine and the presenter's
// interactive gate. It calls p.Prompt(gate) — the single render-and-read entry
// point — and returns the resulting presenter.Choice on success.
//
// It maps the presenter's machine-readable error contract to an engine abort: a
// Prompt error (the forbidden non-TTY-without-`-y` combination surfaced as
// ErrNotInteractive, or EOF mid-gate surfaced as ErrInputClosed) becomes an
// *AbortError carrying a non-zero exit code. The underlying sentinel is preserved
// in the abort's chain so the entry point can still branch on it via errors.Is.
//
// The presenter ALREADY renders the ErrNotInteractive failure line; ErrInputClosed
// is unrendered by contract, so surfacing it (abort + exit code, and any closing
// narration the orchestrator adds) is the engine's responsibility — owned here.
func ReviewDecision(p presenter.Presenter, gate presenter.Gate) (presenter.Choice, error) {
	choice, err := p.Prompt(gate)
	if err != nil {
		return "", abort(err)
	}
	return choice, nil
}

// emitGateSucceeded narrates a read-only gate's successful completion: a
// NON-blocking StageSucceeded with no StageStarted (the cheap gates animate no
// spinner, so they need only a completion line). Elapsed is left zero — the
// non-blocking flag tells the presenter not to render a duration regardless.
func emitGateSucceeded(p presenter.Presenter, name string) {
	p.StageSucceeded(presenter.StageSuccess{Name: name})
}

// emitBlockingStageStarted narrates the START of a BLOCKING stage — a long/slow
// step (notes generation, the pre_tag build hook, the atomic push) — by emitting a
// StageStarted with Blocking:true so the pretty spinner animates, and starts the
// engine-side elapsed timer. It returns a completion closure that, when the stage
// succeeds, emits the matching StageSucceeded carrying the engine-MEASURED Elapsed
// (the engine times the stage; the presenter never does) and Blocking:true so the
// success event mirrors the start. A FAILED stage simply does not call the closure
// — its StageFailed narrates the stage instead — so a non-success path emits no
// StageSucceeded. The wall-clock measurement uses the standard time package: this is
// production code timing the real product, not the deterministic workflow harness.
func emitBlockingStageStarted(p presenter.Presenter, name string) func() {
	p.StageStarted(presenter.StageStart{Name: name, Blocking: true})
	started := time.Now()
	return func() {
		p.StageSucceeded(presenter.StageSuccess{
			Name:     name,
			Elapsed:  time.Since(started),
			Blocking: true,
		})
	}
}
