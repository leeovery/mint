package main

import (
	"errors"
	"testing"

	"mint/internal/engine"
)

// TestValidateRegenerateRequest covers the semantic axis validation that runs AFTER the
// parse, with access to the loaded config's changelog bool. The source and target axes
// are ORTHOGONAL — every source (fresh, reuse, from-release) can write every target — so
// the validator NEVER mutates the target (no implied release) and rejects only a
// changelog-disabled target or a -y run with no --target. The target passes through
// unchanged on every success case; a run WITHOUT -y and without --target is left for the
// interactive prompt and does NOT error here.
func TestValidateRegenerateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		req              regenerateRequest
		changelogEnabled bool
		wantTarget       regenerateTarget
	}{
		{
			name:             "reuse with no target stays unset (deferred to interactive, not forced to release)",
			req:              regenerateRequest{Source: sourceTag, Target: targetUnset},
			changelogEnabled: true,
			wantTarget:       targetUnset,
		},
		{
			name:             "reuse target changelog passes through (orthogonal axes)",
			req:              regenerateRequest{Source: sourceTag, Target: targetChangelog},
			changelogEnabled: true,
			wantTarget:       targetChangelog,
		},
		{
			name:             "reuse target both passes through",
			req:              regenerateRequest{Source: sourceTag, Target: targetBoth},
			changelogEnabled: true,
			wantTarget:       targetBoth,
		},
		{
			name:             "from-release target changelog passes through",
			req:              regenerateRequest{Source: sourceRelease, Target: targetChangelog},
			changelogEnabled: true,
			wantTarget:       targetChangelog,
		},
		{
			name:             "reuse with explicit target release passes through unchanged",
			req:              regenerateRequest{Source: sourceTag, Target: targetRelease},
			changelogEnabled: true,
			wantTarget:       targetRelease,
		},
		{
			name:             "fresh with explicit target release passes through unchanged",
			req:              regenerateRequest{Source: sourceFresh, Target: targetRelease},
			changelogEnabled: true,
			wantTarget:       targetRelease,
		},
		{
			name:             "fresh target changelog with changelog enabled passes through",
			req:              regenerateRequest{Source: sourceFresh, Target: targetChangelog},
			changelogEnabled: true,
			wantTarget:       targetChangelog,
		},
		{
			name:             "fresh without -y and without target stays unset (deferred to interactive)",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset},
			changelogEnabled: true,
			wantTarget:       targetUnset,
		},
		{
			name:             "reuse without -y and without target stays unset (deferred to interactive)",
			req:              regenerateRequest{Source: sourceTag, Target: targetUnset},
			changelogEnabled: true,
			wantTarget:       targetUnset,
		},
		{
			name:             "fresh --all without -y and without target stays unset",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, All: true},
			changelogEnabled: true,
			wantTarget:       targetUnset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateRegenerateRequest(tt.req, tt.changelogEnabled)
			if err != nil {
				t.Fatalf("validateRegenerateRequest(%+v, %v) returned error: %v", tt.req, tt.changelogEnabled, err)
			}
			if got.Target != tt.wantTarget {
				t.Errorf("Target = %v, want %v", got.Target, tt.wantTarget)
			}
		})
	}
}

// TestValidateRegenerateRequest_PreservesPlain proves the global --plain render
// flag survives the semantic validation unchanged, so the value parsed on the
// regenerate route reaches the presenter startup site (main constructs the
// presenter from validated.Plain). Without this, --plain would parse but the
// downstream presenter would silently fall back to plain=false.
func TestValidateRegenerateRequest_PreservesPlain(t *testing.T) {
	t.Parallel()

	for _, plain := range []bool{true, false} {
		got, err := validateRegenerateRequest(regenerateRequest{Source: sourceFresh, Target: targetRelease, Plain: plain}, true)
		if err != nil {
			t.Fatalf("validateRegenerateRequest(plain=%v) returned error: %v", plain, err)
		}
		if got.Plain != plain {
			t.Errorf("Plain = %v, want %v", got.Plain, plain)
		}
	}
}

