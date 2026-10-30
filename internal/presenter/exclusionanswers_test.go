package presenter_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

var releaseTableAnswers = presenter.ExclusionAnswers{Stage: "notes", File: ".mint.toml", Table: "[release]"}

// driveAnsweredNotesStage plays a notes stage whose exclusion check paused and
// recorded its answers before the notes were written.
func driveAnsweredNotesStage(p presenter.Presenter) {
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
	p.NotesChecking(presenter.NotesCheck{})
	p.NotesChecked(presenter.NotesCheck{})
	p.SuspendSpinner()
	p.ExclusionAnswersRecorded(releaseTableAnswers)
	p.ResumeSpinner()
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Elapsed:  4200 * time.Millisecond,
		Blocking: true,
	})
}

func TestPlainPresenter_ExclusionAnswersRecorded_PrintsItsOwnLine(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { driveAnsweredNotesStage(p) })

	want := "notes: running...\n" +
		"notes: checking for files that don't belong in the notes\n" +
		"notes: answers written to .mint.toml's [release] table and committed\n" +
		"notes: generated (4.2s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain exclusion answers narration", out, errBuf)
}

func TestRecordingPresenter_ExclusionAnswersRecorded_RecordsItsPayload(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	answers := releaseTableAnswers
	answers.Key = "check"
	rec.ExclusionAnswersRecorded(answers)

	if len(rec.Events) != 1 {
		t.Fatalf("events = %d, want one", len(rec.Events))
	}
	if got := rec.Events[0]; got.Kind != presentertest.KindExclusionAnswersRecorded || !reflect.DeepEqual(got.ExclusionAnswers, answers) {
		t.Errorf("event = %v %+v, want %v %+v", got.Kind, got.ExclusionAnswers, presentertest.KindExclusionAnswersRecorded, answers)
	}
	if got := presentertest.KindExclusionAnswersRecorded.String(); got != "ExclusionAnswersRecorded" {
		t.Errorf("kind string = %q, want ExclusionAnswersRecorded", got)
	}
}

var ignoredReleaseTableAnswers = presenter.ExclusionAnswers{Stage: "notes", File: ".mint.toml", Table: "[release]", Ignored: true}

func TestPlainPresenter_ExclusionAnswersRecorded_ForAnIgnoredFile_SaysTheyWereNotCommitted(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { p.ExclusionAnswersRecorded(ignoredReleaseTableAnswers) })

	want := "notes: answers written to .mint.toml's [release] table and not committed, as git ignores .mint.toml\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	assertBytePureASCIIStreams(t, "plain ignored exclusion answers narration", out, errBuf)
}

// TestRailPresenter_ExclusionAnswersRecorded_HangsWhereTheAnswersWent pins the row
// the Rail hangs under the check's node once the answers are written: committed, or
// left uncommitted where git ignores the file.
func TestRailPresenter_ExclusionAnswersRecorded_HangsWhereTheAnswersWent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answers presenter.ExclusionAnswers
		want    []string
	}{
		{
			name:    "committed",
			answers: releaseTableAnswers,
			want:    []string{" │                written to .mint.toml's [release] table and committed"},
		},
		{
			name:    "ignored",
			answers: ignoredReleaseTableAnswers,
			want: []string{
				" │                written to .mint.toml's [release] table and not committed, as git ignores",
				" │                .mint.toml",
			},
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
			r.NotesChecking(presenter.NotesCheck{Key: "check"})
			r.NotesChecked(presenter.NotesCheck{Key: "check", Detail: "1 flagged"})
			a := tt.answers
			a.Key = "check"
			r.SuspendSpinner()
			r.ExclusionAnswersRecorded(a)
			r.ResumeSpinner()

			screen := r.screen(t)
			if got := hungUnder(screen, " ●  check ", " ⠋  notes "); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rows under the check =\n%q\nwant\n%q\nscreen:\n%s", got, tt.want, strings.Join(screen, "\n"))
			}
		})
	}
}

// hungUnder is the rows of screen between the row that starts with node and the
// next that starts with next.
func hungUnder(screen []string, node, next string) []string {
	var rows []string
	in := false
	for _, l := range screen {
		switch {
		case strings.HasPrefix(l, node):
			in = true
		case strings.HasPrefix(l, next):
			return rows
		case in:
			rows = append(rows, l)
		}
	}
	return rows
}
