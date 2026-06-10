TASK: cli-presentation-1-6 — Wire and verify the stdout/stderr stream split

ACCEPTANCE CRITERIA:
- Both presenters accept distinct out/err writers; default startup wiring uses os.Stdout/os.Stderr.
- A success run writes narration to stdout and leaves stderr empty.
- A failure run writes the failure into the stdout narration AND writes the one-line FAILED/error summary to stderr.
- The multi-line captured body is not duplicated to stderr (contract now; fully exercised in Phase 2).
- Both PlainPresenter and PrettyPresenter obey the identical split (fixed regardless of render mode).

STATUS: Complete

SPEC CONTEXT: Spec (spec:16-24,46-51) fixes the output stream as one of three orthogonal axes: narration → stdout, errors/warnings → stderr, "fixed regardless of mode". Redirect-visibility (spec:49): a clean run leaves stderr empty; errors/warnings appear in both narration and stderr. spec:69: multi-line captured body is stdout-only, not duplicated to stderr — only the one-line summary is. Exit-code ownership stays with engine/main (spec:51).

IMPLEMENTATION:
- Status: Implemented
- Location: wiring.go:21-26 (New(mode, out, err) — ModePretty → NewPrettyPresenter(out, WithErr(err)), else NewPlainPresenter(out, err)); wiring.go:64-75 (NewForStartup wiring out=os.Stdout/err=os.Stderr as *os.File); plain.go:78-91 (constructors take out+err), :121-132 (writef→out / errf→err), :213-223 (StageFailed summary to out+err, body to out only), :255-258 (Warn dual-write), :237-240 (Unwound out-only); pretty.go:50-58, :205-209 (WithErr), :318-334 (writef/errf, nil-err no-op), :509-519 (StageFailed styled ✗ to out, unstyled summary to err, body to out only), :556-560 (Warn dual-write unstyled err), :535-539 (Unwound out-only).
- Notes: "Default startup wiring uses os.Stdout/os.Stderr" satisfied at NewForStartup (documented single production construction site). No main/cmd caller yet — intentional per plan phasing. No drift.

TESTS:
- Status: Adequate
- Coverage (wiring_test.go, black-box, bothModes table): SelectsImplementationMatchingMode (:47); SuccessRunLeavesStderrEmpty (:109); NonFailureEventsNeverWriteToStderr (:132); FailureSummaryAppearsOnBothStdoutAndStderr (:154); FailureOutputBodyNotDuplicatedToStderr (:429, AC4 guard); FailureStderrSummaryIsSingleLine (:368); FailureStderrSummaryIsUnstyled (:389, no ANSI even with forced profile on out); NewForStartupWiresStdoutStderrAndMode (:475, /dev/null CI-safe); plus Warn dual-write / Unwound out-only / suppression tests.
- Notes: All four task Tests bullets covered plus body-leak guard and unstyled/single-line stderr contracts. bothModes table keeps parity DRY. Behavioural (stream placement) not type assertions.

CODE QUALITY:
- Project conventions: Followed — black-box package, named subtests, t.Parallel, errf/writef centralise discarded write error, WithErr functional option.
- SOLID principles: Good — New narrow wiring seam returning interface (DI); split localised to StageFailed/Warn; errf isolates err-stream concern.
- Complexity: Low.
- Modern idioms: Yes — options pattern, io.Writer seams, interface return.
- Readability: Good — doc comments state the stream contract.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES:
- [idea] wiring.go:64-75 — NewForStartup is documented as the single production construction site but has no main/cmd caller yet (none in this topic's scope). When the main package lands in a later work unit, route startup through NewForStartup so the os.Stdout/os.Stderr default wiring is exercised end-to-end rather than only in tests.
