package record

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mint/internal/fsutil"
)

// changelogFileName is the fixed name of the changelog at the repo root. mint
// owns this file as a write-only projection — it writes it but never reads it as
// a source of truth.
const changelogFileName = "CHANGELOG.md"

// dateLayout formats the section-header date as YYYY-MM-DD (Go's reference date).
const dateLayout = "2006-01-02"

// kacPreamble is the standard Keep a Changelog (1.1.0) header preamble mint writes
// when creating a new CHANGELOG.md. mint is a generator, not a human-maintained
// changelog, so it keeps KaC's entry structure but omits the [Unreleased] section
// (which exists only for humans accruing notes between releases). The preamble is
// the canonical KaC wording; the per-version sections that follow are mint's.
const kacPreamble = `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`

// sectionMarker is the prefix of every version section header (`## [`). The new
// section is inserted immediately above the first line starting with this marker
// so the newest version sits on top, below the preamble and above all prior
// sections.
const sectionMarker = "## ["

// WriteResult reports the outcome of a changelog write. Changed is true when the
// write produced a net change to the file on disk (a create, or a prepend that
// altered the content). It is false on a no-op — the changelog skip, or a write
// whose result byte-for-byte matches what was already there — which the caller
// uses to skip the bookkeeping commit (no empty commits).
type WriteResult struct {
	Changed bool
}

// WriteChangelog records version's release section into {dir}/CHANGELOG.md and
// reports whether the file's content changed.
//
// version is the bare x.y.z key (no tag prefix) used in the `## [x.y.z] - date`
// header; date is INJECTED so the header is deterministic — the production caller
// passes time.Now(), tests pass a fixed time. body is the full notes body, placed
// verbatim under the header.
//
// When the file is absent it is created with the Keep a Changelog preamble
// followed by the first version section. When it exists the new section is
// prepended below the preamble and above the most recent existing `## [` block
// (newest on top). The write is atomic (temp file + rename) so a crash mid-write
// never leaves a truncated changelog.
//
// When enabled is false the changelog step is skipped entirely (changelog =
// false): nothing is read or written and WriteResult.Changed is false. When the
// rendered content matches the file already on disk byte-for-byte, nothing is
// written and Changed is false too — the no-op signal the caller needs to avoid
// an empty bookkeeping commit.
func WriteChangelog(dir, version string, date time.Time, body string, enabled bool) (WriteResult, error) {
	if !enabled {
		return WriteResult{Changed: false}, nil
	}

	path := filepath.Join(dir, changelogFileName)

	existing, err := readExisting(path)
	if err != nil {
		return WriteResult{}, err
	}

	updated := insertSection(existing, version, date, body)
	if updated == existing {
		return WriteResult{Changed: false}, nil
	}

	if err := writeAtomic(path, updated); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Changed: true}, nil
}

// readExisting returns the current changelog contents, or the empty string when
// the file does not yet exist (the first-release create path). Any other read
// error is surfaced.
func readExisting(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", changelogFileName, err)
	}
	return string(data), nil
}

// insertSection returns existing with the new version section recorded newest-on-
// top. With no existing content it creates the file body: the KaC preamble, a
// blank separator line, then the first section.
//
// When a section for this exact version key is already present it is replaced in
// place (idempotent by version key — Stage 5) so a rewrite of an already-recorded
// version produces a duplicate-free result; rewriting identical content therefore
// yields byte-for-byte the same file, which the caller reads as a no-op. When the
// version is new the section is prepended at the first `## [` line, keeping the
// preamble above and all prior sections below.
func insertSection(existing, version string, date time.Time, body string) string {
	section := renderSection(version, date, body)

	if existing == "" {
		return kacPreamble + "\n" + section
	}

	if before, after, found := splitAroundSection(existing, version); found {
		// Replace the existing same-version block in place, preserving order.
		return before + section + after
	}

	head, tail, found := splitAtFirstSection(existing)
	if !found {
		// No prior version section (e.g. a preamble-only file): append the first
		// section after the existing head, with a single blank-line separator.
		return ensureTrailingBlankLine(existing) + section
	}
	return head + section + tail
}

// splitAroundSection locates the existing section for version and returns the
// content before its header and after its block, so a fresh section can be
// substituted in place. found is false when no section for that version exists.
// The block runs from its `## [version]` header line up to (but not including)
// the next `## [` header, or end of file.
func splitAroundSection(content, version string) (before, after string, found bool) {
	header := sectionMarker + version + "]"

	start := indexOfLine(content, header)
	if start < 0 {
		return "", "", false
	}

	rest := content[start+len(header):]
	if next := indexOfSectionLine(rest); next >= 0 {
		return content[:start], rest[next:], true
	}
	return content[:start], "", true
}

// indexOfLine returns the byte offset of the start of the first line that begins
// with prefix, or -1 if none does. Only line starts match, so a prefix appearing
// mid-line inside a body is never mistaken for a header.
func indexOfLine(content, prefix string) int {
	if strings.HasPrefix(content, prefix) {
		return 0
	}
	if i := strings.Index(content, "\n"+prefix); i >= 0 {
		return i + 1
	}
	return -1
}

// renderSection formats one version section: the `## [x.y.z] - YYYY-MM-DD`
// header, a blank line, the full body, and a trailing newline so the next
// section's header starts on its own line.
func renderSection(version string, date time.Time, body string) string {
	return fmt.Sprintf("## [%s] - %s\n\n%s\n", version, date.Format(dateLayout), body)
}

// splitAtFirstSection partitions content at the first line beginning with the
// section marker. head is everything before that line (the preamble, ending in a
// newline); tail is that line onward (all prior sections). found is false when no
// section marker is present.
func splitAtFirstSection(content string) (head, tail string, found bool) {
	idx := indexOfSectionLine(content)
	if idx < 0 {
		return content, "", false
	}
	return content[:idx], content[idx:], true
}

// indexOfSectionLine returns the byte offset of the start of the first line that
// begins with the section marker, or -1 if none does. Only line starts are
// considered so a `## [` appearing mid-line inside a body can never be mistaken
// for a section header.
func indexOfSectionLine(content string) int {
	if strings.HasPrefix(content, sectionMarker) {
		return 0
	}
	if i := strings.Index(content, "\n"+sectionMarker); i >= 0 {
		return i + 1 // skip the leading newline, point at the marker
	}
	return -1
}

// ensureTrailingBlankLine returns s terminated by a blank line, so an appended
// section is separated from preceding content by exactly one empty line. It is
// only reached for a preamble-only file with no existing version sections.
func ensureTrailingBlankLine(s string) string {
	switch {
	case strings.HasSuffix(s, "\n\n"):
		return s
	case strings.HasSuffix(s, "\n"):
		return s + "\n"
	default:
		return s + "\n\n"
	}
}

// writeAtomic writes content to path crash-safely (temp file + rename) so a reader
// never observes a half-written changelog, with the 0o644 mode of a normal tracked
// source file. It delegates the shared idiom to fsutil.WriteFile and wraps any
// failure with the changelog domain noun so the error names this file.
func writeAtomic(path, content string) error {
	if err := fsutil.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", changelogFileName, err)
	}
	return nil
}
