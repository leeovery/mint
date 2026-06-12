package presenter_test

import (
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// The prompt tests drive a single Prompt through the shared gateDrivers() table
// (see gate_helpers_test.go), selecting termenv.Ascii so captured prompt text is
// deterministic regardless of the test runner's own TTY. d.prompt(...) is the
// string-scripted, default-options convenience over the canonical gateDriver.run.

// TestPromptEmptyEnterSelectsDefault locks the empty-Enter contract: a blank line
// returns the gate's declared Default (y for NotesReviewGate) — the deliberate
// accept path. Mode-invariant, so asserted once per mode via the driver table.
func TestPromptEmptyEnterSelectsDefault(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "\n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt empty-Enter = %q, want %q", d.mode, res.choice, presenter.ChoiceYes)
			}
		})
	}
}

// TestPromptIsCaseInsensitive proves uppercase input maps to the declared
// lowercase key: N -> n, E -> e, in both render modes. The mapping is
// mode-invariant, so each case is asserted once per mode via the driver table.
func TestPromptIsCaseInsensitive(t *testing.T) {
	gate := presenter.NotesReviewGate()

	tests := []struct {
		input string
		want  presenter.Choice
	}{
		{"N\n", presenter.ChoiceNo},
		{"E\n", presenter.ChoiceEdit},
		{"Y\n", presenter.ChoiceYes},
		{"R\n", presenter.ChoiceRegen},
	}
	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			for _, tt := range tests {
				res := d.prompt(termenv.Ascii, tt.input, gate)
				if res.err != nil {
					t.Fatalf("%s Prompt(%q) returned error: %v", d.mode, tt.input, res.err)
				}
				if res.choice != tt.want {
					t.Errorf("%s Prompt(%q) = %q, want %q", d.mode, tt.input, res.choice, tt.want)
				}
			}
		})
	}
}

// TestPromptUnrecognisedReprompts proves an unrecognised key (x) re-prompts and a
// subsequent valid line is accepted — and that the prompt was rendered TWICE (once
// for the initial read, once for the re-prompt). The returned choice is
// mode-invariant; the render count is asserted through the per-mode renderCount
// marker (plain's "Continue?" question, pretty's bar cursor), once per mode via
// the table.
func TestPromptUnrecognisedReprompts(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "x\nn\n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceNo {
				t.Errorf("%s Prompt = %q, want %q", d.mode, res.choice, presenter.ChoiceNo)
			}
			if got := d.renderCount(res.out.String()); got != 2 {
				t.Errorf("%s prompt rendered %d times, want 2 (initial + re-prompt)", d.mode, got)
			}
		})
	}
}

// TestPromptOldMuscleMemoryKeysReprompt proves the stale engine keys a and q —
// superseded by the default-yes Continue? gate — are NOT declared by the
// notes-review gate, so they re-prompt and are NEVER returned; a final y accepts.
// Mode-invariant, so asserted once per mode via the driver table.
func TestPromptOldMuscleMemoryKeysReprompt(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "a\nq\ny\n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt = %q, want %q (a/q must re-prompt, never be returned)", d.mode, res.choice, presenter.ChoiceYes)
			}
		})
	}
}

// TestPromptWhitespaceOnlyTreatedAsDefault locks the documented whitespace-only
// rule: a line of only spaces trims to empty -> empty-Enter -> the Default.
// Mode-invariant, so asserted once per mode via the driver table.
func TestPromptWhitespaceOnlyTreatedAsDefault(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "   \n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt whitespace-only = %q, want %q (treated as empty-Enter default)", d.mode, res.choice, presenter.ChoiceYes)
			}
		})
	}
}

// TestPromptRepeatedUnrecognisedThenValid proves repeated unrecognised lines keep
// re-prompting until a valid line: x, z, ? then r returns ChoiceRegen after three
// re-prompts (prompt rendered four times total). The choice is mode-invariant and
// the render count uses the per-mode renderCount marker, asserted once per mode
// via the table.
func TestPromptRepeatedUnrecognisedThenValid(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "x\nz\n?\nr\n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceRegen {
				t.Errorf("%s Prompt = %q, want %q", d.mode, res.choice, presenter.ChoiceRegen)
			}
			if got := d.renderCount(res.out.String()); got != 4 {
				t.Errorf("%s prompt rendered %d times, want 4 (initial + three re-prompts)", d.mode, got)
			}
		})
	}
}

