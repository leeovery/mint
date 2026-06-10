package record

import (
	"context"
	"fmt"
	"strings"

	"mint/internal/runner"
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
func CommitDirtyTree(ctx context.Context, r runner.CommandRunner, dir, subject string) (bool, error) {
	status, err := r.Run(ctx, "git", "-C", dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("probing tree status in %s: %w", dir, err)
	}
	if strings.TrimSpace(status.Stdout) == "" {
		return false, nil
	}

	if _, err := r.Run(ctx, "git", "-C", dir, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging dirty tree in %s: %w", dir, err)
	}
	if _, err := r.Run(ctx, "git", "-C", dir, "commit", "-m", subject); err != nil {
		return false, fmt.Errorf("committing dirty tree %q: %w", subject, err)
	}
	return true, nil
}

// CommitBookkeeping builds the single release-bookkeeping commit, FOLDING the
// changelog change and the version-file projection into ONE commit with subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). Both git invocations target dir (the repo root) via
// `git -C {dir}` so they are independent of the process working directory.
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
func CommitBookkeeping(ctx context.Context, r runner.CommandRunner, dir, commitPrefix, tag, versionFile string, changelogChanged, versionChanged bool) error {
	paths := bookkeepingPaths(versionFile, changelogChanged, versionChanged)
	if len(paths) == 0 {
		return nil
	}

	addArgs := append([]string{"-C", dir, "add"}, paths...)
	if _, err := r.Run(ctx, "git", addArgs...); err != nil {
		return fmt.Errorf("staging %v: %w", paths, err)
	}

	subject := fmt.Sprintf("%s Release %s", commitPrefix, tag)
	if _, err := r.Run(ctx, "git", "-C", dir, "commit", "-m", subject); err != nil {
		return fmt.Errorf("committing release bookkeeping %q: %w", subject, err)
	}
	return nil
}

// bookkeepingPaths assembles, in stage order, the paths the bookkeeping commit
// stages: the changelog when it changed, then the version file when it changed and
// a path is configured. An empty result is the combined no-op — nothing to stage,
// so no commit.
func bookkeepingPaths(versionFile string, changelogChanged, versionChanged bool) []string {
	var paths []string
	if changelogChanged {
		paths = append(paths, changelogFileName)
	}
	if versionChanged && versionFile != "" {
		paths = append(paths, versionFile)
	}
	return paths
}
