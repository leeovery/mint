package engine

import (
	"slices"
	"testing"

	"mint/internal/hooks"
	"mint/internal/version"
)

// TestHookBump_MapsExplicit proves the engine maps an explicit --set-version run to
// MINT_BUMP=explicit: version.BumpExplicit must render to hooks.BumpExplicit so a
// pinned version is distinguishable in the hook env from a computed patch/minor/major.
func TestHookBump_MapsExplicit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bump version.Bump
		want hooks.Bump
	}{
		{name: "explicit", bump: version.BumpExplicit, want: hooks.BumpExplicit},
		{name: "patch", bump: version.BumpPatch, want: hooks.BumpPatch},
		{name: "minor", bump: version.BumpMinor, want: hooks.BumpMinor},
		{name: "major", bump: version.BumpMajor, want: hooks.BumpMajor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hookBump(tt.bump); got != tt.want {
				t.Errorf("hookBump(%v) = %q, want %q", tt.bump, got, tt.want)
			}
		})
	}
}

// TestBuildHookEnv_DryRunRendersMintDryRun proves the engine's buildHookEnv threads
// the dryRun flag into the assembled HookEnv so MINT_DRY_RUN renders "1" under
// dry-run and "0" otherwise. Because hooks are SKIPPED under dry-run the env is
// never consumed by a real hook run, so the builder is asserted directly — this is
// the engine-level guarantee that the env reflects the run mode.
func TestBuildHookEnv_DryRunRendersMintDryRun(t *testing.T) {
	t.Parallel()

	current := version.SemVer{Major: 0, Minor: 0, Patch: 0}
	next := version.SemVer{Major: 0, Minor: 0, Patch: 1}

	dry := buildHookEnv(current, next, "v0.0.1", version.BumpPatch, true).Render()
	if !slices.Contains(dry, "MINT_DRY_RUN=1") {
		t.Errorf("dry-run env = %v, want it to contain MINT_DRY_RUN=1", dry)
	}

	wet := buildHookEnv(current, next, "v0.0.1", version.BumpPatch, false).Render()
	if !slices.Contains(wet, "MINT_DRY_RUN=0") {
		t.Errorf("non-dry-run env = %v, want it to contain MINT_DRY_RUN=0", wet)
	}
}
