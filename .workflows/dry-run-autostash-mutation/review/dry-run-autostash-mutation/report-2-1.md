TASK: dry-run-autostash-mutation-2-1 — Consolidate The Redundant skipCleanTree Preflight Tests (tick-0941ed)

ACCEPTANCE CRITERIA:
1. No test in internal/preflight/preflight_test.go re-runs the RunLocalGates(..., skipCleanTree=true) fixture with a strictly weaker set of assertions than another test.
2. The surviving skipCleanTree test still proves: clean-tree probe (git status --porcelain) is absent, the branch and tag-free gates still run, and the run passes.
3. go build ./..., gofmt -l . (empty), go vet ./..., go test -race ./..., golangci-lint run (0 issues) all pass.

STATUS: Complete

SPEC CONTEXT:
The dry-run-autostash-mutation change skips the real autostash under DryRun && AutoStash and, because autostash's only job is to clean the tree for the clean-tree gate, also bypasses the clean-tree gate in that combo (mirroring the existing --any-branch skip). Phase 1 (commit b1396e9) added the skipCleanTree parameter to RunLocalGates and two new tests. The spec's preflight verification clause (specification.md:65-67) asks for ONE behaviour: "RunLocalGates(..., skipCleanTree=true) does not run the clean-tree probe (git status --porcelain absent) while the branch and tag-free gates still run; skipCleanTree=false unchanged." Phase 1 expressed that with two tests; this analysis-cleanup task removes the redundant one.

IMPLEMENTATION:
- Status: Implemented (matches the planned "preferred" remediation: remove the redundant test).
- Location: internal/preflight/preflight_test.go:262-288 (surviving test TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate). Removal applied in commit 49e8916.
- Verification of the duplication claim (confirmed against git history):
  - Phase-1 commit b1396e9 added TWO tests over a byte-identical fixture and an identical call:
      * Fixture (both): argRunner seeded only with "rev-parse --abbrev-ref HEAD" -> "main\n" and "rev-parse -q --verify refs/tags/v1.2.3" -> {ExitCode:1, errExit}. No "status --porcelain" seeded.
      * Call (both): preflight.RunLocalGates(t.Context(), r, "main", "v1.2.3", false, true).
      * TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate asserted: nil pass + porcelain-probe absent + exactly 2 recorded calls.
      * TestRunLocalGates_SkipCleanTree_DirtyTreeDoesNotAbort asserted: nil pass ONLY.
    => The removed test's assertion set (nil pass) is a strict subset of the survivor's (nil pass + porcelain absent + 2-call count), over an identical fixture/call. It proved nothing the survivor did not. Claim verified.
  - Task commit 49e8916 deletes exactly TestRunLocalGates_SkipCleanTree_DirtyTreeDoesNotAbort (preflight_test.go diff: -16 lines, the whole function and its trailing blank line) and changes nothing else in the file. Clean, surgical removal.
- Note on the removed test's comment ("a DIRTY tree must NOT abort"): the deleted test did not actually model a dirty tree — its fixture seeded no porcelain response at all, identical to the survivor's. So the "dirty tree" framing was never a distinguishing condition (the porcelain probe is never issued under skipCleanTree, so no dirty status is ever read). Removing it loses no real coverage; the dirty-tree-does-not-abort behaviour is covered at the engine level per the spec (specification.md:62-64 / Phase 1 engine test). No drift.

TESTS:
- Status: Adequate (this task IS a test-consolidation task; correctness = the right test survives intact with full assertions).
- Coverage: The sole surviving skipCleanTree test (TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate, lines 262-288) proves all three required facts:
    * clean-tree probe absent: loop at 279-283 fails on any "status --porcelain" call.
    * branch + tag-free gates still run: len(r.calls) == 2 at 285-287 (the on-branch and tag-free probes), and the fixture only answers those two argv keys.
    * run passes: nil-error check at 274-276.
  This is the exact behaviour the spec asks for at specification.md:65-67. Survivor untouched by the task commit (confirmed: 49e8916 diff shows only the deletion).
- No other strict-subset duplication remains. The five RunLocalGates call sites are each distinct:
    * TestRunLocalGates_AllPass (false,false) — happy path, all three gates seeded.
    * TestRunLocalGates_AnyBranch_SkipsOnBranchGate (true,false) — distinct abbrev-ref-absent + 2-call assertions.
    * TestRunLocalGates_AnyBranch_StillRunsCleanTreeGate (true,false) — dirty tree, GateError abort assertion (different fixture: porcelain dirty).
    * TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate (false,true) — the lone skipCleanTree test.
    * TestRunLocalGates_CheapFirstAbort (false,false) — dirty tree, abort-after-exactly-1-call ordering assertion (different fixture/assertion).
  No two share a fixture-plus-call where one's assertions are a subset of the other's. Criterion 1 satisfied.
- Not over-tested: the consolidation removes the only redundancy; remaining tests each pin a distinct behaviour with exact-argv / exact-count assertions per the CLAUDE.md test idioms.

CODE QUALITY:
- Project conventions: Followed. External test package (preflight_test), t.Parallel() on the survivor (263), exact-argv dispatch via argRunner, exact call-count assertions — all consistent with CLAUDE.md "exact argv + exact rendered lines" and golang-testing "test behaviour, not implementation detail / no redundant assertions" (skill best-practice #5 and the over-test guidance in the review checklist). The removal directly serves golang-testing's "write tests to constrain behavior, not to hit coverage targets."
- SOLID / DRY: DRY improved — the byte-identical-fixture duplicate is gone.
- Complexity: N/A (deletion only).
- Modern idioms: survivor uses t.Context(), errors.As for GateError elsewhere — idiomatic.
- Readability: Good. Surviving test's WHY-comment (265-268) accurately states the dry-run+autostash rationale and is true to as-built.
- Issues: none.

GATES (criterion 3) — NOT EXECUTED:
- My operating rules restrict Bash to the output-file rename only and forbid running the test suite or any build/lint command, so I could not run go build / gofmt / go vet / go test -race / golangci-lint and cannot report their live results. This is a verifier-tooling constraint, not evidence of failure.
- Static assessment of buildability: the task commit ONLY deletes a self-contained test function. It removes no symbol any other code references; the deleted function called preflight.RunLocalGates with the same signature the survivor and the engine caller still use (phase-1 commit b1396e9 already migrated all call sites to the (anyBranch, skipCleanTree) two-bool signature). No dangling references, no unused imports introduced (the deleted body used only runner.Result/errExit/preflight.RunLocalGates, all still used by the survivor). gofmt: the surrounding hunk is gofmt-shaped (tab-indented, aligned map literals). No reason to expect a gate regression from a pure deletion of a compiling, formatted test.
- RECOMMENDATION: the orchestrator (which is not under these Bash restrictions) should run the five project gates once to confirm criterion 3 empirically before sign-off. I have verified everything statically verifiable; only live execution remains.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None. (The removed test's slightly misleading "DIRTY tree" comment is gone with the test, so there is nothing left to correct.)
