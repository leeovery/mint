// Package preflight implements mint's local safety gates — the cheap,
// read-only, reversible checks that must pass before any mutation or network
// call. Each gate runs over the CommandRunner seam so it is scriptable in tests
// without a real repository, and reports a typed *GateError carrying an
// actionable abort message on failure.
//
// The local gates, in cheap-first order, are: clean working tree, on the
// release branch, and the target tag free locally. RunLocalGates runs them in
// that order and aborts on the first failure. The network half — Fetch (read-only
// `git fetch --tags`, never a pull), CheckRemoteSync (abort if behind/diverged
// from upstream), and CheckTagFreeRemote — runs after the local gates; the
// orchestrator fetches first, then runs the remote checks. The escape hatches
// (--autostash, --any-branch) live elsewhere; nothing here mutates.
//
// CheckGhAuth (gate 6 — gh installed + authenticated) is the one CONDITIONAL
// gate: the orchestrator runs it only when actually publishing a GitHub release,
// after the other gates and before tag creation, so a missing/unauthenticated gh
// aborts before any tag is pushed. See CheckGhAuth for the full ordering invariant.
package preflight

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"mint/internal/runner"
)

// ErrNoUpstream is the distinguishable condition CheckRemoteSync surfaces when
// the release branch has no tracking remote configured, so `git rev-list
// @{u}...HEAD` cannot resolve an upstream. It is deliberately NOT a *GateError:
// the caller reports "no tracking remote" as its own clearly-distinguishable
// outcome rather than as a behind/diverged abort, and it is distinct from a
// missing-binary (ErrCommandNotFound) infrastructure failure.
var ErrNoUpstream = errors.New("no upstream configured for the release branch")

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
//
// anyBranch is the --any-branch escape hatch: when true the on-release-branch gate
// is SKIPPED ENTIRELY — it is not evaluated, so no `git rev-parse --abbrev-ref HEAD`
// is issued — letting a deliberate off-branch release proceed. The flag weakens ONLY
// the branch gate; the clean-tree and tag-free gates always run regardless.
func RunLocalGates(ctx context.Context, r runner.CommandRunner, releaseBranch, tag string, anyBranch bool) error {
	if err := CheckCleanTree(ctx, r); err != nil {
		return err
	}
	if !anyBranch {
		if err := CheckOnBranch(ctx, r, releaseBranch); err != nil {
			return err
		}
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

// Fetch refreshes the remote refs read-only with `git fetch --tags`, so the
// complete tag set (Stage 1's source of truth) and the upstream refs are current
// before "latest" is trusted and before the remote gates run. It is deliberately
// a fetch, never a pull: mint must never auto-integrate remote commits. A failure
// (including a missing git binary, which matches ErrCommandNotFound) surfaces
// as-is so it is never mistaken for a successful fetch.
func Fetch(ctx context.Context, r runner.CommandRunner) error {
	if _, err := r.Run(ctx, "git", "fetch", "--tags"); err != nil {
		return fmt.Errorf("fetching tags and remote refs: %w", err)
	}
	return nil
}

// CheckRemoteSync compares HEAD against the release branch's upstream and aborts
// the release if local is behind or diverged — never auto-pulling, since silently
// dragging in unseen remote commits and releasing them must be a conscious act.
// It runs `git rev-list --left-right --count @{u}...HEAD`, whose output is
// "<behind>\t<ahead>": behind counts upstream-only commits, ahead counts
// HEAD-only commits. Behind (>0 behind) or diverged (>0 behind AND >0 ahead)
// fails with a *GateError naming the behind count and upstream; up-to-date or
// purely ahead — the expected release state, those being the commits released —
// passes. A branch with no tracking remote yields ErrNoUpstream (a distinguishable
// condition, not a gate abort); a missing git binary surfaces as ErrCommandNotFound.
//
// releaseBranch names the upstream in the abort message (origin/{releaseBranch});
// the comparison itself uses @{u}, which resolves HEAD's upstream — HEAD is
// already guaranteed to be on the release branch by the on-branch gate that runs
// before this one.
func CheckRemoteSync(ctx context.Context, r runner.CommandRunner, releaseBranch string) error {
	res, err := r.Run(ctx, "git", "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		if errors.Is(err, runner.ErrCommandNotFound) {
			return fmt.Errorf("computing remote sync state: %w", err)
		}
		// A non-zero exit here is the no-upstream case: with no tracking branch,
		// `@{u}` cannot resolve and git fatals with "no upstream configured". Surface
		// it as the distinguishable ErrNoUpstream condition rather than crashing.
		return fmt.Errorf("%w: %s", ErrNoUpstream, strings.TrimSpace(res.Stderr))
	}

	behind, _, err := parseLeftRightCount(res.Stdout)
	if err != nil {
		return fmt.Errorf("parsing remote sync count: %w", err)
	}

	// Any commits behind aborts: behind-only and diverged (also ahead) are both
	// unsafe to auto-integrate and share the same behind-count message. behind == 0
	// — up-to-date or purely ahead — passes; ahead is the commits being released.
	if behind > 0 {
		return newGateError(
			"%d commits behind origin/%s — pull and review, then re-run",
			behind, releaseBranch,
		)
	}
	return nil
}

// parseLeftRightCount parses the "<behind>\t<ahead>" output of
// `git rev-list --left-right --count {upstream}...HEAD` into its two counts.
func parseLeftRightCount(stdout string) (behind, ahead int, err error) {
	fields := strings.Fields(stdout)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q, want two counts", strings.TrimSpace(stdout))
	}

	behind, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing behind count %q: %w", fields[0], err)
	}
	ahead, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing ahead count %q: %w", fields[1], err)
	}
	return behind, ahead, nil
}

