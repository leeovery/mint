package main

import (
	"testing"

	"mint/internal/engine"
)

// TestRegenerateGateSet maps a RESOLVED regenerate target to the engine's
// preflight gate-set selection (task 5-4). The mapping encodes the spec's general
// rule off the target surface: a provider-writing target (release/both) selects
// gh-auth (CallsProvider); a changelog-committing target (changelog/both) selects
// the clean-tree + on-branch + remote-sync bucket (CommitsAndPushes). --reuse is
// resolved to --target release before this runs, so it maps to provider-only.
func TestRegenerateGateSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target regenerateTarget
		want   engine.RegenerateGateSet
	}{
		{
			name:   "release writes the provider only",
			target: targetRelease,
			want:   engine.RegenerateGateSet{CallsProvider: true},
		},
		{
			name:   "changelog commits and pushes only",
			target: targetChangelog,
			want:   engine.RegenerateGateSet{CommitsAndPushes: true},
		},
		{
			name:   "both writes the provider AND commits + pushes",
			target: targetBoth,
			want:   engine.RegenerateGateSet{CallsProvider: true, CommitsAndPushes: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := regenerateGateSet(tt.target)
			if got != tt.want {
				t.Errorf("regenerateGateSet(%v) = %+v, want %+v", tt.target, got, tt.want)
			}
		})
	}
}
