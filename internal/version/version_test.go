package version_test

import (
	"testing"

	"mint/internal/runner"
	"mint/internal/version"
)

// seedTags returns a FakeRunner whose `git` invocation yields the given lines as
// the tag list (one tag per line), mirroring `git tag --list` output.
func seedTags(t *testing.T, tags ...string) *runner.FakeRunner {
	t.Helper()

	r := runner.NewFakeRunner()

	stdout := ""
	for _, tag := range tags {
		stdout += tag + "\n"
	}
	r.Seed("git", runner.Result{Stdout: stdout}, nil)

	return r
}

func TestCurrentVersion_NoTags_ReturnsZero(t *testing.T) {
	t.Parallel()

	r := seedTags(t)

	got, err := version.CurrentVersion(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	want := version.SemVer{Major: 0, Minor: 0, Patch: 0}
	if got != want {
		t.Errorf("CurrentVersion = %+v, want %+v", got, want)
	}
}

func TestCurrentVersion_NoMatchingPrefix_ReturnsZero(t *testing.T) {
	t.Parallel()

	// Tags exist but none carry the configured prefix.
	r := seedTags(t, "1.2.3", "release-2.0.0", "pkg/v1.0.0")

	got, err := version.CurrentVersion(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	want := version.SemVer{Major: 0, Minor: 0, Patch: 0}
	if got != want {
		t.Errorf("CurrentVersion = %+v, want %+v", got, want)
	}
}

func TestCurrentVersion_IgnoresNonConformingTags(t *testing.T) {
	t.Parallel()

	// Every non-conforming shape must be ignored entirely; only the strict
	// 3-part SemVer tag (v1.2.0) should be recognised.
	r := seedTags(t,
		"v1.2",          // too few segments
		"v1.2.0-rc.1",   // pre-release
		"v1.2.0.4",      // 4 segments
		"v1.2.0+build5", // build metadata
		"release-1.2",   // wrong prefix / not semver
		"v1.2.0",        // the only valid tag
	)

	got, err := version.CurrentVersion(t.Context(), r, "v")
	if err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	want := version.SemVer{Major: 1, Minor: 2, Patch: 0}
	if got != want {
		t.Errorf("CurrentVersion = %+v, want %+v", got, want)
	}
}

func TestCurrentVersion_GlobalNumericMax_NotLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []string
		want version.SemVer
	}{
		{
			name: "double-digit major beats single-digit lexically-higher",
			tags: []string{"v9.0.0", "v10.0.0"},
			want: version.SemVer{Major: 10, Minor: 0, Patch: 0},
		},
		{
			name: "double-digit minor beats single-digit lexically-higher",
			tags: []string{"v1.9.0", "v1.10.0"},
			want: version.SemVer{Major: 1, Minor: 10, Patch: 0},
		},
		{
			name: "double-digit patch beats single-digit lexically-higher",
			tags: []string{"v1.0.9", "v1.0.10"},
			want: version.SemVer{Major: 1, Minor: 0, Patch: 10},
		},
		{
			name: "global max across unordered mixed tags",
			tags: []string{"v1.5.0", "v2.0.1", "v2.0.0", "v0.9.9"},
			want: version.SemVer{Major: 2, Minor: 0, Patch: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := seedTags(t, tt.tags...)

			got, err := version.CurrentVersion(t.Context(), r, "v")
			if err != nil {
				t.Fatalf("CurrentVersion returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CurrentVersion = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCurrentVersion_IgnoresOtherPrefixes_MatchesConfigured(t *testing.T) {
	t.Parallel()

	// Only the configured "pkg-name/v" prefix counts; v-prefixed and bare tags
	// (even though they are numerically higher) must be ignored.
	r := seedTags(t,
		"v9.9.9",
		"3.3.3",
		"pkg-name/v1.2.3",
		"pkg-name/v1.0.0",
	)

	got, err := version.CurrentVersion(t.Context(), r, "pkg-name/v")
	if err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	want := version.SemVer{Major: 1, Minor: 2, Patch: 3}
	if got != want {
		t.Errorf("CurrentVersion = %+v, want %+v", got, want)
	}
}

func TestCurrentVersion_EmptyPrefix_MatchesBareSemVer(t *testing.T) {
	t.Parallel()

	// With an empty prefix, bare X.Y.Z tags match and v-prefixed ones do not.
	r := seedTags(t, "v5.0.0", "1.2.3", "0.9.0")

	got, err := version.CurrentVersion(t.Context(), r, "")
	if err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	want := version.SemVer{Major: 1, Minor: 2, Patch: 3}
	if got != want {
		t.Errorf("CurrentVersion = %+v, want %+v", got, want)
	}
}

func TestCurrentVersion_ListsTagsViaRunner(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.0.0")

	if _, err := version.CurrentVersion(t.Context(), r, "v"); err != nil {
		t.Fatalf("CurrentVersion returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("recorded %d invocations, want exactly 1", len(invs))
	}
	if invs[0].Name != "git" {
		t.Errorf("invoked %q, want it to run via git", invs[0].Name)
	}
}

func TestNext_Bumps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current version.SemVer
		bump    version.Bump
		want    version.SemVer
	}{
		{
			name:    "patch from zero",
			current: version.SemVer{Major: 0, Minor: 0, Patch: 0},
			bump:    version.BumpPatch,
			want:    version.SemVer{Major: 0, Minor: 0, Patch: 1},
		},
		{
			name:    "minor from zero",
			current: version.SemVer{Major: 0, Minor: 0, Patch: 0},
			bump:    version.BumpMinor,
			want:    version.SemVer{Major: 0, Minor: 1, Patch: 0},
		},
		{
			name:    "major from zero",
			current: version.SemVer{Major: 0, Minor: 0, Patch: 0},
			bump:    version.BumpMajor,
			want:    version.SemVer{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "patch increments only patch",
			current: version.SemVer{Major: 1, Minor: 2, Patch: 3},
			bump:    version.BumpPatch,
			want:    version.SemVer{Major: 1, Minor: 2, Patch: 4},
		},
		{
			name:    "minor increments minor and zeroes patch",
			current: version.SemVer{Major: 1, Minor: 2, Patch: 3},
			bump:    version.BumpMinor,
			want:    version.SemVer{Major: 1, Minor: 3, Patch: 0},
		},
		{
			name:    "major increments major and zeroes minor and patch",
			current: version.SemVer{Major: 1, Minor: 2, Patch: 3},
			bump:    version.BumpMajor,
			want:    version.SemVer{Major: 2, Minor: 0, Patch: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := version.Next(tt.current, tt.bump)
			if got != tt.want {
				t.Errorf("Next(%+v, %v) = %+v, want %+v", tt.current, tt.bump, got, tt.want)
			}
		})
	}
}

func TestNext_DefaultBumpIsPatch(t *testing.T) {
	t.Parallel()

	// The zero value of Bump must behave as patch — no flag given defaults to patch.
	var defaultBump version.Bump

	got := version.Next(version.SemVer{Major: 0, Minor: 0, Patch: 0}, defaultBump)

	want := version.SemVer{Major: 0, Minor: 0, Patch: 1}
	if got != want {
		t.Errorf("Next with zero-value Bump = %+v, want %+v (patch)", got, want)
	}
}

func TestSemVer_String_FormatsWithPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version version.SemVer
		prefix  string
		want    string
	}{
		{
			name:    "default v prefix",
			version: version.SemVer{Major: 0, Minor: 0, Patch: 1},
			prefix:  "v",
			want:    "v0.0.1",
		},
		{
			name:    "empty prefix yields bare semver",
			version: version.SemVer{Major: 0, Minor: 0, Patch: 1},
			prefix:  "",
			want:    "0.0.1",
		},
		{
			name:    "double-digit segments and component prefix",
			version: version.SemVer{Major: 10, Minor: 12, Patch: 3},
			prefix:  "pkg-name/v",
			want:    "pkg-name/v10.12.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.version.String(tt.prefix)
			if got != tt.want {
				t.Errorf("SemVer%+v.String(%q) = %q, want %q", tt.version, tt.prefix, got, tt.want)
			}
		})
	}
}
