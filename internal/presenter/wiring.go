package presenter

import (
	"io"
	"os"
)

// New is the single wiring point where mode selection and the stdout/stderr
// stream split meet. It returns the Presenter implementation matching the
// already-selected Mode — ModePretty yields the styled presenter, ModePlain (the
// zero value, so any unrecognised Mode safely lands here too) yields the terse
// one — with both writers wired through. Returning the interface type keeps every
// caller depending on the seam rather than a concrete presenter.
//
// The stream contract is fixed and identical across modes: out receives all
// narration; err receives the one-line failure summary (per StageFailed) and
// nothing on a successful run. Centralising construction here means the split is
// established in exactly one place.
func New(mode Mode, out, err io.Writer) Presenter {
	if mode == ModePretty {
		return NewPrettyPresenter(out, err)
	}
	return NewPlainPresenter(out, err)
}

// NewForStartup is the default startup convenience: it selects the render mode
// from the real stdout handle's TTY signal (honouring --plain via plainFlag) and
// wires stdout for narration and stderr for the failure summary — the production
// defaults of out = os.Stdout, err = os.Stderr at the one construction site.
//
// stdout is taken as *os.File (not io.Writer) because DetectMode needs the OS
// stream type to detect a TTY; *os.File satisfies io.Writer, so it doubles as the
// narration writer. Accepting the handles as parameters (rather than reaching for
// the os globals internally) keeps this unit-testable: a test passes a non-TTY
// handle to drive the plain path deterministically.
//
// Width wiring lives HERE rather than in the New(mode, out, err) seam because this
// is the one site that holds the stdout *os.File the OS width probe needs, and
// golang.org/x/term is already this package's TTY dependency. When the selected mode
// is PRETTY, the terminal width is probed from the stdout handle (detectTermWidth)
// and applied to the pretty presenter via WithTermWidth so its decorative notes
// rules cap at min(terminalWidth, ruleCap). For the plain mode the width is
// irrelevant (plain has no decorative rules and does no width math), so it is not
// probed — the plain presenter is returned unchanged. detectTermWidth returns 0 on
// an undetectable width, which ruleWidth maps back to the cap, so even a pretty
// presenter on a width-less terminal renders the fixed-cap rule.
func NewForStartup(plainFlag bool, stdout, stderr *os.File) Presenter {
	mode := DetectMode(plainFlag, stdout)
	if mode == ModePretty {
		return NewPrettyPresenter(stdout, stderr).WithTermWidth(detectTermWidth(stdout))
	}
	return New(mode, stdout, stderr)
}
