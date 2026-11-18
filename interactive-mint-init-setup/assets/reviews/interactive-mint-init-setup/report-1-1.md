TASK: interactive-mint-init-setup-1-1 — Define the config-metadata SoT table (rows + typed level)

ACCEPTANCE CRITERIA:
- config.MetadataRows() returns exactly 25 rows (sanity check; do NOT hard-code 25 in production).
- ai_command and timeout each appear as three distinct rows: LevelShared, LevelRelease, LevelCommit.
- No row for the container keys release, commit, hooks.
- Every row carries a non-empty Description.
- MetadataLevel.String() renders [release], [release.hooks], [commit] and the top-level/shared form for LevelShared.
- All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Config-metadata source of truth (SoT)" (specification.md:79-96) requires a single in-binary, schema-adjacent table — one row per config key with columns key/level/default/description — that renders into `mint setup` and is drift-tested against the real schema. Levels are top-level shared, [release], [release.hooks], [commit]. Spec "Drift test" / bijection contract (:98-106) makes (level, key) the matching unit: ai_command/timeout are distinct rows per level (not collapsed); the [release.hooks] keys are their own rows; the release/commit/hooks container fields emit zero rows (recurse-don't-count). The plan scopes 1-1 to rows/levels/keys/descriptions; default-cell representation is 1-2 and the literal pins are 1-5 (both visibly already landed in the same files).

