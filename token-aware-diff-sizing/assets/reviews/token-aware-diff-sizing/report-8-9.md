TASK: Write the answers without committing them when git ignores `.mint.toml` (token-aware-diff-sizing-8-9)

ACCEPTANCE CRITERIA:
1. An attended release whose `.mint.toml` is untracked and matched by a `.gitignore` rule, existing or absent under a broad rule such as `.*`: after the last pause the answers sit in `.mint.toml`'s `[release]` table as §6.6 writes them (an absent file is created holding only that table), no `git add` of `.mint.toml` runs, no commit carries the answers subject, and the release goes on through notes, review gate, tag and push.
2. On that route the recording line, after the last pause and before the notes line resumes, says the answers were written to `.mint.toml`'s `[release]` table and not committed because git ignores the file; plain mode prints it as its own line; neither mode says they were committed.
3. A tracked `.mint.toml` (ignore-matched or not), and an untracked or absent one no rule matches, take today's route: staged by path, committed as `chore(release): release-notes path answers for {tag}`, and the line says written and committed.
4. On the ignored route, an abort or failure before the push (`n` at the review gate, a later stage such as the bookkeeping commit failing) leaves `.mint.toml` holding the answers: no unstage of `.mint.toml`, no restore of earlier content, no removal of a created file; other commits and tag undone as built; where the unwind narrates, its summary names no answers commit as kept.
5. The next attended release on that clone reads the answers from `.mint.toml`, so the check raises no pause about the answered paths.
6. On the ignored route a failed write (e.g. unwritable `.mint.toml`) fails as today: ✗ `recording the exclusion answers` with the write error verbatim beneath it, aborting before any tag, unwinding as built.
7. The README's recorded-answers account says that when git ignores an untracked `.mint.toml`, the answers are written but not committed, the release goes on, and the write survives an abort or failure before the push.

STATUS: complete

SPEC CONTEXT: §6.6 "Recording the answers", with the 2026-10-21 corrigendum adding the "A `.mint.toml` git ignores" bullet: untracked + ignore-matched means write but never stage/commit (no `git add -f`, per the `--autostash` rule that mint never commits work the operator did not choose to commit), narrate "not committed because git ignores the file", carry on, and keep the write through any later abort as an answers commit is kept. A tracked `.mint.toml` is committed whatever the ignore rules say. The write/stage/commit-failure bullet (failure surfaced verbatim, unwound, `.mint.toml` returned to its prior state) still governs a failed write. §16.1 places the recording line after the last pause and before the notes line resumes. §18.2 has the README name the exception.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/record/commit.go:117-130 — `IsIgnored`: `git -C {dir} check-ignore -q -- {path}` through the Mutator's read pass-through `m.Run` (never `Mutate`); exit 0 -> ignored, exit 1 -> not ignored, anything else -> `*GitError` carrying git's stderr.
  - internal/engine/release.go:1090-1119 — `answersCommit.record`: snapshot (`beforeEdit`), write, set `a.edit`, probe at `a.root`; when not ignored, `a.commit` (stage by path + commit) and `a.landed = true`; on both successful routes `a.edit = nil`, so `made.answersEdit` (release.go:557) is nil and `revertEditIfMade` (unwind.go:242-249) is a no-op for the ignored write. A probe failure returns `answersRecordingError` with `a.edit` still set, so the unwind takes the write back, consistent with the write/stage/commit-failure bullet.
  - internal/engine/release.go:1121-1125 — `narrateAnswers` emits `ExclusionAnswersRecorded` with `Ignored` carried in the payload, between SuspendSpinner/ResumeSpinner.
  - internal/presenter/presenter.go:577-592 — `ExclusionAnswers.Ignored`; `exclusionAnswersText` yields "written to .mint.toml's [release] table and not committed, as git ignores .mint.toml" vs "... and committed". Pretty (pretty.go:615-618) and plain (plain.go:232-235) render it unchanged; the presenter derives nothing.
  - internal/engine/unwind.go:80-82, 92-93, 110-111 and the file header (lines 11-14) — comments updated to the as-built exception.
  - README.md:117 — ignored-file sentences added (AC7).
