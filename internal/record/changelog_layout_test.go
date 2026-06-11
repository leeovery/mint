package record_test

import (
	"testing"

	"mint/internal/record"
)

// TestChangelogDateLayout pins the exported section-header date layout record owns.
// record is the sole writer of the `## [x.y.z] - <date>` header, so the layout it
// emits is canonical; the engine's regenerate heal parses the historical date back
// with this SAME exported layout so the healed header is byte-identical to existing
// record-emitted sections. Pinning it here makes "parse layout == emit layout" a
// compile-time fact referencing one owned symbol.
func TestChangelogDateLayout(t *testing.T) {
	t.Parallel()

	const expected = "2006-01-02"
	if record.ChangelogDateLayout != expected {
		t.Errorf("ChangelogDateLayout = %q, want %q", record.ChangelogDateLayout, expected)
	}
}
