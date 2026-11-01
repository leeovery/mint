package record

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"mint/internal/git"
)

// TrackedSpelling is path, a file under root as RepoPath names it, as git spells it in
// the index where the two differ in case alone and name the same file on disk, as they
// do on a case-insensitive filesystem: there `git add version.txt` stages git's
// VERSION.txt, and a literal pathspec of version.txt then matches nothing. The index
// entries matching path in any case are read with icase magic. A path git tracks as
// spelled, or tracks in no spelling the disk shares, comes back as it is. One the disk
// shares with several entries is an error naming them, as no one spelling is git's.
// Links are compared as themselves, never their targets, as git tracks a link as a
// file of its own. A failed read's error carries git's stderr as a *GitError.
func TrackedSpelling(ctx context.Context, m *git.Mutator, root, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	res, err := m.Run(ctx, "git", "-C", root, "ls-files", "-z", "--", ":(top,icase,literal)"+path)
	if err != nil {
		return "", fmt.Errorf("reading how git spells %s: %w", path, NewGitError(res.Stderr, err))
	}
	listed := strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 })
	if slices.Contains(listed, path) {
		return path, nil
	}
	own, err := os.Lstat(filepath.Join(root, path))
	if err != nil {
		return path, nil
	}
	var same []string
	for _, entry := range listed {
		if info, err := os.Lstat(filepath.Join(root, entry)); err == nil && os.SameFile(own, info) {
			same = append(same, entry)
		}
	}
	switch len(same) {
	case 0:
		return path, nil
	case 1:
		return same[0], nil
	default:
		return "", fmt.Errorf("version_file %s is the file git tracks as %s; set it to one of them", path, strings.Join(same, " and "))
	}
}
