package record

import (
	"fmt"
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
//     (the user-resolved no-data-loss rule).
//
// Any version with no entry in the list is DROPPED (genuine stray-section drift — a
// section matching no real version). The caller therefore expresses "regenerated →
// rendered, skipped-but-real → preserved, stray → omitted" purely by which sections
// it supplies and in what order.

// changelogSectionKind distinguishes a freshly-rendered section from one preserved
// verbatim out of the existing file.
type changelogSectionKind int

const (
	// sectionRendered renders the section from version + date + body via renderSection.
	sectionRendered changelogSectionKind = iota
	// sectionPreserved copies the version's existing section block verbatim from the
	// current file (no re-render — a skipped version keeps its exact recorded notes).
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
// release's recorded notes and original date survive the rebuild untouched. The
// version's section MUST exist in the current file or RebuildChangelog fails loud.
func PreservedSection(version string) ChangelogSection {
	return ChangelogSection{kind: sectionPreserved, version: version}
}

// RebuildChangelog rewrites {dir}/CHANGELOG.md WHOLE from the KaC preamble followed by
// sections in the given order (the caller supplies newest-on-top), and reports whether
// the file's content changed.
//
// Rendered sections are produced from their version/date/body; preserved sections are
// copied verbatim from the existing file (failing loud if a preserved version's section
// is absent). Any existing section with no corresponding entry is dropped — the whole
// rebuild keeps exactly the supplied sections, which is how ordering is repaired and
// stray-section drift removed.
//
// The write is atomic (temp file + rename). When the rebuilt content matches the file
// already on disk byte-for-byte, nothing is written and Changed is false — the no-op
// signal the caller uses to skip an empty commit.
func RebuildChangelog(dir string, sections []ChangelogSection) (WriteResult, error) {
	path := filepath.Join(dir, changelogFileName)

	existing, err := readExisting(path)
	if err != nil {
		return WriteResult{}, err
	}

	rebuilt, err := composeChangelog(existing, sections)
	if err != nil {
		return WriteResult{}, err
	}

	if rebuilt == existing {
		return WriteResult{Changed: false}, nil
	}
	if err := writeAtomic(path, dir, rebuilt); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Changed: true}, nil
}

// composeChangelog builds the whole-file content: the KaC preamble, a blank separator,
// then each section's text in order. A rendered section is produced via renderSection;
// a preserved section's block is extracted verbatim from existing (a loud error when
// absent). The section texts are concatenated directly — each already carries its own
// trailing newline(s) from renderSection / the source file — so the composition matches
// the single-version writer's section spacing exactly.
func composeChangelog(existing string, sections []ChangelogSection) (string, error) {
	out := kacPreamble + "\n"
	for _, s := range sections {
		text, err := sectionText(existing, s)
		if err != nil {
			return "", err
		}
		out += text
	}
	return out, nil
}

// sectionText returns one section's whole-file text: renderSection output for a
// rendered section, or the version's verbatim existing block for a preserved one.
func sectionText(existing string, s ChangelogSection) (string, error) {
	if s.kind == sectionPreserved {
		return preservedSectionText(existing, s.version)
	}
	return renderSection(s.version, s.date, s.body), nil
}

// preservedSectionText extracts version's existing section block verbatim from existing
// — from its `## [version]` header up to (but not including) the next section header or
// end of file — reusing splitAroundSection so the parse matches the writer's own. It is
// a loud error when the version has no section in the current file (the caller asked to
// preserve a section that does not exist).
func preservedSectionText(existing, version string) (string, error) {
	before, after, found := splitAroundSection(existing, version)
	if !found {
		return "", fmt.Errorf("cannot preserve section for %s: not present in %s", version, changelogFileName)
	}
	// The block is everything between the content before the header and the content
	// after the block — i.e. existing with before and after trimmed off the ends.
	return existing[len(before) : len(existing)-len(after)], nil
}
