TASK: cli-presentation-4-7 — Width robustness — decorative rules capped at min(terminalWidth, ~50), wrap-never-truncate, fixed stage lines stay fixed

ACCEPTANCE CRITERIA:
- Decorative notes rules (titled + closing) sized to min(terminalWidth, ~50) via a pure, tested ruleWidth core.
- Terminal narrower than cap → rule equals terminal width (no overflow); wider → rule equals cap.
- Undetectable width (error or <= 0) → rule falls back to cap.
- Notes body never truncated — a body line longer than cap/terminal rendered in full (wraps naturally); no truncation/ellipsis.
- Fixed short stage lines render identically regardless of terminal width.
- No special tiny-terminal handling — tiny width yields a tiny rule via the same min; body still wraps; --plain is the escape hatch.
- Plain mode untouched (no decorative-rule capping, no width math).

STATUS: Complete

SPEC CONTEXT: "The Pretty Layer" (spec:147) "Width robustness (light touch)" — no pervasive width math; single concession is decorative rules capped at min(terminalWidth, ~50); everything else wraps naturally, never truncate; stage lines fixed; tiny terminals are a --plain case. Plain Layer (spec:210,226) keeps fixed terse delimiters with no width math. Follows 2-5 (fixed-width rule, cap deferred here).

IMPLEMENTATION:
- Status: Implemented
- Location: width.go:22 (const ruleCap = 50), :37-42 (pure ruleWidth = min(termWidth, ruleCap), <=0 → cap), :54-60 (detectTermWidth via term.GetSize, error → 0 sentinel); pretty.go:101-112 (termWidth field default 0), :309-312 (WithTermWidth seam), :645-650 (ShowNotes sizes both rules to ruleWidth), :659-666 (notesTitledRule fill clamps >=1 so a title longer than cap is never truncated), :671-673 (notesClosingRule), :679-681 (displayWidth counts runes); wiring.go:67-70 (NewForStartup threads detectTermWidth(stdout) on pretty branch only; plain branch :72-74 doesn't probe).
- Notes: Pure core isolated from OS probe (mirrors mode.go split). Body written unchanged via shared writeNotesBody — never hard-wraps/clips. Plain ShowNotes genuinely untouched. Default termWidth 0 → cap so all 2-5 layout tests render identical fixed-cap rule (no regression). No drift.

TESTS:
- Status: Adequate
- Coverage: width_test.go:14-38 (ruleWidth table: 30→30, 200→cap, cap→cap, 0/-1→cap, 3→3, 1→1); :42-46 (ruleCap==50 pinned); :54-68 (detectTermWidth /dev/null → 0 → cap). pretty_width_test.go:36-52 (narrow 30 both rules 30), :57-70 (wide 200 → 50), :76-108 (undetectable → cap), :115-134 (tiny 3, fill clamp keeps title, body full), :142-164 (wrap-never-truncate 200-char line at width 20, no "…"/"..."), :171-184 (fixed stage line identical at 20 vs 200), :192-206 (plain fixed delimiters regardless of width, no U+2500 leak, no WithTermWidth knob). Test-side helpers source cap via RuleCapForTest (export_test.go:7).
- Notes: Every AC + listed test mapped to spec width axes. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — pure-core/OS-probe split per mode.go; injectable WithTermWidth; rune-count display width avoids U+2500 trap; golang.org/x/term reused.
- SOLID principles: Good — ruleWidth pure single-responsibility; detectTermWidth isolates impurity; ShowNotes composes.
- Complexity: Low.
- Modern idioms: Yes — builtin min, table-driven parallel tests.
- Readability: Good — cap value documented and test-pinned.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
