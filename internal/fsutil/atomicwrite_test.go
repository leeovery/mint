package fsutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"mint/internal/fsutil"
)

// listTempFiles returns the names of any leftover atomic-write temp files in dir
// (the ".tmp-*" siblings WriteFile creates), so a test can assert none survived a
// call. A clean run leaves only the final target.
func listTempFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp-*"))
	if err != nil {
		t.Fatalf("globbing temp files in %s: %v", dir, err)
	}
	return matches
}

func TestWriteFile_CreatesTargetWithContentAndPerm(t *testing.T) {
	t.Parallel()

	// Happy path: WriteFile creates the target holding exactly the given bytes,
	// with exactly the requested permission, and leaves no temp file behind.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	want := []byte("hello world\n")

	if err := fsutil.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("WriteFile returned unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("perm = %o, want %o", perm, 0o644)
	}

	if leftover := listTempFiles(t, dir); len(leftover) != 0 {
		t.Errorf("leftover temp files after successful write: %v", leftover)
	}
}

func TestWriteFile_OverwritesExistingTargetWithGivenPerm(t *testing.T) {
	t.Parallel()

	// Overwrite path: an existing target is replaced atomically with the new bytes
	// and the requested perm, leaving no temp file behind.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old content"), 0o600); err != nil {
		t.Fatalf("seeding existing target: %v", err)
	}

	want := []byte("new content\n")
	if err := fsutil.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("WriteFile returned unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("perm = %o, want %o", perm, 0o644)
	}

	if leftover := listTempFiles(t, dir); len(leftover) != 0 {
		t.Errorf("leftover temp files after overwrite: %v", leftover)
	}
}

func TestWriteFile_CleansUpTempFileWhenRenameFails(t *testing.T) {
	t.Parallel()

	// Cleanup branch: when the final rename fails, the temp file must be removed and
	// the target left untouched. A rename ONTO an existing directory fails (the temp
	// is a regular file), which exercises the os.Remove(tmp) cleanup after the temp
	// file was created, written, closed, and chmod'd.
	dir := t.TempDir()
	path := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("creating directory at target path: %v", err)
	}

	err := fsutil.WriteFile(path, []byte("payload"), 0o644)
	if err == nil {
		t.Fatalf("WriteFile returned nil error, want a rename failure")
	}

	// The directory at the target path is unchanged (still a directory, not clobbered).
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat target after failed write: %v", statErr)
	}
	if !info.IsDir() {
		t.Errorf("target is no longer a directory after failed rename")
	}

	if leftover := listTempFiles(t, dir); len(leftover) != 0 {
		t.Errorf("leftover temp files after failed rename: %v", leftover)
	}
}
