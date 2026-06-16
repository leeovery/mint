package config

// This is an INTERNAL test file (package config, not config_test) so the reflection
// helper schemaLeafKeys can read the UNEXPORTED decode-shape structs (fileShape /
// releaseShape / commitShape / hooksShape) and their toml tags directly — per the
// 1-1 placement decision (option (a)): the authoritative (level, key) set must be
// derived MECHANICALLY from the real schema, and those shapes are unexported, so the
// derivation has to live in the same package. The helper is test-only infrastructure
// — the sole consumer is the 1-4 drift test (the bijection against the SoT) — so it
// lives in a _test.go file rather than a shipped source file; no other phase reuses
// it. The existing metadata_test.go stays an EXTERNAL (config_test) file untouched.
//
// The helper reads toml tags ONLY — it never consults MetadataRows() / any SoT data —
// so the two sides of the future bijection (1-4) stay independent: a divergence
// between the schema and the SoT is caught because each side is derived from a
// different source (schema tags vs the hand-written SoT rows), not from a shared list.

import (
	"reflect"
	"testing"
)

// leafKey is one derived (level, key) pair: the schema's authoritative bijection unit,
// mirroring the SoT's (Level, Key) row identity. ai_command/timeout are NOT collapsed
// across levels — each (level, key) is a distinct leafKey.
type leafKey struct {
	Level MetadataLevel
	Key   string
}

// containerLevels maps a sub-table CONTAINER field's toml tag to the MetadataLevel its
// nested leaf children take. These three tags (release, commit, hooks) name table
// containers, NOT keys: the walk recurses into them and emits NO pair for the container
// itself, threading the mapped level down to the nested leaves. hooks is reached only
// while recursing through release, so its leaves are LevelReleaseHooks. A field whose
// toml tag is in this map is treated as a container; any other struct-kind field would
// be an unmapped container and is rejected loud (see schemaLeafKeysInto) so a future
// sub-table added to the schema cannot be silently skipped.
var containerLevels = map[string]MetadataLevel{
	"release": LevelRelease,
	"commit":  LevelCommit,
	"hooks":   LevelReleaseHooks,
}

// schemaLeafKeys derives the authoritative set of leaf (level, key) pairs purely from
// the decode-shape struct tags, starting at fileShape (whose own leaf fields are
// LevelShared). It walks every toml-tagged field: a CONTAINER field (tag in
// containerLevels) emits no pair and the walk recurses into its nested struct at the
// mapped level; every other toml-tagged LEAF field (scalar, pointer scalar, HookValue
// interface, or []string collection) emits exactly one (currentLevel, tag) pair. Pairs
// are NOT deduplicated across levels — ai_command/timeout at shared/[release]/[commit]
// are three distinct pairs. The result is exactly the set the 1-4 drift test matches
// against the SoT.
func schemaLeafKeys(t *testing.T) []leafKey {
	t.Helper()

	var out []leafKey
	schemaLeafKeysInto(t, reflect.TypeOf(fileShape{}), LevelShared, &out)
	return out
}

// schemaLeafKeysInto recursively appends the leaf (level, key) pairs of struct type st
// at the given level. CONTAINER detection is tag-driven (the robust approach): a field
// whose toml tag is in containerLevels is a sub-table — emit no pair and recurse into
// its nested struct at the mapped level. Every other toml-tagged field is a leaf — emit
// one (level, tag) pair. The container check is on the TAG, not the field's Go kind, so
// a HookValue field (underlying type the empty interface — kind Interface, not Struct)
// is never mistaken for a sub-shape: its tag (preflight/pre_tag/post_release) is not in
// containerLevels, so it falls to the leaf branch. A struct-kind field whose tag is NOT
// a known container is a schema addition the mapping does not cover — fail loud rather
// than silently mis-classify it.
func schemaLeafKeysInto(t *testing.T, st reflect.Type, level MetadataLevel, out *[]leafKey) {
	t.Helper()

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		tag := field.Tag.Get("toml")
		// Defensive: skip a field with no toml tag (or the explicit "-" skip token).
		// None exist today, but guarding the read keeps the walk from emitting a
		// bogus empty-key pair if a non-decoded field is ever added to a shape.
		if tag == "" || tag == "-" {
			continue
		}

		if nestedLevel, isContainer := containerLevels[tag]; isContainer {
			// Container field: emit NO pair, recurse into the nested struct at the
			// level its tag maps to (hooks, reached via release, is LevelReleaseHooks).
			schemaLeafKeysInto(t, field.Type, nestedLevel, out)
			continue
		}

		// A struct-kind field whose tag is not a known container would be an
		// uncovered sub-table — fail loud so it cannot be silently counted as a leaf.
		if field.Type.Kind() == reflect.Struct {
			t.Fatalf("schemaLeafKeys: struct field %q (toml %q) is not a known container — add it to containerLevels", field.Name, tag)
		}

		*out = append(*out, leafKey{Level: level, Key: tag})
	}
}

