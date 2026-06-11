package main

import "testing"

// TestParseRegenerateFlags covers the Phase 5 `mint release regenerate` flag
// surface: the optional <version> positional, the --reuse/--fresh source axis
// (default fresh), the single-value --target axis, and the --all / -y booleans.
// This is the parse skeleton only — the semantic axis-contract validation
// (reuse⇒release-only, changelog-disabled, fresh -y needs target) is task 5-2.
func TestParseRegenerateFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantVersion   string
		wantSource    regenerateSource
		wantSourceSet bool
		wantTarget    regenerateTarget
		wantAll       bool
		wantYes       bool
	}{
		{
			name:          "version with reuse and target release",
			args:          []string{"1.4.0", "--reuse", "--target", "release"},
			wantVersion:   "1.4.0",
			wantSource:    sourceReuse,
			wantSourceSet: true,
			wantTarget:    targetRelease,
		},
		{
			name:        "defaults source to fresh when neither reuse nor fresh given",
			args:        []string{"1.4.0"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantTarget:  targetUnset,
		},
		{
			name:          "explicit fresh resolves to fresh source",
			args:          []string{"1.4.0", "--fresh"},
			wantVersion:   "1.4.0",
			wantSource:    sourceFresh,
			wantSourceSet: true,
		},
		{
			name:        "target both parses as a single-flag value",
			args:        []string{"1.4.0", "--target", "both"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantTarget:  targetBoth,
		},
		{
			name:        "target changelog parses as a single-flag value",
			args:        []string{"1.4.0", "--target", "changelog"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantTarget:  targetChangelog,
		},
		{
			name:          "all with reuse and target release",
			args:          []string{"--all", "--reuse", "--target", "release"},
			wantSource:    sourceReuse,
			wantSourceSet: true,
			wantTarget:    targetRelease,
			wantAll:       true,
		},
		{
			name:       "short yes parses to a boolean",
			args:       []string{"--all", "-y"},
			wantSource: sourceFresh,
			wantAll:    true,
			wantYes:    true,
		},
		{
			name:       "long yes parses to a boolean",
			args:       []string{"--all", "--yes"},
			wantSource: sourceFresh,
			wantAll:    true,
			wantYes:    true,
		},
		{
			name:          "version last after reuse and target value",
			args:          []string{"--reuse", "--target", "release", "1.4.0"},
			wantVersion:   "1.4.0",
			wantSource:    sourceReuse,
			wantSourceSet: true,
			wantTarget:    targetRelease,
		},
		{
			name:          "version mid between reuse and target flag",
			args:          []string{"--reuse", "1.4.0", "--target", "release"},
			wantVersion:   "1.4.0",
			wantSource:    sourceReuse,
			wantSourceSet: true,
			wantTarget:    targetRelease,
		},
		{
			name:        "target equals form does not mis-split the version",
			args:        []string{"--target=release", "1.4.0"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantTarget:  targetRelease,
		},
		{
			name:        "short yes before version",
			args:        []string{"-y", "1.4.0"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantYes:     true,
		},
		{
			name:        "short yes after version",
			args:        []string{"1.4.0", "-y"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantYes:     true,
		},
		{
			name:        "version after target value space form",
			args:        []string{"--target", "release", "1.4.0"},
			wantVersion: "1.4.0",
			wantSource:  sourceFresh,
			wantTarget:  targetRelease,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := parseRegenerateFlags(tt.args)
			if err != nil {
				t.Fatalf("parseRegenerateFlags(%v) returned error: %v", tt.args, err)
			}
			if req.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", req.Version, tt.wantVersion)
			}
			if req.Source != tt.wantSource {
				t.Errorf("Source = %v, want %v", req.Source, tt.wantSource)
			}
			if req.SourceSet != tt.wantSourceSet {
				t.Errorf("SourceSet = %v, want %v", req.SourceSet, tt.wantSourceSet)
			}
			if req.Target != tt.wantTarget {
				t.Errorf("Target = %v, want %v", req.Target, tt.wantTarget)
			}
			if req.All != tt.wantAll {
				t.Errorf("All = %v, want %v", req.All, tt.wantAll)
			}
			if req.Yes != tt.wantYes {
				t.Errorf("Yes = %v, want %v", req.Yes, tt.wantYes)
			}
		})
	}
}

// TestParseRegenerateFlags_UnknownTarget rejects any --target value other than
// release/changelog/both with the exact spec message.
func TestParseRegenerateFlags_UnknownTarget(t *testing.T) {
	t.Parallel()

	const wantMsg = "invalid --target value provider (expected release, changelog, or both)"
	_, err := parseRegenerateFlags([]string{"1.4.0", "--target", "provider"})
	if err == nil {
		t.Fatalf("parseRegenerateFlags returned nil error, want %q", wantMsg)
	}
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
}

// TestParseRegenerateFlags_BareIsError rejects a bare regenerate (no <version>,
// no --all) with the exact spec message — presence rule A.
func TestParseRegenerateFlags_BareIsError(t *testing.T) {
	t.Parallel()

	const wantMsg = "specify a version or --all"
	_, err := parseRegenerateFlags(nil)
	if err == nil {
		t.Fatalf("parseRegenerateFlags(nil) returned nil error, want %q", wantMsg)
	}
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
}

// TestParseRegenerateFlags_VersionAndAllIsError rejects supplying both a
// <version> and --all with the exact spec message — presence rule B.
func TestParseRegenerateFlags_VersionAndAllIsError(t *testing.T) {
	t.Parallel()

	const wantMsg = "cannot combine a version with --all"
	_, err := parseRegenerateFlags([]string{"1.4.0", "--all"})
	if err == nil {
		t.Fatalf("parseRegenerateFlags returned nil error, want %q", wantMsg)
	}
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
}

// TestParseRegenerateFlags_ReuseAndFreshIsError rejects combining --reuse and
// --fresh: the source axis is mutually exclusive.
func TestParseRegenerateFlags_ReuseAndFreshIsError(t *testing.T) {
	t.Parallel()

	if _, err := parseRegenerateFlags([]string{"1.4.0", "--reuse", "--fresh"}); err == nil {
		t.Error("parseRegenerateFlags(--reuse --fresh) returned nil error, want a conflict error")
	}
}

// TestParseRegenerateFlags_TwoPositionalsIsError rejects a second bare positional
// (regenerate takes at most one <version>) with the exact message produced by
// splitRegeneratePositional, naming the offending second token.
func TestParseRegenerateFlags_TwoPositionalsIsError(t *testing.T) {
	t.Parallel()

	const wantMsg = "unexpected argument 4.5.6 (regenerate takes at most one version)"
	_, err := parseRegenerateFlags([]string{"1.4.0", "4.5.6"})
	if err == nil {
		t.Fatalf("parseRegenerateFlags returned nil error, want %q", wantMsg)
	}
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
}
