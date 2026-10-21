package engine_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// docsAndExportDiff is the notes diff for a range changing a doc under docs/api/ and a
// source file.
const docsAndExportDiff = "diff --git a/docs/api/export.md b/docs/api/export.md\n@@ -0,0 +1 @@\n+# Export\n" +
	"diff --git a/src/export.go b/src/export.go\n@@ -0,0 +1 @@\n+package src\n"

// notesCallsUnder runs a release under config, git answering the notes diff and both
// Change Map calls with the scripted outputs, and returns the notes-side `git diff`
// command lines plus the prompt the notes call carried.
func notesCallsUnder(t *testing.T, config string, notes ...runner.ScriptedCall) ([]string, string) {
	t.Helper()

	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", notes...)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	var diffs []string
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "diff" && slices.Contains(inv.Args, priorTag+"..HEAD") {
			diffs = append(diffs, commandLine(inv))
		}
	}
	return diffs, stdinOf(t, f, "claude", leanClaudeArgs...)
}

func TestRelease_ReleaseDiffKeep_LeavesTheNotesDiffAndChangeMapUnchanged(t *testing.T) {
	t.Parallel()

	notes := []runner.ScriptedCall{
		ScriptedOut(docsAndExportDiff),
		ScriptedOut("A\tdocs/api/export.md\nA\tsrc/export.go\n"),
		ScriptedOut("1\t0\tdocs/api/export.md\n12\t0\tsrc/export.go\n"),
	}
	baseDiffs, basePrompt := notesCallsUnder(t, "[release]\n", notes...)
	keepDiffs, keepPrompt := notesCallsUnder(t, "[release]\ndiff_keep = [\"docs/api/\"]\n", notes...)

	if !slices.Equal(keepDiffs, baseDiffs) {
		t.Errorf("notes git calls with diff_keep = %v, want the same as with none %v", keepDiffs, baseDiffs)
	}
	if keepPrompt != basePrompt {
		t.Errorf("notes prompt with diff_keep differs from the one with none:\n%s\nwant:\n%s", keepPrompt, basePrompt)
	}
	if len(baseDiffs) != 3 {
		t.Fatalf("want the notes diff and both Change Map calls, got %v", baseDiffs)
	}
	for _, want := range []string{"docs/api/export.md", "src/export.go"} {
		if !strings.Contains(keepPrompt, want) {
			t.Errorf("notes call does not carry %q; got:\n%s", want, keepPrompt)
		}
	}
}

func TestRelease_ReleaseDiffKeep_NeverOverridesAnExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "release exclusion", config: "[release]\ndiff_exclude = [\"docs/\"]\ndiff_keep = [\"docs/api/\"]\n"},
		{name: "shared exclusion", config: "diff_exclude = [\"docs/\"]\n\n[release]\ndiff_keep = [\"docs/api/\"]\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedExportNotes(f)
			f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)docs/")
			prompt := stdinOf(t, f, "claude", leanClaudeArgs...)
			if strings.Contains(prompt, "docs/api/export.md") {
				t.Errorf("notes call names the excluded, kept path docs/api/export.md; got:\n%s", prompt)
			}
		})
	}
}
