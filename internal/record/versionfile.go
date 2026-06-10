package record

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ProjectVersionFilePlain mirrors version into the PLAIN-mode version file at
// {root}/{versionFile} and reports whether it produced a net change on disk.
//
// PLAIN mode means the WHOLE file contents are the version: there is no surrounding
// source, so the target content is the BARE version string (e.g. "1.4.0") with NO
// tag_prefix — this file is the version MIRROR, not the tag — followed by EXACTLY
// ONE trailing newline (canonical form "1.4.0\n"). This single-trailing-newline
// convention is the contract embedded mode (the version_pattern path) and the tests
// align on, so a file that already holds the bare version but lacks the trailing
// newline is treated as DIFFERENT and rewritten to the canonical form.
//
// The version file is a WRITE-ONLY MIRROR: mint NEVER reads it as a version source
// (Stage 1 is tag-as-truth — the file is derived state). It is read here only to
// detect a no-op.
//
// NO-OP DETECTION: when the file already holds EXACTLY the target content, nothing
// is written and changed is false — so the downstream bookkeeping commit (folded in
// task 3-7) sees nothing to stage and no empty commit is made. Otherwise the target
// content is written (creating the file when absent, overwriting an older/different
// version) via an atomic temp-file+rename so a crash mid-write never leaves a
// truncated mirror, and changed is true.
//
// This produces only the file write and the changed signal; it does NOT create or
// stage any git commit — that is folded into the single bookkeeping commit in task
// 3-7.
func ProjectVersionFilePlain(root, versionFile, version string) (changed bool, err error) {
	path := filepath.Join(root, versionFile)
	target := version + "\n"

	existing, found, err := readVersionFileContent(path)
	if err != nil {
		return false, err
	}
	if found && existing == target {
		return false, nil
	}

	if err := writeFileAtomic(path, target); err != nil {
		return false, err
	}
	return true, nil
}

// readVersionFileContent returns the current contents of the version file. found is
// false (with no error) when the file does not yet exist — the create path — so the
// caller distinguishes "absent" from "present but different". Any other read error
// is surfaced.
func readVersionFileContent(path string) (content string, found bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading version file %s: %w", path, err)
	}
	return string(data), true, nil
}

// writeFileAtomic writes content to path via a temp file in the same directory
// followed by a rename, so a reader never observes a half-written version file. The
// 0o644 mode matches a normal tracked source file. It mirrors writeAtomic (used for
// the changelog) but keeps its own temp-file naming and error context so the two
// write paths stay independent.
func writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp version file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp version file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp version file: %w", err)
	}

	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting version file mode: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing version file %s: %w", path, err)
	}
	return nil
}
