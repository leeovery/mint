TASK: A failed run leaves the notes file as it was (token-aware-diff-sizing-3-4)

ACCEPTANCE CRITERIA:
1. `mint release --notes-file NEXT_RELEASE.md` on a tracked file with uncommitted edits, a pre_tag hook whose artifact commit lands, and an abort at the review gate: the unwind resets the artifact commit, and the file keeps the same edits, staged if they were staged and unstaged if not (§13.2)
2. That file, and a failure before the push after the bookkeeping commit (for example a rejected atomic push): the unwind resets mint's commits and deletes the tag mint created, as built. The file keeps the same edits and the same staged state (§13.2)
3. That file, and a Ctrl-C after mint has made a commit: the unwind runs as built, and the file keeps the same edits and the same staged state (§13.2)
4. That file, `--autostash` with other uncommitted changes, and a failure before the push: the other changes come back after the unwind, as built. The notes file keeps the same edits and the same staged state (§13.2)
5. `mint release regenerate <version> --notes-file NEXT_RELEASE.md --target changelog` on that file, where the push of the changelog commit fails: regenerate resets the changelog commit, as built, and the file keeps the same edits and the same staged state (§13.2)
6. An untracked notes file inside the repo is left untouched, contents included, after either reset (§13.2)
7. In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it: the changelog write, any version-file projection and the hook's artifacts are gone (§13.2)

STATUS: complete

SPEC CONTEXT: §13.2 says the notes file is outside everything mint does: it is never staged or committed, and afterwards it sits in the work tree untouched. The 2026-10-19 corrigenda on §13.2 extend this to a tracked file with uncommitted edits. They also say that a reset during the forward surgical unwind, or during regenerate's post-commit reset, leaves the file's content and its staged state as they were. The ground is CLAUDE.md's "Surgical unwind, then PONR": mint removes only what it created, and the operator's index state is not mint's. §6.1 skips the exclusion check under `--notes-file`, so the answers-commit exception to the unwind never applies here.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/unwind.go:64-66: `StartState.KeepOut` carries the notes file's exclusion into the unwind.
  - internal/engine/unwind.go:197-210: `resetCommitsIfMade` resets through `resetTo`, using `start.KeepOut`. It still issues no reset when `made.Commits <= 0`, as built.
  - internal/engine/unwind.go:276-311: `resetTarget` and `resetTo`.
    - With no path to keep, `resetTo` runs the as-built `git reset --hard {sha}`.
    - With a path to keep, it runs `git reset --soft {sha}` and then `git restore --source={sha} --staged --worktree -- ':(top,literal,exclude)P'`. Both go through `git.Mutator`, and each failure has its own manual step.
  - internal/engine/release.go:503: the forward spine records `KeepOut: keepOut` on `StartState`. `keepOut` comes from `notesExclusion` (release.go:922-935). That value is empty for a notes file outside the repo or an ignored untracked file. In both cases the plain `reset --hard` still leaves the file alone.
  - internal/engine/regenerate_write.go:304, 313-329, 373-402: regenerate builds `resetTarget{HEAD, KeepOut: req.KeepOut}` and threads it through `pushChangelogCommit` and `resetAndAbort`. These still detach from cancellation (`context.WithoutCancel`, line 397).
  - internal/engine/regenerate_interactive.go:197-204, 263: regenerate computes `keepOut` for a changelog-writing target and passes it into `RegenerateWriteRequest.KeepOut`.
  - internal/engine/regenerate_batch_changelog.go:150, 155: the batch path passes `resetTarget{HEAD: startingHEAD}` with no `KeepOut`. That is correct, because `--notes-file` is refused with `--all`.
- Notes:
  - The two-step reset matches `git reset --hard` under git's documented semantics everywhere except the kept path.
    - `reset --soft` moves HEAD only.
    - `restore` is no-overlay by default, so it removes tracked paths the source lacks (committed hook artifacts and a newly added CHANGELOG.md) from both the index and the work tree.
    - The excluded path's index entry and work-tree file are not touched.
  - A staged-new notes file (in the index but not in HEAD) is kept too. `notesExclusion` lists only `--others` as ignored, so `KeepOut` is set for that file, and plain `reset --hard` would have deleted it.
  - The autostash pop stays deferred (release.go:450-453), so it runs after the unwind.
  - No drift from the task or the spec.

