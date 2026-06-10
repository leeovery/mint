TASK: cli-presentation-6-4 — Add golden full-worked-example transcript test per render mode

ACCEPTANCE CRITERIA:
- A plain-mode test drives the spec's full plain -y worked example and asserts the complete composed transcript.
- A pretty-mode test drives the spec's full pretty worked example with a spy/no-op spinner and a fixed profile, asserting the stable composed lines.
- The transcripts assert composition (inter-block spacing, notes-between-plan-and-gate ordering, gate echo column, trailing footer), not just individual events — and are NOT tautological.

STATUS: Complete

SPEC CONTEXT: Spec ships two worked transcripts — pretty human run (spec:153-190), plain -y agent run (spec:233-252) — as the contract for how per-event renderings COMPOSE. Spec transcripts are illustrative snapshots; the implementation legitimately differs (no decorative inter-block blank lines, plain "running..." start verb, flush notes body/rules).

IMPLEMENTATION:
- Status: Implemented
- Location: golden_transcript_test.go (new) — driveWorkedExample (:95-114) drives one canonical ~15-event sequence through both modes; TestPlainGoldenWorkedExampleTranscript (:122-151, byte-exact + err-empty); TestPrettyGoldenWorkedExampleTranscript (:161-207, termenv.Ascii + spy spinner + spinner-lifecycle assertions).
- Notes: Every golden byte cross-checked against real rendering code (plain.go RunStarted/StageSucceeded/ShowPlan/ShowNotes/Prompt echo/RunFinished; pretty.go brand/padStage/Plan block/flush rules/renderGate menu/footer). notesBody + test-side rule builders mirror production; default termWidth 0 → ruleCap. Pretty drives blocking StageStarted (feeds spy spinner, no live frame).

TESTS:
- Status: Adequate
- Coverage: both modes. Composition genuinely pinned (reasoned expected strings, full byte stream) — any mutation (blank line, reorder, wrong column, moved footer) fails. Plain asserts err empty; pretty asserts exactly two spy spinners created, none active.
- Notes: Not over-tested — per-event renderings unit-tested elsewhere; this adds assembly-level assertion. Spy count/active assertions verify the no-live-frame precondition.

CODE QUALITY:
- Project conventions: Followed — raw byte-compare matches package golden idiom; termenv.Ascii + spy factory standard.
- SOLID principles: Good — single shared driver.
- Complexity: Low.
- Modern idioms: Yes — diff-friendly string concat, shared notesBody const.
- Readability: Excellent — file header documents every spec-snapshot-vs-implementation reconciliation.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
