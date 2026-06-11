TASK: mint-release-tool-1-11 — Release command wiring (end-to-end first-release)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. CLI wiring cmd/mint/main.go (run/runRelease, classifyCommand, exitCode) + flags.go (parseReleaseFlags); orchestrator engine/release.go Release. Presenter constructed once at main.go:201 via presenter.NewForStartup(plainFlag, yes, os.Stdout, os.Stderr, os.Stdin); --plain parsed engine-side, no downstream re-detection. Spine order exact: version → preflight → notes → record → gh gate (before tag) → tag → atomic push → publish. PONR asymmetry explicit (pre-push surface/unwind; Stage-7 publish failure warn-only). Code evolved through later phases but 1-11 spine intact.

TESTS:
- Status: Adequate. Every AC has a focused test on FakeRunner + RecordingPresenter: full-spine order (command + event timelines), bump matrix, always-prompts-under-y, Prompt-error abort both sentinels, failing gate aborts before mutation, publish-fails-after-push warns only, publish=false tag+push only no gh, blocking/Elapsed stage events, CLI flags + dispatch.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, single runner, thin main shell). SOLID good — orchestrator only orders units. Release is long but linear and narrated. Good idioms/readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/engine/release_test.go:1244 — fix malformed comment: line reads `/ (here: the working tree is dirty)` (single leading slash, breaks doc-comment continuation); make it `// (here: …)`.
- [quickfix] cmd/mint/main.go:45 (run) and :192 (runRelease) — neither exercised by a test; only constituent parts (classifyCommand, parseReleaseFlags, exitCode) are. Add a thin table test driving run([]string{...}) against fake runner/presenter seam (or document the deliberate gap) so presenter+deps construction wiring is regression-covered end to end.
