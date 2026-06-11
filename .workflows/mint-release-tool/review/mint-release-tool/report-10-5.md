TASK: Stop Reporting "Repo Clean" When the Unwind Itself Failed (mint-release-tool-10-5, type: bug)

ACCEPTANCE CRITERIA:
- A failed mid-unwind recovery `Mutate` produces a `Warn` naming the needed manual cleanup.
- The summary in that case does NOT contain "repo clean".
- A fully-successful unwind still reports "repo clean" as before.
- Test: a failed mid-unwind `Mutate` yields a warn and a summary without "repo clean".

STATUS: Complete

SPEC CONTEXT:
specification.md "Failure model" (line 416) — for any pre-push failure mint "auto-unwinds its own mutations … and it reports what it undid." Lines 50/52 establish unwind is the local-only pre-PONR recovery. The remediation makes that report TRUTHFUL: when the recovery mutation itself fails the repo is NOT clean, so the narration must not claim it is. Aligns with the regenerate sibling recovery (lines 559-560).

IMPLEMENTATION:
- Status: Implemented (all three sites)
- Location:
  - internal/engine/unwind.go:111-120 (deleteTagIfMade) — captures the Mutate err, emits warnUnwindIncomplete naming `git tag -d {tag}`, returns (issued, ok) with ok=false on failure. No longer discards the error.
  - internal/engine/unwind.go:130-139 (resetCommitsIfMade) — captures the Mutate err, emits warnUnwindIncomplete naming `git reset --hard {start.HEAD}`, returns ok=false on failure.
  - internal/engine/unwind.go:95-101 (Unwind) — threads tagOK && resetOK into unwindEvent as the `clean` flag, so the summary tail is gated on full recovery success.
  - internal/engine/unwind.go:146-152 (warnUnwindIncomplete) — shared helper; emits presenter.Warning{Label:"unwind incomplete", Message:"automatic recovery failed; manually "+action, Output: cause.Error()} mirroring warnPublishFailed's captured-output style.
  - internal/engine/unwind.go:176-199 (surgicalSummary / summaryTail) — tail is "; repo clean" only when clean, else "; manual cleanup required". Conditional appended correctly.
  - internal/engine/regenerate_write.go:330-347 (resetAndAbort) — the third site (the task names line 322, which renumbered to the reset Mutate now at 342); captures the reset Mutate err and calls the SAME warnUnwindIncomplete helper naming `git reset --hard {startingHEAD}`. No longer swallowed.
- Notes: All three discard sites named in the task are remediated. The helper is defined once and reused across unwind.go and regenerate_write.go — no duplication. The bool-pair (issued, ok) is a clean way to both narrate the issued item and gate the tail. Note: the task referenced regenerate_write.go:322; in the as-built file line 322 is now pushDone() and the actual reset Mutate is at 342 inside resetAndAbort — the same logical site, correctly fixed.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/unwind_test.go:261 TestUnwind_ResetFails_WarnsManualCleanupAndSummaryNotClean — reset --hard fails: asserts a recovery Warn naming the starting sha AND summary lacks "repo clean". Directly satisfies the acceptance test.
  - internal/engine/unwind_test.go:292 TestUnwind_TagDeleteFails_WarnsManualCleanupAndSummaryNotClean — tag -d fails: asserts Warn naming the tag AND summary lacks "repo clean". Covers the second discard site.
  - internal/engine/unwind_test.go:322 TestUnwind_AllRecoverySucceeds_NoWarnAndReportsClean — proves the success path is unchanged: NO Warn and summary keeps "; repo clean". Satisfies the third acceptance criterion (regression guard).
  - internal/engine/regenerate_write_test.go:229 TestRegenerateWrite_RecoveryResetFails_WarnsManualCleanup — push fails then the reset recovery fails: asserts the reset was attempted, a recovery Warn names the startingHEAD, and the run still aborts non-zero. Covers the third site.
  - scriptedMutateFailure (unwind_test.go:237) models a NON-lock failure so the Mutator surfaces on first attempt without retrying — correct, avoids masking the failure behind retry semantics.
- Notes: Tests assert behaviour (Warn label/message content, summary string, abort exit) not implementation details. The Warn lookup is by label "unwind incomplete" via the shared recoveryWarn helper (one definition, reused across both test files in-package). Not over-tested: each test isolates one site/path; the all-success test is a necessary regression guard, not redundant. Not under-tested: both unwind sites (reset, tag-delete) and the regenerate reset site each have a dedicated failure test plus a success guard. Would fail if the feature broke (e.g. if the tail were unconditional the not-clean assertions would catch it; if the warn were dropped recoveryWarn would t.Fatalf).

CODE QUALITY:
- Project conventions: Followed. Mutate errors no longer discarded (golang-error-handling). Warn rides the existing seam mirroring warnPublishFailed (golang-safety: recovery is best-effort, abort still carries the original reason). Tests use the in-package fake runner/recording presenter per golang-testing.
- SOLID principles: Good. Single shared warnUnwindIncomplete avoids duplicating the manual-cleanup notice across three sites; summaryTail isolates the tail selection; the (issued, ok) pair keeps narration and gating concerns separable without leaking Mutate internals to the caller.
- Complexity: Low. surgicalSummary switch is unchanged in shape; summaryTail is a two-line branch.
- Modern idioms: Yes. Multi-return bool pair, context.WithoutCancel preserved for cancellation-resilient recovery.
- Readability: Good. Doc comments at each site explain the "no longer discarded" change and why the tail is conditional.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/unwind.go:158-160 — unwindEvent/Unwound fires only when (tagDeleted || commitsReset); if the ONLY issued op fails (e.g. tag-not-made, single reset that fails), the user still gets the manual-cleanup Warn but NO Unwound summary line at all. Current tests always have a second succeeding op so a summary is always emitted. Consider whether a fully-failed single-op unwind should still emit a summary with the "; manual cleanup required" tail, or whether the standalone Warn suffices — a decision, not a defect.
