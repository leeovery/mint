package config_test

import (
	"strings"
	"testing"

	"mint/internal/config"
)

const (
	floorOffer = "an attended release is offered the commit-subject list, writing the notes in the editor, or an abort"
	floorStop  = "an unattended one stops"

	notesFailureHeading = "### When the notes fail"
)

func assertNamesTheFloor(t *testing.T, where, text string) {
	t.Helper()
	flat := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{floorOffer, floorStop} {
		if !strings.Contains(flat, want) {
			t.Errorf("%s does not say %q; got:\n%s", where, want, text)
		}
	}
}

// notesFailureSection is the README's notes-failure section, whitespace-flattened so a
// pinned phrase matches across line wraps.
func notesFailureSection(t *testing.T) string {
	t.Helper()
	section := readmeSection(t, readREADME(t, readmePath(t)), notesFailureHeading)
	return strings.Join(strings.Fields(section), " ")
}

func TestMetadataRows_OnNotesFailureNamesTheFloorChoice(t *testing.T) {
	t.Parallel()

	for _, row := range config.MetadataRows() {
		if row.Key == "on_notes_failure" {
			assertNamesTheFloor(t, "on_notes_failure's description", row.Description)
			return
		}
	}
	t.Fatal("config metadata carries no on_notes_failure row")
}

func TestREADME_OnNotesFailureRowNamesTheFloorChoice(t *testing.T) {
	t.Parallel()

	for _, line := range strings.Split(readREADME(t, readmePath(t)), "\n") {
		if strings.HasPrefix(line, "| `on_notes_failure`") {
			assertNamesTheFloor(t, "README on_notes_failure row", line)
			return
		}
	}
	t.Fatal("README carries no on_notes_failure row")
}

func TestREADME_NotesFailureSectionNamesTheFloorChoice(t *testing.T) {
	t.Parallel()

	assertNamesTheFloor(t, "README notes-failure section", readmeSection(t, readREADME(t, readmePath(t)), notesFailureHeading))
}

func TestREADME_NotesFailureSectionDescribesTheFloorsEdit(t *testing.T) {
	t.Parallel()

	flat := notesFailureSection(t)
	for _, want := range []string{
		"**`e`** opens your editor on the chunk notes this run saved, where it saved a file, and otherwise empty",
		"empty or whitespace-only body returns to the choice",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README notes-failure section does not say %q", want)
		}
	}
}

func TestREADME_NotesFailureSectionSaysAnUnattendedStopPrintsNextMoves(t *testing.T) {
	t.Parallel()

	flat := notesFailureSection(t)
	for _, want := range []string{
		"an unattended one stops with next moves fitted to the cause",
		"the `--notes-file` route",
		"`--no-ai`, to release with the commit-subject list",
		"`on_notes_failure = \"fallback\"` under `[release]`",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README notes-failure section does not say %q", want)
		}
	}
}

func TestREADME_NotesFailureSectionNamesTheSizeStopsSettings(t *testing.T) {
	t.Parallel()

	flat := notesFailureSection(t)
	for _, want := range []string{
		"Lower `max_prompt_bytes`",
		"Shorten `[release].prompt` when it's set (it replaces `context`), else `[release].context`",
		"so raise `max_prompt_bytes`",
		"| Part notes too large to merge | None beyond the three above",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README notes-failure section does not say %q", want)
		}
	}
}

func TestREADME_NotesFailureSectionNamesTheTimeoutMissingToolAndOtherMoves(t *testing.T) {
	t.Parallel()

	flat := notesFailureSection(t)
	for _, want := range []string{
		"| Timeout | Raise `[release].timeout`",
		"| AI tool missing | Install it, or set `ai_command` |",
		"The AI's own output names the cause",
		"where the release's `ai_command` isn't the shipped default, the moves also point at the lean command",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README notes-failure section does not say %q", want)
		}
	}
}
