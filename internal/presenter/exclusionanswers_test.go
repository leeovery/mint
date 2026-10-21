package presenter_test

import (
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
	p.NotesChecking()
	p.NotesChecked()
	p.SuspendSpinner()
	p.ExclusionAnswersRecorded(releaseTableAnswers)
	p.ResumeSpinner()
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated",
		Sentence: "Generated release notes",
		Elapsed:  4200 * time.Millisecond,
		Blocking: true,
	})
}

func TestPrettyPresenter_ExclusionAnswersRecorded_PrintsOneLineBeforeTheNotesLineResumes(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) { driveAnsweredNotesStage(p) })

	want := "✓ Answers written to .mint.toml's [release] table and committed\n" +
		"✓ Generated release notes (4.2s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
	if len(tr.created) != 2 {
		t.Fatalf("spinners created = %d, want the notes line and its resumption", len(tr.created))
	}
	if !tr.created[0].stopped || !tr.created[1].started {
		t.Errorf("the notes line did not stop for the answers line and resume after it")
	}
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
	rec.ExclusionAnswersRecorded(releaseTableAnswers)

	if len(rec.Events) != 1 {
		t.Fatalf("events = %d, want one", len(rec.Events))
	}
	if got := rec.Events[0]; got.Kind != presentertest.KindExclusionAnswersRecorded || got.ExclusionAnswers != releaseTableAnswers {
		t.Errorf("event = %v %+v, want %v %+v", got.Kind, got.ExclusionAnswers, presentertest.KindExclusionAnswersRecorded, releaseTableAnswers)
	}
	if got := presentertest.KindExclusionAnswersRecorded.String(); got != "ExclusionAnswersRecorded" {
		t.Errorf("kind string = %q, want ExclusionAnswersRecorded", got)
	}
}

var ignoredReleaseTableAnswers = presenter.ExclusionAnswers{Stage: "notes", File: ".mint.toml", Table: "[release]", Ignored: true}

func TestPrettyPresenter_ExclusionAnswersRecorded_ForAnIgnoredFile_SaysTheyWereNotCommitted(t *testing.T) {
	t.Parallel()

	out, _ := drivePrettySpy(t, func(p *presenter.PrettyPresenter) { p.ExclusionAnswersRecorded(ignoredReleaseTableAnswers) })

	want := "✓ Answers written to .mint.toml's [release] table and not committed, as git ignores .mint.toml\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}

func TestPlainPresenter_ExclusionAnswersRecorded_ForAnIgnoredFile_SaysTheyWereNotCommitted(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { p.ExclusionAnswersRecorded(ignoredReleaseTableAnswers) })

	want := "notes: answers written to .mint.toml's [release] table and not committed, as git ignores .mint.toml\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	assertBytePureASCIIStreams(t, "plain ignored exclusion answers narration", out, errBuf)
}
