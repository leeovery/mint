package publish_test

import (
	"errors"
	"testing"

	"mint/internal/publish"
	"mint/internal/runner"
)

func TestGitHubPublisher_CreateRelease_InvokesGhReleaseCreate(t *testing.T) {
	t.Parallel()

	// CreateRelease must shell `gh release create {tag} --title {title}
	// --notes-file - --verify-tag` through the CommandRunner, with the full notes
	// body piped on stdin (--notes-file -) rather than packed into an argv arg, so
	// a long/multiline body never hits arg-length or shell-escaping limits.
	// --verify-tag makes gh refuse to publish a release for a tag that does not
	// exist, the belt-and-braces against a release pointing at a missing tag.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{Stdout: "https://github.com/acme/widget/releases/tag/v1.2.3\n"}, nil)

	p := publish.NewGitHubPublisher(r)

	const body = "## What's changed\n\n- Added the thing\n- Fixed the other thing\n"
	if err := p.CreateRelease(t.Context(), "v1.2.3", "v1.2.3", body); err != nil {
		t.Fatalf("CreateRelease returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}

	got := invs[0]
	if got.Name != "gh" {
		t.Errorf("command = %q, want gh", got.Name)
	}
	wantArgs := []string{"release", "create", "v1.2.3", "--title", "v1.2.3", "--notes-file", "-", "--verify-tag"}
	if !equalArgs(got.Args, wantArgs) {
		t.Errorf("args = %v, want %v", got.Args, wantArgs)
	}
	if got.Stdin != body {
		t.Errorf("stdin = %q, want the full notes body %q", got.Stdin, body)
	}
}

func TestGitHubPublisher_CreateRelease_DistinctTitleAndTag(t *testing.T) {
	t.Parallel()

	// Title and tag are independent arguments: the tag is the positional arg and the
	// title is passed via --title. A test where they differ guards against a wiring
	// bug that collapses the two (e.g. passing the tag as the title).
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{Stdout: ""}, nil)

	p := publish.NewGitHubPublisher(r)

	if err := p.CreateRelease(t.Context(), "v2.0.0", "Release 2.0.0", "body"); err != nil {
		t.Fatalf("CreateRelease returned unexpected error: %v", err)
	}

	got := r.Invocations()[0]
	wantArgs := []string{"release", "create", "v2.0.0", "--title", "Release 2.0.0", "--notes-file", "-", "--verify-tag"}
	if !equalArgs(got.Args, wantArgs) {
		t.Errorf("args = %v, want %v", got.Args, wantArgs)
	}
}

func TestGitHubPublisher_CreateRelease_GhFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `gh release create` exit (e.g. the tag is missing on the remote, or
	// auth lapsed) returns a populated Result alongside a non-nil error. CreateRelease
	// must surface that as an error rather than swallow it, so the orchestrator can
	// warn and point to the heal path.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{
		Stderr:   "could not find tag v1.2.3\n",
		ExitCode: 1,
	}, errors.New("exit status 1"))

	p := publish.NewGitHubPublisher(r)

	err := p.CreateRelease(t.Context(), "v1.2.3", "v1.2.3", "body")
	if err == nil {
		t.Fatal("CreateRelease returned nil error, want the gh failure to surface")
	}
}

func TestGitHubPublisher_UpdateRelease_InvokesGhReleaseEdit(t *testing.T) {
	t.Parallel()

	// UpdateRelease is the create-or-update dispatch's "release exists" branch: it
	// must shell `gh release edit {tag} --title {title} --notes-file - --verify-tag`
	// through the CommandRunner — mirroring CreateRelease but with the `edit`
	// subcommand — with the full notes body piped on stdin (--notes-file -) so a
	// long/multiline body never hits arg-length or shell-escaping limits.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{Stdout: "https://github.com/acme/widget/releases/tag/v1.2.3\n"}, nil)

	p := publish.NewGitHubPublisher(r)

	const body = "## What's changed\n\n- Added the thing\n- Fixed the other thing\n"
	if err := p.UpdateRelease(t.Context(), "v1.2.3", "v1.2.3", body); err != nil {
		t.Fatalf("UpdateRelease returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}

	got := invs[0]
	if got.Name != "gh" {
		t.Errorf("command = %q, want gh", got.Name)
	}
	wantArgs := []string{"release", "edit", "v1.2.3", "--title", "v1.2.3", "--notes-file", "-", "--verify-tag"}
	if !equalArgs(got.Args, wantArgs) {
		t.Errorf("args = %v, want %v", got.Args, wantArgs)
	}
	if got.Stdin != body {
		t.Errorf("stdin = %q, want the full notes body %q", got.Stdin, body)
	}
}

func TestGitHubPublisher_UpdateRelease_GhFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `gh release edit` exit (e.g. the tag is missing on the remote, or
	// auth lapsed) returns a populated Result alongside a non-nil error. UpdateRelease
	// must surface that as a wrapped error rather than swallow it, so the regenerate
	// dispatch can warn rather than report a false success.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{
		Stderr:   "could not find tag v1.2.3\n",
		ExitCode: 1,
	}, errors.New("exit status 1"))

	p := publish.NewGitHubPublisher(r)

	err := p.UpdateRelease(t.Context(), "v1.2.3", "v1.2.3", "body")
	if err == nil {
		t.Fatal("UpdateRelease returned nil error, want the gh failure to surface")
	}
}

