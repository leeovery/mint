TASK: mint-release-tool-5-9 — Single-version regenerate write, push & recovery

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/engine/regenerate_write.go: RegenerateWrite (137-179) owns ordering; gateRegenerate/reuseConfirm (186-225) source gating; writeAndPushChangelog (234-271) commit+push; pushChangelogCommit (306-313, plain `git push origin HEAD`) and resetAndAbort (319-325, `git reset --hard {startingHEAD}` only when committed) the PONR/recovery; warnRegenerateProviderFailed (357-363) post-PONR warn; readHistoricalDate (336-350, `git for-each-ref --format=%(creatordate:short)`) preserves original version date. Reuses shared stageAndCommitChangelog/pushChangelogCommit/resetAndAbort with the batch path so push form + recovery can't drift. Provider write via DispatchRelease (5-7). pushed flag gates reset-vs-warn.

TESTS:
- Status: Adequate. regenerate_write_test.go: plain-push-no-tag (no --atomic/no-tag-arg), historical date, gate-abort-no-push, pre-push-failure-resets-commit-no-tag, provider-failure-post-push warn-only-no-reset, both ordering changelog-push-before-provider-dispatch, reuse two-choice vs fresh four-choice gate, release-only no changelog mutation, reuse decline aborts no dispatch/git, r/e review-gate peers thread regenerated/edited body.

CODE QUALITY:
- Followed conventions (mutations via Mutator, reads via CommandRunner, presenter seam, engine-level enums avoid threading cmd flag types). SOLID good — single-responsibility helpers, shared stage/commit/push/reset. Low complexity, self-documenting PONR asymmetry.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/regenerate_write_test.go:226 — the warn-only test forces the post-push provider failure via the ReleaseExists probe (fake's Create/Update can't fail). Consider adding a fake-publisher write-error seam + a test where UpdateRelease/CreateRelease itself fails after the push, to pin warn-only on the actual write surface, not only the probe.
- [quickfix] internal/engine/regenerate_write_test.go:188-191 — the doc comment above TestRegenerateWrite_PrePushFailure_ResetsCommit still reads "TestRegenerateWrite_Fresh_GateAbortAfterCommit_ResetsCommit proves…", a stale name not matching the function below. Update the leading identifier to the actual function name.
