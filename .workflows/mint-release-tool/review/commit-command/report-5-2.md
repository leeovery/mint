TASK: 5-2 — Push after a successful gate-accept commit (tick-c5c87c)

ACCEPTANCE CRITERIA:
- With -p armed, after a successful gate-accept commit a plain git push runs via the consumed git_safe
- The git push carries no upstream/remote/branch arguments and no -u/--set-upstream — git resolves current branch -> configured upstream
- The push runs strictly after the commit succeeds (ordered after the git_safe commit)
- With -p unarmed, no push runs after the commit
- If the commit did not complete (no commit produced), no push runs
- The push fires after a -y auto-accept commit exactly as after an interactive accept
- The push is a single shared step (reusable by 5-3), not a gate-path-only implementation
- No failure-warn / never-unwind / empty-suppression behaviour is implemented here (deferred to 5-4/5-5)

STATUS: Complete

SPEC CONTEXT:
Auto-push Behaviour + Commit Flow / Lifecycle: push is opt-in via -p/--push, flag-only with no config default. Step 6 of the lifecycle runs push only after a successful commit. mint runs a plain `git push` (current branch -> configured upstream) and adds NO special upstream logic — it defers all upstream handling to git. Push must be a single shared step both accept paths (gate-accept and editor save-as-accept) reach. Reversibility: a commit is local and reversible; push is a best-effort final step (failure-warn is 5-4). The push flows through git_safe like every other commit mutation.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/run.go:844-861 — pushAfterCommit: the single shared auto-push step. `if !deps.Push { return nil }` gates on the armed flag; otherwise `deps.Mutator.Mutate(ctx, nil, "git", "push")` — argv exactly ["push"], nil stdin, through the git_safe Mutator.
  - internal/commit/run.go:797 — commitAccept calls pushAfterCommit AFTER createCommit returns nil, so push is strictly ordered after the commit; sequencing makes "push only after a successful commit" free.
  - internal/commit/run.go:367 — gate-accept (Run) routes to commitAccept; internal/commit/run.go:445 — editor save-as-accept (runEditorFallback) routes to the SAME commitAccept. One shared tail, no parallel push implementation (satisfies the 5-3 reusability requirement).
  - internal/commit/run.go:227 — Deps.Push field; cmd/mint/commit_flags.go:67-78 — -p/--push wired to the same `push` var and threaded as Push (flag-only, no config read).
- Notes: Plain `git push` confirmed — no -u/--set-upstream, no remote/branch args, no current-branch detection or upstream inference. No pre-push / remote-sync gate (the dropped gates stay dropped). All three commit mutations (stage/commit/push) route through the git_safe Mutator, never the raw runner. The -y auto-accept reaches commitAccept through the same reviewLoop path, so the push fires identically.

TESTS:
- Status: Adequate
- Coverage (internal/commit/run_push_test.go covers all 7 plan micro-acceptance tests, 1:1):
  - TestRun_PushArmed_PushesAfterGateAcceptCommit — armed push runs via the Mutator-wrapped runner after a gate-accept commit.
  - TestRun_PushArmed_PlainPushNoUpstreamArgs — argv exactly ["push"], empty stdin (no upstream/remote/branch args, no -u).
  - TestRun_PushArmed_RunsStrictlyAfterCommit — commit index < push index.
  - TestRun_PushUnarmed_NoPushAfterCommit — zero pushes when Push=false, commit still ran.
  - TestRun_PushArmedNoCommit_GateNoAbort_NoPush — `n` abort produces no commit, no push.
  - TestRun_PushArmedNoCommit_GenerateFailure_NoPush — generate failure aborts pre-commit, no push (failure side of the same criterion).
  - TestRun_PushArmed_PushesAfterDashYAutoAcceptCommit — push fires after a -y auto-accept commit, strictly after the commit.
  - TestRun_PushViaGitSafe_NotRawRunner — a seeded first-attempt lock contention is retried (2 push attempts) proving the push flows through git_safe, not the raw runner.
  - Additionally internal/commit/run_editor_push_test.go proves the editor save-as-accept path reuses the SAME single shared push step (single push, plain argv, strict stage->commit->push order, unarmed = no push) — covering the 5-3-reuse acceptance criterion ahead of 5-3.
- Notes: Tests assert behaviour (recorded invocations, argv, order, stdin) not implementation internals. Ordering assertions via recorded git-invocation indices are precise and would fail if the push moved before the commit or ran on a no-commit path. The git_safe-vs-raw-runner distinction is proven by a lock-retry rather than a brittle type assertion — a strong, behaviour-level test. Not over-tested: each test pins a distinct criterion; the gate-path and editor-path plain-push/ordering tests look parallel but legitimately guard two separate call sites. No redundant assertions, no excessive mocking (FakeRunner + RecordingPresenter only).

CODE QUALITY:
- Project conventions: Followed. Sentinel errors via errors.New (errPushFailed), lowercase error strings, errors.Is matching, %w wrapping, t.Parallel() on every test, single-purpose named test functions. git mutations consistently routed through the Mutator seam.
- SOLID principles: Good. pushAfterCommit is a single-responsibility routine; commitAccept owns the ordered stage->commit->push tail in one place so the invariant cannot drift between the two accept paths (the spec's load-bearing single-shared-step requirement).
- Complexity: Low. pushAfterCommit is a guard + one Mutate call + failure branch; the happy path is two statements.
- Modern idioms: Yes.
- Readability: Good. Intent is self-evident; doc comments are thorough (arguably verbose, but accurate).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] (out of 5-2 scope — belongs to 5-4) internal/commit/run.go:849-858 — pushAfterCommit's failure branch warns and returns errPushFailed, which is a plain error. cmd/mint exitCode matches only *engine.AbortError (per the run.go:112 comment), so a failed push exits the generic 1 with the commit kept. This is correct per spec, but is 5-4 behaviour present in the file under the 5-2 task; flagging only so the reviewer is aware the failure path is already landed (no action needed for 5-2 — verify under 5-4's task). Not a defect in the 5-2 happy path.
