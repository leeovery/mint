package record_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mint/internal/record"
)

// readVersionFile returns the exact bytes of {dir}/{name} as a string, failing the
// test on a read error.
func readVersionFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading version file %s: %v", name, err)
	}
	return string(data)
}

// seedVersionFile writes content to {dir}/{name}, failing the test on error, so a
// test can stage a pre-existing version file before the call under test.
func seedVersionFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("seeding version file %s: %v", name, err)
	}
}

// statModTime returns the modification time of {dir}/{name}, used to assert a no-op
// write left the file physically untouched (no temp-file+rename occurred).
func statModTime(t *testing.T, dir, name string) time.Time {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat version file %s: %v", name, err)
	}
	return info.ModTime()
}

func TestProjectVersionFilePlain_AbsentFile_CreatedWithBareVersion(t *testing.T) {
	t.Parallel()

	// PLAIN mode: the whole file IS the version. With no file present, mint creates
	// it holding the BARE version (no tag_prefix — this is the version mirror, not
	// the tag) followed by exactly one trailing newline. The exact bytes are
	// deterministic, so they are asserted verbatim rather than by substring.
	dir := t.TempDir()

	changed, err := record.ProjectVersionFilePlain(dir, "release.txt", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFilePlain returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (absent file was created)")
	}

	if got := readVersionFile(t, dir, "release.txt"); got != "1.4.0\n" {
		t.Errorf("version file = %q, want %q", got, "1.4.0\n")
	}
}

func TestProjectVersionFilePlain_OlderVersion_Overwritten(t *testing.T) {
	t.Parallel()

	// A file holding an OLDER version is overwritten whole with the new bare version
	// plus a single trailing newline; the change is signalled with changed == true.
	dir := t.TempDir()
	seedVersionFile(t, dir, "release.txt", "1.3.9\n")

	changed, err := record.ProjectVersionFilePlain(dir, "release.txt", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFilePlain returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (older version overwritten)")
	}

	if got := readVersionFile(t, dir, "release.txt"); got != "1.4.0\n" {
		t.Errorf("version file = %q, want %q", got, "1.4.0\n")
	}
}

func TestProjectVersionFilePlain_AlreadyAtTarget_NoOp(t *testing.T) {
	t.Parallel()

	// A file ALREADY holding exactly the target content (bare version + single
	// trailing newline) is a no-op: changed == false so the downstream bookkeeping
	// commit sees nothing to stage. The on-disk bytes must be untouched, and the
	// file must not be rewritten (asserted via a stable mtime).
	dir := t.TempDir()
	seedVersionFile(t, dir, "release.txt", "1.4.0\n")
	before := statModTime(t, dir, "release.txt")

	changed, err := record.ProjectVersionFilePlain(dir, "release.txt", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFilePlain returned unexpected error: %v", err)
	}
	if changed {
		t.Errorf("changed = true, want false (file already at the target version)")
	}

	if got := readVersionFile(t, dir, "release.txt"); got != "1.4.0\n" {
		t.Errorf("version file = %q, want it untouched %q", got, "1.4.0\n")
	}
	if after := statModTime(t, dir, "release.txt"); !after.Equal(before) {
		t.Errorf("version file mtime changed on a no-op: before %v, after %v", before, after)
	}
}

func TestProjectVersionFilePlain_TargetVersionWithoutNewline_Rewritten(t *testing.T) {
	t.Parallel()

	// The trailing-newline convention is canonical and load-bearing: a file holding
	// the target version but WITHOUT the trailing newline ("1.4.0") differs from the
	// canonical target ("1.4.0\n"), so it is rewritten to the canonical form and
	// changed == true. This pins the documented single-trailing-newline convention.
	dir := t.TempDir()
	seedVersionFile(t, dir, "release.txt", "1.4.0")

	changed, err := record.ProjectVersionFilePlain(dir, "release.txt", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFilePlain returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (missing trailing newline is a difference)")
	}

	if got := readVersionFile(t, dir, "release.txt"); got != "1.4.0\n" {
		t.Errorf("version file = %q, want canonical %q", got, "1.4.0\n")
	}
}
