TASK: Commit-subject lists leave out the answers commit (token-aware-diff-sizing-8-7, tick-2c0f33)

ACCEPTANCE CRITERIA:
- Attended release of v1.3.0 over `feat: add export` and `fix: handle empty input` that recorded answers, notes call failing under `on_notes_failure = "fallback"`: the shipped body lists both subjects and not `chore(release): release-notes path answers for v1.3.0` (§6.6, §14.1)
- Same release under `on_notes_failure = abort`: `c` at the floor gives the same list (§6.6, §12.2)
- Earlier attempt at v1.3.0 aborted after answering and kept its answers commit; re-run `mint release -M --no-ai` cutting v2.0.0 lists the range's other subjects without the answers subject (§6.6, §14.1)
- A range holding two answers commits (one kept by an aborted attempt, one from the releasing run): neither is listed (§6.6)
- Released v1.3.0 whose range holds `feat: add export`, the pre-tag artifact commit, the answers commit and its bookkeeping commit: `c` at a later `mint release regenerate v1.3.0` floor gives `feat: add export` and the pre-tag artifact subject, without the answers subject (§6.6, §12.3, §14.1)
- A list with nothing left once answers commits (and, on regenerate, the bookkeeping commit) are out gives the maintenance-release body (§14.1)

STATUS: complete

SPEC CONTEXT: §6.6 makes the answers commit (`chore(release): release-notes path answers for {tag}`, fixed prefix, never commit_prefix) part of the release range — on the run that made it, on a later run's range when an abort kept it, and on regenerate of that version. §14.1 makes the commit-subject list the one non-AI body for `--no-ai`, `on_notes_failure = "fallback"` and the floor's `c`; it leaves out subjects with the answers prefix, on regenerate also the version's bookkeeping commit (the commit the tag sits on when its subject ends ` Release {tag}`), keeps the pre-tag artifact commit, and floors an empty list to the maintenance-release body. §5.3 keeps `.mint.toml` out of every notes diff, so the subject is the only place the commit could surface.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/record/commit.go:213-225 — `answersSubjectPrefix`, `AnswersSubject(tag)`, `IsAnswersSubject(subject)` (prefix match, any tag); the subject builder moved here from engine so producer and filter share one constant
  - internal/engine/release.go:1132, :1145 — the answers commit and the unwind's manual-step text now build the subject through `record.AnswersSubject`
  - internal/notes/resolve.go:99-115 — `subjectBody` drops answers subjects (`withoutAnswersCommits`, line-wise `strings.SplitAfter` + `IsAnswersSubject`) before the empty-list floor to `StubBody()`
  - internal/notes/resolve.go:65-71 (`resolveFallbackBody`, used by `NoAIBody` at internal/notes/noai.go:25 and by `ResolveFailure` at resolve.go:59) and :78-84 (`VersionNoAIBody`) both route through `subjectBody`
- Notes: Every commit-subject route funnels through `subjectBody`: `--no-ai` (internal/notes/select.go:202), fallback (select.go:266 → `ResolveFailure`), the release floor's `c` (internal/engine/notesfloor.go:189 → `NoAIBody`), the regenerate floor's `c` (notesfloor.go:198 → `VersionNoAIBody`). `git log --format=%s` is issued from exactly one place (resolve.go:22), so no route escapes the filter. Ordering in `VersionNoAIBody` is correct: the bookkeeping-commit check runs on git's first subject before answers commits are stripped, so a tag sitting on an answers commit (no bookkeeping commit made) cannot promote the subject beneath it into the bookkeeping slot — pinned by `TestVersionNoAIBody_TagOnTheAnswersCommitLeavesTheSubjectBeneathListed`. Prefix-only matching agrees with the spec's "subjects starting ..." wording; a `revert: chore(release): release-notes path answers for ...` subject stays listed. The `notes → record` import predates this task (resolve.go already used `record.BookkeepingSubject`), so no new dependency edge.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/answerssubject_test.go — both forward routes (`NoAIBody`, `ResolveFailure` under fallback) table-driven over: answers commit at the tip, answers commits for two different tags, an unterminated last line, and the prefix not at the start of the subject (kept, with the pre-tag artifact subject kept); only-answers-commits → `StubBody()`; `VersionNoAIBody` dropping bookkeeping + answers while keeping the pre-tag artifact subject (AC5 shape with v1.3.0); a tag on the answers commit; bookkeeping + answers only → `StubBody()` (AC6 regenerate).
  - internal/engine/release_answerssubjects_test.go — behaviour-level through `engine.Release` with a recorded Exclude answer and a failing notes call, attended (`StdinInteractive = true`), asserting the tag annotation body exactly: fallback and floor `c` × (this run's answers commit; a kept answers commit plus this run's) covers AC1, AC2, AC4; floor `c` with only an answers commit → maintenance body (AC6); `--no-ai` with `BumpMajor` from v1.2.3 → v2.0.0 with a kept answers commit, with and without other subjects (AC3, AC6); `RegenerateRun` floor `c` asserting the provider release body (artifact subject + feat kept; bookkeeping + answers only → maintenance body) (AC5, AC6).
  - Each test would fail if `withoutAnswersCommits` were removed or bypassed: the exact-body assertions would carry the answers subject.
- Notes: The engine tests overlap the unit table in places, but they prove the wiring per route through the real spine, as the project's test idioms ask; not bloated. The regenerate engine case uses `regenFloorTag` (v1.2.0) rather than v1.3.0; the v1.3.0 shape is covered at the unit level, and the tag value is immaterial to the behaviour.

CODE QUALITY:
- Project conventions: Followed — external test packages, `t.Parallel()`, table-driven, FakeRunner/RecordingPresenter, exact-body assertions; no new subprocess or output surface
- SOLID principles: Good — one filter applied at the one shared body builder; subject construction and recognition co-located in `record`
- Complexity: Low
- Modern idioms: Yes
- Readability: Good — comments updated in the same commit (resolve.go:37-39, :97-98; noai.go:17; notesfloor.go:194-195) and hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
