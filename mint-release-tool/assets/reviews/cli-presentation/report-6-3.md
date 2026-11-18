TASK: cli-presentation-6-3 — Consolidate decorative notes-rule expectation and the ANSI-strip helper into one shared pretty test helper (tick-76a288)

ACCEPTANCE CRITERIA:
- The "── release notes · v" prefix literal and the fill/clamp arithmetic appear exactly once in the test tree.
- The decorative rule width is sourced once (not a duplicated scattered 50 literal).
- stripANSI/ruleDisplayWidth live in one shared helper file and are referenced (not re-defined) by the consuming pretty tests.

STATUS: Complete

SPEC CONTEXT: Test-only consolidation (duplication cleanup from analysis cycle c2). Underlying behaviour (spec:132,147): titled rule + verbatim body + closing rule, capped at min(terminalWidth, ~50); production cap lives once in width.go (ruleCap=50). Ensures the test side doesn't re-encode the prefix literal, arithmetic, cap, or ANSI stripper in multiple places.

IMPLEMENTATION:
- Status: Implemented
- Location: pretty_helpers_test.go — single notesTitlePrefix (:27), notesTitledRule/notesClosingRule builders (:36,:47) sourcing width from presenter.RuleCapForTest, ruleDisplayWidth (:56), single stripANSI (:80); export_test.go:7 (RuleCapForTest = ruleCap); pretty_width_test.go:13-16 (consumes shared helpers); pretty_test.go:1169-1259, golden_transcript_test.go:182-184, init_test.go:179,202 (consume shared helpers).
- Notes: Production notesTitledRule/notesClosingRule (pretty.go:659,671, package presenter, width param) correctly NOT collapsed into the test helper (production code, not a duplicate); production builds the prefix from ruleChar constants so the literal "── release notes · v" appears only once across the test tree.

TESTS:
- Status: Adequate
- Coverage: grep confirms single definition of "── release notes · v" (pretty_helpers_test.go:28), func stripANSI (:80), ruleDisplayWidth. Cap single-sourcing guarded by TestRuleCapForTestMirrorsProductionRuleCap (:15, fails if RuleCapForTest drifts from 50). Downstream width/layout tests assert exact rendered rule against the consolidated builders.
- Notes: Not over-tested — consolidation removed a copy. Test-side builder re-derives arithmetic as an independent oracle (catches production regression), guarded against cap drift by the mirror test.

CODE QUALITY:
- Project conventions: Followed — black-box package, cap via tiny export_test.go accessor, t.Helper() where applicable.
- SOLID principles: Good — one shared primitive per concern.
- Complexity: Low — stripANSI a minimal CSI-SGR scanner.
- Modern idioms: Yes — strings.Builder, rune-slice iteration.
- Readability: Good — each helper documents why it's the single definition.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] internal/presenter/pretty_width_test.go:64,67,100,103 — the "want 50" cap assertions use a bare 50 literal rather than presenter.RuleCapForTest. These are expected-output assertions (not rule-construction sourcing, which is correctly single-sourced), so outside this task's acceptance. Decide whether to route them through RuleCapForTest for full single-sourcing or keep 50 as a deliberate spec-pin.
