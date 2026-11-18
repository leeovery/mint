# Implementation Review: Dry-Run Autostash Mutation

**Plan**: dry-run-autostash-mutation
**QA Verdict**: Approve

## Summary

The quick-fix lands exactly as specified: `mint release --dry-run --autostash` against a dirty tree is now provably mutation-free. The real stash is skipped under `DryRun && AutoStash` and the clean-tree preflight gate is conditionally bypassed for that same combo (mirroring the existing `--any-branch` skip pattern). All three tasks across the three phases — the core change plus two analysis-cycle test-consolidation cleanups — are implemented faithfully, with WHY-comments updated true-to-as-built. The five project gates all pass (build, gofmt clean, vet, `go test -race` green, golangci-lint 0 issues). No blocking issues; one non-blocking idea about a test helper.

## QA Verification

### Specification Compliance

Implementation aligns with the specification on every point:
- Autostash block gated `if opts.AutoStash && !opts.DryRun` (`internal/engine/release.go:392`) — neither `autostashPush` nor the deferred `autostashPop` runs in dry-run, so the `git.Mutator` is never reached. The CLAUDE.md load-bearing invariant ("a dry run NEVER reaches the Mutator; the repo is byte-for-byte unchanged") now genuinely holds for the `--autostash` combo — the one path that previously violated it.
- `skipCleanTree bool` added to `preflight.RunLocalGates` (`internal/preflight/preflight.go:81`), mirroring `anyBranch`; `CheckCleanTree` wrapped in `if !skipCleanTree` so the `git status --porcelain` probe is skipped entirely while the on-branch and tag-free gates still run.
- `runPreflight` threads the parameter (`release.go:1106`) and the call site passes `opts.DryRun && opts.AutoStash` (`release.go:413`) — the bypass is correctly conditioned on the combo, never `DryRun` alone (single call site, no second trigger path).
- All declared exclusions honoured: no change to real-run autostash, non-autostash dry-run, other gates, and no new flags/config/presenter events. Scope is clean (commit `b1396e9` touches only the three production files + tests + bookkeeping).

### Plan Completion
- [x] Phase 1 (core change) acceptance criteria met
- [x] Phase 2 (consolidate redundant skipCleanTree preflight tests) acceptance criteria met
- [x] Phase 3 (reuse `seedDryRunFirstRelease` for the dry-run+autostash test) acceptance criteria met
- [x] All 3 completed tasks verified
- [x] No scope creep

### Code Quality

No issues found. The change adds one single-responsibility boolean threaded through the existing gate driver exactly like `anyBranch` — no new abstraction, low complexity (two `if !flag` guards + one `&&`). Every mutation still flows through `git.Mutator`, reads stay on the runner, output stays on the presenter. WHY-comments updated across `ReleaseOptions.AutoStash`/`.DryRun` field docs, the Stage-2 autostash block, the `Release` and `runPreflight` docs, `RunLocalGates`, and the `autostash.go` header — verified line-by-line as true-to-as-built (no comment claims the stash runs in dry-run).

### Test Quality

Tests adequately verify requirements; neither under- nor over-tested.
- Engine (`release_autostash_test.go:301`): dry-run+autostash on a dirty tree asserts **zero** `git stash push`, **zero** `git stash pop`, **zero** `git status --porcelain` (gate bypassed), no gh, `assertNoMutation`, terminal `RunFinished`, no `StageFailed` — exact argv per the project idiom.
- Preflight (`preflight_test.go:262`): `skipCleanTree=true` asserts the porcelain probe is absent and exactly 2 gate calls ran (branch + tag-free); `skipCleanTree=false` remains exercised by the existing suite.
- Regression guards intact: real (non-dry) autostash still stashes/pops; non-autostash dirty dry-run still aborts at the clean-tree gate (`release_dryrun_test.go` positively asserts the porcelain probe WAS issued).
- Phase 2 removed a strict-subset duplicate test; Phase 3 folded the hand-copied read-gate timeline into a `skipCleanTree` parameter on the shared `seedDryRunFirstRelease` helper, leaving one definition of the gate order with the conditional clean-tree slot exercised by contradictory positive assertions on both sides.

### Required Changes (if any)

None.

## Recommendations

### Applied during review

1. `internal/engine/release_test.go:1304` — broadened the `assertNoMutation` helper to also reject `git stash push` / `git stash pop` (Report 1-1) — **APPLIED**
   - The helper previously flagged only `git tag -a` / `git push` / `gh release create`, framed as "mutations to the remote". Stash is a local working-tree mutation that still flows through `git.Mutator`, so the dry-run invariant ("never reaches the Mutator / byte-for-byte unchanged") already covers it. Added two `case` arms (`git stash push`, `git stash pop`) and reworded the doc comment to state the helper now mirrors the full Mutator-invariant. Verified safe: none of the ~40 `assertNoMutation` callers performs a real stash, so no existing test regressed. Future dry-run tests now inherit the stash guard automatically rather than each restating it (the dry-run+autostash test's explicit `invokedWith` checks at `release_autostash_test.go:318-323` remain as belt-and-braces). All five project gates pass.
