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
//   - EmitPlan / EmitStageFailed / EmitNotes / EmitWarning: thin Phase 1
//     event->method mappers proving the adoption is real. They each forward an
//     engine-supplied payload to the matching presenter method and nothing more;
//     the actual orchestration ORDERING (when each fires, in what release sequence)
//     belongs to a later orchestrator task.
package engine

import (
	"fmt"

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

// EmitPlan forwards the Phase 1 plan/version summary to the presenter: the upcoming
// plan via ShowPlan and the start-of-run header via RunStarted. It is a thin
// mapper — it supplies no ordering policy of its own beyond emitting the plan
// before the header — proving the plan/version event maps onto the shipped
// methods. The release-wide ordering of this relative to other events is the
// orchestrator's concern.
func EmitPlan(p presenter.Presenter, plan presenter.Plan, info presenter.RunInfo) {
	p.ShowPlan(plan)
	p.RunStarted(info)
}

// EmitStageFailed forwards a stage/gate failure to the presenter's StageFailed,
// passing the engine-captured command Output through unchanged. Thin mapper: the
// engine builds the StageFailure payload; the presenter renders it.
func EmitStageFailed(p presenter.Presenter, failure presenter.StageFailure) {
	p.StageFailed(failure)
}

// EmitNotes forwards the generated release notes to the presenter's ShowNotes for
// the zero-risk review window. Thin mapper: the body is the engine's, written
// verbatim by the presenter.
func EmitNotes(p presenter.Presenter, notes presenter.Notes) {
	p.ShowNotes(notes)
}

// EmitWarning forwards a structured warning (including the post-PONR warn-only
// case) to the presenter's Warn. The Warning's Label and Message stay SEPARATE
// structured fields end to end — the engine never collapses them into one combined
// string, and the presenter prefixes the label from its own field. Thin mapper.
func EmitWarning(p presenter.Presenter, warning presenter.Warning) {
	p.Warn(warning)
}
