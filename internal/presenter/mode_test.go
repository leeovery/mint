package presenter_test

import (
	"os"
	"testing"

	"mint/internal/presenter"
)

// TestSelectModeAppliesPrecedence covers the pure decision core across every
// (plainFlag, isTTY) combination: --plain wins outright, otherwise the TTY
// signal decides, with non-TTY falling back to plain.
func TestSelectModeAppliesPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		plainFlag bool
		isTTY     bool
		expected  presenter.Mode
	}{
		{
			name:      "plain flag on a tty still selects plain",
			plainFlag: true,
			isTTY:     true,
			expected:  presenter.ModePlain,
		},
		{
			name:      "plain flag on a non-tty selects plain",
			plainFlag: true,
			isTTY:     false,
			expected:  presenter.ModePlain,
		},
		{
			name:      "tty without plain flag selects pretty",
			plainFlag: false,
			isTTY:     true,
			expected:  presenter.ModePretty,
		},
		{
			name:      "piped non-tty without plain flag selects plain",
			plainFlag: false,
			isTTY:     false,
			expected:  presenter.ModePlain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := presenter.SelectMode(tt.plainFlag, tt.isTTY)
			if got != tt.expected {
				t.Errorf("SelectMode(%v, %v) = %v, want %v", tt.plainFlag, tt.isTTY, got, tt.expected)
			}
		})
	}
}

// TestSelectModeIgnoresEnvironment asserts the no-sniffing ban: setting the
// environment variables a sniffing implementation would consult must not change
// the selected mode for any (plainFlag, isTTY) combination. The decision is
// governed solely by the arguments.
func TestSelectModeIgnoresEnvironment(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	t.Setenv("TERM", "xterm")
	t.Setenv("CI", "true")
	t.Setenv("NO_COLOR", "1")

	tests := []struct {
		name      string
		plainFlag bool
		isTTY     bool
		expected  presenter.Mode
	}{
		{name: "plain flag on a tty", plainFlag: true, isTTY: true, expected: presenter.ModePlain},
		{name: "plain flag on a non-tty", plainFlag: true, isTTY: false, expected: presenter.ModePlain},
		{name: "tty without plain flag", plainFlag: false, isTTY: true, expected: presenter.ModePretty},
		{name: "non-tty without plain flag", plainFlag: false, isTTY: false, expected: presenter.ModePlain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := presenter.SelectMode(tt.plainFlag, tt.isTTY)
			if got != tt.expected {
				t.Errorf("SelectMode(%v, %v) with env set = %v, want %v", tt.plainFlag, tt.isTTY, got, tt.expected)
			}
		})
	}
}

// TestIsTerminalOnNonTTY confirms IsTerminal reports false for a real but
// non-character-device *os.File. /dev/null is a reliable, CI-safe non-TTY
// handle, so this stays deterministic regardless of the test runner's stdout.
func TestIsTerminalOnNonTTY(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if presenter.IsTerminal(f) {
		t.Errorf("IsTerminal(%s) = true, want false", os.DevNull)
	}
}

// TestDetectModeOnNonTTYSelectsPlain wires the OS probe through to SelectMode:
// a known non-TTY stdout (/dev/null) selects plain regardless of the plain flag.
func TestDetectModeOnNonTTYSelectsPlain(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if got := presenter.DetectMode(false, f); got != presenter.ModePlain {
		t.Errorf("DetectMode(false, %s) = %v, want %v", os.DevNull, got, presenter.ModePlain)
	}
	if got := presenter.DetectMode(true, f); got != presenter.ModePlain {
		t.Errorf("DetectMode(true, %s) = %v, want %v", os.DevNull, got, presenter.ModePlain)
	}
}

// TestDetectModeIgnoresEnvironment asserts DetectMode's no-sniffing ban end to
// end: with the sniffable env vars set, a non-TTY stdout still selects plain and
// the plain flag still selects plain — the env does not influence the decision.
func TestDetectModeIgnoresEnvironment(t *testing.T) {
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

	if got := presenter.DetectMode(false, f); got != presenter.ModePlain {
		t.Errorf("DetectMode(false, %s) with env set = %v, want %v", os.DevNull, got, presenter.ModePlain)
	}
	if got := presenter.DetectMode(true, f); got != presenter.ModePlain {
		t.Errorf("DetectMode(true, %s) with env set = %v, want %v", os.DevNull, got, presenter.ModePlain)
	}
}
