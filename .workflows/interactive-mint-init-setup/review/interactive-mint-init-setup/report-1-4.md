TASK: interactive-mint-init-setup-1-4 — Drift test: total bijection over leaf keys (SoT ↔ derived schema set)

ACCEPTANCE CRITERIA:
- [AC1] The test passes against the current schema (25 pairs match exactly, bijective).
- [AC2] Removing a leaf field from any decode-shape struct (without updating the SoT) FAILS the test naming the now-orphaned SoT row.
- [AC3] Adding a leaf field to any decode-shape struct (without updating the SoT) FAILS the test naming the unmatched schema leaf.
- [AC4] An SoT row whose (level, key) has no schema leaf FAILS the test (removed/renamed key still in the SoT).
- [AC5] A duplicate SoT row for one (level, key) FAILS the test.
- [AC6] Each of the three dual-level rows (ai_command, timeout × shared/release/commit) matches independently per level — dropping any one fails the bijection while the others still match.
- [AC7] All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Drift test (the anti-drift enforcement)" requires a build-failing Go test proving the config-metadata SoT cannot drift from the canonical schema. Key contract points:
- Match on (level, key) PAIRS, one SoT row per pair — never bare key names.
- ai_command/timeout are distinct rows at shared + [release] + [commit] (matched per level, not collapsed).
- [release.hooks] keys are their own rows at LevelReleaseHooks.
- The authoritative key set is derived MECHANICALLY from the decode-shape struct toml tags (fileShape/releaseShape/commitShape/hooksShape) via task 1-3, not a hand list.
- Bijection is TOTAL over leaf keys; container fields (release/commit/hooks) are recursed-not-counted.
- Mirror the existing initgen↔config drift discipline (build-failing, names the offender).
- Plan edge case: the added/renamed/removed cases cannot be asserted by editing the fixed struct in-test; instead factor the comparison into a PURE helper taking two pair sets and unit-test it against synthetic divergent inputs (dropped/phantom/duplicated pair).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/metadata_bijection_test.go:39-78 — pure `bijectionDiff(sot, schema []leafKey)` helper (three divergence classes out).
  - internal/config/metadata_bijection_test.go:84-91 — `sotLeafKeys()` projects MetadataRows() to (Level, Key) pairs; reads only the SoT.
  - internal/config/metadata_bijection_test.go:97-111 — `TestMetadataSoT_BijectsSchemaLeafKeys`, the real build-failing drift guard.
  - internal/config/metadata_bijection_test.go:117-127 — `TestMetadataSoT_BijectionPairCount` (25-pair sanity pin on both sides).
  - internal/config/metadata_bijection_test.go:129-282 — five synthetic-divergence unit tests on the pure helper.
  - internal/config/metadata_bijection_test.go:291-324 — `TestMetadataSoT_DualLevelRowsMatchIndependently` (per-level independence over the REAL SoT minus one pair).
  - internal/config/metadata_drift_test.go:54-100 — `schemaLeafKeys` reflection helper (task 1-3, the independent schema side).
  - internal/config/metadata_census_test.go — single shared 25-pair census (ExpectedLeafKeys).
- Notes:
  - The bijection is matched on the full `leafKey{Level, Key}` struct value (Go == on the two-field struct), never the bare key — directly satisfying the per-(level,key) contract. Verified at :56, :67, :71 and in the map keys `sotCounts`/`schemaSet` keyed on `leafKey`.
  - The two sides are independent: `sotLeafKeys()` reads only MetadataRows(); `schemaLeafKeys()` reads only the struct tags (drift_test.go:54-100, and the 1-3 independence test :213-230). Genuine drift is therefore detectable, not two copies of one hand-list.
  - Both `bijectionDiff` direction loops report each offender exactly once via dedup maps (:54, :64-65), so failure messages name a pair once even if a side accidentally repeats it. Sound and defensive.
  - Schema-side count derives correctly to 25: fileShape = 4 leaf + 2 containers; releaseShape = 14 leaf + Hooks container; hooksShape = 3 leaf; commitShape = 4 leaf (confirmed against config.go:330-383). Containers recurse-don't-count.
  - The real drift test (:97) and the dual-level independence test (:291) call `schemaLeafKeys(t)` over the REAL structs and `sotLeafKeys()` over the REAL SoT — they prove the two real sides agree today (AC1) and that the mechanism catches a per-level drop without mutating either real source (AC6).
  - No production code touched — task is purely test infrastructure, as planned. `go vet ./internal/config/` is clean; the package compiles.

