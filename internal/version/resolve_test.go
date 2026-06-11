package version_test

import (
	"strings"
	"testing"

	"mint/internal/version"
)

// TestResolveRegenerateTarget_NormalisesWithPrefix confirms a <version> supplied
// WITH the configured tag_prefix resolves to the canonical prefixed tag.
func TestResolveRegenerateTarget_NormalisesWithPrefix(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.3.0", "v1.4.0")

	got, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v1.4.0")
	if err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	if got.Tag != "v1.4.0" {
		t.Errorf("Tag = %q, want %q", got.Tag, "v1.4.0")
	}
}

// TestResolveRegenerateTarget_NormalisesWithoutPrefix confirms a <version>
// supplied WITHOUT the tag_prefix resolves identically to the prefixed form.
func TestResolveRegenerateTarget_NormalisesWithoutPrefix(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.3.0", "v1.4.0")

	withPrefix, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v1.4.0")
	if err != nil {
		t.Fatalf("with-prefix resolve returned unexpected error: %v", err)
	}

	r2 := seedTags(t, "v1.3.0", "v1.4.0")
	withoutPrefix, err := version.ResolveRegenerateTarget(t.Context(), r2, "v", "1.4.0")
	if err != nil {
		t.Fatalf("without-prefix resolve returned unexpected error: %v", err)
	}

	if withoutPrefix != withPrefix {
		t.Errorf("bare-version resolve = %+v, want identical to prefixed resolve %+v", withoutPrefix, withPrefix)
	}
	if withoutPrefix.Tag != "v1.4.0" {
		t.Errorf("Tag = %q, want %q", withoutPrefix.Tag, "v1.4.0")
	}
}

// TestResolveRegenerateTarget_MonorepoPrefix confirms a component/monorepo
// tag_prefix is honoured in BOTH stripping the supplied value and re-applying it
// to the canonical tag.
func TestResolveRegenerateTarget_MonorepoPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "value with monorepo prefix", value: "pkg-name/v1.2.3"},
		{name: "value without monorepo prefix", value: "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := seedTags(t, "pkg-name/v1.2.0", "pkg-name/v1.2.3")

			got, err := version.ResolveRegenerateTarget(t.Context(), r, "pkg-name/v", tt.value)
			if err != nil {
				t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
			}

			if got.Tag != "pkg-name/v1.2.3" {
				t.Errorf("Tag = %q, want %q", got.Tag, "pkg-name/v1.2.3")
			}
			if got.PreviousTag != "pkg-name/v1.2.0" {
				t.Errorf("PreviousTag = %q, want %q", got.PreviousTag, "pkg-name/v1.2.0")
			}
		})
	}
}

// TestResolveRegenerateTarget_NoMatchingTag_FailsLoud confirms a <version> with
// no matching tag fails loud with "no tag <canonical-tag> found", using the
// CANONICAL (re-prefixed) tag string in the message.
func TestResolveRegenerateTarget_NoMatchingTag_FailsLoud(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.3.0", "v1.4.0")

	_, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "9.9.9")
	if err == nil {
		t.Fatalf("ResolveRegenerateTarget returned nil error, want a fail-loud rejection")
	}

	want := "no tag v9.9.9 found"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestResolveRegenerateTarget_RejectsInvalidVersion confirms a non-strict-3-part
// <version> is rejected (reusing the tag-grammar parser) rather than silently
// treated as no-match.
func TestResolveRegenerateTarget_RejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.4.0")

	if _, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "1.4"); err == nil {
		t.Errorf("ResolveRegenerateTarget(%q) returned nil error, want a rejection", "1.4")
	}
}

