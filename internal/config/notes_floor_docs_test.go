package config_test

import (
	"strings"
	"testing"

	"mint/internal/config"
)

const (
	floorOffer = "an attended release is offered the commit-subject list or an abort"
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
