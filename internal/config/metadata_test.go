package config_test

import (
	"strconv"
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

// hooksDefaultCell is the chosen sentinel-blank representation for the
// [release.hooks] keys (preflight, pre_tag, post_release): the spec offers blank
// OR em-dash; we pick the EMPTY STRING (blank), matching the README's
// [release.hooks] table, which carries no default column at all. A hooks key is
// activate-only — it has no compiled default — so a blank cell (the same token
// used for empty-string-default keys) is the honest representation; the
// description carries when each hook runs. This constant pins the choice so the
// hooks-default test and the blank-vs-auto test agree on the same token.
const hooksDefaultCell = ""

// TestMetadataRows_EmptyStringDefaultsRenderBlank proves the keys whose compiled
// default is the empty string — and whose empty MEANS "no value" (not auto) —
// render a BLANK default cell: [release] context/prompt/fallback/version_file/
// version_pattern and [commit] context/prompt. This is the genuinely-no-value
// case, distinct from the sentinel-auto keys asserted below.
func TestMetadataRows_EmptyStringDefaultsRenderBlank(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	blank := []rowKey{
		{config.LevelRelease, "context"},
		{config.LevelRelease, "prompt"},
		{config.LevelRelease, "fallback"},
		{config.LevelRelease, "version_file"},
		{config.LevelRelease, "version_pattern"},
		{config.LevelCommit, "context"},
		{config.LevelCommit, "prompt"},
	}

	for _, want := range blank {
		row, ok := set[want]
		if !ok {
			t.Fatalf("missing SoT row for (level %q, key %q)", want.level, want.key)
		}
		if row.Default != "" {
			t.Errorf("(level %q, key %q) Default = %q, want blank", want.level, want.key, row.Default)
		}
	}
}

// TestMetadataRows_SentinelAutoDefaultsRenderAuto proves release_branch and
// provider render the word "auto" — NOT a blank cell — even though both default
// to "" in the Go struct. Their empty string is a SENTINEL meaning auto-derive
// (release_branch from origin/HEAD) / auto-detect (provider from the remote host),
// so the SoT renders "auto" so the agent can tell "auto" from "no value". The
// second assertion pins the load-bearing distinction: these cells are NOT blank.
func TestMetadataRows_SentinelAutoDefaultsRenderAuto(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	auto := []rowKey{
		{config.LevelRelease, "release_branch"},
		{config.LevelRelease, "provider"},
	}

	for _, want := range auto {
		row, ok := set[want]
		if !ok {
			t.Fatalf("missing SoT row for (level %q, key %q)", want.level, want.key)
		}
		if row.Default != "auto" {
			t.Errorf("(level %q, key %q) Default = %q, want %q", want.level, want.key, row.Default, "auto")
		}
		// The blank-vs-auto distinction is load-bearing: assert NOT blank so a
		// regression collapsing auto into the empty-string token is caught.
		if row.Default == "" {
			t.Errorf("(level %q, key %q) Default is blank, must be distinct from the empty-string keys", want.level, want.key)
		}
	}
}

// TestMetadataRows_DiffExcludeRendersEmptyCollection proves the empty-collection
// default (diff_exclude) renders the literal "[]" token — distinct from both
// blank and auto — matching the README's diff_exclude cell.
func TestMetadataRows_DiffExcludeRendersEmptyCollection(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	row, ok := set[rowKey{config.LevelShared, "diff_exclude"}]
	if !ok {
		t.Fatal("missing SoT row for (shared, diff_exclude)")
	}
	if row.Default != "[]" {
		t.Errorf("(shared, diff_exclude) Default = %q, want %q", row.Default, "[]")
	}
}

// TestMetadataRows_PerVerbOverridesRenderShared proves the four per-verb
// ai_command/timeout override rows ([release].ai_command/timeout,
// [commit].ai_command/timeout) render the word "shared" — they INHERIT the shared
// top-level value unless overridden, so their default is "inherit", not the real
// compiled value. This is the inverse of the shared-level rows (asserted below),
// where the real default lives.
func TestMetadataRows_PerVerbOverridesRenderShared(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	inherit := []rowKey{
		{config.LevelRelease, "ai_command"},
		{config.LevelRelease, "timeout"},
		{config.LevelCommit, "ai_command"},
		{config.LevelCommit, "timeout"},
	}

	for _, want := range inherit {
		row, ok := set[want]
		if !ok {
			t.Fatalf("missing SoT row for (level %q, key %q)", want.level, want.key)
		}
		if row.Default != "shared" {
			t.Errorf("(level %q, key %q) Default = %q, want %q", want.level, want.key, row.Default, "shared")
		}
	}
}

// TestMetadataRows_HooksRenderNoDefault proves the three [release.hooks] keys
// (preflight, pre_tag, post_release) carry the SAME no-default token consistently
// — the chosen blank cell (hooksDefaultCell). Hooks are activate-only: no compiled
// default, the description carries when each runs.
func TestMetadataRows_HooksRenderNoDefault(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	hooks := []string{"preflight", "pre_tag", "post_release"}

	for _, key := range hooks {
		row, ok := set[rowKey{config.LevelReleaseHooks, key}]
		if !ok {
			t.Fatalf("missing SoT row for (release.hooks, %q)", key)
		}
		if row.Default != hooksDefaultCell {
			t.Errorf("(release.hooks, %q) Default = %q, want %q (consistent no-default token)", key, row.Default, hooksDefaultCell)
		}
	}
}

// TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim proves every key with a
// concrete scalar default renders that default's string form verbatim. The
// [release] scalar defaults are asserted against literals (their config constants
// are unexported, so the external test cannot reference them — the value-drift
// pin for those moves to the schema drift test in 1-4); the shared ai_command/
// timeout literals are pinned against the EXPORTED config constants so they
// cannot drift (task 1-5 adds the dedicated pinning test for those two).
func TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim(t *testing.T) {
	t.Parallel()

	set := rowSet(t)
	tests := []struct {
		key  rowKey
		want string
	}{
		{rowKey{config.LevelShared, "ai_command"}, config.DefaultAICommand},
		{rowKey{config.LevelShared, "timeout"}, strconv.Itoa(int(config.DefaultTimeout.Seconds()))},
		{rowKey{config.LevelShared, "max_diff_lines"}, "50000"},
		{rowKey{config.LevelRelease, "tag_prefix"}, "v"},
		{rowKey{config.LevelRelease, "commit_prefix"}, "🌿"},
		{rowKey{config.LevelRelease, "publish"}, "true"},
		{rowKey{config.LevelRelease, "changelog"}, "true"},
		{rowKey{config.LevelRelease, "on_notes_failure"}, "abort"},
	}

	for _, tt := range tests {
		row, ok := set[tt.key]
		if !ok {
			t.Fatalf("missing SoT row for (level %q, key %q)", tt.key.level, tt.key.key)
		}
		if row.Default != tt.want {
			t.Errorf("(level %q, key %q) Default = %q, want %q", tt.key.level, tt.key.key, row.Default, tt.want)
		}
	}
}
