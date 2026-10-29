package notes_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// recordedAnswers records every call its record makes, returning err from each.
type recordedAnswers struct {
	calls []notes.Answers
	err   error
}

func (r *recordedAnswers) record(_ context.Context, a notes.Answers) error {
	r.calls = append(r.calls, a)
	return r.err
}

func TestExclusionCheck_HandsRecordEveryAnswerOnceTheLastPauseIsAnswered(t *testing.T) {
	t.Parallel()

	pauses := &recordedPauses{answers: []notes.Answer{notes.AnswerExclude, notes.AnswerKeep, notes.AnswerExclude}}
	recorded := &recordedAnswers{}
	vendorDiff := addedFileDiff("vendor/lib.go", "+package lib\n")
	check := notes.NewExclusionCheck(&recordingTransport{body: "docs/design/: design\ntest/fixtures/: test data\nvendor/: vendored\n"}, nil, nil).
		WithPauses(pauses).
		WithRecord(recorded.record)

	excluded, err := check.Check(t.Context(), sourceDiff+designDiff+fixturesDiff+vendorDiff, config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	want := []notes.Answers{{Exclude: []string{"docs/design/", "vendor/"}, Keep: []string{"test/fixtures/"}}}
	if !slices.EqualFunc(recorded.calls, want, equalAnswers) {
		t.Errorf("recorded %v, want %v", recorded.calls, want)
	}
	if wantExcluded := []string{"docs/design/", "vendor/"}; !slices.Equal(excluded, wantExcluded) {
		t.Errorf("excluded = %q, want %q", excluded, wantExcluded)
	}
}

func TestExclusionCheck_AnswerNone_RecordsNothing(t *testing.T) {
	t.Parallel()

	recorded := &recordedAnswers{}
	check := notes.NewExclusionCheck(&recordingTransport{body: "none\n"}, nil, nil).
		WithPauses(&recordedPauses{}).
		WithRecord(recorded.record)

	if _, err := check.Check(t.Context(), sourceDiff+designDiff, config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	if len(recorded.calls) != 0 {
		t.Errorf("recorded %v, want nothing", recorded.calls)
	}
}

func TestExclusionCheck_AFailedRecordIsReturned(t *testing.T) {
	t.Parallel()

	failure := errors.New("disk full")
	check := notes.NewExclusionCheck(&recordingTransport{body: "docs/design/: design\n"}, nil, nil).
		WithPauses(&recordedPauses{answer: notes.AnswerExclude}).
		WithRecord((&recordedAnswers{err: failure}).record)

	if _, err := check.Check(t.Context(), sourceDiff+designDiff, config.DefaultMaxPromptBytes); !errors.Is(err, failure) {
		t.Errorf("Check error = %v, want the record's failure", err)
	}
}

func TestExclusionCheck_ResolvesWithEachAnswerInTheOrderAsked_OnceTheAnswersAreRecorded(t *testing.T) {
	t.Parallel()

	narration := &checkNarration{}
	pauses := &recordedPauses{answers: []notes.Answer{notes.AnswerExclude, notes.AnswerKeep}}
	record := func(context.Context, notes.Answers) error {
		narration.events = append(narration.events, "recorded")
		return nil
	}
	// docs/ is never asked about: the docs/design/ answer before it decides every path it matches.
	check := notes.NewExclusionCheck(&recordingTransport{body: "docs/design/: design\ndocs/: docs\ntest/fixtures/: test data\n"}, nil, narration).
		WithPauses(pauses).
		WithRecord(record)

	if _, err := check.Check(t.Context(), sourceDiff+designDiff+fixturesDiff, config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	want := []notes.CheckResult{{Answered: []notes.AnsweredSuggestion{
		{Suggestion: notes.Suggestion{Glob: "docs/design/", Reason: "design"}, Answer: notes.AnswerExclude},
		{Suggestion: notes.Suggestion{Glob: "test/fixtures/", Reason: "test data"}, Answer: notes.AnswerKeep},
	}}}
	if !reflect.DeepEqual(narration.results, want) {
		t.Errorf("resolved %+v, want %+v", narration.results, want)
	}
	if wantEvents := []string{"checking", "checked", "recorded", "resolved"}; !slices.Equal(narration.events, wantEvents) {
		t.Errorf("narration = %q, want %q", narration.events, wantEvents)
	}
}

func TestExclusionCheck_PausesThatEndInAnError_ResolveNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		pauses *recordedPauses
		record func(context.Context, notes.Answers) error
	}{
		{name: "a failed pause", pauses: &recordedPauses{err: errors.New("input closed")}},
		{
			name:   "a failed record",
			pauses: &recordedPauses{answer: notes.AnswerExclude},
			record: (&recordedAnswers{err: errors.New("disk full")}).record,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			narration := &checkNarration{}
			check := notes.NewExclusionCheck(&recordingTransport{body: "docs/design/: design\n"}, nil, narration).WithPauses(tt.pauses)
			if tt.record != nil {
				check = check.WithRecord(tt.record)
			}

			if _, err := check.Check(t.Context(), sourceDiff+designDiff, config.DefaultMaxPromptBytes); err == nil {
				t.Fatal("Check returned nil, want the error")
			}
			if len(narration.results) != 0 {
				t.Errorf("resolved %+v, want nothing", narration.results)
			}
		})
	}
}

func equalAnswers(a, b notes.Answers) bool {
	return slices.Equal(a.Exclude, b.Exclude) && slices.Equal(a.Keep, b.Keep)
}
