TASK: interactive-mint-init-setup-1-5 — Pin the subsumed scaffold default values on the SoT default column

ACCEPTANCE CRITERIA (from phase-1-tasks.md):
- The shared-level `ai_command` SoT row's `Default` cell equals `config.DefaultAICommand`.
- The shared-level `timeout` SoT row's `Default` cell equals `int(config.DefaultTimeout / time.Second)` rendered as a decimal string ("60"), derived from the constant, not a re-typed literal.
- A build-failing test fails if the `ai_command` cell drifts from `config.DefaultAICommand`.
- A build-failing test fails if the `timeout` cell drifts from the seconds derived from `config.DefaultTimeout`.
- The `timeout` cell is the integer-seconds form ("60"), NOT the duration string ("1m0s") and NOT a re-typed 60.
- `internal/initgen` is untouched in this task (its existing value-drift pins still pass).
- All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "`initgen` scope of change" → "The scaffold-value drift-pin moves to the SoT" (specification.md:71) and "Definition of done" (specification.md:215): initgen's drift tests historically pinned the scaffold's literal `ai_command`/`timeout` equal to `config.DefaultAICommand`/`config.DefaultTimeout`. The minimal template (Phase 3) carries no defaults to pin, so the value-drift discipline is SUBSUMED by the SoT drift test — the SoT `default` column becomes the drift-pinned carrier. The subsumption scope is exactly the two `initgen`-pinned keys (`ai_command`, `timeout`); `max_diff_lines` is not in scope for the subsumption (handled separately).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/metadata.go:127 — `ai_command` shared row: `Default: DefaultAICommand` (sourced from the exported constant, NOT a re-typed literal). ✓ criterion (a).
  - internal/config/metadata.go:129 — `timeout` shared row: `Default: strconv.Itoa(int(DefaultTimeout / time.Second))` — integer-seconds derivation via duration division (the same form initgen used), NOT `.Seconds()` float, NOT the duration String. ✓ criteria (c) integer-seconds, derived-not-literal.
  - internal/config/metadata.go:128 — `max_diff_lines` shared row: `Default: strconv.Itoa(DefaultMaxDiffLines)` (added later by task 7-1; out of 1-5's named scope but consistent with the "no default left unpinned" intent).
  - internal/config/metadata.go:150-151, 165-166 — the four per-verb `ai_command`/`timeout` rows ([release], [commit]) carry `Default: "shared"` (inherit), deliberately NOT pinned to a literal. ✓ criterion (d).
- Notes: The production spelling at metadata.go:129 (`int(DefaultTimeout / time.Second)`) matches the test's derivation exactly (metadata_test.go:357, :402), so the pin is anchored to the same expression — no second copy of the literal `60` in production. config.go:72/91/103 confirm `DefaultMaxDiffLines`, `DefaultAICommand`, `DefaultTimeout` are all EXPORTED, so the SoT (in `package config`) sources from them directly.

TESTS:
- Status: Adequate
- Location: internal/config/metadata_test.go (external `package config_test`).
  NOTE: the task text referenced `metadata_drift_test.go`; the pins actually live in `metadata_test.go`. `metadata_drift_test.go` is the internal-package reflection/derivation file (task 1-3). The pins correctly belong with the SoT row assertions in `metadata_test.go`. This is a documentation-vs-as-built mismatch in the TASK pointer, not an implementation defect.
- Coverage:
  - TestMetadataRows_SharedAICommandDefaultEqualsConfigConstant (metadata_test.go:309) — pins `ai_command` shared cell == `config.DefaultAICommand`. ✓ the build-failing drift guard for criterion (a)/(third bullet).
  - TestMetadataRows_SharedTimeoutDefaultEqualsConfigConstant (metadata_test.go:349) — pins `timeout` shared cell == `strconv.Itoa(int(config.DefaultTimeout / time.Second))`, the exact `initgen` derivation. ✓ criterion (b)/(fourth bullet).
  - TestMetadataRows_SharedTimeoutDefaultIsIntegerSecondsNotDuration (metadata_test.go:368) — asserts the cell is NOT `config.DefaultTimeout.String()` ("1m0s") AND equals "60". ✓ criterion (c)/(fifth bullet). This is the third test the task listed ("it renders the timeout default as integer seconds, not the duration string"). Added after the original 1-5 commit (3daf7d0 shipped only the two equality pins); present at HEAD.
  - TestMetadataRows_SharedMaxDiffLinesDefaultEqualsConfigConstant (metadata_test.go:329) — bonus pin for `max_diff_lines` (task 7-1), out of 1-5's named scope.
  - TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim (metadata_test.go:393) — table covering all concrete-scalar cells; the shared `ai_command`/`timeout`/`max_diff_lines` wants are derived from the exported constants (not literals), the [release] scalars are literals (their constants are unexported).
- Notes:
  - Not under-tested: the two in-scope cells each have a dedicated equality drift guard PLUS the timeout has the explicit not-the-duration-string guard. The duplicate-(level,key) collision in `configtest.MustByLevelKey` (configtest.go via ByLevelKey/ErrDuplicateRowKey) ensures the lookups can't silently resolve a stray row.
  - Mild over-test / redundancy (non-blocking): the `(shared, ai_command)`, `(shared, timeout)`, `(shared, max_diff_lines)` cells are now each asserted by BOTH a dedicated `*EqualsConfigConstant` test AND the `ConcreteScalarDefaultsRenderVerbatim` table (metadata_test.go:401-403). The table rows for those three are subsumed by the dedicated pins. Not a defect — the table's purpose is the [release] scalars — but the three shared rows are belt-and-braces. Worth a comment or trimming, not a blocker.
  - The tests would genuinely fail if the feature broke: changing the `ai_command` cell to a literal that drifts, rendering timeout as "1m0s", or re-typing "60" via `.Seconds()` against a non-60 constant would each surface.

CODE QUALITY:
- Project conventions: Followed. External test package (`config_test`), `t.Parallel()` throughout, derivation-not-literal discipline mirrors initgen_test.go's `int(config.DefaultTimeout / time.Second)` exactly (CLAUDE.md "no second copy of the literal"). Exported-constant single-sourcing matches config.go's stated intent (config.go:69-71, 83-86, 94-96). WHY-comments are heavy and true-to-as-built (the file header metadata.go:9-38 documents the subsumption and the integer-seconds derivation).
- SOLID principles: Good. SoT is single-responsibility; the pin tests depend on the abstraction (exported constants), not duplicated literals.
- Complexity: Low. The change is data (one row-cell expression) + three focused tests.
- Modern idioms: Yes. `strconv.Itoa(int(DefaultTimeout / time.Second))` is the idiomatic integer-seconds form; division (not `.Seconds()` float-truncation) is the safer spelling.
- Readability: Good. Comments name the subsumed initgen tests by symbol so the lineage is traceable.
- Issues: None blocking.

CRITERION VERIFICATION:
- (a) ai_command shared cell sourced from config.DefaultAICommand, not a literal — VERIFIED (metadata.go:127, pinned metadata_test.go:309).
- (b) build-failing drift tests pin both against the constants — VERIFIED (metadata_test.go:309, :349).
- (c) timeout is integer-seconds ("60"), not the duration string, not a re-typed literal — VERIFIED (metadata.go:129; guards at metadata_test.go:349, :368).
- (d) per-verb rows render "shared" and are deliberately NOT pinned — VERIFIED (metadata.go:150-151, 165-166; TestMetadataRows_PerVerbOverridesRenderShared metadata_test.go:260).
- (e) internal/initgen untouched by THIS task — VERIFIED: task 1-5's commit 3daf7d0 touched only metadata.go, metadata_test.go, and tracking files (`git show --stat 3daf7d0`); no initgen file in the changeset. (At HEAD initgen's pins are gone, but that removal is Phase 3 / commit 59d527c, the correct later phase — not attributable to 1-5.)
- (f) test quality — Adequate; see TESTS above.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/config/metadata_test.go:401-403 — the `(shared, ai_command)`, `(shared, timeout)`, `(shared, max_diff_lines)` rows in TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim duplicate coverage already owned by the three dedicated `*EqualsConfigConstant` pins. Consider dropping those three table entries (keeping only the [release] scalars the table exists for) so each shared-constant cell has exactly one guard, or add a one-line comment noting the intentional overlap.
- [do-now] .workflows/interactive-mint-init-setup/planning/interactive-mint-init-setup/phase-1-tasks.md:231-232 (task pointer) — the task names `metadata_drift_test.go` as the home of the pins, but they live in `metadata_test.go` (metadata_drift_test.go is the internal reflection/derivation file). Historical planning note only; no code action. Correct the pointer if the plan doc is still maintained.
