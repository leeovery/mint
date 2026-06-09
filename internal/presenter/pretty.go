package presenter

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// defaultLeaf is the fallback brand glyph. The engine supplies the leaf in the
// run payload (it ties to the engine's commit_prefix brand); when that datum is
// empty the presenter falls back to mint's own leaf rather than hardcoding the
// brand at every render site.
const defaultLeaf = "🌿"

// stageColumn is the width the stage name is padded to so the detail column lines
// up across stages — the "padded to a column" rule from the spec's stage-line
// description. It is the longest worked-example name ("preflight", 9) plus a
// two-space gap, so "preflight" gets its two trailing spaces and shorter names
// ("version") pad out to the same detail column. Names at or beyond the column
// still get a single separating space so glyph/name/detail never collide.
const stageColumn = 11

// stageIndent is the two-space indent every non-brand line carries. Brand lines
// are flush-left; everything else indents under them.
const stageIndent = "  "

// decorativeRuleWidth is the fixed cap width the notes rules render to. The spec
// caps decorative rules at min(terminalWidth, ~50) so they cannot overflow and
// wrap into junk; this task renders at the fixed cap and does NOT detect terminal
// width. Phase 4 (task cli-presentation-4-7) hardens the width SOURCE to
// min(terminalWidth, cap) and reuses THIS constant as the cap — hence the named
// constant rather than a magic number.
const decorativeRuleWidth = 50

// ruleChar is the box-drawing horizontal line (U+2500) the decorative notes rules
// are built from. It is layout, not colour — it survives a colour downgrade.
const ruleChar = "─"

// PrettyPresenter is the styled, human-facing Presenter: brand lines, colour, and
// status glyphs rendered as linear print-style narration (no alt-screen, no event
// loop). All colour and styling go through lipgloss, which auto-downgrades colour
// when the output is piped or the terminal is colour-incapable — the presenter
// itself performs no NO_COLOR/TERM sniffing.
//
// Stage progress is a SPINNER (pretty-only): a blocking StageStarted starts a
// single spinner on the current stage line; the spinner is replaced in place by the
// ✓/✗ completion line on StageSucceeded/StageFailed. Short (non-blocking) stages
// emit no start line — only their completion line — consistent with plain and the
// spec's worked example. The spinner is the briandowns standalone library (NOT
// Bubble Tea; print-style, no alt-screen), reached only through the injectable
// newSpinner seam so its timed goroutine never makes lifecycle tests flaky.
type PrettyPresenter struct {
	// out receives the narration stream (stdout in production).
	out io.Writer
	// err receives the one-line failure/warning summary per the stream contract
	// (stderr in production), for redirect-visibility. The summary written here is
	// plain (unstyled) text: stderr is a redirect-visibility channel, not a styled
	// surface, so it carries no ANSI — keeping it simple and consistent with plain
	// mode. A clean run writes nothing to err.
	err io.Writer
	// renderer is the lipgloss renderer bound to out. Injecting it (rather than
	// using the package default) is the test seam: production binds it to out so
	// lipgloss auto-detects the terminal's colour profile, while tests force a
	// profile for deterministic colour-on / colour-off assertions.
	renderer *lipgloss.Renderer

	// in is the gate INPUT stream (os.Stdin in production), injected so Prompt is
	// testable without a real terminal — the same input axis as the plain
	// presenter. It is wrapped ONCE in the persistent reader below.
	in io.Reader
	// reader is the single persistent buffered wrapper around in, constructed
	// lazily on the first Prompt read so bytes bufio reads ahead survive across a
	// re-prompt (a fresh wrapper per read would drop them).
	reader *bufio.Reader
	// yes records the -y/--yes gating decision (the gating axis — orthogonal to
	// render mode and the stream split). When set, Prompt SKIPS the gate entirely:
	// it neither renders the vertical menu nor reads the input stream, instead
	// emitting the rendered accept line and returning the gate's declared default.
	// The production default is false (interactive); it is set via WithYes at the
	// one site the -y flag is parsed (a later main/cmd task) and by tests. Threading
	// it as construction state (not a Prompt parameter) keeps the Prompt(Gate) seam
	// signature stable across both render modes.
	yes bool
	// stdinInteractive records whether stdin can host an interactive prompt — the
	// gating-INPUT axis (is stdin a TTY?), orthogonal to render mode (is stdout a
	// TTY?) and to -y. When false AND -y is absent, Prompt hits the
	// FORBIDDEN-COMBINATION rule: it fails loud (rendering a styled ✗ failure +
	// returning ErrNotInteractive) rather than blocking on a stdin read that never
	// returns. The render stays STYLED because render mode is selected from stdout,
	// independently of the non-TTY stdin that triggered the failure.
	//
	// The DEFAULT is true (interactive) — set explicitly in newPrettyPresenter's
	// struct literal, NOT left to the bool zero value — so the existing
	// interactive-path tests (yes=false, scripted reader) keep hitting the
	// interactive loop, not the fail path. Production sets it from
	// DetectStdinTTY(os.Stdin) at the same one site the -y flag is parsed (a later
	// main/cmd task) via WithInteractiveStdin — the same deferral as -y.
	stdinInteractive bool

	// Styles are derived once from the renderer so every render shares the same
	// colour profile. Under a no-colour profile lipgloss renders these as the bare
	// text, preserving layout and glyphs without emitting any ANSI escape.
	success lipgloss.Style
	failure lipgloss.Style
	warn    lipgloss.Style
	dim     lipgloss.Style
	// unwound styles the ↩ auto-unwind glyph. The spec assigns ↩ no specific
	// colour, so it is styled DIM (bright-black) — a subdued, recovery-flavoured
	// tone consistent with the other secondary narration (StageStarted, the Plan
	// header, the notes rules are all dim) and deliberately not competing with the
	// load-bearing ✓/✗/⚠ status colours. Like every style here it is layout-safe:
	// the indent, glyph, and column padding survive a colour downgrade as bare text.
	unwound lipgloss.Style

	// terminalFailure records that the run has hit a terminal failure or abort —
	// set by StageFailed (a failed stage) and by Unwound (a failure or gate-n
	// abort). It makes the presenter STATEFUL per run: when set, RunFinished
	// suppresses the success bottom brand line, which is SUCCESS-ONLY. There is no
	// failure-flavoured closing brand line — failure/abort is signalled by the
	// ✗/unwound lines plus the engine-owned non-zero exit code. Warn does NOT set
	// this flag (a warn-only run still ends with the success brand line). One
	// presenter instance is constructed per run, so this per-run state is sound;
	// tests construct a fresh presenter per scenario.
	terminalFailure bool

	// newSpinner builds the stage-progress spinner that animates a blocking stage's
	// line. It is the INJECTION SEAM that keeps the spinner lifecycle testable: the
	// constructors default it to the real briandowns wrapper (newBriandownsSpinner),
	// while a test injects a spy via WithSpinnerFactory so Start/Stop can be asserted
	// without the real library's timed goroutine and frame output. It is never nil
	// after construction.
	newSpinner spinnerFactory
	// activeSpinner is the SINGLE spinner currently animating, or nil when none is
	// running — the "one spinner at a time" invariant lives here. A blocking
	// StageStarted defensively stops any lingering spinner before starting (and
	// storing) a new one; StageSucceeded/StageFailed stop it and reset this to nil
	// before rendering the completion line in the cleared place. Held as construction
	// state on the per-run presenter instance (mirroring terminalFailure), so there is
	// no shared mutable global.
	activeSpinner StageSpinner
	// activeSpinnerText is the dim start text the active spinner was created with —
	// remembered so SuspendSpinner can carry it over to suspendedText and ResumeSpinner
	// can recreate the spinner on the SAME stage line. It is set wherever a spinner is
	// created (StageStarted and ResumeSpinner) and is only meaningful while
	// activeSpinner is non-nil.
	activeSpinnerText string

	// spinnerSuspended records that SuspendSpinner stopped the active spinner around
	// the engine's $EDITOR hand-off and is awaiting a ResumeSpinner. It is the flag
	// that lets ResumeSpinner know whether to recreate a spinner: set true only when a
	// spinner was actually running at suspend time (a suspend with no active spinner
	// is a safe no-op that leaves this false). It is CLEARED both by ResumeSpinner
	// (after recreating the spinner) and by StageSucceeded/StageFailed via stopSpinner
	// (so a stage that completes WHILE suspended is not resurrected by a later
	// ResumeSpinner). Per-run construction state, like activeSpinner.
	spinnerSuspended bool
	// suspendedText is the dim start text of the spinner SuspendSpinner stopped,
	// remembered so ResumeSpinner recreates the spinner on the SAME stage line with
	// the identical text. It is only meaningful while spinnerSuspended is true.
	suspendedText string
}

