TASK: interactive-mint-init-setup-2-2 — Render the config-reference section from the Phase 1 SoT

ACCEPTANCE CRITERIA:
- [ ] The config-reference section renders one table line per config.MetadataRows() row, carrying key · level · default · description.
- [ ] Level is rendered via MetadataLevel.String() (TOML form: [release], [release.hooks], [commit], shared).
- [ ] ai_command and timeout each render as three distinct rows (shared, [release], [commit]); the render never collapses dual-level keys.
- [ ] Default-column tokens (blank / auto / [] / shared / hooks-blank) are carried verbatim from the SoT — never transformed, re-defaulted, or re-derived.
- [ ] The render reads the SoT (config.MetadataRows()) and re-derives no metadata; the test drives expectations from the SoT so a divergent re-derived table would fail.
- [ ] The rendered table is spliced under the config-reference marker so Guide() returns one finished string; the structural marker test from Task 2-1 still passes.
- [ ] All standard gates pass.

STATUS: Complete

SPEC CONTEXT:
The spec ("Config-metadata source of truth (SoT)" → "default column representation") decides the default-column tokens are part of the DECIDED behaviour, not a rendering choice: empty-string defaults → blank; sentinel-auto (release_branch, provider) → "auto"; empty collection (diff_exclude) → "[]"; per-verb inherit ([release]/[commit] ai_command/timeout) → "shared"; [release.hooks] keys → no default (blank/—). The render carries these verbatim; it does NOT implement the convention (Phase 1's metadata.go does). "Render targets and layering" decides mint setup is the SoT's single in-binary render target and the config reference is rendered from the SoT "so the agent reads option meanings from a drift-tested table rather than from template comments." The dual-level ai_command/timeout keys appear as one distinct row per level (shared + [release] + [commit]), never collapsed (the bijection contract).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/setupguide/setupguide.go:286-295 (configReferenceSection — marker + framing + splice)
  - internal/setupguide/setupguide.go:324-342 (renderConfigReference — the SoT-driven table render)
  - internal/setupguide/setupguide.go:355-360 (LevelCell — level cell via MetadataLevel.String() with top-level placeholder)
  - internal/setupguide/setupguide.go:366-371 (defaultCell — verbatim default token, single-space for blank)
  - internal/config/metadata.go:119-168 (the SoT consumed; confirmed signatures match the documented API)
- Notes:
  (a) Renders from the SoT, never re-derives: renderConfigReference() iterates config.MetadataRows() (setupguide.go:329) and copies row.Key, LevelCell(row.Level), defaultCell(row.Default), row.Description through verbatim. No metadata is hand-written in the prose path — confirmed via grep: the package's only config import is on this render path; every prose helper is config-free, matching the package doc (setupguide.go:7-10) and CLAUDE.md's SoT-as-single-source discipline.
  (b) Dual-level rows render distinctly: the render emits one line per SoT row in SoT order; metadata.go carries ai_command/timeout three times each (shared at :127/:129, [release] at :150/:151, [commit] at :165/:166), so the table inherits three distinct rows per key — never collapsed. Level cell differs per row ([release] vs [commit] vs top-level), so the lines are unambiguous.
  (c) Default tokens carried verbatim: defaultCell(row.Default) returns row.Default unchanged for any non-empty token ("auto", "[]", "shared", "v", "true", "🌿", "abort", numeric); for "" it emits a single space so the markdown cell is well-formed WITHOUT mutating the SoT token. This is a render-presentation choice (markdown needs a non-empty cell), not a re-default — the blank IDENTITY is preserved and the test asserts the raw-field shape rather than a transformed value. Aligns with the spec's "carries these verbatim".
  (d) Level via MetadataLevel.String(): LevelCell(level) returns level.String() when non-empty (TOML forms [release]/[release.hooks]/[commit]); the empty shared form is surfaced as "top-level". The placeholder is deliberately NOT "shared" — that token already means the per-verb inherit default in the Default column, so a "shared" level cell would read as two meanings in one table (documented at setupguide.go:344-354 and config/metadata.go:69-77). Level identity stays single-sourced in config.String(); the placeholder applies only when String()=="". This is a justified, well-reasoned divergence from a literal "shared" level label and does not re-derive level identity.
  (e) Splice + Guide() one-string: configReferenceSection() emits MarkerConfigReference + framing + renderConfigReference(); Guide() (setupguide.go:68-78) joins it as the final section. The Task 2-1 structural marker tests still pass (markers are unaffected by the table contents).
  (f) Markdown table integrity: confirmed no Description carries a literal "|" (would break the four-column row); descriptions use "→" not "|". Safe today; see NON-BLOCKING note on guarding this.

TESTS:
- Status: Adequate
- Coverage:
  - "it renders a config-reference line for every SoT row" → TestGuide_ConfigReferenceHasLinePerSoTRow (setupguide_test.go:333-344): drives the per-row presence assertion FROM config.MetadataRows(), so adding/removing a SoT row propagates without a test edit and a re-derived divergent table fails. Matches the acceptance "test drives expectations from the SoT".
  - "it renders ai_command/timeout as distinct shared, [release], [commit] rows" → TestGuide_ConfigReferenceRendersDualLevelKeysDistinctly (:350-363): asserts each (level, key) pair appears on its own line via lineHasKeyAtLevel. Covers the no-collapse edge case.
  - default-token verbatim (the [] / auto / shared / blank tokens) → TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim (:383-413): table-driven over diff_exclude=[], release_branch/provider=auto, [release].ai_command/[commit].timeout=shared, [release].context=blank. Drives expectations from the live SoT via configtest.MustByLevelKey, so a token change in the SoT flows through.
  - blank-default seam → TestGuide_ConfigReferenceBlankDefaultRendersSingleSpaceCell (:425-454): the LOAD-BEARING test. It guards the blank case against the vacuous strings.Contains(line, "") trap by inspecting the RAW third pipe-field shape (blankDefaultCellRenders, :72-83) — so a regression emitting "auto", collapsing to "||", or dropping a delimiter for a blank-default key turns it red. This is genuinely strong (not a token-presence check). It also self-guards by Fatalf-ing if the chosen key (context) ever gains a non-blank default.
  - level via String() → TestGuide_ConfigReferenceRendersLevelViaString (:462-473): each row carries its LevelCell on its line, driven from the SoT.
  - shared-level non-colliding token → TestGuide_ConfigReferenceSharedLevelUsesNonCollidingToken (:480-501): uses max_diff_lines (shared-only, no dual-level twin) as an unambiguous shared row and asserts its level cell is "top-level" and not "shared".
  - closed-enum render half → TestLevelCell_OutOfRangeLevelDoesNotRenderTopLevel (:509-522): proves an out-of-range MetadataLevel is NOT masqueraded as the shared/top-level cell.
- Notes:
  Not under-tested: every acceptance criterion has a matching assertion; the spec's edge cases (dual-level distinctness, verbatim tokens incl. the blank trap, level via String()) are each covered. The drift-resistance is real — expectations come from config.MetadataRows()/configtest, not frozen literals.
  Not over-tested: the suite is focused. TestGuide_ConfigReferenceRendersLevelViaString (per-row level presence) and TestGuide_ConfigReferenceHasLinePerSoTRow (per-row full-line presence) overlap partially — the level-via-String test is a strict subset of the per-row line check (the latter already asserts the level cell is on the row). This is mild redundancy, justified by intent-naming (one pins "level comes from String()", the other pins "every row present") and is not bloat. Likewise the dual-level test is partly subsumed by the per-row test, but it pins the no-collapse contract explicitly — worth keeping. No excessive mocking; no implementation-detail coupling beyond the markdown column shape (which IS the contract for an agent-readable table).

CODE QUALITY:
- Project conventions: Followed. External test package (package setupguide_test), t.Parallel() throughout, table-driven where the shape fits, helpers carry WHY-comments. The single config import on the render path mirrors the SoT-single-source discipline (CLAUDE.md) and the package doc states it explicitly. No IO in the package (pure emitter, in the initgen spirit). The configtest seam is reused rather than re-duplicating the (level, key) indexer — exactly the pattern that package was created for.
- SOLID principles: Good. renderConfigReference / LevelCell / defaultCell each have a single responsibility; the render holds zero metadata logic (delegated to the SoT). Open/closed: a new SoT row needs no render change.
- Complexity: Low. Linear iteration, no branching beyond the two tiny cell helpers.
- Modern idioms: Yes. strings.Builder for the table, strings.TrimRight to drop the trailing newline.
- Readability: Good. The doc comments state the contracts (verbatim tokens, the non-colliding placeholder reasoning, the single-source claim) without drifting from as-built.
- Issues: None blocking. See non-blocking notes.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/setupguide/setupguide.go:355 — LevelCell is exported solely so the external test can single-source the placeholder through the production seam (no production caller exists; grep shows only setup.go calling Guide() and a comment reference in config). This widens the production surface for test convenience — the exact thing the configtest package was introduced to avoid. Decide whether to (a) keep it exported as a deliberate, documented render seam, or (b) move the placeholder constant into configtest / make the test internal so LevelCell can be unexported. Genuine design call; current choice is defensible (the comment at :350-354 documents it as "the ONE place the placeholder string lives").
- [quickfix] internal/setupguide/setupguide.go:337 — row.Description is written into a markdown cell unescaped. Today no description contains a literal "|" (verified), so the four-column row is intact, but a future SoT description with a "|" would silently break the table and the per-row tests would still pass (they use strings.Contains, not column-count). Add a guard test asserting no MetadataRow.Description contains "|" (or escape "|" in defaultCell/description rendering) so the markdown contract is enforced at the SoT boundary.
- [quickfix] internal/setupguide/setupguide_test.go:462 — TestGuide_ConfigReferenceRendersLevelViaString is a strict subset of TestGuide_ConfigReferenceHasLinePerSoTRow (both assert the level cell sits on each row, driven from the SoT). Consider folding it in or retitling to pin only the String()-sourcing aspect, to remove the partial duplication. Minor; the intent-naming arguably justifies keeping both.
