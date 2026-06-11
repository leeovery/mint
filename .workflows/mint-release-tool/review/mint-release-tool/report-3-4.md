TASK: mint-release-tool-3-4 — post_release hook (warn-only on failure)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go:646-648 Stage 7 call (after CreateRelease:630), runPostReleaseHook, warnPostReleaseFailed (spec-exact "post_release hook failed; tag is already published"), hookFailureOutput; shared array/repo-root/env mechanism in hooks.go + env.go. Hook runs UNCONDITIONALLY (past PONR tag is public whether publish true/false). Non-zero → warn-only (no return/unwind), run reaches RunFinished returns nil. Array stop-on-first-failure → warn not abort. Absent → nil no-op. MINT_* via buildHookEnv + RunInDir.

TESTS:
- Status: Adequate. release_postreleasehook_test.go: runs-after-publish (ordering), non-zero-warns-only (label + exact message, no StageFailed/Unwound, RunFinished, nil), non-zero-does-not-unwind (no reset/tag-d), array-stops-at-first-failure (one sh, single Warn, finish), absent-skipped. Env-from-root + MINT_* at the seam.

CODE QUALITY:
- Followed conventions (accept-interfaces seams, runner/presenter, table-comment discipline). SOLID good — mirrors runPreflightHook/runPreTagHook; consequence at call site. Low complexity, errors.As for HookError.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/release_postreleasehook_test.go:34-76 — add an assertion in RunsAfterProviderRelease that the recorded post_release sh invocation carries MINT_* env (Dir==root, Env contains MINT_VERSION_TAG=v0.0.1), to pin "from repo root with MINT_* injected" directly on this hook point rather than relying solely on shared-runner tests.
- [do-now] internal/engine/release_postreleasehook_test.go:199 — AbsentSkipped writes no .mint.toml at all, exercising "no hooks table" rather than "config present, post_release key absent"; add a one-line comment (or sibling config with [release.hooks] present but no post_release) to make the absent-key path explicit.
