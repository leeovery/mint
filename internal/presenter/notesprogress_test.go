package presenter_test

import (
	"slices"
	"testing"
	"time"

	"mint/internal/presenter"
)

// driveChunkedNotesStage plays a six-part chunked notes stage on p: the split, each
// part finishing, the merge, then the completion line.
func driveChunkedNotesStage(p presenter.Presenter) {
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
	for done := 0; done <= 6; done++ {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: done})
	}
	p.NotesMerging(presenter.NotesMerge{Parts: 6})
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated in 6 parts, merged",
		Sentence: "Notes written (6 parts, merged)",
		Elapsed:  64200 * time.Millisecond,
		Blocking: true,
	})
}

func TestPrettyPresenter_ChunkedNotesStage_UpdatesTheOneNotesLine(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) { driveChunkedNotesStage(p) })

	if len(tr.created) != 1 {
		t.Fatalf("spinners created = %d, want the one notes line", len(tr.created))
	}
	want := []string{
		"Writing notes: 3.1 MB release, in 6 parts · 0 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 1 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 2 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 3 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 4 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 5 of 6 done",
		"Writing notes: 3.1 MB release, in 6 parts · 6 of 6 done",
		"Writing notes: merging 6 parts",
	}
	if got := tr.created[0].texts; !slices.Equal(got, want) {
		t.Errorf("notes line texts =\n%q\nwant\n%q", got, want)
	}
	if got, want := out.String(), "✓ Notes written (6 parts, merged) (64.2s)\n"; got != want {
		t.Errorf("output = %q, want only the completion line %q", got, want)
	}
}

func TestPrettyPresenter_NotesProgressAfterTheStageEnds_StartsNoSpinner(t *testing.T) {
	t.Parallel()

	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: 1})
		p.NotesMerging(presenter.NotesMerge{Parts: 6})
	})

	if len(tr.created) != 0 {
		t.Errorf("spinners created = %d, want none with no notes line running", len(tr.created))
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing", out.String())
	}
}

func TestPlainPresenter_ChunkedNotesStage_PrintsALinePerUpdate(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { driveChunkedNotesStage(p) })

	want := "notes: running...\n" +
		"notes: 3.1 MB release, in 6 parts, 0 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 1 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 2 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 3 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 4 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 5 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 6 of 6 done\n" +
		"notes: merging 6 parts\n" +
		"notes: generated in 6 parts, merged (64.2s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain chunked notes narration", out, errBuf)
}

func TestPresenters_NotesProgress_WriteSizesInDecimalUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int
		want  string
	}{
		{bytes: 1_500_000, want: "1.5 MB"},
		{bytes: 38_000_000, want: "38 MB"},
		{bytes: 3_100_000, want: "3.1 MB"},
		{bytes: 850_000, want: "850 KB"},
		{bytes: 640, want: "640 B"},
		{bytes: 999, want: "999 B"},
		{bytes: 1_000, want: "1.0 KB"},
		{bytes: 9_949, want: "9.9 KB"},
		{bytes: 9_950, want: "10 KB"},
		{bytes: 999_499, want: "999 KB"},
		{bytes: 999_500, want: "1.0 MB"},
		{bytes: 9_950_000, want: "10 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			progress := presenter.NotesProgress{PromptBytes: tt.bytes, Parts: 2, Done: 1}
			out, _ := drive(func(p *presenter.PlainPresenter) { p.NotesProgress(progress) })
			if got, want := out.String(), "notes: "+tt.want+" release, in 2 parts, 1 of 2 done\n"; got != want {
				t.Errorf("plain line = %q, want %q", got, want)
			}

			_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
				p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
				p.NotesProgress(progress)
			})
			want := []string{"Writing notes: " + tt.want + " release, in 2 parts · 1 of 2 done"}
			if got := tr.created[0].texts; !slices.Equal(got, want) {
				t.Errorf("pretty line = %q, want %q", got, want)
			}
		})
	}
}
