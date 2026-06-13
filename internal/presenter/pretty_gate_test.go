package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// drivePrettyPromptProfile is the single pretty-prompt single-Prompt driver. It
// builds the pretty presenter via the shared prettyGate construction seam with the
// colour profile under test, scripts a single Prompt from input, and returns the
// captured out buffer. The scripted strings.Reader is NOT a terminal, so the
// Prompt takes the shared LINE-read loop — the raw single-keypress path needs a
// real TTY and is exercised live, not here. Forcing the profile (Ascii for
// deterministic plain text, TrueColor where ANSI must be present) keeps the
// bar-structure-survives-downgrade and colour-on-keeps-structure assertions
// deterministic regardless of the test runner's own TTY.
func drivePrettyPromptProfile(input string, gate presenter.Gate, profile termenv.Profile) *bytes.Buffer {
	p, out, _ := prettyGate(profile, strings.NewReader(input), gateOpts{})
	_, _ = p.Prompt(gate)
	return out
}

// notesBar is the Ascii render of the NotesReviewGate hotkey bar: a leading blank
// line, then the Question verbatim on its own line, then a line of every declared
// "[key] action" pair in order joined by TWO spaces, then a final line carrying the
// "› " cursor ALONE. NO trailing newline. The bar is QUESTION-led: the gate's
// Subject is NOT rendered (it only feeds the -y echo).
const notesBar = "\nUse these notes?\n[y] accept  [n] abort  [e] edit  [r] regenerate\n› "

// gateNoSubject is a two-choice gate WITHOUT a Subject, used to prove the bar
// renders the Question verbatim — the Subject plays no part in the bar.
func gateNoSubject() presenter.Gate {
	return presenter.Gate{
		Question: "Continue?",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept"},
			{Key: presenter.ChoiceNo, Action: "abort"},
		},
		Default: presenter.ChoiceNo,
	}
}

// TestPrettyGateRendersHotkeyBar locks the bar layout: a leading blank line, the
// Question verbatim on its own line, then every declared key/action pair in
// declared order joined by two-space separators, then the "› " cursor ALONE on a
// final line — with NO stacked option lines.
func TestPrettyGateRendersHotkeyBar(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, notesBar) {
		t.Fatalf("hotkey bar not rendered:\nwant substring %q\ngot:\n%q", notesBar, got)
	}
	// The stacked-menu form is GONE: no indented option lines, no [default] tag.
	for _, stale := range []string{"\n    y", "\n    n", "[default]"} {
		if strings.Contains(got, stale) {
			t.Errorf("stacked-menu artefact %q must not render in the bar design:\n%q", stale, got)
		}
	}
}

// TestPrettyGateBarRendersQuestionVerbatim proves the bar leads with the gate's
// Question rendered VERBATIM — "?" intact, no Subject-derived label, no fallback
// munging — even on a Subject-less gate.
func TestPrettyGateBarRendersQuestionVerbatim(t *testing.T) {
	out := drivePrettyPromptProfile("n\n", gateNoSubject(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "\nContinue?\n[y] accept  [n] abort\n› ") {
		t.Errorf("Subject-less gate must lead the bar with the Question verbatim:\n%q", got)
	}
}

// TestPrettyGateTwoChoiceRendersOnlyDeclared proves the bar is built from the
// gate's declared set: the reuse confirm renders exactly y/n pairs and no e/r.
func TestPrettyGateTwoChoiceRendersOnlyDeclared(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.ReuseConfirmGate(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "\nUse these notes?\n[y] accept  [n] abort\n› ") {
		t.Errorf("reuse confirm bar missing:\n%q", got)
	}
	if strings.Contains(got, "[e]") || strings.Contains(got, "[r]") {
		t.Errorf("reuse confirm must NOT render e/r pairs (not declared):\n%q", got)
	}
}

// TestPrettyGateRedrawnAfterBadInput proves the bar is redrawn after
// unrecognised LINE input: with "x\ny\n" the bar renders TWICE (initial + the
// post-bad-input re-render) — the linear-scroll redraw with no screen-clearing.
func TestPrettyGateRedrawnAfterBadInput(t *testing.T) {
	out := drivePrettyPromptProfile("x\ny\n", presenter.NotesReviewGate(), termenv.Ascii)
	got := out.String()

	if n := strings.Count(got, notesBar); n != 2 {
		t.Errorf("bar rendered %d times, want 2 (initial + redraw after bad input):\n%q", n, got)
	}
}

// TestPrettyGateBarBuiltFromDeclaredChoices proves the bar derives from the gate's
// declared choices: changing the action labels and order changes the render (no
// hardcoded option list), and declared order survives.
func TestPrettyGateBarBuiltFromDeclaredChoices(t *testing.T) {
	gate := presenter.Gate{
		Question: "Proceed?",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceNo, Action: "stop here"},
			{Key: presenter.ChoiceYes, Action: "go go go"},
		},
		Default: presenter.ChoiceYes,
	}
	out := drivePrettyPromptProfile("y\n", gate, termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "\nProceed?\n[n] stop here  [y] go go go\n› ") {
		t.Errorf("custom gate bar not rendered from declared choices in declared order:\n%q", got)
	}
}

