// Package preflight implements mint's local safety gates — the cheap,
// read-only, reversible checks that must pass before any mutation or network
// call. Each gate runs over the CommandRunner seam so it is scriptable in tests
// without a real repository, and reports a typed *GateError carrying an
// actionable abort message on failure.
//
// The local gates, in cheap-first order, are: clean working tree, on the
// release branch, and the target tag free locally. RunLocalGates runs them in
// that order and aborts on the first failure. The network gates (remote sync,
// tag-free remote) and the escape hatches (--autostash, --any-branch) live
// elsewhere / arrive in a later phase; nothing here mutates.
package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/runner"
)

// GateError is the typed failure a preflight gate returns when its check does
// not pass. It carries a human-readable abort message the caller renders
// verbatim; it is distinct from infrastructure errors (e.g. a missing git
// binary) so the caller can tell a failed-but-clean gate apart from a tool that
// could not run at all.
type GateError struct {
	message string
}

// Message returns the actionable abort text for display.
func (e *GateError) Message() string {
	return e.message
}

// Error satisfies the error interface; the message is already display-ready.
func (e *GateError) Error() string {
	return e.message
}

// newGateError builds a *GateError with a formatted abort message.
func newGateError(format string, args ...any) *GateError {
	return &GateError{message: fmt.Sprintf(format, args...)}
}

// RunLocalGates runs the local preflight gates in cheap-first order — clean
// tree, on the release branch, then target tag free locally — and aborts on the
// first failure, returning that gate's error without running the rest. A nil
// return means every local gate passed. Network gates run separately, after
// these.
func RunLocalGates(ctx context.Context, r runner.CommandRunner, releaseBranch, tag string) error {
	if err := CheckCleanTree(ctx, r); err != nil {
		return err
	}
	if err := CheckOnBranch(ctx, r, releaseBranch); err != nil {
		return err
	}
	return CheckTagFreeLocal(ctx, r, tag)
}

// CheckCleanTree enforces a strict clean working tree: `git status --porcelain`
// must be empty. Any porcelain entry — uncommitted/unstaged tracked changes or a
// non-ignored untracked file — fails the gate. Gitignored files are exempt:
// porcelain omits them by default and --ignored is deliberately not passed, so
// build outputs never trip the gate.
func CheckCleanTree(ctx context.Context, r runner.CommandRunner) error {
	res, err := r.Run(ctx, "git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("checking working tree status: %w", err)
	}

	if strings.TrimSpace(res.Stdout) != "" {
		return newGateError("working tree is not clean; commit or stash your changes, then re-run")
	}
	return nil
}

// CheckOnBranch enforces that HEAD is on the release branch. The current branch
// comes from `git rev-parse --abbrev-ref HEAD`; if it differs from releaseBranch
// the gate aborts with a message naming both the current and expected branches.
func CheckOnBranch(ctx context.Context, r runner.CommandRunner, releaseBranch string) error {
	res, err := r.Run(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("resolving current branch: %w", err)
	}

	current := strings.TrimSpace(res.Stdout)
	if current != releaseBranch {
		return newGateError(
			"on branch %q, but the release branch is %q; switch branches, then re-run",
			current, releaseBranch,
		)
	}
	return nil
}

// CheckTagFreeLocal enforces that the computed tag does not already exist
// locally, closing the double-release / re-run footgun. It uses
// `git rev-parse -q --verify refs/tags/{tag}`: a zero exit with a resolved hash
// means the tag exists (the gate aborts), while a clean ran-and-exited-non-zero
// means the tag is absent — the PASS case — and is NOT treated as a hard error.
// A missing git binary (ErrCommandNotFound) is a genuine error and surfaces
// as-is so it is never mistaken for the tag being free.
func CheckTagFreeLocal(ctx context.Context, r runner.CommandRunner, tag string) error {
	res, err := r.Run(ctx, "git", "rev-parse", "-q", "--verify", "refs/tags/"+tag)
	if err != nil {
		// A missing binary is a real failure; a clean non-zero exit (tag absent) is
		// the pass case and is swallowed here.
		if errors.Is(err, runner.ErrCommandNotFound) {
			return fmt.Errorf("verifying local tag %q: %w", tag, err)
		}
		return nil
	}

	if strings.TrimSpace(res.Stdout) != "" {
		return newGateError("tag %q already exists locally; bump the version or delete the tag, then re-run", tag)
	}
	return nil
}
