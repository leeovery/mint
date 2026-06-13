package presenter

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// PlainPresenter is the token-efficient, agent-friendly Presenter: terse
// lowercase "key: value" lines, no ANSI, no glyphs, no animation, and crucially
// no UI library — narration is built from plain fmt lines alone. It is the cheap
// render mode that proves the Presenter seam end-to-end and establishes the plain
// vocabulary the later phases extend.
type PlainPresenter struct {
	// out receives the narration stream (stdout in production).
	out io.Writer
	// err receives errors/warnings per the stream contract (stderr in
	// production). Only the one-line failure/warning summary is duplicated here,
	// for redirect-visibility — the full narration still goes to out. A clean run
	// writes nothing to err.
	err io.Writer
	// in is the gate INPUT stream (os.Stdin in production). It is injected so
	// Prompt is testable without a real terminal: tests pass a strings.Reader
	// script. It is wrapped ONCE in a persistent bufio.Reader (reader, below) so
	// buffered bytes survive across re-prompt reads.
	in io.Reader
	// reader is the single persistent buffered wrapper around in, constructed
	// lazily on the first Prompt read. Reusing one bufio.Reader for every read is
	// essential: a fresh wrapper per read would drop bytes bufio read ahead into
	// its buffer, losing the next line across a re-prompt.
	reader *bufio.Reader
	// yes records the -y/--yes gating decision (the gating axis — orthogonal to
	// render mode and the stream split). When set, Prompt SKIPS the gate entirely:
	// it neither renders the menu nor reads the input stream, instead emitting the
	// rendered auto-accept echo and returning the gate's declared default. The
	// production default is false (interactive); it is threaded via WithYes at the
	// converged startup seam (NewForStartup) from the -y decision the caller passes,
	// and by tests. Threading it as construction state (not a Prompt parameter)
	// keeps the Prompt(Gate) seam signature stable across both render modes.
	yes bool
	// stdinInteractive records whether stdin can host an interactive prompt — the
	// gating-INPUT axis (is stdin a TTY?), orthogonal to render mode (is stdout a
	// TTY?) and to -y. When false AND -y is absent, Prompt hits the
	// FORBIDDEN-COMBINATION rule: it fails loud (rendering a failure + returning
	// ErrNotInteractive) rather than blocking on a stdin read that never returns.
	//
	// The DEFAULT is true (interactive) — set explicitly in the constructor's struct
	// literal, NOT left to the bool zero value. That matters: the existing
	// interactive-path tests construct presenters with yes=false and a scripted
	// reader and must keep hitting the interactive loop, not the new fail path, so
	// the safe default is "interactive". Production threads the detected stdin signal
	// (DetectStartupSignals' StdinInteractive, from the stdin descriptor) at the
	// converged startup seam (NewForStartup) via WithInteractiveStdin — the same
	// place the -y decision is threaded.
	stdinInteractive bool
	// terminalFailure records that the run has hit a terminal failure or abort —
	// set by StageFailed (a failed stage) and by Unwound (a failure or gate-n
	// abort). It makes the presenter STATEFUL per run: when set, RunFinished
	// suppresses the success end-of-run line, which is SUCCESS-ONLY. There is no
	// failure-flavoured closing line — failure/abort is signalled by the
	// FAILED/unwound lines plus the engine-owned non-zero exit code. Warn does NOT
	// set this flag (a warn-only run still ends with the success line). One
	// presenter instance is constructed per run via NewPlainPresenter, so this
	// per-run state is sound; tests construct a fresh presenter per scenario.
	terminalFailure bool
}

// Compile-time proof the plain presenter satisfies the seam it renders.
var _ Presenter = (*PlainPresenter)(nil)

// NewPlainPresenter constructs a PlainPresenter writing narration to out and the
// one-line failure/warning summary additionally to err (stdout/stderr in
// production). The split is fixed regardless of render mode. Gate input defaults
// to os.Stdin (the production default); tests inject a scripted reader via
// NewPlainPresenterWithInput.
func NewPlainPresenter(out, err io.Writer) *PlainPresenter {
	return NewPlainPresenterWithInput(out, err, os.Stdin)
}

