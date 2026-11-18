---
topic: dry-run-autostash-mutation
cycle: 1
total_proposed: 1
---
# Analysis Tasks: Dry-Run Autostash Mutation (Cycle 1)

## Task 1: Consolidate the redundant skipCleanTree preflight tests
status: approved
severity: low
sources: duplication

**Problem**: The change added two new `RunLocalGates` skipCleanTree tests in `internal/preflight/preflight_test.go` that build a byte-identical `argRunner` (same two seeded responses: `rev-parse --abbrev-ref HEAD` and `rev-parse -q --verify refs/tags/v1.2.3`) and make the identical `RunLocalGates(ctx, r, "main", "v1.2.3", false, true)` call. `TestRunLocalGates_SkipCleanTree_DirtyTreeDoesNotAbort` (lines ~290-304) asserts strictly less than `TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate` (lines ~262-288): it drops the call-count and porcelain-absence checks while exercising the same fixture, so it proves nothing the first test does not already prove (no `status --porcelain` is seeded and the clean-tree gate is skipped, so the first test's nil pass plus its porcelain-absence assertion already establishes that a dirty tree does not abort under `skipCleanTree=true`).

**Solution**: Remove the redundant test so a single test carries the skipCleanTree coverage, OR — if the "dirty tree does not abort" framing is wanted as documentation — keep it but make it earn its place by adding the one distinguishing assertion the first test lacks rather than re-running the same fixture for a weaker check. Prefer removal: the surviving test already asserts the porcelain probe is absent, the run passes, and the call count is exactly the on-branch + tag-free probes.

**Outcome**: The skipCleanTree preflight behaviour is covered by exactly one test (or by two tests each asserting something distinct), with no test that is a strict subset of another over an identical fixture. Coverage of the skip semantics is unchanged.

**Do**:
1. Open `internal/preflight/preflight_test.go`.
2. Confirm `TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate` (around lines 262-288) asserts: the run passes (nil/expected pass), `git status --porcelain` is absent from recorded calls, and the call count equals exactly the on-branch + tag-free probes.
3. Delete `TestRunLocalGates_SkipCleanTree_DirtyTreeDoesNotAbort` (around lines 290-304) and its setup. (Alternative, only if the dirty-tree framing is explicitly preferred: instead of deleting, seed a genuinely dirty fixture for it and add the distinguishing assertion that the dirty status is never read, so it stops being a subset of the first test.)
4. Run the project gates.

**Acceptance Criteria**:
- No test in `internal/preflight/preflight_test.go` re-runs the `RunLocalGates(..., skipCleanTree=true)` fixture with a strictly weaker set of assertions than another test.
- The surviving skipCleanTree test still proves: clean-tree probe (`git status --porcelain`) is absent, the branch and tag-free gates still run, and the run passes.
- `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, and `golangci-lint run` (0 issues) all pass.

**Tests**:
- The retained `TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate` continues to pass and remains the authoritative proof that `skipCleanTree=true` bypasses the clean-tree probe while leaving the branch and tag-free gates intact.
