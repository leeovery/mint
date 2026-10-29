// Package presenter defines the event/step-oriented seam between mint's engine
// and its terminal output. The engine emits semantic lifecycle events; an
// implementation (the Rail on a terminal, or plain) decides how they look.
package presenter

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// writeNotesBody writes a body (release notes, a commit message, captured command
// output) to w BYTE-FOR-BYTE VERBATIM, the single point plain writes every body
// through, with the UNCHANGED payload. It is the mechanical heart of the
// non-negotiable byte-identity invariant: plain adds only the surrounding
// delimiters, never a byte to the body.
//
// It applies NO transformation — no stripping, no emoji handling, no re-wrapping,
// no truncation, and crucially NO indentation, because adding indent bytes would
// break byte-identity, which the spec calls non-negotiable ("what previews is what
// ships"). The Rail shows a body behind its gutter, for display only; what ships is
// still the body byte for byte.
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
// touches colour, spinners, the cursor or TTY state, which live entirely behind
// the interface.
//
// The contract follows the event-payload principle: the engine supplies, in
// each event's payload, every datum the rendering consumes. A Presenter renders
// what it is handed and never re-derives engine knowledge — it holds no
// hardcoded stage-name lists. The engine measures every elapsed time that
// settles; the one clock a presenter runs is a running stage's, ticking for
// display only and never settled.
//
// This started as the minimal event set for the walking skeleton (start-of-run,
// the three per-stage transitions, end-of-run); the fuller vocabulary (Warn,
// Unwound, ShowPlan, ShowNotes, Prompt) was added in later phases without
// churning callers — proof the payload structs extend by adding fields. Prompt
// is the gate seam: its Gate model lives in gate.go.
type Presenter interface {
	// RunStarted renders the start-of-run brand/header line. A run opens once: a
	// RunStarted while one is open fills in only the start station it lacks, so a
	// regenerate can draw its header ahead of its questions and its station once they
	// are answered, and plain's start line can still come later, as it always has, and
	// once per version under regenerate --all (oldest→newest), whose blocks plain
	// renders LINEARLY in emit order. Ordering is engine-owned.
	RunStarted(info RunInfo)
	// StageStarted renders the beginning of a stage (the Rail's running node, its
	// spinner in the mark's place; a terse start line in plain for blocking stages
	// only).
	//
	// The Rail draws everything from the live node down in a region it redraws in
	// place, and settled lines above it never change; every event redraws it, so any
	// event may arrive while a stage runs. A Warn may END a blocking stage in place of
	// its completion (the notes batch skip, a failed post_release hook). Around the
	// engine's own $EDITOR hand-off it calls SuspendSpinner/ResumeSpinner.
	StageStarted(s StageStart)
	// StageSucceeded renders a stage's successful completion.
	StageSucceeded(s StageSuccess)
	// StageFailed renders a stage's failure, including captured command output.
	StageFailed(s StageFailure)
	// NotesChecking narrates the exclusion check's call starting inside the blocking
	// notes stage, and NotesChecked its end, with the call's elapsed: the Rail runs
	// the check as its own node and settles it with what the call found; plain
	// prints the check as its own line and nothing at its end.
	NotesChecking(c NotesCheck)
	NotesChecked(c NotesCheck)
	// NotesProgress, NotesHalving and NotesMerging narrate a chunked notes run
	// inside its blocking notes stage: the Rail draws them as the running notes
	// node's bar, plain prints a line per event. Each may be called from any
	// goroutine, but never concurrently with another Presenter call.
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
	// failure state. A Quiet gate's preview only fills in the run's structure.
	PreviewGate(gate Gate)
	// Warn renders a structured, label-prefixed warning. It is INDEPENDENT of
	// StageFailed/Unwound: it does not set failure state and does not suppress the
	// success end-of-run line. Per the stream contract a warning is narration → out
	// AND is duplicated to err (stderr) for redirect-visibility.
	Warn(w Warning)
	// Unwound renders the auto-unwind event — a FIRST-CLASS event distinct from
	// StageFailed — narrating what the engine undid after a failure or an abort. It
	// has its own mark (↩ on the Rail) and renders the engine-supplied parts (the
	// presenter does NOT synthesise the "repo clean" tail — that is the engine's Repo
	// part). Unlike StageFailed/Warn, Unwound is
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
	// version)` consumes it cleanly. The Rail dresses it (" mint v{value}") since
	// styling is additive only in pretty; the bare value is the floor.
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
	//   - There is NO screen-clearing, NO alt-screen, and NO cursor-home overwrite —
	//     mint is not a Bubble Tea / full-screen TUI. Plain is linear: each re-entry
	//     pass re-prints the notes block + gate BELOW the previous output. The Rail
	//     redraws only its live region, the question and what it asks about, in place;
	//     lines above it never change. The SuspendSpinner/ResumeSpinner around the
	//     $EDITOR hand-off is a separate, engine-driven concern, not part of this
	//     render contract.
	//
	// Both implementations are FULLY BEHAVIOURAL and share one parse (parseChoice):
	// empty Enter selects the gate's Default, case-insensitive input maps to a
	// declared key, and unrecognised input re-prompts. Plain reads a line; the Rail
	// reads a single keypress where the terminal allows and every key is one
	// character, and a line otherwise.
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
	// the editor — per the render-only Prompt contract the engine owns the e/r
	// re-entry loop and the $EDITOR hand-off). $EDITOR takes over the terminal, so
	// the engine calls SuspendSpinner before handing off: the Rail erases its live
	// region and stops its spinner, so the editor session is animation-free. On
	// ResumeSpinner it draws the region again.
	//
	// Both are SAFE NO-OPS when nothing is drawn live, and both are no-ops in plain
	// (which never animates). Neither uses the alternate screen or clears the
	// screen.
	//
	// These hooks suspend/resume the presenter's OWN animation only; the presenter
	// imports no os/exec and spawns no subprocess (guarded by a test).
	SuspendSpinner()
	// ResumeSpinner draws again what SuspendSpinner erased (see SuspendSpinner). It is
	// a no-op when nothing was suspended and in plain mode.
	ResumeSpinner()
	// InitResult renders one init outcome — a created or skipped target — in the
	// shared cross-verb vocabulary (the Rail: a ● row per file created, a dim ○ row
	// per file left alone with its reason; plain: "{target}: created" /
	// "{target}: skipped ({reason})"). It is narration → out ONLY; it never writes to
	// err.
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
	// does not special-case init for the footer; rather an init run opens Quiet, with
	// the header alone, and closes as OutcomeInitialised, which has no line.
	InitResult(r InitOutcome)
	// RunFinished closes the run and renders the end-of-run success line for the
	// outcomes that have one (see Outcome). Every run RunStarted opened closes with
	// exactly one, however it ends. A regenerate --all backfill also sends plain's
	// closing line Ahead, as its version loop finishes, even with every version
	// skipped, before the changelog rebuild that can still fail after it; that one
	// closes nothing, and its close, Quiet, follows the rebuild.
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

// Plan is the ShowPlan payload. Steps are plain's: it joins them into a
// "plan: …; …" one-liner. The worked-example abbreviations are illustrative wording
// the engine supplies in the step targets, not a distinct terse payload. An empty
// plan (no steps) is legal: plain renders exactly "plan:" with no dangling
// separator.
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
	// byte for byte from before the rail, has no line for it, so plain writes
	// nothing.
	Quiet bool
}

