TASK: interactive-mint-init-setup-1-3 — Mechanically derive the schema leaf-key set from the decode-shape structs via reflection

ACCEPTANCE CRITERIA:
- `schemaLeafKeys()` returns exactly 25 (level, key) pairs derived from struct tags — no hand-maintained key list.
- No pair has key `release`, `commit`, or `hooks` (containers emit zero pairs).
- Traversal recurses into `fileShape.Release`, `fileShape.Commit`, `releaseShape.Hooks` and tags leaf children at LevelRelease, LevelCommit, LevelReleaseHooks.
- `ai_command` and `timeout` each appear once at each of LevelShared, LevelRelease, LevelCommit.
- The three `[release.hooks]` leaf keys appear at LevelReleaseHooks.
- The helper reads `toml` tags only — does not consult the SoT (no coupling).
- All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Drift test (the anti-drift enforcement)" → "What counts as one 'key' (the bijection contract)" (specification.md:102-106): the authoritative key set is derived MECHANICALLY from the `config` decode-shape structs' `toml` tags (fileShape/releaseShape/commitShape/hooksShape), not a hand list. The bijection is total over LEAF keys; sub-table container fields (release/commit/hooks) are recursed-not-counted, the nested struct's tag supplying the level. This is the inverse of the dual-level case. The 1-1 package-layout decision (option (a)) mandates the helper live in `package config` to read the unexported shapes.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/metadata_drift_test.go:54-100 (schemaLeafKeys / schemaLeafKeysInto), :39-43 (containerLevels map), :250-256 (tomlTag guard).
- Notes:
  - (a) Reflection derivation, no hand list: schemaLeafKeysInto walks reflect.TypeOf(fileShape{}) and reads Field(i).Tag.Get("toml") via tomlTag (metadata_drift_test.go:75-99). The derived set is never seeded from a literal key list. Verified against config.go:330-382: fileShape (4 leaf + Release/Commit containers), releaseShape (14 leaf + Hooks container), commitShape (4 leaf), hooksShape (3 leaf) = 25 leaf pairs. Correct.
  - (b) recurse-don't-count: containerLevels (metadata_drift_test.go:39-43) maps release→LevelRelease, commit→LevelCommit, hooks→LevelReleaseHooks. A field whose tag is in this map emits NO pair and recurses at the mapped level (:85-90). The level threads down through recursion (the nested-tag-supplies-level edge case), not inferred from the leaf field name. Correct.
  - (c) Dual-level once per level: no cross-level dedup — leafKey identity is the (Level, Key) pair (:26-29), so ai_command/timeout each emit one pair at shared/[release]/[commit]. Confirmed by reading config.go: AICommand/Timeout appear on fileShape (shared), releaseShape, and commitShape. Correct.
  - (d) Reads tags only, independent of SoT: schemaLeafKeys never references MetadataRows()/sotLeafKeys(). The two sides stay derived from different origins (file header documents this at :13-16). Correct.
  - Robustness beyond the plan: the implementer chose the "robust alternative" container detection (explicit containerLevels map) AND added a fail-loud guard (:94-96) — a struct-kind field whose tag is not a known container triggers t.Fatalf, so a future sub-table added to the schema cannot be silently mis-classified as a leaf. This is a genuine improvement over the recommended kind-based heuristic and is correctly documented (:33-38, :66-71).

