package engine

import (
	"errors"

	"mint/internal/cmdline"
	"mint/internal/presenter"
	"mint/internal/record"
)

// warnIndexBehind reports a commit of mint's that landed with the index left behind it
// (record.IndexBehindError) as a warning under label, git's words beneath, returning
// nil: the commit stands, and the run goes on. Any other err comes back as it is. The
// warning names the command that matches the index to the commit in the repo at root
// by hand, reading the paths from the commit itself, as mint does.
func warnIndexBehind(p presenter.Presenter, root, label string, err error) error {
	var behind *record.IndexBehindError
	if !errors.As(err, &behind) {
		return err
	}
	git := "git -C " + cmdline.Quote(root)
	p.Warn(presenter.Warning{
		Label: label,
		Message: "committed, but the index still holds what the commit hooks staged as it was before them; " +
			"match it to the commit (`" + git + " diff-tree --no-commit-id --name-only -r -z HEAD | " +
			git + " --literal-pathspecs restore --source=HEAD --staged --pathspec-from-file=- --pathspec-file-nul && " +
			git + " update-index -q --refresh`)",
		Output: failedGitOutput(behind.Stderr, behind),
	})
	return nil
}
