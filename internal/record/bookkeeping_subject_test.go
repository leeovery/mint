package record_test

import (
	"testing"

	"mint/internal/record"
)

// TestBookkeepingSubject pins the single release-bookkeeping commit subject record
// owns. record is the sole producer of the `{commitPrefix} Release {tag}` subject;
// the actual bookkeeping commit, the dry-run plan, and the annotated-tag subject
// line all obtain it from this SAME exported symbol, so they can never drift.
// Pinning the exact format here makes "commit == plan == tag" a compile-time fact
// referencing one owned symbol.
func TestBookkeepingSubject(t *testing.T) {
	t.Parallel()

	const (
		prefix   = "🌿"
		tag      = "v0.0.1"
		expected = "🌿 Release v0.0.1"
	)
	if got := record.BookkeepingSubject(prefix, tag); got != expected {
		t.Errorf("BookkeepingSubject(%q, %q) = %q, want %q", prefix, tag, got, expected)
	}
}
