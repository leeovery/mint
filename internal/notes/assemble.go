// Package notes is the ASSEMBLY half of mint's release-notes engine — the
// git-aware, release-specific side that builds what the AI describes (the
// content-agnostic AI transport lives in the sibling internal/ai package). This
// is the boundary the spec draws between context assembly and AI transport:
// quality work on the diff/context changes only this side, never transport.
//
// The diff base and exclusion tiers are layered in: the last_tag..HEAD diff with
// the built-in CHANGELOG.md always-exclude (Phase 2), the configurable diff_exclude
// globs, and the strategy-aware version_file exclusion (plain mode excludes the
// bookkeeping file; embedded mode keeps the real source). The max_diff_lines cap is
// a later layer — deliberately not here.
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

// versionFileExcludePathspec is the STRATEGY-AWARE decision for the configured
// version_file: whether it is excluded from the diff, and if so as which :(exclude)
// pathspec. The version_file is deliberately NOT blanket-excluded — the mode decides:
//
//   - versionFile == "" → ("", false): no version_file configured, nothing for this rule.
//   - versionFile set AND versionPattern == "" → PLAIN mode: the WHOLE file is the
//     version (e.g. release.txt), pure bookkeeping with no source signal, so EXCLUDE it:
//     (":(exclude)"+versionFile, true).
//   - versionFile set AND versionPattern != "" → EMBEDDED mode: the version line lives
//     inside a REAL source file (e.g. main.go) we WANT in notes, so do NOT exclude:
//     ("", false). The lone version-line bump is neutralised by the default prompt's
//     ignore-version-bumps rule (task 2-5), not by hiding real code.
//
// FORWARD-PATH NOTE: notes generate (Stage 4) runs BEFORE the version write (Stage 5),
// so the version file is inherently UNCHANGED at notes time on the forward path — the
// decision is therefore INERT in practice there (the pathspec is carried but excludes
// nothing). The rule exists so the regenerate path (Phase 5), which diffs a tag range
// that already contains the version write, inherits a CORRECT decision. The decision is
// computed and unit-tested here regardless of the path that consumes it.
func versionFileExcludePathspec(versionFile, versionPattern string) (pathspec string, exclude bool) {
	if versionFile == "" {
		return "", false
	}
	if versionPattern != "" {
		// Embedded mode: the version line is in real source we want in the notes.
		return "", false
	}
	// Plain mode: the whole file is the version — pure bookkeeping, exclude it.
	return ":(exclude)" + versionFile, true
}

// ExcludeConfig is the consolidated diff-exclusion input the Assembler layers on top
// of the built-in CHANGELOG.md always-exclude. It bundles the configurable diff_exclude
// globs with the strategy-aware version_file decision so the constructor signature is
// STABLE across exclusion tiers — the same struct feeds the forward path here and the
// regenerate path (Phase 5), which inherits the identical version_file rule. The final
// exclusion set is the UNION of the tiers (built-in + globs + strategy version_file);
// duplicate :(exclude) entries (e.g. a version_file also listed in Globs) are harmless,
// git tolerates them, so there is deliberately no de-dup.
type ExcludeConfig struct {
	// Globs are the extra project-artifact pathspec globs (config's diff_exclude),
	// excluded in order ON TOP OF CHANGELOG.md. nil/empty adds no glob entries.
	Globs []string
	// VersionFile is the configured version_file path (config's version_file). Empty
	// means tag-only (no projection) — nothing for the strategy rule.
	VersionFile string
	// VersionPattern is the configured version_pattern (config's version_pattern). Empty
	// with a set VersionFile selects PLAIN mode (exclude the file); non-empty selects
	// EMBEDDED mode (do NOT exclude — real source we want in the notes).
	VersionPattern string
}

// Assembler builds the release diff context through the CommandRunner seam. It
// holds the runner (so production wiring passes the os/exec-backed runner while
// tests pass a FakeRunner — no real git, fully scriptable) and the consolidated
// ExcludeConfig (diff_exclude globs + the strategy-aware version_file decision)
// layered on top of the built-in CHANGELOG.md exclusion.
type Assembler struct {
	runner  runner.CommandRunner
	exclude ExcludeConfig
}

// NewAssembler builds an Assembler over r with the consolidated ExcludeConfig. The
// runner is injected for the same seam-testability reason as the sibling engine
// packages. exclude carries the diff_exclude globs and the version_file strategy
// inputs, all excluded ON TOP OF the built-in CHANGELOG.md. A zero ExcludeConfig
// (no globs, no version_file) reproduces exactly the built-in-only behaviour (only
// CHANGELOG.md excluded).
func NewAssembler(r runner.CommandRunner, exclude ExcludeConfig) *Assembler {
	return &Assembler{runner: r, exclude: exclude}
}

// excludePathspecs returns the ordered :(exclude) pathspec arguments shared by the
// diff and Change Map git calls — the UNION of the exclusion tiers in a fixed order:
// FIRST the built-in :(exclude)CHANGELOG.md, THEN one :(exclude)<glob> per configured
// diff_exclude glob (in config order), THEN — only in PLAIN mode — the strategy-aware
// :(exclude)<version_file> (embedded mode / no version_file appends nothing here). git
// interprets each glob as a pathspec pattern — mint does NO Go-side glob matching — so a
// glob matching nothing is harmless and a force-added tracked file matching a glob is
// excluded by git like any other path. A version_file ALSO listed in Globs is excluded
// by the glob tier regardless of mode; the resulting duplicate (in plain mode) is
// harmless and not de-duplicated.
func (a *Assembler) excludePathspecs() []string {
	pathspecs := make([]string, 0, 2+len(a.exclude.Globs))
	pathspecs = append(pathspecs, changelogExcludePathspec)
	for _, glob := range a.exclude.Globs {
		pathspecs = append(pathspecs, ":(exclude)"+glob)
	}
	if pathspec, exclude := versionFileExcludePathspec(a.exclude.VersionFile, a.exclude.VersionPattern); exclude {
		pathspecs = append(pathspecs, pathspec)
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
