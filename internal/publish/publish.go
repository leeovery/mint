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
// Provider auto-detection from the remote host lives alongside this seam in
// resolve.go (ResolvePublisher): it parses the remote host across the HTTPS/SSH
// URL forms (overridable by the provider config) and selects a driver, exposing
// ErrProviderUnresolved (wrapped in *UnresolvedError with a named reason) when none
// matches. The engine layers a loud downgrade-to-tag+push on top of that sentinel
// rather than ever silently assuming GitHub.
package publish

import (
	"context"
	"errors"
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
// title and full notes body, and returns the created release's URL. UpdateRelease
// overwrites the release for an existing tag (the heal/regenerate --source tag path
// recreates a provider release from the tag annotation) and returns the updated
// release's URL; it is part of the seam from the start so drivers implement the
// whole contract.
//
// Both return the release URL so the release-success footer can render the real
// release URL the driver published to. An empty URL is valid (the driver simply has
// none to report) and is NOT an error; on a genuine publish failure the driver
// returns an empty URL alongside the wrapped error.
//
// ReleaseExists is the heal/regenerate create-or-update PROBE: it reports whether a
// provider release already exists at tag so DispatchRelease can pick UpdateRelease
// (exists) over CreateRelease (absent) per version. "Absent" is a clean false with
// NO error; a genuine probe failure (missing CLI, auth/network) is surfaced so the
// dispatch never silently defaults to create on a real failure.
//
// ReadReleaseBody is the regenerate provider-release SOURCE read: it returns the
// published release's body verbatim so a regenerate run can re-project the existing
// release notes onto the changelog (or back onto the release). It mirrors the tag
// annotation reuse read (engine.ReadTagBody): hasBody reports whether the release
// carries a usable body, the single branch point both regenerate modes consume
// (single → fail loud, --all → skip-and-continue). An ABSENT release is a clean
// (\"\", false, nil) — there is nothing to read, not a failure; a genuine read
// failure (missing CLI, auth/network) is surfaced.
type Publisher interface {
	CreateRelease(ctx context.Context, tag, title, body string) (url string, err error)
	UpdateRelease(ctx context.Context, tag, title, body string) (url string, err error)
	ReleaseExists(ctx context.Context, tag string) (bool, error)
	ReadReleaseBody(ctx context.Context, tag string) (body string, hasBody bool, err error)
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
//
// On success `gh release create` prints the created release URL to stdout; the
// trimmed stdout is returned as the URL so the success footer renders the real
// release URL. An empty stdout yields an empty URL with no error (the URL is
// optional, not a failure); on a gh failure the URL is empty alongside the error.
func (p *GitHubPublisher) CreateRelease(ctx context.Context, tag, title, body string) (string, error) {
	res, err := p.runner.RunWith(
		ctx,
		strings.NewReader(body),
		"gh", "release", "create", tag,
		"--title", title,
		"--notes-file", "-",
		"--verify-tag",
	)
	if err != nil {
		return "", fmt.Errorf("creating GitHub release for tag %q: %w", tag, err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// notFoundMarker is the substring `gh release view` writes to stderr when no
// release exists at the tag. It is the one signal that distinguishes "the release
// is absent" (a clean false from ReleaseExists) from a genuine probe failure (auth,
// network, …), which gh reports with different stderr and which ReleaseExists must
// surface rather than treat as absent.
const notFoundMarker = "release not found"

// ReleaseExists probes whether a GitHub release exists at tag via
// `gh release view {tag}` through the CommandRunner, returning true when gh exits
// zero (the release is present) and false with NO error when gh reports the release
// is absent.
//
// Classification of a non-zero gh exit is the load-bearing distinction the
// create-or-update dispatch depends on:
//   - "release not found" in stderr → the release is genuinely ABSENT, so this
//     returns (false, nil) and the dispatch routes to CreateRelease.
//   - any other failure — a missing gh binary (ErrCommandNotFound) or any non-zero
//     exit whose stderr is NOT the not-found marker (auth lapse, network, rate
//     limit) — is a GENUINE probe failure and is surfaced as an error, so the
//     dispatch never silently defaults to create-or-update on a real failure.
//
// gh distinguishes these for us: an absent release is the not-found marker, whereas
// other failures carry HTTP/auth stderr — so a marker check is the correct,
// non-heuristic classification.
func (p *GitHubPublisher) ReleaseExists(ctx context.Context, tag string) (bool, error) {
	res, err := p.runner.Run(ctx, "gh", "release", "view", tag)
	if err == nil {
		return true, nil
	}
	// A missing gh binary is never "release absent" — it is a prerequisite failure.
	if errors.Is(err, runner.ErrCommandNotFound) {
		return false, fmt.Errorf("probing GitHub release for tag %q: %w", tag, err)
	}
	// gh ran and exited non-zero: only the not-found marker means absent; anything
	// else (auth, network, …) is a genuine failure that must surface.
	if strings.Contains(res.Stderr, notFoundMarker) {
		return false, nil
	}
	return false, fmt.Errorf("probing GitHub release for tag %q: %w", tag, err)
}

// UpdateRelease overwrites the GitHub release for an existing tag via
// `gh release edit {tag} --title {title} --notes-file - --verify-tag`. It is the
// create-or-update dispatch's "release exists" branch (the heal/regenerate path
// recreates a provider release from the tag annotation), mirroring CreateRelease
// but with the `edit` subcommand.
//
// As in CreateRelease, the full notes body is piped on stdin through --notes-file -
// (RunWith) rather than packed into an argv arg, sidestepping OS arg-length limits
// and shell-escaping breakage on a long/multiline body; --verify-tag makes gh
// refuse to edit a release whose tag does not exist on the remote.
//
// A non-zero gh exit (the runner returns a populated Result alongside a non-nil
// error) surfaces as a wrapped error so the orchestrator can warn rather than
// report a false success.
//
// On success `gh release edit` prints the updated release URL to stdout; the trimmed
// stdout is returned as the URL, mirroring CreateRelease. An empty stdout yields an
// empty URL with no error; on a gh failure the URL is empty alongside the error.
func (p *GitHubPublisher) UpdateRelease(ctx context.Context, tag, title, body string) (string, error) {
	res, err := p.runner.RunWith(
		ctx,
		strings.NewReader(body),
		"gh", "release", "edit", tag,
		"--title", title,
		"--notes-file", "-",
		"--verify-tag",
	)
	if err != nil {
		return "", fmt.Errorf("updating GitHub release for tag %q: %w", tag, err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// ReadReleaseBody reads the published release's body for tag via
// `gh release view {tag} --json body --template {{.body}}` through the CommandRunner.
// The `--template` form (not `--jq`) emits the body bytes WITHOUT jq's appended
// trailing newline, so the body round-trips verbatim — the same verbatim philosophy
// the tag-annotation reuse read (engine.ReadTagBody) follows.
//
// The non-zero classification mirrors ReleaseExists exactly, because an absent release
// is not a read failure:
//   - "release not found" in stderr → the release is ABSENT, so this returns
//     ("", false, nil) and the caller skips (--all) or fails loud (single).
//   - a missing gh binary (ErrCommandNotFound) or any other non-zero exit (auth,
//     network) → a GENUINE read failure, surfaced as a wrapped error.
//
// hasBody is the trim-and-check-empty signal: a release that exists but carries an
// empty/whitespace-only body surfaces as ("…", false, nil), so the caller never writes
// an empty body downstream. The returned body is the RAW template stdout (verbatim,
// never trimmed); only the hasBody decision trims.
func (p *GitHubPublisher) ReadReleaseBody(ctx context.Context, tag string) (string, bool, error) {
	res, err := p.runner.Run(ctx, "gh", "release", "view", tag, "--json", "body", "--template", "{{.body}}")
	if err != nil {
		// A missing gh binary is never "release absent" — it is a prerequisite failure.
		if errors.Is(err, runner.ErrCommandNotFound) {
			return "", false, fmt.Errorf("reading GitHub release body for tag %q: %w", tag, err)
		}
		// gh ran and exited non-zero: only the not-found marker means absent; anything
		// else (auth, network, …) is a genuine failure that must surface.
		if strings.Contains(res.Stderr, notFoundMarker) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading GitHub release body for tag %q: %w", tag, err)
	}
	return res.Stdout, strings.TrimSpace(res.Stdout) != "", nil
}
