package presenter_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

const savedNotesPath = "/Users/lee/Library/Caches/mint/chunk-notes/-Users-lee-Code-app/v1.4.0.md"

func notesFailureWithSavedNotes() presenter.StageFailure {
	return presenter.StageFailure{
		Name:       "notes",
		Message:    "AI call failed (part 3 of 6)",
		Output:     "API Error: usage limit reached for this account",
		SavedNotes: savedNotesPath,
	}
}

func TestPlainPresenterStageFailed_SavedNotes_NamesThePathAfterTheOutput(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) {
		p.StageFailed(notesFailureWithSavedNotes())
	})

	want := "notes: FAILED - AI call failed (part 3 of 6)\n" +
		"--- output ---\n" +
		"API Error: usage limit reached for this account\n" +
		"--- end output ---\n" +
		"notes: part notes saved to " + savedNotesPath + "\n"
	if got := out.String(); got != want {
		t.Errorf("StageFailed out\n got: %q\nwant: %q", got, want)
	}
	if got, want := errBuf.String(), "notes: FAILED - AI call failed (part 3 of 6)\n"; got != want {
		t.Errorf("stderr = %q, want only the one-line summary %q", got, want)
	}
}

func TestPrettyPresenterStageFailed_SavedNotes_NamesThePathAfterTheOutput(t *testing.T) {
	t.Parallel()

	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.Ascii))
	p.StageFailed(notesFailureWithSavedNotes())

	want := "✗ notes      AI call failed (part 3 of 6)\n" +
		"API Error: usage limit reached for this account\n" +
		"part notes saved to " + savedNotesPath + "\n"
	if got := out.String(); got != want {
		t.Errorf("StageFailed out\n got: %q\nwant: %q", got, want)
	}
	if got, want := errBuf.String(), "✗ notes  AI call failed (part 3 of 6)\n"; got != want {
		t.Errorf("stderr = %q, want only the one-line summary %q", got, want)
	}
}

func TestStageFailed_NoSavedNotes_NamesNoPath(t *testing.T) {
	t.Parallel()

	failure := notesFailureWithSavedNotes()
	failure.SavedNotes = ""

	plain, _ := drive(func(p *presenter.PlainPresenter) { p.StageFailed(failure) })
	if want := "notes: FAILED - AI call failed (part 3 of 6)\n--- output ---\nAPI Error: usage limit reached for this account\n--- end output ---\n"; plain.String() != want {
		t.Errorf("plain StageFailed = %q, want %q", plain.String(), want)
	}
	pretty := drivePretty(termenv.Ascii, func(p *presenter.PrettyPresenter) { p.StageFailed(failure) })
	if want := "✗ notes      AI call failed (part 3 of 6)\nAPI Error: usage limit reached for this account\n"; pretty.String() != want {
		t.Errorf("pretty StageFailed = %q, want %q", pretty.String(), want)
	}
}
