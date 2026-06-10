// Package notes is the ASSEMBLY half of mint's release-notes engine — the
// git-aware, release-specific side that builds what the AI describes (the
// content-agnostic AI transport lives in the sibling internal/ai package). This
// is the boundary the spec draws between context assembly and AI transport:
// quality work on the diff/context changes only this side, never transport.
//
// Phase 2 implements the diff base and the built-in always-exclude: the
// last_tag..HEAD diff with CHANGELOG.md filtered out. The Change Map preamble,
// the configurable diff_exclude globs, strategy-aware version_file exclusion, and
// the max_diff_lines cap are later layers — deliberately not here.
package notes

import (
	"context"
	"fmt"

	"mint/internal/runner"
)

// changelogExcludePathspec is the built-in, NON-configurable always-exclude:
// CHANGELOG.md is pure mint output and never meaningful source, so it is dropped
// from the diff in both the forward and regenerate paths. The exclusion is
// performed by GIT via the :(exclude) pathspec — mint does no Go-side text
// stripping — so a change set whose only modification is CHANGELOG.md yields an
// empty post-exclusion diff straight from git.
const changelogExcludePathspec = ":(exclude)CHANGELOG.md"

// Assembler builds the release diff context through the CommandRunner seam. It
// holds only the runner so production wiring passes the os/exec-backed runner
// while tests pass a FakeRunner — no real git, fully scriptable.
type Assembler struct {
	runner runner.CommandRunner
}

// NewAssembler builds an Assembler over r. The runner is injected for the same
// seam-testability reason as the sibling engine packages.
func NewAssembler(r runner.CommandRunner) *Assembler {
	return &Assembler{runner: r}
}

// AssembleDiff returns the release diff for lastTag..HEAD with CHANGELOG.md
// excluded, as git's raw post-exclusion stdout, ready for downstream layering
// (the Change Map preamble and max_diff_lines cap are applied by later layers,
// not here).
//
// The diff is produced by `git diff {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'`,
// run cwd-relative like the other engine git calls. GIT performs the CHANGELOG.md
// exclusion via the :(exclude) pathspec; mint does no Go-side hunk stripping. The
// returned text is git's stdout verbatim — whatever git emits flows through
// unfiltered. A consequence: a gitignored-but-force-added file is tracked, so git
// includes it in this commit-to-commit diff and it appears here; that edge is
// deliberate and is NOT special-cased.
//
// A normal EMPTY diff (e.g. the only change was CHANGELOG.md, now excluded) is
// NOT an error — the empty string is returned for the degenerate path downstream.
// A missing git binary is surfaced as a condition matching runner.ErrCommandNotFound
// (via errors.Is); any other non-zero git exit is surfaced as a wrapped error so an
// unexpected failure is never mistaken for an empty diff.
func (a *Assembler) AssembleDiff(ctx context.Context, lastTag string) (string, error) {
	res, err := a.runner.Run(ctx, "git", "diff", lastTag+"..HEAD", "--", ".", changelogExcludePathspec)
	if err != nil {
		return "", fmt.Errorf("assembling release diff for %s..HEAD: %w", lastTag, err)
	}

	return res.Stdout, nil
}
