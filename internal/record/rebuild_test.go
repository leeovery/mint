package record_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mint/internal/record"
)

// rebuildDate returns a fixed section-header date so the rendered `## [x.y.z] - date`
// headers are deterministic in the rebuild tests.
func rebuildDate(t *testing.T, day int) time.Time {
	t.Helper()
	return time.Date(2024, time.March, day, 0, 0, 0, 0, time.UTC)
}

// seedChangelogFile writes content as the existing CHANGELOG.md so a rebuild can be
// asserted against the exact prior file (drop-stray, preserve-verbatim, no-op cases).
func seedChangelogFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("seeding CHANGELOG.md: %v", err)
	}
}

// TestRebuildChangelog_ComposesPreambleThenSectionsNewestOnTop proves the whole-file
// rebuild writes the KaC preamble followed by every supplied section in the order
// given (the caller supplies newest-on-top), reusing record's own rendering primitives.
func TestRebuildChangelog_ComposesPreambleThenSectionsNewestOnTop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No existing file: a pure compose from rendered sections.
	sections := []record.ChangelogSection{
		record.RenderedSection("2.0.0", rebuildDate(t, 3), "## v2 body\n"),
		record.RenderedSection("1.1.0", rebuildDate(t, 2), "## v1.1 body\n"),
		record.RenderedSection("1.0.0", rebuildDate(t, 1), "## v1 body\n"),
	}

	result, err := record.RebuildChangelog(dir, sections)
	if err != nil {
		t.Fatalf("RebuildChangelog returned unexpected error: %v", err)
	}
	if !result.Changed {
		t.Errorf("Changed = false, want true (the file was created)")
	}

	got := readChangelog(t, dir)
	// Each body ends in "\n", so renderSection appends a trailing blank line per
	// section (header\n\nbody\n + \n); the concatenation matches that exactly.
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## v2 body\n\n" +
		"## [1.1.0] - 2024-03-02\n\n## v1.1 body\n\n" +
		"## [1.0.0] - 2024-03-01\n\n## v1 body\n\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant preamble + sections newest-on-top\n%q", got, want)
	}
}

// TestRebuildChangelog_DropsStraySectionAndRepairsOrder proves the rebuild composes
// ONLY the supplied sections in the supplied order: an existing file with a stray
// section (one not in the supplied set) and out-of-order sections is replaced wholesale
// by the rebuilt content, dropping the stray and repairing the order.
func TestRebuildChangelog_DropsStraySectionAndRepairsOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Existing file: mis-ordered sections plus a stray (9.9.9) with no matching version.
	seedChangelogFile(t, dir, kacPreamble+"\n"+
		"## [1.0.0] - 2024-03-01\n\nold v1\n\n"+
		"## [9.9.9] - 2024-03-09\n\nstray\n\n"+
		"## [2.0.0] - 2024-03-03\n\nold v2\n")

	sections := []record.ChangelogSection{
		record.RenderedSection("2.0.0", rebuildDate(t, 3), "## v2 body\n"),
		record.RenderedSection("1.0.0", rebuildDate(t, 1), "## v1 body\n"),
	}

	result, err := record.RebuildChangelog(dir, sections)
	if err != nil {
		t.Fatalf("RebuildChangelog returned unexpected error: %v", err)
	}
	if !result.Changed {
		t.Errorf("Changed = false, want true (order repaired + stray dropped)")
	}

	got := readChangelog(t, dir)
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## v2 body\n\n" +
		"## [1.0.0] - 2024-03-01\n\n## v1 body\n\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant the stray dropped + order repaired\n%q", got, want)
	}
}

// TestRebuildChangelog_PreservesExistingSectionVerbatim proves a PreservedSection
// emits the version's EXISTING section block verbatim (header + body + date), pulled
// from the current file — the user-resolved no-data-loss behaviour for skipped versions.
func TestRebuildChangelog_PreservesExistingSectionVerbatim(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\nold v2 stale\n\n" +
		"## [1.0.0] - 2024-03-01\n\nKEEP ME exactly - skipped\n"
	seedChangelogFile(t, dir, existing)

	sections := []record.ChangelogSection{
		record.RenderedSection("2.0.0", rebuildDate(t, 3), "## v2 regenerated\n"),
		record.PreservedSection("1.0.0"),
	}

	result, err := record.RebuildChangelog(dir, sections)
	if err != nil {
		t.Fatalf("RebuildChangelog returned unexpected error: %v", err)
	}
	if !result.Changed {
		t.Errorf("Changed = false, want true (v2 was regenerated)")
	}

	got := readChangelog(t, dir)
	// v2 is rendered (trailing blank line from its "\n"-terminated body); v1's block is
	// copied verbatim from the existing file (its single trailing "\n" preserved).
	want := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## v2 regenerated\n\n" +
		"## [1.0.0] - 2024-03-01\n\nKEEP ME exactly - skipped\n"
	if got != want {
		t.Errorf("rebuilt CHANGELOG.md =\n%q\nwant v1's existing section preserved verbatim\n%q", got, want)
	}
}

// TestRebuildChangelog_ByteIdenticalReportsNoChange proves a rebuild that yields
// byte-for-byte the existing file reports Changed=false so the caller makes no commit.
func TestRebuildChangelog_ByteIdenticalReportsNoChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// The existing file is exactly what a prior rebuild would emit for these inputs
	// (each "\n"-terminated body yields a trailing blank line per section), so the
	// re-rebuild is a true byte-for-byte no-op.
	existing := kacPreamble + "\n" +
		"## [2.0.0] - 2024-03-03\n\n## v2 body\n\n" +
		"## [1.0.0] - 2024-03-01\n\n## v1 body\n\n"
	seedChangelogFile(t, dir, existing)

	sections := []record.ChangelogSection{
		record.RenderedSection("2.0.0", rebuildDate(t, 3), "## v2 body\n"),
		record.RenderedSection("1.0.0", rebuildDate(t, 1), "## v1 body\n"),
	}

	result, err := record.RebuildChangelog(dir, sections)
	if err != nil {
		t.Fatalf("RebuildChangelog returned unexpected error: %v", err)
	}
	if result.Changed {
		t.Errorf("Changed = true, want false (rebuilt content is byte-identical)")
	}
	if got := readChangelog(t, dir); got != existing {
		t.Errorf("a byte-identical rebuild rewrote the file:\n%q", got)
	}
}

// TestRebuildChangelog_PreservedSectionMissing_Errors proves a PreservedSection for a
// version absent from the existing file is a loud error (the caller asked to preserve a
// section that does not exist — a programming/data fault, never silently dropped).
func TestRebuildChangelog_PreservedSectionMissing_Errors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelogFile(t, dir, kacPreamble+"\n## [2.0.0] - 2024-03-03\n\nv2\n")

	sections := []record.ChangelogSection{
		record.RenderedSection("2.0.0", rebuildDate(t, 3), "## v2 body\n"),
		record.PreservedSection("1.0.0"), // not present in the file
	}

	if _, err := record.RebuildChangelog(dir, sections); err == nil {
		t.Fatal("RebuildChangelog returned nil error, want a loud error for a missing preserved section")
	}
}
