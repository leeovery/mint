TASK: mint-release-tool-6-6 — `release` shim generation

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, matches ACs exactly. internal/initgen/shim.go: ReleaseShim() (line 24) returns shebang + `command -v mint` guard (redirected >/dev/null 2>&1) echoing the exact hint `brew install leeovery/tools/mint` to stderr + `exit 1` + `exec mint release "$@"`; ShimMode os.FileMode = 0o755 (line 8). Pure generator, no IO. exec replaces the shim with mint release for clean exit-code propagation.

TESTS:
- Status: Adequate (string assertions + two host-gated runtime checks). shim_test.go: shebang prefix, exact `exec mint release "$@"` line, `command -v mint` guard, exact install hint + exit 1, ShimMode==0o755. Runtime: absent-mint → non-zero exit + hint on stderr (skips if no sh), present-mint stub → forwards `release -m --set-version 2.0.0`.

CODE QUALITY:
- Followed conventions (pure generator + exposed mode constant matches 6-5/6-7 split, t.Parallel). SOLID/DRY good — single responsibility. Low complexity, errors.As for *exec.ExitError, raw-string literal. Precise load-bearing comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
