TASK: Record nothing unless the last pause is answered (token-aware-diff-sizing-8-3, tick-c2ce9c)

ACCEPTANCE CRITERIA:
- Given an attended release of `v1.3.0` with three surviving suggestions, the first answered Exclude and the second Keep, a Ctrl-C at the third pause ends the run with no notes call and no tag. `.mint.toml` is byte-identical to before the run, and no commit carries the subject `chore(release): release-notes path answers for v1.3.0` (§6.6)
- Given the same release with a pre-tag artifact commit, and stdin closing at the third pause, the run aborts with no tag and unwinds to the starting commit, taking the artifact commit with it. `.mint.toml` is unchanged, no answers commit is made, and none of the three globs is in `[release].diff_exclude` or `[release].diff_keep` (§6.6)
- Given `mint release -y` with surviving suggestions, at a terminal or not, `.mint.toml` is unchanged after the run and no answers commit is made (§6.7)
- Given `mint release --dry-run` at a terminal without `-y`, or `mint release --dry-run -y`, with surviving suggestions, `.mint.toml` is unchanged and no commit is made (§6.7)
- Given an attended release where the AI answers "none", or the check fails, `.mint.toml` is unchanged and no answers commit is made (§6.4, §6.6, §6.8)

STATUS: complete

SPEC CONTEXT: §6.6 holds the exclusion answers until the last pause and writes/commits them together; a Ctrl-C during the pauses cancels and records nothing. The 2026-10-19 corrigendum on §6.6 makes stdin closing at a pause an abort before any tag that unwinds as any pre-push failure and records nothing, never read as Keep. §6.7: unattended (`-y`, non-TTY) and dry runs ask nothing and write nothing. §6.4/§6.8: an answer of "none" or a failed check (generation failure, timeout, missing binary, unreadable answer, list that won't fit) carries on with no suggestions.

IMPLEMENTATION:
- Status: Implemented (the behaviour was built by earlier tasks 7-7, 7-10, 7-11, 8-1, 8-2; this task's commit 1b9150c adds only the proving tests)
- Location:
  - internal/notes/exclusioncheck.go:176-196 — `pause` checks `ctx.Err()` and the pause's own error after each Pause and returns before `decided.record`; `c.record` is called only after the loop finishes and only when an answer exists (`decided.answered()`), so a cancel or closed input at any pause, after earlier answers included, records nothing.
  - internal/notes/exclusioncheck.go:134-150 — a cancel during the check call is returned; a failed check, "none", or no usable suggestion returns before any pause or record.
  - internal/engine/release.go:1180-1200 — `exclusionPauses.Pause` returns the Prompt error unchanged (the `notes.AnswerKeep` it pairs with it is discarded by `pause`), so closed input is never read as Keep.
  - internal/engine/release.go:1063-1073 — `WithRecord` is wired only on the `asksAtTerminal` branch (attended and not a dry run, internal/engine/notesfloor.go:18-26); the unattended and attended-dry-run branches wire report/preview only, which never record.
  - internal/engine/release.go:550-575 — a closed-input or cancel error from resolveBody is not an `answersRecordingError` and `reachesNotesFloor` rejects it (cancel explicitly; `ErrInputClosed` is unknown to `notes.ClassifyFailure`), so it goes to `Unwind`, which resets the pre-tag artifact commit; `answers.landed` is false so no answers commit is counted or kept.
  - internal/presenter/pretty.go:949-952 — Ctrl-C/Ctrl-D at the raw-mode key read return `ErrInputClosed`, matching the test's stated model of how a Ctrl-C reaches the engine.
- Notes: None

TESTS:
- Status: Adequate
- Coverage: internal/engine/release_answersheld_test.go
  - TestRelease_CtrlCAtTheThirdPauseAfterAnsweringTwo_RecordsNothing (:82) — v1.3.0 via minorOptions, Exclude then Keep, third pause cancels; asserts the cancel error, exactly three pauses, no notes call, no tag/push (assertNoMutation), .mint.toml byte-identical, no `.mint.toml` staging, no answers commit for v1.3.0, no answers narration.
  - TestRelease_StdinClosedAtTheThirdPauseAfterAnsweringTwo_UnwindsTheArtifactCommitAndRecordsNothing (:114) — pre_tag artifact commit, closed input at the third pause; asserts the error, three pauses, no notes call (guards against closed input read as Keep), `reset --hard startsha`, summary "reset 1 commit; repo clean" (which would differ if an answers commit had been made or kept), no tag, nothing recorded, and none of the three globs in either `[release]` list after a strict `config.Load`.
  - TestRelease_UnderYes_WithSurvivingSuggestions_RecordsNothing (:156) — `-y` with stdin a terminal and not; guards against vacuity by asserting both suggestions were printed.
  - TestRelease_DryRun_WithSurvivingSuggestions_WritesNothingAndCommitsNothing (:178) — attended dry run and `--dry-run -y`; asserts no pause, nothing recorded, and no `git commit` of any kind.
  - TestRelease_Attended_CheckAnsweringNoneOrFailing_RecordsNothing (:212) — "none", an unreadable answer, generation failure, timeout, missing binary; asserts no pause and nothing recorded on a run that ships.
  - Each test would fail if recording moved ahead of the last pause, if closed input were read as Keep, or if record were wired on the unattended or dry-run branches. The 8-3 tests extend the 7-7 single-pause abort tests (internal/engine/release_exclusionpause_test.go:272, :298) to the after-earlier-answers case and to the recording assertions, so they are not duplicates.
- Notes: None

CODE QUALITY:
- Project conventions: Followed — external test package, t.Parallel throughout, t.TempDir roots, FakeRunner/RecordingPresenter doubles, exact-argv commit-subject assertions, behaviour-level proofs.
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes (slices.Contains, t.Context, table-driven subtests)
- Readability: Good — the helper comments (answeredConfig, pausesThen, assertNothingRecorded, the Ctrl-C comment at :93) hold true against the code they describe.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
