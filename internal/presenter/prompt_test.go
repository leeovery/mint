package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// drivePlainPrompt constructs a PlainPresenter with the supplied input script and
// captures its narration, returning the choice and the captured out buffer from a
// single Prompt call against the given gate, plus any error. Centralising
// construction keeps each prompt test focused on the input script and the asserted
// outcome.
func drivePlainPrompt(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	p := presenter.NewPlainPresenterWithInput(out, errBuf, strings.NewReader(input))
	choice, err := p.Prompt(gate)
	return choice, out, err
}

// drivePrettyPrompt mirrors drivePlainPrompt for the pretty presenter, forcing the
// ASCII profile so the captured prompt text is deterministic regardless of the
// test runner's own TTY.
func drivePrettyPrompt(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii), presenter.WithInput(strings.NewReader(input)))
	choice, err := p.Prompt(gate)
	return choice, out, err
}

// promptModeDriver pairs a render-mode name with the matching single-Prompt driver
// (drivePlainPrompt / drivePrettyPrompt). It mirrors promptDriver in
// prompt_render_only_test.go but reuses the prompt-test drivers, which force the
// Ascii profile so captured prompt text stays deterministic.
type promptModeDriver struct {
	mode string
	run  func(input string, gate presenter.Gate) (presenter.Choice, *bytes.Buffer, error)
}

// promptModeDrivers returns one driver per render mode so a genuinely
// mode-invariant Prompt property is asserted ONCE inside t.Run(d.mode, ...) rather
// than as two hand-duplicated plain-then-pretty arms that can drift independently.
// Mode-SPECIFIC rendering (e.g. the plain [y/n/e/r] hint vs the pretty menu) stays
// in its own dedicated test and is NOT driven through this table.
func promptModeDrivers() []promptModeDriver {
	return []promptModeDriver{
		{mode: "plain", run: drivePlainPrompt},
		{mode: "pretty", run: drivePrettyPrompt},
	}
}

// TestPromptEmptyEnterSelectsDefault locks the empty-Enter contract: a blank line
// returns the gate's declared Default (y for NotesReviewGate) — the deliberate
// accept path. Mode-invariant, so asserted once per mode via the driver table.
func TestPromptEmptyEnterSelectsDefault(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, _, err := d.run("\n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt empty-Enter = %q, want %q", d.mode, choice, presenter.ChoiceYes)
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
	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			for _, tt := range tests {
				choice, _, err := d.run(tt.input, gate)
				if err != nil {
					t.Fatalf("%s Prompt(%q) returned error: %v", d.mode, tt.input, err)
				}
				if choice != tt.want {
					t.Errorf("%s Prompt(%q) = %q, want %q", d.mode, tt.input, choice, tt.want)
				}
			}
		})
	}
}

// TestPromptUnrecognisedReprompts proves an unrecognised key (x) re-prompts and a
// subsequent valid line is accepted — and that the prompt was rendered TWICE (once
// for the initial read, once for the re-prompt). Both the returned choice and the
// "Continue?" question count are mode-invariant (the question text appears in both
// the plain hint and the pretty menu), so asserted once per mode via the table.
func TestPromptUnrecognisedReprompts(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, out, err := d.run("x\nn\n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceNo {
				t.Errorf("%s Prompt = %q, want %q", d.mode, choice, presenter.ChoiceNo)
			}
			if got := strings.Count(out.String(), "Continue?"); got != 2 {
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

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, _, err := d.run("a\nq\ny\n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt = %q, want %q (a/q must re-prompt, never be returned)", d.mode, choice, presenter.ChoiceYes)
			}
		})
	}
}

// TestPromptWhitespaceOnlyTreatedAsDefault locks the documented whitespace-only
// rule: a line of only spaces trims to empty -> empty-Enter -> the Default.
// Mode-invariant, so asserted once per mode via the driver table.
func TestPromptWhitespaceOnlyTreatedAsDefault(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, _, err := d.run("   \n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt whitespace-only = %q, want %q (treated as empty-Enter default)", d.mode, choice, presenter.ChoiceYes)
			}
		})
	}
}

