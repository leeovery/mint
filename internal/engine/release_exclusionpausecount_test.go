package engine_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

func TestRelease_APause_ShowsHowManyFilesTheNotesCarryTheirSizeAndTheKeptOnes(t *testing.T) {
	t.Parallel()

	const config = "[release]\n" +
		"diff_keep = [\"docs/api.md\"]\n" +
		"diff_exclude = [\"docs/old/\"]\n" +
		"version_file = \"docs/VERSION\"\n"
	root := t.TempDir()
	writeConfig(t, root, config)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// git leaves docs/old/c.md and the plain-mode version_file out of the notes diff.
	diff := fileDiffOfSize(t, "docs/api.md", 2_000) +
		fileDiffOfSize(t, "docs/design/a.md", 12_000) +
		fileDiffOfSize(t, "docs/design/b.md", 3_000)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", withAnswersCommit(
		ScriptedOut(diff),
		ScriptedOut("M\tdocs/api.md\nM\tdocs/design/a.md\nM\tdocs/design/b.md\n"),
		ScriptedOut("1\t1\tdocs/api.md\n1\t1\tdocs/design/a.md\n1\t1\tdocs/design/b.md\n"),
	)...)
	f.SeedSequence("claude", ScriptedOut("docs/: documentation\n"), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD",
		":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)docs/old/", ":(exclude)docs/VERSION")
	want := []string{"Exclude docs/ from the notes? (documentation; 3 files, 17 KB; on the keep list: docs/api.md)"}
	if got := pauseQuestions(pauseGates(rec)); !slices.Equal(got, want) {
		t.Errorf("pauses = %q, want %q", got, want)
	}
}

func TestRelease_APauseOnOneFile_CountsItInTheSingular(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := pausedRelease(t, rec, "", "docs/design/: design documents\n"); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{"Exclude docs/design/ from the notes? (design documents; 1 file, 176 B)"}
	if got := pauseQuestions(pauseGates(rec)); !slices.Equal(got, want) {
		t.Errorf("pauses = %q, want %q", got, want)
	}
}
