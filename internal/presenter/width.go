package presenter

import (
	"os"

	"golang.org/x/term"
)

// ruleCap is the maximum width the decorative notes rules render to — the "~50"
// upper bound the spec puts on the capped width. It is the SINGLE width concession
// pretty mode makes: the titled "── release notes · v{X} ──"/closing rules are
// sized to min(terminalWidth, ruleCap) so they can neither overflow a narrow
// terminal (wrapping into junk) nor sprawl across a very wide one. 50 columns is a
// comfortable rule on any reasonable terminal and matches the worked example in the
// spec. Everything else in pretty mode does NO width math and wraps naturally.
//
// This is the ONE width constant: notesTitledRule/notesClosingRule build the rules
// from ruleWidth(p.termWidth), which is bounded above by this cap. (It was named
// decorativeRuleWidth in task 2-5, which rendered at the fixed cap and deferred the
// terminal-width source to this task; the rename to ruleCap reflects its role as the
// upper bound rather than the literal rule width.)
const ruleCap = 50

// ruleWidth is the PURE width source for the decorative notes rules: it caps the
// rule at min(termWidth, ruleCap), falling back to the full cap whenever the width
// is undetectable (termWidth ≤ 0, the sentinel detectTermWidth returns on error).
//
// It is deliberately pure — no OS call, no terminal device — so the width logic is
// unit-testable across narrow/wide/tiny/undetectable widths without a real terminal
// (the OS probe is isolated in detectTermWidth). A tiny width (e.g. 3) yields a tiny
// rule via the SAME min — there is NO bespoke tiny-terminal branch; a genuinely
// tiny/weird terminal is a `--plain` case, not a special layout here.
//
// A zero termWidth (the PrettyPresenter default) returns ruleCap, so a presenter
// constructed without WithTermWidth renders the same fixed-cap rule task 2-5 did —
// keeping the 2-5 ShowNotes layout tests green unchanged.
func ruleWidth(termWidth int) int {
	if termWidth <= 0 {
		return ruleCap
	}
	return min(termWidth, ruleCap)
}

// detectTermWidth is the production width-detection helper: it probes the terminal
// width of f via term.GetSize, returning the column count or 0 when the width is
// undetectable (f is not a terminal, or the probe errors). The 0 sentinel feeds
// ruleWidth, which falls back to the cap — so an undetectable width renders the
// fixed-cap rule.
//
// This is the SOLE site of the OS terminal-size call, isolating it from the pure
// ruleWidth core (the same separation mode.go keeps between IsTerminal's OS probe
// and the pure SelectMode). It reuses golang.org/x/term — already this package's TTY
// dependency — rather than introducing a second mechanism.
func detectTermWidth(f *os.File) int {
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return width
}