IMPLEMENTATION:
- Status: Implemented (and correct)
- Location:
  - internal/config/metadata.go:46-91 — MetadataLevel int enum (LevelShared iota-0, LevelRelease, LevelReleaseHooks, LevelCommit) + String().
  - internal/config/metadata.go:102-107 — MetadataRow{Key, Level, Default, Description} (all exported, matching the plan's "Add a row type" spec exactly).
  - internal/config/metadata.go:119-168 — MetadataRows() returning the ordered 25-row slice, built fresh each call.
- Notes:
  - Package placement follows the recommended option (a): SoT lives in `package config` (metadata.go), so the 1-3 reflection reads the unexported shapes directly with no exported reflection seam. Correct decision, recorded in the file header (:9-15).
  - Row ordering matches schema field order exactly. Verified field-by-field against config.go: fileShape leaves ai_command/max_diff_lines/timeout/diff_exclude (config.go:333-336) ↔ metadata.go:127-130; releaseShape 14 leaves (config.go:358-371) ↔ metadata.go:138-151; hooksShape preflight/pre_tag/post_release (config.go:380-382) ↔ metadata.go:156-158; commitShape context/prompt/ai_command/timeout (config.go:351-354) ↔ metadata.go:163-166. Stable, deterministic order.
  - ai_command (metadata.go:127,150,165) and timeout (:129,151,166) each present at exactly the three levels — tri-level, not collapsed. Correct per AC and spec bijection.
  - No row for release/commit/hooks containers — those fields are the only non-leaf fields and they are simply not enumerated. Correct (inverse of the dual-level case).
  - Every Description is non-empty and is the one-line meaning, DRY of default values (defaults live in the Default column). Spot-checked against README per-key table (README.md:212-223) and config semantics: tag_prefix, release_branch, on_notes_failure, fallback, version_pattern all align.
  - The count 25 is NOT hard-coded in production code; MetadataRows() is a literal slice and the file header (:117-118) explicitly defers the count guard to the drift test. Satisfies the "derive nothing from a count" instruction.
  - The Default cells (1-2/1-5 scope) are already populated and sourced from exported constants (DefaultAICommand, strconv.Itoa(DefaultMaxDiffLines), strconv.Itoa(int(DefaultTimeout/time.Second))) at metadata.go:127-130 — no drift-prone re-typed literals for the three shared scalar defaults. Out of strict 1-1 scope but correct and consistent.

TESTS:
- Status: Adequate
- Location: internal/config/metadata_test.go (external package config_test); shared index seam internal/configtest/configtest.go; expected-pair census internal/config/metadata_census_test.go.
- Coverage (mapped to the six 1-1 test names):
  - "one row per (level, key) pair for all 25 keys" → TestMetadataRows_OneRowPerLevelKeyPair (:32-54): asserts len == census len (25) AND that every expected pair is present, naming any missing pair precisely. Would fail if a key were dropped/renamed.
  - "ai_command at three levels" → TestMetadataRows_AICommandTriLevel (:59-68).
  - "timeout at three levels" → TestMetadataRows_TimeoutTriLevel (:73-82).
  - "no container rows" → TestMetadataRows_NoContainerRows (:88-97): would fail if a release/commit/hooks row were emitted.
  - "non-empty description on every row" → TestMetadataRows_EveryRowHasDescription (:101-109).
  - "renders [release]/[release.hooks]/[commit] level strings" → TestMetadataLevel_String (:114-137), plus TestMetadataLevel_String_OutOfRangeIsDistinctFromShared (:147-165) pinning the closed-enum fallback.
- Each test would fail if the behaviour broke: the present-pair loop catches dropped keys; NoContainerRows catches a stray container row; the String table catches a level-rendering regression; the out-of-range test catches a fallback collapsing to "" (which would let a corrupted level masquerade as shared).
- Not over-tested: the expected-pair census is single-sourced (metadata_census_test.go via config.ExpectedLeafKeys) and consumed by both the internal drift test and the external naming test, so the 25-pair enumeration lives in exactly one place — no duplicate hand-list. The shared configtest.MustByLevelKey seam removes the previously-duplicated rowSet builders across config_test and setupguide_test. Good consolidation, not bloat.
- Not under-tested for 1-1: every AC has a direct, behaviour-level assertion. (Default-cell and drift/bijection coverage for 1-2/1-3/1-4/1-5 also present in metadata_test.go, metadata_drift_test.go, metadata_bijection_test.go — out of 1-1 scope but confirms the table is fully guarded.)

CODE QUALITY:
- Project conventions: Followed. External test package (config_test) per the test idiom; internal helpers in package config only where they must read unexported shapes (the documented option-(a) rationale). configtest mirrors the stdlib nettest/fstest test-support-package idiom and is never linked into the production binary (no production importer), honouring the strict minimal-production-surface discipline in CLAUDE.md. No new exported production API beyond the SoT itself (the census accessor is test-only, in a _test.go file).
- SOLID principles: Good. MetadataLevel/MetadataRow/MetadataRows are single-responsibility; the (level, key) identity is single-sourced in configtest.RowKey; the pure ByLevelKey core is separated from the *testing.T-taking MustByLevelKey wrapper so collision detection is independently testable.
- Complexity: Low. MetadataRows is a flat literal; String() is a closed switch with an enforced default.
- Modern idioms: Yes. iota enum with the zero value deliberately = LevelShared (matches the schema top level); fresh-slice-per-call to prevent shared-backing-array mutation; conventional Stringer sentinel ("MetadataLevel(N)") for out-of-range rather than panicking in a render path.
- Readability: Good. Heavy, accurate WHY-comments (file header, the String() default-branch rationale, the per-section row comments) that match as-built behaviour — consistent with the repo's comment discipline. Descriptions are crisp and DRY of defaults.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/metadata.go:78-91 — MetadataLevel.String() returns a self-describing sentinel for an out-of-range value rather than panicking. This is a deliberate, well-reasoned choice (a Stringer should be safe; a panic would crash `mint setup`). Decide whether the closed enum also warrants a compile-time/exhaustiveness lint (e.g. an exhaustive-switch linter or a test that iterates declared constants) so a newly-added level that forgets a String() case is caught at build time rather than only via the drift test downstream. No change required for this task.
- [idea] internal/config/metadata_census_test.go:26-56 — the expected-pair census is hand-maintained and lives separately from the reflection-derived schemaLeafKeys() (1-3) and from MetadataRows() (1-1); the bijection (1-4) compares all three. This is intentional triangulation (hand census vs schema tags vs SoT rows), but it does mean a schema key change requires editing two hand lists (the census AND MetadataRows). Confirm in the 1-4 review that the bijection actually fails when only one of the two hand lists is updated; if it does, the triangulation holds. No action in 1-1.