// Compile-time proof the pretty presenter satisfies the seam it renders.
var _ Presenter = (*PrettyPresenter)(nil)

// NewPrettyPresenter constructs a PrettyPresenter writing narration to out. The
// renderer is bound to out so lipgloss auto-detects (and auto-downgrades) the
// terminal's colour capabilities — the production path needs no explicit profile.
// The err writer is accepted now to keep the constructor signature stable across
// phases; this task narrates only to out. Gate input defaults to os.Stdin (the
// production default); tests inject a scripted reader via
// NewPrettyPresenterWithInput.
func NewPrettyPresenter(out, err io.Writer) *PrettyPresenter {
	return newPrettyPresenter(out, err, os.Stdin, lipgloss.NewRenderer(out))
}

// NewPrettyPresenterWithProfile constructs a PrettyPresenter whose renderer is
// forced to the given colour profile, with no err writer wired. It is the test
// seam for out-only assertions: tests pass termenv.TrueColor/ANSI to assert ANSI
// codes are emitted, or termenv.Ascii to assert the colour auto-downgrade emits
// none while layout and glyphs survive. Use NewPrettyPresenterWithErr when the
// stderr split itself is under test. Gate input defaults to os.Stdin; use
// NewPrettyPresenterWithInput to script Prompt.
func NewPrettyPresenterWithProfile(out io.Writer, profile termenv.Profile) *PrettyPresenter {
	return NewPrettyPresenterWithErr(out, nil, profile)
}

// NewPrettyPresenterWithErr constructs a PrettyPresenter whose renderer is forced
// to the given colour profile AND whose err writer is wired. It is the test seam
// for the stream-split contract: forcing colour on out while capturing err proves
// the stderr summary stays unstyled by design — not merely because lipgloss
// auto-downgrades on a non-TTY buffer. Gate input defaults to os.Stdin.
func NewPrettyPresenterWithErr(out, err io.Writer, profile termenv.Profile) *PrettyPresenter {
	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(profile)
	return newPrettyPresenter(out, err, os.Stdin, renderer)
}

// NewPrettyPresenterWithInput is the test seam for the gate input axis: it forces
// the colour profile (like NewPrettyPresenterWithProfile, out-only) AND injects
// the input reader so Prompt can be driven from a scripted strings.Reader without
// a real terminal. Production uses NewPrettyPresenter, which defaults in to
// os.Stdin.
func NewPrettyPresenterWithInput(out io.Writer, profile termenv.Profile, in io.Reader) *PrettyPresenter {
	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(profile)
	return newPrettyPresenter(out, nil, in, renderer)
}

