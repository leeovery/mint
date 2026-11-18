TASK: Fix Mutator.Mutate Retrying with a Consumed io.Reader (mint-release-tool-10-4, type: bug)

ACCEPTANCE CRITERIA:
- A lock-retried stdin-bearing mutation receives the full stdin (not an empty reader) on the second attempt.
- `git tag -a … -F -` produces the full annotation even when the first attempt hit a lock and retried.
- `regenerate --reuse` reads back the complete annotation body after a retried tag creation.

STATUS: Complete

SPEC CONTEXT:
The tag annotation is "the single source mint ever reads" (specification.md:338, :503): `regenerate --reuse` sources the release body solely from the annotation via `git for-each-ref … contents:body`, parse-free. The "Lock-resilient git" section (:404) establishes that every git mutation flows through the retry-on-lock wrapper. These two facts compound the bug: a lock retry that pipes an exhausted reader to `git tag -a … -F -` writes an empty annotation, and spec :528 confirms an empty annotation body silently degrades `--reuse` (fail-loud in single mode / skip-and-report in --all). The fix preserves the load-bearing invariant that the full annotation always reaches git.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/git/mutator.go:151 — `Mutate` signature now takes `stdin []byte` (was `io.Reader`).
  - internal/git/mutator.go:184-189 — `invoke` takes `stdin []byte`; builds a FRESH `bytes.NewReader(stdin)` on every call (line 188), so each retry attempt pipes the complete payload. The `nil` path still delegates to `m.runner.Run` unchanged (line 185-186).
  - internal/git/mutator.go:156-157 — the retry loop calls `invoke(ctx, stdin, …)` per attempt, so a fresh reader is constructed per attempt.
  - internal/release/release.go:99 — `createAnnotatedTag` (the only stdin-bearing caller) now passes `[]byte(message)`.
- Notes: All 13 production `Mutate` call sites were swept (record/commit.go x4, release/release.go x2, engine/autostash.go x2, engine/unwind.go x2, engine/regenerate_write.go x3). Every non-stdin call passes `nil` and only `createAnnotatedTag` passes bytes — exactly as required. No `Mutator` interface exists to keep in sync (concrete `*git.Mutator` is injected everywhere), so the signature change is localized. The doc comment (mutator.go:138-142, :180-183) explicitly documents the consumed-reader trap and references the same discipline ai.Transport uses, matching the task's framing.

TESTS:
- Status: Adequate
- Coverage:
  - internal/git/mutator_test.go:332 `TestMutate_StdinBearingLockRetry_SecondAttemptGetsFullPayload` is the required regression test: a stdin-bearing mutation hits a (gone) lock on attempt 1, retries, and the test asserts BOTH recorded invocations piped the full body (`for i, inv := range invs { if inv.Stdin != body }`). This directly covers acceptance criteria 1 and 2.
  - internal/git/mutator_test.go:309 `TestMutate_WithStdin_PipesThroughRunWith` covers the non-retry stdin happy path (one attempt pipes the body).
  - The FakeRunner records stdin by draining it fresh per `RunWith` call (fake_runner.go:101, drainStdin at :163-169) — so the OLD single-reader code would record an empty string on attempt 2, which the regression test's per-attempt assertion would catch. The test genuinely fails against the pre-fix code (not a tautology).
  - The `nil`-path and lock-classification behaviours remain covered by the existing suite (stale-clear, live-backoff, exhausted-budget, non-lock-immediate, gone-lock-retry, clamped-budget, Run/RunWith pass-through).
- Notes: The third acceptance criterion ("regenerate --reuse reads back the complete annotation body after a retried tag creation") is satisfied transitively — the reuse read path lives in a separate unit (internal/engine/regenerate_reuse.go) with its own tests, and is the downstream consumer rather than this task's surface. A correct full-payload tag write is the necessary-and-sufficient condition for the reuse read to be correct; no end-to-end retry+reuse test is needed at this task's level, and adding one would be over-testing across unit boundaries. Not flagged.

CODE QUALITY:
- Project conventions: Followed. Matches golang-safety (no consumed-reader reuse — the exact trap the fix removes), golang-design-patterns (constructor-options seam unchanged), golang-testing (FakeRunner-scripted, deterministic, no real git, table-free focused regression test). `[]byte` over a `func() io.Reader` factory is the simpler of the two suggested options and the right call here — the payload is small and fully in-memory, so a factory would add indirection for no benefit.
- SOLID principles: Good. The change is contained to the one method whose contract was wrong; `invoke` retains single responsibility (one underlying run + fresh-reader construction).
- Complexity: Low. No new branches; the reader construction moved inside `invoke` so it re-runs per attempt for free.
- Modern idioms: Yes. `bytes.NewReader` per attempt is the idiomatic fix.
- Readability: Good. The doc comments (mutator.go:138-142, :180-183) explain WHY bytes not a reader, which is the load-bearing rationale a future reader needs.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
