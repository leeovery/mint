TASK: mint-release-tool-8-2 — Close the release-success footer URL seam — feed the real release URL into RunResult.URL

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (seam closed end-to-end). internal/publish/publish.go:54-58 (interface now returns (url, err)), :98-111 CreateRelease + :172-185 UpdateRelease capture res from RunWith and return strings.TrimSpace(res.Stdout); internal/engine/regenerate_dispatch.go:40-49 DispatchRelease propagates URL; internal/engine/release.go:624-636 threads CreateRelease's URL into releaseURL → RunResult.URL at :653; regenerate paths discard URL with explanatory comments; presenter branches unchanged at pretty.go:923-930 / plain.go:481-487. Forward success uses CreateRelease directly (correct — freshly-pushed tag is always a create). releaseURL only assigned in err==nil branch (empty on failure/downgrade).

TESTS:
- Status: Adequate. Driver: trimmed URL parsed for create AND update, empty-stdout→empty-URL-no-error, gh-failure→empty-URL+error. Engine: success threads URL, warn-only failure empty, downgrade empty. Dispatch: returns publisher URL on both create/update. Presenter populated-URL footer in both modes + empty-URL-omission. fakePublisher updated to new signature w/ compile-time assertion.

CODE QUALITY:
- Followed conventions (runner seam, %w, table tests, t.Parallel, compile-time interface assertions). SOLID good — interface shape unchanged, drivers cheap. Low complexity, comments explain why URL discarded on regenerate / empty on failure.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
