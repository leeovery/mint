package presenter_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

func twoExclusionSuggestions() presenter.ExclusionSuggestions {
	return presenter.ExclusionSuggestions{
		Stage: "notes",
		Table: "[release]",
		Suggestions: []presenter.ExclusionSuggestion{
			{
				Glob:    "docs/design/",
				Reason:  "design documents",
				Matches: "2 files, 15 KB; on the keep list: docs/design/api.md",
				Line:    `diff_exclude = ["specs/", "docs/design/"]`,
			},
			{
				Glob:    "tmp/",
				Reason:  "scratch files",
				Matches: "1 file, 176 B",
				Line:    `diff_exclude = ["specs/", "tmp/"]`,
			},
		},
	}
}

func TestPlainPresenterExclusionSuggestions_PrintsEachSuggestionWithItsLineOnOneLine(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { p.ExclusionSuggestions(twoExclusionSuggestions()) })

	want := `notes: SUGGEST - exclude docs/design/ (design documents; 2 files, 15 KB; on the keep list: docs/design/api.md) with diff_exclude = ["specs/", "docs/design/"] under [release]` + "\n" +
		`notes: SUGGEST - exclude tmp/ (scratch files; 1 file, 176 B) with diff_exclude = ["specs/", "tmp/"] under [release]` + "\n"
	if got := out.String(); got != want {
		t.Errorf("ExclusionSuggestions out\n got: %q\nwant: %q", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain exclusion suggestions", out, errBuf)
}

func TestExclusionSuggestions_NoSuggestionsPrintNothing(t *testing.T) {
	t.Parallel()

	empty := presenter.ExclusionSuggestions{Stage: "notes", Table: "[release]"}
	plain, _ := drive(func(p *presenter.PlainPresenter) { p.ExclusionSuggestions(empty) })
	if plain.Len() != 0 {
		t.Errorf("plain ExclusionSuggestions = %q, want nothing", plain.String())
	}
	rail, railOut, _ := railGate(termenv.Ascii, nil, gateOpts{})
	rail.ExclusionSuggestions(empty)
	rail.Close()
	if railOut.Len() != 0 {
		t.Errorf("rail ExclusionSuggestions = %q, want nothing", railOut.String())
	}
}

func TestExclusionSuggestions_LeaveTheSuccessLineStanding(t *testing.T) {
	t.Parallel()

	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ExclusionSuggestions(twoExclusionSuggestions())
		p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0"})
	})

	if want := "done: acme v1.4.0\n"; !bytes.HasSuffix(out.Bytes(), []byte(want)) {
		t.Errorf("plain output = %q, want it to end %q", out.String(), want)
	}
}

func TestRecordingPresenter_RecordsExclusionSuggestionsWithTheirPayload(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	rec.ExclusionSuggestions(twoExclusionSuggestions())

	if len(rec.Events) != 1 || rec.Events[0].Kind != presentertest.KindExclusionSuggestions {
		t.Fatalf("events = %v, want one ExclusionSuggestions", rec.Events)
	}
	got := rec.Events[0].ExclusionSuggestions
	want := twoExclusionSuggestions()
	if got.Stage != want.Stage || got.Table != want.Table || len(got.Suggestions) != len(want.Suggestions) {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
	for i := range want.Suggestions {
		if got.Suggestions[i] != want.Suggestions[i] {
			t.Errorf("suggestion %d = %+v, want %+v", i, got.Suggestions[i], want.Suggestions[i])
		}
	}
	if s := presentertest.KindExclusionSuggestions.String(); s != "ExclusionSuggestions" {
		t.Errorf("kind string = %q, want ExclusionSuggestions", s)
	}
}
