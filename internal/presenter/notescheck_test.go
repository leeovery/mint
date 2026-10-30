package presenter_test

import (
	"strings"
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
		Elapsed:  4200 * time.Millisecond,
		Blocking: true,
	})
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

// TestRailPresenter_FailedExclusionCheck_KeepsTheNotesRunning plays the notes stage's
// failed-check narration, the warning inside a SuspendSpinner/ResumeSpinner bracket,
// in both of its orders: after the check's call ended unusable, and with no call at
// all when the call would not fit. The warning ends the check's node; the notes node
// still runs, its spinner ticking, and the progress that follows draws on it.
func TestRailPresenter_FailedExclusionCheck_KeepsTheNotesRunning(t *testing.T) {
	t.Parallel()

	failedCheck := func(p presenter.Presenter) {
		p.SuspendSpinner()
		p.Warn(presenter.Warning{Key: "check", Label: "notes", Message: "couldn't check for files that don't belong in the notes"})
		p.ResumeSpinner()
	}
	tests := []struct {
		name  string
		drive func(p presenter.Presenter)
	}{
		{
			name: "after the check's call",
			drive: func(p presenter.Presenter) {
				p.NotesChecking(presenter.NotesCheck{Key: "check"})
				p.NotesChecked(presenter.NotesCheck{Key: "check", Failed: true})
				failedCheck(p)
				p.StageStarted(notesRestarted())
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

			r := newRailTerm(t, 100, 24, answers())
			r.RunStarted(releaseStarted(false))
			r.ShowPlan(presenter.Plan{Stages: []presenter.PlanStage{
				{Key: "check", Name: "check", Detail: "files that don't belong in the notes", Inside: true},
				{Key: "notes", Name: "notes", Detail: "written by the AI"},
			}})
			r.StageStarted(notesStarted())
			tt.drive(r)
			r.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: 0})

			screen := strings.Join(r.screen(t), "\n")
			if !strings.Contains(screen, " !  check         couldn't check for files that don't belong in the notes") {
				t.Errorf("the warning didn't end the check's node:\n%s", screen)
			}
			if !strings.Contains(screen, " ⠋  notes") || !strings.Contains(screen, "0 of 6 parts done") {
				t.Errorf("the notes don't run on with their progress:\n%s", screen)
			}
			if !r.ticks.running() {
				t.Error("the spinner's ticker isn't running for the notes")
			}
		})
	}
}
