package engine

import (
	"context"
	"fmt"
	"strings"

	"mint/internal/git"
	"mint/internal/record"
)

// userChangeError is files mint is about to write holding a change it did not make:
// writing them would take the change into mint's commit, or an unwind would discard
// it, so the run stops before writing anything.
type userChangeError struct{ paths []string }

func (e *userChangeError) Error() string {
	if len(e.paths) == 1 {
		return e.paths[0] + " changed while mint ran; commit or stash the change and run again"
	}
	return joinAnd(e.paths) + " changed while mint ran; commit or stash the changes and run again"
}

// recordPaths are the files the record stage writes: the changelog where it is kept,
// then the version file where one is set.
func recordPaths(versionFile string, changelog bool) []string {
	var paths []string
	if changelog {
		paths = append(paths, record.ChangelogFileName)
	}
	if versionFile != "" {
		paths = append(paths, versionFile)
	}
	return paths
}

// refuseUserChanges stops the run before mint writes paths, files of its own in the
// repo at root, where any of them holds a change against HEAD, staged, unstaged or
// untracked: as the run starts from a clean tree, such a change was made while it ran,
// by the user or a tool of theirs. HEAD holds every commit mint made this run, so what
// mint wrote and committed earlier counts as no change. A file git ignores shows no
// change, so it is never refused. Untracked files are read as the clean-tree gate reads
// them, under the user's status.showUntrackedFiles, so a file the gate let through is
// never refused here: with it set to no, an untracked file shows no change. The error is a *userChangeError naming the changed
// files, or the read's own failure.
func refuseUserChanges(ctx context.Context, m *git.Mutator, root string, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"-C", root, "--no-optional-locks", "status", "--porcelain", "-z", "--no-renames", "--"}
	for _, path := range paths {
		args = append(args, git.LiteralPathspec(path))
	}
	res, err := m.Run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("checking %s for changes: %w", joinAnd(paths), record.NewGitError(res.Stderr, err))
	}
	var changed []string
	for _, entry := range strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 }) {
		// "XY path": the path starts after the two status letters and a space.
		if len(entry) > 3 {
			changed = append(changed, entry[3:])
		}
	}
	if len(changed) > 0 {
		return &userChangeError{paths: changed}
	}
	return nil
}