// newPrettyPresenter is the shared constructor core: it derives the styles from
// the supplied renderer so colour-on and colour-off paths build identically and
// differ only in the renderer's profile.
func newPrettyPresenter(out, err io.Writer, in io.Reader, renderer *lipgloss.Renderer) *PrettyPresenter {
	return &PrettyPresenter{
		out:      out,
		err:      err,
		in:       in,
		renderer: renderer,
		// stdinInteractive defaults to true (interactive) explicitly — see the field
		// doc: the existing interactive-path tests must keep hitting the interactive
		// loop, not the forbidden-combination fail path.
		stdinInteractive: true,
		success:          renderer.NewStyle().Foreground(lipgloss.Color("2")),   // green
		failure:          renderer.NewStyle().Foreground(lipgloss.Color("1")),   // red
		warn:             renderer.NewStyle().Foreground(lipgloss.Color("214")), // amber / orange
		dim:              renderer.NewStyle().Foreground(lipgloss.Color("8")),   // bright black / dim
		unwound:          renderer.NewStyle().Foreground(lipgloss.Color("8")),   // ↩ glyph — dim (no spec colour; subdued recovery tone)
		// Default to the real briandowns spinner factory; a test overrides it via
		// WithSpinnerFactory. Never left nil so StageStarted can call it unconditionally.
		newSpinner: newBriandownsSpinner,
	}
}

// WithYes sets the -y/--yes gating decision and returns the presenter so it chains
// onto any constructor (e.g. NewPrettyPresenterWithInput(...).WithYes(true)). It is
// a builder-style setter — kept off the constructors so their signatures stay
// stable and so task 3-6's stdin-interactive gating signal can be added the same
// way without a constructor explosion. Production sets it where the -y flag is
// parsed; the zero value (no call) is the interactive default.
func (p *PrettyPresenter) WithYes(yes bool) *PrettyPresenter {
	p.yes = yes
	return p
}

// WithInteractiveStdin sets the stdin-interactive gating signal and returns the
// presenter so it chains onto any constructor, mirroring WithYes exactly. It is a
// builder-style setter — kept off the constructors so their signatures stay stable
// — so production can thread DetectStdinTTY(os.Stdin) at the same one site the -y
// flag is parsed (a later main/cmd task). The constructor default is true
// (interactive); call WithInteractiveStdin(false) to arm the
// forbidden-combination fail path.
func (p *PrettyPresenter) WithInteractiveStdin(interactive bool) *PrettyPresenter {
	p.stdinInteractive = interactive
	return p
}

// WithInput overrides the gate input reader and returns the presenter so it chains
// onto a constructor that did not take one (e.g. NewPrettyPresenterWithErr, whose
// stream-split test seam defaults input to os.Stdin). It exists so the stderr-split
// constructor can be combined with a scripted reader without adding yet another
// constructor; production never needs it.
func (p *PrettyPresenter) WithInput(in io.Reader) *PrettyPresenter {
	p.in = in
	return p
}

// WithSpinnerFactory overrides the stage-progress spinner factory and returns the
// presenter so it chains onto any constructor, mirroring WithYes/WithInput. It is
// the TEST SEAM for the spinner lifecycle: a test injects a spy factory whose
// spinners record Start/Stop so the lifecycle ("started on a blocking StageStarted,
// stopped on completion") and the "one spinner at a time" invariant are asserted
// deterministically, without the real briandowns library's timed goroutine and
// frame output. Production never calls it — the constructors default the factory to
// the real briandowns wrapper. The factory type is unexported, so only this package
// (and its external test, via the exported StageSpinner interface) can build one.
func (p *PrettyPresenter) WithSpinnerFactory(factory func(out io.Writer, text string) StageSpinner) *PrettyPresenter {
	p.newSpinner = factory
	return p
}

// writef writes one narration line to out. As with the plain presenter, a write
// error to the output stream has nowhere to propagate (Presenter methods return
// nothing — the engine narrates fire-and-forget) so it is discarded here, in one
// place, rather than ignored ad hoc at each call site.
func (p *PrettyPresenter) writef(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format, args...)
}

// errf writes one plain (unstyled) line to the err stream (stderr in
// production). Per the stream contract only the one-line failure/warning summary
// is duplicated here for redirect-visibility — never the multi-line captured
// body — and it is intentionally unstyled: stderr is a visibility channel, not a
// styled surface. The err writer is nil under the profile-forcing test
// constructor (which exercises only out), so a nil err is a no-op rather than a
// panic. As with writef, the write error is discarded.
func (p *PrettyPresenter) errf(format string, args ...any) {
	if p.err == nil {
		return
	}
	_, _ = fmt.Fprintf(p.err, format, args...)
}

// leafOrDefault returns the engine-supplied brand leaf, falling back to mint's
// own leaf when the payload omits it. Keeping the default in one helper means no
// render site hardcodes the brand glyph.
func leafOrDefault(leaf string) string {
	if leaf == "" {
		return defaultLeaf
	}
	return leaf
}

// RunStarted renders the top brand line, flush-left:
// "{leaf} mint · {project}  ›  {action} v{X}". The leaf and the action word are
// both engine-supplied from RunInfo, so the line is brand- and verb-shaped from
// the payload rather than hardcoding "🌿" or "releasing".
func (p *PrettyPresenter) RunStarted(info RunInfo) {
	leaf := leafOrDefault(info.Leaf)
	p.writef("%s mint · %s  ›  %s v%s\n", leaf, info.Project, info.Action, info.Version)
}

