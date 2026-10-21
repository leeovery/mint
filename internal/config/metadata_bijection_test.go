package config

// This is an INTERNAL test file (package config) so it can reuse the unexported
// leafKey type and the schemaLeafKeys reflection helper from metadata_drift_test.go
// (task 1-3). It carries the 1-4 DRIFT TEST: a TOTAL BIJECTION over leaf (level, key)
// pairs between the config-metadata SoT (MetadataRows, tasks 1-1/1-2) and the
// mechanically-derived schema leaf-key set (schemaLeafKeys, 1-3).
//
// The bijection is the anti-drift enforcement: every schema leaf has exactly one SoT
// row at its level, every SoT row maps back to exactly one schema leaf, and no SoT row
// is duplicated. A future schema edit (a new/renamed/removed key) that does not update
// the SoT fails the build here, naming the offending (level, key) pair — mirroring the
// existing initgen↔config drift discipline (initgen_test.go's value-pin tests).
//
// The comparison is factored into the PURE helper bijectionDiff so the guard mechanism
// itself is unit-tested against SYNTHETIC divergent inputs (a dropped pair, a phantom
// pair, a duplicated pair) without mutating the real SoT or the real decode-shape
// structs — the fixed structs cannot be edited per-test, so the divergence cases prove
// the comparison MECHANISM catches drift, and the real test proves the two real sides
// agree today.

import "testing"

// bijectionDiff is the PURE comparison helper at the heart of the drift test: given the
// SoT pair set and the schema pair set, it returns the three divergence classes —
//
//   - missingFromSoT: schema leaves with no matching SoT row (a new/renamed schema key
//     the SoT does not list);
//   - missingFromSchema: SoT rows with no matching schema leaf (a removed/renamed key
//     still listed in the SoT);
//   - duplicates: (level, key) pairs appearing more than once in the SoT.
//
// Matching is strictly on the FULL (level, key) pair, never the bare key — so the three
// legitimate ai_command/timeout rows at distinct levels are distinct pairs, never
// duplicates. Keeping the comparison pure (two slices in, three slices out) lets the
// guard mechanism be unit-tested against synthetic divergent inputs without mutating the
// real SoT or the decode-shape structs. Each returned slice carries DISTINCT pairs (a
// pair appears at most once per slice) so the failure messages name each offender once.
func bijectionDiff(sot, schema []leafKey) (missingFromSoT, missingFromSchema, duplicates []leafKey) {
	// Count SoT occurrences per pair: presence drives the bijection, count > 1 drives
	// duplicate detection (over the full pair, so same-named cross-level rows are fine).
	sotCounts := map[leafKey]int{}
	for _, lk := range sot {
		sotCounts[lk]++
	}
	schemaSet := map[leafKey]bool{}
	for _, lk := range schema {
		schemaSet[lk] = true
	}

	// Schema leaves with no SoT row — iterate the schema slice (not the dedup set) so the
	// reported order follows the schema field order; guard against a duplicated schema
	// pair reporting the same offender twice.
	reportedMissingFromSoT := map[leafKey]bool{}
	for _, lk := range schema {
		if sotCounts[lk] == 0 && !reportedMissingFromSoT[lk] {
			reportedMissingFromSoT[lk] = true
			missingFromSoT = append(missingFromSoT, lk)
		}
	}

	// SoT rows with no schema leaf, and SoT pairs listed more than once — iterate the SoT
	// slice in row order, reporting each offending pair exactly once.
	reportedMissingFromSchema := map[leafKey]bool{}
	reportedDuplicate := map[leafKey]bool{}
	for _, lk := range sot {
		if !schemaSet[lk] && !reportedMissingFromSchema[lk] {
			reportedMissingFromSchema[lk] = true
			missingFromSchema = append(missingFromSchema, lk)
		}
		if sotCounts[lk] > 1 && !reportedDuplicate[lk] {
			reportedDuplicate[lk] = true
			duplicates = append(duplicates, lk)
		}
	}

	return missingFromSoT, missingFromSchema, duplicates
}

// sotLeafKeys projects MetadataRows() to the (Level, Key) pairs — the SoT side of the
// bijection. It reads ONLY the SoT rows (never the schema), keeping the two sides of
// the bijection independent: divergence is caught because each side derives from a
// different source (hand-written rows vs schema tags).
func sotLeafKeys() []leafKey {
	rows := MetadataRows()
	out := make([]leafKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, leafKey{Level: r.Level, Key: r.Key})
	}
	return out
}

// TestMetadataSoT_BijectsSchemaLeafKeys is the DRIFT GUARD: the SoT rows and the
// mechanically-derived schema leaf-key set must be a TOTAL BIJECTION over (level, key)
// pairs. Any divergence — a schema leaf with no SoT row, an SoT row with no schema leaf,
// or a duplicate SoT row — fails the build naming the offender.
func TestMetadataSoT_BijectsSchemaLeafKeys(t *testing.T) {
	t.Parallel()

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sotLeafKeys(), schemaLeafKeys(t))

	for _, lk := range missingFromSoT {
		t.Errorf("schema leaf (level %q, key %q) has no matching SoT row — add the metadata row (or it is a newly added/renamed schema key)", lk.Level.String(), lk.Key)
	}
	for _, lk := range missingFromSchema {
		t.Errorf("SoT row (level %q, key %q) has no matching schema leaf — remove the metadata row (or it is a removed/renamed schema key still in the SoT)", lk.Level.String(), lk.Key)
	}
	for _, lk := range duplicates {
		t.Errorf("duplicate SoT row for (level %q, key %q) — each (level, key) pair must appear exactly once", lk.Level.String(), lk.Key)
	}
}