// NewPlainPresenterWithInput is the test seam for the gate input axis: it is
// NewPlainPresenter with the input reader injected, so Prompt can be driven from a
// scripted strings.Reader without a real terminal. Production uses
// NewPlainPresenter, which defaults in to os.Stdin.
func NewPlainPresenterWithInput(out, err io.Writer, in io.Reader) *PlainPresenter {
	// stdinInteractive defaults to true (interactive) explicitly — see the field
	// doc: the existing interactive-path tests must keep hitting the interactive
	// loop, not the forbidden-combination fail path.
	return &PlainPresenter{out: out, err: err, in: in, stdinInteractive: true}
}

// writef writes one narration line to out. A Presenter method returns nothing —
// the engine narrates fire-and-forget — so a write error to the output stream has
// nowhere to propagate and is deliberately discarded here, in one place, rather
// than ignored ad hoc at every call site.
func (p *PlainPresenter) writef(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format, args...)
}

// errf writes one line to the err stream (stderr in production). Per the stream
// contract only the one-line failure/warning summary is duplicated here for
// redirect-visibility; the multi-line captured body never reaches err. As with
// writef, the write error is discarded — the engine narrates fire-and-forget and
// a failed write to err has nowhere to propagate.
func (p *PlainPresenter) errf(format string, args ...any) {
	_, _ = fmt.Fprintf(p.err, format, args...)
}

// RunStarted renders the start-of-run line: "mint: {action} {project} v{X}". The
// action word is engine-supplied (RunInfo.Action) so the line is verb-shaped —
// "releasing", "regenerating", … — never a hardcoded literal. The bare payload
// version is rendered with a "v" prefix; a version-LESS run (commit has no version
// to announce) omits the segment entirely rather than dangling a bare "v" — the
// same no-dangling-segment rule the success/footer lines follow.
func (p *PlainPresenter) RunStarted(info RunInfo) {
	if info.Version == "" {
		p.writef("mint: %s %s\n", info.Action, info.Project)
		return
	}
	p.writef("mint: %s %s v%s\n", info.Action, info.Project, info.Version)
}

// StageStarted emits plain's spinner-equivalent: a terse start line for a
// blocking (long/slow) stage only, so a live-tail consumer isn't staring at
// silence through a multi-second wait. Short stages stay silent until completion.
// The start word is a STAGE-AGNOSTIC synthesised verb — "running..." — correct for
// every named blocking stage (notes generation AND the pre_tag build hook) rather
// than a stage-specific guess: the StageStart payload carries only Name + Blocking,
// so the presenter must not invent stage-specific narration (the event-payload
// principle). It is the plain equivalent of the pretty spinner. Being synthesised,
// it stays byte-pure ASCII — an ASCII ellipsis ("..."), not the U+2026 glyph the
// pretty spinner uses (the spec's "wording refinable" latitude; the byte-purity
// guard is fixed).
func (p *PlainPresenter) StageStarted(s StageStart) {
	if !s.Blocking {
		return
	}
	p.writef("%s: running...\n", s.Name)
}

// SuspendSpinner and ResumeSpinner are NO-OPS in plain mode — plain never animates
// (a stage emits exactly one terse line on its transition), so there is nothing to
// suspend or resume around the engine's $EDITOR hand-off. They produce NO output and
// never error, satisfying the engine-callable interface uniformly across both render
// modes without plain pulling in any animation. The presenter does not invoke
// $EDITOR; these hooks only exist for the pretty spinner's sake.
func (p *PlainPresenter) SuspendSpinner() {}

// ResumeSpinner is the plain no-op pair of SuspendSpinner (see above).
func (p *PlainPresenter) ResumeSpinner() {}

// StageSucceeded renders a stage's single completion line as "{stage}: {detail}",
// falling back to "{stage}: ok" when no detail travels with the event. A blocking
// stage additionally carries an elapsed suffix " ({elapsed})" after the
// detail/ok text — the same long/blocking gate the pretty presenter uses. The
// suffix is gated on Blocking, not on the Elapsed value: a short stage never shows
// elapsed even with a non-zero duration, and a blocking stage that finished under
// the timer's resolution still renders "(0.0s)" rather than suppressing it.
func (p *PlainPresenter) StageSucceeded(s StageSuccess) {
	detail := s.Detail
	if detail == "" {
		detail = "ok"
	}
	if s.Blocking {
		// formatElapsed (in pretty.go) is the package-shared "{seconds}s" helper —
		// same compact one-decimal form both presenters render. The suffix is gated
		// on Blocking, not the Elapsed value, so a zero duration still renders
		// "(0.0s)" rather than being suppressed.
		p.writef("%s: %s (%s)\n", s.Name, detail, formatElapsed(s.Elapsed))
		return
	}
	p.writef("%s: %s\n", s.Name, detail)
}

