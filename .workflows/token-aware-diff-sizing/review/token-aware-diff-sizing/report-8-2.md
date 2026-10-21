TASK: Commit the answers after the last pause (token-aware-diff-sizing-8-2, tick-075b92)

ACCEPTANCE CRITERIA:
- Given an attended release of `v1.3.0` whose pre_tag hook changed `dist/bundle.js`, an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the branch carries the pre-tag artifact commit, then one commit with the subject `chore(release): release-notes path answers for v1.3.0`, then the release bookkeeping commit. The answers commit's only change is `.mint.toml`, carrying both answers, and `git push --atomic origin HEAD v1.3.0` sends all three (§6.6)
- Given `commit_prefix = "🚀"`, the answers commit's subject is still `chore(release): release-notes path answers for v1.3.0`, while the bookkeeping commit reads `🚀 Release v1.3.0` (§6.6)
- Given that release in pretty mode, once the answers commit lands, one line says the answers were written to `.mint.toml`'s `[release]` table and committed, before the notes line resumes. In plain mode it prints as its own line. A run that records no answers shows no such line (§6.6, §16.1, §16.2, Corrigendum 2026-10-19)
- Given a release whose only changes are under `docs/design/`, and an Exclude answer on `docs/design/`, the answers commit is made and the release's body is the maintenance-release body, with no notes call (§6.6, §7.1)
- Given a released `v1.3.0` that recorded an Exclude answer on `docs/design/` and a Keep answer on `test/fixtures/`, the next release, whose range changes `docs/design/next.md`, `test/fixtures/case.json` and `src/export.go`, lists neither recorded path in its check call and pauses on neither. Its notes diff carries `test/fixtures/case.json` and no `docs/design/` change. The same holds when that release runs from a fresh clone of the pushed branch (§5.2, §6.5, §6.6)
- The README's `release` section says the pauses' answers are recorded in `.mint.toml`'s `[release]` table and committed with the release (§6.6, §18.2)

STATUS: issues_found

SPEC CONTEXT: §6.6 holds the answers until the last pause, writes them into `.mint.toml`'s `[release]` table, then stages `.mint.toml` by path and commits it through the mutator with the fixed subject `chore(release): release-notes path answers for {tag}` (constant prefix, never `commit_prefix`), on the release branch, going out with the atomic push. The 2026-10-19 corrigendum adds a presenter event narrating the recording before the notes line resumes (plain: its own line). §7.2/§15.1: answers are recorded before the first notes attempt and their exclusions shape every later step; an all-excluded diff gets the maintenance-release body. §5.3 keeps `.mint.toml` out of every notes diff, so the answers commit never reaches the notes. §18.3 requires the commit-graph comments (`MadeState.Commits`, the COMMIT-GRAPH INVARIANT) to change with the graph. Later corrigenda (2026-10-21) add the git-ignored `.mint.toml` route (task 8-9), which this task's code now carries alongside.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/release.go:548-561 — `answersCommit` wired into Stage 4; `made.Commits++` and `keptAnswers` folded in when the commit landed, before the error check, so a later stage failure still counts it
  - internal/engine/release.go:1063-1073 — the record hook is wired only on an attended real run (`asksAtTerminal`), so unattended and dry runs never write or commit
  - internal/engine/release.go:1090-1119 — write, ignore probe, `record.CommitFile`, `landed = true`, then the narration
  - internal/engine/release.go:1121-1125 — `narrateAnswers` wraps the event in `SuspendSpinner`/`ResumeSpinner`
  - internal/engine/release.go:718-735 — COMMIT-GRAPH INVARIANT updated to up to three commits, in order
  - internal/engine/unwind.go:73-76 — `MadeState.Commits` comment updated to 0 to 3
  - internal/record/commit.go:104-115 — `CommitFile`: `git -C {dir} add {path}` then `git -C {dir} commit -m {subject}` through the Mutator
  - internal/record/commit.go:213-220 — `AnswersSubject` with the constant prefix
  - internal/notes/exclusioncheck.go:191-195 — record runs once, after the last pause, only when something was answered
  - internal/notes/select.go:214-220 — the check runs before the reuse hook and the notes call; an all-excluded re-assembled diff returns `StubBody`/`KindDegenerate`
  - internal/presenter/presenter.go:107-110, 577-592; plain.go:232-235; pretty.go:615-618; presentertest/recording.go:202-205 — the new `ExclusionAnswersRecorded` event, rendered by both presenters and recorded by the test double
  - README.md:117 — the `release` section describes the recording and the commit