// PlanStage is one stage of the rail's plan: the Key stage events carry, the Name
// shown in its node and the Detail of what it will do ("main + v1.4.0 → origin,
// atomic").
type PlanStage struct {
	Key    string
	Name   string
	Detail string
	// Inside marks a stage that runs inside the next planned stage, which starts
	// first: the exclusion check, inside the notes. Plain ignores it.
	Inside bool
}

// Notes is the ShowNotes payload: the generated release notes the engine is
// presenting for review. Version labels the surrounding delimiters; Body is the
// notes content itself.
//
// THE NON-NEGOTIABLE INVARIANT: Body is shipped BYTE-FOR-BYTE VERBATIM — no
// stripping, no emoji removal, no case-folding, no re-wrapping, no truncation, and
// NO indentation added. Plain writes it flush between its delimiters through the
// shared writeNotesBody helper; the Rail shows it behind its gutter, wrapped to the
// width and with control characters made visible, for display only. Transforming
// the body would contradict the engine's "use the body whole" rule and break the
// "what previews is what ships" invariant. Emoji
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
// Title labels plain's surrounding delimiters and is engine-supplied, rendered
// VERBATIM (plain "--- {title} ---" … "--- end {title} ---"); the Rail draws no
// title. The presenter synthesises only the delimiter framing, never the title
// text. For the plain delimiters to stay byte-pure the engine supplies ASCII
// titles (the same convention as the gate Subject/AcceptEcho values).
//
// Body carries the SAME non-negotiable invariant as Notes.Body: plain writes it
// BYTE-FOR-BYTE VERBATIM through the shared writeNotesBody helper — no stripping,
// no re-wrapping, no truncation, no indentation — and the Rail's view of it is
// display only.
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