// leafKeyCount collapses the derived pairs into a (level, key) → count map and fails on
// any duplicate pair (a collision would mean the walk emitted the same leaf twice).
func leafKeySet(t *testing.T) map[leafKey]int {
	t.Helper()

	out := map[leafKey]int{}
	for _, lk := range schemaLeafKeys(t) {
		out[lk]++
		if out[lk] > 1 {
			t.Fatalf("schemaLeafKeys emitted duplicate pair (level %q, key %q)", lk.Level, lk.Key)
		}
	}
	return out
}

// TestSchemaLeafKeys_DerivesAllLeafPairs proves the helper derives exactly the 25 leaf
// (level, key) pairs from the decode-shape struct tags: 4 shared, 14 [release], 3
// [release.hooks], 4 [commit]. Every expected pair is named so an added/renamed/removed
// struct field surfaces precisely. This is the derivation-correctness counterpart of
// the 1-4 bijection (which matches this derived set against the SoT).
func TestSchemaLeafKeys_DerivesAllLeafPairs(t *testing.T) {
	t.Parallel()

	expected := []leafKey{
		// Shared top-level engine keys (fileShape leaf fields).
		{LevelShared, "ai_command"},
		{LevelShared, "max_diff_lines"},
		{LevelShared, "timeout"},
		{LevelShared, "diff_exclude"},
		// [release] leaf keys (releaseShape leaf fields).
		{LevelRelease, "tag_prefix"},
		{LevelRelease, "commit_prefix"},
		{LevelRelease, "release_branch"},
		{LevelRelease, "publish"},
		{LevelRelease, "changelog"},
		{LevelRelease, "provider"},
		{LevelRelease, "context"},
		{LevelRelease, "prompt"},
		{LevelRelease, "on_notes_failure"},
		{LevelRelease, "fallback"},
		{LevelRelease, "version_file"},
		{LevelRelease, "version_pattern"},
		{LevelRelease, "ai_command"},
		{LevelRelease, "timeout"},
		// [release.hooks] keys (hooksShape fields, reached via release recursion).
		{LevelReleaseHooks, "preflight"},
		{LevelReleaseHooks, "pre_tag"},
		{LevelReleaseHooks, "post_release"},
		// [commit] keys (commitShape leaf fields).
		{LevelCommit, "context"},
		{LevelCommit, "prompt"},
		{LevelCommit, "ai_command"},
		{LevelCommit, "timeout"},
	}

	got := schemaLeafKeys(t)
	if len(got) != len(expected) {
		t.Fatalf("schemaLeafKeys() returned %d pairs, want %d", len(got), len(expected))
	}

	set := leafKeySet(t)
	for _, want := range expected {
		if set[want] != 1 {
			t.Errorf("missing derived leaf pair (level %q, key %q)", want.Level, want.Key)
		}
	}
}

// TestSchemaLeafKeys_NoContainerPairs proves the walk emits ZERO pairs for the three
// sub-table CONTAINER tags (release, commit, hooks) — recurse-don't-count. The
// container fields hold keys; they are not keys. This is the inverse of the dual-level
// case (a container maps to zero pairs + recursion; a dual-level key maps to one pair
// per level).
func TestSchemaLeafKeys_NoContainerPairs(t *testing.T) {
	t.Parallel()

	containers := map[string]bool{"release": true, "commit": true, "hooks": true}
	for _, lk := range schemaLeafKeys(t) {
		if containers[lk.Key] {
			t.Errorf("walk must emit no pair for container key %q (level %q)", lk.Key, lk.Level)
		}
	}
}