// TestPromptRepeatedUnrecognisedThenValid proves repeated unrecognised lines keep
// re-prompting until a valid line: x, z, ? then r returns ChoiceRegen after three
// re-prompts (prompt rendered four times total). Both the choice and the
// "Continue?" count are mode-invariant, so asserted once per mode via the table.
func TestPromptRepeatedUnrecognisedThenValid(t *testing.T) {
	gate := presenter.NotesReviewGate()

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, out, err := d.run("x\nz\n?\nr\n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceRegen {
				t.Errorf("%s Prompt = %q, want %q", d.mode, choice, presenter.ChoiceRegen)
			}
			if got := strings.Count(out.String(), "Continue?"); got != 4 {
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

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			// "x" with no trailing newline then EOF: the only line is unrecognised,
			// then the reader hits EOF — must error, must NOT return the default.
			choice, _, err := d.run("x", gate)
			if err == nil {
				t.Fatalf("%s Prompt returned nil error on EOF, want non-nil (choice was %q)", d.mode, choice)
			}
			if choice == gate.Default {
				t.Errorf("%s Prompt default-accepted %q on EOF; must not silently default", d.mode, choice)
			}

			// Immediate EOF (empty input, no line at all) must also error — not be
			// mistaken for a deliberate empty-Enter default.
			if _, _, err := d.run("", gate); err == nil {
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
		plainChoice, _, plainErr := drivePlainPrompt(tt.input, gate)
		prettyChoice, _, prettyErr := drivePrettyPrompt(tt.input, gate)

		if plainErr != nil || prettyErr != nil {
			t.Fatalf("input %q: plain err=%v pretty err=%v", tt.input, plainErr, prettyErr)
		}
		if plainChoice != tt.want || prettyChoice != tt.want {
			t.Errorf("input %q: plain=%q pretty=%q, want %q (modes must agree)", tt.input, plainChoice, prettyChoice, tt.want)
		}
	}
}

// TestPromptHonoursReuseGateDeclaredSet proves the loop reads the gate's declared
// set, not a global y/n/e/r: the two-choice ReuseConfirmGate does NOT declare e,
// so 'e' re-prompts, then 'y' is accepted. Mode-invariant, so asserted once per
// mode via the driver table.
func TestPromptHonoursReuseGateDeclaredSet(t *testing.T) {
	gate := presenter.ReuseConfirmGate()

	for _, d := range promptModeDrivers() {
		t.Run(d.mode, func(t *testing.T) {
			choice, out, err := d.run("e\ny\n", gate)
			if err != nil {
				t.Fatalf("%s Prompt returned error: %v", d.mode, err)
			}
			if choice != presenter.ChoiceYes {
				t.Errorf("%s Prompt = %q, want %q ('e' is not declared by reuse gate -> re-prompt)", d.mode, choice, presenter.ChoiceYes)
			}
			if got := strings.Count(out.String(), "Continue?"); got != 2 {
				t.Errorf("%s prompt rendered %d times, want 2 ('e' re-prompts)", d.mode, got)
			}
		})
	}
}

// TestPromptHintUsesDeclaredKeys proves the prompt hint is built from the gate's
// declared keys, not a hardcoded y/n/e/r literal: the four-choice gate shows
// [y/n/e/r] while the two-choice reuse gate shows only [y/n].
func TestPromptHintUsesDeclaredKeys(t *testing.T) {
	_, out, _ := drivePlainPrompt("\n", presenter.NotesReviewGate())
	if !strings.Contains(out.String(), "[y/n/e/r]") {
		t.Errorf("plain notes-review hint = %q, want it to contain [y/n/e/r]", out.String())
	}

	_, out, _ = drivePlainPrompt("\n", presenter.ReuseConfirmGate())
	if !strings.Contains(out.String(), "[y/n]") {
		t.Errorf("plain reuse hint = %q, want it to contain [y/n]", out.String())
	}
	if strings.Contains(out.String(), "[y/n/e/r]") {
		t.Errorf("plain reuse hint = %q, must NOT show e/r the gate does not declare", out.String())
	}
}
