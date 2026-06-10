// Package gitrepo anchors every mint operation at the repository root and
// resolves the release branch before the preflight gates run. Both answers come
// from git through the CommandRunner seam, so they are scriptable in tests
// without a real repository.
//
// ResolveRoot underpins config location (.mint.toml lives at the root) and the
// gate set; ResolveReleaseBranch feeds the on-branch gate. The two are split
// because the root is unconditional while the branch is config-overridable.
package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/config"
	"mint/internal/runner"
)

// originHeadPrefix is the leading segment `git symbolic-ref --short` writes in
// front of the branch name (e.g. "origin/main"); stripped to yield the bare
// branch.
const originHeadPrefix = "origin/"

// ErrNotARepository is the distinguishable condition returned by ResolveRoot
// when the invocation directory is not inside a git work tree. Callers branch on
// it with errors.Is to render the gate-1 "not a git repository" abort instead of
// a raw git error.
var ErrNotARepository = errors.New("gitrepo: not a git repository")

// ErrOriginHeadUnset is the distinguishable condition returned by
// ResolveReleaseBranch when no release_branch override is configured and
// origin/HEAD cannot be resolved (no remote, or HEAD never set). It exists so
// the caller can surface a clear message rather than silently defaulting to
// main/master — picking a branch the user never chose is exactly the footgun
// gate 2 must avoid.
var ErrOriginHeadUnset = errors.New("gitrepo: origin/HEAD is unset")

// ResolveRoot returns the absolute repository root via
// `git rev-parse --show-toplevel`, trimmed of its trailing newline. A non-zero
// exit (the directory is not a git work tree) is reported as ErrNotARepository
// so the caller can abort cleanly — never a panic.
//
// Resolution semantics are git's, deliberately unmodified here: --show-toplevel
// resolves to the innermost enclosing repository or linked worktree from the
// invocation directory. A submodule is its own repository (its own tags and its
// own .mint.toml); a linked worktree resolves to the worktree root and shares
// the main repository's ref store. mint anchors to and runs from whatever git
// reports — git is authoritative, so no special-casing is needed.
func ResolveRoot(ctx context.Context, r runner.CommandRunner) (string, error) {
	res, err := r.Run(ctx, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotARepository, err)
	}

	return strings.TrimSpace(res.Stdout), nil
}

// ResolveReleaseBranch returns the branch the on-branch gate enforces. A
// non-empty cfg.Release.ReleaseBranch is an explicit override, used verbatim
// with derivation skipped (git is never consulted). Otherwise the branch is
// derived from origin/HEAD, which resolves main/master with zero config. When
// derivation fails and no override is set, it returns ErrOriginHeadUnset rather
// than guessing a default.
func ResolveReleaseBranch(ctx context.Context, r runner.CommandRunner, cfg config.Config) (string, error) {
	if cfg.Release.ReleaseBranch != "" {
		return cfg.Release.ReleaseBranch, nil
	}

	return deriveFromOriginHead(ctx, r)
}

// deriveFromOriginHead resolves the symbolic ref origin/HEAD points at and
// returns the short branch name with the leading "origin/" stripped. A non-zero
// exit means origin/HEAD is unset, reported as ErrOriginHeadUnset.
func deriveFromOriginHead(ctx context.Context, r runner.CommandRunner) (string, error) {
	res, err := r.Run(ctx, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOriginHeadUnset, err)
	}

	ref := strings.TrimSpace(res.Stdout)
	return strings.TrimPrefix(ref, originHeadPrefix), nil
}
