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
		// This is the FINAL gate of a first release — accepting it records, tags,
		// pushes, and publishes. The wording mirrors ReleaseReviewGate ("[y] release")
		// so the user is clear acceptance proceeds with the release, not merely approves
		// the text. It offers y/n/e only (no r) — first-release notes are fixed-body, so
		// there is no AI to regenerate.
		Question:   "Release with these notes?",
		Subject:    "notes",
		AcceptEcho: "accepted",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "release"},
			{Key: presenter.ChoiceNo, Action: "abort"},
			{Key: presenter.ChoiceEdit, Action: "edit in $EDITOR"},
		},
		Default: presenter.ChoiceYes,
		Review:  true,
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
// detail is the terse completion value ("v1.4.0 (patch bump)").
func emitGateSucceeded(p presenter.Presenter, name, detail string) {
	p.StageSucceeded(presenter.StageSuccess{Name: name, Key: name, Detail: detail})
}

// startBlockingStage narrates the START of a BLOCKING stage — a long/slow step
// (notes generation, the pre_tag and post_release hooks, the atomic push) — by
// emitting a StageStarted with Blocking:true so the Rail's spinner animates, and
// starts the engine-side elapsed timer. It returns the stage under way, whose done,
// when the stage succeeds, emits the matching StageSucceeded carrying the
// engine-MEASURED Elapsed (the engine times the stage, by now; the presenter never
// does) and Blocking:true so the success event mirrors the start. A FAILED stage never
// calls done — its StageFailed, timed by failure (for post_release, its warn-only
// Warn), narrates the stage instead — so a non-success path emits no StageSucceeded.
// The stage's name is its plan Key too. text is the OPTIONAL activity phrase shown
// while the stage runs ("generating release notes…") — the
// state-what-is-happening rule; empty falls back to the plan stage's detail.
func startBlockingStage(p presenter.Presenter, now func() time.Time, name, text string) *blockingStage {
	return startBlockingStageSince(p, now, name, text, now())
}

// startBlockingStageSince is startBlockingStage for a stage whose wait began at
// started, before the stage was narrated.
func startBlockingStageSince(p presenter.Presenter, now func() time.Time, name, text string, started time.Time) *blockingStage {
	return startStep(p, now, stepOf{name: name, key: name}, text, started)
}

// stepOf is what a blocking stage is a step of: name is its label, plain's text, and
// key the plan stage its events fill, its name but where it runs on another stage's
// node. ongoing marks a step the node goes on past, whose success is Ongoing: a
// regenerate --all version's notes, a changelog's push.
type stepOf struct {
	name, key string
	ongoing   bool
}

// startStep is startBlockingStageSince for a stage that is step.
func startStep(p presenter.Presenter, now func() time.Time, step stepOf, text string, started time.Time) *blockingStage {
	p.StageStarted(presenter.StageStart{Name: step.name, Key: step.key, Blocking: true, Text: text})
	return &blockingStage{p: p, now: now, step: step, started: started}
}

// startQuietStep is startStep, from now, for a step plain has no lines for: its start
// and its success are Quiet.
func startQuietStep(p presenter.Presenter, now func() time.Time, step stepOf, text string) *blockingStage {
	p.StageStarted(presenter.StageStart{Name: step.name, Key: step.key, Blocking: true, Text: text, Quiet: true})
	return &blockingStage{p: p, now: now, step: step, started: now(), quiet: true}
}

// blockingStage is a blocking stage under way, timed by now from when its wait
// began, less any time it spent waiting on an answer; a quiet one's events are
// Quiet.
type blockingStage struct {
	p       presenter.Presenter
	now     func() time.Time
	step    stepOf
	started time.Time
	quiet   bool
	waits   *answerWaits
}

// done narrates the stage's success with detail.
func (s *blockingStage) done(detail string) {
	s.succeeded(presenter.StageSuccess{Detail: detail})
}

// succeeded narrates the stage's success, filling in its name, key, elapsed and
// whether its node goes on past it.
func (s *blockingStage) succeeded(success presenter.StageSuccess) {
	success.Name, success.Key, success.Quiet, success.Ongoing = s.step.name, s.step.key, s.quiet, s.step.ongoing
	success.Elapsed, success.Blocking = s.elapsed(), true
	s.p.StageSucceeded(success)
}

// elapsed is how long the stage has run, less the time it spent waiting on an answer.
func (s *blockingStage) elapsed() time.Duration {
	d := s.now().Sub(s.started)
	if s.waits != nil {
		d -= s.waits.total
	}
	return d
}

// failure is the StageFailed payload for the stage failing on cause, timed.
func (s *blockingStage) failure(cause error) presenter.StageFailure {
	failure := stageFailure(s.step.name, cause)
	failure.Key, failure.Elapsed = s.step.key, s.elapsed()
	return failure
}
