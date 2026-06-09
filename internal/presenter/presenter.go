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
	// ShowPlan renders the upcoming plan — the steps mint is about to perform.
	// It is narration → out only; it never writes to err.
	ShowPlan(plan Plan)
	// RunFinished renders the end-of-run success line.
	RunFinished(r RunResult)
}

// PlanStep is one structured step of the plan the engine is about to perform: an
// engine-supplied Verb (e.g. "commit", "tag", "push", "publish") and a Target
// describing what it acts on (e.g. "v1.4.0", "--atomic → origin"). Both fields
// are engine-supplied and rendered verbatim — the presenter synthesises no part
// of a step.
//
// Target may be empty: a verb that takes no target renders as just the verb in
// both modes, with no trailing space or separator.
type PlanStep struct {
	// Verb is the engine-supplied action word, rendered verbatim.
	Verb string
	// Target describes what the verb acts on, rendered verbatim. The empty string
	// is legal and renders the verb alone.
	Target string
}

// Plan is the ShowPlan payload: an ordered list of structured PlanStep values.
// Per the event-payload principle, BOTH render modes format from this SAME
// []Steps — there is NO separate pre-formatted or terse field. Pretty renders a
// bulleted block; plain joins the steps into a "plan: …; …" one-liner. The
// worked-example abbreviations are illustrative wording the engine supplies in
// the step targets, not a distinct terse payload.
//
// An empty plan (no steps) is legal: plain renders exactly "plan:" with no
// dangling separator; pretty omits the entire block (no orphan header).
type Plan struct {
	// Steps is the ordered list of plan steps. A nil/empty slice is an empty plan.
	Steps []PlanStep
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
// pretty always shows progress. The presenter never infers Blocking from the
// stage Name: there is no hardcoded list of long stages here.
type StageStart struct {
	// Name is the engine-supplied stage label rendered verbatim.
	Name string
	// Blocking marks a long/slow stage. Renderers consume the flag directly and
	// never derive it from Name.
	Blocking bool
}

// StageSuccess carries the StageSucceeded payload. Elapsed is measured by the
// engine — the presenter does not time stages. Pretty renders ({elapsed}) on
// long/blocking stages only, which is why Blocking travels with the success
// event too: it mirrors the StageStart.Blocking flag for the same stage.
//
// Zero-value semantics — fixed here so the rendering tasks can rely on them:
//
//  1. A short stage (Blocking==false) carries no meaningful elapsed. Renderers
//     MUST NOT print elapsed for a short stage regardless of the Elapsed value;
//     the flag, not the duration, gates elapsed rendering.
//  2. Elapsed==0 is legal even when Blocking==true and MUST NOT be treated as
//     "no elapsed" — a long stage that completed in under the timer's resolution
//     still renders as a long stage. There is no sentinel duration.
//  3. Detail=="" is legal; the payload supplies no default. Renderers fall back
//     to the ok/detail-less form.
type StageSuccess struct {
	// Name is the engine-supplied stage label rendered verbatim.
	Name string
	// Detail is the engine-supplied completion detail. The empty string is legal
	// (semantic 3) and means "render the detail-less form".
	Detail string
	// Elapsed is the engine-measured stage duration. It is only meaningful when
	// Blocking is true (semantic 1); zero is a valid duration there (semantic 2),
	// not a "no elapsed" sentinel.
	Elapsed time.Duration
	// Blocking mirrors StageStart.Blocking for the same stage and gates whether a
	// renderer shows ({elapsed}).
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
