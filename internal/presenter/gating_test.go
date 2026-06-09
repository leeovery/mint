package presenter_test

import (
	"os"
	"testing"

	"mint/internal/presenter"
)

// TestStdinIsInteractiveMirrorsSignal covers the pure gating-input core: the
// stdin-interactive decision is governed solely by the resolved stdin-TTY
// boolean, so it returns exactly that boolean. This is the GATING-INPUT axis,
// computed independently of the stdout/render-mode axis.
func TestStdinIsInteractiveMirrorsSignal(t *testing.T) {
	tests := []struct {
		name       string
		isStdinTTY bool
		expected   bool
	}{
		{name: "stdin tty is interactive", isStdinTTY: true, expected: true},
		{name: "stdin non-tty is not interactive", isStdinTTY: false, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := presenter.StdinIsInteractive(tt.isStdinTTY); got != tt.expected {
				t.Errorf("StdinIsInteractive(%v) = %v, want %v", tt.isStdinTTY, got, tt.expected)
			}
		})
	}
}

// TestAxesAreIndependent proves the stdout/render-mode axis (SelectMode) and the
// stdin/gating axis (StdinIsInteractive) are computed independently across all
// four TTY combinations: the render Mode is derived only from the stdout signal
// and the interactive flag only from the stdin signal — neither is derived from
// the other.
func TestAxesAreIndependent(t *testing.T) {
	tests := []struct {
		name            string
		isStdoutTTY     bool
		isStdinTTY      bool
		expectedMode    presenter.Mode
		expectedGateInt bool
	}{
		{
			name:            "stdin non-tty while stdout tty: pretty render, non-interactive gating",
			isStdoutTTY:     true,
			isStdinTTY:      false,
			expectedMode:    presenter.ModePretty,
			expectedGateInt: false,
		},
		{
			name:            "stdout non-tty while stdin tty: plain render, interactive gating",
			isStdoutTTY:     false,
			isStdinTTY:      true,
			expectedMode:    presenter.ModePlain,
			expectedGateInt: true,
		},
		{
			name:            "both non-tty: plain render, non-interactive gating",
			isStdoutTTY:     false,
			isStdinTTY:      false,
			expectedMode:    presenter.ModePlain,
			expectedGateInt: false,
		},
		{
			name:            "both tty: pretty render, interactive gating",
			isStdoutTTY:     true,
			isStdinTTY:      true,
			expectedMode:    presenter.ModePretty,
			expectedGateInt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := presenter.SelectMode(false, tt.isStdoutTTY); got != tt.expectedMode {
				t.Errorf("SelectMode(false, %v) = %v, want %v", tt.isStdoutTTY, got, tt.expectedMode)
			}
			if got := presenter.StdinIsInteractive(tt.isStdinTTY); got != tt.expectedGateInt {
				t.Errorf("StdinIsInteractive(%v) = %v, want %v", tt.isStdinTTY, got, tt.expectedGateInt)
			}
		})
	}
}

// TestStdinIsInteractiveIgnoresEnvironment asserts the no-sniffing ban for the
// gating axis: setting the variables a sniffing implementation would consult must
// not change the interactive decision for either stdin-TTY value. The decision is
// governed solely by the argument.
func TestStdinIsInteractiveIgnoresEnvironment(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	t.Setenv("TERM", "xterm")
	t.Setenv("CI", "true")
	t.Setenv("NO_COLOR", "1")

	if got := presenter.StdinIsInteractive(true); got != true {
		t.Errorf("StdinIsInteractive(true) with env set = %v, want true", got)
	}
	if got := presenter.StdinIsInteractive(false); got != false {
		t.Errorf("StdinIsInteractive(false) with env set = %v, want false", got)
	}
}

// TestDetectStdinTTYOnNonTTY confirms the startup wiring reuses the same
// IsTerminal primitive against the stdin descriptor: a real but non-character
// device (/dev/null) is reported as a non-TTY stdin, deterministically and
// regardless of the test runner's actual stdin.
func TestDetectStdinTTYOnNonTTY(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if presenter.DetectStdinTTY(f) {
		t.Errorf("DetectStdinTTY(%s) = true, want false", os.DevNull)
	}
}

// TestDetectStdinTTYIgnoresEnvironment asserts the wiring's no-sniffing ban end
// to end: with the sniffable env vars set, a non-TTY stdin (/dev/null) is still
// reported non-interactive — the environment does not influence the stdin axis.
func TestDetectStdinTTYIgnoresEnvironment(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	t.Setenv("TERM", "xterm")
	t.Setenv("CI", "true")
	t.Setenv("NO_COLOR", "1")

	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if presenter.DetectStdinTTY(f) {
		t.Errorf("DetectStdinTTY(%s) with env set = true, want false", os.DevNull)
	}
}

// TestDetectStartupSignalsResolvesBothAxesIndependently proves the two signals are
// resolved once at startup from their OWN descriptors and carried as distinct
// fields: Mode comes from stdout, StdinInteractive from stdin. Driving both from
// the same /dev/null handle (a known non-TTY) yields ModePlain AND
// StdinInteractive=false — and the StdinInteractive field equals
// StdinIsInteractive(DetectStdinTTY(stdin)), confirming neither field is derived
// from the other.
func TestDetectStartupSignalsResolvesBothAxesIndependently(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	sig := presenter.DetectStartupSignals(false, f, f)

	if sig.Mode != presenter.ModePlain {
		t.Errorf("DetectStartupSignals Mode = %v, want %v", sig.Mode, presenter.ModePlain)
	}
	if sig.StdinInteractive {
		t.Errorf("DetectStartupSignals StdinInteractive = true, want false")
	}
	if want := presenter.StdinIsInteractive(presenter.DetectStdinTTY(f)); sig.StdinInteractive != want {
		t.Errorf("DetectStartupSignals StdinInteractive = %v, want %v (must equal the stdin-axis resolution)", sig.StdinInteractive, want)
	}
}
