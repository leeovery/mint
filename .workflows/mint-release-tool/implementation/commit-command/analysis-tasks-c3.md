---
topic: commit-command
cycle: 3
total_proposed: 2
---
# Analysis Tasks: Commit Command (Cycle 3)

> Converged cycle. Production code is clean per all three agents. Both tasks below are LOW-severity, TEST-ONLY polish with no production-code impact. They are proposed honestly at low severity for the orchestrator to accept or defer — neither is a correctness fix.

## Task 1: Extract a shared named-step FakeRunner scripting vocabulary for the commit test suite
status: skipped
severity: low
sources: duplication

**Problem**: ~16 per-file `seed*` FakeRunner helpers across ~13 test files in `internal/commit/` each independently re-spell the same git scripted-call vocabulary in the same orchestrator-dictated order. The recurring `runner.ScriptedCall` literals are copy-pasted verbatim: the non-empty preflight read `{Result: runner.Result{Stdout: "x\n"}} // git diff --cached --name-only` appears ~43 times, the editor resolution `{Result: runner.Result{Stdout: editor + "\n"}} // git var GIT_EDITOR` ~59 times, the bare commit sink `{} // git commit -F -` ~44 times, plus shared L1-diff, ls-files-empty, add, and push calls. The helpers differ only in which subset of these calls they string together. This encodes the production call ORDER (preflight read -> L1 diff -> [editor resolve] -> [stage] -> commit -> [push]) in many hand-maintained copies; a change to that order in `run.go` requires updating all ~16 in lockstep, and a missed one silently mis-scripts a FakeRunner rather than failing obviously. This is the runner-scripting half of the test harness that escaped the cycle-1 task 6-4 consolidation (which centralized the `editorDeps`/`editorDepsOptions` Deps builders and the `gitInvocationsOf`/`gitVerbInvocations` filters — those are already well-factored and out of scope here).

**Solution**: Extract a single shared named-step scripting vocabulary into the existing shared test file (`internal/commit/run_test.go`, alongside `seedDiffThenCommit`). Give the recurring `ScriptedCall`s named constructors — e.g. `scPreflightNonEmpty()`, `scL1Diff(diff)`, `scEditorResolve(editor)`, `scLsFilesEmpty()`, `scAdd()`, `scCommit()`, `scPush()`, `scPushFail(stderr)`. Each `seed*` helper then composes its sequence from these (`r.SeedSequence("git", scPreflightNonEmpty(), scL1Diff(diff), scCommit(), scPush())`). Do NOT collapse the helpers into one mega-builder with flags — that trades duplication for a long parameter list. The goal is a shared step vocabulary that spells each git step, its result, and the call-order knowledge once, while keeping each scenario's distinct call shape visible.

**Outcome**: The git call-order vocabulary and the repeated preflight/editor-resolve/commit literals live in one place. Each `seed*` helper reads as a composition of named steps, the per-test variation stays explicit, and a future change to the production call order is a single-site edit to the step constructors rather than ~16 hand-updates.

**Do**:
1. In `internal/commit/run_test.go`, add named `ScriptedCall` constructor helpers for each recurring git step: `scPreflightNonEmpty()`, `scL1Diff(diff string)`, `scLsFilesEmpty()`, `scEditorResolve(editor string)`, `scAdd()`, `scCommit()`, `scPush()`, `scPushFail(stderr string)`. Each returns the exact `runner.ScriptedCall` (including its trailing `// git ...` comment as the canonical record of which git call it scripts) currently spelled inline.
2. Rewrite each `seed*` helper to compose its `SeedSequence("git", ...)` from these constructors instead of inline literals: `internal/commit/run_test.go:39` (seedDiffThenCommit), `internal/commit/run_push_test.go:25` (seedDiffThenCommitThenPush), `internal/commit/run_push_fail_test.go:41` (seedDiffThenCommitThenFailedPush), `internal/commit/run_oversized_test.go:70,85` (seedOversizedFallback, seedPassesToL2), `internal/commit/run_noai_test.go:45` (seedNoAIDefault), `internal/commit/run_aifail_test.go:50` (seedAIFailFallback), `internal/commit/run_edit_test.go:28` (seedEditThenAccept), `internal/commit/run_edit_nolaunch_test.go:20,37` (seedEditNoEditorThenAccept, seedEditMissingBinaryThenAccept), `internal/commit/run_regen_test.go:23` (seedRegenThenAccept), `internal/commit/run_regen_fallback_test.go:67` (seedRegenFailFallback), `internal/commit/run_editor_push_test.go:37` (seedNoAIDefaultThenPush), `internal/commit/staging_defer_test.go:35,50` (seedAllModeThenStageAndCommit, seedAddAllModeThenStageAndCommit), `internal/commit/run_failloud_test.go:64,75` (seedPreflightOnly, seedAIPreflightOnly).
3. Preserve each helper's existing distinct call SHAPE exactly — only the spelling moves to the shared constructors. Do not change any scenario's call set or order.
4. Confirm no production file (`run.go`, `preflight.go`, `source.go`, etc.) is touched — this is a test-only refactor.

