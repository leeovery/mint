## Attempt 1

ISSUES:
- `/Users/leeovery/Code/mint/internal/notes/resolve.go:27` together with `:90-94` and `:97-106`. `rangeSubjects` now drops answers lines before `VersionNoAIBody` hands the text to `withoutBookkeepingCommit`. That function's comment says "git log lists a range's tip first, so the first subject is the commit the tag sits on", and this task made that false.
  - When it happens: in a tag-only release (changelog = false, no version_file) that recorded answers, no bookkeeping commit is made, so the tag sits on the answers commit (stage 4, `release.go:550`; the bookkeeping commit is made only when `BookkeepingWillCommit`, `:654`).
  - What goes wrong: on a later `regenerate`, the answers line is removed first. The bookkeeping check then tests the commit beneath it for the ` Release {tag}` suffix, and drops that commit if it matches. §14.1 says "a tag on any other commit leaves the list whole".
  - Reproduced in a scratch copy: log `chore(release): release-notes path answers for v1.3.0\ndocs: notes ahead of Release v1.3.0\nfeat: add export\n` through `VersionNoAIBody(…, "v1.3.0")` gives `"feat: add export\n"`. The spec gives `"docs: notes ahead of Release v1.3.0\nfeat: add export\n"`. The trigger is narrow, but a user's commit vanishes from the regenerated notes with no sign.
  FIX: Run the bookkeeping check on git's raw output, before the answers filter.
  - `rangeSubjects` goes back to returning git's stdout verbatim (restore its original one-line comment).
  - Move `withoutAnswersCommits` into the shared tail both lists already end in. For example, a `subjectBody(subjects string) string` that returns `floorEmptySubjects(withoutAnswersCommits(subjects))`, or fold it into `floorEmptySubjects` and rename it.
  - `resolveFallbackBody` returns `subjectBody(log)`, and `VersionNoAIBody` returns `subjectBody(withoutBookkeepingCommit(log, tag))`. Every list still passes through one filter, and the bookkeeping check sees the tag's own commit again.
  - Add a case to `/Users/leeovery/Code/mint/internal/notes/answerssubject_test.go` next to `TestVersionNoAIBody_LeavesOutTheAnswersCommitAndKeepsThePreTagArtifactCommit`: the log above, want `"docs: notes ahead of Release v1.3.0\nfeat: add export\n"`.
  - Wherever the answers filter's comment lands, don't carry over "never a change to the project" from the current `rangeSubjects` comment (`resolve.go:16`). The answers commit does change the project's `.mint.toml`. §5.3's wording is accurate: "mint's own tooling commits, about how releases are made rather than what a release holds."
  ALTERNATIVE: Keep the shared tail as is. Have `VersionNoAIBody` read the raw log through a new unfiltered helper, apply `withoutBookkeepingCommit`, then `withoutAnswersCommits`, while `resolveFallbackBody` calls `withoutAnswersCommits` directly. This is equally correct but calls the answers filter in two places instead of one. The shared tail is the better choice: "every list ends in the same tail" already holds for the empty-list floor.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/resolve.go:49 — a log holding only answers commits is not empty but now gets the maintenance-release body, so "empty log" is false
  OLD: //     selector: the commit-subject list since the last tag, else (empty log) the
  NEW: //     selector: the commit-subject list since the last tag, else (no subject left) the
- /Users/leeovery/Code/mint/internal/notes/noai.go:17 — a range with only answers commits has commits since the last tag yet gets the maintenance-release body
  OLD: //   - else (no commits since the last tag) the maintenance-release body, so the tag
  NEW: //   - else (no subject left once mint's answers commits are out) the maintenance-release body, so the tag

