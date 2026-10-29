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
	p.NotesChecking(presenter.NotesCheck{})
	p.NotesChecked(presenter.NotesCheck{})
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
		p.NotesChecking(presenter.NotesCheck{})
		p.NotesChecked(presenter.NotesCheck{})
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

// TestPrettyPresenter_FailedExclusionCheck_KeepsTheNotesLineRunning plays the notes
// stage's failed-check narration — the ⚠ inside a SuspendSpinner/ResumeSpinner
// bracket — in both of its orders: after the check's call ended, and with no call at
// all when the call would not fit. The bracket's resume must restore the notes line,
// so the chunked run's progress that follows still shows on a live spinner.
func TestPrettyPresenter_FailedExclusionCheck_KeepsTheNotesLineRunning(t *testing.T) {
	t.Parallel()

	failedCheck := func(p *presenter.PrettyPresenter) {
		p.SuspendSpinner()
		p.Warn(presenter.Warning{Label: "notes", Message: "couldn't check for files that don't belong in the notes"})
		p.ResumeSpinner()
	}
	tests := []struct {
		name  string
		drive func(p *presenter.PrettyPresenter)
	}{
		{
			name: "after the check's call",
			drive: func(p *presenter.PrettyPresenter) {
				p.NotesChecking(presenter.NotesCheck{})
				p.NotesChecked(presenter.NotesCheck{})
				failedCheck(p)
			},
		},
		{
			name:  "with no call when it would not fit",
			drive: failedCheck,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
				p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
				tt.drive(p)
				p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: 0})
			})

			if tr.active != 1 {
				t.Fatalf("spinners active after the resume = %d, want the notes line running", tr.active)
			}
			last := tr.created[len(tr.created)-1]
			if !last.started || last.stopped {
				t.Fatalf("notes line after the resume: started=%v stopped=%v, want started and running", last.started, last.stopped)
			}
			want := "Writing notes: 3.1 MB release, in 6 parts · 0 of 6 done"
			if len(last.texts) == 0 || last.texts[len(last.texts)-1] != want {
				t.Errorf("notes line texts = %q, want them to end with %q", last.texts, want)
			}
		})
	}
}

func TestPrettyPresenter_ExclusionCheckWithNoNotesLine_StartsNoSpinner(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.NotesChecking(presenter.NotesCheck{})
		p.NotesChecked(presenter.NotesCheck{})
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
