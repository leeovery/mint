TASK: Defer staging to gate-accept; abort leaves the index untouched (commit-command 2-3)

ACCEPTANCE CRITERIA:
- On accept under -a, git add -u runs then the commit, in that order
- On accept under -A, git add -A runs then the commit, in that order
- On accept under the default mode, no git add runs (existing index committed — Phase 1 path)
- On abort (n) under any mode, no git add and no commit run
- After an abort, the index is exactly the pre-mint state, including pre-existing user staging (untouched)
- -y auto-accept follows the accept path (stages for the mode, then commits)
- The git add and git commit both run via the consumed git_safe wrapper, not the raw runner
- No editor save-as-accept (Phase 3) or -p push (Phase 5) behaviour implemented

STATUS: Complete

SPEC CONTEXT:
Commit Flow / Lifecycle and the "Invariant — mutate nothing until accept; never unwind after" sections make deferred staging the cross-cutting property that makes abort a true no-op: under -a/-A the would-be-committed diff is computed read-only for message generation, and the git add runs only after gate-accept (or -y auto-accept), in stage-then-commit order; -a maps to git commit -a semantics (git add -u, tracked-only), -A maps to git add -A (incl. untracked); the default StagedOnly mode runs no git add. Abort (n) must leave the index byte-identical to its pre-mint state, including any pre-existing user staging. All mutations flow through the lock-resilient git_safe wrapper. The empty-staging cases (2-4) and editor/push (Phases 3/5) are out of scope for this task.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:730-747 (stageForMode) — All → `git add -u`, AddAll → `git add -A`, StagedOnly → early return, no add. Routed through deps.Mutator (git_safe), stdin nil.
  - internal/commit/run.go:786-803 (commitAccept) — the single shared accept tail: stageForMode FIRST, then createCommit, then pushAfterCommit; this is the only place the index is mutated.
  - internal/commit/run.go:342-367 (Run) — reviewLoop/gate sit BEFORE commitAccept; a decline returns errGateAborted (run.go:353-360) before any mutation, so abort touches nothing.
  - internal/commit/run.go:760-766 (createCommit) — `git commit -F -` via Mutator, body byte-for-byte, runs no git add itself.
  - internal/commit/source.go:61-66 / generate.go — the -a/-A read-only source is `git diff HEAD -- .` on the plain Runner (not the Mutator), confirming the read phase never mutates the index.
  - cmd/mint/commit_flags.go:86-97 (resolveStagingMode) — resolves -a/-A into the StagingMode threaded into Deps.Staging.
- Notes: Clean separation between the read phase (plain Runner) and the mutation half (Mutator). The accept tail is shared by both the gate-accept path and the editor save-as-accept path (Phase 3), so the stage→commit ordering lives in exactly one place and cannot drift. No editor or push behaviour is added by this task; those branches exist but are owned by other tasks (3-x, 5-x) and are correctly out of this task's scope.

TESTS:
- Status: Adequate
- Location: internal/commit/staging_defer_test.go
- Coverage: One focused test per acceptance criterion, all driving the REAL Generator + REAL git.Mutator (git_safe) over a single FakeRunner (newCommitDeps, run_test.go:61):
  - TestRun_AcceptUnderAll_AddsTrackedThenCommits — asserts exactly one add, argv `add -u`, and add index < commit index (strict stage-then-commit order).
  - TestRun_AcceptUnderAddAll_AddsEverythingThenCommits — exactly one add, argv `add -A`, add-before-commit ordering.
  - TestRun_AcceptUnderDefault_NoGitAddCommitsIndex — zero adds, commit stdin == generated body verbatim.
  - TestRun_AbortUnderAll_NoGitAddNoCommit — gate-no returns non-zero, zero adds, zero commits.
  - TestRun_AbortUnderAddAll_IndexUntouched — zero adds, zero commits, and an explicit sweep asserting NO add/commit flowed through the sink.
  - TestRun_AbortLeavesPreExistingStagingUntouched — proves no index-altering git call (add/commit) on the abort path.
  - TestRun_DashYAutoAcceptUnderAddAll_StagesThenCommits — unscripted presenter (models -y Default=ChoiceYes); stages `add -A` then commits, in order, with no gate decline.
  - TestRun_StagingAddRunsViaGitSafe — seeds a genuine lock-contention stderr (`Unable to create '...index.lock': File exists` + `Another git process seems to be running`) on the first `add -u`; the real git.Mutator with a no-op backoff retries, yielding TWO add invocations. This is a sound, behavioural proof that the staging add goes through git_safe and not the raw runner (a raw runner would surface the first lock failure and never retry).
- Notes:
  - The abort tests prove "index untouched" indirectly — by the absence of any add/commit invocation — rather than by observing a real index. Given the FakeRunner/unit-level harness this is the correct proxy: the only thing that mutates the index is a `git add`/`git commit`, and both are proven absent. The "pre-existing user staging" criterion is therefore covered by the same absence-of-mutation argument (the test name asserts the intent; the seeding has no pre-staged fixture, but none is needed because the proof is that nothing index-altering runs). Reasonable for this layer.
  - Tests assert ordering via first-add-index vs first-commit-index, which is exact for the single-add modes here.
  - Not over-tested: each test maps to a distinct acceptance criterion; the two abort tests (-a and -A) differ in seeded source shape (the -A thread adds the untracked enumeration) and are not redundant. No excessive mocking — the real Generator and real Mutator are exercised end-to-end.

CODE QUALITY:
- Project conventions: Followed. Read-only git stays on the plain Runner; every mutation (stage, commit, push) goes through the Mutator seam — matching the golang-design-patterns/error-handling guidance and the engine.ReleaseDeps.Mutator precedent. Errors wrapped with %w; sentinels (errGateAborted) carry clean no-op semantics with no spurious failure narration.
- SOLID principles: Good. stageForMode (staging) and createCommit (commit) are single-responsibility; commitAccept composes them as the one shared accept tail, so the gate-accept and editor save-as-accept paths cannot drift on ordering or the never-unwind contract.
- Complexity: Low. stageForMode is a 3-arm switch; commitAccept is a linear stage→commit→push sequence with early-return error handling.
- Modern idioms: Yes. errors.Is sentinel routing, %w wrapping, raw-bytes stdin for retry-safe re-piping (createCommit run.go:751-766).
- Readability: Good. Doc comments are thorough and tie each step back to the mutate-nothing-until-accept invariant.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
