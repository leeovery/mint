package notes_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// recordedPauses answers each pause with the next of answers, then with answer, or
// fails it with err, recording each suggestion it was paused on and what it matched.
type recordedPauses struct {
	answers []notes.Answer
	answer  notes.Answer
	err     error
	// during, when set, runs as each pause waits for its answer.
	during  func()
	paused  []notes.Suggestion
	matches []notes.Matches
}

func (p *recordedPauses) Pause(_ context.Context, s notes.Suggestion, m notes.Matches) (notes.Answer, error) {
	p.paused = append(p.paused, s)
	p.matches = append(p.matches, m)
	if p.during != nil {
		p.during()
	}
	if len(p.answers) > 0 {
		answer := p.answers[0]
		p.answers = p.answers[1:]
		return answer, p.err
	}
	return p.answer, p.err
}

// pausedCheck runs the check over diff with keep, the AI answering answer, and returns
// the suggestions it paused on.
func pausedCheck(t *testing.T, keep []string, diff, answer string) []notes.Suggestion {
	t.Helper()
	pauses := &recordedPauses{}
	check := notes.NewExclusionCheck(&recordingTransport{body: answer}, keep, nil).WithPauses(pauses)
	if _, err := check.Check(t.Context(), diff, config.DefaultMaxPromptBytes); err != nil {
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
	if _, err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), designDiff, config.DefaultMaxPromptBytes); err != nil {
		t.Errorf("Check returned %v, want nil", err)
	}
}

func TestExclusionCheck_APauseThatFails_ReturnsItsErrorAndAsksNoMore(t *testing.T) {
	t.Parallel()

	closed := errors.New("input closed")
	pauses := &recordedPauses{err: closed}
	tr := &recordingTransport{body: "docs/design/: design documents\ntest/fixtures/: test data\n"}

	_, err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(t.Context(), designDiff+fixturesDiff, config.DefaultMaxPromptBytes)

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

	_, err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(ctx, designDiff+fixturesDiff, config.DefaultMaxPromptBytes)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Check returned %v, want the cancel", err)
	}
	if len(pauses.paused) != 1 {
		t.Errorf("pauses = %d, want none after the cancel", len(pauses.paused))
	}
}

func TestExclusionCheck_ReturnsTheGlobsAnsweredExcludeInTheOrderAnswered(t *testing.T) {
	t.Parallel()

	pauses := &recordedPauses{answers: []notes.Answer{notes.AnswerExclude, notes.AnswerKeep, notes.AnswerExclude}}
	tr := &recordingTransport{body: "docs/design/: design documents\nsrc/: source\ntest/fixtures/: test data\n"}

	excluded, err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(t.Context(), sourceDiff+designDiff+fixturesDiff, config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if want := []string{"docs/design/", "test/fixtures/"}; !slices.Equal(excluded, want) {
		t.Errorf("excluded = %q, want %q", excluded, want)
	}
}

func TestExclusionCheck_EveryPauseAnsweredKeep_ReturnsNoGlob(t *testing.T) {
	t.Parallel()

	pauses := &recordedPauses{answer: notes.AnswerKeep}
	tr := &recordingTransport{body: "docs/design/: design documents\ntest/fixtures/: test data\n"}

	excluded, err := notes.NewExclusionCheck(tr, nil, nil).WithPauses(pauses).Check(t.Context(), sourceDiff+designDiff+fixturesDiff, config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if len(excluded) != 0 {
		t.Errorf("excluded = %q, want none", excluded)
	}
}

// TestExclusionCheck_TheCallsEndCountsWhatTheRunWillAskOrReport pins the count the
// check's call ends with: what the pauses go on to ask, a suggestion the ones before
// it already decide left out whatever they are answered, or, reported, every usable
// suggestion; and whether they are asked.
func TestExclusionCheck_TheCallsEndCountsWhatTheRunWillAskOrReport(t *testing.T) {
	t.Parallel()

	// docs/ matches only docs/design/export.md, which docs/design/ before it decides.
	const answer = "docs/design/: design\ndocs/: docs\ntest/fixtures/: test data\n"
	diff := sourceDiff + designDiff + fixturesDiff
	for _, answers := range [][]notes.Answer{{notes.AnswerExclude, notes.AnswerKeep}, {notes.AnswerKeep, notes.AnswerKeep}} {
		narration := &checkNarration{}
		pauses := &recordedPauses{answers: answers}
		if _, err := notes.NewExclusionCheck(&recordingTransport{body: answer}, nil, narration).WithPauses(pauses).Check(t.Context(), diff, config.DefaultMaxPromptBytes); err != nil {
			t.Fatalf("Check returned unexpected error: %v", err)
		}
		want := []notes.CheckResult{{Outcome: notes.CheckAnswered, Flagged: 2, Asked: true}}
		if !slices.Equal(narration.results, want) || len(pauses.paused) != 2 {
			t.Errorf("answered %v: checked %+v and paused %d times, want %+v and the 2 it counted", answers, narration.results, len(pauses.paused), want)
		}
	}

	narration := &checkNarration{}
	reported := 0
	check := notes.NewExclusionCheck(&recordingTransport{body: answer}, nil, narration).
		WithReport(func(notes.Suggestion, notes.Matches) { reported++ })
	if _, err := check.Check(t.Context(), diff, config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	if want := []notes.CheckResult{{Outcome: notes.CheckAnswered, Flagged: 3}}; !slices.Equal(narration.results, want) || reported != 3 {
		t.Errorf("reporting: checked %+v and reported %d, want %+v and the 3 it counted", narration.results, reported, want)
	}
}
