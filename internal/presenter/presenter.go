// Package presenter defines the event/step-oriented seam between mint's engine
// and its terminal output. The engine emits semantic lifecycle events; an
// implementation (pretty or plain) decides how they look.
package presenter

import (
	"io"
	"time"
)

// writeNotesBody writes a release-notes body to w BYTE-FOR-BYTE VERBATIM, the
// single shared point that guarantees the body region is identical across the
// plain and pretty presenters (both call this with the UNCHANGED Notes.Body). It
// is the mechanical heart of the non-negotiable byte-identity invariant: only the
// surrounding delimiters differ between modes, never the body.
//
// It applies NO transformation — no stripping, no emoji handling, no re-wrapping,
// no truncation, and crucially NO indentation. The pretty worked example shows
// the body indented two spaces under the rule; that indentation is ILLUSTRATIVE
// and is deliberately NOT applied here, because adding indent bytes would break
// byte-identity, which the spec calls non-negotiable ("what previews is what
// ships"). The decorative rules are framing and are rendered by each presenter
// separately; the body is written flush and unchanged by this one helper.
//
// An empty body writes nothing — the caller's opener is then immediately
// followed by its closer, with no spurious blank line or invented content. A
// non-empty body is followed by exactly one newline so the caller's closer
// starts on its own line regardless of whether the body already ended in one.
func writeNotesBody(w io.Writer, body string) {
	if body == "" {
		return
	}
	// Fprint writes the body bytes unchanged; the trailing newline ensures the
	// closer that follows lands on its own line. The write error has nowhere to
	// propagate (Presenter methods return nothing) so it is discarded, mirroring
	// the presenters' writef.
	_, _ = io.WriteString(w, body)
	_, _ = io.WriteString(w, "\n")
}

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
// This started as the minimal event set for the walking skeleton (start-of-run,
// the three per-stage transitions, end-of-run); the fuller vocabulary (Warn,
// Unwound, ShowPlan, ShowNotes, Prompt) was added in later phases without
// churning callers — proof the payload structs extend by adding fields. Prompt
// is the gate seam: its Gate model lives in gate.go.
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
	// Warn renders a structured, label-prefixed warning. It is INDEPENDENT of
	// StageFailed/Unwound: it does not set failure state and does not suppress the
	// success end-of-run line. Per the stream contract a warning is narration → out
	// AND is duplicated to err (stderr) for redirect-visibility.
	Warn(w Warning)
	// Unwound renders the auto-unwind event — a FIRST-CLASS event distinct from
	// StageFailed — narrating what the engine undid after a failure or an abort. It
	// has its own glyph (↩ in pretty) and renders the engine-supplied Summary
	// VERBATIM (the presenter does NOT synthesise the "repo clean" tail — that tail
	// is part of the engine's Summary). Unlike StageFailed/Warn, Unwound is
	// narration → out ONLY: per the per-event stream table the auto-unwind line is
	// not duplicated to err. Like StageFailed it marks the run as terminal so the
	// success end-of-run line is suppressed; the presenter never sets an exit code.
	Unwound(u Unwind)
	// ShowPlan renders the upcoming plan — the steps mint is about to perform.
	// It is narration → out only; it never writes to err.
	ShowPlan(plan Plan)
	// ShowNotes renders the generated release notes inside per-mode delimiters.
	// It is narration → out only; it never writes to err. The body is written
	// BYTE-FOR-BYTE VERBATIM in both modes (see Notes) — only the surrounding
	// delimiters differ.
	ShowNotes(notes Notes)
	// Prompt is RENDER-ONLY: it renders the gate's DECLARED choice set (the
	// vertical menu + the Question prompt), reads ONE line of input, and returns a
	// single DECLARED Choice. It NEVER invokes $EDITOR or claude — the engine owns
	// the e/r re-entry loop (it does the edit/regenerate work, re-calls ShowNotes
	// with the refreshed body, and re-calls Prompt, looping until y/n; see the
	// regenerate flow). The presenter only re-renders on each pass.
	//
	// The render-only contract is explicit and load-bearing:
	//
	//   - On a returned e (edit) or r (regenerate) the presenter does NOTHING beyond
	//     returning that Choice — it spawns no subprocess, invokes no $EDITOR, and
	//     runs no claude/regeneration. The "edit in $EDITOR"/"regenerate" strings are
	//     display LABELS and doc comments only; the package imports no os/exec or any
	//     subprocess-spawning package (guarded by a test). The work is the ENGINE's.
	//   - The ENGINE owns the e/r re-entry loop end to end: on e/r it does the
	//     edit/regenerate work, re-calls ShowNotes with the refreshed body, then
	//     re-calls Prompt — looping until the choice is y or n, which exits the loop.
	//   - Rendering is LINEAR (print-style, append-only): each re-entry pass re-prints
	//     the notes block + gate BELOW the previous output; it scrolls. There is NO
	//     screen-clearing, NO alt-screen, and NO cursor-home overwrite — mint is not a
	//     Bubble Tea / full-screen TUI. (lipgloss SGR colour codes in pretty are not
	//     screen control and are fine; the ban is specifically on clear/alt-screen/home
	//     sequences.) The pretty spinner stop/resume around the $EDITOR hand-off is a
	//     separate, engine-driven concern (Phase 4), not part of this render contract.
	//
	// The error return carries the forbidden-combination / EOF failure surfaced in
	// later phases (non-TTY stdin without -y fails loud; EOF on input). It is
	// declared now so the signature is stable across those phases.
	//
	// This phase ships only the data model + signature: the Plain/Pretty
	// implementations are STUBS that return gate.Default with a nil error. The real
	// behaviour layers in later — line-read input parsing, the pretty vertical
	// menu, the -y gate skip, and the fail-loud forbidden-combination check — each
	// replacing/extending the stub without churning this signature.
	Prompt(gate Gate) (Choice, error)
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

