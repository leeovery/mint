package record

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/git"
)

// CommitDirtyTree commits whatever a pre_tag hook left dirty as its OWN commit,
// kept distinct from the release-bookkeeping commit. It first probes the tree with
// `git -C {dir} status --porcelain` (the clean-tree convention: non-empty output is
// dirty — tracked changes and/or non-ignored untracked files; gitignored entries
// are omitted by porcelain, so they never count as dirty). When the tree is dirty it
// stages everything (`git -C {dir} add -A`) and commits it with subject (e.g.
// `chore(release): pre-tag artifacts for {tag}`), returning committed=true.
//
// When the tree is clean (empty porcelain) it commits NOTHING and returns
// committed=false. This single check naturally handles every no-commit case: a hook
// that built nothing, a hook that made its OWN commit and handed back a clean tree,
// and gitignored-only outputs. "Commit only if changed" falls out for free.
//
// A non-zero `git status` or `git add` exit short-circuits before the next step (so
// a failed probe or stage can never produce a commit) and the error is surfaced for
// the orchestrator to abort/unwind on.
//
// A keepOut path is left out of the probe, the stage and the commit: its changes
// neither trigger the commit nor ride in it, and any of them already staged stay
// staged (see CommitStaged). When that path's staged edits are all the index holds,
// no commit is made and committed is false.
func CommitDirtyTree(ctx context.Context, m *git.Mutator, dir, subject string, keepOut git.Exclusion) (bool, error) {
	// The status probe is a READ, so it goes through the Mutator's plain pass-through
	// (m.Run) — no lock logic. Only the add/commit mutations carry lock resilience.
	status, err := m.Run(ctx, "git", withExclusion(keepOut, "-C", dir, "status", "--porcelain")...)
	if err != nil {
		return false, fmt.Errorf("probing tree status in %s: %w", dir, err)
	}
	if strings.TrimSpace(status.Stdout) == "" {
		return false, nil
	}

	if _, err := m.Mutate(ctx, nil, "git", withExclusion(keepOut, "-C", dir, "add", "-A")...); err != nil {
		return false, fmt.Errorf("staging dirty tree in %s: %w", dir, err)
	}
	committed, err := CommitStaged(ctx, m, dir, subject, keepOut)
	if err != nil {
		return false, fmt.Errorf("committing dirty tree %q: %w", subject, err)
	}
	return committed, nil
}

// CommitBookkeeping builds the single release-bookkeeping commit, FOLDING the
// changelog change and the version-file projection into ONE commit with subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). Every git invocation targets dir (the repo root) via
// `git -C {dir}` so it is independent of the process working directory.
//
// changelogChanged and versionChanged are the net-change signals from
// WriteChangelog and ProjectVersionFile. Only what actually changed is staged: the
// changelog when changelogChanged, and versionFile when versionChanged (and a
// versionFile path is configured) — mint never `git add`s an untouched or absent
// path. Whatever changed is staged in ONE `git -C {dir} add {paths…}` invocation,
// so the version file is NEVER given its own separate commit — it is always folded
// here alongside the changelog.
//
// COMBINED NO-OP: when neither the changelog nor the version file changed there is
// nothing to stage, so CommitBookkeeping is a no-op and creates NO commit — mint
// never makes an empty commit. A non-zero `git add` exit short-circuits before the
// commit so a failed stage can never produce a commit; the error is surfaced for
// the orchestrator to unwind.
//
// This bookkeeping commit is kept DISTINCT from the pre_tag hook-artifact commit
// (CommitDirtyTree), which precedes it with its own chore subject.
//
// A keepOut path is left out of the commit, so edits to it the operator had already
// staged neither ride in the commit nor lose their staged state (see CommitStaged).
// Should nothing else be staged, it fails rather than report a commit it never made.
func CommitBookkeeping(ctx context.Context, m *git.Mutator, dir, commitPrefix, tag, versionFile string, changelogChanged, versionChanged bool, keepOut git.Exclusion) error {
	// The no-op branch consumes the SAME shared predicate the engine spine consults
	// to fold the resulting commit into its tracked state, so the two can never desync.
	if !BookkeepingWillCommit(versionFile, changelogChanged, versionChanged) {
		return nil
	}

	paths := bookkeepingPaths(versionFile, changelogChanged, versionChanged)
	addArgs := append([]string{"-C", dir, "add"}, paths...)
	if _, err := m.Mutate(ctx, nil, "git", addArgs...); err != nil {
		return fmt.Errorf("staging %v: %w", paths, err)
	}

	subject := BookkeepingSubject(commitPrefix, tag)
	committed, err := CommitStaged(ctx, m, dir, subject, keepOut)
	if err != nil {
		return fmt.Errorf("committing release bookkeeping %q: %w", subject, err)
	}
	if !committed {
		return fmt.Errorf("committing release bookkeeping %q: %w", subject, ErrNothingStaged)
	}
	return nil
}

