TASK: mint-release-tool-3-2 — preflight hook (runs after built-in gates, aborts on non-zero)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. Wired in engine/release.go:372-374 (call site immediately after runPreflight, before resolveHEAD/StartState and all mutation); helper runPreflightHook; env via buildHookEnv; mechanism in hooks.go. Ordering correct — gated behind built-in gates, runs before startingHEAD capture. Absent hook → no-op. Non-zero → *hooks.HookError surfaced as "preflight" StageFailed via plain surface (no unwind, nothing mutated). Array first-failure via shared runner. Reuses 3-1 mechanism; wires only the point.

TESTS:
- Status: Adequate. release_preflighthook_test.go: runs after gates + before mutation w/ successful finish (timeline order), non-zero aborts before mutation ("preflight" StageFailed, NO Unwound), array stops at first non-zero (exactly one sh ran), absent skipped, not-run-when-gate-fails-first. FakeRunner + RecordingPresenter. "runs from repo root w/ MINT_*" verified at hooks-package layer (3-1).

CODE QUALITY:
- Followed conventions (accept-interfaces, runner seam, Presenter-only output). SOLID good — runPreflightHook single-responsibility; mechanism/policy separated. Low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