// Notes is the ShowNotes payload: the generated release notes the engine is
// presenting for review. Version labels the surrounding delimiters; Body is the
// notes content itself.
//
// THE NON-NEGOTIABLE INVARIANT: Body is written BYTE-FOR-BYTE VERBATIM in BOTH
// render modes — no stripping, no emoji removal, no case-folding, no re-wrapping,
// no truncation, and NO indentation added. Only the surrounding delimiters
// differ between plain and pretty; the body region is provably identical across
// modes (both presenters write Body through the same shared writeNotesBody
// helper). Transforming the body would contradict the engine's "use the body
// whole" rule and break the "what previews is what ships" invariant. Emoji
// section headers (✨ Features, 🐛 Fixes) therefore survive verbatim in plain
// mode too — the plain byte-purity guard applies to synthesised stage narration,
// not to this engine-supplied body.
//
// Notes are narration → out only, never stderr.
//
// Edge cases the renderers must honour (all flow from "verbatim, positional"):
//
//   - An empty Body renders the delimiters with NO spurious blank line or
//     invented content between them (the opener line is immediately followed by
//     the closer line).
//   - A Body line that itself looks like a delimiter is written verbatim; the
//     real closing delimiter still follows it. Delimiters are POSITIONAL, never
//     content-matched — the body is never escaped or scanned for them.
//   - Internal blank lines in a multi-line Body are preserved exactly.
type Notes struct {
	// Version labels the surrounding delimiters (rendered as "v{Version}").
	Version string
	// Body is the release-notes content, written byte-for-byte verbatim in both
	// modes. The empty string is legal (bare delimiters, no invented content).
	Body string
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

// Warning carries the Warn payload: a structured, engine-supplied Label and
// Message kept as SEPARATE fields. The presenter NEVER parses a label out of a
// single combined string — both fields arrive independently and both renderings
// are label-prefixed from them ("{label}: WARN - {message}" in plain, "⚠ {label}
// {message}" in pretty).
//
// A Warn is INDEPENDENT of the stage transitions: it does not set failure state
// and does not suppress the success end-of-run line — a warn can occur on an
// otherwise-successful run. Multiple warnings render independently and in order;
// the presenter never collapses or de-duplicates them.
//
// Per the stream contract a warning is narration → out AND is additionally written
// to err (stderr) for visibility under redirection, mirroring StageFailed's err
// summary.
//
// Edge case: an empty Message is legal — the label still prefixes the fixed
// "WARN - " form with nothing after it; the presenter invents no message text.
type Warning struct {
	// Label is the engine-supplied warning label (e.g. "post_release"), rendered
	// verbatim as the line's prefix.
	Label string
	// Message is the engine-supplied warning text, rendered verbatim. The empty
	// string is legal and renders the label-prefixed form with no message.
	Message string
}

// Unwind carries the Unwound payload: the engine's verbatim "what it undid"
// summary, narrated after a failed or aborted run. Summary is a single
// engine-authored string that INCLUDES its own trailing "repo clean" tail (the
// pretty worked example reads "… — repo clean"; the plain example reads "…; repo
// clean"). The presenter renders Summary VERBATIM and synthesises NO part of it —
// in particular it does not append, normalise, or invent the "repo clean" tail,
// because the tail's exact wording and separator are engine content.
//
// Unwound is a first-class event (not a StageFailed). It is the abort path's
// terminal narration: gate-n produces an Unwound with no prior StageFailed, while
// a failed stage produces a StageFailed followed by an Unwound. Either way the
// presenter treats the run as terminal and suppresses the success end-of-run line.
type Unwind struct {
	// Summary is the engine-supplied "what it undid" text, rendered verbatim and
	// INCLUDING its own "repo clean" tail. The empty string is legal and renders the
	// label-prefixed form with no summary.
	Summary string
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
