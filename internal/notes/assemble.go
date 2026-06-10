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
// holds the runner (so production wiring passes the os/exec-backed runner while
// tests pass a FakeRunner — no real git, fully scriptable) and the configured
// diff_exclude globs layered on top of the built-in CHANGELOG.md exclusion.
type Assembler struct {
	runner       runner.CommandRunner
	excludeGlobs []string
}

// NewAssembler builds an Assembler over r with the configured diff_exclude globs.
// The runner is injected for the same seam-testability reason as the sibling
// engine packages. excludeGlobs are the extra project-artifact globs (config's
// diff_exclude) excluded ON TOP OF the built-in CHANGELOG.md — each becomes its own
// :(exclude)<glob> pathspec, in order, after CHANGELOG.md. A nil/empty excludeGlobs
// reproduces exactly the built-in-only behaviour (only CHANGELOG.md excluded).
func NewAssembler(r runner.CommandRunner, excludeGlobs []string) *Assembler {
	return &Assembler{runner: r, excludeGlobs: excludeGlobs}
}

// excludePathspecs returns the ordered :(exclude) pathspec arguments shared by the
// diff and Change Map git calls: FIRST the built-in :(exclude)CHANGELOG.md, THEN one
// :(exclude)<glob> per configured diff_exclude glob, in config order. git interprets
// each glob as a pathspec pattern — mint does NO Go-side glob matching — so a glob
// matching nothing is harmless and a force-added tracked file matching a glob is
// excluded by git like any other path.
func (a *Assembler) excludePathspecs() []string {
	pathspecs := make([]string, 0, 1+len(a.excludeGlobs))
	pathspecs = append(pathspecs, changelogExcludePathspec)
	for _, glob := range a.excludeGlobs {
		pathspecs = append(pathspecs, ":(exclude)"+glob)
	}
	return pathspecs
}

// AssembleDiff returns the release diff for lastTag..HEAD with CHANGELOG.md and the
// configured diff_exclude globs excluded, as git's raw post-exclusion stdout, ready
// for downstream layering (the Change Map preamble and max_diff_lines cap are applied
// by later layers, not here).
//
// The diff is produced by `git diff {lastTag}..HEAD -- . {excludePathspecs}`, where
// excludePathspecs is `:(exclude)CHANGELOG.md` followed by one `:(exclude)<glob>` per
// configured diff_exclude glob, run cwd-relative like the other engine git calls. GIT
// performs every exclusion via the :(exclude) pathspecs; mint does no Go-side hunk
// stripping or glob matching. The returned text is git's stdout verbatim — whatever
// git emits flows through unfiltered. A consequence: a gitignored-but-force-added file
// is tracked, so git includes it in this commit-to-commit diff unless a configured
// glob excludes it; that path-based (never commit-based) behaviour is git's and is NOT
// special-cased.
//
// A normal EMPTY diff (e.g. the only change was CHANGELOG.md, now excluded) is
// NOT an error — the empty string is returned for the degenerate path downstream.
// A missing git binary is surfaced as a condition matching runner.ErrCommandNotFound
// (via errors.Is); any other non-zero git exit is surfaced as a wrapped error so an
// unexpected failure is never mistaken for an empty diff.
func (a *Assembler) AssembleDiff(ctx context.Context, lastTag string) (string, error) {
	args := append([]string{"diff", lastTag + "..HEAD", "--", "."}, a.excludePathspecs()...)
	res, err := a.runner.Run(ctx, "git", args...)
	if err != nil {
		return "", fmt.Errorf("assembling release diff for %s..HEAD: %w", lastTag, err)
	}

	return res.Stdout, nil
}