// StageStarted starts the pretty stage-progress spinner for a BLOCKING stage, and
// renders NOTHING for a short one.
//
//   - Blocking: start a single spinner on the current stage line, animating the dim
//     start text (the stage name) — a frame renders as "⠋ {name}". DEFENSIVELY stop
//     any spinner that is somehow still active first (a malformed event sequence
//     could leave one running), so there are never two concurrent spinners. The
//     handle is stored in activeSpinner; the matching StageSucceeded/StageFailed
//     stops it. The braille frame is the library's; the text is the dim start text.
//   - Non-blocking: render NOTHING — no spinner, no static line. This REPLACES the
//     Phase-1 placeholder static-dim-line: a short stage shows only its ✓ completion
//     line, consistent with plain and the spec's worked example.
func (p *PrettyPresenter) StageStarted(s StageStart) {
	if !s.Blocking {
		return
	}
	p.stopSpinner()
	startText := p.dim.Render(s.Name)
	p.activeSpinnerText = startText
	p.activeSpinner = p.newSpinner(p.out, startText)
	p.activeSpinner.Start()
}

// stopSpinner stops the active stage spinner (clearing its line in place), resets
// the handle to nil, AND clears any pending suspend state. It is a no-op for the
// handle when no spinner is running, so completion events for short (non-spinner)
// stages — and a defensive double-stop — are safe. Called by StageStarted
// (defensively before a new spinner) and by StageSucceeded/StageFailed (before the
// ✓/✗ line is printed in the cleared place). The real spinner's Stop is synchronous,
// so after this returns no further frame is written and the completion line is not
// interleaved with animation.
//
// Clearing spinnerSuspended here is what makes a stage that COMPLETES WHILE SUSPENDED
// safe: when SuspendSpinner has already stopped the spinner (activeSpinner nil) and a
// StageSucceeded/StageFailed then fires, the active-handle stop is a no-op but the
// suspend flag is still cleared, so a later ResumeSpinner does NOT resurrect a spinner
// for the already-completed stage. SuspendSpinner deliberately sets spinnerSuspended
// AFTER its own stop (it does not route through stopSpinner), so its own flag is never
// clobbered.
func (p *PrettyPresenter) stopSpinner() {
	p.spinnerSuspended = false
	if p.activeSpinner == nil {
		return
	}
	p.activeSpinner.Stop()
	p.activeSpinner = nil
}

// SuspendSpinner stops the active stage spinner around the engine's $EDITOR
// hand-off, releasing the terminal so the editor session is ANIMATION-FREE — no
// frame is written between this and ResumeSpinner. It is ENGINE-DRIVEN: the engine
// owns the e/r re-entry loop and invokes $EDITOR; this hook only suspends the
// presenter's OWN animation on command (the presenter never detects or invokes the
// editor).
//
// When a spinner is active it Stop()s it directly (NOT via stopSpinner, which would
// clear the suspend flag), REMEMBERS the spinner's start text so ResumeSpinner can
// recreate it on the SAME stage line, nils the active handle, and sets
// spinnerSuspended. When NO spinner is active it does NOTHING — a safe no-op that
// leaves spinnerSuspended false, so a paired ResumeSpinner also no-ops. The stop is a
// plain stop (no alt-screen, no screen-clear), consistent with the linear narration,
// and it preserves the one-spinner-at-a-time invariant (the handle is nil after).
func (p *PrettyPresenter) SuspendSpinner() {
	if p.activeSpinner == nil {
		return
	}
	p.activeSpinner.Stop()
	p.suspendedText = p.activeSpinnerText
	p.activeSpinner = nil
	p.spinnerSuspended = true
}

// ResumeSpinner restarts the spinner SuspendSpinner stopped, recreating it on the
// SAME stage line via the remembered start text and Start()ing exactly one — so the
// one-spinner-at-a-time invariant holds across any number of suspend/resume cycles.
// It clears spinnerSuspended once resumed.
//
// When nothing was suspended (spinnerSuspended false — either no spinner was active
// at the paired SuspendSpinner, or the stage already completed while suspended and
// cleared the flag) it does NOTHING: no spinner is created or started, so a completed
// stage is never resurrected. The restart is a plain Start (no alt-screen, no
// screen-clear).
func (p *PrettyPresenter) ResumeSpinner() {
	if !p.spinnerSuspended {
		return
	}
	p.activeSpinner = p.newSpinner(p.out, p.suspendedText)
	p.activeSpinnerText = p.suspendedText
	p.activeSpinner.Start()
	p.spinnerSuspended = false
}

// StageSucceeded first STOPS the active stage spinner (if any) — the spinner clears
// its line in place — then renders the success stage line in the cleared place:
// two-space indent, a green ✓, the stage name padded to a column, then the
// engine-supplied detail. The detail is rendered verbatim (it may already contain a
// "→" from the engine — the presenter never synthesises it). The elapsed suffix
// "({elapsed})" is appended on blocking stages only; short stages render the detail
// without it. The completion-line rendering below is REUSED unchanged from Phase 2;
// this task only added the spinner stop.
//
// When nothing trails the name — an empty Detail on a short stage — the name is
// emitted unpadded so the column's trailing spaces never become a
// trailing-whitespace artefact (the padding exists only to align a following
// detail; with nothing following it is noise).
func (p *PrettyPresenter) StageSucceeded(s StageSuccess) {
	p.stopSpinner()
	glyph := p.success.Render("✓")
	trailing := stageTrailing(s)
	if trailing == "" {
		p.writef("%s%s %s\n", stageIndent, glyph, s.Name)
		return
	}
	p.writef("%s%s %s%s\n", stageIndent, glyph, padStage(s.Name), trailing)
}

