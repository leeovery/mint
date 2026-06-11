// Package fsutil holds mint's small, shared filesystem primitives. Its first and
// only member is WriteFile: the single crash-safe atomic-write idiom (temp file in
// the target directory, then rename) that every mint writer of a tracked artifact —
// the changelog, the version-file mirror, and the dry-run note cache — delegates to,
// so the rename-based crash safety lives in ONE tested place rather than in triplicate.
package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes data to path crash-safely: it creates a temp file in path's own
// directory, writes data, closes it, sets its mode to perm, then renames it onto
// path. Because the final step is an atomic rename within one directory, a reader
// (or a crash) never observes a half-written file — path holds either the old
// contents or the complete new contents, never a truncation.
//
// On any failure AFTER the temp file is created it is removed before the error is
// returned, so a failed write leaves no stray temp file and the existing target
// untouched. Errors are wrapped with the step that failed but NOT with any domain
// noun: callers wrap the returned error with their own context (e.g. "changelog",
// "version file") so per-domain wording is preserved at the call site.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting file mode: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing file: %w", err)
	}
	return nil
}
