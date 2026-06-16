package setupguide_test

import (
	"strings"
	"testing"

	"mint/internal/setupguide"
)

// sectionMarkers pairs each required section's exported marker constant with a
// human label used in failure messages. The structural test keys on these
// CONSTANTS, never on representative prose, so a wording change inside any
// section cannot break the test and removing a marker must break it.
var sectionMarkers = []struct {
	label  string
	marker string
}{
	{"pipeline", setupguide.MarkerPipeline},
	{"etiquette", setupguide.MarkerEtiquette},
	{"minimalism", setupguide.MarkerMinimalism},
	{"existing-config", setupguide.MarkerExistingConfig},
	{"config-reference", setupguide.MarkerConfigReference},
}

// TestGuide_EmitsEverySectionMarker proves Guide() carries each of the five
// required section markers. Detection keys on the marker CONSTANTS, decoupled
// from prose, so this is the structural presence proof the spec's "Stable
// section markers" mandate requires.
func TestGuide_EmitsEverySectionMarker(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	for _, sm := range sectionMarkers {
		if !strings.Contains(body, sm.marker) {
			t.Errorf("guide missing %s section marker %q", sm.label, sm.marker)
		}
	}
}

// TestGuide_MarkersAreUnique guards against two sections sharing a marker — a
// collision would let one section's presence mask another's absence, defeating
// the per-section structural proof.
func TestGuide_MarkersAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]string, len(sectionMarkers))
	for _, sm := range sectionMarkers {
		if prior, dup := seen[sm.marker]; dup {
			t.Errorf("marker %q is shared by %s and %s sections", sm.marker, prior, sm.label)
		}
		seen[sm.marker] = sm.label
	}
}

// TestGuide_MarkersAreCommentAnchorsNotProse proves the markers are HTML-comment
// anchors that will not collide incidentally with prose: each begins with the
// fixed `<!-- mint:section:` namespace and closes the comment. This is what
// keeps detection decoupled from the body text.
func TestGuide_MarkersAreCommentAnchorsNotProse(t *testing.T) {
	t.Parallel()

	for _, sm := range sectionMarkers {
		if !strings.HasPrefix(sm.marker, "<!-- mint:section:") {
			t.Errorf("%s marker %q must use the <!-- mint:section: anchor namespace", sm.label, sm.marker)
		}
		if !strings.HasSuffix(sm.marker, "-->") {
			t.Errorf("%s marker %q must close the HTML comment with -->", sm.label, sm.marker)
		}
	}
}

// TestGuide_EachMarkerSitsOnItsOwnLine proves every marker is emitted on its own
// line immediately preceding its section. This is the negative guard the spec
// demands: detection is line-anchored on the marker, so prose alone — a section
// body without its marker line — would NOT be detected. A marker buried mid-line
// inside prose would fail here.
func TestGuide_EachMarkerSitsOnItsOwnLine(t *testing.T) {
	t.Parallel()

	lines := strings.Split(setupguide.Guide(), "\n")
	for _, sm := range sectionMarkers {
		if !lineEquals(lines, sm.marker) {
			t.Errorf("%s marker %q must appear on its own line, not embedded in prose", sm.label, sm.marker)
		}
	}
}

// lineEquals reports whether marker appears as a whole line (ignoring leading or
// trailing whitespace) anywhere in lines.
func lineEquals(lines []string, marker string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

// TestGuide_MentionsReleaseShimRole proves the pipeline section carries the
// one-line release-shim role mention: what ./release is and that mint init
// creates it, so the agent's picture of the release pipeline is complete.
func TestGuide_MentionsReleaseShimRole(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	if !strings.Contains(body, "./release") {
		t.Error("guide must mention the ./release shim by name")
	}
	if !strings.Contains(body, "mint init") {
		t.Error("guide must say mint init creates the release shim")
	}
}

// TestGuide_FirstProcedureStepIsCwdConfirm proves the cwd-confirm safety step is
// the FIRST ordered procedure step — the in-instructions safety net that
// replaces mint setup's missing cwd guard. The procedure numbers its steps, so
// step "1." must reference confirming the working directory / repo root before
// any other numbered step.
func TestGuide_FirstProcedureStepIsCwdConfirm(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	firstStep := firstNumberedStep(body)
	if firstStep == "" {
		t.Fatal("guide carries no numbered procedure steps")
	}
	lower := strings.ToLower(firstStep)
	if !strings.Contains(lower, "working directory") && !strings.Contains(lower, "repo root") {
		t.Errorf("first procedure step must confirm the working directory / repo root, got: %q", firstStep)
	}
}

// firstNumberedStep returns the first line that opens an ordered "1." procedure
// step, trimmed. Returns "" if none is found.
func firstNumberedStep(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "1.") {
			return trimmed
		}
	}
	return ""
}

// TestGuide_CarriesAIModelPerVerbMapping proves the guide explains the
// AI-model-per-verb mapping: the same model -> the shared top-level ai_command;
// different models -> [release].ai_command + [commit].ai_command per-verb
// overrides.
func TestGuide_CarriesAIModelPerVerbMapping(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	for _, want := range []string{"ai_command", "[release].ai_command", "[commit].ai_command"} {
		if !strings.Contains(body, want) {
			t.Errorf("AI-model-per-verb mapping must reference %q", want)
		}
	}
}

// TestGuide_CarriesDiffExcludeScope proves the guide carries diff_exclude scope:
// it is release-notes noise (tracked process/meta/doc files such as .workflows/,
// .claude/, docs/), not generated code, and gitignore'd paths are already
// absent.
func TestGuide_CarriesDiffExcludeScope(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	if !strings.Contains(body, "diff_exclude") {
		t.Error("guide must reference diff_exclude")
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "release-notes noise") && !strings.Contains(lower, "release notes noise") {
		t.Error("diff_exclude scope must frame it as release-notes noise, not generated code")
	}
	if !strings.Contains(lower, "gitignore") {
		t.Error("diff_exclude scope must note that gitignore'd paths are already absent from the diff")
	}
}
