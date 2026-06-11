package version

import (
	"context"
	"fmt"

	"mint/internal/runner"
)

// Resolution is the outcome of resolving a regenerate `<version>` argument against
// the repository's tag set. It RESOLVES only — it names the canonical target tag,
// its fresh diff base (the previous matching tag), and whether the target is the
// oldest release. Consuming it (reading the reuse annotation, running the fresh
// diff/AI) is a later concern.
type Resolution struct {
	// Tag is the canonical target tag: the supplied <version> normalised through the
	// Phase 1 grammar and re-prefixed (so `v1.4.0` and `1.4.0` both yield `v1.4.0`
	// under prefix "v", and `pkg-name/v1.2.3` under a monorepo prefix).
	Tag string

	// PreviousTag is the canonical predecessor tag — the numerically-next-lower
	// MATCHING tag — that the fresh diff base ranges from. It is empty when the
	// target is the oldest release (FirstRelease is then true).
	PreviousTag string

	// FirstRelease is true when the target has no predecessor matching tag (the
	// oldest release). The fresh source path mirrors the forward first-release rule:
	// it skips the AI and emits the fixed "Initial release." body. PreviousTag is
	// then empty and DiffRange returns "".
	FirstRelease bool
}

// DiffRange returns the fresh diff base in git range form, `{PreviousTag}..{Tag}`
// (e.g. `v1.3.0..v1.4.0`). For the oldest release (FirstRelease) there is no
// predecessor, so it returns "" — the fresh path skips the diff and uses the fixed
// first-release body instead of computing a range.
func (r Resolution) DiffRange() string {
	if r.FirstRelease {
		return ""
	}
	return r.PreviousTag + ".." + r.Tag
}

// ResolveRegenerateTarget resolves a regenerate `<version>` argument into a
// Resolution. It REUSES the Phase 1 machinery throughout: ParseSemVer normalises
// the supplied value (stripping a leading prefix if present and parsing the strict
// 3-part grammar), the same `git tag --list` read path lists the existing tags, and
// the matching set is scanned with the shared prefixed grammar.
//
// The canonical target tag must exist in the matching set or resolution fails loud
// ("no tag <canonical-tag> found"). The fresh diff base is the numerically-previous
// MATCHING tag (NOT git-describe ancestry; non-matching tags are ignored); when the
// target has no predecessor it is the oldest release and FirstRelease is set.
//
// It performs NO version computation — regenerate targets an EXISTING version, so
// there is no bump or next-version derivation.
func ResolveRegenerateTarget(ctx context.Context, r runner.CommandRunner, prefix, rawVersion string) (Resolution, error) {
	target, err := ParseSemVer(rawVersion, prefix)
	if err != nil {
		return Resolution{}, err
	}
	canonical := target.String(prefix)

	res, err := r.Run(ctx, "git", "tag", "--list")
	if err != nil {
		return Resolution{}, fmt.Errorf("listing git tags: %w", err)
	}
	matching := matchingVersions(res.Stdout, prefix)

	if !contains(matching, target) {
		return Resolution{}, fmt.Errorf("no tag %s found", canonical)
	}

	predecessor, ok := highestBelow(matching, target)
	if !ok {
		return Resolution{Tag: canonical, FirstRelease: true}, nil
	}

	return Resolution{Tag: canonical, PreviousTag: predecessor.String(prefix)}, nil
}

// matchingVersions parses tagList (newline-separated tag names) against the
// prefixed strict-SemVer grammar and returns every matching version. It reuses the
// same per-tag parse the highest-tag scan uses, so non-matching tags (wrong prefix,
// non-semver, pre-release, build metadata) are dropped identically.
func matchingVersions(tagList, prefix string) []SemVer {
	pattern := prefixedPattern(prefix)

	var matches []SemVer
	for _, line := range splitTags(tagList) {
		if v, ok := parseTag(pattern, line); ok {
			matches = append(matches, v)
		}
	}
	return matches
}

// contains reports whether target appears in versions (exact 3-part equality).
func contains(versions []SemVer, target SemVer) bool {
	for _, v := range versions {
		if v == target {
			return true
		}
	}
	return false
}

// highestBelow returns the numerically-greatest version in versions that sorts
// strictly below target — the predecessor for the fresh diff base. ok is false
// when nothing sorts below target (the target is the oldest matching release).
func highestBelow(versions []SemVer, target SemVer) (SemVer, bool) {
	var best SemVer
	found := false
	for _, v := range versions {
		if !target.GreaterThan(v) {
			continue
		}
		if !found || v.GreaterThan(best) {
			best = v
			found = true
		}
	}
	return best, found
}