// StageFailed renders the one-line failure summary "{stage}: FAILED - {message}"
// to out (the narration) AND duplicates that same one-line summary to err so a
// failure cannot silently vanish under redirection.
//
// When the engine captured underlying-command output (s.Output non-empty), the
// captured body is rendered to OUT ONLY, below the FAILED line, wrapped in the
// sliceable "--- output ---" … "--- end output ---" delimiters — mirroring the
// notes block so a reader/agent can extract it. The body is written through the
// package-shared writeNotesBody helper UNCHANGED — byte-for-byte verbatim — so a
// body line that itself reads like a delimiter is written through as-is; the real
// closing delimiter still follows (delimiters are positional, never
// content-matched). An empty Output renders NO delimiter block — the FAILED line
// stands alone.
//
// The captured body is narration → out only and is NEVER duplicated to err: only
// the one-line summary goes there. The synthesised delimiter lines are byte-pure
// ASCII; the body may legitimately contain non-ASCII engine content, which is
// rendered verbatim (the plain byte-purity guard scans synthesised narration, not
// this engine-supplied body).
func (p *PlainPresenter) StageFailed(s StageFailure) {
	p.terminalFailure = true
	p.writef("%s: FAILED - %s\n", s.Name, s.Message)
	p.errf("%s: FAILED - %s\n", s.Name, s.Message)
	if s.Output == "" {
		return
	}
	p.writef("--- output ---\n")
	writeNotesBody(p.out, s.Output)
	p.writef("--- end output ---\n")
}

// Unwound renders the auto-unwind line "unwound: {summary}" to OUT ONLY — the
// summary is the engine's verbatim "what it undid" text, rendered as-is INCLUDING
// its own "; repo clean" tail (the presenter synthesises no tail of its own). Per
// the per-event stream table the auto-unwind line is narration only and is NOT
// duplicated to err, unlike the FAILED/WARN summaries.
//
// Unwound marks the run terminal (setting terminalFailure) so a subsequent
// RunFinished suppresses the success end-of-run line — covering BOTH the failure
// path (StageFailed → Unwound) and the abort path (gate-n: Unwound with no prior
// StageFailed). The synthesised "unwound: " prefix is byte-pure ASCII; the summary
// is engine content rendered verbatim (the byte-purity guard scans the synthesised
// prefix, not the engine summary).
func (p *PlainPresenter) Unwound(u Unwind) {
	p.terminalFailure = true
	p.writef("unwound: %s\n", u.Summary)
}

// Warn renders the structured warning as "{label}: WARN - {message}" to out (the
// narration) AND duplicates that same one-line summary to err so a warning is
// visible under redirection — mirroring StageFailed's dual-write. Label and message
// are separate engine-supplied fields rendered verbatim; the presenter never parses
// a label out of a combined string.
//
// Warn is independent of run state: it does not set failure and does not suppress
// the success end-of-run line (that suppression is owned elsewhere). Multiple Warn
// calls each render their own line, in order — there is no collapsing.
//
// Empty-message edge: the fixed "{label}: WARN - " prefix renders with nothing
// after it — no invented message text. The line is synthesised byte-pure ASCII (no
// ⚠ glyph; that is pretty-only).
//
// When the warning carries captured underlying-command output (w.Output
// non-empty), the captured body is rendered to OUT ONLY, below the WARN line,
// wrapped in the same sliceable "--- output ---" … "--- end output ---"
// delimiters StageFailed uses, with the body written through the package-shared
// writeNotesBody helper UNCHANGED — byte-for-byte verbatim, delimiters positional
// (never content-matched). The body is NEVER duplicated to err: only the one-line
// WARN summary goes there. An empty Output renders NO block, and rendering the
// block does not set failure state — the warn stays non-terminal.
func (p *PlainPresenter) Warn(w Warning) {
	p.writef("%s: WARN - %s\n", w.Label, w.Message)
	p.errf("%s: WARN - %s\n", w.Label, w.Message)
	if w.Output == "" {
		return
	}
	p.writef("--- output ---\n")
	writeNotesBody(p.out, w.Output)
	p.writef("--- end output ---\n")
}

