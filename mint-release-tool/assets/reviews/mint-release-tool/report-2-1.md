TASK: mint-release-tool-2-1 — AI transport layer (content-agnostic)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/ai/transport.go: Generate (two-attempt), attempt (fresh reader per attempt), classifyFatal (timeout/missing-tool sentinels), isValid, parseCommand, sentinels ErrNotesFailure/ErrTimeout/ErrCommandMissing, Config+defaults (claude -p), NewTransport. Retry covers content only; fatal causes short-circuit. Fresh strings.NewReader per attempt avoids consumed-reader empty-retry bug. No git/diff logic. All invocation via runner.CommandRunner.RunWith.

TESTS:
- Status: Adequate. Covers valid body unchanged + single invocation, stdin/stdout wiring, default claude -p name+args, overridden command split, retry-once-then-fail across empty/whitespace/non-zero-exit table w/ cross-sentinel exclusion, success on second attempt, fresh-prompt-on-retry regression, timeout not retried + distinguishable, missing tool not retried. Invocation counts asserted.

CODE QUALITY:
- Followed conventions (sentinels + errors.Is/%w, doc comments, runner seam, table tests + t.Parallel/t.Context). SOLID good — transport-only responsibility, DI. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] internal/runner/exec_runner.go:97-100 (assumption surfaced by transport.go:160) — on a context-deadline kill, exec.CommandContext returns *exec.ExitError (killed by signal), so translateRun takes the errors.As(&exitErr) branch and wraps the ExitError, NOT context.DeadlineExceeded. classifyFatal detects timeouts solely via errors.Is(err, context.DeadlineExceeded), which would be false in production — a real timeout would be misclassified as bad content and RETRIED, defeating "timeout not retried" end-to-end. Transport's own contract/tests are correct (they inject a DeadlineExceeded-wrapping error); the gap is the runner not surfacing the deadline. Fix: in translateRun, when ctx.Err() is DeadlineExceeded/Canceled, wrap that cause so errors.Is(err, context.DeadlineExceeded) holds. Strictly belongs to runner task (1-1); flagged here because 2-1 depends on it for the timeout criterion outside tests.
- [idea] internal/ai/transport.go:170-179 — isValid is empty/whitespace only; "not an error/refusal" is satisfied for refusals solely via the non-zero exit path. A polite-refusal body returned with exit 0 ("I'm sorry, I can't help with that.") would pass validation. Comment documents this as deliberate minimal choice with human review gate as backstop. Confirm minimal refusal detection is intended or add a sentinel/heuristic later.
