package main

import (
	"testing"

	"mint/internal/version"
)

// TestParseReleaseFlags covers the Phase 1 `mint release` flag surface: the bump
// selection (-p/-m/-M, default patch), the -y/--yes skip, and the global --plain
// flag. The long and short forms must be equivalent, and the bump flags resolve
// to the matching version.Bump.
func TestParseReleaseFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantBump      version.Bump
		wantYes       bool
		wantPlain     bool
		wantNoAI      bool
		wantAutoStash bool
		wantAnyBranch bool
	}{
		{name: "no flags defaults to patch", args: nil, wantBump: version.BumpPatch},
		{name: "short patch", args: []string{"-p"}, wantBump: version.BumpPatch},
		{name: "long patch", args: []string{"--patch"}, wantBump: version.BumpPatch},
		{name: "short minor", args: []string{"-m"}, wantBump: version.BumpMinor},
		{name: "long minor", args: []string{"--minor"}, wantBump: version.BumpMinor},
		{name: "short major", args: []string{"-M"}, wantBump: version.BumpMajor},
		{name: "long major", args: []string{"--major"}, wantBump: version.BumpMajor},
		{name: "short yes", args: []string{"-y"}, wantYes: true},
		{name: "long yes", args: []string{"--yes"}, wantYes: true},
		{name: "plain", args: []string{"--plain"}, wantPlain: true},
		{name: "no-ai", args: []string{"--no-ai"}, wantNoAI: true},
		{name: "autostash", args: []string{"--autostash"}, wantAutoStash: true},
		{name: "any-branch", args: []string{"--any-branch"}, wantAnyBranch: true},
		{name: "minor with yes and plain", args: []string{"-m", "-y", "--plain"}, wantBump: version.BumpMinor, wantYes: true, wantPlain: true},
		{name: "no-ai with minor and yes", args: []string{"--no-ai", "-m", "-y"}, wantBump: version.BumpMinor, wantYes: true, wantNoAI: true},
		{name: "autostash with minor and yes", args: []string{"--autostash", "-m", "-y"}, wantBump: version.BumpMinor, wantYes: true, wantAutoStash: true},
		{name: "any-branch with autostash and yes", args: []string{"--any-branch", "--autostash", "-y"}, wantYes: true, wantAutoStash: true, wantAnyBranch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseReleaseFlags(tt.args)
			if err != nil {
				t.Fatalf("parseReleaseFlags(%v) returned error: %v", tt.args, err)
			}
			if opts.Bump != tt.wantBump {
				t.Errorf("Bump = %v, want %v", opts.Bump, tt.wantBump)
			}
			if opts.Yes != tt.wantYes {
				t.Errorf("Yes = %v, want %v", opts.Yes, tt.wantYes)
			}
			if opts.Plain != tt.wantPlain {
				t.Errorf("Plain = %v, want %v", opts.Plain, tt.wantPlain)
			}
			if opts.NoAI != tt.wantNoAI {
				t.Errorf("NoAI = %v, want %v", opts.NoAI, tt.wantNoAI)
			}
			if opts.AutoStash != tt.wantAutoStash {
				t.Errorf("AutoStash = %v, want %v", opts.AutoStash, tt.wantAutoStash)
			}
			if opts.AnyBranch != tt.wantAnyBranch {
				t.Errorf("AnyBranch = %v, want %v", opts.AnyBranch, tt.wantAnyBranch)
			}
			// The --no-ai flag must thread through to the engine options.
			if got := opts.ReleaseOptions().NoAI; got != tt.wantNoAI {
				t.Errorf("ReleaseOptions().NoAI = %v, want %v", got, tt.wantNoAI)
			}
			// The --autostash flag must thread through to the engine options.
			if got := opts.ReleaseOptions().AutoStash; got != tt.wantAutoStash {
				t.Errorf("ReleaseOptions().AutoStash = %v, want %v", got, tt.wantAutoStash)
			}
			// The --any-branch flag must thread through to the engine options.
			if got := opts.ReleaseOptions().AnyBranch; got != tt.wantAnyBranch {
				t.Errorf("ReleaseOptions().AnyBranch = %v, want %v", got, tt.wantAnyBranch)
			}
		})
	}
}

// TestParseReleaseFlags_ConflictingBumps rejects more than one bump flag at once:
// the bump selectors are mutually exclusive, so combining them is a usage error
// rather than silent precedence.
func TestParseReleaseFlags_ConflictingBumps(t *testing.T) {
	t.Parallel()

	conflicts := [][]string{
		{"-m", "-M"},
		{"-p", "-m"},
		{"--minor", "--major"},
	}
	for _, args := range conflicts {
		if _, err := parseReleaseFlags(args); err == nil {
			t.Errorf("parseReleaseFlags(%v) returned nil error, want a conflict error", args)
		}
	}
}

// TestParseReleaseFlags_SetVersion threads the --set-version value through to the
// engine options unparsed (the engine owns parsing + the strictly-greater gate). A
// bare --set-version (no bump flag) is legal and leaves the bump at its default.
func TestParseReleaseFlags_SetVersion(t *testing.T) {
	t.Parallel()

	opts, err := parseReleaseFlags([]string{"--set-version", "2.0.0"})
	if err != nil {
		t.Fatalf("parseReleaseFlags returned error: %v", err)
	}
	if opts.SetVersion != "2.0.0" {
		t.Errorf("SetVersion = %q, want %q", opts.SetVersion, "2.0.0")
	}
	if got := opts.ReleaseOptions().SetVersion; got != "2.0.0" {
		t.Errorf("ReleaseOptions().SetVersion = %q, want %q", got, "2.0.0")
	}
}

// TestParseReleaseFlags_SetVersionWithBumpFlag rejects combining --set-version with
// ANY bump flag: it is a usage error with the exact spec message, not silent
// precedence. Every short and long bump form must conflict.
func TestParseReleaseFlags_SetVersionWithBumpFlag(t *testing.T) {
	t.Parallel()

	const wantMsg = "can't combine --set-version with a bump flag"
	conflicts := [][]string{
		{"--set-version", "2.0.0", "-p"},
		{"--set-version", "2.0.0", "--patch"},
		{"--set-version", "2.0.0", "-m"},
		{"--set-version", "2.0.0", "--minor"},
		{"--set-version", "2.0.0", "-M"},
		{"--set-version", "2.0.0", "--major"},
	}
	for _, args := range conflicts {
		_, err := parseReleaseFlags(args)
		if err == nil {
			t.Errorf("parseReleaseFlags(%v) returned nil error, want a conflict error", args)
			continue
		}
		if err.Error() != wantMsg {
			t.Errorf("parseReleaseFlags(%v) error = %q, want %q", args, err.Error(), wantMsg)
		}
	}
}
