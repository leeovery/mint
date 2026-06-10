package notes_test

import (
	"strings"
	"testing"

	"mint/internal/notes"
)

func TestIsDegenerate_EmptyDiff_True(t *testing.T) {
	t.Parallel()

	// The small-end guard: an EMPTY post-exclusion diff is degenerate, so the
	// caller (precedence, task 2-10) writes a stub and never calls the AI.
	if !notes.IsDegenerate("") {
		t.Error(`IsDegenerate("") = false, want true (empty post-exclusion diff is degenerate)`)
	}
}

func TestIsDegenerate_AllExcludedDiff_True(t *testing.T) {
	t.Parallel()

	// The input is ALREADY post-exclusion (the output of AssembleDiff). When every
	// changed file fell under exclusion — in Phase 2, the only exclusion is
	// CHANGELOG.md, so "only CHANGELOG.md changed" — git emits an empty diff. The
	// detector therefore sees "" and treats all-excluded uniformly with empty: a
	// re-tag with no source change and an all-excluded release both reduce to "".
	allExcludedPostExclusion := ""

	if !notes.IsDegenerate(allExcludedPostExclusion) {
		t.Error("IsDegenerate(all-excluded -> empty post-exclusion) = false, want true")
	}
}

func TestIsDegenerate_WhitespaceOnlyDiff_True(t *testing.T) {
	t.Parallel()

	// Whitespace-only post-exclusion diffs are degenerate too: pure churn that
	// leaves only blank space carries nothing notable. Spaces, tabs, newlines, and
	// CR — alone and in combination — all reduce to nothing.
	tests := []struct {
		name string
		diff string
	}{
		{name: "single space", diff: " "},
		{name: "tabs", diff: "\t\t"},
		{name: "newlines", diff: "\n\n\n"},
		{name: "carriage returns", diff: "\r\r"},
		{name: "crlf", diff: "\r\n"},
		{name: "mixed whitespace", diff: " \t\r\n \n\t "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !notes.IsDegenerate(tt.diff) {
				t.Errorf("IsDegenerate(%q) = false, want true (whitespace-only diff is degenerate)", tt.diff)
			}
		})
	}
}

func TestIsDegenerate_RealDiff_False(t *testing.T) {
	t.Parallel()

	// A real, non-empty post-exclusion diff with actual content is NOT degenerate:
	// the caller proceeds to the normal AI path. This pins the false branch with a
	// genuine `diff --git` header plus +/- hunk lines.
	realDiff := "diff --git a/api.go b/api.go\n--- a/api.go\n+++ b/api.go\n@@ -1 +1 @@\n-old\n+new\n"

	if notes.IsDegenerate(realDiff) {
		t.Errorf("IsDegenerate(%q) = true, want false (a real diff is not degenerate)", realDiff)
	}
}

func TestIsDegenerate_RealContentWithSurroundingWhitespace_False(t *testing.T) {
	t.Parallel()

	// Whitespace AROUND real content does not make a diff degenerate — only an
	// entirely-whitespace (or empty) diff does. A diff padded with leading/trailing
	// blank lines still carries a notable change, so it is NOT degenerate.
	padded := "\n\n+meaningful change\n\n"

	if notes.IsDegenerate(padded) {
		t.Errorf("IsDegenerate(%q) = true, want false (real content amid whitespace is not degenerate)", padded)
	}
}

func TestStubBody_ReturnsMaintenanceStubLine(t *testing.T) {
	t.Parallel()

	// The degenerate stub is a fixed, honest line. Pin the EXACT wording so the
	// truthful record is deterministic. It is just the body — the version header is
	// added by the Phase 1 changelog writer, not here.
	const want = "Maintenance release — no notable source changes"

	if got := notes.StubBody(); got != want {
		t.Errorf("StubBody() = %q, want %q", got, want)
	}
}

func TestStubBody_IsSingleNonEmptyHonestLine(t *testing.T) {
	t.Parallel()

	// The stub must be a real entry: non-empty, a SINGLE line (no embedded
	// newlines), and not an error or skipped/empty placeholder. This is the
	// "minimal honest entry, not a hard error and not a skipped entry" guarantee.
	got := notes.StubBody()

	if got == "" {
		t.Fatal("StubBody() = empty, want a non-empty honest stub line (not a skipped entry)")
	}
	if strings.Contains(got, "\n") {
		t.Errorf("StubBody() = %q, want a single line with no embedded newline", got)
	}
	if strings.TrimSpace(got) == "" {
		t.Errorf("StubBody() = %q, want a substantive line, not whitespace", got)
	}
}