// stageTrailing builds the text that follows the padded stage name: the
// engine-supplied detail, with the compact "({elapsed})" appended on blocking
// stages only. Detail and elapsed are joined with a single space only when both
// are present, so an empty-detail blocking stage renders "({elapsed})" flush at
// the column with no stray leading space, and a short empty-detail stage returns
// the empty string (signalling the caller to drop the column padding entirely).
func stageTrailing(s StageSuccess) string {
	if !s.Blocking {
		return s.Detail
	}
	elapsed := fmt.Sprintf("(%s)", formatElapsed(s.Elapsed))
	if s.Detail == "" {
		return elapsed
	}
	return s.Detail + " " + elapsed
}

// StageFailed first STOPS the active stage spinner (if any) — the spinner clears
// its line in place — then renders the styled failure stage line to out (two-space
// indent, a red ✗, the padded stage name, then the message) in the cleared place,
// AND duplicates the one-line "✗ {stage}  {message}" summary to err — unstyled,
// since stderr is a redirect-visibility channel, not a styled surface — so a failure
// cannot silently vanish under redirection. The ✗-line and captured-body rendering
// below is REUSED unchanged from Phase 2; this task only added the spinner stop.
//
// When the engine captured underlying-command output (s.Output non-empty), the
// captured body is rendered to OUT ONLY, below the ✗ line — NO box, consistent
// with the boxless notes treatment. The body is written through the
// package-shared writeNotesBody helper UNCHANGED — byte-for-byte verbatim, the
// same bytes the plain presenter writes — so internal newlines/blank lines are
// preserved and a body line that reads like a delimiter survives as-is. Styling
// is intentionally MINIMAL: the body bytes are load-bearing, so they are written
// flush and unstyled (no dim wrap), which keeps them verbatim and means a colour
// downgrade leaves them untouched.
//
// The captured body is narration → out only and is NEVER duplicated to err: only
// the one-line summary goes there. An empty Output renders NO body block — the ✗
// line stands alone.
func (p *PrettyPresenter) StageFailed(s StageFailure) {
	p.stopSpinner()
	p.terminalFailure = true
	glyph := p.failure.Render("✗")
	p.writef("%s%s %s%s\n", stageIndent, glyph, padStage(s.Name), s.Message)
	p.errf("✗ %s  %s\n", s.Name, s.Message)
	if s.Output == "" {
		return
	}
	writeNotesBody(p.out, s.Output)
}

// Unwound renders the auto-unwind line to OUT ONLY, MIRRORING the StageSucceeded
// line shape exactly: two-space indent, the ↩ glyph styled through the renderer,
// the literal label "unwound" padded to the stage column, then the engine-supplied
// summary rendered VERBATIM — INCLUDING its own "— repo clean" tail (the presenter
// synthesises no tail of its own). The worked example reads
// "  ↩ unwound    removed tag v1.4.0, reset 2 release commit(s) — repo clean".
//
// Per the per-event stream table the auto-unwind line is narration only and is NOT
// duplicated to err, unlike the ✗/⚠ summaries. Unwound marks the run terminal
// (setting terminalFailure) so a subsequent RunFinished suppresses the success
// bottom brand line — covering BOTH the failure path (StageFailed → Unwound) and
// the abort path (gate-n: Unwound with no prior StageFailed). The ↩ glyph is styled
// dim; the layout (indent, glyph, column padding) survives a colour downgrade as
// bare text.
func (p *PrettyPresenter) Unwound(u Unwind) {
	p.terminalFailure = true
	glyph := p.unwound.Render("↩")
	p.writef("%s%s %s%s\n", stageIndent, glyph, padStage("unwound"), u.Summary)
}

// Warn renders a standalone amber warning line to out — two-space indent, the
// amber ⚠ glyph, the label, a two-space gap, then the message — and duplicates an
// UNSTYLED copy of the same text to err per the stream contract, mirroring
// StageFailed's err summary (stderr is a redirect-visibility channel, not a styled
// surface). Label and message are separate engine-supplied fields; the presenter
// never parses a label out of a combined string.
//
// The label is NOT padStage-aligned: warnings are standalone, not part of the
// aligned stage sequence, so they carry no column padding. Warn is independent of
// run state — it sets no failure and suppresses no end-of-run line — and multiple
// Warn calls each render their own line, in order, with no collapsing.
//
// Empty-message edge: the line collapses to "{stageIndent}⚠ {label}" (and the err
// copy to "⚠ {label}") with NO trailing-whitespace artefact — the two-space gap
// and the message are both dropped when there is no message.
func (p *PrettyPresenter) Warn(w Warning) {
	glyph := p.warn.Render("⚠")
	p.writef("%s%s %s\n", stageIndent, glyph, warnText(w))
	p.errf("⚠ %s\n", warnText(w))
}

// warnText builds the layout that follows the ⚠ glyph: the label, then a two-space
// gap and the message when a message is present. With an empty message it returns
// just the label so neither the out line nor the err copy dangles a trailing-space
// artefact where the message would be.
func warnText(w Warning) string {
	if w.Message == "" {
		return w.Label
	}
	return w.Label + "  " + w.Message
}

// planIndent is the four-space indent every plan bullet line carries — one level
// deeper than the two-space stage/header indent, nesting the steps under the
// "Plan" header. The "•" bullet that follows is a pretty-only glyph (never
// rendered in plain mode); only the glyph is styled, so this indent and the
// padding/verb/target that follow stay as plain layout and survive a colour
// downgrade intact.
const planIndent = "    "