// CommitFile stages path in dir by name and commits it with subject. The commit takes
// the whole index, so it carries path alone only when nothing else is staged.
func CommitFile(ctx context.Context, m *git.Mutator, dir, path, subject string) error {
	if _, err := m.Mutate(ctx, nil, "git", "-C", dir, "add", path); err != nil {
		return fmt.Errorf("staging %s: %w", path, err)
	}
	if _, err := m.Mutate(ctx, nil, "git", "-C", dir, "commit", "-m", subject); err != nil {
		return fmt.Errorf("committing %q: %w", subject, err)
	}
	return nil
}

// ErrNothingStaged reports a commit that had nothing to take once keepOut was left out.
var ErrNothingStaged = errors.New("nothing staged to commit")

// CommitStaged commits the index in dir with subject, reporting whether a commit was
// made. Only when keepOut has staged edits does it name the other staged paths on
// stdin instead: a bare exclusion pathspec on `git commit` would commit the work-tree
// content of every other tracked path, sweeping in unstaged and assume-unchanged
// edits. The named paths are still committed from the work tree, which matches the
// index only for paths staged just before the call. The list is never empty when the
// commit runs, since an empty pathspec file commits the whole index, keepOut's staged
// edits included. On this pathspec route git runs the commit hooks against a
// temporary index, so a hook that re-stages its rewrite leaves the real index behind.
func CommitStaged(ctx context.Context, m *git.Mutator, dir, subject string, keepOut git.Exclusion) (bool, error) {
	keepOutStaged, err := hasStagedEdits(ctx, m, dir, keepOut)
	if err != nil {
		return false, err
	}
	if !keepOutStaged {
		if _, err := m.Mutate(ctx, nil, "git", "-C", dir, "commit", "-m", subject); err != nil {
			return false, err
		}
		return true, nil
	}

	staged, err := m.Run(ctx, "git", withExclusion(keepOut, "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames")...)
	if err != nil {
		return false, fmt.Errorf("listing staged paths in %s: %w", dir, err)
	}
	if staged.Stdout == "" {
		return false, nil
	}
	if _, err := m.Mutate(ctx, literalPathspecs(staged.Stdout), "git", "-C", dir, "commit", "-m", subject, "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return false, err
	}
	return true, nil
}

// hasStagedEdits reports whether keepOut names a path with edits in the index.
func hasStagedEdits(ctx context.Context, m *git.Mutator, dir string, keepOut git.Exclusion) (bool, error) {
	if keepOut == "" {
		return false, nil
	}
	res, err := m.Run(ctx, "git", "-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", git.LiteralPathspec(string(keepOut)))
	if err != nil {
		return false, fmt.Errorf("checking %s for staged edits: %w", keepOut, err)
	}
	return res.Stdout != "", nil
}

// literalPathspecs turns a NUL-terminated path list into NUL-terminated root-anchored
// literal pathspecs. The magic goes on each entry rather than `--literal-pathspecs`,
// which would export GIT_LITERAL_PATHSPECS to the commit hooks and break their globs.
func literalPathspecs(paths string) []byte {
	var b strings.Builder
	for _, path := range strings.Split(strings.TrimSuffix(paths, "\x00"), "\x00") {
		b.WriteString(git.LiteralPathspec(path))
		b.WriteByte(0)
	}
	return []byte(b.String())
}

// BookkeepingSubject is the SINGLE source of the release-bookkeeping commit subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). The actual bookkeeping commit (CommitBookkeeping), the
// dry-run plan, and the annotated-tag subject line all obtain the subject from here,
// so a format change can never desync the plan or the tag from the real commit.
func BookkeepingSubject(commitPrefix, tag string) string {
	return fmt.Sprintf("%s Release %s", commitPrefix, tag)
}

// BookkeepingWillCommit is the SINGLE commit-or-not predicate for the bookkeeping
// commit: it reports whether CommitBookkeeping will make a commit for the given
// net-change signals — exactly when bookkeepingPaths yields something to stage (the
// changelog and/or a configured version file changed). CommitBookkeeping's own no-op
// branch consumes it, and the engine spine consults it to fold the resulting commit
// into its MadeState the moment it lands WITHOUT re-probing git — so the spine's
// commit tracking can never desync from the commit rule.
func BookkeepingWillCommit(versionFile string, changelogChanged, versionChanged bool) bool {
	return len(bookkeepingPaths(versionFile, changelogChanged, versionChanged)) > 0
}

// bookkeepingPaths assembles, in stage order, the paths the bookkeeping commit
// stages: the changelog when it changed, then the version file when it changed and
// a path is configured. An empty result is the combined no-op — nothing to stage,
// so no commit.
func bookkeepingPaths(versionFile string, changelogChanged, versionChanged bool) []string {
	var paths []string
	if changelogChanged {
		paths = append(paths, ChangelogFileName)
	}
	if versionChanged && versionFile != "" {
		paths = append(paths, versionFile)
	}
	return paths
}

func withExclusion(keepOut git.Exclusion, args ...string) []string {
	return append(args, keepOut.Pathspec()...)
}
