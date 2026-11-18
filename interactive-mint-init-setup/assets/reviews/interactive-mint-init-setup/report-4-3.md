TASK: 4-3 — Add the optional key-presence tripwire test (interactive-mint-init-setup-4-3)

ACCEPTANCE CRITERIA (from plan):
- A Go test in the `config` test package reads the README from disk and fails loudly (naming the attempted path) if it cannot be read.
- The test derives the distinct set of schema key names from a single authoritative source, with the key-source choice (decode-shape `toml` tags vs `config.MetadataRows()`) documented in a comment explaining why that source was chosen.
- The dual-level `ai_command` and `timeout` are deduped to one name each (name-presence/substring, NOT the (level, key) pairs the Phase 1 drift test uses).
- The table-container tags `release`, `commit`, `hooks` are excluded from the key set.
- The test asserts every distinct key name appears as a substring of the README and reports ALL missing names in a single failure message.
- The test PASSES against the reconciled README (Tasks 4-1 and 4-2 landed first).
- A removed/renamed schema key with no README mention fails the test; an added schema key not yet in the README fails the test (verified by two negative tests).
- All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
The spec ("README — config reference verification", Definition of done → "README tripwire (optional)") describes an OPTIONAL cheap tripwire that asserts every schema key name appears somewhere in the README. The README is the human config-reference surface (manual narrative, per-key tables) and — unlike the machine surfaces (mint setup's SoT table) — was previously not mechanically coupled to the schema. The Phase 1 drift test guards the SoT↔schema relationship on the FULL (level, key) pair; this tripwire is deliberately COARSER (distinct NAMES, substring presence) because the README lists each key by name in backticks, not by (level,key) coordinate. The spec explicitly flagged the key-source as a fork (decode-shape toml tags vs MetadataRows()) and recommended deriving from whatever the drift test treats as canonical so the tripwire stays in one chain with the schema.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/readme_tripwire_test.go (whole file, package config_test).
- Notes:
  - distinctSchemaKeyNames() (lines 45-56) derives names from config.MetadataRows(), deduping by first-seen Key. MetadataRows() is exported (internal/config/metadata.go:119) and returns []MetadataRow with an exported Key field (metadata.go:102-107). The 19 distinct names produced match the schema leaf-key set exactly.
  - Key-source choice (MetadataRows() over raw toml tags) is documented in the file header (lines 12-20) with a sound rationale: MetadataRows() IS the SoT, the Phase 1 bijection/drift test pins the SoT to the real schema leaf-key set, so the tripwire transitively tracks the schema with no re-reflection. This satisfies the spec's "stay one chain" recommendation and resolves the flagged ambiguity correctly — MetadataRows() does exist and is exported, so the recommended source was the right pick.
  - Container-tag exclusion (release/commit/hooks) is by construction: MetadataRows() emits no row for a sub-table container (metadata.go:113-114 confirms "the container fields release/commit/hooks emit NO row"). Documented at lines 16-20 as absence-by-construction, guarded by a dedicated test (lines 153-162). This is cleaner than the plan's "EXCLUDE the table-container tags" framing — no explicit filter is needed because the source never yields them.
  - README path resolved via runtime.Caller(0) anchored on the test file (readmePath, lines 79-87) — more robust than the plan's minimum acceptable CWD-relative `../../README.md`, and correctly independent of the `go test` invocation directory.
  - PATH-DEPTH NOTE (not a defect): the plan prose said "three directories up" then wrote `../../README.md` (two). The implementation's comment (line 77, "two directories up") and code (two `..`, line 86) are CORRECT — internal/config → internal → repo root. Verified: README.md resolves and is read. The implementer silently corrected the plan's internal inconsistency; no action needed.

TESTS:
- Status: Adequate
- Coverage:
  - TestREADME_DeclaresEverySchemaKeyName (lines 119-128): the live tripwire — every distinct schema key name is a substring of the real README. Verified PASS; all 19 names present in README.md (counts: ai_command 11, timeout 8, ... version_pattern 1, all >= 1).
  - TestDistinctSchemaKeyNames_DedupesDualLevelKeys (lines 134-147): proves ai_command and timeout each appear exactly once in the distinct set even though MetadataRows() carries three rows each — covers AC (c) directly.
  - TestDistinctSchemaKeyNames_ExcludesContainerTags (lines 153-162): proves release/commit/hooks are absent from the distinct set — covers AC for container exclusion.
  - TestMissingKeysInREADME_FlagsAbsentKey (lines 169-181): drives the PURE missingKeysInREADME helper against a synthetic body missing tag_prefix; proves the tripwire bites for a removed/renamed key WITHOUT mutating the real README/schema. Covers the negative half of AC (d).
  - TestMissingKeysInREADME_ReportsAllMissing (lines 185-197): proves ALL missing names are reported in order, not just the first — covers the "single failure message lists every offender" AC.
  - TestReadFileNamingPath_FailsLoudlyOnMissingFile (lines 204-216): drives pure readFileNamingPath against a non-existent path; asserts error is non-nil AND names the attempted path — covers the loud-read-failure AC and the "no vacuous pass" edge case.
- Notes:
  - Test design is excellent: the comparison and read logic are extracted into PURE helpers (missingKeysInREADME, readFileNamingPath) so the negative/loud-fail contracts are unit-tested directly, while the live tripwire wires the same helpers against the real README. This mirrors the Phase 1 bijection helper's pure-comparison pattern (consistent with the package's established idiom).
  - Not over-tested: each test asserts a distinct contract; no redundant happy-path duplication.
  - Not under-tested: both negative directions (absent key, multi-absent, unreadable file) are covered. The "added key not yet in README" direction (AC (d) second half) is covered by the SAME mechanism as removed/renamed — any name in distinctSchemaKeyNames() not in README text fails — and the synthetic absent-key test proves that mechanism. This is adequate.
  - Minor observation (non-blocking): the live TestREADME_DeclaresEverySchemaKeyName only exercises the happy path (real README contains all keys); its failure path is proven indirectly via the pure helper. This is the correct design (you cannot make the live test fail without breaking the real README), so it is not a gap — noted only for completeness.

CODE QUALITY:
- Project conventions: Followed. Plain stdlib t.Errorf/t.Fatalf (no testify) — consistent with the rest of internal/config tests (metadata_drift_test.go, metadata_bijection_test.go use no testify). External test package (config_test). t.Parallel() on every test. Test naming follows TestType_Behaviour idiom. t.TempDir() used for the bad-path fixture.
- SOLID principles: Good. Single-responsibility helpers; pure functions cleanly separated from the *testing.T-coupled wrappers (readFileNamingPath vs readREADME, missingKeysInREADME vs the tripwire test).
- Complexity: Low. Straight loops, no branching beyond the contains/seen checks.
- Modern idioms: Yes. runtime.Caller anchoring, os.ReadFile, fmt.Errorf("%w") error wrapping, filepath.Join.
- Readability: Good. Heavy, accurate WHY-comments matching the codebase's documented style: key-source rationale, dedupe-vs-pair distinction, substring-overlap caveat with a forward-looking flag for future short key names.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- (verification, no change) Substring-overlap safety claim (file header lines 22-26) verified: across the 19 current key names no name is a substring of another. The comment already flags that a future short key name would require tightening to whole-word — correctly forward-looking. No action.
