TASK: Fold `RangeNoAIBody` back into the notes package (token-aware-diff-sizing-5-16)

ACCEPTANCE CRITERIA:
- `--no-ai` and `on_notes_failure = "fallback"` still produce the commit-subject list since the last tag from one `git log --format=%s {lastTag}..HEAD`, and the maintenance-release body when it lists none: the `NoAIBody` and `ResolveFailure` tests in internal/notes/noai_test.go pass unchanged.
- The regenerate floor's `c` still lists the version's own range less its bookkeeping commit, and a git failure still reads `building commit-subject fallback for <range>: <cause>`: the `VersionNoAIBody` tests pass unchanged.
- `notes` exports no `RangeNoAIBody`: `rg -n 'RangeNoAIBody' internal cmd` matches nothing.

STATUS: complete

SPEC CONTEXT: Spec §14.1 makes the commit-subject list the one non-AI body: `git log --format=%s` over `{lastTag}..HEAD` on a forward release (`--no-ai`, `on_notes_failure = "fallback"`, the release floor's `c`), and over the version's own range on regenerate, where the version's ` Release {tag}` bookkeeping commit is also left out (corrigenda of 2026-10-19). Answers commits are left out on both routes, and an empty list gives the maintenance-release body. The task removes the exported range builder that keeps the bookkeeping commit, so a version-range caller can't pick it by mistake.

IMPLEMENTATION:
- Status: Implemented
- Location: internal/notes/resolve.go:64-71 (`resolveFallbackBody` now calls `rangeSubjects(ctx, r, lastTag+"..HEAD")` and `subjectBody` itself); internal/notes/resolve.go:78-84 (`VersionNoAIBody`, unchanged); internal/notes/noai.go:25-27 (`NoAIBody`, unchanged); internal/engine/notesfloor.go:198 (the regenerate floor's `c` calls `notes.VersionNoAIBody`); internal/engine/notesfloor.go:189 (the release floor's `c` calls `notes.NoAIBody`)
- Notes: Commit 4e26f96 inlines the old `RangeNoAIBody` body into `resolveFallbackBody` (the fold option the task allowed). The logic is identical: the same `rangeSubjects` call over `lastTag+"..HEAD"`, the same error propagation and the same `subjectBody` floor. A case-sensitive search of all `.go` files finds no `RangeNoAIBody` or `rangeNoAIBody`. `resolveFallbackBody` has two callers, `ResolveFailure` (resolve.go:59) and `NoAIBody` (noai.go:26). The doc comments that name it (resolve.go:37, noai.go:13) still describe it accurately.

TESTS:
- Status: Adequate
- Coverage: The diff only deletes the three `TestRangeNoAIBody_*` tests. Every other test is untouched. The remaining tests cover what the deleted ones covered:
  - argv and verbatim subject list: `TestNoAIBody_ProducesCommitSubjectListBodyNoAICall` (noai_test.go:14).
  - empty or whitespace-only list gives the maintenance-release body, on both routes: `TestNonAIBody_EmptySubjectList_ShipsTheMaintenanceBody` (noai_test.go:144).
  - exact `building commit-subject fallback for <range>: <cause>` error from the shared `rangeSubjects`: `TestVersionNoAIBody_GitFailureNamesTheRange` (noai_test.go:261).
  - git failure surfaces on the `{lastTag}..HEAD` route: `TestNoAIBody_GitFails_SurfacesGenuineError` (noai_test.go:118).
  The `errors` and `runner` imports are still used (noai_test.go:125, :265), so the file still compiles.
- Notes: None

CODE QUALITY:
- Project conventions: Followed
- SOLID principles: Good
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None. The new one-line comment on `resolveFallbackBody` (resolve.go:64) is true: it lists `{lastTag}..HEAD` and keeps any bookkeeping commit, which `VersionNoAIBody` drops. It names no process artifacts.

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "the `NoAIBody` and `ResolveFailure` tests in internal/notes/noai_test.go (:14-193) pass unchanged" / "the `VersionNoAIBody` tests (noai_test.go:242-319) pass unchanged" — Reading settles "unchanged": the commit touches no test except the three dropped ones, and the inlined code is byte-for-byte the logic it replaced. Only a run of `go test -race ./internal/notes/...` can settle "pass".