// TestSchemaLeafKeys_ReleaseLeavesTaggedAtReleaseLevel proves the leaf keys inside the
// [release] container take LevelRelease — the level comes from the release container
// tag threaded down through recursion, NOT inferred from the leaf field name. A
// [release]-only leaf (tag_prefix) is asserted at LevelRelease and NOT at LevelShared.
func TestSchemaLeafKeys_ReleaseLeavesTaggedAtReleaseLevel(t *testing.T) {
	t.Parallel()

	set := leafKeySet(t)
	if set[leafKey{LevelRelease, "tag_prefix"}] != 1 {
		t.Errorf("tag_prefix must be derived at LevelRelease")
	}
	if set[leafKey{LevelShared, "tag_prefix"}] != 0 {
		t.Errorf("tag_prefix must not appear at LevelShared — its level comes from the release container tag")
	}
}

// TestSchemaLeafKeys_HooksLeavesTaggedAtHooksLevel proves the three [release.hooks] leaf
// keys (preflight, pre_tag, post_release) take LevelReleaseHooks — the nested-tag level
// supplied by the hooks container, which is itself reached only by recursing through
// release. HookValue fields (interface-kind) are leaves, never mistaken for containers.
func TestSchemaLeafKeys_HooksLeavesTaggedAtHooksLevel(t *testing.T) {
	t.Parallel()

	set := leafKeySet(t)
	for _, key := range []string{"preflight", "pre_tag", "post_release"} {
		if set[leafKey{LevelReleaseHooks, key}] != 1 {
			t.Errorf("hooks key %q must be derived at LevelReleaseHooks", key)
		}
	}
}

// TestSchemaLeafKeys_AICommandAndTimeoutAtAllThreeLevels proves the dual-level keys
// ai_command and timeout each derive as THREE distinct pairs — shared, [release],
// [commit] — never collapsed (no cross-level dedup). This is the inverse of the
// recurse-don't-count rule.
func TestSchemaLeafKeys_AICommandAndTimeoutAtAllThreeLevels(t *testing.T) {
	t.Parallel()

	set := leafKeySet(t)
	for _, key := range []string{"ai_command", "timeout"} {
		for _, level := range []MetadataLevel{LevelShared, LevelRelease, LevelCommit} {
			if set[leafKey{level, key}] != 1 {
				t.Errorf("%q must be derived once at level %q", key, level)
			}
		}
	}
}

// TestSchemaLeafKeys_IndependentOfSoT proves the derived set comes purely from the
// decode-shape struct tags, NOT from MetadataRows(): the helper must remain a sound
// independent side of the 1-4 bijection. We confirm independence positively — every
// derived pair maps back to a real toml-tagged field on the named shape — so the
// derivation could not have been seeded from the SoT slice.
func TestSchemaLeafKeys_IndependentOfSoT(t *testing.T) {
	t.Parallel()

	// Build the authoritative tag set straight from the shapes (the same source the
	// helper reads), keyed by level, and confirm every derived pair is present in it.
	tagsByLevel := map[MetadataLevel]map[string]bool{
		LevelShared:       tomlTagsOf(reflect.TypeOf(fileShape{})),
		LevelRelease:      tomlTagsOf(reflect.TypeOf(releaseShape{})),
		LevelReleaseHooks: tomlTagsOf(reflect.TypeOf(hooksShape{})),
		LevelCommit:       tomlTagsOf(reflect.TypeOf(commitShape{})),
	}

	for _, lk := range schemaLeafKeys(t) {
		if !tagsByLevel[lk.Level][lk.Key] {
			t.Errorf("derived pair (level %q, key %q) has no matching toml tag on its shape — derivation is not schema-sourced", lk.Level, lk.Key)
		}
	}
}

// tomlTagsOf returns the toml tag names of st's fields (container tags included) — a
// schema-sourced lookup used by the independence test to confirm the helper derives
// from struct tags rather than the SoT.
func tomlTagsOf(st reflect.Type) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < st.NumField(); i++ {
		if tag := st.Field(i).Tag.Get("toml"); tag != "" && tag != "-" {
			out[tag] = true
		}
	}
	return out
}
