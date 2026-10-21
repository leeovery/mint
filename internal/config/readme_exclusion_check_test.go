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

func TestREADME_ReleaseSectionDescribesTheExclusionPauses(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"An attended release then pauses once on each suggestion that matches a changed path nothing already covers, in the order the AI gave them, showing the glob and the AI's reason",
		"**`e`** excludes the glob's files from this release's notes, **`k`** keeps them (Enter also keeps)",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}

func TestREADME_ReleaseSectionDescribesTheUnattendedSuggestions(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"An unattended release (`-y`) pauses on nothing and excludes nothing",
		"After the notes it prints each suggestion with the AI's reason, how many changed files it matches and the total size of their diffs, any of those files on `[release].diff_keep`, and the `[release].diff_exclude` line that would exclude it",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}

func TestREADME_ReleaseSectionDescribesTheDryRunSuggestions(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### `release`")
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"A `--dry-run` asks nothing about the suggestions and writes nothing",
		"at a terminal without `-y` it shows each one where its pause would be, as the pause would show it, with no choice offered",
		"an unattended dry run (`-y`, or stdin not a terminal) prints them as an unattended release does, `[release].diff_exclude` line included",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README release section does not say %q", want)
		}
	}
}