// ShowPlan renders the plan as a styled bulleted block: a two-space-indented
// "Plan" header, then one "    • {verb}<pad>{target}" line per step. Verbs pad to
// a per-plan column — the longest verb in THIS plan plus two spaces — so the
// targets align (matching the worked example's dynamic alignment). It derives
// entirely from the SAME structured steps the plain one-liner does.
//
// Edge forms: an empty plan omits the ENTIRE block (no header, no bullets — no
// orphan header); a step with an empty target renders "    • {verb}" with no
// trailing pad or space. The header and bullet glyph are styled through the
// lipgloss renderer, but all layout (indents, the bullet, the column padding)
// survives a colour downgrade as plain text.
func (p *PrettyPresenter) ShowPlan(plan Plan) {
	if len(plan.Steps) == 0 {
		return
	}
	p.writef("%s%s\n", stageIndent, p.dim.Render("Plan"))
	column := planVerbColumn(plan.Steps)
	bullet := p.dim.Render("•")
	for _, step := range plan.Steps {
		if step.Target == "" {
			p.writef("%s%s %s\n", planIndent, bullet, step.Verb)
			continue
		}
		p.writef("%s%s %s%s\n", planIndent, bullet, padVerb(step.Verb, column), step.Target)
	}
}

// planVerbColumn computes the per-plan alignment column: the longest verb in the
// plan plus a two-space gap, so every target starts at the same column two spaces
// past the widest verb. The column is dynamic per plan (matching the worked
// example, where publish=7 sets the column and the shorter verbs pad up to it).
func planVerbColumn(steps []PlanStep) int {
	longest := 0
	for _, step := range steps {
		if len(step.Verb) > longest {
			longest = len(step.Verb)
		}
	}
	return longest + 2
}

// padVerb right-pads a verb with spaces to the given column so the following
// target aligns across steps.
func padVerb(verb string, column int) string {
	return fmt.Sprintf("%-*s", column, verb)
}

// ShowNotes renders the release notes as a titled opener rule, the body verbatim,
// and a closing rule — NO box (the rounded box was dropped: it forced
// wrap/truncate on arbitrary-width AI notes and read as clutter). The rules may be
// dim-styled through the lipgloss renderer, but their layout (the title text and
// the U+2500 rule characters) survives a colour downgrade; the body is written via
// the package-shared writeNotesBody helper — UNCHANGED, the same bytes the plain
// presenter writes — so the body region is provably byte-identical across modes.
//
// The body is written flush, NOT indented: the worked example shows the body
// indented two spaces under the rule, but that indentation is illustrative and is
// deliberately not applied, because adding indent bytes would break byte-identity
// (non-negotiable — "what previews is what ships"). The body is NEVER truncated.
//
// Edge forms mirror plain: an empty body writes nothing between the rules, so the
// titled rule is immediately followed by the closing rule — no spurious blank
// line. A body line that reads like a delimiter is written verbatim; the real
// closing rule still follows (rules are positional, never content-matched).
func (p *PrettyPresenter) ShowNotes(notes Notes) {
	p.writef("%s\n", p.dim.Render(notesTitledRule(notes.Version)))
	writeNotesBody(p.out, notes.Body)
	p.writef("%s\n", p.dim.Render(notesClosingRule()))
}

// notesTitledRule builds the opener rule: the "── release notes · v{X} " title
// prefix filled with U+2500 up to decorativeRuleWidth. The fill count is clamped
// to a minimum of one so a title prefix longer than the cap (version strings are
// short, so this is just defensive) never produces a negative repeat count.
func notesTitledRule(version string) string {
	prefix := ruleChar + ruleChar + " release notes · v" + version + " "
	fill := decorativeRuleWidth - displayWidth(prefix)
	if fill < 1 {
		fill = 1
	}
	return prefix + strings.Repeat(ruleChar, fill)
}

// notesClosingRule builds the closing rule: U+2500 repeated to the cap width.
func notesClosingRule() string {
	return strings.Repeat(ruleChar, decorativeRuleWidth)
}

// displayWidth counts the runes in s — the column count for the ASCII/box-drawing
// rule text the title is built from (each such rune occupies one cell). Counting
// runes rather than bytes keeps the multi-byte U+2500 characters from inflating
// the width math, so the rule fills to the intended cap.
func displayWidth(s string) int {
	return len([]rune(s))
}

// menuIndent is the four-space indent every gate menu option line carries — one
// level deeper than the two-space prompt/question indent, nesting the options
// under the (later-printed) "{Question} › " line, matching the spec's worked
// example which indents the y/n/e/r options four spaces.
const menuIndent = "    "

// defaultMarker is the suffix appended to the one option line whose key equals the
// gate's Default, flagging the empty-Enter accept path (the spec's "[default]"
// beside its action). It carries a leading space so it reads as " [default]" after
// the action text; it is plain layout (not styled), so it survives a colour
// downgrade verbatim and stays a contiguous substring under colour.
const defaultMarker = " [default]"

// promptMarker is the "› " cursor marker that ends the prompt line. It is dim
// (secondary narration, like the Plan header and notes rules) and is styled as ONE
// unit — glyph plus its trailing space — so the "› " pair stays a contiguous
// substring even under colour, and survives a colour downgrade as bare "› ".
const promptMarker = "› "

// ShowVersion renders the dressed version line to OUT ONLY: the engine-supplied
// brand leaf (default 🌿 via leafOrDefault), the "mint" brand, and the value with a
// decorative "v" prefix — "{leaf} mint v{value}", matching the worked spec form
// "🌿 mint v1.4.0". It is flush-left like the other brand lines.
//
// Styling is ADDITIVE only: the whole line is dim-styled through the lipgloss
// renderer (the subdued brand tone), but the layout — the leaf, "mint", and the
// load-bearing "v{value}" — survives a colour downgrade as bare text, so under a
// no-colour profile the line is exactly "{leaf} mint v{value}" with no SGR codes and
// the value stays present and legible. The "v" prefix is PRETTY-only (plain writes
// the bare value). version has no gate and no release footer — this line is the
// terminal output, narration → out only (never err).
func (p *PrettyPresenter) ShowVersion(v Version) {
	leaf := leafOrDefault(v.Leaf)
	line := fmt.Sprintf("%s mint v%s", leaf, v.Value)
	p.writef("%s\n", p.dim.Render(line))
}

