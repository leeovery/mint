package record_test

import (
	"testing"

	"mint/internal/record"
)

// TestChangelogFileName pins the exported changelog filename record owns. record is
// the sole writer of CHANGELOG.md at the repo root; the engine stages exactly that
// path (`git -C {root} add CHANGELOG.md`) via this SAME exported symbol, so the
// written path and the staged path can never drift. Pinning it here makes
// "written == staged" a compile-time fact referencing one owned symbol.
func TestChangelogFileName(t *testing.T) {
	t.Parallel()

	const expected = "CHANGELOG.md"
	if record.ChangelogFileName != expected {
		t.Errorf("ChangelogFileName = %q, want %q", record.ChangelogFileName, expected)
	}
}