// TestPrettyGateDefaultMarkedByWeightOnly proves the default choice is marked by
// STYLE WEIGHT (its action stays plain while the others are dim), not by a
// "[default]" tag: under a colour profile, moving the default from y to n changes
// the render; under the Ascii downgrade the two renders are byte-identical (the
// marking is colour-only and the structure carries no tag).
func TestPrettyGateDefaultMarkedByWeightOnly(t *testing.T) {
	gateWithDefault := func(def presenter.Choice) presenter.Gate {
		return presenter.Gate{
			Question: "Continue?",
			Subject:  "notes",
			Choices: []presenter.GateChoice{
				{Key: presenter.ChoiceYes, Action: "accept"},
				{Key: presenter.ChoiceNo, Action: "abort"},
			},
			Default: def,
		}
	}

	colourY := drivePrettyPromptProfile("y\n", gateWithDefault(presenter.ChoiceYes), termenv.TrueColor).String()
	colourN := drivePrettyPromptProfile("y\n", gateWithDefault(presenter.ChoiceNo), termenv.TrueColor).String()
	if colourY == colourN {
		t.Errorf("under colour, the default's weight marking must track gate.Default (renders identical):\n%q", colourY)
	}

	asciiY := drivePrettyPromptProfile("y\n", gateWithDefault(presenter.ChoiceYes), termenv.Ascii).String()
	asciiN := drivePrettyPromptProfile("y\n", gateWithDefault(presenter.ChoiceNo), termenv.Ascii).String()
	if asciiY != asciiN {
		t.Errorf("under the Ascii downgrade the marking is colour-only — structures must be identical:\n%q\nvs\n%q", asciiY, asciiN)
	}
	if strings.Contains(asciiY, "[default]") {
		t.Errorf("the bar carries no [default] tag:\n%q", asciiY)
	}
}

// TestPrettyGateColourDowngradePreservesStructure forces the no-colour (Ascii)
// profile and asserts the bar structure — label, every key/action pair, the "› "
// cursor — survives as plain text with NO ESC (0x1b) byte.
func TestPrettyGateColourDowngradePreservesStructure(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.Ascii)

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded gate output contains an ESC (0x1b) — colour codes leaked:\n%q", out.String())
	}
	if !strings.Contains(out.String(), notesBar) {
		t.Errorf("bar structure missing under colour downgrade:\n%q", out.String())
	}
}

// TestPrettyGateColourOnEmitsANSIButKeepsStructure forces a colour-capable profile
// and asserts ANSI escapes ARE emitted while the bar structure (the question, every
// action word, the › cursor) still survives. Because lipgloss wraps styled spans
// in ANSI, the structure is asserted as substrings the styling does not split.
func TestPrettyGateColourOnEmitsANSIButKeepsStructure(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.TrueColor)

	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on gate output contains no ESC (0x1b) — expected ANSI codes:\n%q", out.String())
	}
	got := out.String()
	for _, want := range []string{"Use these notes?", "accept", "abort", "edit", "regenerate", "›"} {
		if !strings.Contains(got, want) {
			t.Errorf("bar structure %q missing under colour-on:\n%q", want, got)
		}
	}
}

// TestDeclineChoiceFindsChoiceNo proves the Escape-key decline maps to the gate's
// ChoiceNo when declared (the review gates) and reports none for an enumerated gate
// that declares no decline (where Escape is ignored instead).
func TestDeclineChoiceFindsChoiceNo(t *testing.T) {
	t.Parallel()

	if c, ok := presenter.DeclineChoiceForTest(presenter.NotesReviewGate()); !ok || c != presenter.ChoiceNo {
		t.Errorf("review gate decline = (%q, %v), want (ChoiceNo, true)", c, ok)
	}
	enum := presenter.Gate{
		Question: "Source?",
		Choices: []presenter.GateChoice{
			{Key: presenter.Choice("github"), Action: "github"},
			{Key: presenter.Choice("gitlab"), Action: "gitlab"},
		},
		Default: presenter.Choice("github"),
	}
	if _, ok := presenter.DeclineChoiceForTest(enum); ok {
		t.Error("enumerated gate with no ChoiceNo reported a decline; Escape should be ignored there")
	}
}
