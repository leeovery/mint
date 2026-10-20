## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/resolve.go:14-24. Before this diff, `resolveFallbackBody` called `FallbackBody`, so it was the one production builder of the commit-subject list. Now `resolveFallbackBody` goes `RangeNoAIBody` → `rangeSubjects`, and `FallbackBody` is called only by `TestFallbackBody_BuildsCommitSubjectListSinceLastTag` and `TestFallbackBody_GitFails_SurfacesError` (/Users/leeovery/Code/mint/internal/notes/resolve_test.go:30-69). Its doc still calls it "the one non-AI body" that "exists purely to give the non-AI routes" a record, but no route uses it. The two copies hold the same rule (the argv and error wrap of the subject list). Two upcoming changes will edit that list: task 5-10, whose own task text names `notes.FallbackBody` as the forward list's builder, and Phase 8's answers-commit exclusion. An edit made in `FallbackBody` would change nothing that ships while its own tests stay green. Nobody would notice until the wrong subjects show up in released notes.
  FIX: Delete `FallbackBody`. Move its "NEVER fed to the AI" contract paragraph onto `rangeSubjects` (or onto `RangeNoAIBody`'s doc). Delete the two `TestFallbackBody_*` tests; no coverage is lost, because `TestNoAIBody_ProducesCommitSubjectListBodyNoAICall` and `TestResolveFailure_Fallback_*` pin the `{lastTag}..HEAD` argv, and `TestNoAIBody_GitFails_SurfacesGenuineError` and `TestRangeNoAIBody_GitFailureNamesTheRange` pin error surfacing. Reword the helper comments at resolve_test.go:14 and :21 that name `FallbackBody` so they name `git log` or the non-AI body instead.
  ALTERNATIVE: Keep `FallbackBody` as the forward-range lister and route `resolveFallbackBody` through it, with `RangeNoAIBody` taking the listed stdout. This keeps the exported name live but leaves two entry points over one rule, so deletion is the better option.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/resolve.go:71-76 — false: the regenerate floor's `c` is a non-AI route that calls `RangeNoAIBody` directly, not this function. The rest restates `RangeNoAIBody`'s doc.
  OLD: // resolveFallbackBody is the SHARED non-AI body selector every non-AI route calls,
// so they can never drift. It produces a NON-EMPTY body:
//
//   - the commit-subject list since the last tag; a genuine git error surfaces.
//   - if that list is empty or whitespace-only (no commits since the last tag), the
//     maintenance-release body (StubBody) instead, so the tag body is never empty.
  NEW:
- /Users/leeovery/Code/mint/internal/engine/regenerate_fresh.go:26-30 — the rewritten line keeps a task id ("5-12's"), and the paragraph still says the write and push are "later tasks (5-7/5-8)", which they no longer are.
  OLD: // FAILURE ROUTING: an AI failure is SURFACED (wrapped, errors.Is still matches), not
// swallowed — a single-version run answers it at the notes floor's choice when attended
// and aborts otherwise, and 5-12's `--all` intercepts it for skip-and-continue. This
// function owns the fresh-source body PRODUCTION only; the provider/changelog write and
// push are later tasks (5-7/5-8).
  NEW: // FAILURE ROUTING: an AI failure is SURFACED (wrapped, errors.Is still matches), not
// swallowed — a single-version run answers it at the notes floor's choice when attended
// and aborts otherwise, and `--all` skips the version and carries on. This function
// owns the fresh-source body PRODUCTION only, not the provider/changelog write and push.

NOTES:
- The task 5-10 text's "As built" bullet says "`notes.FallbackBody` ... builds the forward list". After this diff that is false: the forward list goes through `rangeSubjects`/`RangeNoAIBody`. Whoever runs 5-10 should be pointed at `RangeNoAIBody`.
- README /Users/leeovery/Code/mint/README.md:153 spells `c` out as `git log --format=%s {previous tag}..{tag}`. That is true today, but task 5-10 drops the bookkeeping commit from that list, so the sentence needs updating in 5-10.
- `failure.Recoverable = offersChoice` in /Users/leeovery/Code/mint/internal/engine/regenerate_interactive.go:240 has no rendered effect yet, because a single-version regenerate emits no `RunFinished`. It is harmless and matches release.
- `sourceLabel` (/Users/leeovery/Code/mint/internal/engine/regenerate_interactive.go:407) would label `regenerateSourceFloor` as "fresh" through its default arm. That cannot happen, because the plan renders before the notes stage.
- /Users/leeovery/Code/mint/internal/engine/regenerate_write.go:31: the edited comment line runs well past the width of the rest of its paragraph. Cosmetic only; the content is true.
- As the executor said, the failed-`r`, Ctrl-C and unattended tests passed before the change. That is expected, since they guard behaviour that was already right.
- Nothing came with the dispatch beyond the enumerated inputs.
