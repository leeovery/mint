TASK: mint-release-tool-3-3 — pre_tag hook execution & artifact commit (commit-interplay rule)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go:411-417 Stage 3 wiring (after startingHEAD capture / before notes), runPreTagHook, pretagArtifactSubject; record.CommitDirtyTree; hook runner hooks.go. Ordering correct (post-startingHEAD so artifact commit is unwind-covered, pre-notes so notes diff at post-hook HEAD). Fixed `chore(release):` subject (not configurable commit_prefix) keeps it distinct from bookkeeping. Dirty-probe is a read via m.Run; add -A/commit via m.Mutate (lock-resilient). Clean/own-commit/gitignored all collapse to empty-porcelain check (status without --ignored). committed folds into made.Commits only when a commit lands. Non-zero → surfaceAndUnwind("pre_tag", …).

TESTS:
- Status: Adequate. release_pretaghook_test.go covers all 8 ACs: dirty→one separate artifact commit (distinct-from/precedes bookkeeping), Stage-3 ordering, clean→none, own-commit→none, gitignored→none (no --ignored), non-zero→clean abort + pre_tag StageFailed + no mutation, absent→skip. record/commit_test.go covers CommitDirtyTree dirty/clean/probe-fail/stage-fail. Commit-graph ordering in release_commitgraph_test.go (3-8) — complementary.

CODE QUALITY:
- Followed conventions (accept-interfaces/return-structs, runner seam, single-source subject helper, godoc). SOLID good — CommitDirtyTree single-responsibility. Low complexity, intent-rich comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/release_pretaghook_test.go:294-295 — remove the stale extra f.SeedSequence("git", ScriptedOut(startingSHA)) and its "unwind: rev-parse HEAD (unchanged)" comment in TestRelease_PreTagHook_NonZeroAbortsBeforeTag. The surgical unwind no longer re-probes HEAD (rev-parse HEAD count==1 in the sibling test), so this seed is never consumed — dead scaffolding.
- [do-now] internal/engine/release_pretaghook_test.go:293-294 — fix the misleading comment "the unwind re-probes HEAD; the hook made no commit, so HEAD is unchanged"; the unwind drives off tracked MadeState and does not probe HEAD. Reword to match the surgical model.
- [do-now] internal/engine/release_pretaghook_test.go:13-18 — the test redeclares a local pretagArtifactSubject helper duplicating the production release.go:1087 function (fine for black-box isolation, but add a one-line comment noting it intentionally mirrors the production constant so a future subject change updates both). Optional.
