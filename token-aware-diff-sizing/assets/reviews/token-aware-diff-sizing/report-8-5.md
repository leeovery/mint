TASK: An abort after answering keeps the answers commit (token-aware-diff-sizing-8-5)

ACCEPTANCE CRITERIA:
- Attended v1.3.0 release, pre_tag artifact commit, Exclude answer on docs/design/, decline at the review gate: branch ends at the starting commit with one commit on top (`chore(release): release-notes path answers for v1.3.0`) carrying the answer in .mint.toml; artifact commit gone; no v1.3.0 tag; the unwind's narration names the answers commit as kept and reports a clean repo; exit 1 (§6.6)
- Same release, a failed `r`, a launched editor failing at the review gate's `e`, or stdin closing at the review gate's keypress: same state, same narration (§6.6, §15.1)
- Same release, notes failing under on_notes_failure = abort, then `a` at the floor, a launched editor failing at the floor's `e`, or stdin closing at the floor's choice: same state, same narration (§6.6, §12.2)
- Same release, bookkeeping commit fails, or the push is rejected after the tag: same state, same narration; after the rejected push the bookkeeping commit is gone and the tag deleted (§6.6)
- Ctrl-C while the notes call runs after the last pause: same state, same narration (§6.6, §11.5)
- No pre_tag hook, decline at the review gate after answering: starting commit with the answers commit on top, narration names it kept (§6.6)
- After an abort that kept the answers commit, the next `mint release` neither lists nor pauses on the paths those answers cover (§6.6)
- README "One atomic point of no return" feature and "Surgical unwind before the point of no return" bullet, `mint setup`'s point-of-no-return line, and CLAUDE.md's "Surgical unwind, then PONR" invariant each name the answers commit as the exception, re-committed on the starting commit (§18.1, §18.2, §18.3)

STATUS: complete

SPEC CONTEXT: §6.6 makes the exclusion answers the one deliberate exception to the surgical unwind: an answer is about the repo, not the release, so after any abort or failure before the push the unwind resets to the starting commit as built (taking the pre-tag artifact commit, bookkeeping commit and tag) and then re-commits the answers on the starting commit, narrating the commit as kept so it does not read as a failed unwind. The §12.2 and §15.1 corrigenda put every floor and review-gate abort (decline, failed `r`, failing editor, closed stdin) on the unwind; §9.1/§11.5 make a Ctrl-C a cancel that never reaches on_notes_failure. §18.1-18.3 require the setup guide, README and CLAUDE.md to name the exception, and code comments stating the unwind's contract to stay true. Task 8-6 owns the reset/re-commit failure paths; 8-8 autostash; 8-9 the git-ignored .mint.toml route.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/unwind.go:114-135 (Unwind: reset via resetCommitsIfMade, then recommitAnswersIfKept; Unwound carries unwindOutcome{answersKept, clean})
  - internal/engine/unwind.go:141-150 (recommitAnswersIfKept: re-commits only when made.keptAnswers is set and the reset succeeded)
  - internal/engine/unwind.go:353-368 (surgicalSummary: "; kept the answers commit on the starting commit" inserted before the "; repo clean" tail)
  - internal/engine/unwind.go:83-85 (MadeState.keptAnswers)
  - internal/engine/release.go:550-556 (answers commit folded into made.Commits, its position recorded, made.keptAnswers set whenever the commit landed, before any error routing)
  - internal/engine/release.go:1138-1149 (answersCommit.recommit: AppendReleaseGlobs on the reset .mint.toml, then record.CommitFile with the fixed subject)
  - Routes reaching Unwind after the answers land: release.go:560, 574, 578 (notes stage / floor, including cancel since reachesNotesFloor returns false for context.Canceled at notesfloor.go:33), 613 (every review-gate abort), 632/644/656 (record), 700/711 (publisher/gh), 751/767 (cancel before push, tag/push with made.TagCreated from release.ErrPushRejected). No return between release.go:551 and :768 bypasses the unwind except the dry-run exit, which records no answers.
  - Docs: README.md:30 and README.md:353; internal/setupguide/setupguide.go:265-270; CLAUDE.md:42
  - Contract comments updated: internal/engine/unwind.go:3-30 header and :88-113 Unwind doc; internal/engine/release.go:15-29 AUTO-UNWIND paragraph; internal/engine/autostash.go:9-14 header
- Notes: The re-commit stages .mint.toml alone on an index the hard reset just matched to the starting commit, so `git commit -m` carries nothing else. The KeepOut soft-reset path (which keeps a staged notes file) cannot coincide with kept answers, because KeepOut comes only from --notes-file (release.go:365) and the exclusion check is skipped under --notes-file (§6.1). Unwind detaches from cancellation before the re-commit, so the Ctrl-C route re-commits too.

TESTS:
- Status: Adequate
- Coverage: internal/engine/release_answerskept_test.go covers each criterion through Release with a shared assertion (assertAnswersKept, :116-158): exit 1; a reset to the starting sha followed by exactly `git -C root add .mint.toml` and `git -C root commit -m <answers subject>`; .mint.toml holding the starting config plus [release].diff_exclude = ["docs/design/"]; tags made equal tags deleted; one Unwound with the exact summary. Cases: decline with artifact commit (:160), review-gate failed r / failing editor / closed stdin (:174), floor a / failing editor / closed stdin under on_notes_failure = "abort" with one floor gate asserted (:234), bookkeeping-commit failure and rejected push with "reset 3 commits and deleted tag v1.3.0; kept ..." (:278), Ctrl-C during the notes call (:321), no pre_tag hook ("reset 1 commit; kept ...") (:341), and the next release carrying `:(exclude)docs/design/` with no pause (:352). startResettingRunner (:26-39) plays the reset's effect on .mint.toml, so the content assertion would fail if the re-commit did not re-apply the answer, and the next-release test would fail if the answers were not kept.
- Notes: The doc criterion has no test, and none is required; reading confirms it. Tests are table-driven and focused, with no redundant cases.

CODE QUALITY:
- Project conventions: Followed (mutations through git.Mutator, narration through the presenter as an engine-authored verbatim Unwound summary, external test package, FakeRunner + RecordingPresenter, exact argv assertions)
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
