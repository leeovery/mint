package presenter_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

func notesNextMoves() presenter.NextMoves {
	return presenter.NextMoves{
		Stage: "notes",
		Moves: []presenter.NextMove{
			{Lead: "write the notes to a file and release with", Command: "mint release --notes-file <file>"},
			{Lead: "re-run with --no-ai to release with the commit-subject list"},
		},
	}
}

func TestPlainPresenterNextMoves_PrintsEachMoveOnItsOwnLine(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { p.NextMoves(notesNextMoves()) })

	want := "notes: NEXT - write the notes to a file and release with mint release --notes-file <file>\n" +
		"notes: NEXT - re-run with --no-ai to release with the commit-subject list\n"
	if got := out.String(); got != want {
		t.Errorf("NextMoves out\n got: %q\nwant: %q", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain next moves", out, errBuf)
}

func TestPlainPresenterNextMoves_AMoveWithASentenceOfItsOwnPrintsThatSentence(t *testing.T) {
	t.Parallel()

	move := presenter.NextMove{
		Lead:     "Ship that list whenever the AI fails, under [release] in .mint.toml:",
		Command:  `on_notes_failure = "fallback"`,
		Sentence: `set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`,
	}
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.NextMoves(presenter.NextMoves{Stage: "notes", Moves: []presenter.NextMove{move}})
	})

	if want := "notes: NEXT - " + move.Sentence + "\n"; out.String() != want {
		t.Errorf("NextMoves out = %q, want the move's own sentence %q", out.String(), want)
	}
}

func TestNextMoves_NoMovesPrintNothing(t *testing.T) {
	t.Parallel()

	empty := presenter.NextMoves{Stage: "notes"}
	plain, _ := drive(func(p *presenter.PlainPresenter) { p.NextMoves(empty) })
	if plain.Len() != 0 {
		t.Errorf("plain NextMoves = %q, want nothing", plain.String())
	}
	rail, railOut, _ := railGate(termenv.Ascii, nil, gateOpts{})
	rail.NextMoves(empty)
	rail.Close()
	if railOut.Len() != 0 {
		t.Errorf("rail NextMoves = %q, want nothing", railOut.String())
	}
}

func TestNextMoves_LeaveTheSuccessLineStanding(t *testing.T) {
	t.Parallel()

	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.NextMoves(notesNextMoves())
		p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0"})
	})

	if want := "done: acme v1.4.0\n"; !bytes.HasSuffix(out.Bytes(), []byte(want)) {
		t.Errorf("plain output = %q, want it to end %q", out.String(), want)
	}
}
