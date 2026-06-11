package engine

// This file is the engine's REGENERATE preflight subset selector (task 5-4).
// Regenerate's preflight is the SAME forward gate SET (mint/internal/preflight),
// run as a SUBSET driven off the resolved request — it never re-implements a gate.
//
// The general selection rule (spec "Preflight subset per verb"):
//   - calls gh (a provider write — target release or both) → gh-auth.
//   - commits + pushes (target changelog or both) → clean-tree + on-branch +
//     remote-sync.
//   - cuts a new tag → tag-free — NEVER true for regenerate (the target tag is
//     supposed to exist; regenerate never cuts a new tag), so the tag-free gate
//     never runs in any regenerate mode.
//
// There is also NO version compute in any regenerate mode: 5-3 already resolved an
// existing tag, so there is no bump and no `git tag --list` here.

import (
	"context"

	"mint/internal/preflight"
)

// regeneratePreflightStage is the stage label a failing regenerate gate surfaces
// through the presenter — matching the forward path's "preflight" stage so the
// abort narration is consistent across verbs.
const regeneratePreflightStage = "preflight"

// RegenerateGateSet is the RESOLVED preflight selection for one regenerate run,
// derived from the request's target surface. It carries only the two facts that
// drive the subset, so the selector stays a pure mapping with no knowledge of the
// cmd-layer source/target enums:
//
//   - CallsProvider is set when the run writes the provider release (target
//     release or both), which requires the gh-auth gate — even for --reuse, since
//     a dead gh auth is the usual reason you are healing.
//   - CommitsAndPushes is set when the run commits + pushes the changelog (target
//     changelog or both), which requires the clean-tree, on-branch, and
//     remote-sync gates.
//
// The tag-free gate is deliberately UNREPRESENTABLE here: regenerate never cuts a
// tag, so there is no field that could turn it on.
type RegenerateGateSet struct {
	// CallsProvider selects the gh-auth gate: true when the run writes the provider
	// release (target release or both, including every --reuse run).
	CallsProvider bool
	// CommitsAndPushes selects the clean-tree + on-branch + remote-sync gates: true
	// when the run commits + pushes the changelog (target changelog or both).
	CommitsAndPushes bool
}

// regenerateGateSet maps a RESOLVED engine target — together with whether the
// publisher actually RESOLVED — to its preflight gate-set selection. It is the single
// engine-owned gate-set selector, keyed off the engine's RegenerateTarget. Gate
// selection lives here (not in the cmd layer) because the interactive and batch flows
// run preflight AFTER the target resolves, and the cmd layer dispatches before the
// interactive target is known.
//
// The gh-auth gate (CallsProvider) requires BOTH a provider-writing target AND a
// resolved publisher — mirroring the forward spine's `if publisher != nil` guard
// (release.go). On a DOWNGRADED run (the provider could not be resolved on a
// non-github / no-remote origin, so the caller threads a nil publisher and the
// provider write is nil-guarded and skipped) CallsProvider is FALSE, so a dead gh
// auth can no longer abort a run whose provider write will be skipped anyway. The
// changelog commit/push bucket (CommitsAndPushes) is selected SOLELY from the target
// (it touches no provider), so it is unaffected by publisher presence.
func regenerateGateSet(target RegenerateTarget, publisherResolved bool) RegenerateGateSet {
	return RegenerateGateSet{
		CallsProvider:    target.writesProvider() && publisherResolved,
		CommitsAndPushes: target.writesChangelog(),
	}
}

// RegeneratePreflight runs the regenerate preflight SUBSET for the resolved gate
// set, aborting cleanly on the first failing APPLICABLE gate BEFORE any work. It
// reuses the forward gate implementations unchanged — only the SELECTION differs.
//
// Order mirrors the forward path's cheap-local-then-network sequencing: the
// commits + pushes bucket runs the cheap local gates first (clean-tree,
// on-branch), then fetches and runs remote-sync (the network half, fetch first
// exactly as the forward path sequences it); the gh-auth network gate runs last.
// The tag-free gate never runs, and no version is computed.
//
// A failing gate is surfaced through the presenter (StageFailed) and returned as
// the engine's typed non-zero abort, identical to the forward path's pre-mutation
// gate failures — there is nothing to unwind because no work has begun.
func RegeneratePreflight(ctx context.Context, deps ReleaseDeps, releaseBranch string, set RegenerateGateSet) error {
	p := deps.Presenter

	if set.CommitsAndPushes {
		if err := preflight.CheckCleanTree(ctx, deps.Runner); err != nil {
			return surface(p, regeneratePreflightStage, err)
		}
		if err := preflight.CheckOnBranch(ctx, deps.Runner, releaseBranch); err != nil {
			return surface(p, regeneratePreflightStage, err)
		}
		// Remote-sync reads @{u}; fetch first so the upstream refs are current — the
		// same network-gate sequencing the forward path uses.
		if err := preflight.Fetch(ctx, deps.Runner); err != nil {
			return surface(p, regeneratePreflightStage, err)
		}
		if err := preflight.CheckRemoteSync(ctx, deps.Runner, releaseBranch); err != nil {
			return surface(p, regeneratePreflightStage, err)
		}
	}

	if set.CallsProvider {
		if err := preflight.CheckGhAuth(ctx, deps.Runner); err != nil {
			return surface(p, regeneratePreflightStage, err)
		}
	}

	return nil
}
