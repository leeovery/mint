package presenter_test

import (
	"strings"
	"testing"
)

// gutterLines builds the expected pretty gutter rendering of a panel BODY under
// the Ascii profile: one rendered line per body line (trailing newline trimmed
// first), each non-empty line as "│ {line}" and each empty line as the bare "│".
// An empty body yields nil — the panel renders only its title line. This is the
// SINGLE test-side definition of the gutter arithmetic, mirroring the
// presenter's renderGutterPanel.
func gutterLines(body string) []string {
	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return nil
	}
	src := strings.Split(trimmed, "\n")
	lines := make([]string, len(src))
	for i, line := range src {
		if line == "" {
			lines[i] = "│"
			continue
		}
		lines[i] = "│ " + line
	}
	return lines
}

// assertGutterPanel asserts a rendered pretty gutter panel (Ascii profile)
// opens with its LEADING blank separator line, then carries the title line
// "│ {title}", the bare-"│" spacer, and then EXACTLY one gutter line per body
// line with the content intact — same line count, same order, nothing truncated.
// It is the shared acceptance for the body-content-intact contract ShowNotes and
// ShowMessage both honour.
func assertGutterPanel(t *testing.T, rendered, title, body string) {
	t.Helper()

	want := []string{"", "│ " + title, "│"}
	want = append(want, gutterLines(body)...)

	got := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("gutter panel has %d lines, want %d (leading blank + title + spacer + one per body line)\n got: %q\nwant: %q", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("gutter panel line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// stripANSI removes CSI SGR escape sequences (ESC '[' … 'm') from s so a rendered
// line can be compared as bare text regardless of colour styling. It is a minimal
// stripper sufficient for the dim styling lipgloss emits on the gutter and bar,
// and is the single shared ANSI-strip primitive for the pretty tests.
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