// ShowPlan renders the plan as a single terse one-liner: "plan: {step}; {step}; …"
// where each step is rendered "{verb} {target}" (or just "{verb}" when the target
// is empty) and the steps are joined by "; ". It derives entirely from the SAME
// structured steps the pretty block does — there is no separate terse field.
//
// Edge forms, all synthesised byte-pure ASCII (the targets are engine-supplied and
// rendered verbatim): an empty plan renders exactly "plan:" with no trailing space
// and no separator; an empty-target step contributes just its verb with no trailing
// space; a single step has no "; " separator at all.
func (p *PlainPresenter) ShowPlan(plan Plan) {
	if len(plan.Steps) == 0 {
		p.writef("plan:\n")
		return
	}
	rendered := make([]string, len(plan.Steps))
	for i, step := range plan.Steps {
		rendered[i] = renderPlainStep(step)
	}
	p.writef("plan: %s\n", strings.Join(rendered, "; "))
}

// renderPlainStep renders one step as "{verb} {target}", collapsing to just
// "{verb}" when the target is empty so no trailing space dangles.
func renderPlainStep(step PlanStep) string {
	if step.Target == "" {
		return step.Verb
	}
	return step.Verb + " " + step.Target
}

// ShowNotes renders the release notes wrapped in the sliceable plain rules
// "--- release notes v{X} ---" … "--- end notes ---" so a reader/agent can
// extract the block reliably. The body is written through the package-shared
// writeNotesBody helper UNCHANGED — byte-for-byte verbatim, the same bytes the
// pretty presenter writes — so the body region is provably identical across
// modes; only these delimiters differ.
//
// The synthesised delimiter lines are byte-pure ASCII. The body itself may
// legitimately contain emoji (e.g. ✨ Features / 🐛 Fixes) — that is engine
// content rendered verbatim, NOT synthesised narration, so the plain byte-purity
// guard (which scans only synthesised stage narration) does not apply to it.
//
// Edge forms: an empty body writes nothing between the rules, so the opener line
// is immediately followed by the closer line — no spurious blank line. A body
// line that itself reads like a delimiter is written verbatim; the real closing
// delimiter still follows it (delimiters are positional, never content-matched).
func (p *PlainPresenter) ShowNotes(notes Notes) {
	p.writef("--- release notes v%s ---\n", notes.Version)
	writeNotesBody(p.out, notes.Body)
	p.writef("--- end notes ---\n")
}

// ShowMessage renders an engine-titled content block wrapped in the sliceable
// plain delimiters "--- {title} ---" … "--- end {title} ---" — the
// general-purpose sibling of ShowNotes (whose delimiters hardcode the
// release-notes framing). The title is engine content rendered VERBATIM in both
// delimiter lines (ASCII by the engine's convention so the synthesised lines stay
// byte-pure); the body is written through the package-shared writeNotesBody
// helper UNCHANGED — byte-for-byte verbatim, the same bytes the pretty presenter
// writes — so the body region is provably identical across modes.
//
// Edge forms mirror ShowNotes exactly: an empty body writes nothing between the
// delimiters; a body line that itself reads like a delimiter is written verbatim
// with the real closer still following (delimiters are positional, never
// content-matched). Narration → out only, never err.
func (p *PlainPresenter) ShowMessage(m Message) {
	p.writef("--- %s ---\n", m.Title)
	writeNotesBody(p.out, m.Body)
	p.writef("--- end %s ---\n", m.Title)
}

// ShowVersion writes the bare version value plus a single trailing newline to OUT
// ONLY — "{value}\n" and NOTHING else. This is the deliberate PAYLOAD EXCEPTION to
// plain's "key: value" narration: version's output is a VALUE, not narration, so it
// carries NO "version:" prefix, NO "v" prefix, NO glyph, NO ANSI, and no second
// line. That exact framing is the load-bearing contract for `$(mint version)` —
// command substitution strips the single trailing newline, leaving exactly the
// value. The value is passed through %s (never interpreted as a format string) and
// is engine content rendered verbatim; the only framing this synthesises is the one
// trailing newline. version has no gate and no release footer — this line is the
// terminal output, narration → out only (never err).
func (p *PlainPresenter) ShowVersion(v Version) {
	p.writef("%s\n", v.Value)
}

