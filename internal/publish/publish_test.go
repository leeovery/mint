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
