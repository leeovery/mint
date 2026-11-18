TASK: ai-model-selection-1-5 — Add the net-new top-level shared timeout key to the schema

ACCEPTANCE CRITERIA:
- A top-level `timeout = 90` decodes and is carried onto `Config` as 90 seconds.
- An absent top-level `timeout` is distinguishable from an explicit `timeout = 0` on the config (nil vs explicit-zero plumbing intact).
- With no `.mint.toml`, the resolved top-level timeout is the shipped 60s (via the seed or the accessor floor).
- A non-integer `timeout` (e.g. `timeout = "fast"`) is rejected at `Load` with a mapped friendly message naming the key and integer/seconds type.
- A `DefaultTimeout` exported constant equals `60 * time.Second`.
- Gates (build, gofmt, vet, test -race, golangci-lint) pass.

STATUS: Complete

SPEC CONTEXT:
specification.md "Config schema: timeout key" (lines 58-82) + "Resolution value semantics > timeout" (95-102). timeout is NET-NEW config surface (only existed as the transport's `defaultTimeout`, never config-populated). Planning resolved the deferred TOML representation to INTEGER SECONDS (`timeout = 90`), decoded as `*int` in the shape, converted to `time.Duration` at the boundary. The representation must preserve value semantics: absent (nil) distinguishable from explicit 0; a non-integer is a strict decode TYPE error at Load; negatives are value-invalid (dropped in 1-7, carried raw here). The config→ai.Config boundary invariant ("no deadline reachable ONLY via explicit 0, never by omission") is served by the `*time.Duration` carrier type chosen here.

IMPLEMENTATION:
- Status: Implemented (matches chosen int-seconds representation; value semantics preserved)
- Location:
  - internal/config/config.go:99 — `DefaultTimeout = 60 * time.Second` exported constant.
  - internal/config/config.go:144 — `Config.Timeout *time.Duration` carrier (nil=absent, non-nil incl. 0/negative=explicit).
  - internal/config/config.go:331 — `fileShape.Timeout *int` with `toml:"timeout"` (int-seconds decode; non-int → strict type error).
  - internal/config/config.go:292,310 — `defaults()` seeds `&timeout` (60s) for the zero-config path.
  - internal/config/config.go:437 — `Load` resolves via `resolveTimeout(shape.Timeout)`.
  - internal/config/config.go:470 — `typeErrorMessages["fileShape.Timeout"] = "timeout must be an integer (seconds)"`.
  - internal/config/config.go:657-663 — `resolveTimeout` (nil→nil; else seconds→duration), preserving absent-vs-explicit.
  - internal/config/config.go:7,102 — package + Config doc comments updated to enumerate `timeout` among shared top-level keys.
- Notes:
  - Absent-vs-zero correctness hinges on `defaults()` being bypassed on the present-file path: `Load` builds a FRESH Config literal (line 432) using `resolveTimeout`, so a present file with an absent timeout yields nil (not the 60s seed). Verified — the seed only reaches the file-absent early return (line 391). Correct and documented at config.go:287-291.
  - The task also defines `releaseShape.Timeout *int` / `commitShape.Timeout *int` and their typeErrorMessages entries; these are the per-verb schema fields whose resolution (Release.Timeout/Commit.Timeout *time.Duration via resolveTimeout in resolveRelease/resolveCommit) is Task 1-6 territory but is already wired here. Not drift — the schema field additions are this task's stated scope ("schema struct field (top-level + both verb shapes)").
  - TimeoutFor accessor (config.go:596) is Task 1-7 scope and is already present; out of scope for this verification but its presence does not harm 1-5's criteria.

TESTS:
- Status: Adequate (one minor redundancy)
- Coverage:
  - TestLoad_TopLevelTimeout_CarriedAsSeconds (config_test.go:1947) — AC1 (timeout=90 → 90s).
  - TestLoad_AbsentTimeout_DistinctFromExplicitZero (1969) — AC2: absent-in-present-file → nil subtest; explicit-zero → non-nil ptr to 0 subtest.
  - TestLoad_AbsentFile_ResolvesShipped60sDefault (2016) — AC3 (zero-config → 60s seed).
  - TestLoad_NonIntegerTimeout_RejectedNamingKeyAndIntegerType (2059) — AC4 (string timeout names key + integer type).
  - TestDefaultTimeout_ExportedCanonicalValue (1935) — AC5 (constant == 60s, cross-package referenceability).
  - TestLoad_TypeMismatch_MappedFriendlyMessages (744) — guards the mapped messages for top-level + release + commit timeout type errors against go-toml field-path drift.
- Notes:
  - Edge cases from spec all covered: absent-vs-explicit-zero, type-mismatch-as-strict-decode-error, zero-config→60s. Negative-drop is value-semantics (1-7) territory and correctly NOT asserted here.
  - Tests verify behaviour (decoded value on Config), not implementation details. Each would fail if the feature broke (e.g. dropping the *int carrier, mis-seeding defaults(), or a missing typeErrorMessages entry).

CODE QUALITY:
- Project conventions: Followed. Reuses the established `*T`-in-shape absent-vs-explicit idiom verbatim (max_diff_lines *int, publish/changelog *bool); external test package (config_test), t.Parallel throughout, t.TempDir roots, friendly type-error mapping. WHY-comments are heavy and true-to-as-built (CLAUDE.md compliant), including the explicit note that the per-verb type differences force the field-by-field resolveCommit copy.
- SOLID principles: Good. resolveTimeout is a single-responsibility boundary converter reused by all three sites (top-level + both verbs), avoiding duplication.
- Complexity: Low. resolveTimeout is a 5-line nil-guard + conversion.
- Modern idioms: Yes. `time.Duration(*seconds) * time.Second` boundary conversion; pointer-distinguishes-absent idiom.
- Readability: Good. Intent is self-documenting and comments state the contract.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/config/config_test.go:1992 and :2036 — the "explicit zero is a non-nil pointer to 0" subtest of TestLoad_AbsentTimeout_DistinctFromExplicitZero and the standalone TestLoad_ExplicitTimeoutZero_CarriedAsDistinguishableExplicitZero assert the same behaviour (explicit `timeout = 0` → non-nil pointer to 0, not coerced to 60s) with the same TOML input. Fold them into one test to remove the redundant case.