// Prompt drives the SAME shared line-read input loop the plain presenter uses
// (readChoice/parseChoice): empty Enter selects the gate's Default, case-insensitive
// input maps to a declared key, unrecognised input re-prompts, and EOF returns a
// non-nil error rather than silently default-accepting. Only the render closure is
// mode-specific.
//
// The pretty render is the FULL vertical menu (renderGate): the gate's declared
// choices listed in order ABOVE the question, "[default]" beside the default
// choice's action, a blank line, then the "{Question} › " prompt line LAST. The
// shared loop calls renderGate on EVERY pass — including after unrecognised input —
// so the menu is redrawn linearly (it scrolls; no screen-clearing, no alt-screen)
// for free.
//
// Under -y the gate is SKIPPED (not drawn-then-auto-pressed): the vertical menu is
// not rendered and the input stream is NOT read at all. Instead the auto-accept is
// communicated as a RENDERED event — a concise accept line in the run's success
// vocabulary, "  ✓ {Subject}  {AcceptEcho} (-y)" (two-space indent, the green ✓
// glyph like every other success line, one space, the subject, two spaces, the
// echo word) — written to OUT only, never an err copy. This is the fixed-2-space
// concise form (NOT padStage column alignment): the line stands on its own at the
// gate point rather than aligning to the run's stage column, and the chosen form
// matches the spec's literal "✓ notes  accepted (-y)" example. Both the Subject and
// the echo word (AcceptEcho — "accepted" for notes, the chosen value for
// source/target) travel in the gate payload, so neither is hardcoded here. The
// gate's declared default is returned with a nil error.
func (p *PrettyPresenter) Prompt(gate Gate) (Choice, error) {
	if p.yes {
		glyph := p.success.Render("✓")
		p.writef("%s%s %s  %s (-y)\n", stageIndent, glyph, gate.Subject, gate.AcceptEcho)
		return gate.Default, nil
	}
	if !p.stdinInteractive {
		return p.failNotInteractive()
	}
	reader := bufferedReader(p.in, &p.reader)
	render := func() { p.renderGate(gate) }
	return readChoice(reader, render, gate)
}

// failNotInteractive renders the FORBIDDEN-COMBINATION failure (non-TTY stdin
// without -y) WITHOUT touching the input stream — the whole point is to never
// block on stdin that will not deliver. It MIRRORS StageFailed's rendering: the
// styled "  ✗ {label}  {message}" line to OUT (two-space indent, the red ✗ glyph
// through the failure style, the fixed gateFailLabel padded to the stage column,
// then the message) AND the UNSTYLED "✗ {label}  {message}" one-line summary to
// ERR per the stream contract. The label is the fixed gateFailLabel ("gate") —
// this is the gate MECHANISM failing, not gate.Subject (the notes content). The
// message is the spec's em-dash form gateNotTTYMessagePretty (the em dash is
// allowed in pretty). The styling stays from the renderer, which is bound to
// stdout, so the failure renders STYLED even though it was the non-TTY STDIN that
// triggered it — the two axes are orthogonal. Prompt returns the exported
// ErrNotInteractive sentinel; the presenter sets no exit code.
func (p *PrettyPresenter) failNotInteractive() (Choice, error) {
	glyph := p.failure.Render("✗")
	p.writef("%s%s %s%s\n", stageIndent, glyph, padStage(gateFailLabel), gateNotTTYMessagePretty)
	p.errf("✗ %s  %s\n", gateFailLabel, gateNotTTYMessagePretty)
	return "", ErrNotInteractive
}

// renderGate writes the pretty vertical menu for one gate to out: one option line
// per declared choice (in declared order), a blank line, then the question prompt
// line. The menu is built ENTIRELY from gate.Choices — there is no hardcoded
// y/n/e/r list — so a two-choice gate renders two option lines and reordering the
// gate reorders the menu. The "[default]" marker is placed by comparing each
// choice's Key to gate.Default, so it lands on whatever choice the gate declares as
// its default (not always y).
//
// Styling is deliberately MINIMAL and layout-preserving: only the option KEY and
// the "› " prompt marker are dim-styled (the secondary-narration tone shared with
// the Plan header and notes rules); the action text, the " [default]" marker, the
// indentation, and the question text stay PLAIN. Under a colour downgrade lipgloss
// emits no ANSI, so the whole menu survives as plain text; under colour the styled
// spans carry ANSI while every structural substring (each option line's action,
// " [default]", "{Question}", "› ") stays contiguous and intact.
//
// The prompt line is written WITHOUT a trailing newline — the cursor sits after
// "› " for the line-read, matching the worked example's "  Continue? › ".
func (p *PrettyPresenter) renderGate(g Gate) {
	for _, choice := range g.Choices {
		p.writef("%s%s  %s%s\n", menuIndent, p.dim.Render(string(choice.Key)), choice.Action, defaultSuffix(choice.Key, g.Default))
	}
	p.writef("\n")
	p.writef("%s%s %s", stageIndent, g.Question, p.dim.Render(promptMarker))
}

// defaultSuffix returns the " [default]" marker when this choice key is the gate's
// default, and the empty string otherwise — so exactly the default option line is
// marked.
func defaultSuffix(key, def Choice) string {
	if key == def {
		return defaultMarker
	}
	return ""
}

