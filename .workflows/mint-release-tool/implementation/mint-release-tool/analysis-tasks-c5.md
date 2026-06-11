---
topic: mint-release-tool
cycle: 5
total_proposed: 1
---
# Analysis Tasks: mint-release-tool (Cycle 5)

## Task 1: Gate regenerate gh-auth on the resolved publisher, not the bare target
status: pending
severity: medium
sources: architecture

**Problem**: The regenerate gh-auth preflight gate is selected purely from the resolved target via `regenerateGateSet(target)` (internal/engine/regenerate.go:59-64), which sets `CallsProvider: target.writesProvider()` with no knowledge of whether the publisher actually resolved. The cmd layer already resolves (and on a non-github / no-remote origin downgrades to a nil publisher with a warn) in `resolveRegeneratePublisher` BEFORE dispatch, and threads that resolved `publisher` into both `RegenerateRun` (internal/engine/regenerate_interactive.go:146) and `RegenerateAllValidated` (internal/engine/regenerate_batch.go:112). But the gate set is re-derived from the bare target at internal/engine/regenerate_interactive.go:164 and internal/engine/regenerate_batch.go:126, so a downgraded `regenerate --reuse` / `--target release` run on a non-github origin first warns "downgrade, provider skipped" and THEN `RegeneratePreflight` still runs `CheckGhAuth` and can abort the run — even though the provider write is nil-guarded and skipped anyway. This is the OPPOSITE of the forward spine, which runs `CheckGhAuth` only `if publisher != nil` (internal/engine/release.go:570), silently skipping the gh-auth gate on a downgrade so the run is never stranded on a gh-auth failure for a release it was never going to publish. The two verbs handle an unresolvable provider with opposite outcomes; the spec's "reuse must run gh-auth" rationale assumes a resolvable provider, and on a downgraded run there is no provider to authenticate against.

**Solution**: Thread the resolved-publisher presence into the regenerate gate-set selection so `CallsProvider` is false on a downgraded run, mirroring the forward path's `if publisher != nil` guard. Derive `CallsProvider` from `target.writesProvider() && publisher != nil` at the two preflight call sites, which already have the resolved `publisher` in scope. Do NOT re-derive the gate set from the bare target inside `RegenerateRun` / `RegenerateAllValidated`.

**Outcome**: A downgraded regenerate run (nil publisher) never runs the gh-auth gate, so it can no longer abort on a gh-auth failure for a provider write that will be skipped anyway — the regenerate and forward verbs handle an unresolvable provider identically (both warn-and-downgrade, neither stranded on gh-auth). A run WITH a resolved publisher still runs gh-auth exactly as before, and the changelog commit/push gate bucket (`CommitsAndPushes`) is unchanged.

**Do**:
1. Extend the gate-set selector in internal/engine/regenerate.go so the `CallsProvider` decision can incorporate publisher presence. Either give `regenerateGateSet` a `publisherResolved bool` parameter and compute `CallsProvider: target.writesProvider() && publisherResolved`, or add a sibling selector that takes the publisher; keep `CommitsAndPushes: target.writesChangelog()` unchanged. Update the doc comment to state that the gh-auth gate now requires both a provider-writing target AND a resolved publisher, matching the forward spine's `if publisher != nil` guard.
2. Update the call site in internal/engine/regenerate_interactive.go:164 (`RegenerateRun`) to pass `publisher != nil` (using the `publisher` parameter already in scope) into the gate-set construction.
3. Update the call site in internal/engine/regenerate_batch.go:126 (`RegenerateAllValidated`) to pass `publisher != nil` (using the `publisher` parameter already in scope) into the gate-set construction.
4. Adjust the existing comments at both call sites that describe the gate-set derivation to note the publisher-presence guard, so the rationale stays accurate.
5. Confirm no other caller of the selector (interactive flow, batch flow) needs updating, and that the engine-side `RegenerateGateSet` struct and `RegeneratePreflight` gate ordering are otherwise untouched.

**Acceptance Criteria**:
- On a downgraded run (publisher resolves to nil after the warn) with a provider-writing target (`release` / `both` / any `--reuse`), `RegeneratePreflight` does NOT run `CheckGhAuth`.
- On a resolved-publisher run with a provider-writing target, `RegeneratePreflight` still runs `CheckGhAuth` exactly as today.
- The `CommitsAndPushes` gate bucket (clean-tree + on-branch + remote-sync) is selected solely from `target.writesChangelog()` and is unaffected by publisher presence.
- The downgrade behaviour for an unresolvable provider is now identical across the forward (`engine.Release`) and regenerate (`RegenerateRun` / `RegenerateAllValidated`) verbs: warn-and-downgrade, gh-auth skipped.
- A changelog-only (`--target changelog`) run's gate selection is unchanged regardless of publisher presence.

**Tests**:
- Add a test proving a provider-writing target with a NIL publisher selects a gate set with `CallsProvider == false` (gh-auth skipped), parallel to the existing forward-path downgrade coverage.
- Add a test proving a provider-writing target with a NON-nil publisher still selects `CallsProvider == true` (gh-auth runs).
- Add/extend a regenerate-flow test proving a downgraded `--reuse` / `--target release` run on a non-github / no-remote origin completes WITHOUT aborting on gh-auth (a failing gh-auth recorder must not be reached on the downgrade path).
- Keep the existing `RegeneratePreflight` gate tests (regenerate_preflight_test.go) green — the gate IMPLEMENTATION and ordering are unchanged; only the SELECTION input changes.
- Confirm the changelog-only and `both` target gate selections are unchanged by the publisher-presence guard.
