package record

import (
	"context"
	"fmt"

	"mint/internal/runner"
)

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
