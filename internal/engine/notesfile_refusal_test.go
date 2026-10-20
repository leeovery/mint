package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// notesFileRefusal is one --notes-file that cannot be used, with the line it is refused with.
type notesFileRefusal struct {
	name    string
	path    string
	wantMsg string
}

func readErrorText(t *testing.T, path string) string {
	t.Helper()
	_, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("reading %q succeeded; the case needs a path mint cannot read", path)
	}
	return err.Error()
}

func notesFileRefusals(t *testing.T) []notesFileRefusal {
	t.Helper()
	dir := t.TempDir()
	empty := writeNotesFile(t, dir, "empty.md", "")
	blank := writeNotesFile(t, dir, "blank.md", " \n\t\r\n  \n")
	missing := filepath.Join(dir, "missing.md")
	unreadable := filepath.Join(dir, "a-directory")
	if err := os.Mkdir(unreadable, 0o755); err != nil {
		t.Fatalf("creating %s: %v", unreadable, err)
	}
	return []notesFileRefusal{
		{name: "an empty file", path: empty, wantMsg: "--notes-file " + empty + " is empty"},
		{name: "a whitespace-only file", path: blank, wantMsg: "--notes-file " + blank + " is empty"},
		{name: "a missing file", path: missing, wantMsg: "reading --notes-file " + missing + ": " + readErrorText(t, missing)},
		{name: "an unreadable file", path: unreadable, wantMsg: "reading --notes-file " + unreadable + ": " + readErrorText(t, unreadable)},
		{name: "an empty path", path: "", wantMsg: "reading --notes-file : " + readErrorText(t, "")},
	}
}

// assertRefusedBeforeAnyStep fails unless the run ended with exit 1 and the one
// refusal line, having run no command and shown nothing else.
func assertRefusedBeforeAnyStep(t *testing.T, err error, rec *presentertest.RecordingPresenter, f *runner.FakeRunner, wantMsg string) {
	t.Helper()
	if got := f.Invocations(); len(got) != 0 {
		t.Errorf("commands ran = %v, want none", commandLines(got))
	}
	assertNotesRefusal(t, err, rec, wantMsg)
}

// assertNotesRefusal fails unless err is an exit-1 abort and the presenter showed
// only the one notes-stage refusal line.
func assertNotesRefusal(t *testing.T, err error, rec *presentertest.RecordingPresenter, wantMsg string) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("error = %v, want an *engine.AbortError", err)
	}
	if abort.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", abort.ExitCode)
	}
	if len(rec.Events) != 1 || rec.Events[0].Kind != presentertest.KindStageFailed {
		t.Fatalf("presenter events = %+v, want exactly one StageFailed", rec.Events)
	}
	want := presenter.StageFailure{Name: "notes", Message: wantMsg}
	if got := rec.Events[0].StageFailed; got != want {
		t.Errorf("StageFailed = %+v, want %+v", got, want)
	}
}

func TestRelease_UnusableNotesFile_IsRefusedBeforeAnyStep(t *testing.T) {
	t.Parallel()

	for _, tt := range notesFileRefusals(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			rec := &presentertest.RecordingPresenter{}
			opts := notesFileOptions(tt.path)
			opts.NotesFileSet = true
			opts.AutoStash = true

			err := engine.Release(t.Context(), newDeps(rec, f), opts)

			assertRefusedBeforeAnyStep(t, err, rec, f, tt.wantMsg)
		})
	}
}

func TestReadNotesFile_UnusableNotesFile_IsRefusedOnTheNotesStage(t *testing.T) {
	t.Parallel()

	for _, tt := range notesFileRefusals(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}

			body, err := engine.ReadNotesFile(rec, tt.path, true)

			if body != nil {
				t.Errorf("body = %q, want none for an unusable file", *body)
			}
			assertNotesRefusal(t, err, rec, tt.wantMsg)
		})
	}
}

func TestReadNotesFile_FlagNotGiven_ReadsNothingAndShowsNothing(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}

	body, err := engine.ReadNotesFile(rec, "", false)

	if err != nil {
		t.Fatalf("ReadNotesFile returned error %v, want nil when --notes-file is not given", err)
	}
	if body != nil {
		t.Errorf("body = %q, want none when --notes-file is not given", *body)
	}
	if len(rec.Events) != 0 {
		t.Errorf("presenter events = %+v, want none", rec.Events)
	}
}

func TestReadNotesFile_UsableFile_ReturnsItsBodyByteIdentical(t *testing.T) {
	t.Parallel()

	const raw = "## Notes\n\n- first\r\n- second  \n\n"
	path := writeNotesFile(t, t.TempDir(), "notes.md", raw)
	rec := &presentertest.RecordingPresenter{}

	body, err := engine.ReadNotesFile(rec, path, true)

	if err != nil {
		t.Fatalf("ReadNotesFile returned error: %v", err)
	}
	if body == nil || *body != raw {
		t.Errorf("body = %v, want %q", body, raw)
	}
	if len(rec.Events) != 0 {
		t.Errorf("presenter events = %+v, want none", rec.Events)
	}
}
