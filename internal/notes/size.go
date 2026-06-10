package notes

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDiffTooLarge is the distinguishable sentinel returned (wrapped) when the
// post-exclusion diff exceeds max_diff_lines. It is matched with errors.Is so the
// on_notes_failure routing (task 2-7) can branch on the too-large case and NAME
// it ("diff too large") distinctly from other notes failures. The wrapped error
// carries the actual line count and configured limit in its message.
var ErrDiffTooLarge = errors.New("diff too large")

// CheckDiffSize is the max_diff_lines guard: a cost+quality ceiling on the
// POST-exclusion diff. It counts the diff's lines and, if that exceeds maxLines,
// returns a wrapped ErrDiffTooLarge so the caller skips the AI entirely (a huge
// diff is slow, costly, and summarises to mush). A diff within the ceiling
// returns nil, and the caller proceeds to the AI in a later wiring task.
//
// The input is ALREADY post-exclusion (the output of AssembleDiff, where git has
// dropped CHANGELOG.md and any configured diff_exclude paths), so excluded-path
// lines are inherently absent and cannot count toward the ceiling — the guard
// counts only what it is handed.
//
// maxLines is passed in (the orchestrator resolves it from config.MaxDiffLines)
// rather than read here, keeping the guard a pure function of (diff, maxLines).
// The boundary is INCLUSIVE: exactly maxLines passes; strictly more fails.
//
// Deferred: the parked "Change Map + trimmed diff" escalation — trimming the diff
// down to the ceiling instead of failing — is intentionally NOT implemented here
// (revisit only on observed need; see the spec's Big-diff handling section).
func CheckDiffSize(diff string, maxLines int) error {
	got := countDiffLines(diff)
	if got > maxLines {
		return fmt.Errorf("%w: diff exceeds max_diff_lines (%d > %d)", ErrDiffTooLarge, got, maxLines)
	}
	return nil
}

// countDiffLines counts the lines of a diff under mint's pinned convention:
//   - empty diff -> 0 lines;
//   - otherwise lines = number of newline-terminated lines, plus 1 for a final
//     partial line that is NOT newline-terminated.
//
// So "a\nb\n" is 2 lines (a trailing newline adds no phantom empty line) and
// "a\nb" is also 2 lines (the final partial line counts). This makes the count a
// stable, cheap token proxy regardless of whether git's stdout ends in a newline.
func countDiffLines(diff string) int {
	if diff == "" {
		return 0
	}

	lines := strings.Count(diff, "\n")
	if !strings.HasSuffix(diff, "\n") {
		lines++ // final partial line (no trailing newline) counts.
	}
	return lines
}
