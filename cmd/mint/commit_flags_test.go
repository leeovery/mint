package main

import (
	"testing"

	"mint/internal/commit"
)

// TestParseCommitFlags covers the `mint commit` flag surface: the presentation flags
// the presenter consumes at startup — --plain (force un-styled output) and -y/--yes
// (auto-accept) — and the Phase 2 staging selectors -a/--all and -A/--add-all,
// resolved into a single commit.StagingMode (default StagedOnly). The push (-p) and
// AI-skip (--no-ai) flags are LATER phases and are not parsed here. The long and
// short forms must be equivalent.
func TestParseCommitFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantYes     bool
		wantPlain   bool
		wantStaging commit.StagingMode
		wantNoAI    bool
	}{
		{name: "no flags", args: nil, wantStaging: commit.StagedOnly},
		{name: "short yes", args: []string{"-y"}, wantYes: true},
		{name: "long yes", args: []string{"--yes"}, wantYes: true},
		{name: "plain", args: []string{"--plain"}, wantPlain: true},
		{name: "yes and plain", args: []string{"-y", "--plain"}, wantYes: true, wantPlain: true},
		{name: "short all", args: []string{"-a"}, wantStaging: commit.All},
		{name: "long all", args: []string{"--all"}, wantStaging: commit.All},
		{name: "short add-all", args: []string{"-A"}, wantStaging: commit.AddAll},
		{name: "long add-all", args: []string{"--add-all"}, wantStaging: commit.AddAll},
		{name: "all with yes and plain", args: []string{"-a", "-y", "--plain"}, wantYes: true, wantPlain: true, wantStaging: commit.All},
		{name: "add-all with yes", args: []string{"-A", "-y"}, wantYes: true, wantStaging: commit.AddAll},
		{name: "no-ai", args: []string{"--no-ai"}, wantNoAI: true},
		{name: "no-ai with all", args: []string{"--no-ai", "-a"}, wantStaging: commit.All, wantNoAI: true},
		{name: "no-ai with add-all", args: []string{"--no-ai", "-A"}, wantStaging: commit.AddAll, wantNoAI: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseCommitFlags(tt.args)
			if err != nil {
				t.Fatalf("parseCommitFlags(%v) returned error: %v", tt.args, err)
			}
			if opts.Yes != tt.wantYes {
				t.Errorf("Yes = %v, want %v", opts.Yes, tt.wantYes)
			}
			if opts.Plain != tt.wantPlain {
				t.Errorf("Plain = %v, want %v", opts.Plain, tt.wantPlain)
			}
			if opts.Staging != tt.wantStaging {
				t.Errorf("Staging = %v, want %v", opts.Staging, tt.wantStaging)
			}
			if opts.NoAI != tt.wantNoAI {
				t.Errorf("NoAI = %v, want %v", opts.NoAI, tt.wantNoAI)
			}
		})
	}
}

// TestParseCommitFlags_UnknownFlag_Errors proves an unrecognised flag is a parse
// error (surfaced as a usage error by the cmd layer), not silently ignored.
func TestParseCommitFlags_UnknownFlag_Errors(t *testing.T) {
	t.Parallel()

	if _, err := parseCommitFlags([]string{"--nope"}); err == nil {
		t.Fatal("parseCommitFlags(--nope) returned nil error, want a parse error")
	}
}

// conflictingStagingMessage is the spec's EXACT conflicting-flags message (Staging
// Model → Decision — two faithful flags). -a and -A are mutually exclusive — -A
// already includes -a's changes — so combining them is fail-loud, never a silent
// winner. The cmd layer surfaces this verbatim as a usage error.
const conflictingStagingMessage = "-a and -A cannot be combined; -A already includes -a's changes"

