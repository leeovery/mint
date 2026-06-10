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

// CommitBookkeeping builds the single Phase 1 release-bookkeeping commit: it
// stages the changelog change and commits it with subject
// `{commitPrefix} Release {tag}` (e.g. the default 🌿 prefix yields
// `🌿 Release v0.0.1`). Both git invocations target dir (the repo root) via
// `git -C {dir}` so they are independent of the process working directory.
//
// changed is the net-change signal from WriteChangelog: when false there is
// nothing to stage, so CommitBookkeeping is a no-op and creates NO commit — mint
// never makes an empty commit. A non-zero `git add` exit short-circuits before
// the commit so a failed stage can never produce a commit; the error is surfaced
// for the orchestrator to unwind.
//
// Phase 1 records exactly this one commit. The separate hook-artifact commit and
// the version-file projection fold-in are Phase 3 and deliberately not here.
func CommitBookkeeping(ctx context.Context, r runner.CommandRunner, dir, commitPrefix, tag string, changed bool) error {
	if !changed {
		return nil
	}

	if _, err := r.Run(ctx, "git", "-C", dir, "add", changelogFileName); err != nil {
		return fmt.Errorf("staging %s: %w", changelogFileName, err)
	}

	subject := fmt.Sprintf("%s Release %s", commitPrefix, tag)
	if _, err := r.Run(ctx, "git", "-C", dir, "commit", "-m", subject); err != nil {
		return fmt.Errorf("committing release bookkeeping %q: %w", subject, err)
	}
	return nil
}