// Version is the ShowVersion payload: the engine-resolved version value, for the
// one payload verb (version's output is a value, not narration).
//
// Value is the resolved version (e.g. "1.4.0"), the load-bearing datum. Plain
// writes it as the BARE value (no "v" prefix, no glyph) so `$(mint version)`
// consumes it cleanly; the Rail dresses it as " mint v{Value}" — the "v" prefix is
// a PRETTY-only decoration, never part of the plain value.
type Version struct {
	// Value is the resolved version, rendered verbatim. Plain writes it bare; the
	// Rail prefixes a decorative "v".
	Value string
}

// RunInfo carries the start-of-run payload. Action is the engine-supplied verb
// word (e.g. "releasing", "regenerating") so the start-of-run line is
// verb-shaped from the payload rather than hardcoding any literal in the
// presenter.
//
// Header and Start carry the rail's opening: its header line and the station the
// run starts from. Plain reads neither.
type RunInfo struct {
	Project string
	Version string
	Action  string
	// DryRun marks the run as a --dry-run preview. When set, the start-of-run line
	// carries a "dry run" marker so the whole transcript is framed as a preview from
	// the first line — a dry run takes no action and (for release) shows no gates, so
	// the marker is the user's standing reminder that nothing will be changed.
	DryRun bool
	Header Header
	Start  Station
	// Toward is the name of the station the run sets out for, which the plan's end
	// waits at, where that is neither v{Version} nor the start's name: a --all
	// backfill's "done". Plain ignores it.
	Toward string
	// Quiet marks a start that only opens the rail's drawing where plain's log, pinned
	// byte for byte from before the rail, has no line for it: a regenerate's header
	// ahead of its questions, and its start station once they are answered, both
	// before plain's start line, which a regenerate --all prints per version. Plain
	// writes nothing.
	Quiet bool
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
// the Rail always shows progress. The presenter never infers Blocking from the
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
	// guideline — the Rail shows it dim beside the spinner. Empty falls back to the
	// plan stage's detail, then Name. Plain ignores it (its terse
	// "{name}: running..." start line stays byte-stable for pipes).
	Text string
	// Quiet marks a start that only shows a stage running again where plain's log,
	// pinned byte for byte from before the rail, has no line for it (an `r` at the
	// review writing the notes again in one call), so plain writes nothing.
	Quiet bool
}

