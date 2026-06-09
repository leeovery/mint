package presenter

import (
	"fmt"
	"io"
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

	// Styles are derived once from the renderer so every render shares the same
	// colour profile. Under a no-colour profile lipgloss renders these as the bare
	// text, preserving layout and glyphs without emitting any ANSI escape.
	success lipgloss.Style
	failure lipgloss.Style
	dim     lipgloss.Style
}

// Compile-time proof the pretty presenter satisfies the seam it renders.
var _ Presenter = (*PrettyPresenter)(nil)

// NewPrettyPresenter constructs a PrettyPresenter writing narration to out. The
// renderer is bound to out so lipgloss auto-detects (and auto-downgrades) the
// terminal's colour capabilities — the production path needs no explicit profile.
// The err writer is accepted now to keep the constructor signature stable across
// phases; this task narrates only to out.
func NewPrettyPresenter(out, err io.Writer) *PrettyPresenter {
	return newPrettyPresenter(out, err, lipgloss.NewRenderer(out))
}

// NewPrettyPresenterWithProfile constructs a PrettyPresenter whose renderer is
// forced to the given colour profile, with no err writer wired. It is the test
// seam for out-only assertions: tests pass termenv.TrueColor/ANSI to assert ANSI
// codes are emitted, or termenv.Ascii to assert the colour auto-downgrade emits
// none while layout and glyphs survive. Use NewPrettyPresenterWithErr when the
// stderr split itself is under test.
func NewPrettyPresenterWithProfile(out io.Writer, profile termenv.Profile) *PrettyPresenter {
	return NewPrettyPresenterWithErr(out, nil, profile)
}

// NewPrettyPresenterWithErr constructs a PrettyPresenter whose renderer is forced
// to the given colour profile AND whose err writer is wired. It is the test seam
// for the stream-split contract: forcing colour on out while capturing err proves
// the stderr summary stays unstyled by design — not merely because lipgloss
// auto-downgrades on a non-TTY buffer.
func NewPrettyPresenterWithErr(out, err io.Writer, profile termenv.Profile) *PrettyPresenter {
	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(profile)
	return newPrettyPresenter(out, err, renderer)
}

// newPrettyPresenter is the shared constructor core: it derives the styles from
// the supplied renderer so colour-on and colour-off paths build identically and
// differ only in the renderer's profile.
func newPrettyPresenter(out, err io.Writer, renderer *lipgloss.Renderer) *PrettyPresenter {
	return &PrettyPresenter{
		out:      out,
		err:      err,
		renderer: renderer,
		success:  renderer.NewStyle().Foreground(lipgloss.Color("2")), // green
		failure:  renderer.NewStyle().Foreground(lipgloss.Color("1")), // red
		dim:      renderer.NewStyle().Foreground(lipgloss.Color("8")), // bright black / dim
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
// silently vanish under redirection. The captured-output body (s.Output) is
// narration → out only; when later phases render it below this line, it MUST NOT
// be duplicated to err — only the one-line summary goes there.
func (p *PrettyPresenter) StageFailed(s StageFailure) {
	glyph := p.failure.Render("✗")
	p.writef("%s%s %s%s\n", stageIndent, glyph, padStage(s.Name), s.Message)
	p.errf("✗ %s  %s\n", s.Name, s.Message)
}

// RunFinished renders the bottom brand line, flush-left:
// "{leaf} released {project} v{X} · {url}". The leaf is engine-supplied (default
// 🌿). When URL is empty (e.g. regenerate, which publishes no release) the
// " · {url}" segment is omitted cleanly with no dangling separator.
//
// NOTE: the literal "released" here is the skeleton placeholder; the verb-shaped
// end-of-run line and regenerate's URL-less variant are owned by a later task.
func (p *PrettyPresenter) RunFinished(r RunResult) {
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