// TestPromptEOFReturnsError proves EOF on input with no usable valid line returns
// a non-nil error and does NOT silently default-accept — the underpinning of the
// fail-loud, unattended-without-y behaviour. Both EOF properties are
// mode-invariant, so asserted once per mode via the driver table.
func TestPromptEOFReturnsError(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			// "x" with no trailing newline then EOF: the only line is unrecognised,
			// then the reader hits EOF — must error, must NOT return the default.
			res := d.prompt(termenv.Ascii, "x", gate)
			if res.err == nil {
				t.Fatalf("%s Prompt returned nil error on EOF, want non-nil (choice was %q)", d.mode, res.choice)
			}
			if res.choice == gate.Default {
				t.Errorf("%s Prompt default-accepted %q on EOF; must not silently default", d.mode, res.choice)
			}

			// Immediate EOF (empty input, no line at all) must also error — not be
			// mistaken for a deliberate empty-Enter default.
			if eof := d.prompt(termenv.Ascii, "", gate); eof.err == nil {
				t.Errorf("%s Prompt returned nil error on immediate EOF, want non-nil", d.mode)
			}
		})
	}
}

// TestPromptCoreIsModeAgnostic proves the parse/loop core is shared: the same
// input sequence yields the same Choice for BOTH plain and pretty across a range
// of scripts (empty, uppercase, re-prompt-then-valid, whitespace).
func TestPromptCoreIsModeAgnostic(t *testing.T) {
	gate := presenter.NotesReviewGate()

	tests := []struct {
		input string
		want  presenter.Choice
	}{
		{"\n", presenter.ChoiceYes},
		{"N\n", presenter.ChoiceNo},
		{"x\nr\n", presenter.ChoiceRegen},
		{"   \n", presenter.ChoiceYes},
		{"a\nq\ne\n", presenter.ChoiceEdit},
	}
	for _, tt := range tests {
		// Drive the SAME input through both real presenters via the shared table and
		// require every mode to land on the same want — the modes must agree.
		for _, d := range gateDrivers() {
			res := d.prompt(termenv.Ascii, tt.input, gate)
			if res.err != nil {
				t.Fatalf("input %q: %s err=%v", tt.input, d.mode, res.err)
			}
			if res.choice != tt.want {
				t.Errorf("input %q: %s=%q, want %q (modes must agree)", tt.input, d.mode, res.choice, tt.want)
			}
		}
	}
}

// TestPromptHonoursReuseGateDeclaredSet proves the loop reads the gate's declared
// set, not a global y/n/e/r: the two-choice ReuseConfirmGate does NOT declare e,
// so 'e' re-prompts, then 'y' is accepted. Mode-invariant, so asserted once per
// mode via the driver table.
func TestPromptHonoursReuseGateDeclaredSet(t *testing.T) {
	gate := presenter.ReuseConfirmGate()

	for _, d := range gateDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			res := d.prompt(termenv.Ascii, "e\ny\n", gate)
			if res.err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, res.err)
			}
			if res.choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt = %q, want %q ('e' is not declared by reuse gate -> re-prompt)", d.mode, res.choice, presenter.ChoiceYes)
			}
			if got := d.renderCount(res.out.String()); got != 2 {
				t.Errorf("%s prompt rendered %d times, want 2 ('e' re-prompts)", d.mode, got)
			}
		})
	}
}

// TestPromptHintUsesDeclaredKeys proves the prompt hint is built from the gate's
// declared keys, not a hardcoded y/n/e/r literal: the four-choice gate shows
// [y/n/e/r] while the two-choice reuse gate shows only [y/n].
func TestPromptHintUsesDeclaredKeys(t *testing.T) {
	// The [y/n/e/r] hint is plain-mode-SPECIFIC rendering, so this builds the plain
	// presenter directly via the shared plainGate construction seam.
	notes, out, _ := plainGate(strings.NewReader("\n"), gateOpts{})
	_, _ = notes.Prompt(presenter.NotesReviewGate())
	if !strings.Contains(out.String(), "[y/n/e/r]") {
		t.Errorf("plain notes-review hint = %q, want it to contain [y/n/e/r]", out.String())
	}

	reuse, reuseOut, _ := plainGate(strings.NewReader("\n"), gateOpts{})
	_, _ = reuse.Prompt(presenter.ReuseConfirmGate())
	if !strings.Contains(reuseOut.String(), "[y/n]") {
		t.Errorf("plain reuse hint = %q, want it to contain [y/n]", reuseOut.String())
	}
	if strings.Contains(reuseOut.String(), "[y/n/e/r]") {
		t.Errorf("plain reuse hint = %q, must NOT show e/r the gate does not declare", reuseOut.String())
	}
}
