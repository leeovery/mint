TASK: interactive-mint-init-setup-6-3 — Single-source the rowKey struct and (level,key) index map across the config and setupguide test packages

ACCEPTANCE CRITERIA:
- The rowKey struct and the (level, key) index-map builder are defined exactly once, in the config package.
- Both metadata_test.go and setupguide_test.go consume that single seam; neither re-declares the struct or re-authors the builder.
- The duplicate-(level,key) collision detection is preserved.
- The seam stays test-support-only, NOT on the production render path.

STATUS: Complete

SPEC CONTEXT:
The spec (specification.md) treats `mint setup`'s config-reference table as a single SoT (config.MetadataRows()) rendered for the agent, and explicitly fights drift surfaces ("exactly the drift surface mint already fights", line 58). It also notes the existing initgen seams must be respected and "package layout ... is a planning detail" (line 69), and the machine/agent surfaces are "held to a single SoT" (line 205). This remediation is a pure DRY/test-architecture cleanup: two external test packages had independently authored an identical (level,key) index of the SoT. Consolidating it onto one seam keeps the (level,key) indexing single-sourced next to the SoT it indexes — directly in the spirit of the spec's single-SoT, anti-drift posture. No production behaviour or user-facing surface changes.

IMPLEMENTATION:
- Status: Implemented (one defensible deviation from the literal "Do" wording — see Notes)
- Location:
  - internal/configtest/configtest.go:33 — `RowKey` struct (the single rowKey definition, exported `{Level config.MetadataLevel; Key string}`).
  - internal/configtest/configtest.go:48 — `ByLevelKey([]config.MetadataRow) (map[RowKey]config.MetadataRow, error)`: pure, error-returning index builder with collision detection.
  - internal/configtest/configtest.go:64 — `MustByLevelKey(t)`: the *testing.T wrapper both test packages call.
  - internal/configtest/configtest.go:41 — `ErrDuplicateRowKey` sentinel.
  - internal/config/metadata_test.go:18,48,62,76 (+ all assertion blocks) — consumes `configtest.RowKey` and `configtest.MustByLevelKey`; no local rowKey/rowSet remains.
  - internal/setupguide/setupguide_test.go:387,406,429 — consumes `configtest.MustByLevelKey` and `configtest.RowKey`; the former local `rowKey`/`rowByLevelKey` are gone.
- Notes:
  - DEVIATION (sound): the task "Do" offered two options — an exported `config.MetadataByLevelKey()` on the config package, OR a small `config/configtest` support file. The implementer chose the support-package route and documented exactly why in configtest.go:6-17: a symbol in a `_test.go` file compiles only into that package's own test binary and cannot be shared across packages, so the shared seam must live in an importable (non-`_test.go`) file; and widening the production config API purely for test convenience would ship in the mint binary and violate CLAUDE.md's strict minimal-production-surface discipline. The configtest package is importable but, because nothing in production imports it, is never linked into the mint binary. This is the better of the two offered options and is consistent with the acceptance criterion "defined exactly once, in the config package" read at the package-family level (config + configtest both `internal/config*`). Mirrors stdlib net/nettest, testing/fstest idiom.
  - "Defined exactly once" — confirmed by grep: `RowKey`/`ByLevelKey` are declared only in configtest.go; every other `rowKey`/`rowSet`/`rowByLevelKey` hit across the tree is in COMMENTS/historical prose, not live code.
  - Seam is test-support-only — confirmed: the production render path renderConfigReference() (internal/setupguide/setupguide.go:324) iterates `config.MetadataRows()` directly; setupguide.go has zero references to configtest/RowKey/ByLevelKey. The only importers of mint/internal/configtest are configtest_test.go, config/metadata_test.go, setupguide_test.go — all test files.

TESTS:
- Status: Adequate
- Coverage:
  - internal/configtest/configtest_test.go:15 TestByLevelKey_IndexesEveryRowUnderItsLevelKeyPair — the builder folds a slice and resolves each row by its exact (level,key) pair (including the ai_command shared-vs-release distinctness).
  - configtest_test.go:46 TestByLevelKey_ReportsDuplicateLevelKeyCollision — feeds two rows sharing (shared, ai_command), asserts a non-nil error AND `errors.Is(err, ErrDuplicateRowKey)`. This is the dedicated collision proof the task's "Tests" bullet demands, preserving rowSet's original Fatalf intent.
  - configtest_test.go:66 TestMustByLevelKey_FoldsMetadataRows — the *testing.T wrapper folds the real SoT, count-matched and per-row resolved.
  - Downstream: metadata_test.go and setupguide_test.go's existing assertions all now run through MustByLevelKey and continue to assert SoT identity, level cells, and default tokens — satisfying "existing tests that previously used the local builders pass against the shared seam".
- Notes:
  - Not under-tested: collision is proven through the pure error-returning `ByLevelKey`, which is the correct seam to test it on (a *testing.T-only wrapper would have had to assert via t.Fatalf, untestable without a fake T). Splitting the core (error return) from the wrapper (Fatalf) is exactly what makes the collision provable — good design.
  - Not over-tested: the three configtest tests are distinct (happy-path indexing / collision / real-SoT fold); no redundant restating. Downstream tests were not duplicated into configtest — they keep their own behavioural focus.
  - Note: `ByLevelKey` cannot in practice receive a real duplicate from `config.MetadataRows()` (the bijection drift test already forbids duplicate SoT rows), so the collision branch is exercised only by the synthetic-slice test — which is the right and only way to cover it. Correct, not a gap.

CODE QUALITY:
- Project conventions: Followed. External test packages (configtest_test, config_test, setupguide_test) per the test idiom; t.Parallel() throughout; lowercase no-trailing-punctuation error messages; sentinel matched with errors.Is and wrapped with %w (configtest.go:53). Honours CLAUDE.md's minimal-production-surface rule by keeping the seam out of the shipped binary, with the reasoning captured in the package doc.
- SOLID principles: Good. Single responsibility (the package does one thing — index the SoT by (level,key)); the pure-core / Must-wrapper split is a clean interface segregation between the testable error path and the ergonomic test-helper path.
- Complexity: Low. ByLevelKey is a single linear fold; MustByLevelKey is a 4-line wrapper.
- Modern idioms: Yes. errors.New + fmt.Errorf("%w", …), map capacity hint `make(..., len(rows))`, t.Helper() in the wrapper.
- Readability: Good. The package doc (configtest.go:1-17) is an exemplary WHY-comment explaining the cross-package-test constraint and the production-surface tradeoff — true to as-built. metadata_test.go:12-25 and metadata_census_test.go:1-25 likewise explain the projection from the unexported leafKey census to the exported RowKey.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/configtest/configtest.go:6-17 — the package doc says the seam was duplicated across config_test's "rowKey + rowSet" and setupguide_test's "rowKey + rowByLevelKey". Those names no longer exist anywhere in live code; the comment is accurate as historical motivation but a future reader may grep for them and find only comments. Optionally add a half-clause noting these are the now-removed predecessors (the metadata_census_test.go doc already frames its census as "the third copy that previously lived inline", which reads cleanly — mirror that past-tense framing here). Purely a documentation polish; no logic impact.
