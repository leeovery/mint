TASK: mint-release-tool-10-1 — Close the Interactive-Regenerate Preflight Gate Bypass (bug remediation)

ACCEPTANCE CRITERIA:
- A bare interactive `changelog` (or `both`) choice runs clean-tree, on-branch, and remote-sync gates before any CHANGELOG commit/push.
- A bare interactive `release` (or `both`) choice runs the gh-auth gate before any provider write.
- A failing applicable gate aborts cleanly before any mutation or network call.
- (Do #4) Tag-free and version-compute gates remain excluded (regenerate cuts no tag).

STATUS: Complete

SPEC CONTEXT:
Spec "Regenerate / Backfill Notes" → "Preflight subset per verb" (specification.md ~546-550): an interactively-chosen changelog/both must run the clean-tree/on-branch/remote-sync bucket before committing+pushing the CHANGELOG; an interactively-chosen release/both must run gh-auth before writing the provider; regenerate never cuts a tag, so the tag-free gate (and version compute) never run. The earlier reports (5-10, 5-4) found the gate set was computed from `targetUnset` at cmd dispatch (zero gates) BEFORE the interactive target resolved, leaving the real write path ungated.

IMPLEMENTATION:
- Status: Implemented (fix present, committed as ec3012d "Tmint-release-tool-10-1 — close interactive-regenerate preflight gate bypass"; refined by 11-1/a982e92).
- Location:
  - internal/engine/regenerate_interactive.go:152-170 — RegenerateRun now resolves both axes via ResolveRegenerateAxes, then runs RegeneratePreflight(ctx, deps, req.ReleaseBranch, regenerateGateSet(target, publisher != nil)) BEFORE body production, mutation, or network write.
  - internal/engine/regenerate.go:52-72 — regenerateGateSet maps the RESOLVED target (+ publisher presence) to the gate set; CallsProvider = writesProvider() && publisherResolved, CommitsAndPushes = writesChangelog(). Tag-free is structurally unrepresentable.
  - internal/engine/regenerate.go:87-114 — RegeneratePreflight runs the forward gate implementations as a subset: clean-tree, on-branch, fetch, remote-sync (commits+pushes bucket), then gh-auth. Never runs tag-free; no version compute.
  - cmd/mint/main.go:122-152 — the cmd layer dispatch was correctly restructured: line 129 now only resolves the release branch (read-only) and threads it into the engine; NO early empty-gate preflight remains. Comments document why preflight runs in the engine after axis resolution. The batch path mirrors this at internal/engine/regenerate_batch.go:131.
  - internal/engine/regenerate_write.go:137-189 — RegenerateWrite runs NO gate of its own, so there is no double-run/double-prompt. Chosen approach = "preflight after axis resolution inside the engine" (Do #1/#2), not the alternative cmd-layer restructure (#3) — and the two are not both present. Plan Do-item #3 (do not leave both an early empty-gate call and a late resolved-gate call) is satisfied.
- Notes: The 11-1 refinement (publisher-resolved guard on gh-auth) goes beyond the original plan text but is correct and consistent with the forward spine's `if publisher != nil` guard — it prevents a dead gh auth from aborting a downgraded run whose provider write is skipped anyway. The `regenerateGateSet` two-arg shape is used identically by single (regenerate_interactive.go:168) and batch (regenerate_batch.go:131) paths.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/regenerate_interactive_test.go:436 TestRegenerateRun_InteractiveChangelog_RunsCommitPushGates — bare run, target resolved to "changelog" at the prompt (TargetUnset); asserts clean-tree, on-branch, remote-sync ran AND precede the changelog `git add` (assertCommitPushGatesBeforeCommit). Covers AC#1.
  - internal/engine/regenerate_interactive_test.go:493 TestRegenerateRun_InteractiveRelease_RunsGhAuthBeforeProviderWrite — bare run, target resolved to "release"; snapshots gh-auth-ran state at dispatch time via pub.beforeDispatch and asserts gh-auth preceded the single provider dispatch. Covers AC#2.
  - internal/engine/regenerate_interactive_test.go:538 TestRegenerateRun_FailingGate_AbortsBeforeMutation — dirty tree on an interactive changelog choice aborts non-zero with no push and zero provider dispatch. Covers AC#3.
  - internal/engine/regenerate_interactive_test.go:582/610 Downgraded-reuse-skips-gh-auth and Resolved-release-runs-gh-auth — cover the 11-1 publisher-presence guard both ways.
  - internal/engine/regenerate_preflight_test.go — unit coverage of the selector/preflight: Reuse_GhAuthOnly, FreshChangelog_CommitPushGates, FreshBoth_AllApplicableGates, FreshRelease_GhAuthOnly, NeverTagFree, NoVersionCompute, plus gate-failure abort cases. Covers Do#4 (tag-free + version-compute exclusion) explicitly.
  - Gate-detection helpers (regenerate_preflight_test.go:33-51) match the real preflight commands exactly (`git status --porcelain`, `git rev-parse --abbrev-ref HEAD`, `git rev-list --left-right --count @{u}...HEAD`, `gh auth status`), so the assertions are genuine, not vacuous.
- Notes: The two plan-required tests both exist and assert via the FakeRunner seam as specified. Ordering is enforced (gate-before-mutation, gate-before-dispatch), not merely presence — this is exactly the right depth for a bypass-fix and would fail if the gate moved back after the write. Not over-tested: the interactive tests cover end-to-end wiring while the preflight unit tests cover the matrix, with minimal overlap. Tag-free exclusion is asserted at the unit level (TestRegeneratePreflight_NeverTagFree) rather than re-asserted in the interactive tests — appropriate, no gap.

CODE QUALITY:
- Project conventions: Followed. FakeRunner seam + RecordingPresenter per golang-testing; gate selector is a pure mapping (no cmd-enum knowledge) per separation of concerns; sentinel/error surfacing via the shared `surface` helper consistent with the forward path.
- SOLID principles: Good. regenerateGateSet is a single-responsibility pure selector; RegeneratePreflight reuses forward gate implementations unchanged (open/closed — selection differs, not implementation); single home for gate selection shared by single + batch paths (DRY).
- Complexity: Low. Linear gate sequence; the only branching is the two boolean buckets.
- Modern idioms: Yes. slices.Equal/Contains in tests, t.Context(), table-free focused tests.
- Readability: Good. Comments are load-bearing and explain WHY preflight moved into the engine (the cmd layer cannot know the interactive target). The RegenerateGateSet doc explicitly notes tag-free is unrepresentable.
- Issues: None.

BLOCKING ISSUES:
- None. The fix is present, committed, correct, and not double-running gates. All three acceptance criteria and Do#4 are satisfied and tested.

NON-BLOCKING NOTES:
- None.
