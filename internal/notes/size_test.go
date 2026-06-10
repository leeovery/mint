package notes_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/notes"
)

// repeatLines builds a diff text of exactly n newline-terminated lines (each
// "x\n"). With a final newline, the line count equals n under the guard's
// convention. n == 0 yields the empty string (zero lines).
func repeatLines(n int) string {
	if n == 0 {
		return ""
	}
	return strings.Repeat("x\n", n)
}

func TestCheckDiffSize_ExactlyMaxLines_Passes(t *testing.T) {
	t.Parallel()

	// The boundary is inclusive: a post-exclusion diff of exactly maxLines lines
	// passes (returns nil). 50000 is the default ceiling.
	diff := repeatLines(50000)

	if err := notes.CheckDiffSize(diff, 50000); err != nil {
		t.Errorf("CheckDiffSize(50000 lines, max 50000) = %v, want nil (boundary is inclusive)", err)
	}
}

func TestCheckDiffSize_OverMaxLines_ReturnsDiffTooLarge(t *testing.T) {
	t.Parallel()

	// maxLines + 1 (strictly greater) fails with the distinguishable ErrDiffTooLarge
	// so the AI is never called on an oversized diff.
	diff := repeatLines(50001)

	err := notes.CheckDiffSize(diff, 50000)
	if err == nil {
		t.Fatalf("CheckDiffSize(50001 lines, max 50000) = nil, want ErrDiffTooLarge")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
}

func TestCheckDiffSize_OverMaxLines_MessageCarriesCounts(t *testing.T) {
	t.Parallel()

	// The too-large failure must carry the actual counts so on_notes_failure (task
	// 2-7) can route and NAME it, e.g. "diff exceeds max_diff_lines (50001 > 50000)".
	err := notes.CheckDiffSize(repeatLines(50001), 50000)
	if err == nil {
		t.Fatal("CheckDiffSize(over max) = nil, want an error carrying the counts")
	}

	msg := err.Error()
	if !strings.Contains(msg, "50001") {
		t.Errorf("error %q does not carry the actual line count 50001", msg)
	}
	if !strings.Contains(msg, "50000") {
		t.Errorf("error %q does not carry the configured limit 50000", msg)
	}
	if !strings.Contains(msg, "max_diff_lines") {
		t.Errorf("error %q does not name the max_diff_lines guard", msg)
	}
}

func TestCheckDiffSize_EmptyDiff_IsZeroLines_Passes(t *testing.T) {
	t.Parallel()

	// Convention: an empty diff counts as 0 lines, so it passes any non-negative
	// ceiling (here a tiny limit of 1). The degenerate/too-small stub is task 2-8,
	// not this too-large guard.
	if err := notes.CheckDiffSize("", 1); err != nil {
		t.Errorf("CheckDiffSize(empty, max 1) = %v, want nil (empty diff is 0 lines)", err)
	}
}

func TestCheckDiffSize_TrailingPartialLine_Counts(t *testing.T) {
	t.Parallel()

	// Convention: a final partial line with no trailing newline still counts. "a\nb"
	// is 2 lines, so against a ceiling of 1 it must fail. This pins the documented
	// line-count rule.
	err := notes.CheckDiffSize("a\nb", 1)
	if err == nil {
		t.Fatalf(`CheckDiffSize("a\nb", max 1) = nil, want failure (final partial line counts as line 2)`)
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want notes.ErrDiffTooLarge", err)
	}
}

func TestCheckDiffSize_TrailingNewlineOnly_DoesNotAddPhantomLine(t *testing.T) {
	t.Parallel()

	// The mirror of the partial-line case: "a\nb\n" is exactly 2 lines, NOT 3 — a
	// trailing newline terminates line 2 and does not introduce an empty line 3. So
	// against a ceiling of 2 it passes.
	if err := notes.CheckDiffSize("a\nb\n", 2); err != nil {
		t.Errorf(`CheckDiffSize("a\nb\n", max 2) = %v, want nil (trailing newline adds no phantom line)`, err)
	}
}

func TestCheckDiffSize_CountsOnlyPostExclusionText_ExcludedLinesDoNotCount(t *testing.T) {
	t.Parallel()

	// The guard counts ONLY the post-exclusion diff text it is handed (the output of
	// AssembleDiff, where git has already dropped excluded paths). Lines that would
	// have belonged to an excluded path are simply not present in the input, so they
	// cannot count. Here the post-exclusion text is 2 lines and passes a ceiling of
	// 2, even though the original pre-exclusion diff might have been far larger.
	postExclusion := "diff --git a/api.go b/api.go\n+added\n"

	if err := notes.CheckDiffSize(postExclusion, 2); err != nil {
		t.Errorf("CheckDiffSize(2-line post-exclusion diff, max 2) = %v, want nil — only post-exclusion lines count", err)
	}
}

func TestCheckDiffSize_CustomLimit_HonouredAtBoundary(t *testing.T) {
	t.Parallel()

	// A custom (non-default) ceiling is honoured at its own boundary: exactly the
	// limit passes, one over fails. Here the limit is 10, proving the guard uses the
	// passed maxLines rather than a hard-coded 50000.
	const limit = 10

	if err := notes.CheckDiffSize(repeatLines(limit), limit); err != nil {
		t.Errorf("CheckDiffSize(%d lines, max %d) = %v, want nil at boundary", limit, limit, err)
	}

	over := notes.CheckDiffSize(repeatLines(limit+1), limit)
	if over == nil {
		t.Fatalf("CheckDiffSize(%d lines, max %d) = nil, want ErrDiffTooLarge", limit+1, limit)
	}
	if !errors.Is(over, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want notes.ErrDiffTooLarge for custom over-limit", over)
	}
}
