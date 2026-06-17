TASK: 1-3 — Pin the load-bearing AI-seam invariants against the carrier change (test-only)

ACCEPTANCE CRITERIA (from plan phase-1-tasks.md):
1. context.Canceled propagates UNCHANGED from Generate: errors.Is(err, context.Canceled) true, none of the three transport sentinels match, AND errors.As(err, &genErr) returns false (no carrier).
2. The timeout path returns ErrTimeout with no *ai.GenerationError carrier (errors.As false) and exactly one invocation.
3. The missing-tool path returns ErrCommandMissing with no *ai.GenerationError carrier (errors.As false) and exactly one invocation.
4. A valid body is returned verbatim with err == nil and no carrier (byte-identical success path untouched — Invariant 5).
5. The no-deadline (Timeout: &0) parent-context cancel path also propagates context.Canceled unchanged with no carrier.
6. All project gates pass.

STATUS: Complete

SPEC CONTEXT:
Spec "Invariants to Preserve" #2 (context.Canceled stays a passthrough — never swallowed by the carrier or routed to a fallback) and #5 (byte-identical bodies on success). Spec "Per-cause Output behaviour": only the generation-failed cause carries captured output; ErrTimeout and ErrCommandMissing short-circuit via classifyFatal and do not populate Output (a missing binary has no output; a timed-out call's partial output is not captured by this fix). CLAUDE.md non-negotiable seam 5: a cancel is not an AI failure and never routes to a fallback. This task is the regression-guard layer over the Task 1-1/1-2 carrier change — test-only, no production change.

IMPLEMENTATION:
- Status: Implemented (test-only, as required)
- Location: internal/ai/transport_test.go; commit d82f3e0 ("T1-3 — pin AI-seam invariants against the carrier change").
- The T1-3 commit changed exactly three files: internal/ai/transport_test.go (+33 lines), .tick/tasks.jsonl, and a .tick manifest (both bookkeeping). NO production file (transport.go) was touched by this commit — confirming the test-only constraint. classifyFatal (transport.go:272-291) and the success path are untouched by this task.
- All five guards were ADDED as concrete errors.As(err, &genErr) negative assertions on the four named existing tests plus the no-deadline test — not left to assumption.

Per-criterion verification (negative-carrier guards added):
(a) context.Canceled — TestTransport_Generate_DoesNotRetryCancel (transport_test.go:314-345): asserts errors.Is(err, context.Canceled) true (line 328), all three sentinels false (line 331), errors.As(err, &genErr) == false (lines 338-341), exactly one invocation (line 342). MET.
(b) timeout — TestTransport_Generate_DoesNotRetryTimeout (transport_test.go:286-312): asserts errors.Is(err, ai.ErrTimeout) true (line 297), not ErrGenerationFailed (line 300), errors.As false (lines 305-308), one invocation (line 309). MET.
(c) missing-tool — TestTransport_Generate_DoesNotRetryMissingTool (transport_test.go:514-543): asserts errors.Is(err, ai.ErrCommandMissing) true (line 525), distinguishable from ErrGenerationFailed/ErrTimeout (lines 528, 531), errors.As false (lines 536-539), one invocation (line 540). MET.
(d) valid body verbatim — TestTransport_Generate_ReturnsValidBodyUnchanged (transport_test.go:42-72): asserts body verbatim (line 57), err == nil (line 54), errors.As false (lines 63-66), one invocation (line 69). MET.
(e) no-deadline parent-context cancel — TestTransport_Generate_NoDeadlinePathPropagatesParentCancellationUnchanged (transport_test.go:430-460): Timeout: &0 via ptrTo(time.Duration(0)) (line 443), seeds context.Canceled, asserts errors.Is(context.Canceled) true (line 445), sentinels false (line 448), errors.As false (lines 453-456), one invocation (line 457). MET.
(f) classifyFatal production logic UNCHANGED — verified: T1-3 commit touched no production file; classifyFatal's last change predates this work unit's carrier tasks. MET.

TESTS:
- Status: Adequate
- Coverage: All five named negative-carrier guards present and correctly seeded. Cancel seed uses fmt.Errorf("running claude: %w", context.Canceled); timeout uses context.DeadlineExceeded; missing-tool uses SeedNotFound("claude"); success uses a good Stdout body; no-deadline uses Timeout: &0 + context.Canceled — matching the plan's "Tests" list exactly.
- The guards complement (do not duplicate) the pre-existing sentinel/invocation-count assertions: each new block is the single errors.As-false addition the plan prescribed, layered onto the existing tests rather than re-asserting matched sentinels.
- The positive carrier-presence counterpart is covered by the unified Task 1-1/1-2 tests (TestTransport_Generate_RetriesOnceThenFailsOnBadContent asserts errors.As succeeds for all three bad-content shapes; transport_test.go:212-215), so the suite proves the symmetric contract: carrier present on bad-content, absent on cancel/timeout/missing/success.
- Would fail if broken: yes. If a future change wrapped context.Canceled (or a timeout/missing-tool error) into a *ai.GenerationError, the corresponding errors.As block fails; if the success path constructed a carrier on nil error, the success guard fails.
- Not over-tested: the success-path guard (criterion d) is the only one that is trivially true (errors.As on a nil error is always false). The plan explicitly anticipated this ("a nil error trivially yields no carrier ... If a dedicated guard reads cleaner, add a one-liner") and the as-built keeps it as a cheap, explicit pin with a clear WHY-comment (Invariant 5). Acceptable, not redundant — it documents intent and guards against a future success path that returns a non-nil error.

CODE QUALITY:
- Project conventions: Followed. package ai_test (external), t.Parallel() on every test, FakeRunner seeding (Seed/SeedNotFound), exact-matcher assertions, errors.Is/errors.As per golang-error-handling. Each guard carries a WHY-comment stating the invariant it pins (Invariant 2 for cancel, Invariant 5 for success), consistent with the codebase's heavy WHY-comment contract.
- SOLID / DRY: Good. The repeated `var genErr *ai.GenerationError; if errors.As(...)` idiom across five tests is intentional per-test pinning, not extractable duplication — each lives in its own behavioural test and a shared helper would obscure which invariant each guards.
- Complexity: Low — straight-line assertions.
- Modern idioms: Yes — errors.As with a typed target pointer, t.Context().
- Readability: Good — comments name the precise load-bearing risk (cancel misrouted to commit's editor fallback) rather than restating the code.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. The implementation matches the plan's "Tests" list and "Do" steps verbatim; no actionable concrete change identified.
