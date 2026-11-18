---
topic: dry-run-autostash-mutation
cycle: 2
total_proposed: 1
---
# Analysis Tasks: Dry-Run Autostash Mutation (Cycle 2)

## Task 1: Reuse seedDryRunFirstRelease for the dry-run+autostash dirty-tree test instead of hand-copying the read-gate timeline
status: approved
severity: low
sources: duplication

**Problem**: `TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview` (internal/engine/release_autostash_test.go:309-320) inlines a 10-line `f.SeedSequence("git", ...)` read-side timeline that is byte-for-byte the existing `seedDryRunFirstRelease` helper (internal/engine/release_dryrun_test.go:33-47) with exactly one line removed — the `status --porcelain` clean-tree probe, which is the very gate the dry-run+autostash combo bypasses. The two timelines are the same first-release dry-run read sequence and will drift together whenever the read-side gate order changes. Because the only difference is the presence/absence of one gate line, the divergence is the meaningful signal and is easy to lose in a hand-copied block. Both tests live in `package engine_test`, so the helper is already in scope.

**Solution**: Express the one intentional difference (the absent clean-tree probe) once and share the read-gate prefix. Parameterise `seedDryRunFirstRelease` with a `skipCleanTree bool` that conditionally seeds the `status --porcelain` line, and have the new dry-run+autostash test call it with `skipCleanTree=true`. This turns the load-bearing omission from a hand-copied gap (which could be mistaken for an error) into an explicit, named argument, while making it impossible for the shared prefix to silently diverge. (Equivalent alternative: add a sibling helper `seedDryRunAutostashReadGates` that scripts the identical sequence minus the clean-tree probe — choose whichever fits the existing helper conventions in the file; the parameterised helper is preferred because it keeps a single source of truth for the shared prefix.)

**Outcome**: There is exactly one definition of the first-release dry-run read-gate timeline. The intentional clean-tree-gate skip is expressed as a single explicit flag at the call site rather than a silent line omission. A future change to the read-side gate order updates one place and both tests stay consistent automatically.

**Do**:
1. In internal/engine/release_dryrun_test.go, change the signature of `seedDryRunFirstRelease` to add a trailing `skipCleanTree bool` parameter.
2. Inside the helper, seed the `status --porcelain` (clean) line conditionally — emit it only when `skipCleanTree` is false. Keep the rest of the `SeedSequence` order identical (rev-parse --show-toplevel → symbolic-ref → tag --list → fetch --tags → [status --porcelain] → rev-parse --abbrev-ref HEAD → rev-parse -q --verify → rev-list → ls-remote → rev-parse HEAD → remote get-url).
3. Update the helper's WHY-comment so it remains true-to-as-built: note that it seeds the read-only first-release dry-run timeline and that the clean-tree probe is omitted when `skipCleanTree` is set (the gate the DryRun && AutoStash combo bypasses).
4. Update every existing caller of `seedDryRunFirstRelease` to pass `skipCleanTree=false` (preserving current behaviour). Search the engine test package for all call sites.
5. In internal/engine/release_autostash_test.go, replace the inlined 10-line `f.SeedSequence("git", ...)` block (lines 309-320) in `TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview` with a single call to `seedDryRunFirstRelease(f, root, "main", "v0.0.1", true)` (matching the existing argument shape — confirm the branch and tag values against the inlined comments). Keep the test's existing inline WHY-comment about the absent clean-tree probe and dirty-tree bypass.
6. Run the project gates and confirm the test still asserts zero `status --porcelain`, zero stash push/pop, no mutation tail, no gh, and a clean `RunFinished`.

**Acceptance Criteria**:
- `seedDryRunFirstRelease` carries a `skipCleanTree bool` parameter and seeds the `status --porcelain` line only when it is false; all other seeded lines and their order are unchanged.
- The inlined `SeedSequence` block in `TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview` is gone, replaced by a single call to the helper with `skipCleanTree=true`.
- Every other caller of `seedDryRunFirstRelease` passes `skipCleanTree=false` and its behaviour is byte-for-byte unchanged.
- The helper's WHY-comment is updated true-to-as-built and explains the conditional clean-tree slot.
- All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Tests**:
- No new test is required — this is a test-only refactor. The existing `TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview` and all other `seedDryRunFirstRelease` callers must continue to pass unchanged, proving the seeded read timelines are equivalent after consolidation.
- Confirm `go test -race ./internal/engine/...` passes and that `TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview` still asserts zero `git status --porcelain`, zero `git stash push/pop`, no mutation tail, no `gh`, and a final `RunFinished` event with no `StageFailed`.
