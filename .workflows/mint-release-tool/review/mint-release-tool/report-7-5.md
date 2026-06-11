TASK: mint-release-tool-7-5 — Consolidate copied cross-boundary constants and the remote-URL reader to single owned symbols

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. (1) engine.RemoteURL single reader at internal/engine/release.go:905, called by both cmd sites (main.go:166, regenerate_all.go:49) + engine's resolvePublisher; regenerateRemoteURL gone. (2) record.ChangelogDateLayout (changelog.go:27) parsed against in regenerate_write.go:345; no engine-local layout. (3) record.ChangelogFileName (changelog.go:20) staged in regenerate_write.go:285-286; no engine-local const. (4) engine.ErrChangelogDisabled (regenerate_batch.go:125) referenced by both validators (regenerate_validate.go:63, regenerate_batch.go:134); both keep distinct concrete-enum signatures. Repo-wide grep confirms each literal lives in one owner + test pins; no orphaned references. (Task path hint internal/engine/remote_url.go was a suggestion; reader kept in release.go alongside resolvePublisher — reasonable; only the test is named remote_url_test.go.)

TESTS:
- Status: Adequate. Four focused unit pins (remote_url_test.go trim/empty/non-zero-exit; changelog_filename_test.go; changelog_layout_test.go; changelog_disabled_test.go message). written==staged via git add CHANGELOG.md + content; date-heal round-trip (parse==emit layout, header retains historical date); both validators' sentinel identity via errors.Is on cmd + batch paths.

CODE QUALITY:
- Followed conventions (exported symbols owner-doc'd, accept-interfaces seam on RemoteURL, sentinel as package var). SOLID good — record owns write-projection symbols, engine owns reader/sentinel; validators kept separate as specified. Low complexity, pure extraction.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] cmd/mint/main.go:166 + cmd/mint/regenerate_all.go:49 — the two regenerate cmd paths still duplicate `publisher, _ := publish.ResolvePublisher(engine.RemoteURL(ctx, r), cfg.Release.Provider, r)` plus near-identical rationale comment (logged as analysis-duplication-c2/c3, out of scope for this reader-only task). Decide whether to lift a single engine.ResolvePublisher(ctx, r, cfg) helper both cmd paths reuse. [Recurring across 4-9/7-5.]
- [quickfix] cmd/mint regenerate wiring — no cmd-package test exercises "the cmd regenerate path resolves the publisher via the shared reader"; RemoteURL is pinned in engine and resolution covered at engine level, but the cmd swap is verified only by compilation. If/when the resolve helper is extracted, add one cmd-level test asserting an empty remote yields a nil/unresolved publisher through that path.
