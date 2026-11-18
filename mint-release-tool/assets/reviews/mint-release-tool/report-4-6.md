TASK: mint-release-tool-4-6 — Explicit version (--set-version) validation (MINT_BUMP=explicit)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. cmd/mint/flags.go:78,92,106-111 (flag registration + mutual-exclusivity, exact "can't combine --set-version with a bump flag"); version/version.go:42-46 BumpExplicit, :63-83 ParseSemVer (prefix-tolerant strict grammar); engine/release.go:679-695 resolveNextVersion (parse + strictly-greater gate `!pinned.GreaterThan(current)`, returns BumpExplicit), :1105-1108 hookBump → hooks.BumpExplicit; hooks/env.go renders MINT_BUMP=explicit. CLI threads raw value unparsed; engine owns parse + gate (needs prefix + resolved latest). Gate in Stage 1 before any mutation. No --force/downgrade override.

TESTS:
- Status: Adequate. flags_test.go: thread-through + combined-with-every-bump-form error exact message. version_test.go: ParseSemVer valid (prefix-stripped/bare/component) + rejects (2.0, 2.0.0.1, rc, +b5, abc, empty, v2.0.0.1, 1.2.x). release_setversion_test.go full spine: malformed-rejected, equal-rejected exact message, less-rejected, greater-becomes-next + computed-patch-NOT-tagged, first-release accepted, MINT_BUMP=explicit injected, BumpExplicit-distinct enum guard. assertNoMutation on rejections.

CODE QUALITY:
- Followed conventions (table tests, FakeRunner+RecordingPresenter, single-render hook env). SOLID good — parse/gate/render separated; reuses single tag grammar. Low complexity, intent-rich doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/version/version.go:79 — strings.TrimPrefix(value, prefix) strips at most one prefix occurrence, so a doubled prefix (vv1.2.3 under prefix v) or a pathological numeric prefix yields a loud rejection rather than a tailored message. Behaviour is correct and spec-compliant ("reject ambiguity loudly"); consider whether a more specific error message for a stray-but-recognisable prefix is worth adding.
