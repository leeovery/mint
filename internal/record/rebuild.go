package record

import (
	"path/filepath"
	"time"
)

// This file is the whole-file CHANGELOG rebuild (regenerate `--all`, task 5-13): it
// composes CHANGELOG.md from the KaC preamble + a caller-ordered list of sections
// (the caller supplies newest-on-top) and rewrites the file atomically, reporting
// whether the content changed.
//
// It REUSES record's existing rendering primitives — kacPreamble for the header and
// renderSection for a regenerated section — so there is NO second renderer; the
// single-version in-place writer (WriteChangelog) and this whole-file rebuild emit
// byte-identical section text for the same inputs. The two compose differently
// (in-place section-replace vs. whole-file regenerate) but render identically.
//
// A section is one of two kinds:
//   - RENDERED: a freshly-regenerated version, rendered from its version key + date +
//     body via renderSection (the same call WriteChangelog uses).
//   - PRESERVED: a version skipped during the batch — its EXISTING section block is
//     pulled VERBATIM from the current file so a skipped real release loses no data
//     (the user-resolved no-data-loss rule). When the version has NO section in the
//     current file there is nothing to lose, so it is simply OMITTED rather than
//     failing the rebuild — a skipped version that was never recorded (a fresh
//     CHANGELOG, or one that has never carried that version) must not poison the whole
//     file and discard every other section. The caller already reports the skip
//     separately (the batch end summary), so the omission is visible there.
//
// Any version with no entry in the list is DROPPED (genuine stray-section drift — a
// section matching no real version). The caller therefore expresses "regenerated →
// rendered, skipped-but-real → preserved-if-present-else-omitted, stray → omitted"
// purely by which sections it supplies and in what order.

// changelogSectionKind distinguishes a freshly-rendered section from one preserved
// verbatim out of the existing file.
type changelogSectionKind int

const (
	// sectionRendered renders the section from version + date + body via renderSection.
	sectionRendered changelogSectionKind = iota
	// sectionPreserved copies the version's existing section block verbatim from the
	// current file (no re-render — a skipped version keeps its exact recorded notes).
	// When the version has no section in the current file it is OMITTED (there is
	// nothing to preserve), never an error.
	sectionPreserved
)

// ChangelogSection is one entry in a whole-file rebuild, newest-on-top in caller
// order. It is either a RENDERED section (built from version/date/body) or a
// PRESERVED section (the version's existing block copied verbatim). Construct it with
// RenderedSection or PreservedSection — the zero value is not meaningful.
type ChangelogSection struct {
	kind    changelogSectionKind
	version string
	date    time.Time
	body    string
}

// RenderedSection is a freshly-regenerated section: it is rendered from the bare
// x.y.z version key, the section-header date, and the full notes body using the SAME
// renderSection the single-version writer uses.
func RenderedSection(version string, date time.Time, body string) ChangelogSection {
	return ChangelogSection{kind: sectionRendered, version: version, date: date, body: body}
}

// PreservedSection is a skipped-but-real version whose EXISTING section block is
// copied verbatim from the current CHANGELOG.md (no re-render), so the skipped
// release's recorded notes and original date survive the rebuild untouched. When the
// version has no section in the current file it is simply OMITTED from the rebuild —
// a skipped version that was never recorded (a fresh CHANGELOG, or one that has never
// carried that version) has nothing to preserve, and must not abort the rebuild and
// discard every other section.
func PreservedSection(version string) ChangelogSection {
	return ChangelogSection{kind: sectionPreserved, version: version}
}

// RebuildChangelog rewrites {dir}/CHANGELOG.md WHOLE from the KaC preamble followed by
// sections in the given order (the caller supplies newest-on-top), and reports whether
// the file's content changed.
//
// Rendered sections are produced from their version/date/body; preserved sections are
// copied verbatim from the existing file (a preserved version absent from the file is
// OMITTED — nothing to preserve, never an error). Any existing section with no
// corresponding entry is dropped — the whole rebuild keeps exactly the supplied
// sections, which is how ordering is repaired and stray-section drift removed.
//
// The write is atomic (temp file + rename). When the rebuilt content matches the file
// already on disk byte-for-byte, nothing is written and Changed is false — the no-op
// signal the caller uses to skip an empty commit.
func RebuildChangelog(dir string, sections []ChangelogSection) (WriteResult, error) {
	path := filepath.Join(dir, ChangelogFileName)

	existing, err := readExisting(path)
	if err != nil {
		return WriteResult{}, err
	}

	rebuilt := composeChangelog(existing, sections)

	if rebuilt == existing {
		return WriteResult{Changed: false}, nil
	}
	if err := writeAtomic(path, rebuilt); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Changed: true}, nil
}

// composeChangelog builds the whole-file content: the KaC preamble, a blank separator,
// then each section's text in order. A rendered section is produced via renderSection;
// a preserved section's block is extracted verbatim from existing (an absent preserved
// version contributes nothing — there is no section to copy). The section texts are
// concatenated directly — each already carries its own trailing newline(s) from
// renderSection / the source file — so the composition matches the single-version
// writer's section spacing exactly.
func composeChangelog(existing string, sections []ChangelogSection) string {
	out := kacPreamble + "\n"
	for _, s := range sections {
		out += sectionText(existing, s)
	}
	return out
}

// sectionText returns one section's whole-file text: renderSection output for a
// rendered section, or the version's verbatim existing block for a preserved one (the
// empty string when a preserved version has no section in the current file, so it is
// omitted from the rebuild).
func sectionText(existing string, s ChangelogSection) string {
	if s.kind == sectionPreserved {
		return preservedSectionText(existing, s.version)
	}
	return renderSection(s.version, s.date, s.body)
}

// preservedSectionText extracts version's existing section block verbatim from existing
// — from its `## [version]` header up to (but not including) the next section header or
// end of file — reusing splitAroundSection so the parse matches the writer's own. When
// the version has NO section in the current file it returns the empty string: a skipped
// version that was never recorded has nothing to preserve, so it is omitted from the
// rebuild rather than aborting it (and discarding every other section).
func preservedSectionText(existing, version string) string {
	before, after, found := splitAroundSection(existing, version)
	if !found {
		return ""
	}
	// The block is everything between the content before the header and the content
	// after the block — i.e. existing with before and after trimmed off the ends.
	return existing[len(before) : len(existing)-len(after)]
}
