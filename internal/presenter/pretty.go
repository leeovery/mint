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
// Spinner animation is out of scope this phase: StageStarted renders a single
// static line so the flow stays linear.
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
		success:  renderer.NewStyle().Foreground(lipgloss.Color("2")),   // green
		failure:  renderer.NewStyle().Foreground(lipgloss.Color("1")),   // red
		warn:     renderer.NewStyle().Foreground(lipgloss.Color("214")), // amber / orange
		dim:      renderer.NewStyle().Foreground(lipgloss.Color("8")),   // bright black / dim
		unwound:  renderer.NewStyle().Foreground(lipgloss.Color("8")),   // ↩ glyph — dim (no spec colour; subdued recovery tone)
	}
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

// StageStarted renders a single static dim stage line. The spinner lifecycle is a
// later phase; keeping this one printed line keeps the narration linear. With no
// detail payload to align to this phase, the name is not column-padded — that
// avoids trailing whitespace on a line that has nothing after the name.
func (p *PrettyPresenter) StageStarted(s StageStart) {
	line := fmt.Sprintf("%s%s", stageIndent, s.Name)
	p.writef("%s\n", p.dim.Render(line))
}

// StageSucceeded renders the success stage line: two-space indent, a green ✓,
// the stage name padded to a column, then the engine-supplied detail. The detail
// is rendered verbatim (it may already contain a "→" from the engine — the
// presenter never synthesises it). The elapsed suffix "({elapsed})" is appended
// on blocking stages only; short stages render the detail without it.
//
// When nothing trails the name — an empty Detail on a short stage — the name is
// emitted unpadded so the column's trailing spaces never become a
// trailing-whitespace artefact (the padding exists only to align a following
// detail; with nothing following it is noise).
func (p *PrettyPresenter) StageSucceeded(s StageSuccess) {
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

// StageFailed renders the styled failure stage line to out (two-space indent, a
// red ✗, the padded stage name, then the message) AND duplicates the one-line
// "✗ {stage}  {message}" summary to err — unstyled, since stderr is a
// redirect-visibility channel, not a styled surface — so a failure cannot
// silently vanish under redirection.
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
// for free. The -y gate skip (task 3-5) bypasses this entirely.
func (p *PrettyPresenter) Prompt(gate Gate) (Choice, error) {
	reader := bufferedReader(p.in, &p.reader)
	render := func() { p.renderGate(gate) }
	return readChoice(reader, render, gate)
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

// RunFinished renders the bottom brand line, flush-left:
// "{leaf} released {project} v{X} · {url}". The leaf is engine-supplied (default
// 🌿). When URL is empty (e.g. regenerate, which publishes no release) the
// " · {url}" segment is omitted cleanly with no dangling separator.
//
// NOTE: the literal "released" here is the skeleton placeholder; the verb-shaped
// end-of-run line and regenerate's URL-less variant are owned by a later task.
//
// The bottom brand line is SUCCESS-ONLY: when the run has hit a terminal failure
// or abort (terminalFailure set by StageFailed or Unwound) this emits NOTHING —
// there is no failure-flavoured closing brand line. The run has already ended
// after the ✗/unwound lines; failure is signalled by those lines plus the
// engine-owned non-zero exit code. The presenter never sets the exit code.
func (p *PrettyPresenter) RunFinished(r RunResult) {
	if p.terminalFailure {
		return
	}
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
