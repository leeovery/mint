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

	body, err := notes.NoAIBody(t.Context(), r, "v1.0.0")
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

func TestNoAIBody_NeverAbortsEvenWhenAIWouldHaveFailed(t *testing.T) {
	t.Parallel()

	// --no-ai is NOT a failure path: it has no failure input and never aborts — it
	// takes no on_notes_failure at all, and just returns a body and nil error.
	const subjects = "One change\nTwo change\n"
	r := seedCommitSubjects(t, subjects)

	body, err := notes.NoAIBody(t.Context(), r, "v3.0.0")
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

	body, err := notes.NoAIBody(t.Context(), r, "v4.0.0")
	if err != nil {
		t.Fatalf("NoAIBody returned unexpected error on empty log: %v", err)
	}
	if strings.TrimSpace(body) == "" {
		t.Errorf("body = %q, want a non-empty floor when there are no commits", body)
	}
}

func TestNoAIBody_SharesFallbackBuilderWithOnNotesFailureFallback(t *testing.T) {
	t.Parallel()

	// The non-AI body builder is SHARED with on_notes_failure=fallback: for the same
	// inputs, ResolveFailure (fallback mode) and NoAIBody produce the SAME body — the
	// commit-subject list, or the maintenance-release body when that list is empty.
	cases := []struct {
		name     string
		subjects string // git log stdout.
	}{
		{
			name:     "commit-subject list",
			subjects: "Shared subject one\nShared subject two\n",
		},
		{
			name:     "empty commit-subject list",
			subjects: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// ResolveFailure in fallback mode shares the non-AI body selector.
			resolveRel := config.Release{OnNotesFailure: "fallback"}
			resolveRunner := newFallbackRunner(t, tc.subjects)
			resolveBody, resolveErr := notes.ResolveFailure(t.Context(), resolveRunner, ai.ErrGenerationFailed, "v5.0.0", resolveRel)
			if resolveErr != nil {
				t.Fatalf("ResolveFailure returned unexpected error: %v", resolveErr)
			}

			// NoAIBody must produce an identical body for the same inputs.
			noAIRunner := newFallbackRunner(t, tc.subjects)
			noAIBody, noAIErr := notes.NoAIBody(t.Context(), noAIRunner, "v5.0.0")
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

	_, err := notes.NoAIBody(t.Context(), r, "v9.9.9")
	if err == nil {
		t.Fatal("NoAIBody returned nil error on a genuine git failure, want it surfaced")
	}
}

// newFallbackRunner returns a FakeRunner ready for the non-AI body selector, its
// single `git log` call seeded with subjects as stdout.
func newFallbackRunner(t *testing.T, subjects string) *runner.FakeRunner {
	t.Helper()
	return seedCommitSubjects(t, subjects)
}

// TestNonAIBody_EmptySubjectList_ShipsTheMaintenanceBody proves both non-AI routes —
// --no-ai and on_notes_failure = fallback after a failed AI call — ship the
// maintenance-release body when the commit-subject list comes back empty or
// whitespace-only, so the tag body is never empty.
func TestNonAIBody_EmptySubjectList_ShipsTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	routes := []struct {
		name string
		body func(r runner.CommandRunner) (string, error)
	}{
		{
			name: "--no-ai",
			body: func(r runner.CommandRunner) (string, error) {
				return notes.NoAIBody(t.Context(), r, "v6.0.0")
			},
		},
		{
			name: "on_notes_failure = fallback",
			body: func(r runner.CommandRunner) (string, error) {
				return notes.ResolveFailure(t.Context(), r, ai.ErrGenerationFailed, "v6.0.0", config.Release{OnNotesFailure: "fallback"})
			},
		},
	}
	logs := []struct {
		name   string
		stdout string
	}{
		{name: "empty", stdout: ""},
		{name: "whitespace-only", stdout: " \n\t\n"},
	}

	for _, route := range routes {
		for _, log := range logs {
			t.Run(route.name+"/"+log.name, func(t *testing.T) {
				t.Parallel()

				r := seedCommitSubjects(t, log.stdout)
				body, err := route.body(r)
				if err != nil {
					t.Fatalf("returned unexpected error: %v", err)
				}
				if body != notes.StubBody() {
					t.Errorf("body = %q, want the maintenance-release body %q", body, notes.StubBody())
				}
				invs := r.Invocations()
				if len(invs) != 1 {
					t.Fatalf("invocations = %d, want 1 (the commit-subject git log)", len(invs))
				}
				assertGitArgv(t, invs[0], wantCommitSubjectArgs("v6.0.0"))
			})
		}
	}
}

func TestRangeNoAIBody_ListsTheRangesCommitSubjects(t *testing.T) {
	t.Parallel()

	const subjects = "feat: add export\nfix: handle empty input\n"
	r := seedCommitSubjects(t, subjects)

	body, err := notes.RangeNoAIBody(t.Context(), r, "v1.1.0..v1.2.0")
	if err != nil {
		t.Fatalf("RangeNoAIBody returned unexpected error: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}
	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want the one git log", len(invs))
	}
	assertGitArgv(t, invs[0], []string{"log", "--format=%s", "v1.1.0..v1.2.0"})
}

func TestRangeNoAIBody_EmptyRangeGivesTheMaintenanceReleaseBody(t *testing.T) {
	t.Parallel()

	r := seedCommitSubjects(t, "\n")

	body, err := notes.RangeNoAIBody(t.Context(), r, "v1.1.0..v1.2.0")
	if err != nil {
		t.Fatalf("RangeNoAIBody returned unexpected error: %v", err)
	}
	if body != notes.StubBody() {
		t.Errorf("body = %q, want the maintenance-release body %q", body, notes.StubBody())
	}
}

func TestRangeNoAIBody_GitFailureNamesTheRange(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, errors.New("exit status 128"))

	_, err := notes.RangeNoAIBody(t.Context(), r, "v1.1.0..v1.2.0")

	if got, want := err.Error(), "building commit-subject fallback for v1.1.0..v1.2.0: exit status 128"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
}

func TestVersionNoAIBody_ListsTheVersionsSubjectsAsItsForwardReleaseDid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subjects string
		want     string
	}{
		{
			name:     "the bookkeeping commit the tag sits on is left out",
			subjects: "🌿 Release v1.2.0\nfix: handle empty input\nfeat: add export\n",
			want:     "fix: handle empty input\nfeat: add export\n",
		},
		{
			name:     "a bookkeeping commit made under an earlier commit_prefix is left out",
			subjects: "🚀 Release v1.2.0\nfix: handle empty input\nfeat: add export\n",
			want:     "fix: handle empty input\nfeat: add export\n",
		},
		{
			name:     "a tag on a commit that is not its bookkeeping commit leaves the list whole",
			subjects: "fix: handle empty input\nfeat: add export\n",
			want:     "fix: handle empty input\nfeat: add export\n",
		},
		{
			name:     "the pre-tag artifact commit stays listed",
			subjects: "🌿 Release v1.2.0\nchore(release): pre-tag artifacts for v1.2.0\nfeat: add export\n",
			want:     "chore(release): pre-tag artifacts for v1.2.0\nfeat: add export\n",
		},
		{
			name:     "another version's release subject on the tag leaves the list whole",
			subjects: "🌿 Release v1.1.0\nfeat: add export\n",
			want:     "🌿 Release v1.1.0\nfeat: add export\n",
		},
		{
			name:     "a release subject below the tag's commit stays listed",
			subjects: "fix: handle empty input\n🌿 Release v1.2.0\n",
			want:     "fix: handle empty input\n🌿 Release v1.2.0\n",
		},
		{
			name:     "a range holding only the bookkeeping commit gives the maintenance-release body",
			subjects: "🌿 Release v1.2.0\n",
			want:     notes.StubBody(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := seedCommitSubjects(t, tt.subjects)

			body, err := notes.VersionNoAIBody(t.Context(), r, "v1.1.0..v1.2.0", "v1.2.0")
			if err != nil {
				t.Fatalf("VersionNoAIBody returned unexpected error: %v", err)
			}
			if body != tt.want {
				t.Errorf("body = %q, want %q", body, tt.want)
			}
			invs := r.Invocations()
			if len(invs) != 1 {
				t.Fatalf("invocations = %d, want the one git log", len(invs))
			}
			assertGitArgv(t, invs[0], []string{"log", "--format=%s", "v1.1.0..v1.2.0"})
		})
	}
}

func TestVersionNoAIBody_GitFailureNamesTheRange(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, errors.New("exit status 128"))

	_, err := notes.VersionNoAIBody(t.Context(), r, "v1.1.0..v1.2.0", "v1.2.0")

	if got, want := err.Error(), "building commit-subject fallback for v1.1.0..v1.2.0: exit status 128"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
}
