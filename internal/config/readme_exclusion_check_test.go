package config_test

import (
	"strings"
	"testing"
)

const exclusionCheckHeading = "### Files that don't belong in the notes"

func TestREADME_ExclusionCheckSectionDescribesTheCheck(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"Before the AI writes a release's notes, mint asks the AI which changed paths look like they don't belong in them",
		"`r` at the review gate never repeats it",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}

func TestREADME_ExclusionCheckSectionDescribesThePauses(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"an attended release then pauses once on each suggestion that matches a changed path nothing already covers, in the order the AI gave them",
		"Each pause shows the glob and the AI's reason, how many changed files the notes still carry that it matches, the total size of their diffs, and any of them on `[release].diff_keep` or kept earlier in the run, which an Exclude takes out too",
		"A suggestion that only matches files already excluded, kept, or decided earlier in the run gets no pause",
		"Press **`e`** to exclude the glob's files from this release's notes, or **`k`** to keep them (Enter also keeps)",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}

func TestREADME_ExclusionCheckSectionDescribesTheUnattendedSuggestions(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"**Unattended** (`-y`), a release pauses on nothing and excludes nothing",
		"After the notes, mint prints each suggestion with the AI's reason, how many changed files it matches and the total size of their diffs, any of those files on `[release].diff_keep`, and the `diff_exclude` line that would exclude it, to set under `[release]`",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}

func TestREADME_ExclusionCheckSectionDescribesTheDryRunSuggestions(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"**A `--dry-run`** asks nothing about the suggestions and writes nothing",
		"At a terminal without `-y` it shows each suggestion as a pause would, without asking",
		"an unattended dry run (`-y`, or stdin not a terminal) prints them as an unattended release does, `diff_exclude` line included",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}

func TestREADME_ExclusionCheckSectionDescribesTheFailedCheck(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"mint warns `couldn't check for files that don't belong in the notes`, with the AI's output beneath it where there is any, and the release carries on to its notes with no suggestions",
		"or because the list of changed paths won't fit in `max_prompt_bytes` even summed up",
		"Ctrl-C during the check cancels the run",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}

func TestREADME_ExclusionCheckSectionSaysTheAnswersAreRecordedAndCommittedWithTheRelease(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), exclusionCheckHeading)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"Once the last pause is answered, mint records the answers in `.mint.toml`'s `[release]` table, an Exclude joining `diff_exclude` and a Keep joining `diff_keep`",
		"It commits `.mint.toml` alone as `chore(release): release-notes path answers for {tag}`, whatever `commit_prefix` says",
		"The commit goes out with the release's push",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("README exclusion-check section does not say %q", want)
		}
	}
}