NOTES:
- The two comment corrections are on lines the diff did not touch. This task's behaviour change made them false, and the project CLAUDE.md requires comments to change in the same commit as the behaviour, so I report them.
- The wording fix for the `rangeSubjects` comment (`resolve.go:15-16`) is folded into the ISSUE's FIX rather than listed as a correction, because that fix moves the filter and rewrites the comment anyway.
- `TestRegenerateRun_NotesFloor_CommitSubjectsLeaveOutTheAnswersCommit` (`/Users/leeovery/Code/mint/internal/engine/release_answerssubjects_test.go:137`) uses the fixture's `regenFloorTag` (v1.2.0) instead of the v1.3.0 the criterion names. The tag value does not affect the result.
- The fallback route with only answers commits is pinned at notes level (`TestCommitSubjectList_OnlyAnswersCommitsGiveTheMaintenanceBody`), not at engine level. The engine routes through the same builder, so this is sufficient.
- `pretagArtifactSubject` stays in the engine (`release.go:1591`) while `AnswersSubject` moved to `record`. Nothing outside the engine needs the artifact subject, so this split causes no failure.
- Only one `git log --format=%s` call exists in production code (`resolve.go:23`), so no list bypasses the filter.
- The orchestrator's "Addition" block is task content marked with its origin, and I reviewed against it. Nothing else beyond the enumerated inputs came with the dispatch.

## Attempt 2

ISSUES:
- `/Users/leeovery/Code/mint/README.md:109` and `/Users/leeovery/Code/mint/README.md:163` still describe the commit-subject lists as they were before this task.
  - :109 gives `c`'s list as "`git log --format=%s {lastTag}..HEAD`, the maintenance-release body when it is empty". A `--no-ai` re-run over a range holding only a kept answers commit has a non-empty `git log`, yet ships the maintenance-release body. The README predicts a list showing `chore(release): release-notes path answers for v1.3.0`.
  - :163 lists exactly what regenerate's list leaves out (only the bookkeeping commit) and says "a range with no other commit gives the maintenance-release body". A range holding a bookkeeping commit and an answers commit now gives the maintenance body, which the README contradicts.
  - What goes wrong: an operator, or an agent configuring mint from the README, expects mint's answers commit in the `c`/`--no-ai`/fallback body and is surprised when it is missing. No test catches the drift.
  - Precedent: task 5-10 updated this same `README.md:163` sentence when it changed the same list. §18 makes the README update part of the behaviour change. No README test pins either phrase (checked `internal/config/*_test.go` and `internal/setupguide/*_test.go`), so the fix is text only.
  FIX: Edit both parentheticals.
  - :109: replace "(`git log --format=%s {lastTag}..HEAD`, the maintenance-release body when it is empty)" with "(`git log --format=%s {lastTag}..HEAD`, leaving out mint's own answers commits, `chore(release): release-notes path answers for {tag}`; the maintenance-release body when no subject is left)".
  - :163: replace "under whatever `commit_prefix` it was released with, so the list is the one its release would have shipped;" with "under whatever `commit_prefix` it was released with, and mint's own answers commits, so the list is the one its release would have shipped;". The trailing "a range with no other commit gives the maintenance-release body" then reads correctly: "other" means other than the left-out commits.
  ALTERNATIVE: Also add one sentence to the answers-commit paragraph at `README.md:117`: "The commit-subject list never lists it." It is optional and helps a reader who starts from the exclusion check. It does not replace the two edits above, which are where the list is defined.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/notesfloor.go:195 — this task falsified it: the list now also leaves out answers commits, so "the range less its bookkeeping commit" is wrong whenever the range holds one. The new wording defers to the builder so it outlives the next change to the list.
  OLD: // commit subjects those of the version's own range less its bookkeeping commit.
  NEW: // commit subjects those VersionNoAIBody lists over the version's own range.

NOTES:
- The notesfloor.go comment is on a line the diff did not touch. I report it because this task's behaviour change made it false, and the project CLAUDE.md requires such comments to change in the same commit as the behaviour.
- `TestRegenerateRun_NotesFloor_CommitSubjectsLeaveOutTheAnswersCommit` uses the fixture's `regenFloorTag` (v1.2.0) rather than the v1.3.0 that AC5 names. The tag value does not change the result, and the notes-level test uses v1.3.0.
- `/Users/leeovery/Code/mint/internal/notes/noai.go:17` is now wrapped longer than its neighbours. That is cosmetic, lint passes, and no correction is needed.
- While inspecting uncommitted files I read `.workflows/token-aware-diff-sizing/implementation/token-aware-diff-sizing/fix-tracking-token-aware-diff-sizing-8-7.md` (the round-1 findings) from the working tree. It was not a dispatch input, and I reached these findings independently against the code and spec. The orchestrator's "Addition" block is task content marked with its origin, and I reviewed against it. The `floorEmptySubjects` it names is now `subjectBody` (`resolve.go:100`). Nothing else rode the dispatch beyond the enumerated inputs.
