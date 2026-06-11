package main

import "testing"

// TestParseInitFlags covers the `mint init` flag surface: a bare invocation
// scaffolds without overwriting (Force false), and --force opts into regenerating
// existing files (Force true). init takes no positional arguments.
func TestParseInitFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantForce bool
	}{
		{name: "no flags defaults to non-clobbering", args: nil, wantForce: false},
		{name: "force opts into overwrite", args: []string{"--force"}, wantForce: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseInitFlags(tt.args)
			if err != nil {
				t.Fatalf("parseInitFlags(%v) returned error: %v", tt.args, err)
			}
			if opts.Force != tt.wantForce {
				t.Errorf("Force = %v, want %v", opts.Force, tt.wantForce)
			}
		})
	}
}

// TestParseInitFlags_UnknownFlag is a usage error so a typo'd flag fails loudly
// rather than being silently ignored.
func TestParseInitFlags_UnknownFlag(t *testing.T) {
	t.Parallel()

	if _, err := parseInitFlags([]string{"--nope"}); err == nil {
		t.Fatal("parseInitFlags accepted an unknown flag; want a usage error")
	}
}
