package notes_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

func TestNoAIBody_ProducesCommitSubjectListBodyNoAICall(t *testing.T) {
	t.Parallel()

	// --no-ai is a deliberate skip: it produces the commit-subject list since the last
	// tag via `git log --format=%s {lastTag}..HEAD`, returns it verbatim, and invokes
	// NO AI (the only command issued is the single git log; there is no claude call).
	const subjects = "Add login flow\nFix token refresh\n"
	r := seedCommitSubjects(t, subjects)

	body, err := notes.NoAIBody(t.Context(), r, "v1.0.0", config.Release{})
	if err != nil {
		t.Fatalf("NoAIBody returned unexpected error: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1 (git log only, no AI call)", len(invs))
	}
	assertGitArgv(t, invs[0], wantCommitSubjectArgs("v1.0.0"))
}

func TestNoAIBody_FixedFallbackStringUsedInsteadNoGitLog(t *testing.T) {
	t.Parallel()

	// When [release].fallback is set, --no-ai uses that fixed string verbatim instead
	// of the commit-subject list, and runs NO git log (the fixed string IS the body).
	const fixed = "Notes unavailable — see commit history."
	r := runner.NewFakeRunner()
	rel := config.Release{Fallback: fixed}

	body, err := notes.NoAIBody(t.Context(), r, "v2.0.0", rel)
	if err != nil {
		t.Fatalf("NoAIBody returned unexpected error for a fixed fallback string: %v", err)
	}
	if body != fixed {
		t.Errorf("body = %q, want the fixed fallback string %q", body, fixed)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git was invoked %d times for a fixed fallback string, want 0", len(r.Invocations()))
	}
}

func TestNoAIBody_NeverAbortsEvenWhenAIWouldHaveFailed(t *testing.T) {
	t.Parallel()

	// --no-ai is NOT a failure path: it has no failure input and never aborts. Even
	// with on_notes_failure="abort" (which WOULD abort on the normal AI path), --no-ai
	// just returns a body and nil error — on_notes_failure does NOT govern this path.
	const subjects = "One change\nTwo change\n"
	r := seedCommitSubjects(t, subjects)
	rel := config.Release{OnNotesFailure: "abort"}

	body, err := notes.NoAIBody(t.Context(), r, "v3.0.0", rel)
	if err != nil {
		t.Fatalf("NoAIBody aborted (err = %v), want it to never abort", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}
}

func TestNoAIBody_NonEmptyBodyWhenNoCommitsSinceLastTag(t *testing.T) {
	t.Parallel()

	// With no commits since the last tag (empty git log output), the body must NOT be
	// empty — it floors to the non-empty minimal record so the tag is never empty.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil) // empty log: no commits since last tag.

	body, err := notes.NoAIBody(t.Context(), r, "v4.0.0", config.Release{})
	if err != nil {
		t.Fatalf("NoAIBody returned unexpected error on empty log: %v", err)
	}
	if strings.TrimSpace(body) == "" {
		t.Errorf("body = %q, want a non-empty floor when there are no commits", body)
	}
}

func TestNoAIBody_SharesFallbackBuilderWithOnNotesFailureFallback(t *testing.T) {
	t.Parallel()

	// The fallback-body builder is SHARED with on_notes_failure=fallback (2-7): for the
	// same inputs, ResolveFailure (fallback mode) and NoAIBody produce the SAME body.
	// Asserted across both the commit-subject list and the fixed-string forms.
	cases := []struct {
		name     string
		rel      config.Release
		subjects string // git log stdout; empty means seed nothing (fixed-string path).
	}{
		{
			name:     "commit-subject list",
			rel:      config.Release{},
			subjects: "Shared subject one\nShared subject two\n",
		},
		{
			name:     "fixed fallback string",
			rel:      config.Release{Fallback: "A fixed shared body."},
			subjects: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// ResolveFailure in fallback mode shares the rel.Fallback selector.
			resolveRel := tc.rel
			resolveRel.OnNotesFailure = "fallback"
			resolveRunner := newFallbackRunner(t, tc.subjects)
			resolveBody, resolveErr := notes.ResolveFailure(t.Context(), resolveRunner, ai.ErrGenerationFailed, "v5.0.0", resolveRel)
			if resolveErr != nil {
				t.Fatalf("ResolveFailure returned unexpected error: %v", resolveErr)
			}

			// NoAIBody must produce an identical body for the same inputs.
			noAIRunner := newFallbackRunner(t, tc.subjects)
			noAIBody, noAIErr := notes.NoAIBody(t.Context(), noAIRunner, "v5.0.0", tc.rel)
			if noAIErr != nil {
				t.Fatalf("NoAIBody returned unexpected error: %v", noAIErr)
			}

			if noAIBody != resolveBody {
				t.Errorf("NoAIBody body = %q, want it to equal ResolveFailure fallback body %q", noAIBody, resolveBody)
			}
		})
	}
}

func TestNoAIBody_GitFails_SurfacesGenuineError(t *testing.T) {
	t.Parallel()

	// --no-ai never aborts from the no-AI policy itself, but a GENUINE git failure
	// (the commit-subject log runs and exits non-zero) still surfaces as an error
	// rather than silently fabricating a body.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: bad revision\n", ExitCode: 128}, errors.New("exit status 128"))

	_, err := notes.NoAIBody(t.Context(), r, "v9.9.9", config.Release{})
	if err == nil {
		t.Fatal("NoAIBody returned nil error on a genuine git failure, want it surfaced")
	}
}

// newFallbackRunner returns a FakeRunner ready for the fallback-body selector: when
// subjects is non-empty it seeds the single `git log` call with that stdout; when
// empty (the fixed-string path) it seeds nothing, since no git call is made.
func newFallbackRunner(t *testing.T, subjects string) *runner.FakeRunner {
	t.Helper()
	if subjects == "" {
		return runner.NewFakeRunner()
	}
	return seedCommitSubjects(t, subjects)
}