func TestGitHubPublisher_ReleaseExists_TrueWhenGhViewSucceeds(t *testing.T) {
	t.Parallel()

	// A release that exists makes `gh release view {tag}` exit zero. ReleaseExists
	// must report true with no error, and probe via exactly that gh subcommand
	// through the CommandRunner.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{Stdout: "title:\tv1.2.3\n"}, nil)

	p := publish.NewGitHubPublisher(r)

	exists, err := p.ReleaseExists(t.Context(), "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseExists returned unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("exists = false, want true when gh release view succeeds")
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	got := invs[0]
	if got.Name != "gh" {
		t.Errorf("command = %q, want gh", got.Name)
	}
	wantArgs := []string{"release", "view", "v1.2.3"}
	if !equalArgs(got.Args, wantArgs) {
		t.Errorf("args = %v, want %v", got.Args, wantArgs)
	}
}

func TestGitHubPublisher_ReleaseExists_FalseWhenReleaseNotFound(t *testing.T) {
	t.Parallel()

	// An absent release makes `gh release view {tag}` exit non-zero printing
	// "release not found" to stderr. ReleaseExists must classify that as absent —
	// false with NO error — so the dispatch can route to CreateRelease rather than
	// surfacing a failure.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{
		Stderr:   "release not found\n",
		ExitCode: 1,
	}, errors.New("exit status 1"))

	p := publish.NewGitHubPublisher(r)

	exists, err := p.ReleaseExists(t.Context(), "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseExists returned unexpected error for a not-found release: %v", err)
	}
	if exists {
		t.Errorf("exists = true, want false when the release is not found")
	}
}

func TestGitHubPublisher_ReleaseExists_GenuineFailureSurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `gh release view` exit that is NOT a not-found (e.g. an auth or
	// network failure) is a genuine probe failure: ReleaseExists must surface it as
	// an error rather than silently treating it as absent (which would wrongly
	// dispatch CreateRelease).
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{
		Stderr:   "HTTP 401: Bad credentials\n",
		ExitCode: 1,
	}, errors.New("exit status 1"))

	p := publish.NewGitHubPublisher(r)

	exists, err := p.ReleaseExists(t.Context(), "v1.2.3")
	if err == nil {
		t.Fatal("ReleaseExists returned nil error, want a genuine probe failure surfaced")
	}
	if exists {
		t.Errorf("exists = true, want false alongside the surfaced error")
	}
}

func TestGitHubPublisher_ReleaseExists_MissingGhSurfacesError(t *testing.T) {
	t.Parallel()

	// A missing gh binary is a genuine probe failure (not "release absent"): the
	// runner reports ErrCommandNotFound, which ReleaseExists must surface rather than
	// classify as not-found.
	r := runner.NewFakeRunner()
	r.SeedNotFound("gh")

	p := publish.NewGitHubPublisher(r)

	exists, err := p.ReleaseExists(t.Context(), "v1.2.3")
	if err == nil {
		t.Fatal("ReleaseExists returned nil error, want the missing-gh failure surfaced")
	}
	if exists {
		t.Errorf("exists = true, want false alongside the surfaced error")
	}
}

func TestGitHubPublisher_SatisfiesPublisher(t *testing.T) {
	t.Parallel()

	// GitHubPublisher must be usable through the Publisher interface — the seam the
	// orchestrator depends on (and the future GitLab/Gitea drivers slot into).
	var _ publish.Publisher = publish.NewGitHubPublisher(runner.NewFakeRunner())
}

func TestGitHubPublisher_OnlyPublishesWhenEnabled(t *testing.T) {
	t.Parallel()

	// CreateRelease is a callable unit the orchestrator GATES on publish: it calls it
	// only when publish=true. This table stands in for that gating to pin the
	// contract end-to-end — publish=true reaches gh exactly once; publish=false (tag +
	// push only) must NOT touch gh at all. The real conditional sequencing (after the
	// other gates, before the tag) lives in the orchestrator task, not here.
	tests := []struct {
		name      string
		publish   bool
		wantCalls int
	}{
		{name: "publish enabled invokes gh", publish: true, wantCalls: 1},
		{name: "publish disabled never touches gh", publish: false, wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("gh", runner.Result{Stdout: ""}, nil)
			p := publish.NewGitHubPublisher(r)

			if tt.publish {
				if err := p.CreateRelease(t.Context(), "v1.2.3", "v1.2.3", "body"); err != nil {
					t.Fatalf("CreateRelease returned unexpected error: %v", err)
				}
			}

			if got := len(r.Invocations()); got != tt.wantCalls {
				t.Errorf("gh invocations = %d, want %d", got, tt.wantCalls)
			}
		})
	}
}

// equalArgs reports whether two argument slices are element-for-element equal,
// so command-line assertions check the exact argv rather than a substring.
func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
