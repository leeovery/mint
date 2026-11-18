TASK: mint-release-tool-5-2 — Source × target axis contract validation

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. As-built lives in cmd/mint/regenerate_validate.go (not internal/engine/regenerate_axes.go — the cmd layer is the correct home since validation needs the cmd-layer regenerateRequest/regenerateTarget enums, runs post-parse). validateRegenerateRequest:33, validateTargetAgainstChangelog:61; wired at main.go:103; shared sentinel engine.ErrChangelogDisabled at regenerate_batch.go:125. Check ordering correct — reuse-release-only before changelog-disabled; reuse-target-resolution before changelog check so --reuse under changelog=false safely resolves to release. Reuses ErrChangelogDisabled with 5-12 batch path so wording can't drift.

TESTS:
- Status: Adequate. regenerate_validate_test.go: reuse→changelog/both errors, reuse no-target→release, reuse→release unaffected by -y, changelog/both with changelog=false errors, fresh -y (+ fresh --all -y) without target errors, fresh without -y stays unset, reuse-wins-over-changelog-disabled ordering, reuse+disabled→release. Exact messages; errors.Is against sentinel. Pure-function (no FakeRunner needed).

CODE QUALITY:
- Followed conventions (sentinel + fmt.Errorf, table-driven t.Parallel tests, doc comments). SOLID good — single responsibility; changelog check extracted for 5-12 reuse. Low complexity, ordered guard clauses.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/regenerate_validate_test.go:103 — TestValidateRegenerateRequest_Errors covers reuse-wins-over-disabled only for targetChangelog; add a --reuse --target both + changelog disabled case to fully pin the ordering for the `both` target.
