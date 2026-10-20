package config_test

import (
	"strings"
	"testing"

	"mint/internal/config"
)

const (
	floorOffer = "an attended release is offered the commit-subject list, writing the notes in the editor, or an abort"
	floorStop  = "an unattended one stops"
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

func TestREADME_ReleaseSectionNamesTheFloorChoice(t *testing.T) {
	t.Parallel()

	assertNamesTheFloor(t, "README release section", readmeSection(t, readREADME(t, readmePath(t)), "### `release`"))
}

func TestREADME_ReleaseSectionDescribesTheFloorsEdit(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"**`e`** opens `$EDITOR` on the chunk notes this run saved, where it saved a file, and otherwise empty",
		"an empty or whitespace-only body returns to the choice",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}

func TestREADME_ReleaseSectionSaysAnUnattendedStopPrintsNextMoves(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	if want := "an unattended one stops with next moves fitted to the cause"; !strings.Contains(flat, want) {
		t.Errorf("README release section does not say %q", want)
	}
}

func TestREADME_ReleaseSectionNamesTheSizeStopsSettings(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"A stop on size adds the setting to change.",
		"shorten `[release].prompt` when it is set, since it replaces `context`, and `[release].context` otherwise",
		"so raise `max_prompt_bytes`",
		"Part notes too large to merge get only the moves above",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}

func TestREADME_ReleaseSectionNamesTheTimeoutMissingToolAndOtherMoves(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"A timeout adds raise `[release].timeout`",
		"a missing AI tool adds install it, or set `ai_command`",
		"Any other failure leaves the AI's own output to name the cause",
		"where the release's `ai_command` is not the shipped default, the moves point at the lean command",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}
