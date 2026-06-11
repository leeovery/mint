package main

import "testing"

// TestValidateRegenerateRequest covers the Phase 5-2 semantic axis-contract
// validation that runs AFTER the 5-1 parse, with access to the loaded config's
// changelog bool. It enforces the source × target contract: --reuse is
// release-only (and implies --target release when unset), --target
// changelog/both is rejected when the changelog is disabled in config, and a
// fresh -y run with no --target is rejected because there is no surface to
// guess unattended. A fresh run WITHOUT -y and without --target is left for the
// interactive prompt (task 5-10) and does NOT error here.
func TestValidateRegenerateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		req              regenerateRequest
		changelogEnabled bool
		wantTarget       regenerateTarget
	}{
		{
			name:             "reuse with no target resolves target to release",
			req:              regenerateRequest{Source: sourceReuse, Target: targetUnset},
			changelogEnabled: true,
			wantTarget:       targetRelease,
		},
		{
			name:             "reuse target resolution to release is unaffected by -y",
			req:              regenerateRequest{Source: sourceReuse, Target: targetUnset, Yes: true},
			changelogEnabled: true,
			wantTarget:       targetRelease,
		},
		{
			name:             "reuse with explicit target release passes through unchanged",
			req:              regenerateRequest{Source: sourceReuse, Target: targetRelease},
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
			name:             "fresh --all without -y and without target stays unset",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, All: true},
			changelogEnabled: true,
			wantTarget:       targetUnset,
		},
		{
			name:             "reuse no target -y with changelog disabled resolves to release without tripping changelog check",
			req:              regenerateRequest{Source: sourceReuse, Target: targetUnset, Yes: true},
			changelogEnabled: false,
			wantTarget:       targetRelease,
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

// TestValidateRegenerateRequest_Errors covers the fail-loud axis-contract
// violations with their EXACT spec messages, in the order the validator applies
// them (most specific message wins).
func TestValidateRegenerateRequest_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		req              regenerateRequest
		changelogEnabled bool
		wantMsg          string
	}{
		{
			name:             "reuse target changelog errors release-only",
			req:              regenerateRequest{Source: sourceReuse, Target: targetChangelog},
			changelogEnabled: true,
			wantMsg:          "--reuse writes the provider release only; it cannot target the changelog",
		},
		{
			name:             "reuse target both errors release-only",
			req:              regenerateRequest{Source: sourceReuse, Target: targetBoth},
			changelogEnabled: true,
			wantMsg:          "--reuse writes the provider release only; it cannot target the changelog",
		},
		{
			name:             "reuse target changelog with changelog disabled wins release-only over changelog-disabled",
			req:              regenerateRequest{Source: sourceReuse, Target: targetChangelog},
			changelogEnabled: false,
			wantMsg:          "--reuse writes the provider release only; it cannot target the changelog",
		},
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
			name:             "fresh -y without target errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with --fresh -y",
		},
		{
			name:             "fresh --all -y without target errors",
			req:              regenerateRequest{Source: sourceFresh, Target: targetUnset, All: true, Yes: true},
			changelogEnabled: true,
			wantMsg:          "--target is required with --fresh -y",
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

	const wantMsg = "changelog is disabled in config"

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
			case tt.wantErr && err.Error() != wantMsg:
				t.Errorf("error = %q, want %q", err.Error(), wantMsg)
			case !tt.wantErr && err != nil:
				t.Errorf("validateTargetAgainstChangelog(%v, %v) = %v, want nil", tt.target, tt.changelogEnabled, err)
			}
		})
	}
}
