package presenter

import (
	"fmt"
	"io"
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
}

// Compile-time proof the plain presenter satisfies the seam it renders.
var _ Presenter = (*PlainPresenter)(nil)

// NewPlainPresenter constructs a PlainPresenter writing narration to out and the
// one-line failure/warning summary additionally to err (stdout/stderr in
// production). The split is fixed regardless of render mode.
func NewPlainPresenter(out, err io.Writer) *PlainPresenter {
	return &PlainPresenter{out: out, err: err}
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
// version is rendered with a "v" prefix.
func (p *PlainPresenter) RunStarted(info RunInfo) {
	p.writef("mint: %s %s v%s\n", info.Action, info.Project, info.Version)
}

// StageStarted emits plain's spinner-equivalent: a terse start line for a
// blocking (long/slow) stage only, so a live-tail consumer isn't staring at
// silence through a multi-second wait. Short stages stay silent until completion.
// The wording is presenter-synthesised narration, so it stays byte-pure ASCII —
// "generating..." with an ASCII ellipsis, not the U+2026 glyph the pretty spinner
// uses (the spec's "wording refinable" latitude; the byte-purity guard is fixed).
func (p *PlainPresenter) StageStarted(s StageStart) {
	if !s.Blocking {
		return
	}
	p.writef("%s: generating...\n", s.Name)
}

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
// failure cannot silently vanish under redirection. The captured-output body
// (s.Output) is narration → out only; when later phases render it, it MUST NOT be
// duplicated to err — only the one-line summary goes there.
func (p *PlainPresenter) StageFailed(s StageFailure) {
	p.writef("%s: FAILED - %s\n", s.Name, s.Message)
	p.errf("%s: FAILED - %s\n", s.Name, s.Message)
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
func (p *PlainPresenter) Warn(w Warning) {
	p.writef("%s: WARN - %s\n", w.Label, w.Message)
	p.errf("%s: WARN - %s\n", w.Label, w.Message)
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

// RunFinished renders the success-shaped end-of-run line. With a release URL it is
// "done: {project} v{X} {url}"; verbs that publish no release leave URL empty, so
// the line collapses to "done: {project} v{X}" with no dangling trailing space.
func (p *PlainPresenter) RunFinished(r RunResult) {
	if r.URL == "" {
		p.writef("done: %s v%s\n", r.Project, r.Version)
		return
	}
	p.writef("done: %s v%s %s\n", r.Project, r.Version, r.URL)
}
