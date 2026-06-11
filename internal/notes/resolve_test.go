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

// wantCommitSubjectArgs is the exact git argv FallbackBody must issue to build the
// commit-subject list since the last tag: `git log --format=%s {lastTag}..HEAD`.
// This metadata is a FALLBACK RECORD ONLY — it is NEVER fed to the AI.
func wantCommitSubjectArgs(lastTag string) []string {
	return []string{"log", "--format=%s", lastTag + "..HEAD"}
}

// seedCommitSubjects scripts the single `git log` call FallbackBody makes, returning
// the given subjects stdout.
func seedCommitSubjects(t *testing.T, subjects string) *runner.FakeRunner {
	t.Helper()
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: subjects}, nil)
	return r
}

func TestFallbackBody_BuildsCommitSubjectListSinceLastTag(t *testing.T) {
	t.Parallel()

	// The default fallback body is the commit-subject list since the last tag, built
	// via `git log --format=%s {lastTag}..HEAD` through the CommandRunner seam. The
	// returned body is git's stdout verbatim.
	const subjects = "Add login flow\nFix token refresh\nTidy imports\n"
	r := seedCommitSubjects(t, subjects)

	body, err := notes.FallbackBody(t.Context(), r, "v1.2.3")
	if err != nil {
		t.Fatalf("FallbackBody returned unexpected error: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1 (git log)", len(invs))
	}
	assertGitArgv(t, invs[0], wantCommitSubjectArgs("v1.2.3"))
}

func TestFallbackBody_GitFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A `git log` that runs and exits non-zero is surfaced as an error rather than
	// silently producing an empty fallback record.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: bad revision\n", ExitCode: 128}, errors.New("exit status 128"))

	body, err := notes.FallbackBody(t.Context(), r, "v9.9.9")
	if err == nil {
		t.Fatal("FallbackBody returned nil error on git failure, want it surfaced")
	}
	if body != "" {
		t.Errorf("body = %q, want empty on failure", body)
	}
}

func TestResolveFailure_AbortDefault_ReturnsAbortErrorNamingCauseTaggingNothing(t *testing.T) {
	t.Parallel()

	// Default policy ("abort", absent OnNotesFailure) aborts: ResolveFailure returns
	// NO body and an abort error that NAMES the cause AND still matches the original
	// sentinel via errors.Is. No git log is run — abort tags nothing and builds no
	// fallback record.
	r := runner.NewFakeRunner()
	rel := config.Release{} // OnNotesFailure == "" → abort.

	body, err := notes.ResolveFailure(t.Context(), r, ai.ErrTimeout, "v1.0.0", rel)
	if err == nil {
		t.Fatal("ResolveFailure returned nil error in abort mode, want an abort error")
	}
	if !errors.Is(err, ai.ErrTimeout) {
		t.Errorf("error = %v, want it to match the original cause ai.ErrTimeout", err)
	}
	if body != "" {
		t.Errorf("body = %q, want empty in abort mode (tag nothing)", body)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git was invoked %d times in abort mode, want 0 (no fallback record)", len(r.Invocations()))
	}
}

func TestResolveFailure_AbortExplicit_BehavesLikeDefault(t *testing.T) {
	t.Parallel()

	// An explicit "abort" is identical to the empty-string default.
	r := runner.NewFakeRunner()
	rel := config.Release{OnNotesFailure: "abort"}

	body, err := notes.ResolveFailure(t.Context(), r, notes.ErrDiffTooLarge, "v1.0.0", rel)
	if err == nil {
		t.Fatal("ResolveFailure returned nil error for explicit abort, want an abort error")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
	if body != "" {
		t.Errorf("body = %q, want empty in abort mode", body)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git was invoked %d times in abort mode, want 0", len(r.Invocations()))
	}
}

func TestResolveFailure_Fallback_ReturnsCommitSubjectListBody(t *testing.T) {
	t.Parallel()

	// "fallback" proceeds with the DEFAULT fallback body: the commit-subject list,
	// built via `git log --format=%s {lastTag}..HEAD`, with NO error. The fallback
	// produces a body regardless of which cause triggered it.
	const subjects = "Ship the engine\nWire the resolver\n"
	r := seedCommitSubjects(t, subjects)
	rel := config.Release{OnNotesFailure: "fallback"}

	body, err := notes.ResolveFailure(t.Context(), r, ai.ErrGenerationFailed, "v2.0.0", rel)
	if err != nil {
		t.Fatalf("ResolveFailure returned unexpected error in fallback mode: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1 (git log for the subject list)", len(invs))
	}
	assertGitArgv(t, invs[0], wantCommitSubjectArgs("v2.0.0"))
}

func TestResolveFailure_FallbackWithFixedString_ReturnsThatStringNoGitLog(t *testing.T) {
	t.Parallel()

	// In fallback mode with a fixed [release].fallback string set, ResolveFailure
	// returns that exact string, no error, and runs NO git log (the fixed string is
	// the body, so the commit-subject record is not built). The fixed string is now
	// sourced from the dedicated rel.Fallback key, NOT from on_notes_failure (which is
	// mode-only).
	const fixed = "Notes unavailable — see commit history."
	r := runner.NewFakeRunner()
	rel := config.Release{OnNotesFailure: "fallback", Fallback: fixed}

	body, err := notes.ResolveFailure(t.Context(), r, ai.ErrCommandMissing, "v3.0.0", rel)
	if err != nil {
		t.Fatalf("ResolveFailure returned unexpected error for a fixed fallback string: %v", err)
	}
	if body != fixed {
		t.Errorf("body = %q, want the fixed fallback string %q", body, fixed)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git was invoked %d times for a fixed fallback string, want 0", len(r.Invocations()))
	}
}

func TestResolveFailure_UnknownMode_AbortsTaggingNothing(t *testing.T) {
	t.Parallel()

	// on_notes_failure is MODE-ONLY (abort | fallback). Any value that is not
	// "fallback" — including an unknown string — resolves to ABORT for Phase 2: it
	// returns NO body and an abort error naming the cause, and runs NO git. (Phase 6's
	// typed validation will reject unknown values up front; this resolver just treats
	// them as abort defensively.) The old "literal value as fixed body" overload is
	// gone — the fixed string now comes only from rel.Fallback.
	r := runner.NewFakeRunner()
	rel := config.Release{OnNotesFailure: "something-unknown"}

	body, err := notes.ResolveFailure(t.Context(), r, ai.ErrCommandMissing, "v3.0.0", rel)
	if err == nil {
		t.Fatal("ResolveFailure returned nil error for an unknown mode, want an abort error")
	}
	if !errors.Is(err, ai.ErrCommandMissing) {
		t.Errorf("error = %v, want it to match the cause ai.ErrCommandMissing", err)
	}
	if body != "" {
		t.Errorf("body = %q, want empty (unknown mode aborts, tags nothing)", body)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git was invoked %d times for an unknown mode, want 0", len(r.Invocations()))
	}
}

func TestResolveFailure_FallbackEmptyLog_ReturnsNonEmptyFloor(t *testing.T) {
	t.Parallel()

	// In fallback mode with no fixed string and an EMPTY commit log (no commits since
	// the last tag), the body must NOT be empty — the selector floors it to a
	// non-empty minimal record so the tag is never empty. The git log is still run
	// (there is no fixed string to short-circuit it).
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "   \n\t\n"}, nil) // whitespace-only log.
	rel := config.Release{OnNotesFailure: "fallback"}

	body, err := notes.ResolveFailure(t.Context(), r, ai.ErrGenerationFailed, "v4.0.0", rel)
	if err != nil {
		t.Fatalf("ResolveFailure returned unexpected error in fallback mode: %v", err)
	}
	if strings.TrimSpace(body) == "" {
		t.Errorf("body = %q, want a non-empty floor when the commit log is empty", body)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1 (git log attempted before flooring)", len(invs))
	}
	assertGitArgv(t, invs[0], wantCommitSubjectArgs("v4.0.0"))
}

