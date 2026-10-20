package engine_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

func dryRunSavedFallbackWarning(phrase, output, path string) presenter.Warning {
	return presenter.Warning{
		Label: "notes",
		Message: phrase + "; falling back to the commit-subject list; part notes saved to " + path +
			" — release them with mint release --notes-file " + path,
		Output: output,
	}
}

// assertPreviewedTheCommitSubjectList fails unless the dry run finished without a
// ✗ line or any question, having shown the commit-subject list as its notes.
func assertPreviewedTheCommitSubjectList(t *testing.T, rec *presentertest.RecordingPresenter, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}
	if gates := promptGates(rec); len(gates) != 0 {
		t.Errorf("a dry run asked: %+v", gates)
	}
	if n := indexOfKind(rec, presentertest.KindStageFailed); n != -1 {
		t.Errorf("a ✗ line rendered; kinds = %v", rec.Kinds())
	}
	if got := showNotesBody(t, rec); got != commitSubjects {
		t.Errorf("previewed notes = %q, want the commit-subject list %q", got, commitSubjects)
	}
	if n := indexOfKind(rec, presentertest.KindRunFinished); n == -1 || !rec.Events[n].RunFinished.DryRun {
		t.Errorf("kinds = %v, want the run to close as a dry run", rec.Kinds())
	}
}

func TestRelease_DryRunFallback_PreviewsTheCommitSubjectListWithTheWarningAndAsksNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		yes              bool
		stdinInteractive bool
	}{
		{name: "at a terminal without -y", stdinInteractive: true},
		{name: "-y at a terminal", yes: true, stdinInteractive: true},
		{name: "stdin not a terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, fallbackConfig)
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput + "\n", ExitCode: 1})
			f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
			deps.Yes = tc.yes
			deps.StdinInteractive = tc.stdinInteractive

			err := engine.Release(t.Context(), deps, dryRunOptions())

			assertPreviewedTheCommitSubjectList(t, rec, err)
			assertOnlyWarning(t, rec, fallbackWarning("AI call failed", loginOutput))
			assertWarnedBetweenTheNotesAndTheirShowing(t, rec)
			assertNoMutation(t, f)
		})
	}
}

func TestRelease_DryRunFallback_ChunkedFailureSavesTheChunkNotesAndNamesOnlyTheNotesFileRelease(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "fallback", twoPartTransport(failsSecondPart))
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	run.deps.StdinInteractive = true

	err := engine.Release(t.Context(), run.deps, dryRunOptions())

	assertPreviewedTheCommitSubjectList(t, run.rec, err)
	path := run.savedPath(nextTag)
	if got := readSaved(t, path); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want part 1's notes %q", got, partOneSaved)
	}
	assertOnlyWarning(t, run.rec, dryRunSavedFallbackWarning("AI call failed (part 2 of 2)", usageLimitOutput, path))
	for _, w := range warnings(run.rec) {
		if strings.Contains(w.Message, "regenerate") {
			t.Errorf("warning %q names regenerate, want only the --notes-file release", w.Message)
		}
	}
	assertNoMutation(t, run.f)
}

func TestRelease_DryRunFallback_UnsavedChunkNotesNameNoFileAndNoCommand(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "fallback", twoPartTransport(failsSecondPart))
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}
	run.deps.Yes = true

	err := engine.Release(t.Context(), run.deps, dryRunOptions())

	assertPreviewedTheCommitSubjectList(t, run.rec, err)
	want := []presenter.Warning{
		unsavedChunkNotesWarning(errChunkNotesWrite.Error()),
		fallbackWarning("AI call failed (part 2 of 2)", usageLimitOutput),
	}
	if got := warnings(run.rec); !slices.Equal(got, want) {
		t.Errorf("warnings = %+v, want %+v", got, want)
	}
}
