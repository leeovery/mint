TASK: mint-release-tool-4-5 — Any-branch escape hatch (--any-branch) branch-gate bypass

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/preflight/preflight.go:73-83 RunLocalGates guards CheckOnBranch behind `if !anyBranch`; engine/release.go warn + runPreflight threads anyBranch through; warnAnyBranchBypass; AnyBranch option; CLI wiring in cmd/mint/flags.go. Clean skip — gate not evaluated, no abbrev-ref probe issued (matches spec's "not evaluated"). gh-auth runs separately, untouched. Warn rides existing non-failure Warn seam.

TESTS:
- Status: Adequate. release_anybranch_test.go: off-branch+flag proceeds + no abbrev-ref probe + bypass Warn recorded; off-branch without flag aborts + abbrev-ref WAS evaluated + no mutation; on-branch+flag no-effect; clean-tree/tag-free-local/remote-sync each still abort under the flag. preflight_test.go asserts exactly-two local-gate calls under anyBranch + clean-tree still fails. flags_test.go thread-through.

CODE QUALITY:
- Followed conventions (accept-interfaces seam, scriptable runner, presenter-only, GateError typing). SOLID good — single narrow boolean threaded through one decision point. Low complexity, clear doc comments stating skip-not-relax.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release_anybranch_test.go — no explicit test that gh-auth still gates under --any-branch (criterion names gh auth alongside other gates). Covered implicitly by the happy-path gh seed; a dedicated off-branch+flag+unauthenticated-gh-aborts test would close the criterion symmetrically with the clean-tree/tag-free/remote-sync trio.
