TASK: interactive-mint-init-setup-1-2 — Apply the decided default-column representation convention to the SoT rows

ACCEPTANCE CRITERIA:
- [x] `context`, `prompt`, `fallback`, `version_file`, `version_pattern` ([release]) and `context`, `prompt` ([commit]) carry a blank Default cell.
- [x] `release_branch` and `provider` carry Default == `auto` (distinct from blank).
- [x] `diff_exclude` carries Default == `[]`.
- [x] `[release].ai_command`, `[release].timeout`, `[commit].ai_command`, `[commit].timeout` carry Default == `shared`.
- [x] `[release.hooks]` rows (preflight, pre_tag, post_release) carry no default value (blank, applied consistently).
- [x] tag_prefix==v, commit_prefix==🌿, publish==true, changelog==true, on_notes_failure==abort, max_diff_lines==50000.
- [x] The blank-for-empty-string cells are DISTINGUISHABLE from the `auto` cells (a test asserts release_branch/provider are NOT blank).
- [x] All standard gates pass (build + gofmt verified clean for the package; suite not executed per reviewer constraints).

STATUS: Complete

SPEC CONTEXT:
Spec "Config-metadata source of truth (SoT)" → "`default` column representation" (specification.md:89-96) decides the representation convention as part of the DECIDED behaviour (not a planning-only rendering choice), because the minimalism guidance (spec:185) depends on the column being unambiguous: empty-string defaults → blank; sentinel-empty "auto" defaults (release_branch, provider, where "" MEANS auto-derive/auto-detect) → `auto` (distinct from blank so the agent can tell "auto" from "no value"); empty collection (diff_exclude) → `[]`; per-verb override inherit defaults → `shared`; [release.hooks] keys → no default (blank or em-dash). The spec also requires the SoT convention to match the README per-key tables so the two surfaces (agent-facing SoT, human-facing README) stay mutually consistent.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/config/metadata.go:119-168 (the MetadataRows() literal). Convention documented in the file header comment at metadata.go:23-38.
- Notes:
  - Empty-string-default rows omit the Default field entirely (zero value "" → blank): [release] context/prompt/fallback/version_file/version_pattern (metadata.go:144-149) and [commit] context/prompt (metadata.go:163-164). Correct.
  - Sentinel-auto: release_branch (metadata.go:140) and provider (metadata.go:143) carry Default: "auto". Correct and distinct from blank.
  - diff_exclude carries Default: "[]" (metadata.go:130). Correct.
  - Per-verb overrides carry Default: "shared" — [release].ai_command/timeout (metadata.go:150-151), [commit].ai_command/timeout (metadata.go:165-166). Correct, and the inverse of the shared-level rows where the real value lives.
  - Hooks rows omit Default (blank) consistently across all three (metadata.go:156-158); the chosen no-default token is pinned in the test as hooksDefaultCell = "" (metadata_test.go:175).
  - Concrete scalars verbatim: tag_prefix "v" (metadata.go:138), commit_prefix "🌿" (139), publish "true" (141), changelog "true" (142), on_notes_failure "abort" (146), max_diff_lines via strconv.Itoa(DefaultMaxDiffLines) (128).
  - Shared ai_command/timeout cells are sourced from the EXPORTED constants (DefaultAICommand at 127; strconv.Itoa(int(DefaultTimeout / time.Second)) at 129) — the task 1-5 seam, set here from constants and pinned by 1-5's drift tests. The plan explicitly invites setting these from constants now (do-item 1, task 1-2). No drift.
  - Cross-checked against README per-key tables (README.md:203-242): release_branch→auto, provider→auto, diff_exclude→[], per-verb overrides→shared, blank cells for context/prompt/fallback/version_file/version_pattern, tag_prefix v, on_notes_failure abort, max_diff_lines 50000, timeout 60, ai_command "claude -p --model sonnet". The two surfaces are mutually consistent — exactly the spec's stated requirement. The README hooks table (README.md:231-233) carries no default column, consistent with the SoT's blank hooks cells.
  - No drift from plan. The implementer chose plain blank (not em-dash) for hooks — explicitly permitted by the spec ("blank OR —") and the plan, with the choice pinned in the test.

TESTS:
- Status: Adequate
- Location: internal/config/metadata_test.go
- Coverage (one test per convention branch, all six planned test names present):
  - TestMetadataRows_EmptyStringDefaultsRenderBlank (metadata_test.go:182) — all 7 empty-string-default rows assert Default == "".
  - TestMetadataRows_SentinelAutoDefaultsRenderAuto (213) — release_branch/provider assert Default == "auto" AND a separate NOT-blank assertion (232-234). This directly pins the load-bearing blank-vs-auto distinction the spec calls out (spec:91, edge case at phase-1-tasks.md:103).
  - TestMetadataRows_DiffExcludeRendersEmptyCollection (241) — asserts "[]".
  - TestMetadataRows_PerVerbOverridesRenderShared (260) — all four override rows assert "shared".
  - TestMetadataRows_HooksRenderNoDefault (286) — all three hooks rows assert == hooksDefaultCell, the consistency requirement.
  - TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim (393) — table-driven over the concrete scalars + the three shared-constant-derived cells; the [release] scalar wants are literals (their config constants are unexported, correctly noted in the test comment), the shared cells derive from exported constants.
  - Plus the 1-5 subsuming pins (309, 329, 349, 368) tie the shared ai_command/max_diff_lines/timeout cells to the exported constants, and prove timeout renders integer seconds ("60") not the duration String form ("1m0s").
- Edge cases from spec/plan covered:
  - Blank-vs-auto load-bearing distinction: covered by the explicit NOT-blank assertion (232-234). Strong.
  - Per-verb rows render `shared` NOT the real value: covered (260) and reinforced by the inverse shared-level pins.
  - Hooks no-default applied consistently and the choice documented: covered via the hooksDefaultCell constant + comment (175).
- Not over-tested: each test targets one distinct convention branch; assertions are focused on Default cells. The lookups go through the shared configtest.MustByLevelKey seam (single index), avoiding per-test re-folding. No redundant happy-path duplication. The timeout cell is asserted in three tests (constant-pin, integer-seconds-not-duration, verbatim-table) but each pins a genuinely different property (drift-to-constant, representation form, verbatim equality), so this is layered intent rather than redundancy.
- Would fail if the feature broke: yes — flipping any cell (e.g. blanking release_branch, or rendering a per-verb override as the real value) fails a named assertion that points at the exact (level, key) offender.

CODE QUALITY:
- Project conventions: Followed. External package config_test for the convention tests; internal package config for the SoT itself (the 1-1 option-(a) placement); t.Parallel() throughout; table-driven where the shape fits (ConcreteScalarDefaults, MetadataLevel_String); the (level, key) lookups go through the shared configtest seam. Heavy WHY-comments per CLAUDE.md, kept true to as-built (the header at metadata.go:23-38 states the convention and the constant-sourcing rationale). No fmt/os.Stdout in business logic. Pure value constructor.
- SOLID: Good. MetadataRows() is a single-responsibility pure accessor returning a freshly-built slice each call (no shared mutable backing array — noted at metadata.go:116).
- Complexity: Low. A flat slice literal; no branching in the row construction.
- Modern idioms: Yes. strconv.Itoa + time.Second division for the integer-seconds derivation mirrors the existing initgen derivation (no re-typed literal).
- Readability: Good. Section comments group rows by level and explain the auto-vs-blank and shared-inherit rationale inline.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (The hooks blank-vs-em-dash choice was a permitted ambiguity; the implementer picked blank, pinned it in the test via hooksDefaultCell, and documented why — no action required.)
