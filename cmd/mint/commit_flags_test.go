package main

import "testing"

// TestParseCommitFlags covers the Phase 1 bare `mint commit` flag surface. The bare
// walking-skeleton path wires only the presentation flags the presenter consumes at
// startup: --plain (force un-styled output) and -y/--yes (auto-accept). The staging
// (-a/-A), push (-p), and AI-skip (--no-ai) flags are LATER phases and are not parsed
// here. The long and short -y forms must be equivalent.
func TestParseCommitFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantYes   bool
		wantPlain bool
	}{
		{name: "no flags", args: nil},
		{name: "short yes", args: []string{"-y"}, wantYes: true},
		{name: "long yes", args: []string{"--yes"}, wantYes: true},
		{name: "plain", args: []string{"--plain"}, wantPlain: true},
		{name: "yes and plain", args: []string{"-y", "--plain"}, wantYes: true, wantPlain: true},
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
