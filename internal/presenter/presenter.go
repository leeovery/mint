// Package presenter defines the event/step-oriented seam between mint's engine
// and its terminal output. The engine emits semantic lifecycle events; an
// implementation (pretty or plain) decides how they look.
package presenter

import (
	"fmt"
	"io"
	"strings"
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
	// RunStarted renders the start-of-run brand/header line. Under regenerate's
	// per-version narration (especially --all, oldest→newest) the engine emits ONE
	// block per version, each opening with its own RunStarted; the presenter renders
	// the blocks LINEARLY in emit order and adds NO per-version ordering of its own.
	// Block ordering is engine-owned — the presenter renders whatever sequence it is
	// handed.
	RunStarted(info RunInfo)
	// StageStarted renders the beginning of a stage (a spinner in pretty; a
	// terse start line in plain for blocking stages only).
	//
	// Stream-discipline note for blocking stages: in pretty mode the spinner's
	// frames are written to out from the spinner's own goroutine until the stage
	// completes. Between a blocking StageStarted and its StageSucceeded/StageFailed
	// the engine should therefore emit only SuspendSpinner/ResumeSpinner, the notes
	// stage's NotesChecking/NotesChecked/NotesProgress/NotesHalving/NotesMerging,
	// Warn, and the completion event itself. Warn is safe because pretty's Warn stops
	// the live spinner before it prints, so a Warn may also END a blocking stage in
	// place of its completion (the notes batch skip, a failed post_release hook). Any
	// other narration (ShowPlan, …) would interleave with animation frames on the
	// same stream and garble the display: emit it before the stage starts or after it
	// completes (or wrap it in SuspendSpinner/ResumeSpinner).
	StageStarted(s StageStart)
	// StageSucceeded renders a stage's successful completion.
	StageSucceeded(s StageSuccess)
	// StageFailed renders a stage's failure, including captured command output.
	StageFailed(s StageFailure)
	// NotesChecking narrates the exclusion check's call starting inside the blocking
	// notes stage, and NotesChecked its end, with the call's elapsed: pretty shows the
	// check on the running notes line, then restores the line's own text; plain
	// prints the check as its own line and nothing at its end.
	NotesChecking(c NotesCheck)
	NotesChecked(c NotesCheck)
	// NotesProgress, NotesHalving and NotesMerging narrate a chunked notes run
	// inside its blocking notes stage: pretty rewrites the running notes line in
	// place, plain prints a line per event. They are the stream-discipline
	// exception — events the presenter renders without breaking the running line.
	// With no notes line running, pretty renders nothing. Each may be called from
	// any goroutine, but never concurrently with another Presenter call.
	NotesProgress(n NotesProgress)
	// NotesHalving narrates a call too big for the model being re-cut into smaller
	// parts.
	NotesHalving(h NotesHalving)
	// NotesMerging narrates the merge call that ends a chunked notes run.
	NotesMerging(m NotesMerge)
	// NextMoves renders the ways forward from a failure that ends the run, to out
	// only. Like Warn it sets no failure state.
	NextMoves(m NextMoves)
	// ExclusionSuggestions renders the exclusion check's suggestions no pause asked
	// about, each with the config line that acts on it, to out only. Like Warn it sets
	// no failure state.
	ExclusionSuggestions(s ExclusionSuggestions)
	// ExclusionAnswersRecorded renders one line saying the exclusion check's answers
	// were written to a table of the config file, and whether they were committed, to
	// out only.
	ExclusionAnswersRecorded(a ExclusionAnswers)
	// PreviewGate renders the question gate would ask, without its choices, for a dry
	// run that shows where a real run would stop to ask. It reads no input and sets no
	// failure state.
	PreviewGate(gate Gate)
	// Warn renders a structured, label-prefixed warning. It is INDEPENDENT of
	// StageFailed/Unwound: it does not set failure state and does not suppress the
	// success end-of-run line. Per the stream contract a warning is narration → out
	// AND is duplicated to err (stderr) for redirect-visibility.
	Warn(w Warning)
	// Unwound renders the auto-unwind event — a FIRST-CLASS event distinct from
	// StageFailed — narrating what the engine undid after a failure or an abort. It
	// has its own glyph (↩ in pretty) and renders the engine-supplied parts joined
	// into one line (the presenter does NOT synthesise the "repo clean" tail — that
	// is the engine's Repo part). Unlike StageFailed/Warn, Unwound is
	// narration → out ONLY: per the per-event stream table the auto-unwind line is
	// not duplicated to err. Like StageFailed it marks the run as terminal so the
	// success end-of-run line is suppressed; the presenter never sets an exit code.
	Unwound(u Unwind)
	// ShowPlan renders the upcoming plan — the steps mint is about to perform — or,
	// for a Quiet plan, nothing. It is narration → out only; it never writes to err.
	ShowPlan(plan Plan)
	// ShowNotes renders the generated release notes inside per-mode delimiters.
	// It is narration → out only; it never writes to err. The body is written
	// BYTE-FOR-BYTE VERBATIM in both modes (see Notes) — only the surrounding
	// delimiters differ.
	ShowNotes(notes Notes)
	// ShowMessage renders an engine-titled content block inside per-mode
	// delimiters — the general-purpose sibling of ShowNotes for content that is
	// not release notes (e.g. a generated commit message presented for review).
	// The TITLE labels the delimiters and is engine-supplied verbatim; the BODY
	// is written BYTE-FOR-BYTE VERBATIM in both modes through the same shared
	// helper ShowNotes uses, so the body region is identical across modes and
	// "what previews is what ships" holds for this block too (see Message).
	// Narration → out only; it never writes to err.
	ShowMessage(m Message)
	// ShowVersion renders the resolved version. It is THE PAYLOAD EXCEPTION: version
	// is the one verb whose output is a VALUE, not narration, so its plain output is
	// a RAW VALUE (not the key:value narration every other plain line uses) — the
	// bare value plus a single trailing newline and nothing else — so `$(mint
	// version)` consumes it cleanly. Pretty MAY dress it ("{leaf} mint v{value}")
	// since styling is additive only in pretty; the bare value is the floor.
	//
	// Narration → out ONLY; the value never goes to err. version has NO interactive
	// gate (it never calls Prompt) and NO release-style brand footer / "done:" line
	// (the engine never calls RunFinished for version) — the value line IS the
	// terminal output.
	ShowVersion(v Version)
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
	// Both implementations are FULLY BEHAVIOURAL and share one line-read core
	// (readChoice/parseChoice): empty Enter selects the gate's Default,
	// case-insensitive input maps to a declared key, unrecognised input re-prompts,
	// and only the render closure differs per mode (terse one-liner in plain, the
	// vertical menu in pretty).
	//
	// The -y skip happens INSIDE this method: a presenter constructed with the -y
	// decision neither renders the menu nor reads stdin — it emits the rendered
	// auto-accept echo and returns gate.Default with a nil error. The engine never
	// answers a gate itself on -y: the echo is a rendered event, not engine-printed
	// text.
	//
	// The error return is the machine-readable failure channel, branched via
	// errors.Is: ErrNotInteractive on the forbidden combination (non-TTY stdin
	// without -y; ALSO rendered as a fail-loud failure line by the presenter
	// itself), and ErrInputClosed on EOF mid-gate (NOT rendered — the engine owns
	// that failure's surfacing; see ErrInputClosed). The presenter never sets an
	// exit code.
	Prompt(gate Gate) (Choice, error)
	// AskLine is the FREE-TEXT input seam: it renders a one-line prompt (the
	// engine-supplied prompt label, framed per mode), reads ONE raw line from the
	// SAME persistent input reader Prompt uses, and returns the line VERBATIM with
	// only the trailing newline (and any preceding carriage return) stripped.
	// Unlike Prompt there is no choice set, no default, and no re-prompt loop —
	// the empty string is a legal answer (the engine owns its meaning, e.g. "no
	// extra context"), and leading/inner/trailing spaces are preserved. The engine
	// uses it for one-shot free-text asks such as regenerate's one-time context
	// line; because it shares Prompt's buffered reader, a Prompt followed by an
	// AskLine consumes consecutive lines of the same stream.
	//
	// Input-axis rules: a non-interactive stdin fails loud exactly like Prompt's
	// forbidden combination — the presenter renders the failure (label "input")
	// and returns ErrNotInteractive. -y does NOT auto-answer an AskLine (free text
	// has no declared default to accept); engine flows only reach AskLine from an
	// interactive gate choice, so under -y it is unreachable by construction. EOF
	// with no usable line returns ErrInputClosed (NOT rendered — the engine owns
	// that failure's surfacing); a final line without a trailing newline is still
	// returned. The prompt is narration → out only.
	AskLine(prompt string) (string, error)
	// SuspendSpinner and ResumeSpinner are ENGINE-CALLABLE control hooks the engine
	// invokes AROUND its OWN $EDITOR hand-off (the presenter never detects or invokes
	// the editor — per the Phase-3 render-only Prompt contract the engine owns the
	// e/r re-entry loop and the $EDITOR hand-off). $EDITOR takes over the terminal, so
	// the engine calls SuspendSpinner to stop the pretty spinner's animation before
	// handing off — releasing the terminal so the editor session is animation-free —
	// and ResumeSpinner to restart it on the SAME stage line afterwards.
	//
	// Both are SAFE NO-OPS when no spinner is active, and both are no-ops in plain
	// (which never animates). They are a plain stop-then-start — no alt-screen, no
	// screen-clear — consistent with the linear print-style narration, and they
	// preserve the one-spinner-at-a-time invariant: after any number of suspend/resume
	// cycles there is still at most one spinner. A stage that completes
	// (StageSucceeded/StageFailed) while suspended clears the suspended state, so a
	// later ResumeSpinner does not resurrect a spinner for an already-completed stage.
	//
	// These hooks suspend/resume the presenter's OWN animation only; the presenter
	// imports no os/exec and spawns no subprocess (guarded by a test).
	SuspendSpinner()
	// ResumeSpinner restarts the spinner suspended by SuspendSpinner on the same
	// stage line (see SuspendSpinner). It is a no-op when nothing was suspended and in
	// plain mode.
	ResumeSpinner()
	// InitResult renders one init outcome — a created or skipped target — in the
	// shared cross-verb vocabulary (pretty: "✓ created {target}" / "· skipped
	// {target} ({reason})"; plain: "{target}: created" / "{target}: skipped
	// ({reason})"). It is narration → out ONLY; it never writes to err.
	//
	// Per the event-payload principle the ENGINE resolves created-vs-skipped and,
	// for a skip, supplies the --force reason text; the presenter NEVER decides the
	// Action or knows --force semantics. A --force overwrite arrives as InitCreated
	// (the engine resolved it) and is narrated as a plain created line — the
	// presenter does not special-case --force.
	//
	// init has NO interactive gate (it never calls Prompt — its safety is structural
	// via non-clobber + --force, not a prompt) and NO release-style brand footer /
	// "done:" line: its created/skipped lines ARE the terminal output. The presenter
	// does not special-case init for the footer; rather the engine simply never calls
	// RunFinished or Prompt on an init run.
	InitResult(r InitOutcome)
	// RunFinished closes the run and renders the end-of-run success line for the
	// outcomes that have one (see Outcome). A release, a single-version regenerate and
	// a commit end every run they opened with RunStarted with exactly one. A
	// regenerate --all backfill is the exception: it opens a block per version with a
	// RunStarted of its own and closes the batch once, with one RunFinished, when its
	// version loop finishes, even with every version skipped. That close comes before
	// the changelog rebuild, which can still fail after it; a loop a decline or a
	// cancel ends early closes nothing.
	RunFinished(r RunResult)
}

