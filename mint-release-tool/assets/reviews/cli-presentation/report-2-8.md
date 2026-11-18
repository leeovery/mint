TASK: cli-presentation-2-8 — Unwound first-class event with ↩ glyph; suppress success end-of-run line on failure/abort

ACCEPTANCE CRITERIA:
1. Unwound is a first-class method on the Presenter interface (distinct from StageFailed), carries the summary, recorded by RecordingPresenter.
2. Pretty renders "↩ unwound  {summary}" with ↩ glyph; plain renders "unwound: {summary}"; engine summary (incl. "repo clean" tail) verbatim, no synthesised tail.
3. After a StageFailed, a subsequent RunFinished suppresses the success end-of-run line in both modes.
4. After an Unwound (no prior StageFailed — abort case), RunFinished suppresses the success line in both modes.
5. On a Warn-only run, RunFinished still emits the success line.
6. Unwound renders to stdout only (no stderr); presenter sets no exit code.

STATUS: Complete

SPEC CONTEXT: spec:70 (first-class event), :223 (per-event), :288 (failure/abort suppress success-only line; engine owns exit code). Worked examples spec:196 (pretty), :258 (plain). Gate-n abort produces an Unwound with no prior StageFailed.

IMPLEMENTATION:
- Status: Implemented
- Location: presenter.go:84 (interface), :422-439 (Unwind struct); plain.go:67 (terminalFailure), :213-223 (StageFailed sets flag), :237-240 (Unwound renders+sets flag), :428-440 (RunFinished suppression-first); pretty.go:127,138,256,509-519,535-539,881-893; presentertest/recording.go:26,50,86,151-153.
- Notes: Summary verbatim via %s (no tail synthesis). terminalFailure set by StageFailed and Unwound, NOT by Warn (verified). RunFinished checks suppression BEFORE verb switch, covering every verb shape. Pretty "unwound" padded via shared stageColumn (11) → 4 trailing spaces (spec's 2 spaces is illustrative); more consistent choice, documented, test asserts exact form. Unwound to out only; no exit code set.

TESTS:
- Status: Adequate
- Coverage: interface/payload (presenter_test.go:51,64-74,76-98); plain verbatim (plain_test.go:483-492), stdout-only (:498-509), byte-purity (:514-520), suppression StageFailed→RunFinished (:525-534), failure path (:539-556), abort path (:561-573), warn doesn't suppress (:455-464); pretty ↩ glyph colour-off/on (pretty_test.go:929-966), stdout-only (:972-983), suppression (:988-997), failure path (:1002-1019), abort path (:1024-1036), warn doesn't suppress (:911-920).
- Notes: Every AC in both modes. Not over-tested. Minor gap: empty-Summary edge (documented presenter.go:436-438) only exercised by output-less conformance call; non-blocking.

CODE QUALITY:
- Project conventions: Followed — focused tests, t.Parallel, compile-time interface assertions, presentertest subpackage.
- SOLID principles: Good — first-class method (open-closed/interface segregation); terminalFailure encapsulated per-run.
- Complexity: Low.
- Modern idioms: Yes — iota RunVerb, lipgloss styles, %s passthrough.
- Readability: Exemplary — doc comments state load-bearing invariants.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [quickfix] internal/presenter/plain_test.go + internal/presenter/pretty_test.go — add an empty-Summary render assertion (plain Unwound(Unwind{}) → "unwound: \n"; pretty → ↩ line with no summary) to lock the documented "empty string is legal" claim (presenter.go:436-438).
