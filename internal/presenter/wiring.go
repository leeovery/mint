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
		return NewPrettyPresenter(out, WithErr(err))
	}
	return NewPlainPresenter(out, err)
}

// NewForStartup is the CONVERGED startup seam: the ONE production construction
// site that resolves BOTH orthogonal axes via DetectStartupSignals and threads all
// four signals — render Mode, terminal width, the -y gating decision, and the
// stdin-interactive gating signal — onto the returned presenter. It wires stdout
// for narration and stderr for the failure summary (the production defaults of
// out = os.Stdout, err = os.Stderr), with stdin held only to resolve the gating
// axis. This is the single seam where mode selection, the stream split, width
// probing, and BOTH gating axes meet, so production never drifts into a parallel
// path that leaves a gating field at its interactive default.
//
// The render Mode is selected from stdout's TTY signal (honouring --plain via
// plainFlag); the stdin-interactive signal is detected INDEPENDENTLY from the
// stdin descriptor (DetectStartupSignals, never re-derived from Mode), so the
// forbidden-combination fail-loud path — non-TTY stdin without -y — is reachable
// through this seam. -y is a PARAMETER the caller supplies (the flag is parsed
// elsewhere); the seam only threads it.
//
// All three handles are taken as *os.File (not io.Writer): stdout and stdin so
// DetectStartupSignals can probe each for a TTY, stderr to mirror them. *os.File
// satisfies io.Writer, so stdout/stderr double as the narration/summary writers.
// Accepting the handles as parameters (rather than reaching for the os globals
// internally) keeps this unit-testable: a test passes non-TTY handles to drive the
// plain path and the forbidden-combination path deterministically.
//
// The four signals are applied on the CONCRETE presenter built in each mode branch
// (WithYes/WithInteractiveStdin live on the concrete types, not the Presenter
// interface), then returned as the interface:
//   - PRETTY: the terminal width is probed from the stdout handle (detectTermWidth)
//     and applied via WithTermWidth so the decorative notes rules cap at
//     min(terminalWidth, ruleCap). detectTermWidth returns 0 on an undetectable
//     width, which ruleWidth maps back to the cap, so a width-less terminal still
//     renders the fixed-cap rule.
//   - PLAIN: width is irrelevant (plain has no decorative rules and does no width
//     math), so it is not probed. The plain presenter keeps its production default
//     input reader (os.Stdin) — stdin is held here only to resolve the
//     stdin-interactive signal, not as the gate input stream.
func NewForStartup(plainFlag, yes bool, stdout, stderr, stdin *os.File) Presenter {
	signals := DetectStartupSignals(plainFlag, stdout, stdin)
	if signals.Mode == ModePretty {
		return NewPrettyPresenter(stdout, WithErr(stderr)).
			WithTermWidth(detectTermWidth(stdout)).
			WithYes(yes).
			WithInteractiveStdin(signals.StdinInteractive)
	}
	return NewPlainPresenter(stdout, stderr).
		WithYes(yes).
		WithInteractiveStdin(signals.StdinInteractive)
}
