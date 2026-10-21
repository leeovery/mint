package notes_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/notes"
)

// recordedPauses answers every pause with answer, or fails it with err, recording each
// suggestion it was paused on.
type recordedPauses struct {
	answer notes.Answer
	err    error
	// during, when set, runs as each pause waits for its answer.
	during func()
	paused []notes.Suggestion
}

func (p *recordedPauses) Pause(_ context.Context, s notes.Suggestion) (notes.Answer, error) {
	p.paused = append(p.paused, s)
	if p.during != nil {
		p.during()
	}
	return p.answer, p.err
}

// pausedCheck runs the check over diff with keep, the AI answering answer, and returns
// the suggestions it paused on.
func pausedCheck(t *testing.T, keep []string, diff, answer string) []notes.Suggestion {
	t.Helper()
	pauses := &recordedPauses{}
	check := notes.NewExclusionCheck(&recordingTransport{body: answer}, keep, nil).WithPauses(pauses)
	if err := check.Check(t.Context(), diff); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	return pauses.paused
}

var (
	designDiff   = addedFileDiff("docs/design/export.md", "+# Export design\n")
	fixturesDiff = addedFileDiff("test/fixtures/export.json", "+{}\n")
	sourceDiff   = modifiedFileDiff("src/export.go", "+package src\n")
)

func TestExclusionCheck_PausesOnEachSuggestionInTheOrderTheAIGaveThem(t *testing.T) {
	t.Parallel()

	got := pausedCheck(t, nil, sourceDiff+designDiff+fixturesDiff,
		"docs/design/: design documents, not shipped behaviour\ntest/fixtures/: test data\n")

	want := []notes.Suggestion{
		{Glob: "docs/design/", Reason: "design documents, not shipped behaviour"},
		{Glob: "test/fixtures/", Reason: "test data"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("paused on %v, want %v", got, want)
	}
}

func TestExclusionCheck_AnswerNone_PausesOnNothing(t *testing.T) {
	t.Parallel()

	if got := pausedCheck(t, nil, sourceDiff+designDiff, "none\n"); len(got) != 0 {
		t.Errorf("paused on %v, want no pause", got)
	}
}

func TestExclusionCheck_ASuggestionMatchingNoUncoveredPath_GetsNoPause(t *testing.T) {
	t.Parallel()

	keptDiff := addedFileDiff("docs/api/export.md", "+# Export API\n")
	got := pausedCheck(t, []string{"docs/api/"}, sourceDiff+keptDiff+designDiff,
		"build/: build output\ndocs/api/: API docs\ndocs/design/: design documents\n")

	want := []notes.Suggestion{{Glob: "docs/design/", Reason: "design documents"}}
	if !slices.Equal(got, want) {
		t.Errorf("paused on %v, want only %v", got, want)
	}
}

func TestExclusionCheck_ASuggestionMatchesAsAGitPathspec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, glob string
	}{
		{name: "a * crossing folders", glob: "*.md"},
		{name: "a leading ./", glob: "./docs/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diff := sourceDiff + modifiedFileDiff("docs/guide/intro.md", "+intro\n")
			got := pausedCheck(t, nil, diff, tt.glob+": documentation\n")

			want := []notes.Suggestion{{Glob: tt.glob, Reason: "documentation"}}
			if !slices.Equal(got, want) {
				t.Errorf("paused on %v, want %v", got, want)
			}
		})
	}
}

func TestExclusionCheck_WithoutPauses_AsksNothingAndCarriesOn(t *testing.T) {
	t.Parallel()

	tr := &recordingTransport{body: "docs/design/: design documents\n"}
	if err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), designDiff); err != nil {
		t.Errorf("Check returned %v, want nil", err)
	}
}

func TestExclusionCheck_APauseThatFails_ReturnsItsErrorAndAsksNoMore(t *testing.T) {
	t.Parallel()

	closed := errors.New("input closed")
	pauses := &recordedPauses{err: closed}
	tr := &recordingTransport{body: "docs/design/: design documents\ntest/fixtures/: test data\n"}

	err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(t.Context(), designDiff+fixturesDiff)

	if !errors.Is(err, closed) {
		t.Errorf("Check returned %v, want the pause's error", err)
	}
	if len(pauses.paused) != 1 {
		t.Errorf("pauses = %d, want the one that failed", len(pauses.paused))
	}
}

func TestExclusionCheck_ACancelWhileAPauseWaits_ReturnsTheCancelWhateverItsAnswer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pauses := &recordedPauses{answer: notes.AnswerKeep, during: cancel}
	tr := &recordingTransport{body: "docs/design/: design documents\ntest/fixtures/: test data\n"}

	err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(ctx, designDiff+fixturesDiff)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Check returned %v, want the cancel", err)
	}
	if len(pauses.paused) != 1 {
		t.Errorf("pauses = %d, want none after the cancel", len(pauses.paused))
	}
}