- Notes: The probe runs after the write rather than before it. That is still before staging, as the task requires, and `check-ignore` answers the same for the path whether or not it exists, so the order changes nothing. On the ignored route nothing in the later unwind touches the file: `git reset --hard`, and the KeepOut route's `git restore --source --staged --worktree`, leave untracked paths alone, and autostash uses `--include-untracked` (autostash.go:55), which leaves ignored files out. The surgical summary only names a kept answers commit when `made.keptAnswers` is set, and on this route `landed` is false, so it never is. A probe failure is labelled `committing the exclusion answers`. The spec does not name that case, and the label is defensible, since the probe is the precondition of staging.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/engine/release_answersignored_test.go:85-120 — existing file and absent file under a broad rule. Each asserts the probe ran at the root, there was no add/reset of `.mint.toml` and no answers-subject commit, the exact bytes written (created file holds only `[release]`), only the bookkeeping commit, the push, the AI notes in the tag body, and the excluded path absent from notes calls. Probe placement is checked at :122-143.
  - AC2: release_answersignored_test.go:145-174 (payload `Ignored: true`, one line after the pause, framed by Suspend/Resume, notes stage completing after it); internal/presenter/exclusionanswers_test.go:86-107 (exact pretty and plain lines, plain ASCII-pure).
  - AC3: the not-ignored route is exercised by every pre-existing answers-commit sequence, now carrying `configNotIgnored` (release_answerscommit_test.go:32-45, release_answersfailure_test.go:41-42, release_answerscommit_test.go:181); the committed-route payload is pinned at release_answerscommit_test.go:138; exit-1 -> not ignored is pinned in internal/record/isignored_test.go:12-42. I checked every answering test file (exclusionrecord, exclusionpause, exclusionanswer, answerskept, answersunwindfailure, answersautostash, notes_reporoot). Each scripts the probe through `withAnswersCommit`/`answersCommitCalls`, so none slides onto the ignored route by accident.
  - AC4: release_answersignored_test.go:176-240. It covers a decline after the artifact commit, the bookkeeping commit failing, and a decline with a write-created file. Each asserts no unstage, bytes intact, `reset --hard` to the start where commits exist, summary "reset 1 commit; repo clean" (no kept answers), and no Unwound when nothing was made.
  - AC5: release_answersignored_test.go:242-264 — the next release's notes git calls carry `:(exclude)docs/design/`, read from the ignored file on disk.
  - AC6: release_answersignored_test.go:266-290 — read-only root, ✗ label, write error verbatim as Output, file unchanged, no answers line, aborted before notes/tag/push.
  - Probe failure: release_answersignored_test.go:292-329 (git stderr verbatim, write undone for existing and absent files) and isignored_test.go:44-61.
- Notes: The AC6 test seeds `configIgnored`, but the write fails before the probe, so that seed is never consumed. The test therefore runs the same path as the pre-existing `TestRelease_AnswersWriteFailing_ShowsTheWriteErrorBeneathItsLineAndStopsBeforeTheNotes`. That is harmless: the behaviour does not depend on the ignore answer, and the seed would come into play if the probe were ever moved ahead of the write.

CODE QUALITY:
- Project conventions: Followed. The probe goes through the Mutator's read pass-through (seam 2: reads stay off `Mutate`) with `-C root` (seam 1). Output is payload-driven and the presenter derives nothing (seam 3). Failure output carries git stderr verbatim via `*GitError`/`gitOutput`. Tests are external packages using FakeRunner/RecordingPresenter with exact argv, table-driven, and `t.Parallel()`. CLAUDE.md's "Surgical unwind, then PONR" invariant has been updated to name the uncommitted write.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The new and changed comments match the code as built.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "A tracked `.mint.toml`, one a `.gitignore` rule matches included, and an untracked or absent `.mint.toml` that no rule matches, take today's route" — the engine side is settled by reading and by the fakes (exit 1 -> commit route). The tracked-but-ignore-matched clause rests on git's documented behaviour: `check-ignore` consults the index and never reports a tracked path. No test observes this, because tests never spawn real git. To settle it, run `git -C {root} check-ignore -q -- .mint.toml` in a real repo where `.mint.toml` is tracked and matched by a `.gitignore` rule and confirm exit 1; then confirm exit 0 for an untracked `.mint.toml` under a `.*` rule.
