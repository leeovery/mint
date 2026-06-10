// Package publish is mint's provider-abstracted release-publishing seam. Creating
// the provider release (the GitHub release, the GitLab release, …) is first-class
// in mint — it is NOT a post_release hook, because a hook would re-spread the
// copy-paste of `gh release create --notes … --verify-tag` across every repo and
// would break heal/regenerate, whose reuse path recreates the provider release
// from the tag annotation. Owning it behind a small interface keeps that logic in
// one place.
//
// The Publisher interface is the seam; GitHubPublisher is the only driver
// implemented now, shelling `gh` through the runner.CommandRunner so the fragile
// process handling is scriptable in tests. Additional drivers (GitLab via glab,
// Gitea, …) drop in behind the same interface with zero rework — the interface is
// the cheap future-proofing; extra drivers are YAGNI until needed.
//
// Provider auto-detection from the remote host and the unknown-provider /
// no-driver loud-downgrade live elsewhere (a later phase); this package assumes
// the driver has already been chosen.
package publish

import (
	"context"
	"fmt"
	"strings"

	"mint/internal/runner"
)

// Publisher is the provider-publishing seam: the contract the release
// orchestration depends on, independent of which forge (GitHub, GitLab, …) backs
// it. Keeping it to two methods keeps the abstraction strong and every driver
// cheap to implement.
//
// CreateRelease publishes a brand-new provider release for tag, with the given
// title and full notes body. UpdateRelease overwrites the release for an existing
// tag (the heal/regenerate --reuse path recreates a provider release from the tag
// annotation); it is part of the seam from the start so drivers implement the
// whole contract, but it is exercised by a later phase.
type Publisher interface {
	CreateRelease(ctx context.Context, tag, title, body string) error
	UpdateRelease(ctx context.Context, tag, title, body string) error
}

// GitHubPublisher is the GitHub Publisher driver. It shells the `gh` CLI through
// the CommandRunner seam (never spawning a process directly), so its command
// construction and error handling are asserted in tests without a real gh or a
// real network. The gh install + auth precondition is verified separately by
// preflight.CheckGhAuth before any tag is created, so by the time CreateRelease
// runs gh is known to be present and authenticated.
type GitHubPublisher struct {
	runner runner.CommandRunner
}

// Compile-time assertion that GitHubPublisher satisfies the seam.
var _ Publisher = (*GitHubPublisher)(nil)

// NewGitHubPublisher builds a GitHubPublisher that issues its gh commands through
// r. The runner is injected so production wiring passes the os/exec-backed runner
// and tests pass a FakeRunner.
func NewGitHubPublisher(r runner.CommandRunner) *GitHubPublisher {
	return &GitHubPublisher{runner: r}
}

// CreateRelease publishes a new GitHub release for tag via
// `gh release create {tag} --title {title} --notes-file - --verify-tag`.
//
// The full notes body is piped on stdin through --notes-file - (RunWith), never
// packed into an argv arg: a release body is long and multiline, so an argv arg
// risks OS arg-length limits and shell-escaping breakage, and stdin sidesteps both
// without writing a temp file. --verify-tag makes gh refuse to create a release
// for a tag that does not exist on the remote — defence in depth against
// publishing a release that points at a missing tag.
//
// A non-zero gh exit (the runner returns a populated Result alongside a non-nil
// error) surfaces as an error so the orchestrator can warn and point at the heal
// path; mint never unwinds a published tag.
func (p *GitHubPublisher) CreateRelease(ctx context.Context, tag, title, body string) error {
	_, err := p.runner.RunWith(
		ctx,
		strings.NewReader(body),
		"gh", "release", "create", tag,
		"--title", title,
		"--notes-file", "-",
		"--verify-tag",
	)
	if err != nil {
		return fmt.Errorf("creating GitHub release for tag %q: %w", tag, err)
	}
	return nil
}

// UpdateRelease overwrites the GitHub release for an existing tag. It is part of
// the Publisher seam from the start so the contract is whole, but it is wired into
// the run by a later phase (the heal/regenerate --reuse path); Phase 1 only
// exercises CreateRelease. It is intentionally unimplemented to avoid shipping
// untested release-mutation behaviour ahead of the phase that drives it.
func (p *GitHubPublisher) UpdateRelease(ctx context.Context, tag, title, body string) error {
	return fmt.Errorf("publish: UpdateRelease for tag %q is not yet wired (deferred to the heal/regenerate phase)", tag)
}
