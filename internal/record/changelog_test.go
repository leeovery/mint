package record_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/record"
)

// kacPreamble is the canonical Keep a Changelog 1.1.0 header preamble mint writes
// at the top of a freshly created CHANGELOG.md. The test pins it verbatim so the
// created file's header can never silently drift from the documented format.
const kacPreamble = `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`

func fixedDate(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
}

func readChangelog(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	return string(data)
}

func TestWriteChangelog_AbsentFile_CreatesWithPreambleThenFirstSection(t *testing.T) {
	t.Parallel()

	// With no CHANGELOG.md present, mint creates it with the standard Keep a
	// Changelog header preamble first, then the first version section. The exact
	// whole-file content is deterministic, so it is asserted verbatim rather than
	// by substring.
	dir := t.TempDir()

	res, err := record.WriteChangelog(dir, "0.0.1", fixedDate(t), "Initial release.", true)
	if err != nil {
		t.Fatalf("WriteChangelog returned unexpected error: %v", err)
	}
	if !res.Changed {
		t.Errorf("Changed = false, want true (file was created)")
	}

	want := kacPreamble + "\n## [0.0.1] - 2026-06-10\n\nInitial release.\n"
	if got := readChangelog(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteChangelog_ExistingFile_PrependsBelowPreambleNewestOnTop(t *testing.T) {
	t.Parallel()

	// With an existing CHANGELOG.md, the new section is inserted BELOW the Keep a
	// Changelog preamble and ABOVE the most recent existing `## [` block — newest
	// on top. The preamble is preserved untouched and the prior section follows the
	// new one. The whole-file result is deterministic, so it is asserted verbatim.
	dir := t.TempDir()

	existing := kacPreamble + "\n## [0.0.1] - 2026-06-01\n\nInitial release.\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("seeding CHANGELOG.md: %v", err)
	}

	res, err := record.WriteChangelog(dir, "0.0.2", fixedDate(t), "Second release body.", true)
	if err != nil {
		t.Fatalf("WriteChangelog returned unexpected error: %v", err)
	}
	if !res.Changed {
		t.Errorf("Changed = false, want true (a new section was prepended)")
	}

	want := kacPreamble +
		"\n## [0.0.2] - 2026-06-10\n\nSecond release body.\n" +
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n"
	if got := readChangelog(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}

	// The new header must sit strictly above the prior one — newest on top.
	content := readChangelog(t, dir)
	newIdx := strings.Index(content, "## [0.0.2]")
	oldIdx := strings.Index(content, "## [0.0.1]")
	if newIdx < 0 || oldIdx < 0 || newIdx >= oldIdx {
		t.Errorf("new section (idx %d) must precede prior section (idx %d)", newIdx, oldIdx)
	}
}

func TestWriteChangelog_PreambleOnlyFile_AppendsFirstSection(t *testing.T) {
	t.Parallel()

	// A pre-existing file that holds only the preamble (no `## [` version section
	// yet) gets the first section appended below it, separated by a single blank
	// line — the preamble is preserved and the section sits newest-on-top by
	// default (there is nothing below it).
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(kacPreamble), 0o644); err != nil {
		t.Fatalf("seeding CHANGELOG.md: %v", err)
	}

	res, err := record.WriteChangelog(dir, "0.0.1", fixedDate(t), "Initial release.", true)
	if err != nil {
		t.Fatalf("WriteChangelog returned unexpected error: %v", err)
	}
	if !res.Changed {
		t.Errorf("Changed = false, want true (first section appended)")
	}

	want := kacPreamble + "\n## [0.0.1] - 2026-06-10\n\nInitial release.\n"
	if got := readChangelog(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteChangelog_Disabled_IsNoOp(t *testing.T) {
	t.Parallel()

	// changelog = false skips the step entirely: no file is created, nothing is
	// read or written, and Changed is false so the caller skips the bookkeeping
	// commit.
	dir := t.TempDir()

	res, err := record.WriteChangelog(dir, "0.0.1", fixedDate(t), "Initial release.", false)
	if err != nil {
		t.Fatalf("WriteChangelog returned unexpected error: %v", err)
	}
	if res.Changed {
		t.Errorf("Changed = true, want false (changelog disabled is a no-op)")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "CHANGELOG.md")); !os.IsNotExist(statErr) {
		t.Errorf("CHANGELOG.md exists, want no file written when changelog disabled")
	}
}

func TestWriteChangelog_IdenticalContent_ReportsNoChange(t *testing.T) {
	t.Parallel()

	// Writing a section whose rendered result byte-for-byte matches the file
	// already on disk is a no-op: Changed must be false so the caller skips an
	// empty bookkeeping commit. This is the content-compare basis for no-op safety.
	dir := t.TempDir()

	if _, err := record.WriteChangelog(dir, "0.0.1", fixedDate(t), "Initial release.", true); err != nil {
		t.Fatalf("first WriteChangelog returned unexpected error: %v", err)
	}
	before := readChangelog(t, dir)

	res, err := record.WriteChangelog(dir, "0.0.1", fixedDate(t), "Initial release.", true)
	if err != nil {
		t.Fatalf("second WriteChangelog returned unexpected error: %v", err)
	}
	if res.Changed {
		t.Errorf("Changed = true, want false (re-writing identical content is a no-op)")
	}
	if after := readChangelog(t, dir); after != before {
		t.Errorf("CHANGELOG.md changed on a no-op rewrite:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestWriteChangelog_ExistingVersion_ReplacedInPlaceNoDuplicate(t *testing.T) {
	t.Parallel()

	// Recording a version whose section already exists replaces that block in place
	// (idempotent by version key — Stage 5) rather than appending a duplicate. The
	// surrounding sections keep their order; only the matched block's contents
	// change. This is the basis of the content-compare no-op: an identical rewrite
	// yields an identical file.
	dir := t.TempDir()

	existing := kacPreamble +
		"\n## [0.0.2] - 2026-06-10\n\nOld second body.\n" +
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("seeding CHANGELOG.md: %v", err)
	}

	res, err := record.WriteChangelog(dir, "0.0.2", fixedDate(t), "New second body.", true)
	if err != nil {
		t.Fatalf("WriteChangelog returned unexpected error: %v", err)
	}
	if !res.Changed {
		t.Errorf("Changed = false, want true (the matched section body changed)")
	}

	want := kacPreamble +
		"\n## [0.0.2] - 2026-06-10\n\nNew second body.\n" +
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n"
	if got := readChangelog(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}
	if got := strings.Count(readChangelog(t, dir), "## [0.0.2]"); got != 1 {
		t.Errorf("section [0.0.2] appears %d times, want exactly 1 (no duplicate)", got)
	}
}

func TestWriteChangelog_SectionHeaderUsesInjectedDate(t *testing.T) {
	t.Parallel()

	// The section header is `## [x.y.z] - YYYY-MM-DD` with the release date. The
	// date is INJECTED (not read from the clock inside the writer), so the header
	// is fully deterministic and exactly assertable across versions/dates.
	tests := []struct {
		name    string
		version string
		date    time.Time
		want    string
	}{
		{
			name:    "first patch on june 10",
			version: "0.0.1",
			date:    time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC),
			want:    "## [0.0.1] - 2026-06-10",
		},
		{
			name:    "minor on a single-digit month and day, zero-padded",
			version: "1.2.0",
			date:    time.Date(2025, time.January, 3, 0, 0, 0, 0, time.UTC),
			want:    "## [1.2.0] - 2025-01-03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if _, err := record.WriteChangelog(dir, tt.version, tt.date, "Body.", true); err != nil {
				t.Fatalf("WriteChangelog returned unexpected error: %v", err)
			}

			content := readChangelog(t, dir)
			if !strings.Contains(content, tt.want+"\n") {
				t.Errorf("CHANGELOG.md does not contain the header line %q\nfull content:\n%s", tt.want, content)
			}
		})
	}
}
