TASK: A failed answers write or commit unwinds and keeps nothing (token-aware-diff-sizing-8-4, tick-fb0179)

ACCEPTANCE CRITERIA:
- Given an attended release where the operator answers every pause and `.mint.toml` cannot be written, the run fails with the write error verbatim beneath its ✗ line, makes no notes call and no tag, and exits 1 (§6.6, §15.1)
- Given staging or committing `.mint.toml` fails, the run fails with git's own stderr verbatim beneath its ✗ line, not only the runner's exit-code text, makes no notes call and no tag, and exits 1 (§6.6)
- Given a pre_tag hook that made the pre-tag artifact commit, and committing the answers fails, the unwind resets to the starting commit, taking the artifact commit with it. No commit carries the answers subject, `.mint.toml` reads as it did before the answers were written with no change to it staged, and the unwind's narration names no answers commit as kept (§6.6)
- Given a release with no pre_tag hook, where `.mint.toml` was written and staged but committing it fails, `.mint.toml` is afterwards byte-identical to before the run, with no change to it staged or unstaged (§6.6)
- Given no `.mint.toml` before the run, and committing the answers fails, no `.mint.toml` exists afterwards (§6.6)
- Given any of these failures, the next release's check lists the answered paths again, since nothing was recorded (§6.6)

STATUS: complete

SPEC CONTEXT: §6.6 records the exclusion check's answers in `.mint.toml`'s `[release]` table after the last pause and commits the file through the mutator with the fixed subject `chore(release): release-notes path answers for {tag}`. The "answers write or commit failing" bullet, plus the 2026-10-19 corrigenda, make a failed write/stage/commit a pre-push failure: it is surfaced with the write error or git's own stderr verbatim beneath the ✗ line (not the runner's exit-code text), routed through the surgical unwind back to the starting commit (taking any pre-tag artifact commit with it), and `.mint.toml` is left as it was before the write, with nothing kept or re-committed. §15.1 puts the recording before the first notes attempt, so the failure stops the run before any notes call. The 2026-10-21 corrigendum carves out a `.mint.toml` git ignores (written, not committed); that path is not this task's.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:1090-1119 — `answersCommit.record`: snapshots the file with `beforeEdit` before the write, returns `*answersRecordingError` (step "recording the exclusion answers", output = the write error text) on a failed snapshot or write, sets `a.edit` only once the write has landed, and returns step "committing the exclusion answers" with `gitOutput(err)` (git's stderr from `*record.GitError`) on a failed check-ignore probe or `record.CommitFile`. `a.edit` is cleared and the recording narrated only on success.
  - internal/engine/release.go:550-560 — the spine folds `answers.edit` into `made.answersEdit` and routes any `*answersRecordingError` through `surfaceAndUnwind(ctx, deps, "notes", start, made, err)`, as the task's Do item asks.
  - internal/engine/release.go:2001-2007 (`failureOutput`) and 2042-2055 (`failureMessage`) — the ✗ line carries the step; the tool's own message goes in `StageFailure.Output`.
  - internal/notes/exclusioncheck.go:191-195 and internal/notes/select.go:214-217 — the record error ends the pauses and returns from `SelectBodyWithReuse` before the reuse hook and `GenerateFromDiff`, so no notes call is made.
  - internal/engine/unwind.go:127 and 212-274 — `revertEditIfMade` unstages the path to the starting commit (`git -C root reset -q {start} -- .mint.toml`) and restores the pre-write bytes and mode, or removes a file the write created. Each step runs whatever the other's outcome. The revert runs before `resetCommitsIfMade`, so a run with an artifact commit is then `reset --hard` to the start.
  - internal/record/commit.go:107-115 — `CommitFile` carries git's stderr on both the add and commit failures as `*GitError`.
  - internal/fsutil/atomicwrite.go — the write is temp-plus-rename and removes its temp file on failure, so a failed write leaves the file untouched and needs no revert. That is why `a.edit` is set only after the write.
- Notes: The unwind's narration stays correct. `keptAnswers` is nil on this path, so `surgicalSummary` never adds the "kept the answers commit" phrase. A run with no commit emits no Unwound, as built. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage (internal/engine/release_answersfailure_test.go):
  - AC1 — `TestRelease_AnswersWriteFailing_ShowsTheWriteErrorBeneathItsLineAndStopsBeforeTheNotes` (line 125): uses a read-only root and checks ✗ message, Output == the write error, exit 1, no notes call, no tag/push, and the file unchanged.
  - AC2 — `TestRelease_AnswersStageOrCommitFailing_ShowsGitsOwnStderrBeneathItsLineAndStopsBeforeTheNotes` (line 147): table over stage and commit failure, asserting Output == git's stderr exactly.
  - AC3 — `TestRelease_AnswersCommitFailingAfterTheArtifactCommit_UnwindsToTheStartAndKeepsNoAnswersCommit` (line 174): checks `reset --hard {start}`, a single (failed) answers commit attempt, the unstage ordered before the reset, the bytes restored, and the summary "reset 1 commit; repo clean" with no kept phrase.
  - AC4 — `TestRelease_AnswersStagedButTheirCommitFailing_LeavesMintTomlAsBeforeTheRun` (line 199): checks the bytes are identical, the unstage comes after the failed commit, and no reset runs.
  - AC5 — `TestRelease_AnswersFailingWithNoConfigFile_LeavesNoMintToml` (line 217): stage and commit failure, file absent, unstage issued.
  - AC6 — `TestRelease_AfterTheAnswersFailToRecord_TheNextReleaseListsTheAnsweredPathsAgain` (line 237): all three failures, then a second release in the same root lists the answered path again with the same check list.
  - Three further tests (lines 269, 297, 326) pin the unstage-failure warning, root quoting, and the content restore that still runs. Each covers a distinct contract, so none is redundant.
- Notes: Each test would fail if its behaviour broke: dropping `a.edit`, routing through `surface` instead of the unwind, or using the runner error instead of stderr would each trip an assertion.

CODE QUALITY:
- Project conventions: Followed. Mutations go through `git.Mutator`, output through the presenter, git stderr travels verbatim in `StageFailure.Output`/`Warning.Output`, and the error is matched with `errors.As` and wrapped so `Unwrap` holds.
- SOLID principles: Good. The recording, revert and failure-rendering responsibilities sit in separate small functions.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments on `answersCommit`, `MadeState.answersEdit`, `Unwind` and `revertEditIfMade` match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