// TestParseCommitFlags_ConflictingStaging proves -a and -A together fail loud with
// the spec's EXACT message — whether supplied separately (`-a -A`), bundled
// (`-aA` / `-Aa`), or in long form (`--all --add-all`). The bundled forms exercise
// the short-flag pre-expansion: stdlib flag.NewFlagSet has no POSIX bundling, so
// `-aA` would otherwise be one unknown flag (a generic usage error) and never reach
// the conflict guard. Every form must surface the SAME conflicting-flags message.
func TestParseCommitFlags_ConflictingStaging(t *testing.T) {
	t.Parallel()

	conflicts := [][]string{
		{"-a", "-A"},
		{"-A", "-a"},
		{"-aA"},
		{"-Aa"},
		{"--all", "--add-all"},
		{"--add-all", "--all"},
		{"-aA", "-y"},
	}
	for _, args := range conflicts {
		_, err := parseCommitFlags(args)
		if err == nil {
			t.Errorf("parseCommitFlags(%v) returned nil error, want the conflicting-flags error", args)
			continue
		}
		if err.Error() != conflictingStagingMessage {
			t.Errorf("parseCommitFlags(%v) error = %q, want the spec's exact message %q", args, err.Error(), conflictingStagingMessage)
		}
	}
}

// TestParseCommitFlags_BundledShortFlags proves bundled single-letter flags are
// pre-expanded before fs.Parse so a bundle of DEFINED short flags parses as if each
// were passed separately. `-ay` = -a + -y; `-Ay` = -A + -y. Without the pre-expansion
// stdlib flag would reject `-ay` as one unknown flag.
func TestParseCommitFlags_BundledShortFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantYes     bool
		wantStaging commit.StagingMode
	}{
		{name: "all bundled with yes", args: []string{"-ay"}, wantYes: true, wantStaging: commit.All},
		{name: "add-all bundled with yes", args: []string{"-Ay"}, wantYes: true, wantStaging: commit.AddAll},
		{name: "yes then all bundled", args: []string{"-ya"}, wantYes: true, wantStaging: commit.All},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseCommitFlags(tt.args)
			if err != nil {
				t.Fatalf("parseCommitFlags(%v) returned error: %v", tt.args, err)
			}
			if opts.Yes != tt.wantYes {
				t.Errorf("Yes = %v, want %v", opts.Yes, tt.wantYes)
			}
			if opts.Staging != tt.wantStaging {
				t.Errorf("Staging = %v, want %v", opts.Staging, tt.wantStaging)
			}
		})
	}
}

// TestParseCommitFlags_LongFlagsPassThroughUnexpanded proves long flags (leading
// `--`) are NEVER touched by the short-flag pre-expansion: --all and --add-all parse
// to their modes intact, and a long flag whose name happens to contain defined
// single-letter flag chars (--add-all contains 'a' and 'A') is not mistaken for a
// bundle. An unknown long flag still surfaces as a parse error, not a silent expand.
func TestParseCommitFlags_LongFlagsPassThroughUnexpanded(t *testing.T) {
	t.Parallel()

	allOpts, err := parseCommitFlags([]string{"--all"})
	if err != nil {
		t.Fatalf("parseCommitFlags(--all) returned error: %v", err)
	}
	if allOpts.Staging != commit.All {
		t.Errorf("--all Staging = %v, want commit.All", allOpts.Staging)
	}

	addAllOpts, err := parseCommitFlags([]string{"--add-all"})
	if err != nil {
		t.Fatalf("parseCommitFlags(--add-all) returned error: %v", err)
	}
	if addAllOpts.Staging != commit.AddAll {
		t.Errorf("--add-all Staging = %v, want commit.AddAll", addAllOpts.Staging)
	}

	// An unknown long flag must still be a parse error — the pre-expansion only
	// touches single-`-` bundles, never `--` tokens.
	if _, err := parseCommitFlags([]string{"--aA"}); err == nil {
		t.Error("parseCommitFlags(--aA) returned nil error; an unknown long flag must be a parse error, never expanded")
	}
}