ACCEPTANCE-CRITERIA MAPPING:
- AC1 → TestMetadataSoT_BijectsSchemaLeafKeys (:97) + TestMetadataSoT_BijectionPairCount (:117). MET.
- AC2 (removed schema field → orphaned SoT row named) → proven by the MECHANISM via TestBijectionDiff_SoTRowMissingFromSchema (:182) (the fixed struct can't be edited per-test, per the plan edge case; the mechanism test is the agreed proof). The real guard's missingFromSchema loop (:105) names the offending (level, key). MET via the planned synthetic-input approach.
- AC3 (added schema field → unmatched schema leaf named) → TestBijectionDiff_SchemaLeafMissingFromSoT (:152) proves the mechanism; the real guard's missingFromSoT loop (:102) names it. MET.
- AC4 (SoT row with no schema leaf) → TestBijectionDiff_SoTRowMissingFromSchema (:182). MET.
- AC5 (duplicate SoT row) → TestBijectionDiff_DuplicateSoTRow (:210) + TestBijectionDiff_DuplicateIsPerPairNotPerKey (:239) proving duplicate detection is on the full pair (the three legit ai_command rows are not duplicates). MET.
- AC6 (each dual-level row matches independently) → TestMetadataSoT_DualLevelRowsMatchIndependently (:291) drops each of the six dual-level pairs from a copy of the real SoT and asserts exactly that pair surfaces missing while the same-named rows at other levels still match. MET.
- AC7 (gates) → package vets and compiles clean; tests are external/internal `package config`, t.Parallel throughout, no real subprocess use. MET (full-suite run is out of scope for this verifier; no compile or static issue observed).
- Plus the per-level-not-bare-key edge case → TestBijectionDiff_MatchesPerLevelNotBareKey (:260) (forgotten [commit] timeout surfaces even though bare "timeout" exists). MET.

TESTS:
- Status: Adequate
- Coverage:
  - All seven plan acceptance criteria covered; the five named plan tests are present (bijects-on-current-schema, schema-leaf-missing, SoT-row-missing, duplicate, per-level independence), plus three earned extras (clean-when-equal property, duplicate-is-per-pair-not-per-key, matches-per-level-not-bare-key) that lock the subtle pair-vs-key semantics.
  - The two real-side tests (the drift guard and the dual-level independence test) prove the real SoT and real schema agree TODAY; the synthetic-input tests prove the GUARD MECHANISM catches each divergence class. This is exactly the split the plan and spec require (the real schema is always in sync, so divergence must be proven on synthetic inputs).
- Notes:
  - Not under-tested: every divergence class (missing-from-SoT, missing-from-schema, duplicate, per-level mismatch) has a dedicated test, and each asserts both the named offender AND that the other two output slices stay empty — so a test would fail if the helper over-reported. Good.
  - Not over-tested: no redundant happy-path variations; each synthetic test targets a distinct property. TestMetadataSoT_BijectionPairCount (:117) is a thin sanity pin (25 on both sides) and is justified in its own doc comment as subordinate to the authoritative bijection — acceptable, not redundant, since it pins scale rather than re-proving the bijection.
  - The helper-level tests assert behaviour (divergence classification), not implementation details — they pass slices in and check slices out, never reaching into the map internals. Aligned with the project's behaviour-level-proof idiom.
  - Offender-naming is verified indirectly: the synthetic tests assert `containsPair(...)` on the returned slices, and the real guard's t.Errorf strings (:103, :106, :109) interpolate `lk.Level.String()` + `lk.Key`. The messages are clear and point an implementer at the missing/extra row. Matches the initgen drift-pin "name the offender" style.

CODE QUALITY:
- Project conventions: Followed. External/internal `package config` split is deliberate and documented (the reflection helper must read unexported shapes, so it lives in `package config`; the count/naming external test stays `config_test`). t.Parallel() on every test and subtest. No os/exec, no presenter/runner bypass (N/A here — pure data test). Heavy WHY-comments consistent with the codebase's documented invariants (e.g. the recurse-don't-count and per-pair-not-per-key reasoning).
- SOLID principles: Good. `bijectionDiff` is a single-responsibility pure function; the SoT side (`sotLeafKeys`) and schema side (`schemaLeafKeys`) are cleanly separated so neither contaminates the other.
- Complexity: Low. `bijectionDiff` is three linear passes with dedup maps; no nesting beyond a guard. Clear.
- Modern idioms: Mostly. One stale idiom: the `dropped := dropped` loop-variable copy at metadata_bijection_test.go:305 is unnecessary under go 1.26 (go.mod declares `go 1.26.4`; per-iteration loop variables since 1.22). Harmless, but dead ceremony.
- Readability: Good. Helper names (`bijectionDiff`, `containsPair`, `removePair`, `sotLeafKeys`) state intent; the file header explains why the pure-helper-on-synthetic-inputs split exists.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/config/metadata_bijection_test.go:305 — remove the redundant `dropped := dropped` loop-variable shadow; go.mod is `go 1.26.4` and loop vars are per-iteration since go 1.22, so the copy is dead ceremony (and golangci-lint's copyloopvar would flag it on a strict config).
- [idea] internal/config/metadata_bijection_test.go:97-111 — the real drift guard reports offenders via three sequential `t.Errorf` loops but does not assert "no divergence" as a single positive statement; consider a leading guard that early-returns/logs when all three slices are empty for a clearer pass narrative. Pure cosmetics on an already-correct test — decide whether the extra clarity is worth the line.
- [idea] internal/config/metadata_bijection_test.go:113-127 — TestMetadataSoT_BijectionPairCount hard-codes `wantPairs = 25`. It is correctly scoped as a test-only sanity pin (the plan explicitly bans hard-coding 25 in PRODUCTION code only, and the bijection is the real guard), so this is fine as-is; flagging only so a future reader does not mistake it for the authoritative count. No change needed unless the team prefers deriving it from `len(ExpectedLeafKeys())`.
