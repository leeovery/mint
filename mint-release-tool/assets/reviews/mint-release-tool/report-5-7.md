TASK: mint-release-tool-5-7 — Provider release create-or-update probe

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/publish/publish.go:57 ReleaseExists added to Publisher seam, :118-152 GitHubPublisher.ReleaseExists via `gh release view {tag}`; internal/engine/regenerate_dispatch.go:40 DispatchRelease (probe-then-route). Wired per-version at regenerate_write.go:166 (single) + regenerate_batch.go:267 (batch). Probe classification: zero exit → exists; ErrCommandNotFound + any non-"release not found" stderr → genuine failure surfaced; only "release not found" marker → clean (false, nil). Dispatch surfaces probe error without writing — never silently defaults. No GitHub specifics in engine layer.

TESTS:
- Status: Adequate. regenerate_dispatch_test.go: existing→update, absent→create, per-version mix, probe-error-surfaces-without-dispatch, interface-only dependency, URL threaded back. publish_test.go: ReleaseExists true/not-found/genuine-failure/missing-gh, exact argv `gh release view {tag}`, Create/Update argv+stdin+failure.

CODE QUALITY:
- Followed conventions (runner seam, FakeRunner/fake Publisher, table tests, compile-time var _ Publisher). SOLID good — small interface, dispatch depends on abstraction. Low complexity, errors.Is.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/publish/publish.go:118 — notFoundMarker = "release not found" is a substring match on gh's English stderr; a localised/reworded gh build would misclassify an absent release as a genuine failure. Consider decoupling from message text (JSON/exit-code signal). [Same family as 1-8 note.]
