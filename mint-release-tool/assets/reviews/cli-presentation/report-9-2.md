TASK: cli-presentation-9-2 — Tighten deterministic positive substring assertions to exact line matches

ACCEPTANCE CRITERIA:
- The six cited positive assertions assert the complete deterministic line via == or full-line HasSuffix, not a bare fragment Contains.
- All negative/absence Contains checks in both files are unchanged.
- No production (non-test) source is modified.
- Tightened assertions pass against the real rendered output.

STATUS: Complete

SPEC CONTEXT: spec fixes the exact worked-example lines this task asserts: gate menu "    y  accept & proceed [default]" (spec:137,:178) and init "✓ created .mint.toml" / "· skipped release (exists, use --force)" (spec:280). Tightened assertions match byte-for-byte.

IMPLEMENTATION:
- Status: Implemented
- Location: init_test.go:179 (stripANSI(got) == "  ✓ created .mint.toml\n" — folds the two created fragments into one exact ==); init_test.go:202 (== "  · skipped release (exists, use --force)\n"); pretty_gate_test.go:83 + :125 (hasExactLine(got, "    y  accept & proceed [default]"), was fragment Contains); helper pretty_helpers_test.go:67 (hasExactLine does whole-line == over strings.Split, incl. final unterminated prompt segment).
- Notes: Verified against real renderers — pretty.go:844 (created), :840 (skipped), renderGate :799 (gate line). stripANSI strips only CSI-SGR, preserves trailing \n, so init == checks are strict full-buffer equality. Under Ascii profile lipgloss emits no escapes, so full-line == is the correct strictest tool. No production source modified.

TESTS:
- Status: Adequate
- Coverage: task IS test-tightening; the six cited positive assertions now assert complete deterministic lines. Negative/absence Contains checks confirmed untouched (init_test.go:221,241,317,320,324,343,347,314,340; pretty_gate_test.go:92-94,105,108,128,131-136,147-152,196-206,221-232). init_test.go:182 (Contains("  \x1b[")) is a positive ANSI-presence assertion under TrueColor (full == impossible), correctly not one of the six and left as Contains.
- Notes: Genuinely stricter (any extra surrounding char now fails). No coverage lost, no over-testing.

CODE QUALITY:
- Project conventions: Followed — reuses shared hasExactLine/stripANSI; comments explain "complete line not a fragment" intent.
- SOLID principles: N/A (test-only).
- Complexity: Low.
- Modern idioms: Yes.
- Readability: Good — failure messages retained, rationale documented inline.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/pretty_gate_test.go:128 — the positive Contains(got, "    n  abort") asserts a deterministic complete line (n non-default in ReuseConfirmGate, full line exactly "    n  abort"). Outside the six cited sites so correctly untouched, but for full consistency with this task's rationale could be tightened to hasExactLine. Mechanical, no logic change.
