package config

// This is an INTERNAL test file (package config) carrying the SINGLE ordered census of
// the expected leaf (level, key) pairs — the third copy that previously lived inline in
// BOTH TestSchemaLeafKeys_DerivesAllLeafPairs (metadata_drift_test.go, package config)
// and TestMetadataRows_OneRowPerLevelKeyPair (metadata_test.go, package config_test).
// Those two naming tests sit in DIFFERENT test packages (internal vs external), so the
// census has to be reachable from both: it is published through the EXPORTED accessor
// ExpectedLeafKeys, which the internal drift test consumes directly and the external
// config_test test projects into its own rowKey type. The accessor lives in a _test.go
// file, so the exported symbol never ships in the production package — it is test-only
// support. leafKey carries EXPORTED fields (Level, Key), so the external package can read
// each pair's components even though it cannot name the unexported leafKey type.
//
// This consolidates ONLY the hand-written EXPECTED enumeration. The two DERIVATIONS the
// bijection guards — MetadataRows() (the SoT side) and schemaLeafKeys() (the reflection
// side) — stay untouched and independent: the drift test still compares two sources
// derived from DIFFERENT origins (hand-written rows vs schema tags). Merging only the
// expected census keeps that anti-drift value intact while removing the editor-trap of a
// duplicate list that could silently diverge.

// expectedLeafKeys is the one ordered source of the 25 expected (level, key) pairs
// spanning every config key: 4 shared, 14 [release], 3 [release.hooks], 4 [commit]. Both
// naming tests assert against this single list — a schema key added, renamed, or removed
// is now edited here ONCE, and the two tests stay in lockstep automatically.
var expectedLeafKeys = []leafKey{
	// Shared top-level engine keys.
	{LevelShared, "ai_command"},
	{LevelShared, "max_diff_lines"},
	{LevelShared, "timeout"},
	{LevelShared, "diff_exclude"},
	// [release] leaf keys.
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
	// [release.hooks] keys.
	{LevelReleaseHooks, "preflight"},
	{LevelReleaseHooks, "pre_tag"},
	{LevelReleaseHooks, "post_release"},
	// [commit] keys.
	{LevelCommit, "context"},
	{LevelCommit, "prompt"},
	{LevelCommit, "ai_command"},
	{LevelCommit, "timeout"},
}

// ExpectedLeafKeys returns a fresh copy of the shared expected-pair census so neither the
// internal drift test nor the external config_test can mutate the backing slice. It is
// EXPORTED solely so the external config_test naming test (which cannot see the unexported
// expectedLeafKeys var) can project it into its own rowKey type; the symbol is test-only
// (it lives in a _test.go file) and never ships in the production build.
func ExpectedLeafKeys() []leafKey {
	return append([]leafKey(nil), expectedLeafKeys...)
}
