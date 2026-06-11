package main

import "mint/internal/engine"

// regenerateGateSet maps a RESOLVED regenerate target surface to the engine's
// preflight gate-set selection (task 5-4). It encodes the spec's general rule off
// the target: a provider-writing target (release/both) selects gh-auth
// (CallsProvider); a changelog-committing target (changelog/both) selects the
// clean-tree + on-branch + remote-sync bucket (CommitsAndPushes). targetUnset and
// any unexpected value select nothing — the gates are opt-in by surface, never
// run speculatively.
//
// --reuse is resolved to --target release by validateRegenerateRequest before this
// runs, so the --reuse run maps to provider-only (gh-auth) here, exactly as the
// spec requires (it must run gh-auth even though it never commits/pushes).
func regenerateGateSet(target regenerateTarget) engine.RegenerateGateSet {
	return engine.RegenerateGateSet{
		CallsProvider:    target == targetRelease || target == targetBoth,
		CommitsAndPushes: target == targetChangelog || target == targetBoth,
	}
}
