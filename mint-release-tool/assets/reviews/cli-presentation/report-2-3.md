TASK: cli-presentation-2-3 — Pretty stage narration: stage line with detail and conditional elapsed

ACCEPTANCE CRITERIA:
- A long/blocking stage success renders '  ✓ {stage}  {detail} ({elapsed})' with elapsed appended.
- A short stage success renders '  ✓ {stage}  {detail}' with no elapsed.
- A stage with empty detail renders a detail-only line (glyph + padded name, no trailing detail text), honouring the elapsed rule.
- The stage name is padded to a consistent column so successive stage lines align.
- Under colour-on, the glyph/line carries ANSI colour codes; under colour-downgrade, no colour codes while ✓ glyph, indent, column padding survive.
- No spinner animation or in-place line replacement in this task (deferred to Phase 4).

STATUS: Complete

SPEC CONTEXT: "The Pretty Layer" (spec:130) stage line shape; per-event table (spec:221); event payload principle (spec:67) — elapsed on blocking only, presenter never times; worked example (spec:156-187) supplies literal lines.

IMPLEMENTATION:
- Status: Implemented
- Location: pretty.go:460-469 (StageSucceeded), :477-486 (stageTrailing), :908-916 (padStage), :921-923 (formatElapsed), :27 (stageColumn=11).
- Notes: Detail rendered verbatim (no synthesised separator). Blocking flag (not Elapsed value) gates elapsed — honours zero-value semantics 1 & 2. Detail-less short stage drops column padding to avoid trailing-whitespace artefact (semantic 3). No spinner/in-place logic added by this task. Padding via "%-*s" is byte-width; ASCII stage names in all spec examples so correct in practice (multi-byte out of scope).

TESTS:
- Status: Adequate
- Coverage (pretty_test.go): ElapsedRendersOnlyOnBlockingStages (163-196, flag-gates-not-duration); StageSucceededDetailOnlyHasNoArtefact (198-252, exact bytes + no-trailing-whitespace); StageNamesPadToCommonColumn (254-276); StageColumnSurvivesColourDowngrade (278-295, no ESC + exact line); ColourOnEmitsANSI (110-136); golden_transcript_test.go:174-181 locks full worked example.
- Notes: Behaviour-focused (exact lines / column indices / glyph presence). AC6 deferral is structural; spinner exercised by Phase-4 tests. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — lipgloss sole styling path, small doc-commented helpers.
- SOLID principles: Good — StageSucceeded delegates to stageTrailing/padStage/formatElapsed.
- Complexity: Low.
- Modern idioms: Yes — fmt width verbs.
- Readability: Good.
- Issues: None blocking.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