// TestMetadataSoT_BijectionPairCount sanity-pins the current schema/SoT size so a future
// reader sees the expected scale: 25 leaf (level, key) pairs on both sides, matched
// exactly. The bijection above is the authoritative guard; this asserts the shared count
// the two sides agree on today.
func TestMetadataSoT_BijectionPairCount(t *testing.T) {
	t.Parallel()

	const wantPairs = 25
	if got := len(sotLeafKeys()); got != wantPairs {
		t.Errorf("SoT projected to %d (level, key) pairs, want %d", got, wantPairs)
	}
	if got := len(schemaLeafKeys(t)); got != wantPairs {
		t.Errorf("schema derived %d (level, key) pairs, want %d", got, wantPairs)
	}
}

// TestBijectionDiff_CleanWhenEqual proves the pure helper reports NO divergence when the
// two pair sets are identical — the property the real bijection relies on.
func TestBijectionDiff_CleanWhenEqual(t *testing.T) {
	t.Parallel()

	pairs := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "tag_prefix"},
		{LevelCommit, "ai_command"},
	}
	// Pass a fresh copy as the schema side so the helper compares values, not identity.
	schema := append([]leafKey(nil), pairs...)

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(pairs, schema)

	if len(missingFromSoT) != 0 || len(missingFromSchema) != 0 || len(duplicates) != 0 {
		t.Errorf("identical sets must report no divergence, got missingFromSoT=%v missingFromSchema=%v duplicates=%v", missingFromSoT, missingFromSchema, duplicates)
	}
}

// TestBijectionDiff_SchemaLeafMissingFromSoT proves the helper flags a schema leaf with
// no SoT row (a newly added/renamed schema key the SoT does not list) — reported in
// missingFromSoT, the others empty.
func TestBijectionDiff_SchemaLeafMissingFromSoT(t *testing.T) {
	t.Parallel()

	// SoT drops the [commit] ai_command pair the schema still carries.
	sot := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "ai_command"},
	}
	schema := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "ai_command"},
		{LevelCommit, "ai_command"},
	}

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sot, schema)

	if !containsPair(missingFromSoT, leafKey{LevelCommit, "ai_command"}) {
		t.Errorf("missingFromSoT must name the unmatched schema leaf (LevelCommit, ai_command), got %v", missingFromSoT)
	}
	if len(missingFromSoT) != 1 {
		t.Errorf("missingFromSoT must contain exactly the one unmatched schema leaf, got %v", missingFromSoT)
	}
	if len(missingFromSchema) != 0 || len(duplicates) != 0 {
		t.Errorf("only a schema-missing-from-SoT divergence expected, got missingFromSchema=%v duplicates=%v", missingFromSchema, duplicates)
	}
}

// TestBijectionDiff_SoTRowMissingFromSchema proves the helper flags an SoT row with no
// schema leaf (a removed/renamed key still listed in the SoT) — reported in
// missingFromSchema, the others empty.
func TestBijectionDiff_SoTRowMissingFromSchema(t *testing.T) {
	t.Parallel()

	// SoT carries a phantom [release] key the schema has no leaf for.
	sot := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "removed_key"},
	}
	schema := []leafKey{
		{LevelShared, "ai_command"},
	}

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sot, schema)

	if !containsPair(missingFromSchema, leafKey{LevelRelease, "removed_key"}) {
		t.Errorf("missingFromSchema must name the orphaned SoT row (LevelRelease, removed_key), got %v", missingFromSchema)
	}
	if len(missingFromSchema) != 1 {
		t.Errorf("missingFromSchema must contain exactly the one orphaned SoT row, got %v", missingFromSchema)
	}
	if len(missingFromSoT) != 0 || len(duplicates) != 0 {
		t.Errorf("only an SoT-missing-from-schema divergence expected, got missingFromSoT=%v duplicates=%v", missingFromSoT, duplicates)
	}
}

// TestBijectionDiff_DuplicateSoTRow proves the helper flags a duplicate SoT row for one
// (level, key) pair — reported in duplicates. The duplicated pair still matches the
// schema (it is present once there), so it is NOT reported as missing on either side.
func TestBijectionDiff_DuplicateSoTRow(t *testing.T) {
	t.Parallel()

	sot := []leafKey{
		{LevelShared, "ai_command"},
		{LevelShared, "ai_command"}, // duplicate row for the same pair
		{LevelRelease, "tag_prefix"},
	}
	schema := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "tag_prefix"},
	}

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sot, schema)

	if !containsPair(duplicates, leafKey{LevelShared, "ai_command"}) {
		t.Errorf("duplicates must name the repeated pair (LevelShared, ai_command), got %v", duplicates)
	}
	if len(duplicates) != 1 {
		t.Errorf("duplicates must contain exactly the one repeated pair, got %v", duplicates)
	}
	if len(missingFromSoT) != 0 || len(missingFromSchema) != 0 {
		t.Errorf("a duplicate of a matched pair must not register as missing on either side, got missingFromSoT=%v missingFromSchema=%v", missingFromSoT, missingFromSchema)
	}
}

