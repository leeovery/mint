package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// assertNoCheck fails if the run made the exclusion check's call or showed its line.
func assertNoCheck(t *testing.T, f *runner.FakeRunner, rec *presentertest.RecordingPresenter) {
	t.Helper()
	if got := checkInvocations(f, "claude"); len(got) != 0 {
		t.Errorf("check calls = %d, want none", len(got))
	}
	if got := countKind(rec, presentertest.KindNotesChecking) + countKind(rec, presentertest.KindNotesChecked); got != 0 {
		t.Errorf("check narrations = %d, want none: %v", got, rec.Kinds())
	}
}

func TestRelease_NoAI_MakesNoCheckCallAndShipsTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	const subjects = "Add login flow\nFix token refresh\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(priorTagDiff), // diff priorTag..HEAD
		ScriptedOut(subjects),     // log --format=%s priorTag..HEAD
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	opts := priorTagNormalAIOptions()
	opts.NoAI = true

	if err := engine.Release(t.Context(), newDeps(rec, f), opts); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNoCheck(t, f, rec)
	if claudeInvoked(f) {
		t.Errorf("claude was invoked under --no-ai")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != subjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, subjects)
	}
}

func TestRelease_NotesFile_MakesNoCheckCallAndShipsTheSuppliedBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\ndiff_keep = [\"docs/api/\"]\n")
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNoCheck(t, f, rec)
	assertNoAIWork(t, f)
	assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
}

func TestRelease_FirstRelease_MakesNoCheckCallAndShipsInitialRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNoCheck(t, f, rec)
	if claudeInvoked(f) {
		t.Errorf("claude was invoked on a first release")
	}
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != terminatedAnnotationBody("Initial release.") {
		t.Errorf("tag annotation body = %q, want %q", got, terminatedAnnotationBody("Initial release."))
	}
}

func TestRelease_RangeChangingOnlyExcludedPaths_MakesNoCheckCallAndShipsTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    string
		pathspecs []string
	}{
		{
			name:      "CHANGELOG.md and .mint.toml",
			pathspecs: []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml"},
		},
		{
			name:      "shared diff_exclude",
			config:    "diff_exclude = [\"*.lock\"]\n",
			pathspecs: []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)*.lock"},
		},
		{
			name:      "[release] diff_exclude",
			config:    "[release]\ndiff_exclude = [\"specs/\"]\n",
			pathspecs: []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)specs/"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			f.SeedSequence("git", ScriptedOut("")) // diff priorTag..HEAD: every change excluded
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			args := append([]string{"diff", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", "--no-ext-diff", priorTag + "..HEAD", "--", "."}, tt.pathspecs...)
			if !invokedWith(f, "git", args...) {
				t.Errorf("no `git %s` call; got %v", strings.Join(args, " "), commandLines(f.Invocations()))
			}
			assertNoCheck(t, f, rec)
			if claudeInvoked(f) {
				t.Errorf("claude was invoked for a range whose every change is excluded")
			}
			const want = "Maintenance release — no notable source changes"
			if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(want) {
				t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, terminatedAnnotationBody(want))
			}
		})
	}
}

func TestRelease_EveryChangedPathKeptOrExcluded_MakesNoCheckCallAndWritesTheNotesFromTheKeptPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "diff_exclude = [\"*.lock\"]\n\n[release]\ndiff_keep = [\"docs/api/\"]\n")
	const keptDiff = "diff --git a/docs/api/export.md b/docs/api/export.md\n@@ -0,0 +1 @@\n+# Export API\n"
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// git leaves app.lock out of the notes diff, so it holds the kept path alone.
	f.SeedSequence("git",
		ScriptedOut(keptDiff),
		ScriptedOut("A\tdocs/api/export.md\n"),
		ScriptedOut("1\t0\tdocs/api/export.md\n"),
	)
	f.SeedSequence("claude", ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)*.lock")
	assertNoCheck(t, f, rec)
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one", len(notesCalls))
	}
	if !strings.Contains(notesCalls[0].Stdin, keptDiff) {
		t.Errorf("the notes call does not carry docs/api/export.md's diff:\n%s", notesCalls[0].Stdin)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
	}
}
