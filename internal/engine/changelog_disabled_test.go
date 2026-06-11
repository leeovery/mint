package engine_test

import (
	"testing"

	"mint/internal/engine"
)

// TestErrChangelogDisabledMessage pins the exported changelog-disabled sentinel both
// the cmd validateTargetAgainstChangelog and the engine checkBatchTargetConfig now
// reference. Keeping the single/batch validators separate (their concrete-enum
// signatures differ) is correct, but the pinned wording must be one owned symbol so
// the two paths can never drift. This pins that wording.
func TestErrChangelogDisabledMessage(t *testing.T) {
	t.Parallel()

	const expected = "changelog is disabled in config"
	if engine.ErrChangelogDisabled.Error() != expected {
		t.Errorf("ErrChangelogDisabled = %q, want %q", engine.ErrChangelogDisabled.Error(), expected)
	}
}
