## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/engine/unwind.go:148-158 (`answersRestoreStep`), used at unwind.go:189-193 and via release.go:1125-1127. The step tells the operator to run `git cherry-pick ORIG_HEAD[~k]`. Git rewrites ORIG_HEAD on every reset without paths, even one that does not move HEAD, and also on `git stash`, merge, pull, rebase and am. I reproduced two failures in a scratch repo:
  (a) Running the printed `git reset --hard {start}` a second time sets ORIG_HEAD to the starting commit, so `git cherry-pick ORIG_HEAD~1` picks the starting commit's parent and conflicts on an unrelated file.
  (b) Running `git stash` before the cherry-pick (re-commit write-failure case) sets ORIG_HEAD to HEAD, so `git cherry-pick ORIG_HEAD` picks the starting commit itself and stops with "The previous cherry-pick is now empty".
  In both cases the operator recovering a failed unwind is given a step that no longer restores the answers. They see a confusing cherry-pick error, can only recover by digging through the reflog, and the same questions come back on the next release, which is what §6.6's exception exists to prevent. This also departs from the unwind's own stated recovery convention, `StartState.HEAD` (unwind.go:49-51): "resets to THIS exact sha, not a relative HEAD~N, so the result is provably the exact starting state".
  FIX: Name the answers commit by its sha. In `Unwind`, when `made.keptAnswers != nil` and before any mutation (HEAD is still the tip mint built), read `git rev-parse HEAD~{Commits-position}` (just `HEAD` when that count is 0) through `deps.Runner` or `deps.Mutator.Run`. Store the sha, and have `answersRestoreStep` return `git cherry-pick {sha}`. If the read fails, fall back to the current `ORIG_HEAD~k` form, so that read never blocks the unwind. In the tests:
  - 8-5's `unwound()` helper and this task's three tests each seed one extra leading `ScriptedOut(sha)`.
  - The expected step becomes `git cherry-pick {sha}`.
  - The "tip" and "one below the tip" subtests stay and now pin the `HEAD~k` argv of the read.
  ALTERNATIVE: (1) Resolve the sha only on the failure paths. After a failed hard reset, read `HEAD~k` (git aborts a failed `reset --hard` before moving HEAD). After a successful reset whose re-commit failed, read `ORIG_HEAD~k`, which nothing has touched since mint's own reset. A clean unwind then makes no extra read and 8-5's tests are untouched, but correctness rests on how git orders reset's internal steps. (2) Keep `ORIG_HEAD~k` and accept the fragility: no git call and no test churn, but the step breaks under the operator actions above. I recommend the start-of-unwind capture because it is correct by construction.
  CONFIDENCE: medium

NOTES:
- `recommit(ctx, m, restore)` (release.go:1125) takes `restore` only to hand it back unchanged on the write-failure branch. It works, but the caller could pick the step itself from which part failed. No change requested.
- The executor's extra case, the re-commit failing at the write, is within §6.6's "re-commit failing". It carries the write error, consistent with the 2026-10-19 corrigendum ("the write error, or git's own stderr"), and is tested.
- The `--notes-file` keep-out route of `resetTo` (soft reset then restore) cannot run together with kept answers: a supplied body returns before the exclusion check (`notes/select.go:184`). So the cherry-pick cannot meet the notes file's staged keep-out state.
- `.tick/tasks.jsonl` is modified in the working tree. It is orchestrator bookkeeping and was not reviewed.
- The task content includes an "Addition (from the orchestrator)" marked with its origin. I reviewed it as task content. Nothing else arrived beyond the enumerated inputs.
