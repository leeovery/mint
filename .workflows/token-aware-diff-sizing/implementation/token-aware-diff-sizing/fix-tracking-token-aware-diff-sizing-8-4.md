## Attempt 1

ISSUES:
- internal/engine/unwind.go:192-195. When the unstage fails, `revertEditIfMade` returns before it restores `.mint.toml`'s content (or removes a file the write created), and the warning names only the unstage as the manual step. An operator who follows the warning still has mint's answers write in the work tree:
  - With no commit made (criterion 4's shape), nothing else cleans it up, and no Unwound line runs.
  - With commits, `git reset --hard` cleans a tracked or staged file. It leaves a created `.mint.toml` whose staging failed.
  This breaks "mint removes only what it created" and gives a manual step that does not finish the recovery.
  FIX: Make the two steps independent. Attempt the content restore or remove whether or not the unstage succeeded. Each failed step warns with its own manual step. Return ok only when both succeed. Add a test in release_answersfailure_test.go: no pre_tag hook, commit fails, unstage fails (seed `gitFailing(...)` after `commitFails.git`). Assert that `.mint.toml` reads `answeredConfig` and that the one warning names the unstage step.
  ALTERNATIVE: Keep the early return and have the warning name both manual steps (unstage, then restore or remove). Not recommended: mint would skip a filesystem step it can do itself and push it onto the operator.
  CONFIDENCE: high
- internal/engine/unwind.go:192-193. The unstage's failure warning throws away the `runner.Result` (`_, err := deps.Mutator.Mutate(...)`), and `warnUnwindIncomplete` puts the runner's exit-code text in `Warning.Output`. The test at internal/engine/release_answersfailure_test.go:265-281 seeds git stderr `fatal: Unable to create '.git/index.lock': Permission denied` and asserts `Output: "exit status 1"`, so it pins the dropped stderr. CLAUDE.md "Error & exit idioms" puts git stderr verbatim in `Warning.Output`. Spec §6.6 (corrigendum line 993, and bullet line 316 for the re-commit warning) settles that the feature's new unwind warnings carry git's own stderr, not the exit-code text the as-built sites carry. This is the same blind spot the task exists to remove from the ✗ line: the operator cannot see why the unstage failed.
  FIX: Keep `res` from the unstage Mutate and give the warning `res.Stderr`, falling back to `err.Error()` when stderr is empty. The cleanest shape is to change `warnUnwindIncomplete(p, action, output string)`: the existing tag-delete and reset callers pass `err.Error()` and keep their behaviour (the executor's BANK entry covers moving them to stderr later), and the os.Remove/WriteFile restore failures pass `err.Error()`. Task 8-5's re-commit warning needs the same stderr-carrying path. Update the test's want `Output` to `unstageStderr`.
  ALTERNATIVE: Add a sibling helper that takes an output string and leave `warnUnwindIncomplete` untouched. That is less churn but leaves two near-identical warn builders.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/unwind.go:93-95 — line 94 runs past the block's wrap width (the added sentence was not reflowed).
  OLD: // With zero mutations (no tag, no commits, no write) it is a no-op: it emits NO
// Unwound and issues NO git command. A reverted write alone emits no Unwound either. Either way it returns abort(reason) so the engine owns the
// non-zero exit; it emits NO StageFailed (a stage-failure caller surfaces that first).
  NEW: // With zero mutations (no tag, no commits, no write) it is a no-op: it emits NO
// Unwound and issues NO git command. A reverted write alone emits no Unwound either.
// Either way it returns abort(reason) so the engine owns the non-zero exit; it emits
// NO StageFailed (a stage-failure caller surfaces that first).
- /Users/leeovery/Code/mint/internal/engine/unwind.go:97 — "Both mutations" is stale in the doc block this diff edited: the unwind now also unstages through the Mutator.
  OLD: // Both mutations flow through the lock-resilient Mutator and are best-effort — a "tag
  NEW: // Its git mutations flow through the lock-resilient Mutator and are best-effort — a "tag
- /Users/leeovery/Code/mint/internal/engine/unwind.go:247-251 — the "(tag-delete or reset)" list and "the git command to run" / "the failing git error" are false for the new revert callers (unstage, os.Remove, file restore).
  OLD: // warnUnwindIncomplete emits the manual-cleanup notice when a recovery mutation
// (tag-delete or reset) failed mid-unwind: the repo is NOT clean, so the user must
// finish the recovery by hand. action names the exact manual step (the git command to
// run); the failing git error rides the Warn's Output verbatim, mirroring the engine's
// other captured-output warns (warnPublishFailed et al).
  NEW: // warnUnwindIncomplete emits the manual-cleanup notice when a recovery step failed
// mid-unwind: the repo is NOT clean, so the user must finish the recovery by hand.
// action names the exact manual step; the failing step's error rides the Warn's Output
// verbatim, mirroring the engine's other captured-output warns (warnPublishFailed et al).
- /Users/leeovery/Code/mint/internal/engine/release.go:1392-1394 — "any cause that carries no tool output" implies a cause that does carry output gets it shown. A pre_tag `*hooks.HookError` carries the hook's stderr, yet `failureOutput` returns "" for it.
  OLD: 		// failureOutput is called for EVERY stage — not special-cased to "notes" — because
		// it returns "" for any cause that carries no tool output, leaving the ✗ line
		// standing alone.
  NEW: 		// failureOutput is called for EVERY stage — not special-cased to "notes" — because
		// it returns "" for any cause it reads no tool output from, leaving the ✗ line
		// standing alone.

NOTES:
- The executor's BANK entry holds up. `CommitDirtyTree` (internal/record/commit.go:44), `CommitBookkeeping` (:89) and `CommitStaged` (:146, :159) all drop the Mutate result, and `warnUnwindIncomplete` (unwind.go:256) puts `cause.Error()` in Output. I have not added a duplicate entry.
- The explicit `surfaceAndUnwind` route behaves the same as the notes failure branch for this error: `reachesNotesFloor` is false for a recording error, and `warnUnsaved` has nothing to warn. No test tells the two routes apart, and none needs to. The explicit route matches the Do item.
- No test pins the revert-before-reset order that the inline comment at unwind.go:111-112 states. Swapping the order would pass every test, because no fixture's pre_tag artifact commit touches `.mint.toml`. An index-order assertion (unstage before `reset --hard`) in the criterion-3 test would close it. The criteria do not name the order.
- A reverted write with no commit emits no Unwound line. The operator sees the ✗ line and git's stderr, but nothing says `.mint.toml` was put back. The spec says nothing about this, and it matches the as-built silence of an unwind with no commits.
- If the unstage fails and the following `git reset --hard` succeeds, the summary says "manual cleanup required" even though the reset restored the tracked state. That is conservative, not false. It is true for a created file whose staging failed.
- Gates I re-ran on the changed packages: build, gofmt, vet, `go test -race` for internal/engine and internal/record, and golangci-lint (0 issues). All are clean. All eight new engine tests and the new record test pass.
- Nothing beyond the enumerated inputs came with the dispatch. The "Addition (from the orchestrator)" block is marked task content and was reviewed as such.
