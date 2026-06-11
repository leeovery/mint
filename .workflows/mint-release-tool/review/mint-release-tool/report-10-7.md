TASK: Fix the Production Timeout Misclassification (Promoted Bug) — mint-release-tool-10-7 (type: bug)

ACCEPTANCE CRITERIA:
- A real context-deadline kill from exec.CommandContext produces an error for which errors.Is(err, context.DeadlineExceeded) is true.
- The AI transport classifies that as a non-retried timeout (single invocation).

STATUS: Complete

SPEC CONTEXT:
Specification "Output format & validation" (line 302) states a timeout is NOT retried — it goes straight to on_notes_failure, because "retrying a hung call only risks a second full timeout"; the single retry covers empty/error/refusal CONTENT only, bounding worst-case latency at one ~60s timeout. The defaultTimeout is the production ~60s per-attempt deadline (transport.go:62-64). This task fixes the production gap where a real deadline kill was being misclassified as bad content and retried, doubling worst-case latency and violating that guarantee.

IMPLEMENTATION:
- Status: Implemented (correct)
- Location:
  - internal/runner/exec_runner.go:99-108 — the new ctx.Err() branch in translateRun, placed BEFORE the *exec.ExitError branch (line 112-115). When ctx.Err() is non-nil (DeadlineExceeded or Canceled) it returns fmt.Errorf("running %q: %w", name, ctxErr), so errors.Is(err, context.DeadlineExceeded)/Canceled holds. Crucially this precedes the ExitError branch — exec.CommandContext SIGKILLs the child on deadline, surfacing an *exec.ExitError ("signal: killed") that would otherwise be taken first and hide the real cause.
  - internal/ai/transport.go:158-168 — classifyFatal unchanged; its errors.Is(err, context.DeadlineExceeded) check (line 160) now sees the wrapped cause through the runner error chain, returns ErrTimeout, and Generate (line 119-121) short-circuits without the retry.
- Notes: Ordering is correct and load-bearing — the ctx.Err() check is deliberately ahead of both the ExitError branch and the catch-all, and the code comment (lines 99-105, 117-118) documents exactly why. The branch returns the populated res (not Result{}), which is fine; callers on the timeout path branch on the error, not the Result. ProcessState.ExitCode() at line 85 is nil-safe per its comment. translateRun is shared by RunWith (the transport's path, line 50) and RunInDir (hooks, line 71), so the fix covers both real-exec entry points.

TESTS:
- Status: Adequate
- Coverage:
  - internal/runner/exec_runner_test.go:80-107 TestExecRunner_Run_DeadlineKillMatchesDeadlineExceeded — drives a REAL kill (sleep 5 under a 50ms context deadline), asserts errors.Is(err, context.DeadlineExceeded) AND that it does NOT match ErrCommandNotFound. This is the runner-level proof of acceptance criterion 1 with a genuine kill, not an injected wrapper.
  - exec_runner_test.go:109-128 TestExecRunner_Run_NonZeroExitDoesNotMatchDeadlineExceeded — guards the inverse: a normal non-zero exit must NOT match DeadlineExceeded or Canceled, so an ordinary command failure is never misread as a timeout. This is the regression guard that the new branch is gated on ctx.Err() and not over-broad.
  - internal/ai/transport_test.go:248-290 TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout — the END-TO-END production-path test required by the task: a real exec deadline kill (a temp executable script sleeping 5s under a 300ms Config.Timeout) classified as ai.ErrTimeout and distinguishable from ai.ErrGenerationFailed. This is the post-12-1 form: it no longer reads/writes a marker file nor asserts an invocation count (the load-dependent flake was removed per task 12-1), but the core classification assertion — errors.Is(err, ai.ErrTimeout), NOT ErrGenerationFailed, against a REAL kill — is present and is exactly what the task asked to preserve.
  - The "single invocation / no-retry-on-timeout" half of criterion 2 is covered deterministically by transport_test.go:226-246 TestTransport_Generate_DoesNotRetryTimeout (FakeRunner, asserts exactly 1 invocation). The real-kill test cross-references this (lines 259-263), correctly delegating the count assertion to the deterministic test to avoid a startup-race flake.
- Notes: No under-testing — both criteria are covered, the REAL-kill requirement is met at both the runner and transport layers, and the inverse (non-zero exit) is guarded. No over-testing — the real-kill transport test deliberately drops the redundant/racy invocation-count assertion and leans on the deterministic FakeRunner test for it; this is a sound split, not a gap. The rename ErrNotesFailure -> ErrGenerationFailed (commit 9e378d6) is applied consistently; no stale ErrNotesFailure references remain anywhere under internal/.

CODE QUALITY:
- Project conventions: Followed. Uses %w wrapping so errors.Is composes (golang-error-handling), inspects ctx.Err() rather than string-matching "signal: killed" (golang-context), and the tests use plain testify-free table/std-error assertions consistent with the surrounding file's style.
- SOLID principles: Good. Single shared translateRun keeps RunWith/RunInDir/(interactive analog) consistent; the fix lives in one place and propagates to all exec callers (DRY).
- Complexity: Low. One added guard clause with an early return; no added branching depth.
- Modern idioms: Yes. errors.As for the ExitError branch, errors.Is at the classify site, %w throughout.
- Readability: Good. The 99-105 comment explains the SIGKILL-hides-the-cause subtlety and why the branch must precede the ExitError branch; the 117-118 comment notes the catch-all no longer needs to handle ctx cancellation. Intent is self-documenting.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
