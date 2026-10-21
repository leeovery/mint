package presenter_test

import (
	"slices"
	"testing"
	"time"

	"mint/internal/presenter"
)

// driveCheckedNotesStage plays a single-pass notes stage that runs the exclusion check
// first: the check's call, then the notes, then the completion line.
func driveCheckedNotesStage(p presenter.Presenter) {
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
	p.NotesChecking()
	p.NotesChecked()
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Sentence: "Generated release notes",
		Elapsed:  4200 * time.Millisecond,
		Blocking: true,
	})
}

func TestPrettyPresenter_ExclusionCheck_ShowsOnTheNotesLineThenRestoresIt(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) { driveCheckedNotesStage(p) })

	if len(tr.created) != 1 {
		t.Fatalf("spinners created = %d, want the one notes line", len(tr.created))
	}
	want := []string{
		"Checking for files that don't belong in the notes…",
		"generating release notes…",
	}
	if got := tr.created[0].texts; !slices.Equal(got, want) {
		t.Errorf("notes line texts =\n%q\nwant\n%q", got, want)
	}
	if got, want := out.String(), "✓ Generated release notes (4.2s)\n"; got != want {
		t.Errorf("output = %q, want only the completion line %q", got, want)
	}
}

func TestPrettyPresenter_ExclusionCheckThenChunkedNotes_NarratesTheNotesAsWithoutTheCheck(t *testing.T) {
	t.Parallel()

	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
		p.NotesChecking()
		p.NotesChecked()
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: 0})
	})

	want := []string{
		"Checking for files that don't belong in the notes…",
		"generating release notes…",
		"Writing notes: 3.1 MB release, in 6 parts · 0 of 6 done",
	}
	if got := tr.created[0].texts; !slices.Equal(got, want) {
		t.Errorf("notes line texts =\n%q\nwant\n%q", got, want)
	}
}

func TestPrettyPresenter_ExclusionCheckWithNoNotesLine_StartsNoSpinner(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.NotesChecking()
		p.NotesChecked()
	})

	if len(tr.created) != 0 {
		t.Errorf("spinners created = %d, want none with no notes line running", len(tr.created))
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing", out.String())
	}
}

func TestPlainPresenter_ExclusionCheck_PrintsItsOwnLineBeforeTheNotesLines(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { driveCheckedNotesStage(p) })

	want := "notes: running...\n" +
		"notes: checking for files that don't belong in the notes\n" +
		"notes: generated (4.2s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain exclusion check narration", out, errBuf)
}
