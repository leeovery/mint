package presenter

import "os"

// The GATING-INPUT axis lives here, deliberately separate from the stdout
// render-mode axis in mode.go. The two signals are ORTHOGONAL and resolved
// independently at startup: Mode is derived from the stdout TTY (SelectMode /
// DetectMode), while the stdin-interactive boolean below is derived from the
// stdin TTY. Neither is computed from the other — gating asks "is stdin a TTY?",
// render mode asks "is stdout a TTY?", and they are threaded through startup as
// two distinct inputs.
//
// Same no-sniffing ban as Phase 1: this axis reads NO LANG, LC_*, TERM, CI, or
// NO_COLOR. The only signal is the OS-reported stream type via the shared
// IsTerminal primitive (see mode.go) — there is no second/duplicate TTY
// mechanism, and the environment is never guessed from.

// StdinIsInteractive is the pure gating-input core, mirroring SelectMode's shape.
// It takes the already-resolved isStdinTTY boolean (so it is trivially
// unit-testable without a real terminal device) and reports whether interactive
// gating is possible: a TTY stdin can host a prompt, a non-TTY cannot. It returns
// exactly the resolved signal — the gating decision (consumed by the
// forbidden-combination rule and the prompt path) reads THIS value, never the
// stdout-derived Mode.
func StdinIsInteractive(isStdinTTY bool) bool {
	return isStdinTTY
}

// DetectStdinTTY is the thin startup wiring for the gating axis: it resolves the
// TTY signal from the real stdin handle using the SAME IsTerminal primitive that
// mode.go applies to stdout — there is no separate TTY mechanism, only the
// descriptor differs. Callers pass the returned boolean (or its StdinIsInteractive
// result) downstream as the gating signal, kept distinct from the render Mode.
func DetectStdinTTY(stdin *os.File) bool {
	return IsTerminal(stdin)
}

// StartupSignals is the once-at-startup resolution of the two ORTHOGONAL axes,
// carried as two distinct fields so downstream code threads them separately and
// nothing re-derives one from the other. Mode is the stdout render-mode signal;
// StdinInteractive is the stdin gating signal. Keeping them as separate fields on
// one value (rather than a single conflated flag) is what enforces the design's
// backbone: the gating path reads StdinInteractive, the render path reads Mode.
type StartupSignals struct {
	// Mode is the render mode selected from the stdout descriptor (and --plain).
	Mode Mode
	// StdinInteractive reports whether stdin can host an interactive gate, resolved
	// from the stdin descriptor — never from Mode.
	StdinInteractive bool
}

// DetectStartupSignals resolves BOTH axes once, each from its own descriptor:
// Mode from stdout (via DetectMode, honouring plainFlag) and StdinInteractive from
// stdin (via DetectStdinTTY → StdinIsInteractive). The two probes are independent
// — stdout never feeds the stdin field and vice versa — so all four TTY
// combinations are representable. Neither path sniffs the environment (same ban as
// Phase 1). Handles are taken as parameters (not the os globals) to keep this
// unit-testable with /dev/null.
//
// It is consumed at the converged startup seam (NewForStartup), which threads
// signals.Mode onto the render path and signals.StdinInteractive onto the gating
// field — so the forbidden-combination fail-loud path (non-TTY stdin without -y)
// is reachable from the one production construction site.
func DetectStartupSignals(plainFlag bool, stdout, stdin *os.File) StartupSignals {
	return StartupSignals{
		Mode:             DetectMode(plainFlag, stdout),
		StdinInteractive: StdinIsInteractive(DetectStdinTTY(stdin)),
	}
}
