package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// notesTitlePrefix is the fixed title segment the pretty titled rule opens with —
// "── release notes · v{X} " — before the trailing run of U+2500 fills out to the
// capped width. The width tests build expected rules by filling this prefix to the
// asserted width, mirroring the presenter's own notesTitledRule construction.
func notesTitlePrefix(version string) string {
	return "── release notes · v" + version + " "
}

// ruleDisplayWidth counts the display columns of a rendered rule line: the rune
// count of the line with any ANSI SGR escapes stripped. The decorative rule is
// built from single-column ASCII/box-drawing runes, so the rune count IS the column
// width — this lets a test assert "the rule is N columns wide" independent of
// colour styling.
func ruleDisplayWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

// stripANSI removes CSI SGR escape sequences (ESC '[' … 'm') from s so a rendered
// line can be measured by display columns regardless of colour styling. It is a
// minimal stripper sufficient for the dim styling lipgloss emits on the rules.
func stripANSI(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// notesRuleLines splits a rendered pretty ShowNotes block into its lines (dropping
// the trailing empty element from the final newline) and returns the titled opener
// rule (first line) and the closing rule (last line). The body, if any, sits
// between them; the rules are positional.
func notesRuleLines(t *testing.T, rendered string) (opener, closer string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered notes block has too few lines: %q", rendered)
	}
	return lines[0], lines[len(lines)-1]
}

// TestPrettyShowNotesRuleSizedToNarrowTerminalWidth covers the narrower-than-cap
// edge: with WithTermWidth(30) — narrower than the ~50 cap — both decorative rules
// render exactly 30 columns wide (no overflow), and the titled rule keeps its title
// prefix with the trailing U+2500 fill clamped to 30. The no-colour profile keeps
// the assertion on display width rather than ANSI bytes.
func TestPrettyShowNotesRuleSizedToNarrowTerminalWidth(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii).WithTermWidth(30)
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})

	opener, closer := notesRuleLines(t, out.String())

	if got := ruleDisplayWidth(opener); got != 30 {
		t.Errorf("narrow titled rule width = %d, want 30:\n%q", got, opener)
	}
	if got := ruleDisplayWidth(closer); got != 30 {
		t.Errorf("narrow closing rule width = %d, want 30:\n%q", got, closer)
	}
	if !strings.Contains(stripANSI(opener), notesTitlePrefix("1.4.0")) {
		t.Errorf("narrow titled rule dropped its title prefix:\n%q", opener)
	}
}

// TestPrettyShowNotesRuleClampsToCapOnWideTerminal covers the wider-than-cap edge:
// with WithTermWidth(200) — far wider than the cap — both rules clamp to the cap
// (50), never sprawling to 200 columns.
func TestPrettyShowNotesRuleClampsToCapOnWideTerminal(t *testing.T) {
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii).WithTermWidth(200)
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})

	opener, closer := notesRuleLines(t, out.String())

	if got := ruleDisplayWidth(opener); got != 50 {
		t.Errorf("wide titled rule width = %d, want 50 (cap):\n%q", got, opener)
	}
	if got := ruleDisplayWidth(closer); got != 50 {
		t.Errorf("wide closing rule width = %d, want 50 (cap):\n%q", got, closer)
	}
}

// TestPrettyShowNotesRuleFallsBackToCapWhenUndetectable covers the undetectable
// edge: the default presenter (no WithTermWidth → termWidth 0) and an explicitly
// undetectable WithTermWidth(0) both render the rule at the full cap (50), the same
// fixed rule task 2-5 produced.
func TestPrettyShowNotesRuleFallsBackToCapWhenUndetectable(t *testing.T) {
	tests := []struct {
		name  string
		apply func(p *presenter.PrettyPresenter) *presenter.PrettyPresenter
	}{
		{
			name:  "default presenter (no WithTermWidth)",
			apply: func(p *presenter.PrettyPresenter) *presenter.PrettyPresenter { return p },
		},
		{
			name:  "explicit undetectable width zero",
			apply: func(p *presenter.PrettyPresenter) *presenter.PrettyPresenter { return p.WithTermWidth(0) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			p := tt.apply(presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii))
			p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})

			opener, closer := notesRuleLines(t, out.String())
			if got := ruleDisplayWidth(opener); got != 50 {
				t.Errorf("undetectable titled rule width = %d, want 50 (cap):\n%q", got, opener)
			}
			if got := ruleDisplayWidth(closer); got != 50 {
				t.Errorf("undetectable closing rule width = %d, want 50 (cap):\n%q", got, closer)
			}
		})
	}
}

