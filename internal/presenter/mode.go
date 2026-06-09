package presenter

import (
	"os"

	"golang.org/x/term"
)

// Mode is the render mode selected once at startup and passed down unchanged.
// Nothing downstream re-derives it: the engine and presenters consume the
// already-chosen Mode rather than re-checking the flag or the TTY.
type Mode int

const (
	// ModePlain renders terse, token-efficient text — no ANSI, animation, or
	// banner. It is the zero value so an unset Mode safely defaults to the
	// agent-friendly, side-effect-free rendering.
	ModePlain Mode = iota
	// ModePretty renders styled output — brand line, colour, spinners.
	ModePretty
)

// String renders the mode for readable diagnostics and test output.
func (m Mode) String() string {
	switch m {
	case ModePretty:
		return "pretty"
	case ModePlain:
		return "plain"
	default:
		return "unknown"
	}
}

// SelectMode is the pure decision core. It applies the fixed precedence —
// plainFlag forces ModePlain; otherwise an attached TTY selects ModePretty and a
// non-TTY falls back to ModePlain. It takes the already-resolved isTTY boolean
// so it is trivially unit-testable without a real terminal device.
//
// No environment sniffing: this function deliberately reads no LANG, LC_*, TERM,
// CI, or NO_COLOR variable. A flag is an explicit instruction; the ban is on
// guessing render mode from the environment. The only TTY signal is the caller's
// resolved isTTY (produced by IsTerminal), and NO_COLOR is out of scope — passing
// --plain is its explicit equivalent.
func SelectMode(plainFlag bool, isTTY bool) Mode {
	if plainFlag {
		return ModePlain
	}
	if isTTY {
		return ModePretty
	}
	return ModePlain
}

// IsTerminal reports whether f is an interactive terminal, using the OS-reported
// stream type via golang.org/x/term — the spec's stated equivalent of
// os.Stdout.Stat().Mode()&os.ModeCharDevice != 0. This is the sole TTY signal
// feeding mode selection; it consults no environment variable. In particular it
// does NOT special-case TERM (so a colour-incapable but real TTY such as
// TERM=dumb is still a terminal here — colour downgrade is lipgloss's job, not
// the mode selector's).
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// DetectMode is the thin startup wiring: it resolves the TTY signal from the
// real stdout handle and feeds it to the pure SelectMode core. This is the
// single call site where the mode is chosen — callers pass the returned Mode
// downstream and nothing re-detects the flag or the TTY.
func DetectMode(plainFlag bool, stdout *os.File) Mode {
	return SelectMode(plainFlag, IsTerminal(stdout))
}
