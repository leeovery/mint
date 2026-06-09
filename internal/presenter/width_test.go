package presenter

import (
	"os"
	"testing"
)

// TestRuleWidthCapsAtMinTerminalWidthAndCap is the pure-core acceptance for the
// decorative-rule width source: ruleWidth caps the rule at min(terminalWidth,
// ruleCap) and falls back to the cap whenever the width is undetectable (≤ 0).
// It is a white-box test (package presenter) because ruleWidth is unexported and
// deliberately pure — testable without a real terminal device, mirroring the
// pure SelectMode/StdinIsInteractive cores in this package.
func TestRuleWidthCapsAtMinTerminalWidthAndCap(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		want      int
	}{
		{name: "narrower than cap uses the terminal width", termWidth: 30, want: 30},
		{name: "wider than cap clamps to the cap", termWidth: 200, want: ruleCap},
		{name: "exactly the cap stays at the cap", termWidth: ruleCap, want: ruleCap},
		{name: "zero (undetectable) falls back to the cap", termWidth: 0, want: ruleCap},
		{name: "negative (undetectable) falls back to the cap", termWidth: -1, want: ruleCap},
		{name: "tiny width yields a tiny rule via the same min", termWidth: 3, want: 3},
		{name: "one is the smallest meaningful width", termWidth: 1, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ruleWidth(tt.termWidth); got != tt.want {
				t.Errorf("ruleWidth(%d) = %d, want %d", tt.termWidth, got, tt.want)
			}
		})
	}
}

// TestRuleCapValue documents the chosen cap value so a change to the constant is
// a deliberate, test-visible decision rather than a silent drift.
func TestRuleCapValue(t *testing.T) {
	if ruleCap != 50 {
		t.Errorf("ruleCap = %d, want 50 (the documented decorative-rule cap)", ruleCap)
	}
}

// TestDetectTermWidthOnNonTTYReturnsZero confirms the OS width probe returns the 0
// sentinel for a non-terminal *os.File — term.GetSize errors on /dev/null, which
// detectTermWidth maps to 0 so ruleWidth falls back to the cap. /dev/null is a
// reliable, CI-safe non-TTY handle, keeping this deterministic regardless of the
// test runner's own terminal. Pairing detectTermWidth(0) → ruleWidth → ruleCap is
// the documented undetectable-width fallback.
func TestDetectTermWidthOnNonTTYReturnsZero(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if got := detectTermWidth(f); got != 0 {
		t.Errorf("detectTermWidth(%s) = %d, want 0 (undetectable sentinel)", os.DevNull, got)
	}
	// And the 0 sentinel flows through the pure core to the cap.
	if got := ruleWidth(detectTermWidth(f)); got != ruleCap {
		t.Errorf("ruleWidth(detectTermWidth(%s)) = %d, want %d (cap fallback)", os.DevNull, got, ruleCap)
	}
}
