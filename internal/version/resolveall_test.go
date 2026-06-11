package version_test

import (
	"errors"
	"testing"

	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins the regenerate `--all` enumeration: ResolveAllRegenerateTargets
// returns a Resolution for EVERY matching tag, ordered OLDEST → NEWEST, reusing the
// SAME prefixed grammar + numeric sort as the single-version resolve (no second
// parser/sorter). The oldest version is the first-release; every later version
// carries its numerically-previous matching tag as the fresh diff base.

// TestResolveAllRegenerateTargets_OrdersOldestToNewest confirms the enumeration is
// ordered oldest → newest by NUMERIC precedence (not lexical), so a double-digit
// minor sorts after its single-digit siblings.
func TestResolveAllRegenerateTargets_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()

	// Seeded out of order, with a double-digit minor that must sort numerically.
	r := seedTags(t, "v2.0.0", "v1.0.0", "v1.10.0", "v1.9.0")

	got, err := version.ResolveAllRegenerateTargets(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("ResolveAllRegenerateTargets returned unexpected error: %v", err)
	}

	wantTags := []string{"v1.0.0", "v1.9.0", "v1.10.0", "v2.0.0"}
	if len(got) != len(wantTags) {
		t.Fatalf("resolved %d versions, want %d", len(got), len(wantTags))
	}
	for i, want := range wantTags {
		if got[i].Tag != want {
			t.Errorf("position %d Tag = %q, want %q (oldest → newest, numeric)", i, got[i].Tag, want)
		}
	}
}

// TestResolveAllRegenerateTargets_OldestIsFirstRelease confirms the oldest version
// is marked FirstRelease (no predecessor, empty diff range), and every later
// version carries its numerically-previous matching tag as the fresh diff base.
func TestResolveAllRegenerateTargets_OldestIsFirstRelease(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v0.1.0", "v0.2.0", "v1.0.0")

	got, err := version.ResolveAllRegenerateTargets(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("ResolveAllRegenerateTargets returned unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("resolved %d versions, want 3", len(got))
	}

	// Oldest: first-release, no predecessor, empty range.
	if !got[0].FirstRelease {
		t.Errorf("oldest FirstRelease = false, want true")
	}
	if got[0].PreviousTag != "" {
		t.Errorf("oldest PreviousTag = %q, want empty", got[0].PreviousTag)
	}
	if got[0].DiffRange() != "" {
		t.Errorf("oldest DiffRange() = %q, want empty", got[0].DiffRange())
	}

	// Each later version chains off its predecessor as the fresh diff base.
	if got[1].FirstRelease {
		t.Errorf("v0.2.0 FirstRelease = true, want false")
	}
	if got[1].PreviousTag != "v0.1.0" {
		t.Errorf("v0.2.0 PreviousTag = %q, want %q", got[1].PreviousTag, "v0.1.0")
	}
	if got[1].DiffRange() != "v0.1.0..v0.2.0" {
		t.Errorf("v0.2.0 DiffRange() = %q, want %q", got[1].DiffRange(), "v0.1.0..v0.2.0")
	}
	if got[2].PreviousTag != "v0.2.0" {
		t.Errorf("v1.0.0 PreviousTag = %q, want %q", got[2].PreviousTag, "v0.2.0")
	}
}

// TestResolveAllRegenerateTargets_IgnoresNonMatchingTags confirms tags that do not
// match the configured grammar/prefix (a wrong prefix, a non-semver tag, a 4-part
// shape, a pre-release) are dropped — reusing the same grammar as the single resolve.
func TestResolveAllRegenerateTargets_IgnoresNonMatchingTags(t *testing.T) {
	t.Parallel()

	r := seedTags(t,
		"v1.0.0",
		"v2.0.0",
		"nightly",
		"other/v1.5.0",
		"v1.5.0.4",
		"v1.5.0-rc.1",
		"release-1.5.0",
	)

	got, err := version.ResolveAllRegenerateTargets(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("ResolveAllRegenerateTargets returned unexpected error: %v", err)
	}

	wantTags := []string{"v1.0.0", "v2.0.0"}
	if len(got) != len(wantTags) {
		t.Fatalf("resolved %d versions, want %d (non-matching tags ignored)", len(got), len(wantTags))
	}
	for i, want := range wantTags {
		if got[i].Tag != want {
			t.Errorf("position %d Tag = %q, want %q", i, got[i].Tag, want)
		}
	}
}

// TestResolveAllRegenerateTargets_MonorepoPrefix confirms a component/monorepo
// prefix is honoured in both matching and re-applying the canonical tag.
func TestResolveAllRegenerateTargets_MonorepoPrefix(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "pkg-name/v1.2.3", "pkg-name/v1.2.0", "other/v9.9.9")

	got, err := version.ResolveAllRegenerateTargets(t.Context(), r, "pkg-name/v")
	if err != nil {
		t.Fatalf("ResolveAllRegenerateTargets returned unexpected error: %v", err)
	}

	wantTags := []string{"pkg-name/v1.2.0", "pkg-name/v1.2.3"}
	if len(got) != len(wantTags) {
		t.Fatalf("resolved %d versions, want %d", len(got), len(wantTags))
	}
	for i, want := range wantTags {
		if got[i].Tag != want {
			t.Errorf("position %d Tag = %q, want %q", i, got[i].Tag, want)
		}
	}
	if got[1].PreviousTag != "pkg-name/v1.2.0" {
		t.Errorf("PreviousTag = %q, want %q", got[1].PreviousTag, "pkg-name/v1.2.0")
	}
}

// TestResolveAllRegenerateTargets_NoTags_ReturnsEmpty confirms a repo with no
// matching tags yields an empty slice (the batch loop processes nothing), not an
// error.
func TestResolveAllRegenerateTargets_NoTags_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "nightly", "other/v1.0.0")

	got, err := version.ResolveAllRegenerateTargets(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("ResolveAllRegenerateTargets returned unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("resolved %d versions, want 0 for a repo with no matching tags", len(got))
	}
}

// TestResolveAllRegenerateTargets_ListFailureSurfaces confirms a `git tag --list`
// failure is surfaced (not masked as an empty set).
func TestResolveAllRegenerateTargets_ListFailureSurfaces(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{ExitCode: 1}, errors.New("git boom"))

	if _, err := version.ResolveAllRegenerateTargets(t.Context(), r, "v"); err == nil {
		t.Fatalf("ResolveAllRegenerateTargets returned nil error, want the list failure surfaced")
	}
}
