package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// The gutter-panel expectation helpers (gutterLines/assertGutterPanel) and the
// ANSI stripper (stripANSI) live in the shared pretty_helpers_test.go so they are
// defined once and referenced — not re-declared — here.
//
// The old rule-sizing tests (narrow/wide/tiny/undetectable rule widths) were
// DELETED with the titled/closing rules themselves: the gutter panel does no
// width math, so termWidth no longer affects ShowNotes/ShowMessage at all. What
// remains width-relevant — the body is NEVER truncated at any width, and stage
// lines are width-independent — is pinned below.

// TestPrettyShowNotesBodyNeverTruncatedRegardlessOfWidth is the never-truncate
// acceptance: a notes body line LONGER than any plausible terminal is rendered in
// FULL behind the gutter at a narrow, a huge, and an undetectable (0) termWidth —
// every line present and intact, no ellipsis, no clipping. The presenter applies
// no hard-wrap or truncation to the body; terminal soft-wrap handles long lines.
func TestPrettyShowNotesBodyNeverTruncatedRegardlessOfWidth(t *testing.T) {
	longLine := strings.Repeat("x", 200) + " end-of-very-long-line-marker"
	body := "lead in\n" + longLine + "\ntrailing line"

	for _, width := range []int{20, 2000, 0} {
		out := &bytes.Buffer{}
		p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithTermWidth(width)
		p.ShowNotes(presenter.Notes{Version: "1.4.0", Body: body})

		got := out.String()
		// Every body line renders in full behind the gutter — the panel is
		// width-independent, so the same complete layout appears at any width.
		assertGutterPanel(t, got, "release notes · v1.4.0", body)
		// And specifically the tail of the long line survives (no ellipsis/clip).
		if !strings.Contains(got, "│ "+longLine+"\n") {
			t.Errorf("width %d: long line not rendered in full behind the gutter:\n%q", width, got)
		}
		if strings.Contains(got, "…") || strings.Contains(got, "...") {
			t.Errorf("width %d: notes body must not be truncated with an ellipsis:\n%q", width, got)
		}
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
		p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithTermWidth(width)
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
