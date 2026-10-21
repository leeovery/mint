TASK: `--autostash` keeps the answers in the committed `.mint.toml` (token-aware-diff-sizing-8-8, tick-c40020)

ACCEPTANCE CRITERIA:
- Given `mint release --autostash` with an uncommitted edit to `.mint.toml` that adds `diff_exclude = ["specs/"]` under `[release]`, which the committed file lacks, and an Exclude answer on `docs/design/`, the answers commit's `[release].diff_exclude` holds `docs/design/` alone: the stashed `specs/` entry is not committed (§6.6)
- Given a stashed `.mint.toml` edit that restores cleanly over the answers, after the run the work tree's `.mint.toml` carries both the edit and the answers, and the stash entry is gone (§6.6)
- Given a stashed `.mint.toml` edit that conflicts with the answers on restore, after the run the stash entry is still in `git stash list`, mint prints the as-built warning `could not restore stashed changes cleanly — your work is preserved in git stash; resolve manually`, and the answers commit still holds the answers (§6.6)
- Given `--autostash`, a stashed `.mint.toml` edit and an abort after answering, the answers commit sits on the starting commit holding only the answers, and the stashed edit is restored over it, or, on a conflict, left in the stash with the same warning (§6.6)

STATUS: complete

SPEC CONTEXT: §6.6 says that under `--autostash` the answers are written to the committed `.mint.toml`. A stashed edit to the same file that conflicts on restore leaves the stash entry intact, as any conflicting restore does. Related §6.6 bullets: an abort after answering keeps the answers commit, and the unwind re-commits it on the starting commit. Writing the answers changes every other byte of the file in no way. The answers go to `[release]` only.

IMPLEMENTATION:
- Status: Implemented. The behaviour comes from the existing ordering, and commit 7cb27b6 adds only the test file `internal/engine/release_answersautostash_test.go`. No source change was needed, and I found none missing.
- Location:
  - internal/engine/release.go:370: `config.Load` is the engine's only config load. It runs at Stage 1, before the stash, and nothing reloads the config later in the run.
  - internal/engine/release.go:450-454: `autostashPush` runs before the clean-tree gate, and `defer autostashPop` restores the stash only when `Release` returns. That is after every inline `surfaceAndUnwind` or `Unwind`.
  - internal/engine/release.go:1090-1119 (`answersCommit.record`) and :1127-1129 (`write`): the answers go through `config.AppendReleaseGlobs(a.root, a.globs)`, which reads the file on disk (internal/config/appendrelease.go:35-49). After the stash, that file is the committed `.mint.toml`, so the answers merge into the committed content and never into the in-memory config, which can still carry the stashed edit.
  - internal/engine/release.go:1138-1149 (`recommit`): after the unwind's `reset --hard` to the starting commit, the answers are written again to the reset file. The re-commit therefore holds only the answers.
  - internal/engine/unwind.go:114-135: the unwind order is tag delete, revert of an uncommitted write, reset, then re-commit. All of it runs inline, before the deferred pop.
  - internal/engine/autostash.go:72-83 and :91-96: the restore is `git stash pop`. A failed pop calls `warnPopConflict` with the exact AC3 message, and mint never runs `git stash drop`.
- Notes: An ignored `.mint.toml` is not stashed, because `--include-untracked` leaves ignored files alone. It keeps its as-built in-place write, as §6.6 allows. A dry run never stashes (release.go:450).

TESTS:
- Status: Adequate
- Coverage:
  - AC1: `TestRelease_Autostash_TheAnswersCommitHoldsTheAnswersAndNoneOfTheStashedEdit` (release_answersautostash_test.go:194-208) captures the file at the moment of the answers commit and decodes it. `[release].diff_exclude` must equal exactly `["docs/design/"]`. The test would fail if the answers were taken from the Stage-1 config, if `.mint.toml` were kept out of the stash, or if the write never happened (an empty file decodes to nil).
  - AC2: `TestRelease_Autostash_ACleanRestoreLeavesTheEditOverTheAnswersAndDropsTheStash` (:210-230) checks that the pop comes after the answers commit, that the work tree decodes with both the answer and the stashed `[commit]` edit, that the stash is empty, and that there are no warnings.
  - AC3: `TestRelease_Autostash_AConflictingRestoreKeepsTheStashAndWarns` (:232-247) checks that the release still succeeds, that the stash entry is kept, that `git stash drop` ran zero times, that the exact warning appears, and that the answers commit holds only the answers.
  - AC4: `TestRelease_Autostash_AbortAfterAnswering_RecommitsOnlyTheAnswersThenRestoresTheEdit` (:249-307) covers a clean restore and a conflicting restore in one table. It asserts the exact argv after the reset (add, commit, then `git stash pop`), that the abort exits 1, that two answers commits were made, and that the re-commit holds only the answers.
  - `stashingRunner` (:24-53) plays git's effects on the file for stash push, reset and pop, and leaves the file and stash untouched on a failed pop. This keeps the assertions about behaviour rather than call counts.
- Notes: The tests are not over-tested: four tests, one per criterion, with shared helpers that are not redundant.

CODE QUALITY:
- Project conventions: Followed. The tests use the external test package, `t.Parallel()` throughout, `t.TempDir()`, FakeRunner and RecordingPresenter, exact argv and an exact `presenter.Warning`.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (`slices.Equal`/`slices.Contains`, `t.Context()`)
- Readability: Good. The helper comments match the code: `stashingRunner`, `newAutostashedRelease`, `released`, `unwound`, and the edit constants that describe an empty committed file.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
