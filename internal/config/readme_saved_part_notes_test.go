package config_test

import (
	"strings"
	"testing"
)

// savedPartNotesReleasing is the README's Saved part notes paragraph on releasing a
// saved file, whitespace-flattened so a pinned phrase matches across line wraps.
func savedPartNotesReleasing(t *testing.T) string {
	t.Helper()
	section := readmeSection(t, readREADME(t, readmePath(t)), "### Saved part notes")
	for _, paragraph := range strings.Split(section, "\n\n") {
		flat := strings.Join(strings.Fields(paragraph), " ")
		if strings.Contains(flat, "delete the marker line") {
			return flat
		}
	}
	t.Fatal("README's Saved part notes section has no paragraph telling the operator to delete the marker line")
	return ""
}

func TestREADME_SavedPartNotesReleasing_DeletesThePartsNothingToListLinesWithTheMarker(t *testing.T) {
	t.Parallel()

	got := savedPartNotesReleasing(t)
	for _, want := range []string{
		"delete the marker line and any part's line saying it held nothing for the notes, then release it with `--notes-file`",
		"Mint checks for neither the marker nor a part's line: a file that still has one ships with it.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("README's saved-notes releasing paragraph does not say %q; got:\n%s", want, got)
		}
	}
}

func TestREADME_SavedPartNotesReleasing_EveryPartNothingToListReleasesTheMaintenanceLine(t *testing.T) {
	t.Parallel()

	got := savedPartNotesReleasing(t)
	for _, want := range []string{
		"Where every part's notes are that line, the release holds nothing users meet",
		"release the maintenance line, `Maintenance release — no notable source changes`, in their place rather than an empty file, which `--notes-file` refuses",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("README's saved-notes releasing paragraph does not say %q; got:\n%s", want, got)
		}
	}
}
