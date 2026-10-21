package config_test

import (
	"strings"
	"testing"
)

func TestREADME_ReleaseSectionDescribesTheExclusionCheck(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"Before the AI writes a release's notes, mint asks the AI which changed paths look like they don't belong in them",
		"`r` at the review gate never repeats it",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}
