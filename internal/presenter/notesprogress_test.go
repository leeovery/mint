package presenter_test

import (
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
		Elapsed:  64200 * time.Millisecond,
		Blocking: true,
	})
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

func TestPlainPresenter_NotesProgress_WriteSizesInDecimalUnits(t *testing.T) {
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
		})
	}
}

// driveHalvedSinglePassStage plays a notes stage whose single-pass call is re-cut
// into three parts: the halving, each part finishing, the merge, then the completion.
func driveHalvedSinglePassStage(p presenter.Presenter) {
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
	p.NotesHalving(presenter.NotesHalving{Parts: 3})
	for done := 1; done <= 3; done++ {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 1_200_000, Parts: 3, Done: done})
	}
	p.NotesMerging(presenter.NotesMerge{Parts: 3})
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated in 3 parts, merged",
		Elapsed:  12300 * time.Millisecond,
		Blocking: true,
	})
}

// driveHalvedPartStage plays a six-part chunked notes stage whose part 4 is re-cut
// into three parts once two parts are done, taking the run to eight parts.
func driveHalvedPartStage(p presenter.Presenter) {
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true, Text: "generating release notes…"})
	for done := 0; done <= 2; done++ {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 6, Done: done})
	}
	p.NotesHalving(presenter.NotesHalving{Part: 4, Parts: 3})
	for done := 3; done <= 8; done++ {
		p.NotesProgress(presenter.NotesProgress{PromptBytes: 3_100_000, Parts: 8, Done: done})
	}
	p.NotesMerging(presenter.NotesMerge{Parts: 8})
	p.StageSucceeded(presenter.StageSuccess{
		Name:     "notes",
		Detail:   "generated in 8 parts, merged",
		Elapsed:  64200 * time.Millisecond,
		Blocking: true,
	})
}

func TestPlainPresenter_SinglePassHalved_PrintsTheRecutAndEachFinishedPartAsLines(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { driveHalvedSinglePassStage(p) })

	want := "notes: running...\n" +
		"notes: too big for the model in one call, retrying in 3 parts\n" +
		"notes: 1.2 MB release, in 3 parts, 1 of 3 done\n" +
		"notes: 1.2 MB release, in 3 parts, 2 of 3 done\n" +
		"notes: 1.2 MB release, in 3 parts, 3 of 3 done\n" +
		"notes: merging 3 parts\n" +
		"notes: generated in 3 parts, merged (12.3s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain halved single-pass narration", out, errBuf)
}

func TestPlainPresenter_PartHalved_PrintsTheRecutAsItsOwnLineAmongThePartLines(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { driveHalvedPartStage(p) })

	want := "notes: running...\n" +
		"notes: 3.1 MB release, in 6 parts, 0 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 1 of 6 done\n" +
		"notes: 3.1 MB release, in 6 parts, 2 of 6 done\n" +
		"notes: part 4 was too big for the model, retrying it as 3 smaller parts\n" +
		"notes: 3.1 MB release, in 8 parts, 3 of 8 done\n" +
		"notes: 3.1 MB release, in 8 parts, 4 of 8 done\n" +
		"notes: 3.1 MB release, in 8 parts, 5 of 8 done\n" +
		"notes: 3.1 MB release, in 8 parts, 6 of 8 done\n" +
		"notes: 3.1 MB release, in 8 parts, 7 of 8 done\n" +
		"notes: 3.1 MB release, in 8 parts, 8 of 8 done\n" +
		"notes: merging 8 parts\n" +
		"notes: generated in 8 parts, merged (64.2s)\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain halved part narration", out, errBuf)
}
