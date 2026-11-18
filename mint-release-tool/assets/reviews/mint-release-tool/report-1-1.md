TASK: mint-release-tool-1-1 — Project skeleton & CommandRunner seam

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (intentionally exceeds bare Phase 1 scope — interface also grows RunInteractive/RunInDir added by later tasks)
- Location: internal/runner/runner.go, internal/runner/exec_runner.go, internal/runner/fake_runner.go
- Notes: Stdout/Stderr captured separately; ExitCode from ProcessState; non-zero exit → populated Result + non-nil error; missing binary → ErrCommandNotFound sentinel (errors.Is). Module untouched (path stays `mint`), presenter unmodified. stdin affordance (RunWith) present for Stage 4.

TESTS:
- Status: Adequate
- Coverage: All AC-named tests present; not-found-vs-non-zero distinction tested both implementations both directions. Stdlib testing style (testify not a project dep). ExecRunner tests spawn POSIX tools (no git/gh/claude).

CODE QUALITY:
- Followed project conventions (sentinel + errors.Is, %w wrapping, compile-time interface assertions). SOLID good, low complexity, modern idioms, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/runner/exec_runner.go:81 — add a one-line comment noting cmd.ProcessState.ExitCode() is nil-safe (returns -1 for unstarted/not-found) so the unconditional call before the runErr check doesn't read as a latent nil-deref.
- [idea] internal/runner/exec_runner_test.go — ExecRunner tests depend on a POSIX shell + coreutils; decide whether to guard them (build tag / GOOS check) or document the POSIX assumption so "go test ./... on a clean checkout" holds on non-POSIX CI.
- [idea] internal/runner/fake_runner.go:42-43 — FakeRunner matches on command name only (args recorded but not matched); decide whether arg-aware matching is worth adding before more complex multi-call scripts arrive.
