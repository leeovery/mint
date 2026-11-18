TASK: cli-presentation-2-7 — StageFailed renders captured underlying output; FAILED summary to stderr, captured body not

ACCEPTANCE CRITERIA:
- Both modes render the one-line FAILED summary to stdout and stderr.
- Both modes render captured Output to stdout: plain wrapped in `--- output --- … --- end output ---`, pretty below the ✗ line.
- The multi-line captured body is not written to stderr in either mode.
- When Output is empty, no delimiter/body block is rendered (FAILED line stands alone).
- A captured-body line resembling a delimiter is rendered verbatim and not mistaken for the real delimiter.
- Multi-line captured output preserves internal newlines/blank lines exactly.

STATUS: Complete

SPEC CONTEXT: Per-event table (spec:222); Spinner Lifecycle (spec:266) — captured output printed below the ✗ line; Output streams (spec:48-50) — one-line summary duplicated to stderr, captured body stdout-only.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:390-394 (StageFailure carries Name/Message/Output); plain.go:213-223 (FAILED to out AND err; non-empty Output wrapped via writeNotesBody to out only; empty returns early); pretty.go:509-519 (styled ✗ to out, unstyled summary to err; body below ✗ to out only; empty returns early); shared writeNotesBody presenter.go:29-39.
- Notes: Reusing writeNotesBody yields provable byte-identity across modes and positional (never content-matched) delimiters; delimiter-collision and blank-line preservation come for free.

TESTS:
- Status: Adequate
- Coverage (plain): RendersDelimitedOutputBlock (:295), SummaryToStderrWithoutBody (:313), EmptyOutputRendersFailedLineAlone (:337), DelimiterLikeBodyLineIsVerbatim (:359), MultiLineBlankLinesPreserved (:382), RendersOneLineSummary (:271). (pretty): RendersCapturedOutputBelowGlyphLine (:1051), SummaryToStderrWithoutBody (:1076), EmptyOutputRendersGlyphLineAlone (:1100), DelimiterLikeBodyLineIsVerbatim (:1117), MultiLineBlankLinesPreserved (:1131), BodySurvivesColourDowngrade (:1146).
- Notes: Every AC and edge in both modes. Plain uses exact equality; pretty uses Contains + ordering (styled line profile-dependent) with exact equality for deterministic lines. Not over-tested.

CODE QUALITY:
- Project conventions: Followed — reuses writeNotesBody (DRY), single-place fire-and-forget discard, precise doc comments.
- SOLID principles: Good — single responsibility, stream split expressed once, body rendering delegated.
- Complexity: Low.
- Modern idioms: Yes — io.WriteString for verbatim bytes.
- Readability: Good.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [do-now] internal/presenter/plain_test.go:269 — TestPlainPresenterStageFailedRendersOneLineSummary's doc comment still says the delimiter block and stderr duplication are "later phases", which is now stale (this task implemented both); reword to point at the sibling tests covering them.
