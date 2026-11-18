TASK: mint-release-tool-2-15 — Abort auto-unwind from the gate (n)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (and hardened by Phase 4 surgical unwind, per the task's deferral note — as-built reflects the converged form). engine/release.go: gate-n routes errGateAborted → Unwind; reviewGate returns clean errGateAborted (emits no Unwound itself); StartState/MadeState captured before the gate; surfaceAndUnwind is the shared pre-push path; unwind.go Unwind + engine-authored Summary incl. "; repo clean" tail. Gate-n and every pre-push failure converge on identical Unwind call; gate sits before any mutation so made is zero in the common case (no-op, no Unwound). abort carries non-zero exit. Success footer suppressed on Unwound.

TESTS:
- Status: Adequate. Gate-n before mutation no-ops; gate-n after pre_tag commit resets tracked commit, deletes no tag, exact summary; gate-n and pre-push failure byte-identical clean state + summary; pre-push failures reset/delete-tag w/ StageFailed→Unwound ordering + no-finish-after-unwound; post-PONR publish failure never unwinds; Unwind unit matrix (0/1/2 commits × tag). All git via FakeRunner.

CODE QUALITY:
- Followed conventions (accept-interfaces, runner/mutator seams, focused tests). SOLID good — Unwind decomposed into single-purpose helpers. Low complexity, errors.Is discrimination, intent-dense doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
