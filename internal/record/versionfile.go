package record

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// versionPlaceholder is the token in version_pattern that marks the version slot,
// e.g. RELEASE_VERSION="{version}". It is replaced by the version-slot group when
// building the match regex and by the new bare version when building the replacement.
const versionPlaceholder = "{version}"

// versionSlot is the regexp source matching a bare semver (X.Y.Z) — the value the
// version slot of version_pattern is expected to hold in an embedded source file.
const versionSlot = `\d+\.\d+\.\d+`

// ErrVersionPatternNoMatch is returned by ProjectVersionFileEmbedded when the
// configured version_pattern matches nothing in the source file — whether the file
// is absent or present-but-without-a-matching-slot. It is a fail-loud sentinel: the
// release aborts during Record, BEFORE the tag, rather than silently skipping the
// version write. The returned error wraps this sentinel with the offending file path
// so errors.Is matches and the message names the file.
var ErrVersionPatternNoMatch = errors.New("record: version_pattern matched nothing")

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

// ProjectVersionFileEmbedded rewrites the version slot of version_pattern inside an
// EXISTING source file at {root}/{versionFile} with the new bare version, reporting
// whether it produced a net change on disk.
//
// EMBEDDED mode (version_pattern set) operates on a real source file (e.g. a Go const
// or a shell var) rather than owning the whole file. version_pattern carries the
// {version} placeholder marking the slot, e.g. RELEASE_VERSION="{version}". From it:
//   - a MATCH regex is built where every non-placeholder piece is a LITERAL anchor
//     (regexp.QuoteMeta) and each {version} becomes a version-slot group matching a
//     bare semver — so a pattern with more than one placeholder anchors a version
//     group between every literal piece.
//   - a REPLACEMENT is built by substituting {version} with the new BARE version
//     (X.Y.Z, no tag_prefix — consistent with plain mode).
//
// A pattern containing NO {version} token is malformed config and returns an error
// (distinct from the zero-match abort) before the file is touched.
//
// FAIL-LOUD: if the pattern matches nothing — the file is absent OR present without a
// matching slot — the function returns ErrVersionPatternNoMatch (wrapped with the
// path) and writes nothing, so the release aborts during Record before the tag rather
// than silently skipping the version write.
//
// MULTIPLE matches are ALL replaced (legacy sed-replace semantics) so no stale copy of
// the old version survives. A file whose every slot already holds the new version
// still matches the slot, so it does not hit the abort; its replacement is identical
// to the existing content, making it a no-op (changed false, nothing written).
// Otherwise the new content is written atomically (reusing writeFileAtomic) and
// changed is true.
//
// Like plain mode this produces only the file write and the changed signal; it does
// NOT create or stage any git commit — that is folded into the single bookkeeping
// commit in task 3-7.
func ProjectVersionFileEmbedded(root, versionFile, versionPattern, version string) (changed bool, err error) {
	matcher, err := compileVersionPattern(versionPattern)
	if err != nil {
		return false, err
	}
	replacement := strings.ReplaceAll(versionPattern, versionPlaceholder, version)

	path := filepath.Join(root, versionFile)
	content, found, err := readVersionFileContent(path)
	if err != nil {
		return false, err
	}
	if !found || !matcher.MatchString(content) {
		return false, fmt.Errorf("%w in %s", ErrVersionPatternNoMatch, path)
	}

	newContent := matcher.ReplaceAllLiteralString(content, replacement)
	if newContent == content {
		return false, nil
	}

	if err := writeFileAtomic(path, newContent); err != nil {
		return false, err
	}
	return true, nil
}

// compileVersionPattern turns version_pattern into a regexp that anchors its literal
// pieces and matches a bare-semver version slot at each {version} token. The pattern
// is split on the literal {version} token; each piece is QuoteMeta'd so regex
// metacharacters in the surrounding source are matched literally, and the pieces are
// joined by the version-slot group so multiple placeholders each anchor their own
// slot. A pattern with no {version} token is malformed config and returns an error.
func compileVersionPattern(versionPattern string) (*regexp.Regexp, error) {
	if !strings.Contains(versionPattern, versionPlaceholder) {
		return nil, fmt.Errorf("record: version_pattern %q has no %s placeholder", versionPattern, versionPlaceholder)
	}

	pieces := strings.Split(versionPattern, versionPlaceholder)
	quoted := make([]string, len(pieces))
	for i, piece := range pieces {
		quoted[i] = regexp.QuoteMeta(piece)
	}
	source := strings.Join(quoted, "("+versionSlot+")")

	matcher, err := regexp.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("record: compiling version_pattern %q: %w", versionPattern, err)
	}
	return matcher, nil
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
