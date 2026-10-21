package notes_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"mint/internal/notes"
)

// sizedFileDiff is a modified file's diff section for path, exactly size bytes long.
func sizedFileDiff(t *testing.T, path string, size int) string {
	t.Helper()
	header := "diff --git a/" + path + " b/" + path + "\n@@ -1 +1 @@\n+"
	if size < len(header)+1 {
		t.Fatalf("a %d-byte section is smaller than its header", size)
	}
	return header + strings.Repeat("x", size-len(header)-1) + "\n"
}

// answeredCheck runs the check over diff with keep, the AI answering answer and the
// pauses answering answers in turn, and returns the pauses.
func answeredCheck(t *testing.T, keep []string, diff, answer string, answers ...notes.Answer) *recordedPauses {
	t.Helper()
	pauses := &recordedPauses{answers: answers}
	check := notes.NewExclusionCheck(&recordingTransport{body: answer}, keep, nil).WithPauses(pauses)
	if err := check.Check(t.Context(), diff); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	return pauses
}

func globs(suggestions []notes.Suggestion) []string {
	out := make([]string, len(suggestions))
	for i, s := range suggestions {
		out[i] = s.Glob
	}
	return out
}

// docsRelease is the docs/design/a.md, docs/design/b.md and docs/api.md changes, at
// 12,000, 3,000 and 2,000 bytes, plus extra; git has left docs/old/ and the plain-mode
// version_file out of the notes diff already.
func docsRelease(t *testing.T, extra ...string) string {
	t.Helper()
	diff := sizedFileDiff(t, "docs/api.md", 2_000) +
		sizedFileDiff(t, "docs/design/a.md", 12_000) +
		sizedFileDiff(t, "docs/design/b.md", 3_000)
	for _, path := range extra {
		diff += sizedFileDiff(t, path, 500)
	}
	return diff
}

var apiKept = []string{"docs/api.md"}

func TestExclusionCheck_APause_CountsTheFilesTheNotesCarryAndTheirDiffSizes(t *testing.T) {
	t.Parallel()

	pauses := answeredCheck(t, apiKept, docsRelease(t), "docs/: documentation\n")

	want := []notes.Matches{{
		Paths:     []string{"docs/api.md", "docs/design/a.md", "docs/design/b.md"},
		DiffBytes: 17_000,
		Kept:      []string{"docs/api.md"},
	}}
	if !reflect.DeepEqual(pauses.matches, want) {
		t.Errorf("pause matches = %+v, want %+v", pauses.matches, want)
	}
}

func TestExclusionCheck_AnEarlierExcludeAnswer_TakesItsFilesOutOfALaterCount(t *testing.T) {
	t.Parallel()

	pauses := answeredCheck(t, apiKept, sizedFileDiff(t, "README.md", 1_000)+docsRelease(t),
		"docs/design/: design documents\n*.md: documentation\n", notes.AnswerExclude)

	if got, want := globs(pauses.paused), []string{"docs/design/", "*.md"}; !slices.Equal(got, want) {
		t.Fatalf("paused on %q, want %q", got, want)
	}
	want := notes.Matches{Paths: []string{"README.md", "docs/api.md"}, DiffBytes: 3_000, Kept: []string{"docs/api.md"}}
	if !reflect.DeepEqual(pauses.matches[1], want) {
		t.Errorf("the *.md pause matches = %+v, want %+v", pauses.matches[1], want)
	}
}

func TestExclusionCheck_AnEarlierKeepAnswer_CountsItsFilesAsOnTheKeepList(t *testing.T) {
	t.Parallel()

	pauses := answeredCheck(t, apiKept, docsRelease(t, "docs/guide.md"),
		"docs/design/: design documents\ndocs/: documentation\n", notes.AnswerKeep)

	if got, want := globs(pauses.paused), []string{"docs/design/", "docs/"}; !slices.Equal(got, want) {
		t.Fatalf("paused on %q, want %q", got, want)
	}
	want := notes.Matches{
		Paths:     []string{"docs/api.md", "docs/design/a.md", "docs/design/b.md", "docs/guide.md"},
		DiffBytes: 17_500,
		Kept:      []string{"docs/api.md", "docs/design/a.md", "docs/design/b.md"},
	}
	if !reflect.DeepEqual(pauses.matches[1], want) {
		t.Errorf("the docs/ pause matches = %+v, want %+v", pauses.matches[1], want)
	}
}

func TestExclusionCheck_ASuggestionEarlierAnswersAlreadyCover_GetsNoPause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		answer notes.Answer
	}{
		{name: "after Exclude", answer: notes.AnswerExclude},
		{name: "after Keep", answer: notes.AnswerKeep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pauses := answeredCheck(t, nil, docsRelease(t),
				"docs/: documentation\ndocs/design/: design documents\n", tt.answer)

			if got, want := globs(pauses.paused), []string{"docs/"}; !slices.Equal(got, want) {
				t.Errorf("paused on %q, want only %q", got, want)
			}
		})
	}
}

func TestExclusionCheck_ASuggestionWhoseOnlyMatchAnEarlierKeepCovers_GetsNoPause(t *testing.T) {
	t.Parallel()

	pauses := answeredCheck(t, nil, docsRelease(t),
		"docs/design/: design documents\ndocs/design/a.md: a design document\n", notes.AnswerKeep)

	if got, want := globs(pauses.paused), []string{"docs/design/"}; !slices.Equal(got, want) {
		t.Errorf("paused on %q, want only %q", got, want)
	}
}

func TestExclusionCheck_ASuggestionWithAnUndecidedMatch_StillPausesAfterEarlierAnswers(t *testing.T) {
	t.Parallel()

	pauses := answeredCheck(t, nil, docsRelease(t),
		"docs/design/a.md: a design document\ndocs/design/: design documents\n", notes.AnswerExclude)

	if got, want := globs(pauses.paused), []string{"docs/design/a.md", "docs/design/"}; !slices.Equal(got, want) {
		t.Fatalf("paused on %q, want %q", got, want)
	}
	want := notes.Matches{Paths: []string{"docs/design/b.md"}, DiffBytes: 3_000}
	if !reflect.DeepEqual(pauses.matches[1], want) {
		t.Errorf("the docs/design/ pause matches = %+v, want %+v", pauses.matches[1], want)
	}
}