**Acceptance Criteria**:
- The non-empty preflight read, editor resolution, and bare commit literals are each spelled exactly once (in their constructor) rather than copy-pasted across files.
- Every existing `seed*` helper composes from the shared step constructors and scripts the same git call set/order it did before.
- No production code changes; the diff is confined to `internal/commit/*_test.go`.
- A change to the production git call order would require editing only the step constructors, not each `seed*` helper.

**Tests**:
- `go test ./internal/commit/...` passes unchanged (the refactor is behaviour-preserving; the existing suite is the regression guard).
- `go vet ./internal/commit/...` is clean.

## Task 2: Eliminate the parallel test-only probe-argv builders that diverge from production probeArgs
status: skipped
severity: low
sources: architecture

**Problem**: `stagedProbeArgs` / `trackedProbeArgs` / `untrackedProbeArgs` live in production code (`internal/commit/preflight.go:131-148`) but are called ONLY from tests (`source_test.go`). Production's emptiness path goes through `probeArgs(spec, diffExclude)` inside `wouldStageNothing` (`internal/commit/preflight.go:113-118`), never these three. The wrappers also use a DIFFERENT parameter convention from production: they take already-mapped exclude pathspecs and append them directly, whereas `probeArgs` takes the raw `diffExclude` globs and re-derives the tail via `excludePathspecs`. This is a second, parallel place the probe argv is assembled — reintroducing the "two hand-aligned copies" that `source.go` single-sourcing was designed to eliminate, now as a test-facing surface. Their own doc comment concedes the divergence ("They take the pre-mapped excludes for symmetry with their historical callers"). If a future change to `probeArgs`/`nameOnly` is made, these builders would silently drift from production and the tests asserting against them would pass while exercising a stale argv shape — eroding the single-source invariant they appear to validate. Context: cycle-2 task 7-1 intentionally added test-facing probe-argv single-checkables, and the genuinely structural tests (`TestEmptinessVerdictAgreesWithL1Source`, `TestProbeArgv_IsL1SourceArgvPlusNameOnly` via `sourceArgs`) already prove the invariant against the shared base builders — so the per-mode wrappers add maintenance surface without adding coverage the shared-base assertions don't.

**Solution**: Remove the parallel derivation. Either (a) delete the three per-mode wrappers and have `source_test.go` assert against `probeArgs(spec, diffExclude)` directly (the production entry point, with the raw `diffExclude` convention), or (b) if a named per-mode checkable is still wanted, make the three wrappers one-line forwarders to `probeArgs` over the matching `sourceSpec` so there is exactly one derivation. Do NOT keep a parallel builder with a divergent parameter convention.

**Outcome**: There is exactly one place the probe argv is derived (`probeArgs`). The per-mode test assertions exercise the production derivation, so a future change to `probeArgs`/`nameOnly`/the exclusion tail cannot let the test argv shape silently drift from production. The single-source invariant the cycle-2 single-checkables nominally guard is actually enforced.

**Do**:
1. Decide between (a) delete-and-assert-directly or (b) one-line-forwarders. Prefer (b) if per-mode named checkables are still valued by the suite; prefer (a) if the shared-base structural tests already cover the per-mode cases.
2. If (b): rewrite `stagedProbeArgs`/`trackedProbeArgs`/`untrackedProbeArgs` (`internal/commit/preflight.go:131-148`) so each is a one-line forwarder to `probeArgs` over the matching `sourceSpec`, taking the raw `diffExclude` convention (matching production) rather than pre-mapped excludes. Update their doc comment to remove the "pre-mapped excludes for symmetry with historical callers" divergence note.
3. If (a): delete `stagedProbeArgs`/`trackedProbeArgs`/`untrackedProbeArgs` from `preflight.go` and update the `source_test.go` callers to assert against `probeArgs(spec, diffExclude)` directly.
4. Update any `source_test.go` call sites to the chosen convention so they pass raw `diffExclude` globs (not pre-mapped pathspecs).
5. Keep the structural tests `TestEmptinessVerdictAgreesWithL1Source` and `TestProbeArgv_IsL1SourceArgvPlusNameOnly` intact — they remain the primary invariant guard.

**Acceptance Criteria**:
- No production function assembles the probe argv with a parameter convention different from `probeArgs`; the per-mode argv is derived through `probeArgs` (or the wrappers are gone entirely).
- `source_test.go` exercises the production probe derivation, not a parallel copy.
- A future change to `probeArgs`/`nameOnly`/the exclusion tail propagates to the per-mode assertions automatically rather than requiring a hand-aligned second edit.
- The single-source invariant tests continue to pass.

**Tests**:
- `go test ./internal/commit/...` passes; the per-mode probe assertions in `source_test.go` still pass against the production derivation.
- `TestProbeArgv_IsL1SourceArgvPlusNameOnly` and `TestEmptinessVerdictAgreesWithL1Source` pass unchanged.
- `go vet ./internal/commit/...` is clean.
