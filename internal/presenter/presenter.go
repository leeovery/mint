// Package presenter defines the event/step-oriented seam between mint's engine
// and its terminal output. The engine emits semantic lifecycle events; an
// implementation (pretty or plain) decides how they look.
package presenter

import "time"

// Presenter is the dependency-inversion seam for all of mint's output. The
// engine calls these methods at lifecycle points and nothing else — it never
// touches colour, spinners, or TTY state, which live entirely behind the
// interface.
//
// The contract follows the event-payload principle: the engine supplies, in
// each event's payload, every datum the rendering consumes. A Presenter renders
// what it is handed and never re-derives engine knowledge — it holds no
// hardcoded stage-name lists and times no stages.
//
// This is the minimal event set for the walking skeleton: start-of-run, the
// three per-stage transitions, and end-of-run. The fuller vocabulary (Warn,
// Unwound, ShowPlan, ShowNotes, Prompt) is added in later phases; the payload
// structs are designed to extend by adding fields without churning callers.
type Presenter interface {
	// RunStarted renders the start-of-run brand/header line.
	RunStarted(info RunInfo)
	// StageStarted renders the beginning of a stage (a spinner in pretty; a
	// terse start line in plain for blocking stages only).
	StageStarted(s StageStart)
	// StageSucceeded renders a stage's successful completion.
	StageSucceeded(s StageSuccess)
	// StageFailed renders a stage's failure, including captured command output.
	StageFailed(s StageFailure)
	// RunFinished renders the end-of-run success line.
	RunFinished(r RunResult)
}

// RunInfo carries the start-of-run payload. Action is the engine-supplied verb
// word (e.g. "releasing", "regenerating") so the start-of-run line is
// verb-shaped from the payload rather than hardcoding any literal in the
// presenter.
//
// Leaf is the engine-supplied brand glyph for the brand lines. It ties to the
// engine's commit_prefix brand, so — per the event-payload principle — the
// presenter renders the supplied leaf rather than re-deriving or hardcoding one.
// An empty Leaf defaults to 🌿 at render time, keeping existing callers (and
// their leaf-less struct literals) working unchanged.
type RunInfo struct {
	Project string
	Version string
	Action  string
	Leaf    string
}

// StageStart carries the StageStarted payload. Blocking is engine knowledge —
// it is set when the engine is about to invoke a long/slow command (e.g. claude
// or a build hook). Plain uses the flag to decide whether to emit a start line;
// pretty always shows progress.
type StageStart struct {
	Name     string
	Blocking bool
}

// StageSuccess carries the StageSucceeded payload. Elapsed is measured by the
// engine — the presenter does not time stages. Pretty renders the elapsed time
// on long/blocking stages only, which is why Blocking travels with the success
// event too.
type StageSuccess struct {
	Name     string
	Detail   string
	Elapsed  time.Duration
	Blocking bool
}

// StageFailure carries the StageFailed payload. Output is the underlying
// command output the engine captured (git/claude/gh chatter) rather than
// streamed; the presenter renders it. Rendering of Output is exercised in a
// later phase — the field exists now so the contract is stable.
type StageFailure struct {
	Name    string
	Message string
	Output  string
}

// RunResult carries the end-of-run success payload. URL is optional — verbs
// that do not publish a release (e.g. regenerate) leave it empty.
//
// Leaf mirrors RunInfo.Leaf: the engine-supplied brand glyph for the closing
// brand line, defaulting to 🌿 when empty. It travels on the result so the
// end-of-run line renders the brand from the payload rather than hardcoding it.
type RunResult struct {
	Project string
	Version string
	URL     string
	Leaf    string
}