// Prompt drives the shared line-read input loop for the plain gate: it renders a
// terse prompt, reads ONE line, and returns a declared Choice. Empty Enter (and, per
// parseChoice, a whitespace-only line that trims to empty) selects the gate's
// Default; case-insensitive input maps to a declared key; unrecognised input
// re-prompts; EOF returns a non-nil error rather than silently default-accepting.
// The parse/loop core is shared with the pretty presenter (readChoice/parseChoice)
// — only the render closure differs.
//
// The plain render is a single terse line "{Question} [y/n/e/r]" with the hint
// built from the gate's DECLARED keys (not a hardcoded set), so the two-choice
// reuse gate renders "[y/n]". It is byte-pure ASCII — the pretty vertical menu is
// task 3-4 and is not built here.
//
// Under -y the gate is SKIPPED (not drawn-then-auto-pressed): the menu is not
// rendered and the input stream is NOT read at all. Instead the auto-accept is
// communicated as a RENDERED event — the byte-pure ASCII echo "{Subject}:
// {AcceptEcho} (-y)" to OUT only (narration, never an err copy) — and the gate's
// declared default is returned with a nil error. Both the Subject AND the echo word
// (AcceptEcho — "accepted" for notes, the chosen value for source/target) travel in
// the gate payload, so neither is hardcoded here.
func (p *PlainPresenter) Prompt(gate Gate) (Choice, error) {
	if p.yes {
		p.writef("%s: %s (-y)\n", gate.Subject, gate.AcceptEcho)
		return gate.Default, nil
	}
	if !p.stdinInteractive {
		p.failNotInteractive(gateFailLabel)
		return "", ErrNotInteractive
	}
	reader := bufferedReader(p.in, &p.reader)
	render := func() {
		p.writef("%s [%s]\n", gate.Question, plainKeyHint(gate))
	}
	return readChoice(reader, render, gate)
}

// AskLine renders the terse free-text prompt "{prompt}: " (no trailing newline —
// the cursor sits after the colon for the line-read) and returns ONE raw line via
// the shared readLine core, stripped of its line terminator but otherwise
// VERBATIM — the empty string is a legal answer and whitespace is preserved (the
// engine owns interpretation). It reads through the SAME persistent buffered
// reader Prompt uses, so a Prompt followed by an AskLine consumes consecutive
// lines of one stream. The synthesised framing (": ") is byte-pure ASCII; the
// prompt label is engine content rendered verbatim.
//
// A non-interactive stdin fails loud BEFORE any render or read — the same
// forbidden-combination rule as Prompt, labelled "input" (the free-text input
// mechanism, not a gate) — and returns ErrNotInteractive. -y does not bypass the
// check: free text has no declared default to auto-accept, and engine flows only
// reach AskLine from an interactive gate choice. EOF with no usable line returns
// ErrInputClosed, unrendered (the engine owns that failure's surfacing).
func (p *PlainPresenter) AskLine(prompt string) (string, error) {
	if !p.stdinInteractive {
		p.failNotInteractive(inputFailLabel)
		return "", ErrNotInteractive
	}
	p.writef("%s: ", prompt)
	return readLine(bufferedReader(p.in, &p.reader))
}

// failNotInteractive renders the FORBIDDEN-COMBINATION failure (non-TTY stdin
// without -y) WITHOUT touching the input stream — the whole point is to never
// block on stdin that will not deliver. It reuses the established plain FAILED
// vocabulary "{label}: FAILED - {message}": the one-line summary goes to OUT (the
// narration) AND is duplicated to ERR per the stream contract, exactly like
// StageFailed. The label names the failing MECHANISM — gateFailLabel ("gate") from
// Prompt, inputFailLabel ("input") from AskLine — never the gate's Subject (the
// notes content): a reader sees the mechanism failed, not that "notes" failed.
// The message is the byte-pure ASCII gateNotTTYMessageASCII (a semicolon, never
// the em-dash, so the plain byte-purity guard stays green). The caller then
// returns the exported ErrNotInteractive sentinel; the presenter sets no exit
// code.
func (p *PlainPresenter) failNotInteractive(label string) {
	p.writef("%s: FAILED - %s\n", label, gateNotTTYMessageASCII)
	p.errf("%s: FAILED - %s\n", label, gateNotTTYMessageASCII)
}

