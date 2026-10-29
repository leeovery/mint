package engine

import (
	"testing"

	"mint/internal/notes"
)

func TestCheckResultText_StatesWhatTheCheckCameTo(t *testing.T) {
	t.Parallel()

	answered := func(glob, reason string, answer notes.Answer) notes.AnsweredSuggestion {
		return notes.AnsweredSuggestion{Suggestion: notes.Suggestion{Glob: glob, Reason: reason}, Answer: answer}
	}
	design := answered("docs/design/", "design documents", notes.AnswerExclude)
	fixtures := answered("test/fixtures/", "test data", notes.AnswerKeep)
	vendor := answered("vendor/", "vendored code", notes.AnswerExclude)
	specs := answered("docs/specs/", "specs", notes.AnswerKeep)
	for _, tc := range []struct {
		name   string
		result notes.CheckResult
		want   string
	}{
		{name: "one excluded", result: notes.CheckResult{Answered: []notes.AnsweredSuggestion{design}}, want: "docs/design/ excluded · design documents"},
		{name: "one kept", result: notes.CheckResult{Answered: []notes.AnsweredSuggestion{fixtures}}, want: "test/fixtures/ kept · test data"},
		{name: "several of each", result: notes.CheckResult{Answered: []notes.AnsweredSuggestion{design, fixtures, vendor}}, want: "2 left out · 1 kept"},
		{name: "several left out", result: notes.CheckResult{Answered: []notes.AnsweredSuggestion{design, vendor}}, want: "2 left out"},
		{name: "several kept", result: notes.CheckResult{Answered: []notes.AnsweredSuggestion{fixtures, specs}}, want: "2 kept"},
		{name: "flagged, not asked", result: notes.CheckResult{Flagged: 1}, want: "1 flagged, not asked"},
		{name: "nothing asked", result: notes.CheckResult{}, want: "nothing to leave out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := checkResultText(tc.result); got != tc.want {
				t.Errorf("checkResultText = %q, want %q", got, tc.want)
			}
		})
	}
}
