package record_test

import (
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

func TestFirstReleaseBody_IsFixedInitialRelease(t *testing.T) {
	t.Parallel()

	// The first release (no prior tag) has no diff base, so mint skips the AI
	// entirely and uses a fixed body. The body MUST be exactly "Initial release."
	// — Notes-path precedence (1): first release wins over every other guard.
	if got := record.FirstReleaseBody; got != "Initial release." {
		t.Errorf("FirstReleaseBody = %q, want %q", got, "Initial release.")
	}
}

func TestFirstReleaseBody_InvokesNoCommand(t *testing.T) {
	t.Parallel()

	// The first-release body is a pure constant path: it must never invoke an
	// AI/claude command (or any external command). There is no AI transport yet,
	// but the guarantee is structural — obtaining the body touches the runner
	// zero times, so it can never spend an AI call.
	r := runner.NewFakeRunner()

	body := record.FirstReleaseBody

	if body != "Initial release." {
		t.Errorf("FirstReleaseBody = %q, want %q", body, "Initial release.")
	}
	if got := len(r.Invocations()); got != 0 {
		t.Errorf("runner invocations = %d, want 0 (first-release body must not call any command)", got)
	}
}
