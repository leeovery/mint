TASK: mint-release-tool-3-1 — Hook runner foundation (sh -c, repo root, MINT_* env, string|array)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/hooks/hooks.go (Runner, HookError, Run, normalise) + env.go (Bump, HookEnv, NewHookEnv, Render). Run(ctx, value any, repoRoot, env) — repoRoot param is a faithful realization of the plan's "thread repo root" requirement, not drift. Routes through CommandRunner.RunInDir, layering env on inherited env. Each entry runs as `sh -c "<entry>"`; array sequential; first non-zero exit stops sequence; empty/absent → no-op. MINT_BUMP patch/minor/major/explicit. Single render point for MINT_*.

TESTS:
- Status: Adequate. hooks_test.go: single string (one sh -c), array order, repo-root dir, MINT_* injection, dry-run=1, MINT_BUMP=explicit, first-failure stop w/ HookError inspection (errors.Is + errors.As + Entry/Stderr/ExitCode), 4-case no-op table, []any normalisation path. Render-set exactness guards env growth.

CODE QUALITY:
- Followed conventions (runner seam, custom error type Unwrap + exported fields, lowercase error string, %v at boundary). SOLID/DRY good — single render point, normalise decomposed, mechanism/env split. Low complexity, strong doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/hooks/hooks.go:113-120 — normaliseAnySlice silently skips non-string elements in a []any (e.g. a number/bool in the TOML array). Doc comment defers to "full schema validation is a later phase." Confirm Phase 6 schema validation actually rejects a non-string array element so the case isn't lost (decide whether the silent skip is acceptable).
- [quickfix] internal/hooks/hooks_test.go:19-252 — no test asserts that an empty-string element within an array (e.g. ["a","","b"]) is dropped while preserving order, even though normaliseStringSlice/normaliseAnySlice implement that skip. Add a case asserting the two non-empty entries run in order.
