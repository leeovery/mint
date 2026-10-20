package notes

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDiffTooLarge is the sentinel for an input over its size limit, matched with
// errors.Is.
var ErrDiffTooLarge = errors.New("diff too large")

// CheckDiffSize reports whether diff runs past maxLines lines, returning a wrapped
// ErrDiffTooLarge when it does. The boundary is INCLUSIVE: exactly maxLines passes;
// strictly more fails.
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
