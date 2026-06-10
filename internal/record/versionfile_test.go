package record_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

const embeddedPattern = `RELEASE_VERSION="{version}"`

func TestProjectVersionFileEmbedded_SingleMatch_Replaced(t *testing.T) {
	t.Parallel()

	// EMBEDDED mode operates on an existing source file: only the version slot of the
	// configured pattern is rewritten with the new BARE version (X.Y.Z, no tag_prefix,
	// consistent with plain mode), and the surrounding source lines stay byte-for-byte
	// untouched. The change is signalled with changed == true.
	dir := t.TempDir()
	source := "package main\n\nconst RELEASE_VERSION=\"1.3.9\"\n\nfunc main() {}\n"
	seedVersionFile(t, dir, "version.go", source)

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.go", embeddedPattern, "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFileEmbedded returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (version slot rewritten)")
	}

	want := "package main\n\nconst RELEASE_VERSION=\"1.4.0\"\n\nfunc main() {}\n"
	if got := readVersionFile(t, dir, "version.go"); got != want {
		t.Errorf("version file = %q, want %q", got, want)
	}
}

func TestProjectVersionFileEmbedded_MultipleMatches_AllReplaced(t *testing.T) {
	t.Parallel()

	// MULTIPLE matches: the pattern appearing more than once is replaced at EVERY
	// occurrence (legacy sed-replace semantics), so no stale copy of the old version
	// survives anywhere in the file. changed == true.
	dir := t.TempDir()
	source := "RELEASE_VERSION=\"1.3.9\"\n# duplicated below\nRELEASE_VERSION=\"1.3.9\"\n"
	seedVersionFile(t, dir, "version.txt", source)

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.txt", embeddedPattern, "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFileEmbedded returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (both slots rewritten)")
	}

	got := readVersionFile(t, dir, "version.txt")
	want := "RELEASE_VERSION=\"1.4.0\"\n# duplicated below\nRELEASE_VERSION=\"1.4.0\"\n"
	if got != want {
		t.Errorf("version file = %q, want %q", got, want)
	}
	if strings.Contains(got, "1.3.9") {
		t.Errorf("version file still contains a stale old version: %q", got)
	}
}

func TestProjectVersionFileEmbedded_ZeroMatches_FailLoud(t *testing.T) {
	t.Parallel()

	// ZERO matches: a file present but where the configured pattern matches nothing
	// (marker absent / non-semver slot) is a fail-loud abort, NOT a silent skip. It
	// returns record.ErrVersionPatternNoMatch (matchable via errors.Is) wrapped with
	// the path, leaves the file byte-for-byte untouched, and reports changed == false.
	dir := t.TempDir()
	source := "package main\n\nconst OTHER_MARKER=\"1.3.9\"\n"
	seedVersionFile(t, dir, "version.go", source)
	before := statModTime(t, dir, "version.go")

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.go", embeddedPattern, "1.4.0")
	if !errors.Is(err, record.ErrVersionPatternNoMatch) {
		t.Fatalf("error = %v, want it to wrap ErrVersionPatternNoMatch", err)
	}
	if changed {
		t.Errorf("changed = true, want false (no write on a fail-loud abort)")
	}

	if got := readVersionFile(t, dir, "version.go"); got != source {
		t.Errorf("version file = %q, want it untouched %q", got, source)
	}
	if after := statModTime(t, dir, "version.go"); !after.Equal(before) {
		t.Errorf("version file mtime changed on a zero-match abort: before %v, after %v", before, after)
	}
}

func TestProjectVersionFileEmbedded_AbsentFile_FailLoud(t *testing.T) {
	t.Parallel()

	// An ABSENT source file is a zero-match condition under embedded mode (the file is
	// expected to already exist): same fail-loud abort with ErrVersionPatternNoMatch,
	// and no file is created.
	dir := t.TempDir()

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.go", embeddedPattern, "1.4.0")
	if !errors.Is(err, record.ErrVersionPatternNoMatch) {
		t.Fatalf("error = %v, want it to wrap ErrVersionPatternNoMatch", err)
	}
	if changed {
		t.Errorf("changed = true, want false (absent file is a fail-loud abort)")
	}

	if _, statErr := os.Stat(filepath.Join(dir, "version.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("absent file was created on a fail-loud abort: stat err = %v", statErr)
	}
}

func TestProjectVersionFileEmbedded_AlreadyAtTarget_NoOp(t *testing.T) {
	t.Parallel()

	// A file whose every slot already holds the target version still MATCHES the slot,
	// so it does NOT hit the zero-match abort; the replacement is byte-identical to the
	// existing content, so it is a no-op: changed == false, on-disk bytes untouched,
	// and the file is not physically rewritten (stable mtime).
	dir := t.TempDir()
	source := "package main\n\nconst RELEASE_VERSION=\"1.4.0\"\n"
	seedVersionFile(t, dir, "version.go", source)
	before := statModTime(t, dir, "version.go")

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.go", embeddedPattern, "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFileEmbedded returned unexpected error: %v", err)
	}
	if changed {
		t.Errorf("changed = true, want false (file already at the target version)")
	}

	if got := readVersionFile(t, dir, "version.go"); got != source {
		t.Errorf("version file = %q, want it untouched %q", got, source)
	}
	if after := statModTime(t, dir, "version.go"); !after.Equal(before) {
		t.Errorf("version file mtime changed on a no-op: before %v, after %v", before, after)
	}
}

func TestProjectVersionFileEmbedded_PlaceholderSubstitutedInReplacement(t *testing.T) {
	t.Parallel()

	// The {version} placeholder is substituted in the REPLACEMENT: after projection the
	// written content carries the NEW version in the pattern's slot. This pins that the
	// placeholder is expanded to the new version (not left literal, not the old value).
	dir := t.TempDir()
	seedVersionFile(t, dir, "version.txt", "RELEASE_VERSION=\"0.9.0\"\n")

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.txt", embeddedPattern, "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFileEmbedded returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (placeholder substituted with new version)")
	}

	got := readVersionFile(t, dir, "version.txt")
	if !strings.Contains(got, `RELEASE_VERSION="1.4.0"`) {
		t.Errorf("written content %q does not carry the substituted new version", got)
	}
	if strings.Contains(got, "{version}") {
		t.Errorf("written content %q still carries the literal {version} placeholder", got)
	}
}

func TestProjectVersionFileEmbedded_MalformedPattern_ConfigError(t *testing.T) {
	t.Parallel()

	// A version_pattern with NO {version} token is malformed config: there is no slot
	// to match or substitute. It is a clear config error, distinct from a zero-match
	// abort, and no file is read or written.
	dir := t.TempDir()
	seedVersionFile(t, dir, "version.txt", "RELEASE_VERSION=\"1.3.9\"\n")

	changed, err := record.ProjectVersionFileEmbedded(dir, "version.txt", `RELEASE_VERSION="fixed"`, "1.4.0")
	if err == nil {
		t.Fatalf("err = nil, want a config error for a pattern with no {version} token")
	}
	if errors.Is(err, record.ErrVersionPatternNoMatch) {
		t.Errorf("err = %v, want a config error distinct from the zero-match abort", err)
	}
	if changed {
		t.Errorf("changed = true, want false (malformed pattern makes no change)")
	}
}