// TestValidateRegenerateRequest_Errors covers the fail-loud violations with their EXACT
// messages. With the axes orthogonal, only two checks remain: a changelog-disabled
// target, and a -y run with no --target (for EVERY source — no source has a safe default
// surface to guess unattended).
func TestValidateRegenerateRequest_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		req              regenerateRequest
		changelogEnabled bool
		wantMsg          string
	}{
		{
			name:             "target changelog with changelog disabled errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetChangelog},
			changelogEnabled: false,
			wantMsg:          "changelog is disabled in config",
		},
		{
			name:             "target both with changelog disabled errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetBoth},
			changelogEnabled: false,
			wantMsg:          "changelog is disabled in config",
		},
		{
			name:             "reuse target changelog with changelog disabled errors",
			req:              regenerateRequest{Source: sourceTag, Target: targetChangelog},
			changelogEnabled: false,
			wantMsg:          "changelog is disabled in config",
		},
		{
			name:             "fresh -y without target errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with -y",
		},
		{
			name:             "reuse -y without target errors (no implied release)",
			req:              regenerateRequest{Source: sourceTag, Target: targetUnset, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with -y",
		},
		{
			name:             "from-release -y without target errors",
			req:              regenerateRequest{Source: sourceRelease, Target: targetUnset, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with -y",
		},
		{
			name:             "fresh --all -y without target errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, All: true, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with -y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateRegenerateRequest(tt.req, tt.changelogEnabled)
			if err == nil {
				t.Fatalf("validateRegenerateRequest(%+v, %v) returned nil error, want %q", tt.req, tt.changelogEnabled, tt.wantMsg)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// TestValidateTargetAgainstChangelog verifies the reusable changelog-disabled
// check in isolation — task 5-12 reuses it to validate batch targets up front
// before the batch starts. It rejects a changelog/both target when the
// changelog is disabled and is a no-op for a release target or when the
// changelog is enabled.
func TestValidateTargetAgainstChangelog(t *testing.T) {
	t.Parallel()

	// The validator returns the single owned engine.ErrChangelogDisabled sentinel;
	// sourcing the expected message from it (rather than a copied literal) makes the
	// test itself drift-proof against the wording, and errors.Is below proves the
	// returned error IS that sentinel, not just a string match.
	wantMsg := engine.ErrChangelogDisabled.Error()

	tests := []struct {
		name             string
		target           regenerateTarget
		changelogEnabled bool
		wantErr          bool
	}{
		{name: "changelog target disabled errors", target: targetChangelog, changelogEnabled: false, wantErr: true},
		{name: "both target disabled errors", target: targetBoth, changelogEnabled: false, wantErr: true},
		{name: "changelog target enabled passes", target: targetChangelog, changelogEnabled: true, wantErr: false},
		{name: "both target enabled passes", target: targetBoth, changelogEnabled: true, wantErr: false},
		{name: "release target disabled passes", target: targetRelease, changelogEnabled: false, wantErr: false},
		{name: "unset target disabled passes", target: targetUnset, changelogEnabled: false, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTargetAgainstChangelog(tt.target, tt.changelogEnabled)
			switch {
			case tt.wantErr && err == nil:
				t.Fatalf("validateTargetAgainstChangelog(%v, %v) = nil, want %q", tt.target, tt.changelogEnabled, wantMsg)
			case tt.wantErr && !errors.Is(err, engine.ErrChangelogDisabled):
				t.Errorf("error = %v, want errors.Is(err, engine.ErrChangelogDisabled)", err)
			case tt.wantErr && err.Error() != wantMsg:
				t.Errorf("error = %q, want %q", err.Error(), wantMsg)
			case !tt.wantErr && err != nil:
				t.Errorf("validateTargetAgainstChangelog(%v, %v) = %v, want nil", tt.target, tt.changelogEnabled, err)
			}
		})
	}
}
