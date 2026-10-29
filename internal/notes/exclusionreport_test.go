package notes_test

import (
	"reflect"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
)

type reported struct {
	suggestion notes.Suggestion
	matches    notes.Matches
}

// reportedCheck runs the check over diff with keep, the AI answering answer and each
// suggestion reported rather than paused on, and returns the reports and the globs
// the check excluded.
func reportedCheck(t *testing.T, keep []string, diff, answer string) ([]reported, []string) {
	t.Helper()
	var reports []reported
	check := notes.NewExclusionCheck(&recordingTransport{body: answer}, keep, nil).
		WithReport(func(s notes.Suggestion, m notes.Matches) { reports = append(reports, reported{s, m}) })
	excluded, err := check.Check(t.Context(), diff, config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	return reports, excluded
}

func TestExclusionCheck_Reporting_ReportsEachUsableSuggestionWithItsMatchesAndExcludesNothing(t *testing.T) {
	t.Parallel()

	reports, excluded := reportedCheck(t, apiKept, docsRelease(t, "src/export.go"),
		"docs/: documentation\nvendor/: matches nothing\ndocs/design/: design documents\n")

	want := []reported{
		{
			suggestion: notes.Suggestion{Glob: "docs/", Reason: "documentation"},
			matches: notes.Matches{
				Paths:     []string{"docs/api.md", "docs/design/a.md", "docs/design/b.md"},
				DiffBytes: 17_000,
				Kept:      []string{"docs/api.md"},
			},
		},
		{
			suggestion: notes.Suggestion{Glob: "docs/design/", Reason: "design documents"},
			matches: notes.Matches{
				Paths:     []string{"docs/design/a.md", "docs/design/b.md"},
				DiffBytes: 15_000,
			},
		},
	}
	if !reflect.DeepEqual(reports, want) {
		t.Errorf("reports = %+v, want %+v", reports, want)
	}
	if len(excluded) != 0 {
		t.Errorf("excluded = %q, want nothing", excluded)
	}
}

func TestExclusionCheck_Reporting_ResolvesWithHowManySuggestionsItReported(t *testing.T) {
	t.Parallel()

	narration := &checkNarration{}
	var reports int
	check := notes.NewExclusionCheck(&recordingTransport{body: "docs/design/: design\nvendor/: matches nothing\ntest/fixtures/: test data\n"}, nil, narration).
		WithReport(func(notes.Suggestion, notes.Matches) { reports++ })

	if _, err := check.Check(t.Context(), sourceDiff+designDiff+fixturesDiff, config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if want := []notes.CheckResult{{Flagged: 2}}; reports != 2 || !reflect.DeepEqual(narration.results, want) {
		t.Errorf("reported %d and resolved %+v, want 2 reported and %+v", reports, narration.results, want)
	}
}

func TestExclusionCheck_Reporting_AnswerNone_ReportsNothing(t *testing.T) {
	t.Parallel()

	if reports, _ := reportedCheck(t, nil, sourceDiff+designDiff, "none\n"); len(reports) != 0 {
		t.Errorf("reports = %+v, want none", reports)
	}
}

func TestExclusionCheck_Reporting_AFailedCall_ReportsNothing(t *testing.T) {
	t.Parallel()

	var reports int
	check := notes.NewExclusionCheck(&recordingTransport{err: ai.ErrGenerationFailed}, nil, nil).
		WithReport(func(notes.Suggestion, notes.Matches) { reports++ })
	if _, err := check.Check(t.Context(), sourceDiff+designDiff, config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	if reports != 0 {
		t.Errorf("reports = %d, want none", reports)
	}
}
