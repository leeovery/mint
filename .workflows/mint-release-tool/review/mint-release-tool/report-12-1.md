TASK: Remove the Load-Dependent Timing Flake in the AI Transport Real-Deadline Test (mint-release-tool-12-1, type: bug)

ACCEPTANCE CRITERIA:
- The test no longer reads or writes a marker file and no longer asserts an invocation count.
- The test still asserts errors.Is(err, ai.ErrTimeout) and that the error is NOT ai.ErrGenerationFailed (formerly ErrNotesFailure) against a real exec.CommandContext deadline kill.
- go test ./internal/ai/... passes in isolation.
- go test -race ./internal/... ./cmd/... passes with the real-deadline test included, repeatably under CPU contention.
- The not-retried-on-timeout behaviour remains covered by TestTransport_Generate_DoesNotRetryTimeout.
- No unused imports remain; the package builds and go vet ./internal/ai/... is clean.

STATUS: Complete

SPEC CONTEXT:
specification.md line 302 ("Output format & validation"): "A timeout is not retried — it goes straight to on_notes_failure (retrying a hung call only risks a second full timeout); the single retry covers empty/error/refusal content only." The transport must classify a deadline kill as a distinct timeout failure (ErrTimeout), kept separate from the bad-content failure (ErrGenerationFailed) so on_notes_failure routing can branch on it. This task is a test-hardening bug fix: it removes a load-dependent subprocess-startup race from the real-deadline test without losing the not-retried coverage (which lives in the deterministic FakeRunner sibling).

IMPLEMENTATION:
- Status: Implemented
- Location: internal/ai/transport_test.go:248-290 (TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout)
- Notes:
  - Marker plumbing fully removed. The script body is now exactly "#!/bin/sh\nsleep 5\n" (line 272) — no printf, no marker file, no marker arg. dir := t.TempDir() (line 270) and the script path filepath.Join(dir, "ai-command") (line 271) are retained as required.
  - The os.ReadFile(marker) / readErr / len(got) != 1 read-and-count block is gone. Verified by grep: no "marker", "ReadFile", or invocation-count assertion remains in this test. (The "marker" token appears only once in the file, at line 262, inside the updated explanatory comment that documents WHY no marker is used — correct, not residual code.)
  - ai.NewTransport(runner.NewExecRunner(), ...) with Timeout: 300ms vs a 5s sleep is retained (lines 278-281). NewExecRunner confirmed to exist at internal/runner/exec_runner.go:22.
  - Both assertions retained: errors.Is(err, ai.ErrTimeout) (line 284) and the negative !errors.Is(err, ai.ErrGenerationFailed) (line 287). Per the rename note (commit 9e378d6), ErrNotesFailure -> ErrGenerationFailed; the current code correctly references ErrGenerationFailed, defined at transport.go:30.
  - Explanatory comments (lines 251-269) updated: they now state the test "asserts ONLY the timing-robust classification" and that "exactly one invocation / no retry on timeout" is covered deterministically by TestTransport_Generate_DoesNotRetryTimeout (FakeRunner). The stale "300ms is generous enough to write its byte" claim is gone; lines 276-277 instead explain the 5s sleep guarantees the 300ms deadline fires mid-sleep regardless of load.

TESTS:
- Status: Adequate
- Coverage:
  - Genuinely-new coverage retained: a REAL exec.CommandContext deadline kill classifies as ErrTimeout and is distinguishable from ErrGenerationFailed (the end-to-end production classification path through classifyFatal / ctx.Err() wrapping).
  - The 5s sleep against a 300ms deadline makes the deadline fire mid-sleep independent of subprocess startup speed, so the timeout path is exercised on every run — the assertion is now timing-robust.
  - Not-retried-on-timeout remains covered deterministically by the sibling TestTransport_Generate_DoesNotRetryTimeout (lines 226-246), which seeds a wrapped context.DeadlineExceeded via FakeRunner and asserts invocations == 1. No correctness coverage lost.
- Notes:
  - I did not (and may not) execute the suite. Judging by reading: the removed assertion was the only thing in this test that depended on the fork-vs-SIGKILL wall-clock race; the two surviving assertions depend only on the 300ms deadline firing during a 5s sleep, which is robust under CPU contention. This is consistent with the golang-testing determinism rule (no load-dependent flakes).
  - Not over-tested: the test now asserts exactly the one behaviour it uniquely owns; the redundant marker side-effect (which duplicated the FakeRunner sibling's not-retried coverage while adding a race) is correctly dropped.

CODE QUALITY:
- Project conventions: Followed. Imports verified all-used — context (line 234), errors, fmt (line 234), os + filepath (lines 271-272), testing, time, mint/internal/ai, mint/internal/runner. No pruning was needed because os/filepath remain in use for the script write; the acceptance criterion "no unused imports remain" holds. go vet could not be run (read-only) but no construct in the edit introduces a vet concern.
- SOLID principles: N/A (test code); single clear responsibility per test.
- Complexity: Low. The test is now a linear setup-act-assert with no side-effect read-back.
- Modern idioms: Yes — t.TempDir(), t.Context(), errors.Is for sentinel matching, t.Parallel().
- Readability: Good. Comments accurately describe the narrowed scope and cross-reference the sibling test that owns the not-retried assertion.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