// StageSuccess carries the StageSucceeded payload. Elapsed is measured by the
// engine — the presenter does not time stages. Both modes show it on
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
	// artifacts for v1.4.0"), empty when it made none. Plain ignores it.
	Commit string
	// Note is a line the rail hangs dim under the settled row, verbatim, for what the
	// stage passed over (a dry run's skipped preflight hook), empty for none. Plain
	// ignores it.
	Note string
	// Quiet marks a success that only settles a stage of the run's structure: plain's
	// log, pinned byte for byte from before the rail, has no line for it, so plain
	// writes nothing.
	Quiet bool
	// Detail is the engine-supplied completion detail. The empty string is legal
	// (semantic 3) and means "render the detail-less form".
	Detail string
	// Row is the settled row's detail where it reads differently from Detail, plain's
	// pinned wording (the push naming its branch), empty where they read the same.
	// Plain ignores it.
	Row string
	// Elapsed is the engine-measured stage duration. It is only meaningful when
	// Blocking is true (semantic 1); zero is a valid duration there (semantic 2),
	// not a "no elapsed" sentinel.
	Elapsed time.Duration
	// Blocking mirrors StageStart.Blocking for the same stage and gates whether a
	// renderer shows ({elapsed}).
	Blocking bool
	// Ongoing marks the success of a step its stage goes on past, plain's line for it:
	// a regenerate --all version's notes, which its review and its release follow, and
	// a changelog's push, which its node's own success follows. The rail draws the node
	// done, Row or Detail its detail, and keeps it open for its stage's later events.
	// Plain ignores it.
	Ongoing bool
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
// time of the check's call. Detail, on NotesChecked alone, is what the call found for
// its settled row: nothing to leave out, or how many it flagged, and whether they go
// unasked; empty for a call cancelled. Failed, on NotesChecked alone, marks a call
// that failed or whose answer was unusable: the warning that follows ends the check's
// node in place of a success. Plain ignores both.
type NotesCheck struct {
	Key     string
	Elapsed time.Duration
	Detail  string
	Failed  bool
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
	// zero for a short stage. Plain ignores it.
	Elapsed time.Duration
}

// NextMoves carries the NextMoves payload: the engine-authored ways forward from
// stage's failure, in order. Quiet marks moves plain's line already names (a failed
// unwind's manual steps, a failed publish's heal): plain writes nothing for them.
type NextMoves struct {
	Stage string
	Moves []NextMove
	Quiet bool
}

// NextMove is one way forward: a Lead saying what to do and, when the move ends in
// one, the Command or config line that does it, kept apart so the rail can set it on
// a line of its own. A move whose command sits mid-sentence keeps it in its Lead.
type NextMove struct {
	Lead    string
	Command string
	// Sentence is the move as plain's one sentence where that is not Lead then
	// Command (a config line the sentence has mid-way), empty where it
	// is. The rail never reads it.
	Sentence string
}

// Text is the move as one sentence, plain's line: its Sentence, or Lead then
// Command.
func (m NextMove) Text() string {
	if m.Sentence != "" {
		return m.Sentence
	}
	if m.Command == "" {
		return m.Lead
	}
	return m.Lead + " " + m.Command
}

// ExclusionSuggestions carries the ExclusionSuggestions payload: stage's suggestions,
// each rendered verbatim in order, their Lines set under Table of File. Plain never
// names File.
type ExclusionSuggestions struct {
	Stage       string
	Table       string
	File        string
	Suggestions []ExclusionSuggestion
}

// ExclusionAnswers carries the ExclusionAnswersRecorded payload: stage's answers went
// into Table of File, committed unless Ignored reports that git ignores File. Exclude
// and Keep are the globs answered each way, in the order answered; Key names the
// check's plan stage. Plain reads neither the globs nor Key.
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

// savedNotesLead leads the path of a failed chunked run's saved part notes.
const savedNotesLead = "part notes saved to"

func savedNotesText(path string) string {
	return savedNotesLead + " " + path
}

// Warning carries the Warn payload: a structured, engine-supplied Label and
// Message kept as SEPARATE fields. The presenter NEVER parses a label out of a
// single combined string — both fields arrive independently and both renderings
// are label-prefixed from them ("{label}: WARN - {message}" in plain, a ! node
// named {label} on the Rail).
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
	// ends no stage. Plain ignores it.
	Key string
	// Failed marks a warning about a step that failed once the run could no longer
	// stop on it, a recovery step of the unwind: the rail draws it as a failed node
	// of its own, named for its Key, which waits unsettled for the Unwound keyed as it
	// is, or the run's close. Plain ignores it.
	Failed bool
	// Skipped marks a warning that a stage of the plan did not run, a dry run's hook:
	// the rail draws nothing for it and leaves Key's node to come, its plan detail.
	// Plain ignores it.
	Skipped bool
	// Elapsed is the engine-measured time Key's stage ran before the warning ended it
	// (a regenerate --all version skipped), zero for none. Plain ignores it.
	Elapsed time.Duration
	// Row is the warning's row where it reads differently from Message, plain's
	// pinned line, which also names the remedy the engine sends the rail as Quiet
	// next moves; empty where they read the same. Plain ignores it.
	Row string
	// Output is optional captured underlying-command output (e.g. git's stderr
	// from a failed-but-non-fatal push) rendered VERBATIM beneath the warn line —
	// the warn-flavoured counterpart of StageFailure.Output, for failures the
	// engine deliberately keeps non-terminal. It is narration → out ONLY and is
	// NEVER duplicated to err: the stream contract's "one-line summary to stderr"
	// applies to the warn line alone. Rendering mirrors StageFailed's captured
	// body exactly — plain wraps it in the sliceable "--- output ---" delimiters
	// via the shared verbatim helper, the Rail hangs it behind its gutter. The
	// empty string (the common case) renders NO block; rendering it through Warn
	// does not set failure state.
	Output string
}

