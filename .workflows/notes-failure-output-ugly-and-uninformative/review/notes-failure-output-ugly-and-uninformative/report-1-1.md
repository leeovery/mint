TASK: 1-1 — Introduce the GenerationError carrier and populate it on a non-zero exit

ACCEPTANCE CRITERIA:
1. A *ai.GenerationError type exists in internal/ai, exported, carrying captured Stdout and Stderr as distinct exported fields (plus ExitCode).
2. errors.Is(err, ai.ErrGenerationFailed) returns true for the carrier (Unwrap returns the sentinel) — sentinel routing unaffected.
3. errors.As(err, &genErr) retrieves the carrier from a Generate call exiting non-zero on both attempts, with Stdout/Stderr/ExitCode equal to the runner's captured streams/exit code.
4. Both stdout and stderr are carried when both present (kept distinct; composition is Phase 2).
5. errors.Is(err, ai.ErrTimeout) and errors.Is(err, ai.ErrCommandMissing) are both false for the non-zero-exit carrier.
6. The command is invoked exactly twice on the non-zero-exit-on-both-attempts case.
7. internal/ai does not import config.
8. All project gates pass.

STATUS: Complete

SPEC CONTEXT:
Fix 1 is the load-bearing fix of this work unit: ai.Transport.attempt previously returned ("", err) on a non-zero exit, discarding the fully-populated runner Result (claude's actual message — e.g. "Prompt is too long" — on stdout). ErrGenerationFailed was a bare payload-less sentinel, so StageFailure.Output ended up empty and the operator saw nothing actionable. The spec mandates a typed carrier *ai.GenerationError that (a) wraps the ErrGenerationFailed sentinel so errors.Is routing survives, and (b) carries the captured Stdout/Stderr as distinct fields (plus ExitCode), mirroring *hooks.HookError. Invariants to preserve: errors.Is(ErrGenerationFailed) still matches (Inv 1), context.Canceled passthrough (Inv 2 — pinned by Task 1-3), transport stays content-agnostic / never imports config (Inv 3), single-retry ownership unchanged (Inv 4), byte-identical success (Inv 5). Task 1-1 is confined to internal/ai and to the non-zero-exit path; the empty/whitespace path is Task 1-2 and the Error() dual-provenance honesty is a later task (T4-2) — both already landed, so the as-built file reflects them.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport.go:56-85 (GenerationError struct, Error(), Unwrap()); transport.go:194-234 (Generate); transport.go:257-266 (attempt now returns runner.Result). Commit 533d5d2 is the T1-1 landing.
- Notes:
  - AC1 met: GenerationError is exported with exported Stdout string, Stderr string, ExitCode int fields (transport.go:56-68). Mirrors *hooks.HookError (hooks.go:27-41) but holds streams as distinct fields rather than the whole Result, exactly as the spec/plan require.
  - AC2 met: Unwrap() returns ErrGenerationFailed (transport.go:85); ErrGenerationFailed is retained as the bare sentinel (transport.go:30), now the wrapped target.
  - AC3 met: the non-zero-exit-survives-retry branch returns &GenerationError{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode} from the RETRY attempt's captured res (transport.go:220).
  - "attempt stops discarding res" met: attempt's signature is now (runner.Result, error) and it returns t.runner.RunWith(...) directly (transport.go:257-265) — no more "", err discard.
  - Retry-is-last-word met: the first attempt's res is intentionally not carried (comment at transport.go:204-205); only the second attempt's res is packed.
  - Fatal short-circuits untouched: classifyFatal (transport.go:272-291) is unchanged; timeout/missing-tool/cancel still return before the carrier path (transport.go:201-203, 212-215). context.Canceled propagates unchanged.
  - AC7 met: internal/ai imports only context, errors, fmt, strings, time, and mint/internal/runner (transport.go:13-21). grep confirms no mint/internal/config import anywhere in internal/ai. Invariant 3 holds — the carrier holds raw captured output, no notes/commit framing.
  - Runner-contract precondition verified: internal/runner/runner.go:25-26 and exec_runner.go:38,58,110-114 document that a non-zero exit returns a fully-populated Result alongside the non-nil error (translateRun builds Stdout before the error branch). The captured output is guaranteed present at the seam, not best-effort — the implementation's premise is sound.
  - As-built drift note (NOT a defect): the current file goes beyond the literal T1-1 scope — the empty/whitespace path also returns the carrier (transport.go:222-232, Task 1-2) and Error() handles dual provenance (transport.go:70-81, Task 4-2). This is correct: those tasks landed after T1-1 and the file is the cumulative state. Within the T1-1 commit (533d5d2) the empty-body path was still the bare sentinel and Error() was the simple "(exit %d)" form. Reviewing the live file against the full plan, nothing here contradicts T1-1's acceptance.

TESTS:
- Status: Adequate
- Coverage:
  - The four T1-1-mandated tests all exist with the required behaviour:
    * "carries captured stdout and stderr on a non-zero exit surviving the retry" → TestTransport_Generate_CarriesCapturedStdoutAndStderrOnNonZeroExitSurvivingRetry (transport_test.go:635-665): seeds Result{Stdout:"Prompt is too long", Stderr:"some stderr", ExitCode:1} + errors.New("exit status 1"); asserts errors.As succeeds and all three fields match. This is exactly the previously-untested shape (stdout present on a non-zero exit) that exposed the defect.
    * "still matches ErrGenerationFailed through the carrier wrap" → TestTransport_Generate_CarrierStillMatchesErrGenerationFailed (transport_test.go:667-687): asserts errors.Is(ErrGenerationFailed) true, ErrTimeout/ErrCommandMissing both false.
    * "carries stdout-only when stderr is empty on a non-zero exit" → TestTransport_Generate_CarriesStdoutOnlyWhenStderrEmpty (transport_test.go:689-711): Stderr:"" seed; asserts Stdout set and Stderr empty (no pre-merge).
    * "invokes the command exactly twice on a non-zero exit surviving the retry" → TestTransport_Generate_InvokesCommandTwiceOnNonZeroExitSurvivingRetry (transport_test.go:713-730): asserts len(r.Invocations()) == 2.
  - The existing table-driven TestTransport_Generate_RetriesOnceThenFailsOnBadContent (transport_test.go:182-229) was extended to also assert errors.As(&genErr) succeeds across all three bad-content rows, reinforcing AC2/AC3 without redundancy.
  - Each test would fail if the feature broke: removing the carrier construction (reverting to the bare sentinel) fails the errors.As assertions; dropping the ExitCode/Stderr threading fails the field assertions; re-discarding res in attempt fails all carrier-field tests; a second retry fails the invocation-count tests.
- Notes:
  - Not under-tested: AC1-AC6 are each directly asserted. The non-zero-exit case carries both streams + exit code (AC3/AC4), stdout-only (AC4 distinctness), sentinel matching + non-matching (AC2/AC5), and the exactly-two-invocations count (AC6).
  - Not over-tested: the four dedicated tests each pin one distinct facet; the field-level test and the sentinel-matching test cover orthogonal concerns (payload vs routing). The exactly-twice test is a separate behavioural proof from the field tests. No redundant happy-path variations, no implementation-detail assertions — every assertion is on observable behaviour (errors.Is/errors.As outcomes, field values, invocation count).
  - AC7 ("does not import config") is enforced structurally by the package layering, not by a unit test — appropriate, as it is a compile/import-graph property; the golangci-lint/build gate would surface a forbidden import. No test is warranted here.

CODE QUALITY:
- Project conventions: Followed. Carrier mirrors *hooks.HookError per CLAUDE.md/spec; Error() string is lowercase with no trailing punctuation (transport.go:78,80 — "ai generation failed (exit %d)" / "ai generation failed (empty body)"), matching the error-idiom rule. Sentinel wrapping via Unwrap with errors.Is matching is the project's standard pattern. Transport stays content-agnostic (Invariant 3 / non-negotiable seam 5). Tests are package ai_test (external), t.Parallel() throughout, FakeRunner-seeded, exact-value assertions — all per the test idioms.
- SOLID principles: Good. Single responsibility preserved — the carrier is a pure data carrier with two trivial methods; attempt does one thing (run + return Result); Generate owns routing. No interface churn; Generate's public signature is unchanged (the captured output travels on the returned error, as the spec mandates over a new return value).
- Complexity: Low. attempt simplified to a direct return of RunWith (one branch fewer than before). Generate's added branch is a single carrier construction; cyclomatic complexity is unchanged-to-lower.
- Modern idioms: Yes. Idiomatic errors.Is/errors.As via Unwrap; fmt.Sprintf for the diagnostic string; no reflection or unsafe.
- Readability: Good. The WHY-comments are thorough and true-to-as-built (e.g. transport.go:204-205 explaining the first attempt's res is deliberately not carried; transport.go:240-247 explaining why the whole res is returned). Comments state contracts/invariants the code can't show, per the project's comment policy.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