TESTS:
- Status: Adequate
- Location: internal/config/metadata_drift_test.go:122-230; census in internal/config/metadata_census_test.go.
- Coverage:
  - "it derives all leaf pairs from struct tags" → TestSchemaLeafKeys_DerivesAllLeafPairs (:122-142): count == len(expected) and every expected pair present exactly once. Expected pairs sourced from the shared census (expectedLeafKeys), not re-inlined.
  - "recurse-don't-count" → TestSchemaLeafKeys_NoContainerPairs (:149-158): asserts zero pairs for release/commit/hooks keys.
  - "release leaves tagged at release level" → TestSchemaLeafKeys_ReleaseLeavesTaggedAtReleaseLevel (:164-174): tag_prefix at LevelRelease AND explicitly NOT at LevelShared (proves level comes from container tag, not field name).
  - "hooks leaves at release.hooks level" → TestSchemaLeafKeys_HooksLeavesTaggedAtHooksLevel (:180-189): the three hooks keys at LevelReleaseHooks; comment notes HookValue interface-kind fields are leaves (the interface edge case).
  - "ai_command/timeout at all three levels" → TestSchemaLeafKeys_AICommandAndTimeoutAtAllThreeLevels (:195-206): each key once at each of the three levels.
  - "independent of the SoT" → TestSchemaLeafKeys_IndependentOfSoT (:213-230): every derived pair maps back to a real toml tag on its named shape (tagsByLevel built straight from the shapes), proving the derivation is schema-sourced, not SoT-seeded.
  - Duplicate-emission guard: leafKeySet (:104-115) fails on any duplicate emitted pair — defends against a walk bug emitting the same leaf twice.
- Notes:
  - HookValue interface-leaf edge case: covered behaviourally — the three hooks keys derive as leaves (TestSchemaLeafKeys_HooksLeavesTaggedAtHooksLevel) and HookValue is `any` (config.go:279), kind Interface, never in containerLevels, so it correctly falls to the leaf branch. The tag-driven (not kind-driven) container check (:66-69) is what makes this robust. Adequate.
  - empty/`-` tag guard: tomlTag (:250-256) returns ok=false for "" or "-"; schemaLeafKeysInto skips on !ok (:81-83). NOT directly unit-tested in isolation (no current field exercises it — none exist today, as the task acknowledges). The guard is exercised indirectly only insofar as no real field hits it. This is a minor, non-blocking under-test: a focused test could feed a synthetic struct with an empty/`-` tag to prove the skip, but the guard is trivial, the spec marks the case "none today / be defensive", and adding a synthetic-struct test would touch test logic — recorded as a quickfix below, not blocking.
  - Not over-tested: each test pins one distinct property; no redundant assertions. The census consolidation (metadata_census_test.go) removes a former third inline copy of the 25 pairs — good DRY discipline that keeps the two naming tests in lockstep without weakening the bijection's two independent derivations.

CODE QUALITY:
- Project conventions: Followed. Internal `package config` test file per the 1-1 option (a) decision (justified at metadata_drift_test.go:3-16). External test packages where the shape fits; t.Parallel() throughout; table-ish per-property tests. t.Helper() on all helpers. Offender-naming failure messages (:95, :111, :139, :155, etc.) match the project's "name the offender" drift-test idiom. No subprocess/presenter/git seams touched (pure reflection over in-package types), so no seam-discipline concerns apply.
- SOLID principles: Good. tomlTag is the single tag-extraction primitive consumed by both schemaLeafKeysInto and tomlTagsOf (:245-256) — SRP + DRY, the skip-token convention lives in one place. containerLevels is the single container→level mapping.
- Complexity: Low. schemaLeafKeysInto is a single recursive loop with a three-way branch (container / unmapped-struct-fail / leaf); cyclomatic complexity is minimal and each path is named in a comment.
- Modern idioms: Yes. reflect used idiomatically; map-literal mapping; t.Fatalf for fail-loud on uncovered sub-tables.
- Readability: Good. Heavy WHY-comments stating the recurse-don't-count contract, the tag-vs-kind detection rationale, the interface-leaf reasoning, and the independence guarantee — true to as-built.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/config/metadata_drift_test.go:250-256 — add a focused test for the tomlTag empty/`-` skip guard (e.g. a synthetic struct with an untagged and a `-`-tagged field passed through a small extracted walk, or a direct tomlTag table test) so the defensive guard the edge-case list calls out is proven rather than only implied by the absence of such a field. Touches test logic, so quickfix rather than do-now.