// Unwind carries the Unwound payload: what the engine's unwind undid after a failed
// or aborted run, what it kept, and where it left the repo, each engine-authored and
// kept apart so the rail can set them on rows of their own. Plain joins the
// non-empty parts with "; " into its one line (Text) and synthesises no part of it.
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
	// Key is the key of the unwind's node, the one the Failed warnings of its recovery
	// steps that failed carry: the rail draws an unwind that couldn't finish as those
	// failed nodes, not as a node claiming what it undid. Plain ignores it.
	Key string
	// Quiet marks an unwind plain's log, pinned byte for byte from before the rail, has
	// no line for: a regenerate's reset of its changelog commit. Plain writes
	// nothing.
	Quiet bool
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

// Outcome is how a run ended, the discriminator RunFinished renders by. Every run
// closes with one RunFinished carrying its Outcome; a regenerate --all backfill's
// line, OutcomeAllRegenerated, goes Ahead of its close, as its version loop finishes
// (see RunResult.Ahead). A loop a decline or a cancel ends early sends none Ahead.
// Plain writes its success lines for the outcomes that have one — released (with or
// without a warning), dry run, committed (with or without a warning), and --all
// regenerated — and nothing for the rest, which end on lines of their own (a
// failure, an unwind) or on none. The Rail closes every outcome at its end station.
//
// iota makes the zero value OutcomeReleased, so a RunResult literal that sets no
// Outcome closes as a release.
type Outcome int

const (
	// OutcomeReleased is a release that went out: "done: {project} v{X} {url}"
	// (plain), the URL omitted when empty.
	OutcomeReleased Outcome = iota
	// OutcomeReleasedWithWarning is a release that went out with a failure past the
	// point of no return (a publish or post_release hook failure), warned and never
	// unwound. It closes with the release line.
	OutcomeReleasedWithWarning
	// OutcomeDryRun is a dry run's close-out: "dry run: {project} v{X} — no changes
	// made". A dry run mutated nothing, so it never claims it released anything.
	OutcomeDryRun
	// OutcomeCommitted is a commit that landed, pushed or not: "done: {project}
	// committed" (plain), version-less and
	// URL-less.
	OutcomeCommitted
	// OutcomeCommittedWithWarning is a commit that landed and whose -p push failed,
	// warned and never unwound. It closes with the commit line.
	OutcomeCommittedWithWarning
	// OutcomeRegenerated is a single version regenerated. It has no line in plain.
	OutcomeRegenerated
	// OutcomeAllRegenerated is a --all backfill's close: "done: {project} {Summary}"
	// (plain), URL-less, the engine-computed Summary rendered verbatim.
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
	// OutcomeInitialised is an init that wrote or left alone each of its files. Its
	// lines are its files' rows, so it has no line of its own, and no end station.
	OutcomeInitialised
)

// RunResult carries the end-of-run payload. URL is optional — outcomes that do not
// publish a release leave it empty.
//
// End and Elapsed carry the rail's close; plain reads neither.
type RunResult struct {
	Project string
	Version string
	URL     string
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
	// Ahead marks plain's closing line of a regenerate --all backfill, which plain
	// writes as the version loop ends, ahead of the changelog rebuild: the run goes on
	// past it, so the rail draws nothing for it, and the run's close follows, Quiet.
	Ahead bool
	// Quiet marks a close plain's log has no line for, its line written Ahead. Plain
	// writes nothing.
	Quiet bool
}
