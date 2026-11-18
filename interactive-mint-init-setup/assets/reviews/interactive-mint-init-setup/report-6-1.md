TASK: interactive-mint-init-setup-6-1 — Consolidate the 25-pair (level, key) expected census to one shared list

ACCEPTANCE CRITERIA:
- The full 25-pair (level, key) enumeration appears exactly once in the config package test sources.
- Both naming tests reference the shared census; neither carries an inline 25-pair literal.
- MetadataRows() and schemaLeafKeys() remain separate, independent derivations.
- (Test) Both naming tests pass against the shared census.
- (Test) A deliberate single-pair removal from MetadataRows() still fails the bijection drift test, proving the consolidation did not weaken the bijection.

STATUS: Complete

SPEC CONTEXT:
specification.md:98-115 fixes the bijection contract: the authoritative key set is derived MECHANICALLY from the decode-shape struct toml tags (fileShape/releaseShape/commitShape/hooksShape), matched per (level, key) pair, total over leaf keys, recurse-don't-count for container fields. The whole value of the drift test (spec:18, 100, 105) is that the two compared sides come from DIFFERENT origins so a genuine schema↔SoT divergence is caught — "rather than comparing two copies of the same hand-list". This task removes a hand-maintained THIRD copy (the expected census duplicated across the two naming tests) without merging the two derivation sides, preserving exactly that anti-drift property.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/metadata_census_test.go:26-56 — new file; single ordered `expectedLeafKeys []leafKey` var, grouped comments verbatim (4 shared / 14 [release] / 3 [release.hooks] / 4 [commit] = 25), exported accessor `ExpectedLeafKeys()` (metadata_census_test.go:63-65) returning a defensive copy.
  - internal/config/metadata_drift_test.go:129 — `TestSchemaLeafKeys_DerivesAllLeafPairs` now reads `expected := expectedLeafKeys` (internal package, names the var directly); inline 30-line literal removed (confirmed by `git show e225388`).
  - internal/config/metadata_test.go:18-25, 41 — `TestMetadataRows_OneRowPerLevelKeyPair` projects `config.ExpectedLeafKeys()` into the shared `configtest.RowKey` type via `expectedRowKeys()`; inline literal removed.
- Notes:
  - Criterion (a): verified the 25-pair enumeration now appears exactly once. `grep` for census references shows the only literal pair-list is in metadata_census_test.go; both former inline literals are gone (git diff confirms both deletions).
  - Criterion (c) — derivations untouched: `git show --stat e225388` shows the commit changed ONLY test files (metadata_census_test.go new, metadata_drift_test.go, metadata_test.go) plus workflow bookkeeping. Production internal/config/metadata.go (MetadataRows) was not touched, and the reflection derivation (schemaLeafKeys / schemaLeafKeysInto, metadata_drift_test.go:54-100) was not touched. The two bijection sides remain independent: `sotLeafKeys()` (metadata_bijection_test.go:84-91) reads MetadataRows(); `schemaLeafKeys()` reads struct tags; neither reads the consolidated census.
  - Cross-package reachability handled correctly: the census uses the unexported `leafKey` whose Level/Key fields are EXPORTED (metadata_drift_test.go:26-29), so the external config_test can read each pair's components via the exported `ExpectedLeafKeys()` accessor without naming the unexported type. The accessor lives in a _test.go file, so the exported symbol never ships in the production binary.
  - Note: the report-of-interest path showed metadata_test.go using `configtest.RowKey`/`MustByLevelKey`; that is the later 6-3 migration layered on top. The 6-1 consolidation survives it intact — `expectedRowKeys()` still projects `config.ExpectedLeafKeys()`, now into `configtest.RowKey`.

TESTS:
- Status: Adequate
- Coverage:
  - Both naming tests assert against the shared census (criterion a/b satisfied) and would fail precisely (named pair) on an added/renamed/removed key.
  - Criterion (d) is satisfied structurally and is independently proven by an EXISTING test that the consolidation leaves untouched: `TestMetadataSoT_DualLevelRowsMatchIndependently` (metadata_bijection_test.go:291-324) drives `removePair(sotLeafKeys(), dropped)` for each of the six dual-level pairs and asserts the dropped pair surfaces in `missingFromSoT` with exactly one offender — a deliberate single-pair removal failing the bijection. `TestMetadataSoT_BijectsSchemaLeafKeys` (metadata_bijection_test.go:97-111) is the live total-bijection guard over the two independent sides. Because the bijection never consults `expectedLeafKeys`, the census consolidation cannot weaken it — verified by reading both sides' sources.
  - The hard "25" sanity anchor is preserved independently on the bijection side: `TestMetadataSoT_BijectionPairCount` (metadata_bijection_test.go:117-127) pins `wantPairs = 25` against both `sotLeafKeys()` and `schemaLeafKeys()`. The naming tests now derive their count from `len(census)`, but the absolute 25 is still pinned where it belongs (the derivation side), so a census shrunk in lockstep with the SoT would still trip the bijection count.
- Notes:
  - Not over-tested: the two naming tests retain distinct value — one names the SoT side, one the schema side; both still match their own derivation against the shared expected list. No redundant assertions introduced.
  - Not under-tested: edge cases (dual-level independence, per-pair-not-per-key matching, container-emits-no-pair) remain covered by the untouched bijection/drift suites.

CODE QUALITY:
- Project conventions: Followed. External vs internal test-package split respected (census internal so it can name leafKey; config_test projects via the exported accessor). t.Parallel() throughout. The defensive copy in ExpectedLeafKeys() (append to nil) prevents cross-test mutation of shared backing data — matches the repo's "behaviour-level proofs, no shared mutable fixtures" idiom.
- SOLID principles: Good. Single source of the expected census; the accessor has one responsibility.
- Complexity: Low. One var + one one-line copy accessor + one projection helper.
- Modern idioms: Yes. `append([]leafKey(nil), expectedLeafKeys...)` is the idiomatic defensive copy.
- Readability: Good. The WHY-comments (metadata_census_test.go:3-25, 58-62) explain the internal/external reachability constraint and that ONLY the expected census is consolidated, not the two derivations — true to as-built and consistent with the repo's heavy-WHY-comment culture.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (Observed: the naming tests' length checks now key off len(census) rather than a literal 25; this is correct and the absolute 25 anchor is preserved by TestMetadataSoT_BijectionPairCount — no action.)