// TestResolveRegenerateTarget_FreshDiffBase confirms the fresh diff base is the
// vX-1..vX range: the previous MATCHING tag → the target tag.
func TestResolveRegenerateTarget_FreshDiffBase(t *testing.T) {
	t.Parallel()

	// Predecessor of v2.0.0 is the numerically-next-lower matching tag, v1.9.3 —
	// not the lexically-nearest or git-describe ancestor.
	r := seedTags(t, "v1.0.0", "v1.9.0", "v1.9.3", "v2.0.0", "v2.1.0")

	got, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v2.0.0")
	if err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	if got.FirstRelease {
		t.Errorf("FirstRelease = true, want false for a version with a predecessor")
	}
	if got.PreviousTag != "v1.9.3" {
		t.Errorf("PreviousTag = %q, want %q", got.PreviousTag, "v1.9.3")
	}
	if got.Tag != "v2.0.0" {
		t.Errorf("Tag = %q, want %q", got.Tag, "v2.0.0")
	}
	if want := "v1.9.3..v2.0.0"; got.DiffRange() != want {
		t.Errorf("DiffRange() = %q, want %q", got.DiffRange(), want)
	}
}

// TestResolveRegenerateTarget_NumericPredecessor_NotLexical confirms the
// predecessor is the numerically-previous matching tag, so a double-digit minor
// (v1.10.0) is the predecessor of v2.0.0 over the lexically-greater v1.9.0.
func TestResolveRegenerateTarget_NumericPredecessor_NotLexical(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.9.0", "v1.10.0", "v2.0.0")

	got, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v2.0.0")
	if err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	if got.PreviousTag != "v1.10.0" {
		t.Errorf("PreviousTag = %q, want %q (numeric, not lexical)", got.PreviousTag, "v1.10.0")
	}
}

// TestResolveRegenerateTarget_OldestRelease_MarksFirstRelease confirms the oldest
// release (no predecessor matching tag) is marked firstRelease=true, with no
// predecessor tag and an empty diff range (the fresh path will skip the AI).
func TestResolveRegenerateTarget_OldestRelease_MarksFirstRelease(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v0.1.0", "v0.2.0", "v1.0.0")

	got, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v0.1.0")
	if err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	if !got.FirstRelease {
		t.Errorf("FirstRelease = false, want true for the oldest release")
	}
	if got.PreviousTag != "" {
		t.Errorf("PreviousTag = %q, want empty for the oldest release", got.PreviousTag)
	}
	if got.Tag != "v0.1.0" {
		t.Errorf("Tag = %q, want %q", got.Tag, "v0.1.0")
	}
	if got.DiffRange() != "" {
		t.Errorf("DiffRange() = %q, want empty for the first release", got.DiffRange())
	}
}

// TestResolveRegenerateTarget_IgnoresNonMatchingTags confirms tags that do not
// match the configured grammar/prefix (a stray lightweight tag, a different
// monorepo prefix, a non-semver tag) are ignored when finding the predecessor.
func TestResolveRegenerateTarget_IgnoresNonMatchingTags(t *testing.T) {
	t.Parallel()

	// nightly, a wrong-prefix tag, a 4-segment tag, and a pre-release all sit
	// numerically "between" v1.3.0 and v2.0.0 by shape but must be ignored: the
	// predecessor of v2.0.0 is the matching v1.3.0.
	r := seedTags(t,
		"v1.3.0",
		"v2.0.0",
		"nightly",
		"other/v1.9.0",
		"v1.9.0.4",
		"v1.9.0-rc.1",
		"release-1.9.0",
	)

	got, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v2.0.0")
	if err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	if got.PreviousTag != "v1.3.0" {
		t.Errorf("PreviousTag = %q, want %q (non-matching tags ignored)", got.PreviousTag, "v1.3.0")
	}
}

// TestResolveRegenerateTarget_ListsTagsViaRunner confirms tag discovery goes
// through the CommandRunner seam (git tag --list), reusing the Phase 1 read path
// rather than a second tag lister.
func TestResolveRegenerateTarget_ListsTagsViaRunner(t *testing.T) {
	t.Parallel()

	r := seedTags(t, "v1.3.0", "v1.4.0")

	if _, err := version.ResolveRegenerateTarget(t.Context(), r, "v", "v1.4.0"); err != nil {
		t.Fatalf("ResolveRegenerateTarget returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("recorded %d invocations, want exactly 1", len(invs))
	}
	if invs[0].Name != "git" {
		t.Errorf("invoked %q, want it to run via git", invs[0].Name)
	}
	if got := strings.Join(invs[0].Args, " "); got != "tag --list" {
		t.Errorf("git args = %q, want %q", got, "tag --list")
	}
}
