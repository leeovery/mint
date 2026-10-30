package presenter_test

import (
	"strings"
	"testing"

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

func TestStageFailed_NoSavedNotes_NamesNoPath(t *testing.T) {
	t.Parallel()

	failure := notesFailureWithSavedNotes()
	failure.SavedNotes = ""

	plain, _ := drive(func(p *presenter.PlainPresenter) { p.StageFailed(failure) })
	if want := "notes: FAILED - AI call failed (part 3 of 6)\n--- output ---\nAPI Error: usage limit reached for this account\n--- end output ---\n"; plain.String() != want {
		t.Errorf("plain StageFailed = %q, want %q", plain.String(), want)
	}
	r := newRailTerm(t, 100, 24, answers())
	r.StageFailed(failure)
	if screen := strings.Join(r.screen(t), "\n"); strings.Contains(screen, "part notes saved") {
		t.Errorf("the rail names a saved notes path where there is none:\n%s", screen)
	}
}