// InitResult renders one init outcome to OUT ONLY in plain's "{target}: {action}"
// vocabulary: "{target}: created" for InitCreated, "{target}: skipped ({reason})"
// for InitSkipped. The action word follows the target (the plain key:value form),
// which is the reverse of pretty's "{glyph} {action-word} {target}" word order — the
// spec fixes the order per mode. The engine-supplied Reason is rendered VERBATIM; the
// presenter synthesises no reason text and reads Reason only for a skip.
//
// init has no gate and no release-style footer — these created/skipped lines ARE the
// terminal output. InitResult is narration → out only and is never duplicated to err
// (init carries no failure/warning semantics). The synthesised parts ("{target}: ",
// "created", "skipped (", ")") are byte-pure ASCII — the pretty "·" middot and "✓"
// glyph are PRETTY-only; the target and reason are engine content rendered verbatim.
func (p *PlainPresenter) InitResult(r InitOutcome) {
	if r.Action == InitSkipped {
		p.writef("%s: skipped (%s)\n", r.Target, r.Reason)
		return
	}
	p.writef("%s: created\n", r.Target)
}

// RunFinished renders the success-shaped, VERB-shaped end-of-run line via an
// EXHAUSTIVE dispatch on r.Verb — gated FIRST by the success-suppression flag.
//
// SUPPRESSION PRECEDES SHAPING. The end-of-run line is SUCCESS-ONLY: when the run
// has hit a terminal failure or abort (terminalFailure set by StageFailed or
// Unwound) this emits NOTHING — there is no failure-flavoured "done:" line, even
// for a release that still carries a URL. The run has already ended after the
// FAILED/unwound lines; failure is signalled by those lines plus the engine-owned
// non-zero exit code. The presenter never sets the exit code. The suppression
// check runs BEFORE the verb switch, so it covers EVERY shape — release,
// regenerate, init, and version alike. Warn does NOT set terminalFailure, so a
// warn-only run still emits the success footer.
//
// Verb dispatch (the presenter never re-derives the verb — the shape comes from
// the payload):
//
//   - VerbRelease (the iota-0 default, so a Verb-less literal lands here): the
//     release-success footer "done: {project} v{X} {url}". URL is optional — when
//     empty the line collapses to "done: {project} v{X}" with no dangling trailing
//     space.
//   - VerbRegenerate: regenerate publishes no release and has NO URL, so the close
//     is the URL-less "done: {project} {Summary}" — the {url} field is omitted
//     ENTIRELY (not rendered empty, no dangling separator). The engine-supplied
//     Summary is the single version or, under --all, the set/range/count text,
//     rendered VERBATIM; the presenter never computes the version set. The --all
//     single-version case still lands here (Verb=VerbRegenerate), so it renders the
//     set summary, not a release-style v{X}+url footer.
//   - VerbCommit: version-less and URL-less — "done: {project} committed". A commit
//     publishes no release and announces no version, so neither segment renders.
//   - VerbInit, VerbVersion: NO footer — init's created/skipped lines and version's
//     value line are themselves the terminal output. These arms render NOTHING
//     (defensive completeness; in practice the engine does not call RunFinished for
//     init/version).
func (p *PlainPresenter) RunFinished(r RunResult) {
	if p.terminalFailure {
		return
	}
	if r.DryRun {
		// A dry run changed nothing — never claim a release/commit happened.
		p.writef("dry run: %s v%s — no changes made\n", r.Project, r.Version)
		return
	}
	switch r.Verb {
	case VerbRelease:
		p.renderReleaseFooter(r)
	case VerbRegenerate:
		p.writef("done: %s %s\n", r.Project, r.Summary)
	case VerbCommit:
		// Version-less and URL-less: the commit IS the success, so the close-out
		// carries only the verb word and the project.
		p.writef("done: %s committed\n", r.Project)
	case VerbInit, VerbVersion:
		// No release-style footer: these verbs' own lines are the terminal output.
	}
}

// renderReleaseFooter writes the release-success footer "done: {project} v{X}
// {url}", omitting the trailing " {url}" cleanly when URL is empty so the line
// never dangles a trailing space.
func (p *PlainPresenter) renderReleaseFooter(r RunResult) {
	if r.URL == "" {
		p.writef("done: %s v%s\n", r.Project, r.Version)
		return
	}
	p.writef("done: %s v%s %s\n", r.Project, r.Version, r.URL)
}
