TASK: mint-release-tool-5-4 — Regenerate preflight subset per verb

STATUS: Complete (5-4's own acceptance criteria — phrased on the resolved request — are fully met)

IMPLEMENTATION:
- Status: Implemented. Engine selector RegeneratePreflight (internal/engine/regenerate.go:65-92); resolved-fact RegenerateGateSet (:43-50); cmd mapping regenerateGateSet (cmd/mint/regenerate_preflight.go:16-21); wired at cmd/mint/main.go:129. Genuine gate reuse — calls forward preflight.CheckCleanTree/CheckOnBranch/Fetch/CheckRemoteSync/CheckGhAuth, same surface/"preflight" stage. Tag-free is unrepresentable in the gate set (no field can enable it). Local-then-network ordering mirrors forward. --reuse→targetRelease resolution in regenerate_validate.go.

TESTS:
- Status: Adequate. regenerate_preflight_test.go (8 tests) asserts exact git/gh argv; cmd/mint/regenerate_preflight_test.go table-tests target→gate-set mapping. Covers reuse/release gh-auth-only, fresh-changelog commit-push-gates-without-gh-auth, fresh-both all-applicable, never-tag-free (all 3 modes), no-version-compute, two abort-cleanly cases (gh-auth fail on reuse, dirty-tree on fresh-changelog incl. short-circuit before remote-sync).

CODE QUALITY:
- Followed conventions (runner seam, typed *GateError/AbortError, StageFailed, table tests). SOLID/DRY good — selector is pure mapping; gates reused not re-implemented. Low complexity, excellent readability.

BLOCKING ISSUES:
- None (per 5-4's own acceptance criteria). See the non-blocking [bug] — it is a real integration safety gap arguably owned by the 5-10 interactive wiring.

NON-BLOCKING NOTES:
- [bug] cmd/mint/main.go:129 — Preflight runs against validated.Target, which stays targetUnset for an interactive fresh run (no -y, no --target; regenerate_validate.go:48-50 deliberately leaves it unset for the 5-10 prompt). regenerateGateSet(targetUnset) returns {false,false} → ZERO gates run. The target is then resolved INSIDE RegenerateRun (regenerate_interactive.go:146/resolveTarget) AFTER preflight, so an interactively-chosen changelog/both commits+pushes with no clean-tree/on-branch/remote-sync, and a chosen release/both writes the provider with no gh-auth. Real heal-path safety hole. Fix: re-run RegeneratePreflight after the interactive axes resolve, or move the preflight call after ResolveRegenerateAxes. (5-4's ACs are phrased on the resolved request and are met; the integration leaves them unenforced for the deferred-target case.)
- [do-now] internal/engine/regenerate.go:5 — file header self-reference ("regenerate_preflight subset selector") disagrees with the filename regenerate.go; align so doc and filename agree.