// TestPrettyShowNotesTinyTerminalYieldsTinyRuleNoSpecialBranch covers the tiny
// edge: WithTermWidth(3) yields a 3-column rule via the SAME min — there is no
// bespoke tiny-terminal layout. The body still appears in full (wraps naturally,
// never truncated); --plain is the documented escape hatch for genuinely tiny
// terminals.
func TestPrettyShowNotesTinyTerminalYieldsTinyRuleNoSpecialBranch(t *testing.T) {
	const body = "Faster cold starts and a calmer log."
	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii).WithTermWidth(3)
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})

	opener, closer := notesRuleLines(t, out.String())
	if got := ruleDisplayWidth(closer); got != 3 {
		t.Errorf("tiny closing rule width = %d, want 3:\n%q", got, closer)
	}
	// The titled rule keeps its title prefix and clamps the fill to ≥ 1 — the title
	// is longer than 3, so the rule is the prefix plus a single trailing U+2500
	// (the negative-fill guard), NOT a truncated title. The body is never clipped.
	if !strings.Contains(stripANSI(opener), notesTitlePrefix("1.4.0")) {
		t.Errorf("tiny titled rule must keep its full title prefix (no truncation):\n%q", opener)
	}
	if !strings.Contains(out.String(), body) {
		t.Errorf("tiny-terminal body must appear in full (never truncated):\n%q", out.String())
	}
}

// TestPrettyShowNotesBodyNeverTruncatedRegardlessOfWidth is the wrap-never-truncate
// acceptance: a notes body line LONGER than both the cap and a narrow termWidth is
// rendered in FULL — every byte present, no ellipsis, no clipping. The presenter
// applies no hard-wrap or truncation to the body; terminal soft-wrap handles long
// lines. Asserted at a narrow width where a truncating implementation would have
// dropped bytes.
func TestPrettyShowNotesBodyNeverTruncatedRegardlessOfWidth(t *testing.T) {
	// A single line far longer than ruleCap (50) and far longer than termWidth 20.
	longLine := strings.Repeat("x", 200) + " end-of-very-long-line-marker"
	body := "lead in\n" + longLine + "\ntrailing line"

	out := &bytes.Buffer{}
	p := presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii).WithTermWidth(20)
	p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})

	got := out.String()
	// The FULL body bytes must appear contiguously — nothing dropped or wrapped by
	// the presenter (the presenter never inserts a newline into the body).
	if !strings.Contains(got, body) {
		t.Errorf("long body was not rendered verbatim in full:\n%q", got)
	}
	// And specifically the tail of the long line survives (no ellipsis/clip).
	if !strings.Contains(got, "end-of-very-long-line-marker") {
		t.Errorf("tail of the long notes line was truncated:\n%q", got)
	}
	if strings.Contains(got, "…") || strings.Contains(got, "...") {
		t.Errorf("notes body must not be truncated with an ellipsis:\n%q", got)
	}
}

// TestPrettyStageLineByteIdenticalAcrossWidths is the fixed-stage-line acceptance:
// a short stage ✓ line is width-INDEPENDENT — it renders BYTE-IDENTICALLY at a tiny
// termWidth (20) and a wide one (200). Stage lines carry no width-dependent wrapping
// or padding beyond the existing column alignment, so the rendered bytes match
// exactly across widths.
func TestPrettyStageLineByteIdenticalAcrossWidths(t *testing.T) {
	render := func(width int) string {
		out := &bytes.Buffer{}
		p := presenter.NewPrettyPresenterWithProfile(out, termenv.Ascii).WithTermWidth(width)
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean · on main · tag free · in sync with origin"})
		return out.String()
	}

	narrow := render(20)
	wide := render(200)
	if narrow != wide {
		t.Errorf("stage line differs across terminal widths\n width=20 : %q\n width=200: %q", narrow, wide)
	}
}

// TestPlainShowNotesUntouchedByTermWidth confirms the plain presenter has no width
// math: its fixed "--- release notes v{X} ---" / "--- end notes ---" delimiters are
// rendered regardless of termWidth, and plain exposes no WithTermWidth knob to begin
// with. The plain rendering is identical to the spec's fixed form at any width — so
// driving the plain presenter (which ignores width entirely) yields the fixed
// delimiters unchanged.
func TestPlainShowNotesUntouchedByTermWidth(t *testing.T) {
	out := &bytes.Buffer{}
	presenter.NewPlainPresenter(out, &bytes.Buffer{}).ShowNotes(presenter.Notes{Version: "1.4.0", Body: "hi"})

	got := out.String()
	want := "--- release notes v1.4.0 ---\nhi\n--- end notes ---\n"
	if got != want {
		t.Errorf("plain notes block changed\n got: %q\nwant: %q", got, want)
	}
	// No decorative box-drawing rule (U+2500) leaks into plain — plain has no
	// capped decorative rules at all.
	if strings.Contains(got, "─") {
		t.Errorf("plain notes block must not contain a decorative U+2500 rule:\n%q", got)
	}
}
