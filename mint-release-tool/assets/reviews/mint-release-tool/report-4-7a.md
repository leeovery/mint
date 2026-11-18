TASK: mint-release-tool-4-7a — Dry-run core: read-only run, skip all mutations, print the full plan

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. Boundary placed AFTER the review gate (engine/release.go:480-482) so an interactive dry run still shows plan/notes and runs the gate (4-8 orthogonality), then returns via finishDryRun without reaching Record/tag/push/publish — never touching deps.Mutator. finishDryRun, buildDryRunPlan, dryRunPublishTarget, writeDryRunNoteCache. Flag wired cmd/mint/flags.go. Plan includes optional pre_tag artifact commit, bookkeeping commit (real subject), tag, atomic push, publish step honoring publish=false (omitted)/resolved (tag)/unresolved (downgraded, warned). Hook skips (3-11) + note-cache write (4-7) reused.

TESTS:
- Status: Adequate. release_dryrun_test.go: no-mutation first-release (unseeded mutation commands prove no mutation), read-only preflight + version, full-plan print (commit/tag/push/publish), AI notes preview generated, publish=false omits publish step, unresolved-provider downgrade warned + not naming provider release, all three hooks skipped+reported, working-tree byte-for-byte unchanged. Mutation-free enforced structurally (FakeRunner errors on unseeded commands).

CODE QUALITY:
- Followed conventions (accept-interfaces for NoteCache, runner/Mutator seam discipline, presenter-only, doc comments). SOLID/DRY good — dryRunPublishTarget mirrors Stage-6 via shared resolvePublisher/warnPublishDowngraded; plan subjects reuse BookkeepingSubject/pretagArtifactSubject. Low complexity.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release.go:1153,1176 — finishDryRun takes current/next/bumpKind solely to forward to runPostReleaseHook, which early-returns under dryRun=true before buildHookEnv reads them; dead on the dry-run path. Decide whether to drop them from the signature or keep for caller-signature parity.
- [quickfix] internal/engine/release_dryrun_test.go:385-392 — downgradeWarned matches ev.Warn.Label == "publish skipped" as a string literal; producer is warnPublishDowngraded (release.go:1447). Reference a shared label constant so a future label rename can't silently break the match.
