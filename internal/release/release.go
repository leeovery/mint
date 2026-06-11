// Package release performs mint's point-of-no-return git step: it creates the
// annotated release tag and pushes the commits and tag together with a single
// atomic push. Both git mutations go through the runner.CommandRunner seam so the
// exact argv and the piped tag message are scripted and asserted in tests without
// touching real git.
//
// The tag is always ANNOTATED (`git tag -a … -F -`), never lightweight: the
// annotation body is the single source mint ever reads later (regenerate --reuse
// reads it back, parse-free). The atomic push (`git push --atomic origin HEAD
// {tag}`) is the single point of no return — commits and tag go up together or
// not at all. On a successful push the returned Outcome signals that the point of
// no return has been crossed, so the orchestrator knows publish may proceed and
// any later failure is warn-only; on a rejected push the failure surfaces and the
// run stops while still pre-PONR.
//
// This package surfaces failures and stops. The surgical local auto-unwind (delete
// the tag, reset the commits) and lock-resilient git wrapping live in a later phase
// and are deliberately not here — this unit calls the runner directly.
package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/git"
	"mint/internal/record"
	"mint/internal/runner"
)

// ErrPushRejected is returned (wrapped) when the atomic push ran and exited
// non-zero (the remote rejected it). Callers branch on it with errors.Is to tell
// a ran-and-rejected push apart from a failure that occurred before the push (tag
// creation, or a missing git binary), because the two demand different handling:
// a pre-push failure never reached the remote, whereas a rejected push still did
// not cross the point of no return but failed at the moment of crossing.
var ErrPushRejected = errors.New("push rejected")

// Outcome reports the result of TagAndPush. PointOfNoReturnCrossed is true only
// when the atomic push succeeded: from that instant the commits and tag are
// public, so the orchestrator may publish and must treat any later failure as
// warn-only. It is false on every failure path (tag creation, push rejection,
// missing git), all of which leave the release pre-PONR.
type Outcome struct {
	PointOfNoReturnCrossed bool
}

// Releaser creates the annotated tag and performs the atomic push through the
// injected lock-resilient git Mutator. Both the tag and the push are MUTATIONS, so
// they flow through Mutate (a contended `.git` lock is retried, a stale lock cleared)
// rather than the raw runner. Production wiring passes a Mutator over the os/exec
// runner; tests pass a Mutator over a FakeRunner so the git invocations are scripted
// and asserted.
type Releaser struct {
	mutator *git.Mutator
}

// NewReleaser builds a Releaser that issues its git mutations through m.
func NewReleaser(m *git.Mutator) *Releaser {
	return &Releaser{mutator: m}
}

// TagAndPush creates the annotated release tag at the current HEAD and then pushes
// the commits and tag together atomically.
//
// The tag is created with `git tag -a {tag} -F -`, the message piped on stdin:
// the subject `{commitPrefix} Release {tag}` (e.g. `🌿 Release v0.0.1`), a blank
// line, then the full notes body verbatim. -a/-F guarantees an annotated tag
// carrying that body — the single source mint reads later.
//
// The push is the exact form `git push --atomic origin HEAD {tag}` so commits and
// tag go up together or not at all — the single point of no return.
//
// On success the returned Outcome has PointOfNoReturnCrossed set, signalling the
// orchestrator may publish. A failure creating the tag surfaces as an error and
// the push is never attempted (still pre-PONR). A rejected push surfaces wrapped in
// ErrPushRejected so the orchestrator can tell it apart from a pre-tag failure and
// stop; this unit never publishes. A missing git binary surfaces matching
// ErrCommandNotFound and is distinct from a ran-and-rejected push.
func (rel *Releaser) TagAndPush(ctx context.Context, tag, commitPrefix, body string) (Outcome, error) {
	if err := rel.createAnnotatedTag(ctx, tag, commitPrefix, body); err != nil {
		return Outcome{}, err
	}

	if err := rel.atomicPush(ctx, tag); err != nil {
		return Outcome{}, err
	}

	return Outcome{PointOfNoReturnCrossed: true}, nil
}

// createAnnotatedTag runs `git tag -a {tag} -F -`, piping the composed annotation
// message on stdin so the body is never packed into an argv arg (it is long and
// multiline). A non-zero exit (including a missing git binary, which matches
// ErrCommandNotFound) surfaces as-is; it is deliberately NOT wrapped in
// ErrPushRejected because no push has happened yet.
func (rel *Releaser) createAnnotatedTag(ctx context.Context, tag, commitPrefix, body string) error {
	message := composeTagMessage(tag, commitPrefix, body)
	if _, err := rel.mutator.Mutate(ctx, strings.NewReader(message), "git", "tag", "-a", tag, "-F", "-"); err != nil {
		return fmt.Errorf("creating annotated tag %q: %w", tag, err)
	}
	return nil
}

// atomicPush runs `git push --atomic origin HEAD {tag}`, sending the commits and
// the tag together — the single point of no return. A missing git binary
// (ErrCommandNotFound) surfaces as an infrastructure failure and is left
// unwrapped by ErrPushRejected; a ran-and-exited-non-zero push (the remote
// rejected it) is wrapped in ErrPushRejected so the caller can distinguish it from
// a pre-tag failure and stop without publishing.
func (rel *Releaser) atomicPush(ctx context.Context, tag string) error {
	_, err := rel.mutator.Mutate(ctx, nil, "git", "push", "--atomic", "origin", "HEAD", tag)
	if err == nil {
		return nil
	}
	if errors.Is(err, runner.ErrCommandNotFound) {
		return fmt.Errorf("pushing tag %q atomically: %w", tag, err)
	}
	return fmt.Errorf("pushing tag %q atomically: %w: %w", tag, ErrPushRejected, err)
}

// composeTagMessage builds the annotation message: subject
// `{commitPrefix} Release {tag}`, a blank line, then the full notes body verbatim.
func composeTagMessage(tag, commitPrefix, body string) string {
	subject := record.BookkeepingSubject(commitPrefix, tag)
	return subject + "\n\n" + body
}