func TestResolveFailure_VariedCauses_RouteThroughBothModes(t *testing.T) {
	t.Parallel()

	// VARIED CAUSES all route the SAME way: abort NAMES whichever cause (and keeps it
	// matchable via errors.Is); fallback produces the commit-subject body regardless of
	// cause. Each known sentinel maps to a readable cause phrase in the abort message.
	cases := []struct {
		name      string
		cause     error
		causeText string
	}{
		{"timeout", ai.ErrTimeout, "AI timed out"},
		{"missing tool", ai.ErrCommandMissing, "AI tool not installed"},
		{"empty after retry", ai.ErrGenerationFailed, "AI returned empty/invalid notes after retry"},
		{"diff too large", notes.ErrDiffTooLarge, "diff too large"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// ABORT branch: names the cause and stays matchable; no fallback record.
			abortRunner := runner.NewFakeRunner()
			_, abortErr := notes.ResolveFailure(t.Context(), abortRunner, tc.cause, "v1.0.0", config.Release{OnNotesFailure: "abort"})
			if abortErr == nil {
				t.Fatalf("abort mode returned nil error for cause %v, want an abort error", tc.cause)
			}
			if !errors.Is(abortErr, tc.cause) {
				t.Errorf("abort error = %v, want it to match the cause %v", abortErr, tc.cause)
			}
			if !errorContains(abortErr, tc.causeText) {
				t.Errorf("abort error = %q, want it to name the cause %q", abortErr.Error(), tc.causeText)
			}
			if len(abortRunner.Invocations()) != 0 {
				t.Errorf("abort mode invoked git %d times for cause %v, want 0", len(abortRunner.Invocations()), tc.cause)
			}

			// FALLBACK branch: yields the commit-subject body regardless of cause.
			const subjects = "One subject\nTwo subject\n"
			fbRunner := seedCommitSubjects(t, subjects)
			body, fbErr := notes.ResolveFailure(t.Context(), fbRunner, tc.cause, "v1.0.0", config.Release{OnNotesFailure: "fallback"})
			if fbErr != nil {
				t.Fatalf("fallback mode returned unexpected error for cause %v: %v", tc.cause, fbErr)
			}
			if body != subjects {
				t.Errorf("fallback body = %q for cause %v, want the commit-subject list %q", body, tc.cause, subjects)
			}
		})
	}
}

func TestResolveFailure_UnknownCause_AbortFallsBackToFailureMessage(t *testing.T) {
	t.Parallel()

	// An unknown cause (not one of the mapped sentinels) still aborts, naming the
	// cause from the failure's OWN message rather than a mapped phrase, and remaining
	// matchable via errors.Is.
	unknown := errors.New("some unmapped notes failure")
	r := runner.NewFakeRunner()

	_, err := notes.ResolveFailure(t.Context(), r, unknown, "v1.0.0", config.Release{OnNotesFailure: "abort"})
	if err == nil {
		t.Fatal("ResolveFailure returned nil error for an unknown cause, want an abort error")
	}
	if !errors.Is(err, unknown) {
		t.Errorf("error = %v, want it to match the unknown cause", err)
	}
	if !errorContains(err, "some unmapped notes failure") {
		t.Errorf("error = %q, want it to carry the unknown failure's own message", err.Error())
	}
}

// errorContains reports whether err's message contains sub.
func errorContains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}
