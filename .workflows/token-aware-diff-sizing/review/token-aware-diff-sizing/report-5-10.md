TASK: A version's commit subjects match its forward release's (token-aware-diff-sizing-5-10)

ACCEPTANCE CRITERIA:
- Given `v1.2.0` whose range `v1.1.0..v1.2.0` holds `feat: add export`, `fix: handle empty input` and the bookkeeping commit `🌿 Release v1.2.0` that the tag sits on, `c` at regenerate's floor gives `feat: add export` and `fix: handle empty input`, without the bookkeeping subject (§14.1)
- Given that version was released under an earlier `commit_prefix`, so its tag sits on `🚀 Release v1.2.0` while the repo's `commit_prefix` is now `🌿`, the bookkeeping subject is still left out (§14.1)
- Given a version whose release made no bookkeeping commit, so its tag sits on `fix: handle empty input`, every subject in the range is listed, that one included (§14.1)
- Given a range holding `chore(release): pre-tag artifacts for v1.2.0` before the bookkeeping commit, the artifact commit's subject stays listed (§14.1)
- Given a range whose only commit is its bookkeeping commit, the body is the maintenance-release body (§14.1)

STATUS: complete

SPEC CONTEXT: §14.1 makes the commit-subject list (`git log --format=%s` over the release's range) the one non-AI body. On regenerate the range is the version's own `{PreviousTag}..{Tag}`, which holds the version's bookkeeping commit (made after the forward notes, so never in the forward list). The 2026-10-19 corrigenda fix recognition: the commit the tag sits on, when its subject ends in ` Release {tag}` under any prefix; a tag on any other commit leaves the list whole; the pre-tag artifact commit stays; an empty list floors to the maintenance-release body. Regenerate has no `--no-ai` and does not read `on_notes_failure`, so the floor's `c` is the only regenerate route that ships the list.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/resolve.go:78 `VersionNoAIBody(ctx, r, revRange, tag)` lists subjects over the version's range and passes them through `withoutBookkeepingCommit` and then `subjectBody` (answers-commit filter plus the StubBody floor)
  - internal/notes/resolve.go:89 `withoutBookkeepingCommit` drops only the first subject (the range tip, i.e. the commit the tag peels to) when it ends in `record.BookkeepingSubject("", tag)` = " Release {tag}"; the suffix is derived from the same builder the forward release uses, so recognition follows the subject format and ignores whatever prefix preceded it
  - internal/notes/resolve.go:65 forward `resolveFallbackBody` (`{lastTag}..HEAD`) is left unchanged, as it must be
  - internal/engine/notesfloor.go:196-198 `regenerateNotesFloor` builds `c` from `notes.VersionNoAIBody(ctx, deps.Runner, req.DiffRange, req.Tag)`
  - cmd/mint/main.go:247 wires `DiffRange` from `version.Resolution.DiffRange()` (`{PreviousTag}..{Tag}`), and `req.Tag` is the canonical tag the bookkeeping subject was built from
- Notes: Logic is sound against every criterion. First-line-only matching is correct: for `A..B`, git's walk emits B before any of its ancestors, so the first subject is always the tagged commit; an empty range yields "" and floors to StubBody. An empty `commit_prefix` still matches (subject " Release {tag}", whose leading space git keeps in `%s`). README.md:165 documents the behaviour accurately. No drift from the spec or the plan.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/noai_test.go:195 table covers all five criteria exactly (current prefix, earlier `🚀` prefix, no bookkeeping commit, pre-tag artifact kept, bookkeeping-only range gives StubBody), plus two discriminating guards: another version's release subject on the tip is kept (tag specificity), and a release subject below the tip is kept (first-line-only). Each case also pins the exact `git log --format=%s v1.1.0..v1.2.0` argv
  - internal/notes/noai_test.go:261 pins the git-failure error text naming the range
  - internal/engine/regenerate_floor_test.go:168 proves the behaviour end to end at the floor's `c`: the bookkeeping line is seeded on the tip and the provider body written is the list without it
  - internal/notes/answerssubject_test.go:92/105/118 cover the interaction with the answers-commit filter (bookkeeping plus answers, tag on the answers commit, bookkeeping plus answers only gives StubBody)
- Notes: Every test would fail if the exclusion broke (the body would carry the `Release` line or lose a legitimate subject). Not over-tested: each extra case guards a distinct failure mode.

CODE QUALITY:
- Project conventions: Followed (subprocess through runner.CommandRunner, errors wrapped with %w, external test packages, t.Parallel, table-driven, exact argv asserted)
- SOLID principles: Good. Recognition sits in one small pure helper, and the subject format comes from the record package's single builder
- Complexity: Low
- Modern idioms: Yes (strings.Cut)
- Readability: Good
- Issues: None. Comments on the changed code (resolve.go:64, :73-77, :86-88; notesfloor.go:194-195; regenerate_interactive.go:110-111) hold true against the code and cite no process artifacts

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
