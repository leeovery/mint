// Package version derives mint's current and next release version purely from
// git tags — there is no file-based or embedded version source, because brew
// installs from tags, so the tag *is* the real version. CurrentVersion lists the
// complete tag set through the CommandRunner seam, keeps only the strict 3-part
// SemVer tags carrying the configured prefix, and returns the global numeric
// maximum (or 0.0.0 when nothing matches). Next computes the bumped version, and
// SemVer.String writes the prefix back for tagging.
package version

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mint/internal/runner"
)

// SemVer is a strict 3-part SemVer 2.0.0 version: MAJOR.MINOR.PATCH, numeric
// segments only. Pre-release and build metadata are deliberately unrepresented —
// mint neither parses nor produces them.
type SemVer struct {
	Major int
	Minor int
	Patch int
}

// Bump selects which segment Next increments. The zero value is BumpPatch so a
// release with no explicit bump flag defaults to a patch bump.
type Bump int

const (
	// BumpPatch increments the patch segment. It is the zero value and the
	// default when no bump flag is given.
	BumpPatch Bump = iota
	// BumpMinor increments the minor segment and resets patch to zero.
	BumpMinor
	// BumpMajor increments the major segment and resets minor and patch to zero.
	BumpMajor
	// BumpExplicit marks a --set-version run: the next version was PINNED outright
	// rather than computed from current, so Next is bypassed for it (the spine uses
	// the parsed version directly). It exists so the chosen kind can flow to the
	// MINT_BUMP hook env as "explicit"; passing it to Next would (harmlessly) fall to
	// the patch default, but the spine never does.
	BumpExplicit
)

// CurrentVersion lists the repository's tags through the runner, parses those
// matching ^{prefix}(\d+)\.(\d+)\.(\d+)$, and returns the numerically highest.
// Tags that don't match the prefix or aren't strict 3-part SemVer are ignored.
// When nothing matches (including a tagless repo) it returns 0.0.0, so the
// first-release path needs no special casing.
func CurrentVersion(ctx context.Context, r runner.CommandRunner, prefix string) (SemVer, error) {
	res, err := r.Run(ctx, "git", "tag", "--list")
	if err != nil {
		return SemVer{}, fmt.Errorf("listing git tags: %w", err)
	}

	return highestMatching(res.Stdout, prefix), nil
}

// ParseSemVer parses a single explicit version string (the --set-version value)
// into a SemVer, REUSING the same strict SemVer 2.0.0 grammar the tag scanner uses
// (^{prefix}(\d+)\.(\d+)\.(\d+)$). It accepts the value with OR without the
// configured prefix — `v2.0.0` and `2.0.0` both parse under prefix "v" — mirroring
// regenerate's <version> normalisation, so a user who habitually types the prefix
// is not punished; the prefix mint writes back at tag time is owned by String. Any
// non-3-part shape (`2.0`, `2.0.0.1`), pre-release/build metadata (`2.0.0-rc.1`,
// `2.0.0+b5`), or non-numeric segment (`abc`, `1.2.x`) is rejected, so explicit
// versions obey the exact same tag-as-truth grammar as discovered tags.
func ParseSemVer(value, prefix string) (SemVer, error) {
	// Strip a leading configured prefix if the caller supplied one (the value is meant
	// to be bare, but `v2.0.0` is normalised to `2.0.0` under prefix "v"), then match
	// against the bare strict grammar — the same `(\d+)\.(\d+)\.(\d+)` the tag scanner
	// uses. A non-empty prefix that is NOT present is left untouched, so a stray suffix
	// like `v2.0.0.1` still fails the strict match below.
	bare := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	if v, ok := parseTag(bare, strings.TrimPrefix(value, prefix)); ok {
		return v, nil
	}
	return SemVer{}, fmt.Errorf("invalid version %q: expected strict 3-part SemVer (MAJOR.MINOR.PATCH)", value)
}

// Next returns current bumped per bump: patch increments patch; minor increments
// minor and zeroes patch; major increments major and zeroes minor and patch.
func Next(current SemVer, bump Bump) SemVer {
	switch bump {
	case BumpMajor:
		return SemVer{Major: current.Major + 1, Minor: 0, Patch: 0}
	case BumpMinor:
		return SemVer{Major: current.Major, Minor: current.Minor + 1, Patch: 0}
	default: // BumpPatch is the zero value / default bump.
		return SemVer{Major: current.Major, Minor: current.Minor, Patch: current.Patch + 1}
	}
}

// String renders the version with prefix written back: {prefix}{M}.{m}.{p}. An
// empty prefix yields a bare X.Y.Z; "v" yields v0.0.1; a component prefix like
// "pkg-name/v" yields pkg-name/v10.12.3.
func (v SemVer) String(prefix string) string {
	return fmt.Sprintf("%s%d.%d.%d", prefix, v.Major, v.Minor, v.Patch)
}

// highestMatching parses tagList (newline-separated tag names) against the
// prefixed strict-SemVer pattern and returns the numeric maximum, or the 0.0.0
// zero value when none match.
func highestMatching(tagList, prefix string) SemVer {
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `(\d+)\.(\d+)\.(\d+)$`)

	var highest SemVer
	for _, line := range strings.Split(tagList, "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" {
			continue
		}

		v, ok := parseTag(pattern, tag)
		if !ok {
			continue
		}
		if v.GreaterThan(highest) {
			highest = v
		}
	}

	return highest
}

// parseTag matches tag against pattern and converts the three captured numeric
// segments into a SemVer. ok is false when the tag does not match. The capture
// groups are guaranteed numeric by the pattern, so the conversions cannot fail.
func parseTag(pattern *regexp.Regexp, tag string) (SemVer, bool) {
	m := pattern.FindStringSubmatch(tag)
	if m == nil {
		return SemVer{}, false
	}

	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	return SemVer{Major: major, Minor: minor, Patch: patch}, true
}

// GreaterThan reports whether v sorts strictly above other by numeric
// (Major, Minor, Patch) precedence — not lexical, so v10.0.0 > v9.0.0. It backs
// both the highest-tag scan and the --set-version strictly-greater gate.
func (v SemVer) GreaterThan(other SemVer) bool {
	switch {
	case v.Major != other.Major:
		return v.Major > other.Major
	case v.Minor != other.Minor:
		return v.Minor > other.Minor
	default:
		return v.Patch > other.Patch
	}
}
