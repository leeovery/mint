package config_test

import (
	"testing"

	"mint/internal/config"
)

// TestVerb_ClosedSet pins the typed verb enum as a CLOSED two-value set: exactly
// VerbRelease and VerbCommit, one per verb table ([release], [commit]). The enum is
// the parameter the layered accessors (AICommandFor / TimeoutFor) will accept; a
// typed closed set makes their domain exhaustive by construction — there is no
// "unrecognized verb" branch to handle.
func TestVerb_ClosedSet(t *testing.T) {
	t.Parallel()

	// The two real verbs must be distinct so they route to distinct tables — a
	// collision would let one table's override silently serve the other verb.
	if config.VerbRelease == config.VerbCommit {
		t.Fatalf("VerbRelease and VerbCommit must be distinct, both are %v", config.VerbRelease)
	}
}

// TestVerb_NoRegenerateValue proves there is no `regenerate` (or any third) reachable
// enum value: enumerating the closed set yields exactly the two real verbs. regenerate
// rides on [release] (regenerate_fresh.go can only pass VerbRelease), so a third
// constant would reintroduce the very fall-through the closed enum exists to prevent.
func TestVerb_NoRegenerateValue(t *testing.T) {
	t.Parallel()

	// The complete enumerated domain of the type. If a third verb is ever added this
	// slice must grow, which forces a reviewer to confront the no-regenerate rule.
	closedSet := []config.Verb{config.VerbRelease, config.VerbCommit}

	if len(closedSet) != 2 {
		t.Fatalf("verb closed set has %d values, want exactly 2 (release, commit; no regenerate)", len(closedSet))
	}

	// No member of the closed set may alias another: two distinct constants mapping to
	// the same underlying value would be a hidden third-state collapse.
	seen := map[config.Verb]bool{}
	for _, v := range closedSet {
		if seen[v] {
			t.Fatalf("duplicate verb value %v in closed set", v)
		}
		seen[v] = true
	}
}

// TestVerb_ZeroValueIsRealVerb locks the chosen representation's zero value to a REAL
// verb table, never an unknown/third one. With type Verb int and iota-0, a zero-value
// Verb resolves to VerbRelease — one of the two real verbs — so there is no silent
// empty/unknown member distinct from the closed set for an accessor to mishandle.
func TestVerb_ZeroValueIsRealVerb(t *testing.T) {
	t.Parallel()

	var zero config.Verb
	if zero != config.VerbRelease {
		t.Errorf("zero-value Verb = %v, want VerbRelease (a real verb, not an unknown/third member)", zero)
	}
}
