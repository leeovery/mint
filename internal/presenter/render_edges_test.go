package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// TestPlainUnwoundEmptySummaryRendersLabelOnly locks the documented "empty
// string is legal" Unwind edge in plain: the synthesised "unwound: " prefix
// renders with nothing after it — no invented summary text.
func TestPlainUnwoundEmptySummaryRendersLabelOnly(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, &bytes.Buffer{})

	p.Unwound(presenter.Unwind{})

	if out.String() != "unwound: \n" {
		t.Errorf("plain empty-summary unwound = %q, want %q", out.String(), "unwound: \n")
	}
}

// TestPlainPromptFinalLineWithoutNewlineSelectsChoice directly exercises the
// EOF positive path: a final "y" with no trailing newline is still a parsed
// answer, not an ErrInputClosed.
func TestPlainPromptFinalLineWithoutNewlineSelectsChoice(t *testing.T) {
	p, _, _ := plainGate(strings.NewReader("y"), gateOpts{})

	choice, err := p.Prompt(presenter.NotesReviewGate())
	if err != nil {
		t.Fatalf("plain Prompt on un-terminated final line returned error: %v", err)
	}
	if choice != presenter.ChoiceYes {
		t.Errorf("plain Prompt = %q, want %q", choice, presenter.ChoiceYes)
	}
}

// TestRailPromptFinalLineWithoutNewlineSelectsChoice mirrors the EOF positive path
// through the Rail's typed line.
func TestRailPromptFinalLineWithoutNewlineSelectsChoice(t *testing.T) {
	p, _, _ := railGate(termenv.Ascii, strings.NewReader("n"), gateOpts{})
	defer p.Close()

	choice, err := p.Prompt(presenter.NotesReviewGate())
	if err != nil {
		t.Fatalf("rail Prompt on un-terminated final line returned error: %v", err)
	}
	if choice != presenter.ChoiceNo {
		t.Errorf("rail Prompt = %q, want %q", choice, presenter.ChoiceNo)
	}
}

// TestPromptMatchesDeclaredKeyCaseInsensitively proves the parse folds BOTH
// sides: a gate that declares a non-lowercase key (a source value like "GitHub")
// is selectable by any casing of input, and the DECLARED key is returned —
// canonical, never the raw input.
func TestPromptMatchesDeclaredKeyCaseInsensitively(t *testing.T) {
	gate := presenter.SourceGate([]presenter.GateChoice{
		{Key: presenter.Choice("GitHub"), Action: "publish to GitHub"},
		{Key: presenter.Choice("gitlab"), Action: "publish to GitLab"},
	}, presenter.Choice("GitHub"))
	p, _, _ := plainGate(strings.NewReader("github\n"), gateOpts{})

	choice, err := p.Prompt(gate)
	if err != nil {
		t.Fatalf("plain Prompt returned error: %v", err)
	}
	if choice != presenter.Choice("GitHub") {
		t.Errorf("plain Prompt = %q, want the DECLARED key %q (canonical, case-folded match)", choice, "GitHub")
	}
}