TESTS:
- Status: Adequate
- Coverage:
  - Each scenario test runs over three cases: an untracked notes.md, a tracked NEXT_RELEASE.md with unstaged edits, and a tracked NEXT_RELEASE.md with staged edits.
    - Review-gate abort after the artifact commit: internal/engine/notesfile_reset_test.go:53
    - Rejected atomic push, with tag delete and reset: :99
    - Pre-cancelled context reaching the last pre-PONR gate after the bookkeeping commit: :131
    - `--autostash`, with the stash pop asserted after the reset's restore: :163
    - Regenerate `--target changelog` push failure through `RegenerateRun`: :200
  - `assertResetKeptNotesFile` (:43-51) pins the exact `reset --soft` and `restore` argv, asserts that no `reset --hard` ran, and asserts that no git step names the notes file outside its exclusion pathspec. The test fails if the reset reverts to `--hard`, drops `--staged`, or drops the exclusion.
  - The Unwound summaries are pinned per scenario.
  - Unit coverage:
    - kept-path reset around a tag delete: :231
    - no commits, so no git command at all: :250
    - a failed soft reset or restore warns with the remaining manual steps, in both `Unwind` and regenerate: :266, :313
    - the shell quoting of the printed restore step, and a round-trip through `cmdline.Split`: :343, :400, :428
  - `resetIssued` (regenerate_write_test.go:75) was widened to count `reset` or `restore`. Its two callers (regenerate_write_test.go:290, regenerate_notesfile_test.go:404) both use it negatively, so the widening only makes those assertions stricter.
- Notes:
  - With FakeRunner, staged versus unstaged notes differ only in the staged-edit probe that feeds the commit (task 3-2's route). The reset assertions are the same for both, so whether the staged state really survives is a property of git, not something these tests observe. That is consistent with the project's test idioms (no real git, exact argv) and is recorded under UNSETTLED.
  - `assertFileUnchanged` passes trivially under the fake, because the fake never touches files. The `no reset --hard` assertion is the one that would catch a regression.

CODE QUALITY:
- Project conventions: Followed. Every mutation goes through `git.Mutator` (seam 2). Recovery is detached from cancellation as before. Warnings carry git's stderr verbatim in `Warning.Output`. External test package, table-driven, `t.Parallel()`, exact argv assertions.
- SOLID principles: Good. One `resetTo` serves both the forward unwind and regenerate's recovery, so the two cannot drift.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments at unwind.go:64-66, 94-97, 283-290 and regenerate_write.go:16-20, 382-384 match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it: the changelog write, any version-file projection and the hook's artifacts are gone (§13.2)". To settle this, run the two-step reset in a real git repo. Set up a starting commit, then mint commits that modify CHANGELOG.md, add a new hook artifact and change the version file, with the notes file excluded. Run `git reset --soft S` followed by `git restore --source=S --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'`, once from the repo root and once from a subdirectory. Confirm that HEAD == S, that the artifact is gone from both the index and the work tree, and that CHANGELOG.md and the version file match S. The subdirectory run checks that git expands the exclude-only pathspec to the whole tree.
- "the file carries the same edits it had, staged if they were staged and unstaged if not (§13.2)" (criteria 1-3 and 5). In the same real-repo scenario, record NEXT_RELEASE.md's index blob (`git ls-files -s`) and its work-tree content before mint's commits and after the two-step reset, for both a staged edit and an unstaged edit. Confirm both are unchanged. Repeat with an untracked notes file (criterion 6).
- "Given that file, `--autostash` with other uncommitted changes, and a failure before the push, the other changes are restored after the unwind as built. The notes file carries the same edits, with the same staged state (§13.2)". Check with a real `git stash push --include-untracked -- ':(top,literal,exclude)NEXT_RELEASE.md'`, then mint's commits, the two-step reset, and `git stash pop`, with NEXT_RELEASE.md's edit staged. Confirm the pop succeeds with a staged entry in the index, that the other changes come back, and that NEXT_RELEASE.md's index blob and content are unchanged afterwards.