- Notes: The commit takes the whole index (`git commit -m`), but nothing else can be staged at that point: preflight confirmed a clean tree (autostash stashes WIP first), the pre_tag hook's dirt is committed by `CommitDirtyTree`'s `add -A`, and the only `keepOut` path comes from `--notes-file`, which never runs the check (§6.1). The file always changes when written, because a paused glob must match an uncovered path, so no empty commit is possible. With no bookkeeping commit, the tag lands on the answers commit by construction, as the updated invariant comment says. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - Criterion 1: internal/engine/release_answerscommit_test.go:63-108 asserts the exact commit-subject order (artifact, answers, bookkeeping), that the call before the answers commit is `git -C root add .mint.toml` alone, that the atomic push follows the bookkeeping commit, and that the written config holds both answers.
  - Criterion 2: :110-124 asserts `[answers subject, "🚀 Release v1.2.4"]` under `commit_prefix = "🚀"`. The tag differs from the criterion's v1.3.0, which does not matter because the subject is parameterised by tag.
  - Criterion 3:
    - :126-155 checks that exactly one answers event follows the last of two pauses, with the full payload, wrapped in Suspend/Resume, and that the notes stage completes after it.
    - :157-174 checks that a run with no pause has no line and runs no git call naming `.mint.toml`.
    - :176-190 checks that a failed commit has no line.
    - internal/presenter/exclusionanswers_test.go:31-65 pins the exact pretty output and spinner stop/restart, and the exact plain line with byte-purity.
  - Criterion 4: :192-213 checks for one answers commit, no notes call, and a tag annotation equal to the maintenance body.
  - Criterion 5: :215-261 runs "this clone" and "a fresh clone" subtests and checks:
    - the notes git calls carry `:(exclude)docs/design/`
    - the check list holds only `src/export.go`
    - there are no pauses
    - the notes call carries `test/fixtures/case.json`
  - Criterion 6: internal/config/readme_exclusion_check_test.go:84-99 pins the README sentence inside the `### \`release\`` section.
  - `CommitFile` unit tests: internal/record/commitfile_test.go cover the argv, a stage failure making no commit, and git stderr on each step.
- Notes: Every test would fail if its behaviour broke. The subjects are hard-coded in the tests rather than taken from `record.AnswersSubject`, so subject drift is caught. No redundant or over-mocked tests.

CODE QUALITY:
- Project conventions: Followed. The mutation goes through `git.Mutator`, output through a payload-driven presenter event, and gates through `Prompt`. Plain narration is ASCII. The constant subject prefix follows `pretagArtifactSubject`.
- SOLID principles: Good. `answersCommit` owns write, commit and recommit state. `record.CommitFile` is a small reusable primitive. Narration is separated.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: One stale test-file header comment on the commit graph (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/release_commitgraph_test.go:11 — the file header still describes "the up-to-two-commit graph" and says the tag lands on "the bookkeeping commit when one exists, else the hook-artifact commit, else the pre-existing HEAD". Since this task, the graph is up to three commits, and without a bookkeeping commit the tag sits on the answers commit when one was made. Reword lines 11-15 so the file says it pins the hook-artifact/bookkeeping combinations of a first release (where no exclusion check runs, so no answers commit is made), and state the tag rule as "the last commit mint made this run, else the pre-existing HEAD", matching internal/engine/release.go:729-730. Comment-only remedy, so not blocking. — FAILS: the comment states a commit-graph and tag-placement rule the shipped code now falsifies. That breaks CLAUDE.md's "Keep them TRUE TO AS-BUILT" rule and §18.3. A maintainer reading this file as the graph's pin would believe the three-commit order and the tag-on-answers-commit case are pinned here, when they are pinned only in release_answerscommit_test.go.

UNSETTLED:
- None