// CheckTagFreeRemote enforces that the computed tag does not already exist on the
// remote, the remote half of the gate-4 free-tag check (the local half is
// CheckTagFreeLocal). It probes `git ls-remote --tags origin refs/tags/{tag}`: a
// non-empty result means the remote already carries the tag and the gate aborts;
// empty output is the PASS case. A missing git binary (ErrCommandNotFound) is a
// genuine error and surfaces as-is so it is never mistaken for the tag being free.
func CheckTagFreeRemote(ctx context.Context, r runner.CommandRunner, tag string) error {
	res, err := r.Run(ctx, "git", "ls-remote", "--tags", "origin", "refs/tags/"+tag)
	if err != nil {
		return fmt.Errorf("checking remote for tag %q: %w", tag, err)
	}

	if strings.TrimSpace(res.Stdout) != "" {
		return newGateError("tag %q already exists on the remote; bump the version, then re-run", tag)
	}
	return nil
}

// CheckGhAuth is gate 6: the GitHub CLI must be installed AND authenticated. It is
// a CONDITIONAL gate — the orchestrator runs it ONLY when actually publishing a
// GitHub release (publish=true); when publish=false (tag + push only) it is
// skipped entirely and nothing here is called.
//
// ORDERING INVARIANT (load-bearing): when it runs, this gate runs AFTER the other
// preflight gates (local + remote) and BEFORE tag creation. Verifying gh up front
// means a missing or unauthenticated gh aborts the release before any tag is
// created or pushed, so it can never strand a pushed tag with no release to back
// it. The orchestrator (a separate task) owns this sequencing; this gate only
// provides the check.
//
// The probe is `gh auth status`, which exits zero when authenticated and non-zero
// when not. The two failure conditions are deliberately distinguished:
//
//   - gh not installed: the runner reports ErrCommandNotFound. This is a missing
//     prerequisite, surfaced as a *GateError naming "gh not installed".
//   - gh not authenticated: gh ran and exited non-zero (a populated Result
//     alongside a non-nil error, per the runner contract). This is an EXPECTED
//     condition, not an infrastructure crash, so it is branched on and surfaced as
//     a *GateError naming "gh not authenticated" — NOT re-raised as a raw error.
func CheckGhAuth(ctx context.Context, r runner.CommandRunner) error {
	_, err := r.Run(ctx, "gh", "auth", "status")
	if err != nil {
		// A missing gh binary is the not-installed prerequisite failure; any other
		// non-zero exit is gh ran-and-reported not authenticated. Both are clean gate
		// aborts that must stop the release before the tag.
		if errors.Is(err, runner.ErrCommandNotFound) {
			return newGateError("gh is not installed; install GitHub CLI (https://cli.github.com), then re-run — or set publish = false to tag and push only")
		}
		return newGateError("gh is not authenticated; run `gh auth login`, then re-run — or set publish = false to tag and push only")
	}
	return nil
}
