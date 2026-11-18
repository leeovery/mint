TASK: interactive-mint-init-setup-5-1 — Disambiguate the "shared" level token and single-source the levelCell placeholder

ACCEPTANCE CRITERIA:
- The emitted guide's config-reference table renders shared-level rows with the new token ("top-level") in the LEVEL column; per-verb [release]/[commit] ai_command/timeout rows still render "shared" in the DEFAULT column — the two no longer collide.
- The level-column cell value is still derived from config.MetadataLevel.String(); the placeholder is applied only when String() is empty.
- internal/config/metadata.go Default-column tokens are unchanged.
- The test suite no longer contains a verbatim re-implementation of levelCell's conditional; the level-cell expectation has a single source that fails if production's placeholder changes.
- All gates pass.

STATUS: Complete

SPEC CONTEXT:
The specification fixes the Default-column convention: per-verb override "inherit-the-shared" defaults ([release]/[commit] ai_command/timeout) render as the word "shared" (spec line 93) — a load-bearing, spec-decided token. The Level column is described as "top-level shared, [release], [release.hooks], [commit]" (spec line 83). The render is a pure emitter sourcing all key metadata from the config SoT (config.MetadataRows()). The Level-column "shared" placeholder was a render-site invention (not spec) that collided with the spec's Default-column "shared" inherit token in adjacent cells of the same table; the remediation resolves that collision while leaving the spec convention intact.

IMPLEMENTATION:
- Status: Implemented (remediation commit 13430e4)
- Location:
  - internal/setupguide/setupguide.go:355-360 — LevelCell now returns "top-level" (was "shared") for the empty-String() shared form; still driven by level.String() (the placeholder applies only when String() == "").
  - internal/setupguide/setupguide.go:344-354 — doc comment rewritten to state the new placeholder and the WHY (avoids collision with the Default-column inherit "shared" token).
  - internal/setupguide/setupguide.go:333 — renderConfigReference now calls the exported LevelCell.
  - internal/setupguide/setupguide.go:307-314 — renderConfigReference doc comment updated to "top-level (NOT 'shared' ...)".
- Notes:
  - Criterion (a) MET: shared-level cell renders "top-level"; metadata.go:150-151,165-166 still carry Default "shared" on the per-verb override rows — distinct columns, no collision.
  - Criterion (b) MET: LevelCell keeps `if s := level.String(); s != "" { return s }` then the placeholder — identity stays single-sourced in config.MetadataLevel.String().
  - Criterion (c) MET: the 5-1 commit (13430e4) did NOT touch internal/config/metadata.go (verified via `git show --stat`); the four Default "shared" tokens and all other Default tokens are unchanged.
  - The placeholder was given a single home by EXPORTING levelCell → LevelCell, so the cross-package test can call production rather than re-derive. Export is justified as a test seam and documented as "the ONE place the placeholder string lives".

TESTS:
- Status: Adequate
- Coverage:
  - TestGuide_ConfigReferenceSharedLevelUsesNonCollidingToken (setupguide_test.go:480-501) — pins max_diff_lines (a shared-only key, no dual-level twin) renders level cell exactly "top-level" AND not containing "shared". This is the new disambiguation proof; it parses the row's level cell via splitRow/levelCellOf rather than substring-matching the whole line, so it bites precisely on the level column.
  - TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim (setupguide_test.go:383-413) includes per-verb [release].ai_command and [commit].timeout cases asserting the Default cell still carries "shared" verbatim — proves the disambiguation did not disturb the Default convention.
  - Single-sourcing: the test-local levelCell helper (former setupguide_test.go:17-22) was REMOVED. All level-cell expectations now flow through setupguide.LevelCell (setupguide_test.go:339,358-359,407,448,468-470). Changing production's placeholder turns these red instead of silently tracking it. The remaining levelCellOf (line 538) parses a rendered row's second column — it is NOT a re-implementation of the empty-String() conditional, so criterion (d) is genuinely met.
  - TestLevelCell_OutOfRangeLevelDoesNotRenderTopLevel (setupguide_test.go:509-522) — bonus from task 6-5; confirms only the genuine LevelShared maps to "top-level", an out-of-range level renders its Stringer sentinel verbatim. Strengthens the seam.
- Notes:
  - Not over-tested: the new test targets max_diff_lines specifically because it has no dual-level twin, making the row unambiguously shared — a deliberate, minimal choice.
  - Not under-tested: collision-absence, Default-preservation, and single-sourcing are each independently proven.
  - rowLineFor matches on the exact first (Key) cell, so the assertion pins the correct row even when several rows share a key — correct given the dual-level ai_command/timeout rows.

CODE QUALITY:
- Project conventions: Followed. External test package (setupguide_test); t.Parallel() throughout; behaviour-level assertions on rendered table lines; no subprocess/IO. The render stays config-sourced (CLAUDE.md SoT discipline) and the package's "one config import path" invariant is preserved.
- SOLID principles: Good. LevelCell is a single-responsibility render seam; the placeholder lives in exactly one place.
- Complexity: Low. LevelCell is a two-branch function; the test helpers (splitRow/levelCellOf/rowLineFor) are small and clear.
- Modern idioms: Yes.
- Readability: Good. The WHY-comment on LevelCell states the collision rationale fully and matches the as-built behaviour (CLAUDE.md comment discipline honoured).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/setupguide/setupguide.go:355 — LevelCell is now exported solely for cross-package test consumption (no production caller outside the setupguide package). This is a legitimate, well-documented test seam, but it widens the package's public API for a test-only need; consider whether a same-package render test or an internal-test seam would keep the symbol unexported. Requires a design judgment (export-for-test trade-off), hence idea, not a mechanical fix. Note this was the analysis task's explicitly-offered option ("export a render seam"), so it is a sanctioned choice, not a defect.