// InitAction is the engine-resolved disposition of one init target: it was either
// created (written fresh, or overwritten under --force) or skipped (it already
// existed and --force was not passed). It is a typed action the engine ALWAYS sets
// explicitly on the InitOutcome — the presenter renders whichever action it is
// handed and never resolves the disposition itself.
//
// iota makes the zero value InitCreated. That is intentional and safe here:
// InitAction never travels as a default-constructed zero value — the engine sets it
// explicitly on every outcome — so the zero-value identity carries no hidden
// "unset" meaning to guard against.
type InitAction int

const (
	// InitCreated is the created disposition: the target was written fresh, or
	// overwritten under --force. Both render as the created line — the presenter does
	// not distinguish a fresh write from a --force overwrite (the engine resolved
	// both to InitCreated).
	InitCreated InitAction = iota
	// InitSkipped is the skipped disposition: the target already existed and --force
	// was not passed. It renders the skipped notice with the engine-supplied Reason.
	InitSkipped
)

// String renders the action word ("created"/"skipped") used in both the rendered
// narration and readable test output. An unknown value (never produced by the
// engine) renders "unknown" so a drifted action is self-evident rather than a bare
// integer.
func (a InitAction) String() string {
	switch a {
	case InitCreated:
		return "created"
	case InitSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// InitOutcome is the InitResult payload: the engine-resolved disposition of one
// init target. Action is created-vs-skipped (engine-resolved — the presenter never
// decides it); Target is the file/path the action applied to, rendered verbatim;
// Reason is the engine-supplied skip explanation (e.g. "exists, use --force"),
// rendered VERBATIM and only meaningful for InitSkipped — a created outcome leaves
// it empty and the renderers never read it.
type InitOutcome struct {
	// Action is the engine-resolved disposition rendered as its action word. The
	// engine sets it explicitly on every outcome; the presenter renders it.
	Action InitAction
	// Target is the file/path the action applied to (e.g. ".mint.toml", "release"),
	// rendered verbatim.
	Target string
	// Reason is the engine-supplied skip explanation, rendered verbatim and only
	// meaningful for InitSkipped. The presenter synthesises no part of it and never
	// reads it for a created outcome.
	Reason string
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
//
// Stages and PointOfNoReturn are the rail's plan: the run's stages in order, each
// the node a stage event's Key fills. A run may show both kinds: release shows its
// Stages as soon as preflight passes, in a Quiet plan, and its Steps after the notes,
// where plain's line has always been.
type Plan struct {
	// Steps is the ordered list of plan steps. A nil/empty slice is an empty plan.
	Steps []PlanStep
	// Stages are the stages the run will go through, in order, those already done
	// included. A stage that will not run is left out.
	Stages []PlanStage
	// PointOfNoReturn is the Key of the first stage past the point of no return,
	// empty for a run with none.
	PointOfNoReturn string
	// Quiet marks a plan that only lays out the run's structure: plain's log, pinned
	// byte for byte from before the rail, has no line for it, so plain and today's
	// pretty write nothing.
	Quiet bool
}

// PlanStage is one stage of the rail's plan: the Key stage events carry, the Name
// shown in its node and the Detail of what it will do ("main + v1.4.0 → origin,
// atomic").
type PlanStage struct {
	Key    string
	Name   string
	Detail string
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

// Message is the ShowMessage payload: an engine-titled content block presented
// inside per-mode delimiters — the general-purpose sibling of Notes for content
// that is not release notes (e.g. a generated commit message shown for review).
//
// Title labels the surrounding delimiters and is engine-supplied, rendered
// VERBATIM (plain "--- {title} ---" … "--- end {title} ---"; pretty a titled
// rule). The presenter synthesises only the delimiter framing, never the title
// text. For the plain delimiters to stay byte-pure the engine supplies ASCII
// titles (the same convention as the gate Subject/AcceptEcho values).
//
// Body carries the SAME non-negotiable invariant as Notes.Body: it is written
// BYTE-FOR-BYTE VERBATIM in BOTH render modes through the shared writeNotesBody
// helper — no stripping, no re-wrapping, no truncation, no indentation — so the
// body region is provably identical across modes and only the delimiters differ.
// The same edge rules apply: an empty Body renders the delimiters with nothing
// between them, and a Body line that looks like a delimiter is written verbatim
// (delimiters are POSITIONAL, never content-matched).
//
// Messages are narration → out only, never stderr.
type Message struct {
	// Title labels the surrounding delimiters, rendered verbatim (ASCII by the
	// engine's convention so the plain delimiters stay byte-pure).
	Title string
	// Body is the block content, written byte-for-byte verbatim in both modes.
	// The empty string is legal (bare delimiters, no invented content).
	Body string
}

// Version is the ShowVersion payload: the engine-resolved version value plus the
// engine-supplied brand leaf, for the one payload verb (version's output is a
// value, not narration).
//
// Value is the resolved version (e.g. "1.4.0"), the load-bearing datum. Plain
// writes it as the BARE value (no "v" prefix, no glyph) so `$(mint version)`
// consumes it cleanly; pretty dresses it as "v{Value}" inside the brand line — the
// "v" prefix is a PRETTY-only decoration, never part of the plain value.
//
// Leaf mirrors RunInfo.Leaf / RunResult.Leaf: the engine-supplied brand glyph,
// defaulting to 🌿 when empty. Plain IGNORES it (the bare value carries no brand);
// pretty renders it as the brand leaf, consistent with the other brand lines.
type Version struct {
	// Value is the resolved version, rendered verbatim. Plain writes it bare; pretty
	// prefixes a decorative "v".
	Value string
	// Leaf is the engine-supplied brand glyph for the pretty brand line, defaulting
	// to 🌿 when empty. Plain never reads it.
	Leaf string
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
//
// Header and Start carry the rail's opening: its header line and the station the
// run starts from. Plain and today's pretty read neither.
type RunInfo struct {
	Project string
	Version string
	Action  string
	Leaf    string
	// DryRun marks the run as a --dry-run preview. When set, the start-of-run line
	// carries a "dry run" marker so the whole transcript is framed as a preview from
	// the first line — a dry run takes no action and (for release) shows no gates, so
	// the marker is the user's standing reminder that nothing will be changed.
	DryRun bool
	Header Header
	Start  Station
}

// Header is the data of the rail's header line: " mint ─ {Command}" on the left,
// "{Where} · v{Version}" on the right.
type Header struct {
	// Command is the command words as the header shows them: "release", "commit",
	// "release regenerate".
	Command string
	// Where is the repo root, the home directory shortened to "~".
	Where string
	// Version is mint's own bare version ("0.1.2"), empty for a dev build, which
	// shows none.
	Version string
}

// Station is a point the rail starts from or lands at: a name ("v1.3.2", "main")
// and its detail ("main @ 7c1e0b4", "released").
type Station struct {
	Name   string
	Detail string
}

// StageStart carries the StageStarted payload. Blocking is engine knowledge —
// it is set when the engine is about to invoke a long/slow command (e.g. claude
// or a build hook). Plain uses the flag to decide whether to emit a start line;
// pretty always shows progress. The presenter never infers Blocking from the
// stage Name: there is no hardcoded list of long stages here.
type StageStart struct {
	// Name is the engine-supplied stage label rendered verbatim.
	Name string
	// Key names the plan stage this event fills (PlanStage.Key). A stage outside the
	// plan (the review gate's edit) carries a key no plan stage has. It is separate
	// from Name because a stage's label is plain's text and can differ from its
	// stage: a failed release push is labelled "tag".
	Key string
	// Blocking marks a long/slow stage. Renderers consume the flag directly and
	// never derive it from Name.
	Blocking bool
	// Text is the OPTIONAL engine-supplied activity phrase shown WHILE the stage
	// runs ("generating commit message…"), per the state-what-is-happening CLI
	// guideline — the pretty spinner animates it. Empty falls back to Name. The
	// completion line still uses Name (+Detail), so Text never affects the
	// success/failure column layout, and plain mode ignores it (its terse
	// "{name}: running..." start line stays byte-stable for pipes).
	Text string
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
	// Key names the plan stage this event settles (see StageStart.Key).
	Key string
	// Commit is the subject of the commit the stage made ("chore(release): pre-tag
	// artifacts for v1.4.0"), empty when it made none. Plain and today's pretty
	// ignore it.
	Commit string
	// Quiet marks a success that only settles a stage of the run's structure: plain's
	// log, pinned byte for byte from before the rail, has no line for it, so plain
	// and today's pretty write nothing.
	Quiet bool
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
	// Sentence is the OPTIONAL human-readable completion line the PRETTY presenter
	// renders in place of the "{name}  {detail}" column form — a full past-tense
	// sentence ("Generated release notes", "Pushed branch + v1.4.0 atomically"). It
	// reads as narration rather than a stage codeword. Pretty appends ({elapsed})
	// to it for a blocking stage exactly as it would the column form. PLAIN ignores
	// it entirely and keeps the terse "{name}: {detail}" line (its pipe/log
	// contract); an empty Sentence makes pretty fall back to the column form too, so
	// the field is purely additive.
	Sentence string
}

// Settled is the Quiet success that settles the plan stage key with detail.
func Settled(key, detail string) StageSuccess {
	return StageSuccess{Name: key, Key: key, Detail: detail, Quiet: true}
}

// NotesProgress carries a chunked notes run's progress: Done of Parts finished,
// Done 0 as the run splits.
type NotesProgress struct {
	// PromptBytes is the size of the composed single-pass prompt the run split from.
	PromptBytes int
	Parts       int
	Done        int
}

// NotesHalving carries the NotesHalving payload: the re-cut call and the number of
// parts its re-cut yields.
type NotesHalving struct {
	// Part is the re-cut part's number within the run's total before the re-cut, 0
	// when the re-cut call is the single-pass call.
	Part  int
	Parts int
}

func halvingText(h NotesHalving) string {
	if h.Part == 0 {
		return fmt.Sprintf("too big for the model in one call, retrying in %d parts", h.Parts)
	}
	return fmt.Sprintf("part %d was too big for the model, retrying it as %d smaller parts", h.Part, h.Parts)
}

// NotesMerge carries the NotesMerging payload: the number of parts being merged.
type NotesMerge struct {
	Parts int
}

// NotesCheck carries the NotesChecking and NotesChecked payloads: Key names the
// check's plan stage, and Elapsed, on NotesChecked alone, is the engine-measured
// time of the check's call.
type NotesCheck struct {
	Key     string
	Elapsed time.Duration
}

// StageFailure carries the StageFailed payload. Output is the underlying
// command output the engine captured (git/claude/gh chatter) rather than
// streamed; the presenter renders it. SavedNotes is the path of the file a failed
// chunked notes run saved its finished part notes to, named after Output; empty
// names none.
type StageFailure struct {
	Name string
	// Key names the plan stage that failed (see StageStart.Key).
	Key        string
	Message    string
	Output     string
	SavedNotes string
	// Recoverable marks a failure the run may still go on past: it renders as any
	// other, but leaves a later RunFinished's success line standing.
	Recoverable bool
	// Elapsed is the engine-measured time a blocking stage ran before it failed,
	// zero for a short stage. Plain and today's pretty ignore it.
	Elapsed time.Duration
}

// NextMoves carries the NextMoves payload: the engine-authored ways forward from
// stage's failure, in order.
type NextMoves struct {
	Stage string
	Moves []NextMove
}

// NextMove is one way forward: a Lead saying what to do and, when the move ends in
// one, the Command or config line that does it, kept apart so the rail can set it on
// a line of its own. A move whose command sits mid-sentence keeps it in its Lead.
type NextMove struct {
	Lead    string
	Command string
}

// Text is the move as one sentence, Lead then Command: plain's and today's pretty's
// line.
func (m NextMove) Text() string {
	if m.Command == "" {
		return m.Lead
	}
	return m.Lead + " " + m.Command
}

// ExclusionSuggestions carries the ExclusionSuggestions payload: stage's suggestions,
// each rendered verbatim in order, their Lines set under Table.
type ExclusionSuggestions struct {
	Stage       string
	Table       string
	Suggestions []ExclusionSuggestion
}

// ExclusionAnswers carries the ExclusionAnswersRecorded payload: stage's answers went
// into Table of File, committed unless Ignored reports that git ignores File. Exclude
// and Keep are the globs answered each way, in the order answered; Key names the
// check's plan stage. Plain and today's pretty read neither the globs nor Key.
type ExclusionAnswers struct {
	Stage   string
	Key     string
	File    string
	Table   string
	Ignored bool
	Exclude []string
	Keep    []string
}

func exclusionAnswersText(a ExclusionAnswers) string {
	written := "written to " + a.File + "'s " + a.Table + " table"
	if a.Ignored {
		return written + " and not committed, as git ignores " + a.File
	}
	return written + " and committed"
}

// ExclusionSuggestion is one glob the exclusion check suggests leaving out, the AI's
// reason for it, what it matches, and the config line that would exclude it.
type ExclusionSuggestion struct {
	Glob    string
	Reason  string
	Matches string
	Line    string
}

func savedNotesText(path string) string {
	return "part notes saved to " + path
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
	// Key names the plan stage the warning ends in place of its success (a failed
	// post_release hook or publish, a failed commit push), empty for a warning that
	// ends no stage. Plain and today's pretty ignore it.
	Key string
	// Output is optional captured underlying-command output (e.g. git's stderr
	// from a failed-but-non-fatal push) rendered VERBATIM beneath the warn line —
	// the warn-flavoured counterpart of StageFailure.Output, for failures the
	// engine deliberately keeps non-terminal. It is narration → out ONLY and is
	// NEVER duplicated to err: the stream contract's "one-line summary to stderr"
	// applies to the warn line alone. Rendering mirrors StageFailed's captured
	// body exactly — plain wraps it in the sliceable "--- output ---" delimiters,
	// pretty writes it flush and unstyled — via the shared verbatim helper. The
	// empty string (the common case) renders NO block; rendering it through Warn
	// does not set failure state.
	Output string
}

// Unwind carries the Unwound payload: what the engine's unwind undid after a failed
// or aborted run, what it kept, and where it left the repo, each engine-authored and
// kept apart so the rail can set them on rows of their own. Plain and today's pretty
// join the non-empty parts with "; " into today's one line (Text) and synthesise no
// part of it.
//
// Unwound is a first-class event (not a StageFailed). It is the abort path's
// terminal narration: gate-n produces an Unwound with no prior StageFailed, while
// a failed stage produces a StageFailed followed by an Unwound. Either way the
// presenter treats the run as terminal and suppresses the success end-of-run line.
type Unwind struct {
	// Undone names what was undone: "reset 2 commits and deleted tag v1.4.0".
	Undone string
	// Kept names what the unwind kept on purpose, empty for nothing:
	// "kept the answers commit on the starting commit".
	Kept string
	// Repo is where the repo ended up: "repo clean", or "manual cleanup required"
	// when a recovery step failed.
	Repo string
}

// Text is the unwind as one line, its non-empty parts joined with "; ".
func (u Unwind) Text() string {
	var parts []string
	for _, part := range []string{u.Undone, u.Kept, u.Repo} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

// Outcome is how a run ended, the discriminator RunFinished renders by. A release, a
// single-version regenerate and a commit close every run they opened with one
// RunFinished carrying its Outcome; a regenerate --all backfill opens a block per
// version and closes the batch once, as OutcomeAllRegenerated, when its version loop
// finishes, even with every version skipped, and before the changelog rebuild, which
// can still fail after the close. A loop a decline or a cancel ends early closes
// nothing.
// Plain and today's pretty write today's success lines for the outcomes that had one —
// released (with or without a warning), dry run, committed (with or without a
// warning), and --all regenerated — and nothing for the rest, which end on lines of
// their own (a failure, an unwind) or, before the rail, on none.
//
// iota makes the zero value OutcomeReleased, so a RunResult literal that sets no
// Outcome closes as a release.
type Outcome int

const (
	// OutcomeReleased is a release that went out: "done: {project} v{X} {url}"
	// (plain) / "{leaf} released {project} v{X} · {url}" (pretty), the URL omitted
	// when empty.
	OutcomeReleased Outcome = iota
	// OutcomeReleasedWithWarning is a release that went out with a failure past the
	// point of no return (a publish or post_release hook failure), warned and never
	// unwound. It closes with the release line.
	OutcomeReleasedWithWarning
	// OutcomeDryRun is a dry run's close-out: "dry run: {project} v{X} — no changes
	// made". A dry run mutated nothing, so it never claims it released anything.
	OutcomeDryRun
	// OutcomeCommitted is a commit that landed, pushed or not: "done: {project}
	// committed" (plain) / "{leaf} committed {project}" (pretty), version-less and
	// URL-less.
	OutcomeCommitted
	// OutcomeCommittedWithWarning is a commit that landed and whose -p push failed,
	// warned and never unwound. It closes with the commit line.
	OutcomeCommittedWithWarning
	// OutcomeRegenerated is a single version regenerated. It has no line in plain or
	// today's pretty.
	OutcomeRegenerated
	// OutcomeAllRegenerated is a --all backfill's close: "done: {project} {Summary}"
	// (plain) / "{leaf} regenerated {project} {Summary}" (pretty), URL-less, the
	// engine-computed Summary rendered verbatim.
	OutcomeAllRegenerated
	// OutcomeDeclined is a run stopped by a question answered no, with nothing to
	// unwind. Its End detail says what it left (see End).
	OutcomeDeclined
	// OutcomeUnwound is a run whose unwind undid what it had made; the Unwound line
	// says what, and its End detail where that left the repo.
	OutcomeUnwound
	// OutcomeFailed is a run that failed with nothing to unwind, input closing at a
	// question included; its ✗ line says why, and its End detail what it left (see
	// End).
	OutcomeFailed
)

// RunResult carries the end-of-run payload. URL is optional — outcomes that do not
// publish a release leave it empty.
//
// Leaf mirrors RunInfo.Leaf: the engine-supplied brand glyph for the closing
// brand line, defaulting to 🌿 when empty. It travels on the result so the
// end-of-run line renders the brand from the payload rather than hardcoding it.
//
// End and Elapsed carry the rail's close; plain and today's pretty read neither.
type RunResult struct {
	Project string
	Version string
	URL     string
	Leaf    string
	// Outcome is how the run ended, selecting which closing line RunFinished renders.
	Outcome Outcome
	// Summary is the engine-supplied closing detail of a --all backfill, rendered
	// verbatim: the regenerated count and each skipped version.
	Summary string
	// End is the station the run landed at, its detail whole as the rail shows it and
	// derived from what the run did: "v1.4.0" / "released", "main" / "a3f9c21 ·
	// pushed", or the start station's name with what a run that stopped short left.
	// That detail states only mint's own actions and where HEAD stands, never how clean
	// the tree is or what a hook did: "nothing changed" only while no hook and no write
	// of mint's has run; after that what mint did ("changes staged, not committed",
	// "back where it started · nothing pushed", "nothing committed · nothing pushed"),
	// or, once a hook has run and mint has nothing of its own to name, "nothing
	// pushed".
	End Station
	// Elapsed is the engine-measured time of the whole run.
	Elapsed time.Duration
}
