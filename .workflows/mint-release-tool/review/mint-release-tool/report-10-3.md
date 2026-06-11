TASK: 10-3 — Add SIGINT/SIGTERM Handling (type: bug; severity high; sources report-5-10, external-audit)

ACCEPTANCE CRITERIA:
- A cancelled context mid-spine (pre-PONR) triggers the surgical unwind path and pops any autostash.
- No cmd entry point uses a bare context.Background() for the release/regenerate spine.
- Post-PONR cancellation does not attempt an unwind (warn-only, unchanged).

STATUS: Complete

SPEC CONTEXT:
Release Lifecycle Invariants: "Everything before stage 6 is local-only and recoverable" — any stage 1–5 failure or user abort auto-unwinds every mutation back to the clean starting state. `git push --atomic` (stage 6) is the single point of no return; after the PONR mint never unwinds (warn-only on stage 7 failures). The Failure model table makes the asymmetry explicit: before the push mint deletes the tag it made and resets the release commit(s); the autostash restore ordering is load-bearing — unwind first, THEN pop the stash, leaving it intact + warning on a pop conflict. This task closes the gap where a bare context.Background() left Ctrl-C/SIGTERM with no unwind and no autostash pop, contradicting the fail-loud / repo-clean philosophy.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/main.go:64-65 — `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` + `defer stop()` built ONCE in run(), threaded into runRelease/runRegenerate/runInit (main.go:73-77). The version surface deliberately routes BEFORE the signal context (main.go:53-55, 78-79) since it spawns nothing to interrupt — documented.
  - cmd/mint/main.go — every cmd entry point now takes ctx context.Context (runRegenerate:94, runRegenerateSingle:163, runRelease:231, resolveRegeneratePublisher:221) and threads it into the engine; the former per-path `ctx := context.Background()` helpers are gone.
  - internal/engine/release.go:610-611 — the LAST PRE-PONR GATE: `if err := ctx.Err(); err != nil { return surfaceAndUnwind(ctx, deps, "tag", start, made, err) }` sits immediately before the atomic push (line 619-620), catching a Ctrl-C in the no-subprocess window between the bookkeeping commit and the push.
  - internal/engine/unwind.go:93 — `ctx = context.WithoutCancel(ctx)` detaches the recovery from the cancelled parent so the local-only reset/tag-delete still run; same resilience on the regenerate path (regenerate_write.go:342).
- Notes:
  - Only ONE context.Background() remains in all non-test source (main.go:64) — the correct single seed for NotifyContext. Verified by grep: no bare context.Background() survives on any spine entry point.
  - A cancellation DURING a pre-PONR subprocess (the AI call, a hook) already surfaces as that command's error and routes through the same surfaceAndUnwind above; the new gate closes only the remaining no-subprocess (Ctrl-C with nothing running) gap — correct minimal addition, no redundant per-step checks.
  - Matches the golang-cli skill convention exactly (signal.NotifyContext, both os.Interrupt + syscall.SIGTERM, defer stop()) — not a raw signal channel.

TESTS:
- Status: Adequate
- Coverage: internal/engine/release_cancellation_test.go pins both acceptance facets via the FakeRunner+Mutator seam (the FakeRunner ignores ctx, so cancellation is observable ONLY through the spine's explicit ctx.Err() gate — exactly the production gap):
  - TestRelease_CancelledBeforePush_Surgical_Resets — an already-cancelled context drives the spine through the bookkeeping commit (seedHappyGitThroughCommit), then asserts: abort non-zero; the atomic push was NEVER issued (post-PONR contract untouched); surgical reset to startingSHA ran; no tag deleted (none created yet); StageFailed precedes Unwound; exact summary "reset 1 commit; repo clean"; no Finish after Unwound.
  - TestRelease_CancelledBeforePush_Autostash_UnwindsThenPops — pins the load-bearing autostash ordering: reset (unwind) MUST precede the stash pop (indexOfCmd resetAt <= popAt), and the deferred pop runs even though the parent ctx was cancelled (recovery survives cancellation).
- Notes:
  - Both acceptance facets are covered: pre-PONR cancellation -> unwind + autostash pop, AND post-PONR untouched (push-never-issued assertion proves the cancellation was caught before crossing the PONR; the existing warn-only post-PONR path is unchanged and already tested elsewhere — release_test.go publish-failure suite).
  - Not over-tested: each test asserts a distinct invariant (surgical reset vs autostash ordering); no redundant happy-path variants.
  - Would fail if the feature broke: removing the ctx.Err() gate would let the spine reach the push (FakeRunner returns success) -> the push-never-issued and reset assertions both fail; removing context.WithoutCancel in Unwind would not surface in the FakeRunner (it ignores ctx), so that resilience is pinned by inspection/comment rather than a runner-level assertion — acceptable given the FakeRunner seam, see non-blocking note.

CODE QUALITY:
- Project conventions: Followed. Mirrors .claude/skills/golang-cli signal.go example precisely; ctx is the first parameter throughout (golang-context convention); recovery uses context.WithoutCancel rather than a fresh Background (preserves request-scoped values).
- SOLID principles: Good. The gate is a single 2-line check; recovery is the shared Unwind/surfaceAndUnwind already used by every other pre-push failure — the cancellation path reuses it rather than forking a parallel recovery (open/closed honoured).
- Complexity: Low. One added conditional on the spine; no new branching elsewhere.
- Modern idioms: Yes. signal.NotifyContext + context.WithoutCancel are the current (Go 1.21+) idioms; no raw channel + manual select.
- Readability: Good. The intent is documented at every load-bearing site (main.go:57-63, release.go:601-609, unwind.go:88-93) explaining the window, the asymmetry, and why detach is required.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/regenerate_write.go:317-324 (pushChangelogCommit) — the regenerate spine has the cancellation-resilient recovery (WithoutCancel at :342) but NO explicit pre-push ctx.Err() gate mirroring release.go:610. A Ctrl-C in the no-subprocess window between the regenerate CHANGELOG commit and its plain `git push origin HEAD` would not be caught until the push subprocess surfaces it (the FakeRunner ignores ctx, but real git push honours cancellation, so in production the push would fail and route through resetAndAbort anyway). The forward-path gate exists because the push is the one op that could otherwise cross the PONR before the signal is observed; for regenerate the same window is narrower and the push itself observes cancellation, so this is lower-risk. Task 10-3's acceptance criteria and test scope the forward release spine only, so this is not a gap against the task — but worth a decision on whether the regenerate spine warrants a symmetric pre-push gate for parity.
- [idea] internal/engine/unwind.go:93 + release_cancellation_test.go — the context.WithoutCancel resilience in Unwind is pinned by the autostash test only indirectly (the pop runs after cancellation), and the FakeRunner ignores ctx so a regression that dropped WithoutCancel would NOT fail any current test. Consider whether a runner seam that records the ctx passed to Mutate (asserting the unwind's Mutate calls receive a non-cancelled ctx) is worth adding to make that invariant regression-proof; this is a test-strategy decision, not a defect.
