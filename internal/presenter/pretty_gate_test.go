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
// captured out buffer. Forcing the profile (Ascii for deterministic plain text,
// TrueColor where ANSI must be present) keeps the menu-structure-survives-downgrade
// and colour-on-keeps-structure assertions deterministic regardless of the test
// runner's own TTY.
func drivePrettyPromptProfile(input string, gate presenter.Gate, profile termenv.Profile) *bytes.Buffer {
	p, out, _ := prettyGate(profile, strings.NewReader(input), gateOpts{})
	_, _ = p.Prompt(gate)
	return out
}

// gateNoDefault is a two-choice Continue? gate whose default is ChoiceNo, used to
// prove the [default] marker tracks gate.Default rather than always landing on the
// y line. It is a local fixture (not a package constructor) because no real gate
// defaults to abort — it exists only to exercise the non-y-default rendering.
func gateNoDefault() presenter.Gate {
	return presenter.Gate{
		Question: "Continue?",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "accept & proceed"},
			{Key: presenter.ChoiceNo, Action: "abort"},
		},
		Default: presenter.ChoiceNo,
	}
}

// TestPrettyGateRendersOptionsAboveQuestion locks the vertical-menu layout: the
// four option lines (y/n/e/r) render in declared order ABOVE the question, a blank
// line separates them, and the "  Continue? › " prompt line is LAST. Ordering is
// asserted by index so a future reflow cannot silently move the prompt above the
// options.
func TestPrettyGateRendersOptionsAboveQuestion(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.Ascii)
	got := out.String()

	yIdx := strings.Index(got, "    y  accept & proceed")
	nIdx := strings.Index(got, "    n  abort")
	eIdx := strings.Index(got, "    e  edit in $EDITOR")
	rIdx := strings.Index(got, "    r  regenerate")
	promptIdx := strings.Index(got, "  Continue? › ")

	for name, idx := range map[string]int{
		"y line": yIdx, "n line": nIdx, "e line": eIdx, "r line": rIdx, "prompt": promptIdx,
	} {
		if idx < 0 {
			t.Fatalf("%s not found in pretty gate output:\n%q", name, got)
		}
	}
	if yIdx >= nIdx || nIdx >= eIdx || eIdx >= rIdx {
		t.Errorf("option lines out of declared order: y=%d n=%d e=%d r=%d\n%q", yIdx, nIdx, eIdx, rIdx, got)
	}
	if rIdx >= promptIdx {
		t.Errorf("prompt line must come LAST (after every option): r=%d prompt=%d\n%q", rIdx, promptIdx, got)
	}
	// A blank line must separate the last option from the prompt line.
	if !strings.Contains(got, "    r  regenerate\n\n  Continue? › ") {
		t.Errorf("expected a blank line between the last option and the prompt line:\n%q", got)
	}
}

// TestPrettyGateDefaultMarkerOnDefaultLineOnly proves the " [default]" marker
// sits beside the default action (y for NotesReviewGate) and on NO other line.
func TestPrettyGateDefaultMarkerOnDefaultLineOnly(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "    y  accept & proceed [default]") {
		t.Errorf("[default] marker not on the y (default) line:\n%q", got)
	}
	// The marker must appear exactly once across the whole menu render.
	if n := strings.Count(got, "[default]"); n != 1 {
		t.Errorf("[default] marker appears %d times, want exactly 1 (default line only):\n%q", n, got)
	}
	// And explicitly not on the non-default lines.
	for _, line := range []string{"    n  abort", "    e  edit in $EDITOR", "    r  regenerate"} {
		if strings.Contains(got, line+" [default]") {
			t.Errorf("[default] marker wrongly on a non-default line %q:\n%q", line, got)
		}
	}
}

// TestPrettyGateNonYDefaultMarksDeclaredDefault proves the marker tracks
// gate.Default, not the y key: with Default == ChoiceNo the [default] marker sits
// on the n line, NOT the y line.
func TestPrettyGateNonYDefaultMarksDeclaredDefault(t *testing.T) {
	out := drivePrettyPromptProfile("n\n", gateNoDefault(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "    n  abort [default]") {
		t.Errorf("[default] marker not on the n line for a ChoiceNo-default gate:\n%q", got)
	}
	if strings.Contains(got, "    y  accept & proceed [default]") {
		t.Errorf("[default] marker wrongly on the y line for a ChoiceNo-default gate:\n%q", got)
	}
	if n := strings.Count(got, "[default]"); n != 1 {
		t.Errorf("[default] marker appears %d times, want exactly 1:\n%q", n, got)
	}
}

// TestPrettyGateTwoChoiceRendersOnlyYN proves the reuse confirm renders exactly
// two option lines (y/n) and NO e/r lines — the menu is built from the gate's
// declared set, not a hardcoded y/n/e/r list.
func TestPrettyGateTwoChoiceRendersOnlyYN(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.ReuseConfirmGate(), termenv.Ascii)
	got := out.String()

	if !strings.Contains(got, "    y  accept & proceed [default]") {
		t.Errorf("y line missing from reuse confirm menu:\n%q", got)
	}
	if !strings.Contains(got, "    n  abort") {
		t.Errorf("n line missing from reuse confirm menu:\n%q", got)
	}
	if strings.Contains(got, "    e  ") {
		t.Errorf("reuse confirm must NOT render an e line (not declared):\n%q", got)
	}
	if strings.Contains(got, "    r  ") {
		t.Errorf("reuse confirm must NOT render an r line (not declared):\n%q", got)
	}
}

