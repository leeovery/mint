package configtest_test

import (
	"errors"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
)

// TestByLevelKey_IndexesEveryRowUnderItsLevelKeyPair proves the builder folds a row
// slice into a (level, key) → row lookup, indexing every row under its own pair. This
// is the shared replacement for the two test packages' local rowSet/rowByLevelKey
// builders, so it must resolve each row by its exact (level, key) identity.
func TestByLevelKey_IndexesEveryRowUnderItsLevelKeyPair(t *testing.T) {
	t.Parallel()

	rows := []config.MetadataRow{
		{Key: "ai_command", Level: config.LevelShared, Default: "claude"},
		{Key: "ai_command", Level: config.LevelRelease, Default: "shared"},
		{Key: "tag_prefix", Level: config.LevelRelease, Default: "v"},
	}

	index, err := configtest.ByLevelKey(rows)
	if err != nil {
		t.Fatalf("ByLevelKey returned unexpected error: %v", err)
	}
	if len(index) != len(rows) {
		t.Fatalf("ByLevelKey indexed %d rows, want %d", len(index), len(rows))
	}
	for _, want := range rows {
		got, ok := index[configtest.RowKey{Level: want.Level, Key: want.Key}]
		if !ok {
			t.Errorf("missing indexed row for (level %q, key %q)", want.Level, want.Key)
			continue
		}
		if got != want {
			t.Errorf("indexed row for (level %q, key %q) = %+v, want %+v", want.Level, want.Key, got, want)
		}
	}
}

// TestByLevelKey_ReportsDuplicateLevelKeyCollision preserves rowSet's original Fatalf
// intent: two rows sharing a (level, key) pair would hide a duplicate or a dropped row,
// so the builder must report a collision rather than silently overwriting.
func TestByLevelKey_ReportsDuplicateLevelKeyCollision(t *testing.T) {
	t.Parallel()

	rows := []config.MetadataRow{
		{Key: "ai_command", Level: config.LevelShared, Default: "claude"},
		{Key: "ai_command", Level: config.LevelShared, Default: "duplicate"},
	}

	index, err := configtest.ByLevelKey(rows)
	if err == nil {
		t.Fatalf("ByLevelKey accepted duplicate (level, key) rows, want a collision error; index = %+v", index)
	}
	if !errors.Is(err, configtest.ErrDuplicateRowKey) {
		t.Errorf("collision error = %v, want errors.Is ErrDuplicateRowKey", err)
	}
}

// TestMustByLevelKey_FoldsMetadataRows proves the *testing.T convenience wrapper folds
// the real config.MetadataRows() SoT into the index — the shape both external test
// packages consume in place of their old local builders. Every SoT row must resolve.
func TestMustByLevelKey_FoldsMetadataRows(t *testing.T) {
	t.Parallel()

	index := configtest.MustByLevelKey(t)

	rows := config.MetadataRows()
	if len(index) != len(rows) {
		t.Fatalf("MustByLevelKey indexed %d rows, want %d (config.MetadataRows count)", len(index), len(rows))
	}
	for _, want := range rows {
		got, ok := index[configtest.RowKey{Level: want.Level, Key: want.Key}]
		if !ok {
			t.Errorf("MustByLevelKey missing SoT row for (level %q, key %q)", want.Level, want.Key)
			continue
		}
		if got != want {
			t.Errorf("MustByLevelKey row for (level %q, key %q) = %+v, want %+v", want.Level, want.Key, got, want)
		}
	}
}
