package engine

import (
	"slices"
	"testing"

	"mint/internal/version"
)

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