// TestPrettyGateRedrawnAfterBadInput proves the FULL menu (option block + prompt)
// is redrawn after unrecognised input: with "x\ny\n" the option block and the
// prompt line each appear TWICE (initial render + the post-bad-input re-render),
// the linear-scroll redraw with no screen-clearing.
func TestPrettyGateRedrawnAfterBadInput(t *testing.T) {
	out := drivePrettyPromptProfile("x\ny\n", presenter.NotesReviewGate(), termenv.Ascii)
	got := out.String()

	if n := strings.Count(got, "  Continue? › "); n != 2 {
		t.Errorf("prompt line rendered %d times, want 2 (initial + redraw after bad input):\n%q", n, got)
	}
	if n := strings.Count(got, "    y  accept & proceed"); n != 2 {
		t.Errorf("option block rendered %d times, want 2 (initial + redraw after bad input):\n%q", n, got)
	}
}

// TestPrettyGateMenuBuiltFromDeclaredChoices proves the menu derives from the
// gate's declared choices: changing the action labels and order changes the
// rendered menu (no hardcoded option list). A custom gate with reordered choices
// and custom labels renders those labels in that order.
func TestPrettyGateMenuBuiltFromDeclaredChoices(t *testing.T) {
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

	nIdx := strings.Index(got, "    n  stop here")
	yIdx := strings.Index(got, "    y  go go go [default]")
	promptIdx := strings.Index(got, "  Proceed? › ")
	if nIdx < 0 || yIdx < 0 || promptIdx < 0 {
		t.Fatalf("custom gate menu not rendered from declared choices:\n%q", got)
	}
	// Declared order is n THEN y here — the menu must follow the gate, not a fixed y-first list.
	if nIdx >= yIdx {
		t.Errorf("menu did not follow the gate's declared order (n before y): n=%d y=%d\n%q", nIdx, yIdx, got)
	}
	if yIdx >= promptIdx {
		t.Errorf("prompt line must come last: y=%d prompt=%d\n%q", yIdx, promptIdx, got)
	}
}

// TestPrettyGateColourDowngradePreservesStructure forces the no-colour (Ascii)
// profile and asserts the menu structure — every option line, the [default]
// marker, and the "› " prompt — survives as plain text with NO ESC (0x1b) byte.
func TestPrettyGateColourDowngradePreservesStructure(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.Ascii)

	if bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("downgraded gate output contains an ESC (0x1b) — colour codes leaked:\n%q", out.String())
	}
	got := out.String()
	for _, want := range []string{
		"    y  accept & proceed [default]",
		"    n  abort",
		"    e  edit in $EDITOR",
		"    r  regenerate",
		"  Continue? › ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("menu structure %q missing under colour downgrade:\n%q", want, got)
		}
	}
}

// TestPrettyGateColourOnEmitsANSIButKeepsStructure forces a colour-capable profile
// and asserts ANSI escapes ARE emitted while the menu structure (option text, the
// [default] marker, the › prompt marker) still survives. Because lipgloss wraps
// styled spans in ANSI, the structure is asserted as substrings that the styling
// does not split (the layout — indent, action text, [default], › — is unstyled).
func TestPrettyGateColourOnEmitsANSIButKeepsStructure(t *testing.T) {
	out := drivePrettyPromptProfile("y\n", presenter.NotesReviewGate(), termenv.TrueColor)

	if !bytes.ContainsRune(out.Bytes(), 0x1b) {
		t.Errorf("colour-on gate output contains no ESC (0x1b) — expected ANSI codes:\n%q", out.String())
	}
	got := out.String()
	for _, want := range []string{
		"accept & proceed [default]",
		"abort",
		"edit in $EDITOR",
		"regenerate",
		"Continue?",
		"› ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("menu structure %q missing under colour-on:\n%q", want, got)
		}
	}
}
