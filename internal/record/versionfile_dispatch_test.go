package record_test

import (
	"errors"
	"testing"

	"mint/internal/record"
)

func TestProjectVersionFile_NoVersionFile_NoProjection(t *testing.T) {
	t.Parallel()

	// An empty versionFile means tag-only: there is no projection at all, so the
	// dispatcher reports no change and never touches the filesystem (no file is read
	// or created in dir).
	dir := t.TempDir()

	changed, err := record.ProjectVersionFile(dir, "", "", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFile returned unexpected error: %v", err)
	}
	if changed {
		t.Errorf("changed = true, want false (no version_file means no projection)")
	}
}

func TestProjectVersionFile_PlainMode_RoutesByAbsentPattern(t *testing.T) {
	t.Parallel()

	// With a version_file but NO version_pattern the dispatcher routes to PLAIN mode:
	// the whole file becomes the bare version plus one trailing newline. The created
	// content proves the plain path ran (canonical "1.4.0\n").
	dir := t.TempDir()

	changed, err := record.ProjectVersionFile(dir, "release.txt", "", "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFile returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (plain mode created the version file)")
	}

	if got := readVersionFile(t, dir, "release.txt"); got != "1.4.0\n" {
		t.Errorf("version file = %q, want plain-mode canonical %q", got, "1.4.0\n")
	}
}

func TestProjectVersionFile_EmbeddedMode_RoutesByPresentPattern(t *testing.T) {
	t.Parallel()

	// A version_pattern present routes to EMBEDDED mode: only the pattern's version
	// slot in the existing source file is rewritten; the surrounding lines stay
	// untouched. The whole-file plain content would differ, so the embedded result
	// proves the embedded path ran.
	dir := t.TempDir()
	source := "package main\n\nconst RELEASE_VERSION=\"1.3.9\"\n"
	seedVersionFile(t, dir, "version.go", source)

	changed, err := record.ProjectVersionFile(dir, "version.go", embeddedPattern, "1.4.0")
	if err != nil {
		t.Fatalf("ProjectVersionFile returned unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("changed = false, want true (embedded mode rewrote the slot)")
	}

	want := "package main\n\nconst RELEASE_VERSION=\"1.4.0\"\n"
	if got := readVersionFile(t, dir, "version.go"); got != want {
		t.Errorf("version file = %q, want embedded-mode %q", got, want)
	}
}

func TestProjectVersionFile_EmbeddedMismatch_PropagatesFailLoud(t *testing.T) {
	t.Parallel()

	// In embedded mode a pattern that matches nothing is a fail-loud abort: the
	// dispatcher propagates ErrVersionPatternNoMatch (matchable via errors.Is) and
	// reports no change, so the engine aborts before the tag.
	dir := t.TempDir()
	seedVersionFile(t, dir, "version.go", "package main\n\nconst OTHER=\"1.3.9\"\n")

	changed, err := record.ProjectVersionFile(dir, "version.go", embeddedPattern, "1.4.0")
	if !errors.Is(err, record.ErrVersionPatternNoMatch) {
		t.Fatalf("error = %v, want it to wrap ErrVersionPatternNoMatch", err)
	}
	if changed {
		t.Errorf("changed = true, want false (fail-loud mismatch makes no change)")
	}
}