// initSkipGlyph is the NEUTRAL middot (U+00B7) the pretty init skipped notice
// leads with. A skip is neither a success nor a failure, so it deliberately uses
// none of the load-bearing status glyphs (✓ success, ✗ failure, ⚠ warn, ↩ unwind):
// it is a neutral notice, and the middot reads as a quiet bullet. It is styled
// through the dim style (the same subdued, secondary-narration tone shared with the
// Plan header and notes rules), which suits a non-status notice; the glyph and the
// layout that follows survive a colour downgrade as bare text.
const initSkipGlyph = "·"

// InitResult renders one init outcome to OUT ONLY in pretty's "{glyph} {action-word}
// {target}" vocabulary — the word order is glyph-led, the reverse of plain's
// "{target}: {action}". A created outcome renders "  ✓ created {target}": two-space
// indent, the GREEN ✓ (the success style shared with StageSucceeded), the action word
// "created", then the target. A skipped outcome renders
// "  · skipped {target} ({reason})": two-space indent, the dim NEUTRAL middot (a skip
// is neither success nor failure — see initSkipGlyph), "skipped", the target, then
// " ({reason})" with the engine-supplied Reason rendered VERBATIM.
//
// Only the leading glyph is styled (green ✓ / dim ·); the action word, target, and
// reason stay plain layout, so the whole line survives a colour downgrade as bare
// text. init has no gate and no release-style footer — these lines ARE the terminal
// output. InitResult is narration → out only and is never duplicated to err.
func (p *PrettyPresenter) InitResult(r InitOutcome) {
	if r.Action == InitSkipped {
		glyph := p.dim.Render(initSkipGlyph)
		p.writef("%s%s skipped %s (%s)\n", stageIndent, glyph, r.Target, r.Reason)
		return
	}
	glyph := p.success.Render("✓")
	p.writef("%s%s created %s\n", stageIndent, glyph, r.Target)
}

// RunFinished renders the bottom brand line, flush-left, via an EXHAUSTIVE
// dispatch on r.Verb — gated FIRST by the success-suppression flag.
//
// SUPPRESSION PRECEDES SHAPING. The bottom brand line is SUCCESS-ONLY: when the
// run has hit a terminal failure or abort (terminalFailure set by StageFailed or
// Unwound) this emits NOTHING — there is no failure-flavoured closing brand line,
// even for a release that still carries a URL. The run has already ended after the
// ✗/unwound lines; failure is signalled by those lines plus the engine-owned
// non-zero exit code. The presenter never sets the exit code. The suppression
// check runs BEFORE the verb switch, so it covers EVERY shape — release,
// regenerate, init, and version alike. Warn does NOT set terminalFailure, so a
// warn-only run still emits the success footer.
//
// Verb dispatch (the presenter never re-derives the verb — the shape comes from
// the payload):
//
//   - VerbRelease (the iota-0 default, so a Verb-less literal lands here):
//     "{leaf} released {project} v{X} · {url}". The leaf is the ENGINE-SUPPLIED
//     brand leaf (r.Leaf via leafOrDefault, default 🌿) — never hardcoded — so a
//     customised commit_prefix brand stays consistent with the start-of-run brand
//     line. When URL is empty the " · {url}" segment is omitted cleanly with no
//     dangling separator.
//   - VerbRegenerate: regenerate publishes no release and has NO URL, so the close
//     is the URL-less, verb-shaped "{leaf} regenerated {project} {Summary}" — the
//     {url} field is omitted ENTIRELY (not rendered empty, no dangling " · "). The
//     verb word "regenerated" mirrors "released"; the engine-supplied Summary is the
//     single version or, under --all, the set/range/count text, rendered VERBATIM
//     (the presenter never computes the version set). The --all single-version case
//     still lands here (Verb=VerbRegenerate), so it renders the set summary, not a
//     release-style v{X}+url footer.
//   - VerbInit, VerbVersion: NO footer — init's created/skipped lines and version's
//     value line are themselves the terminal output. These arms render NOTHING
//     (defensive completeness; in practice the engine does not call RunFinished for
//     init/version).
func (p *PrettyPresenter) RunFinished(r RunResult) {
	if p.terminalFailure {
		return
	}
	switch r.Verb {
	case VerbRelease:
		p.renderReleaseFooter(r)
	case VerbRegenerate:
		p.writef("%s regenerated %s %s\n", leafOrDefault(r.Leaf), r.Project, r.Summary)
	case VerbInit, VerbVersion:
		// No release-style footer: these verbs' own lines are the terminal output.
	}
}

// renderReleaseFooter writes the release-success bottom brand line
// "{leaf} released {project} v{X} · {url}", omitting the " · {url}" segment
// cleanly when URL is empty so no dangling separator is left. The leaf is the
// engine-supplied brand leaf (default 🌿 via leafOrDefault).
func (p *PrettyPresenter) renderReleaseFooter(r RunResult) {
	leaf := leafOrDefault(r.Leaf)
	if r.URL == "" {
		p.writef("%s released %s v%s\n", leaf, r.Project, r.Version)
		return
	}
	p.writef("%s released %s v%s · %s\n", leaf, r.Project, r.Version, r.URL)
}

// padStage right-pads the stage name with spaces to the detail column so the
// detail lines up across stages. Names at or beyond the column width get a single
// trailing space so the glyph/name/detail never collide.
func padStage(name string) string {
	if len(name) >= stageColumn {
		return name + " "
	}
	return fmt.Sprintf("%-*s", stageColumn, name)
}

// formatElapsed renders a duration as the compact "{seconds}s" form shown in the
// worked example (e.g. 2.3s). The engine measures the elapsed time; the presenter
// only formats the supplied value.
func formatElapsed(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