// TestBijectionDiff_DuplicateIsPerPairNotPerKey proves duplicate detection is over the
// full (level, key) PAIR, not the bare key name: the three legitimate ai_command rows
// (shared, [release], [commit]) are DISTINCT pairs and must NOT register as duplicates.
func TestBijectionDiff_DuplicateIsPerPairNotPerKey(t *testing.T) {
	t.Parallel()

	sot := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "ai_command"},
		{LevelCommit, "ai_command"},
	}
	schema := append([]leafKey(nil), sot...)

	_, _, duplicates := bijectionDiff(sot, schema)

	if len(duplicates) != 0 {
		t.Errorf("three same-named ai_command rows at distinct levels are not duplicates, got %v", duplicates)
	}
}

// TestBijectionDiff_MatchesPerLevelNotBareKey proves the bijection matches on the
// (level, key) PAIR, never the bare key: an SoT that lists timeout at shared and
// [release] but FORGETS [commit] fails even though the bare name "timeout" exists in the
// SoT. The forgotten [commit] timeout surfaces as a per-level mismatch.
func TestBijectionDiff_MatchesPerLevelNotBareKey(t *testing.T) {
	t.Parallel()

	sot := []leafKey{
		{LevelShared, "timeout"},
		{LevelRelease, "timeout"},
		// [commit] timeout deliberately omitted.
	}
	schema := []leafKey{
		{LevelShared, "timeout"},
		{LevelRelease, "timeout"},
		{LevelCommit, "timeout"},
	}

	missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sot, schema)

	if !containsPair(missingFromSoT, leafKey{LevelCommit, "timeout"}) {
		t.Errorf("the forgotten (LevelCommit, timeout) pair must surface as missing from the SoT even though the bare key timeout exists, got %v", missingFromSoT)
	}
	if len(missingFromSchema) != 0 || len(duplicates) != 0 {
		t.Errorf("only the per-level [commit] timeout mismatch expected, got missingFromSchema=%v duplicates=%v", missingFromSchema, duplicates)
	}
}

// TestMetadataSoT_DualLevelRowsMatchIndependently proves each of the three dual-level
// rows (ai_command, timeout × shared/[release]/[commit]) matches its OWN level's schema
// leaf independently: dropping any single dual-level pair from the SoT side fails the
// bijection (the dropped pair surfaces missing from the SoT) while every other pair —
// including the same-named rows at other levels — still matches. This is asserted
// through the pure helper against a copy of the real SoT with one pair removed, so the
// real SoT is never mutated.
func TestMetadataSoT_DualLevelRowsMatchIndependently(t *testing.T) {
	t.Parallel()

	schema := schemaLeafKeys(t)
	dualLevel := []leafKey{
		{LevelShared, "ai_command"},
		{LevelRelease, "ai_command"},
		{LevelCommit, "ai_command"},
		{LevelShared, "timeout"},
		{LevelRelease, "timeout"},
		{LevelCommit, "timeout"},
	}

	for _, dropped := range dualLevel {
		t.Run(dropped.Level.String()+"/"+dropped.Key, func(t *testing.T) {
			t.Parallel()

			sot := removePair(sotLeafKeys(), dropped)

			missingFromSoT, missingFromSchema, duplicates := bijectionDiff(sot, schema)

			if !containsPair(missingFromSoT, dropped) {
				t.Errorf("dropping (level %q, key %q) from the SoT must fail the bijection at that exact pair, got missingFromSoT=%v", dropped.Level.String(), dropped.Key, missingFromSoT)
			}
			if len(missingFromSoT) != 1 {
				t.Errorf("only the dropped pair must surface missing — the same-named rows at other levels still match; got %v", missingFromSoT)
			}
			if len(missingFromSchema) != 0 || len(duplicates) != 0 {
				t.Errorf("dropping one SoT pair must not orphan or duplicate any other, got missingFromSchema=%v duplicates=%v", missingFromSchema, duplicates)
			}
		})
	}
}

// containsPair reports whether pairs contains target — a test-local membership check
// used to assert a specific (level, key) appears in a divergence slice.
func containsPair(pairs []leafKey, target leafKey) bool {
	for _, p := range pairs {
		if p == target {
			return true
		}
	}
	return false
}

// removePair returns a copy of pairs with the FIRST occurrence of target removed. Used
// to build a synthetic divergent SoT (one dual-level pair dropped) without mutating the
// real MetadataRows() backing data.
func removePair(pairs []leafKey, target leafKey) []leafKey {
	out := make([]leafKey, 0, len(pairs))
	removed := false
	for _, p := range pairs {
		if !removed && p == target {
			removed = true
			continue
		}
		out = append(out, p)
	}
	return out
}
