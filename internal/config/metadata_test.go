package config_test

import (
	"testing"

	"mint/internal/config"
)

// rowKey identifies a SoT row by its (level, key) pair — the bijection unit the
// drift contract matches on. ai_command and timeout are NOT collapsed across levels:
// each (level, key) pair is a distinct row, so the map is keyed on both.
type rowKey struct {
	level config.MetadataLevel
	key   string
}

// rowSet collapses MetadataRows() into a (level, key) → row lookup, asserting no two
// rows share a (level, key) pair (a collision would hide a duplicate or a dropped row).
func rowSet(t *testing.T) map[rowKey]config.MetadataRow {
	t.Helper()

	out := map[rowKey]config.MetadataRow{}
	for _, row := range config.MetadataRows() {
		k := rowKey{level: row.Level, key: row.Key}
		if _, dup := out[k]; dup {
			t.Fatalf("duplicate SoT row for (level %q, key %q)", row.Level, row.Key)
		}
		out[k] = row
	}
	return out
}

// TestMetadataRows_OneRowPerLevelKeyPair pins the SoT to exactly the 25 (level, key)
// pairs spanning every config key: 4 shared, 14 [release], 3 [release.hooks], 4
// [commit]. The count 25 is a sanity check here (the drift test in 1-4 is the real
// guard against schema divergence); the value of THIS test is naming every expected
// pair so a dropped or renamed key surfaces precisely.
func TestMetadataRows_OneRowPerLevelKeyPair(t *testing.T) {
	t.Parallel()

	expected := []rowKey{
		// Shared top-level engine keys.
		{config.LevelShared, "ai_command"},
		{config.LevelShared, "max_diff_lines"},
		{config.LevelShared, "timeout"},
		{config.LevelShared, "diff_exclude"},
		// [release] leaf keys.
		{config.LevelRelease, "tag_prefix"},
		{config.LevelRelease, "commit_prefix"},
		{config.LevelRelease, "release_branch"},
		{config.LevelRelease, "publish"},
		{config.LevelRelease, "changelog"},
		{config.LevelRelease, "provider"},
		{config.LevelRelease, "context"},
		{config.LevelRelease, "prompt"},
		{config.LevelRelease, "on_notes_failure"},
		{config.LevelRelease, "fallback"},
		{config.LevelRelease, "version_file"},
		{config.LevelRelease, "version_pattern"},
		{config.LevelRelease, "ai_command"},
		{config.LevelRelease, "timeout"},
		// [release.hooks] keys.
		{config.LevelReleaseHooks, "preflight"},
		{config.LevelReleaseHooks, "pre_tag"},
		{config.LevelReleaseHooks, "post_release"},
		// [commit] keys.
		{config.LevelCommit, "context"},
		{config.LevelCommit, "prompt"},
		{config.LevelCommit, "ai_command"},
		{config.LevelCommit, "timeout"},
	}

	rows := config.MetadataRows()
	if len(rows) != len(expected) {
		t.Fatalf("MetadataRows() returned %d rows, want %d", len(rows), len(expected))
	}

	set := rowSet(t)
	for _, want := range expected {
		if _, ok := set[want]; !ok {
			t.Errorf("missing SoT row for (level %q, key %q)", want.level, want.key)
		}
	}
}

// TestMetadataRows_AICommandTriLevel proves ai_command appears as THREE distinct rows
// — shared, [release], [commit] — never collapsed to one. Row identity is the
// (level, key) pair, mirroring the README's per-level model.
func TestMetadataRows_AICommandTriLevel(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	for _, level := range []config.MetadataLevel{config.LevelShared, config.LevelRelease, config.LevelCommit} {
		if _, ok := set[rowKey{level: level, key: "ai_command"}]; !ok {
			t.Errorf("missing ai_command row at level %q", level)
		}
	}
}

// TestMetadataRows_TimeoutTriLevel proves timeout appears as THREE distinct rows —
// shared, [release], [commit] — never collapsed to one (the timeout twin of the
// ai_command tri-level case).
func TestMetadataRows_TimeoutTriLevel(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	for _, level := range []config.MetadataLevel{config.LevelShared, config.LevelRelease, config.LevelCommit} {
		if _, ok := set[rowKey{level: level, key: "timeout"}]; !ok {
			t.Errorf("missing timeout row at level %q", level)
		}
	}
}

// TestMetadataRows_NoContainerRows proves the table emits ZERO rows for the sub-table
// CONTAINER fields (release, commit, hooks) — they are not keys, they hold keys. This
// is the inverse of the dual-level case: a container maps to no row (its children
// carry their own level explicitly), whereas ai_command maps to one row per level.
func TestMetadataRows_NoContainerRows(t *testing.T) {
	t.Parallel()

	containers := map[string]bool{"release": true, "commit": true, "hooks": true}
	for _, row := range config.MetadataRows() {
		if containers[row.Key] {
			t.Errorf("SoT must not emit a row for container key %q (level %q)", row.Key, row.Level)
		}
	}
}

// TestMetadataRows_EveryRowHasDescription proves every row carries a non-empty
// one-line description (the meaning column the mint setup config reference renders).
func TestMetadataRows_EveryRowHasDescription(t *testing.T) {
	t.Parallel()

	for _, row := range config.MetadataRows() {
		if row.Description == "" {
			t.Errorf("SoT row (level %q, key %q) has an empty description", row.Level, row.Key)
		}
	}
}

// TestMetadataLevel_String pins each level's TOML rendering so the render target and
// the drift test agree on level identity: shared is the empty top-level form, the
// others render their bracketed table headers.
func TestMetadataLevel_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    config.MetadataLevel
		expected string
	}{
		{"shared renders the empty top-level form", config.LevelShared, ""},
		{"release renders its table header", config.LevelRelease, "[release]"},
		{"release hooks renders the nested header", config.LevelReleaseHooks, "[release.hooks]"},
		{"commit renders its table header", config.LevelCommit, "[commit]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.level.String(); got != tt.expected {
				t.Errorf("%v.String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}
