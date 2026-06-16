// Package configtest is the single test-support seam for indexing the config-metadata
// SoT (config.MetadataRows()) by its (level, key) row identity. It mirrors the stdlib
// net/nettest, testing/fstest, testing/iotest idiom: a normal importable package whose
// only consumers are _test.go files.
//
// WHY this exists as its own package rather than a config.MetadataByLevelKey() export:
// the (level, key) index builder was duplicated across TWO external test packages —
// config_test (metadata_test.go's rowKey + rowSet) and setupguide_test
// (setupguide_test.go's rowKey + rowByLevelKey). A symbol in a _test.go file compiles
// only into THAT package's test binary, so it cannot be shared across packages; the
// shared seam must live in an importable (non-_test.go) package. Option A — widening the
// production config API with an exported indexer purely for test convenience — would
// ship in the mint binary and violate this repo's strict minimal-production-surface
// discipline (CLAUDE.md). This package is the test-only alternative: because NOTHING in
// production imports it, it is never linked into the mint binary — zero production cost —
// while keeping the RowKey identity and the indexing builder single-sourced next to the
// SoT it indexes (the config package).
package configtest

import (
	"errors"
	"fmt"
	"testing"

	"mint/internal/config"
)

// RowKey identifies a config-metadata SoT row by its (Level, Key) pair — the bijection
// unit the drift contract matches on. ai_command and timeout are NOT collapsed across
// levels: each (level, key) pair is a distinct row, so the index is keyed on both. The
// fields use the EXPORTED config.MetadataLevel + key string so external test packages
// can both construct and read a RowKey.
type RowKey struct {
	Level config.MetadataLevel
	Key   string
}

// ErrDuplicateRowKey is returned by ByLevelKey when two rows share a (level, key) pair.
// A collision would hide a duplicate or a dropped row, so the builder refuses to fold it
// silently — preserving rowSet's original Fatalf intent at the seam.
var ErrDuplicateRowKey = errors.New("duplicate SoT row for a (level, key) pair")

// ByLevelKey folds a config.MetadataRow slice into a (level, key) → row lookup, indexing
// every row under its RowKey. It returns ErrDuplicateRowKey if two rows share a (level,
// key) pair. This is the pure, error-returning core so a dedicated test can prove the
// collision is detected; the *testing.T-taking MustByLevelKey wraps it for the common
// "index the real SoT, fail the test on a collision" call shape both test packages use.
func ByLevelKey(rows []config.MetadataRow) (map[RowKey]config.MetadataRow, error) {
	out := make(map[RowKey]config.MetadataRow, len(rows))
	for _, row := range rows {
		k := RowKey{Level: row.Level, Key: row.Key}
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("%w: (level %q, key %q)", ErrDuplicateRowKey, row.Level, row.Key)
		}
		out[k] = row
	}
	return out, nil
}

// MustByLevelKey indexes the live config.MetadataRows() SoT by (level, key), failing the
// test via t.Fatalf on a duplicate-pair collision. This is the single seam both
// config_test and setupguide_test consume in place of their former local rowSet /
// rowByLevelKey builders — a change to the SoT row identity model is now made once, here.
func MustByLevelKey(t *testing.T) map[RowKey]config.MetadataRow {
	t.Helper()

	index, err := ByLevelKey(config.MetadataRows())
	if err != nil {
		t.Fatalf("indexing config.MetadataRows() by (level, key): %v", err)
	}
	return index
}
