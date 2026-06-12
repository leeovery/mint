TASK: 6-2 — Rename/redocument the Committer seam to reflect it is the lock-resilient sink for ALL commit mutations (stage, commit, push)

ACCEPTANCE CRITERIA:
- The interface doc and the Deps field doc both state the seam handles staging, commit, and push (not commit only).
- If renamed, the type is Mutator and all references (interface, Deps field, cmd-layer wiring, tests) compile and pass.
- No behavioural change: the same three mutations still route through the seam with the lock wrapper.
- go vet, golangci-lint, and `go test ./internal/commit/...` pass clean.

STATUS: Complete

SPEC CONTEXT: The spec mandates git_safe as non-negotiable — every git mutation must flow through the lock-resilient wrapper. Commit makes three distinct mutations: deferred staging (`git add -u`/`-A`, Staging Model), the commit (`git commit -F -`, Commit Flow step 5), and the optional push (`git push`, Auto-push). All three must be lock-resilient. The seam's release-engine equivalent is named Mutator (engine.ReleaseDeps.Mutator), and this task aligns commit's seam name + docs to that as-built whole-verb scope rather than the stale Phase-1 commit-only framing. No spec behaviour change is implied — this is a naming/documentation correction.

IMPLEMENTATION:
- Status: Implemented (rename chosen over doc-only — the stronger option the task preferred)
- Location:
  - internal/commit/run.go:142-164 — interface renamed Committer → Mutator; doc now enumerates "staging (`git add -u`/`git add -A`, stageForMode), the commit itself (`git commit -F -`, createCommit), and the auto-push (`git push`, pushAfterCommit)" and states "It is named to mirror engine.ReleaseDeps.Mutator."
  - internal/commit/run.go:179-183 — Deps.Committer field renamed Deps.Mutator; doc now reads "ALL of commit's git mutations flow through it — staging (`git add`), the commit (`git commit -F -`), and the auto-push (`git push`)".
  - internal/commit/run.go:742, :761, :848 — all three call sites use deps.Mutator.Mutate (stage / commit / push).
  - cmd/mint/main.go:155, :274, :332 — cmd-layer wiring uses Mutator: git.NewMutator(r).
  - Tests construct Deps with the Mutator field (run_test.go, run_push_test.go, run_failloud_test.go, run_regen_test.go, staging_defer_test.go, etc.); compile-time assertion var _ commit.Mutator = (*git.Mutator)(nil) at run_test.go:25.
  - File-header comment (run.go:22-25) already references the Mutator seam consistently.
- Notes: Zero residual "Committer" references in internal/ or cmd/ (grep clean). The mirror claim is accurate — engine.ReleaseDeps has Mutator *git.Mutator (release.go:113). The one remaining "bare commit path" phrase (run.go:170) is on the Deps struct doc describing what the struct carries, not the seam's scope, and is correct/out of scope. Landed in commit 2160efc.

TESTS:
- Status: Adequate
- Coverage: The task is a mechanical rename + non-behavioural doc edit; the acceptance criteria explicitly require no new behavioural test, with correctness preserved by existing end-to-end coverage. The compile-time assertion (run.go:25 in test) directly guards the contract that *git.Mutator satisfies commit.Mutator. The seam's three routes remain exercised by existing tests: staging via staging_defer_test.go, commit via run_test.go, push via run_push_test.go (which asserts a lock-contended push retries through the wrapper). All Deps builders were updated to the new field name, so the suite compiling is itself the rename's correctness proof.
- Notes: No under-testing — the rename has no new behaviour to cover and the existing matrix already drives stage/commit/push through the seam. No over-testing introduced.

CODE QUALITY:
- Project conventions: Followed. Interface defined at the point of consumption (golang-structs-interfaces), name now mirrors the engine's Mutator (golang-naming consistency across the two verbs), Mutate signature unchanged and matches git.Mutator.Mutate.
- SOLID principles: Good. Single lock-resilient mutation sink; the rename strengthens interface-segregation clarity (one seam, one responsibility, accurately named).
- Complexity: Low. Pure rename + doc edit, no control-flow change.
- Modern idioms: Yes. Consumer-defined narrow interface, raw-bytes stdin for retry-safe re-piping.
- Readability: Good — improved. The docs now match the as-built scope, removing the misleading commit-only framing that could have invited a future parallel stage/push path bypassing the wrapper (the exact risk the task called out).
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
