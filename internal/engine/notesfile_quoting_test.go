package engine_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// spacedRoot is a repo root whose path holds a space, as ~/Code/My App does, so the
// chunk notes saved for it land at a path holding one too.
func spacedRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "My App")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("creating the repo root: %v", err)
	}
	return root
}

// assertSpaced fails unless path holds a space and no `'`, so wrapping it in single
// quotes is its whole quoting.
func assertSpaced(t *testing.T, path string) {
	t.Helper()
	if !strings.Contains(path, " ") || strings.Contains(path, "'") {
		t.Fatalf("saved path %q, want one holding a space and no single quote", path)
	}
}

func TestRelease_NotesFloorNextMoves_SavedPathWithASpaceIsQuotedInTheCommandOnly(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRunAt(t, spacedRoot(t), "abort", twoPartTransport(failsMerge))
	run.deps.Yes = true

	err := run.release(t, t.Context())

	path := run.savedPath(nextTag)
	assertSpaced(t, path)
	tidy := "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with mint release --notes-file '" + path + "'"
	assertStopsWithNextMoves(t, run.rec, err, []string{tidy, noAIMove, fallbackMove})
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the raw path %q", sf.SavedNotes, path)
	}
}

func TestRegenerateRun_NotesFloorNextMoves_SavedPathWithASpaceIsQuotedInTheCommandOnly(t *testing.T) {
	t.Parallel()

	run := newUnattendedRegenFloor(t, true, true)
	run.root = spacedRoot(t)

	err := run.regenerateUnattended(t.Context(), freshCfg(), overCapFailing(failsMerge))

	path := run.savedPath(regenFloorTag)
	assertSpaced(t, path)
	tidy := "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with mint release regenerate " +
		regenFloorVersion + " --target release --notes-file '" + path + "'"
	assertRegenStopsWithNextMoves(t, run, err, []string{tidy})
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the raw path %q", sf.SavedNotes, path)
	}
}

func TestRelease_Fallback_SavedPathWithASpaceIsQuotedInBothCommandsOnly(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRunAt(t, spacedRoot(t), "fallback", twoPartTransport(failsSecondPart))
	run.deps.Yes = true
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)

	if err := run.release(t, t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	path := run.savedPath(nextTag)
	assertSpaced(t, path)
	assertOnlyWarning(t, run.rec, presenter.Warning{
		Label: "notes",
		Message: "AI call failed (part 2 of 2); falling back to the commit-subject list; part notes saved to " + path +
			" — once this release is out, put them on it with mint release regenerate " + nextVersion + " --notes-file '" + path +
			"', or if the run ends before the push, release them with mint release --notes-file '" + path + "'",
		Output: usageLimitOutput,
	})
}

func TestRelease_DryRunFallback_SavedPathWithASpaceIsQuotedInTheCommandOnly(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRunAt(t, spacedRoot(t), "fallback", twoPartTransport(failsSecondPart))
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	run.deps.StdinInteractive = true

	err := engine.Release(t.Context(), run.deps, dryRunOptions())

	assertPreviewedTheCommitSubjectList(t, run.rec, err)
	path := run.savedPath(nextTag)
	assertSpaced(t, path)
	assertOnlyWarning(t, run.rec, presenter.Warning{
		Label: "notes",
		Message: "AI call failed (part 2 of 2); falling back to the commit-subject list; part notes saved to " + path +
			" — release them with mint release --dry-run --notes-file '" + path + "'",
		Output: usageLimitOutput,
	})
}

func TestRegenerateAll_SavedPathWithASpaceIsQuotedInTheSkipsCommandOnly(t *testing.T) {
	t.Parallel()

	run := newRegenChunkNotes(t)
	run.root = spacedRoot(t)

	run.regenerateAll(t, overCapFailing(failsSecondPart), "v1.1.0")

	path := run.savedPath("v1.1.0")
	assertSpaced(t, path)
	cause := "AI call failed (part 2 of 2); part notes saved to " + path +
		" — tidy them and put them on it with mint release regenerate 1.1.0 --target release --notes-file '" + path + "'"
	wantWarns := []presenter.Warning{{Label: "skipped v1.1.0", Message: cause, Output: usageLimitOutput}}
	if got := plainWarnings(run.rec); !slices.Equal(got, wantWarns) {
		t.Errorf("warnings = %+v, want %+v", got, wantWarns)
	}
	want := "2 regenerated, 1 skipped: v1.1.0 (" + cause + ")"
	if got := finishEvent(t, run.rec).Summary; got != want {
		t.Errorf("RunFinished.Summary = %q, want %q", got, want)
	}
}
