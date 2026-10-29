package notes_test

import (
	"context"
	"errors"
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

func equalAnswers(a, b notes.Answers) bool {
	return slices.Equal(a.Exclude, b.Exclude) && slices.Equal(a.Keep, b.Keep)
}
