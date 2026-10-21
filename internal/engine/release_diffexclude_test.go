package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// exportDiff is the post-exclusion diff git returns once the lockfile and the spec are
// excluded: src/export.go alone.
const exportDiff = "diff --git a/src/export.go b/src/export.go\n@@ -0,0 +1 @@\n+package src\n"

func seedExportNotes(f *runner.FakeRunner) {
	f.SeedSequence("git",
		ScriptedOut(exportDiff),               // diff priorTag..HEAD (post-exclusion)
		ScriptedOut("A\tsrc/export.go\n"),     // diff --name-status (change map)
		ScriptedOut("12\t0\tsrc/export.go\n"), // diff --numstat (change map)
	)
}

// assertNotesGitCallsExclude fails unless the notes diff and both Change Map git calls
// carry exactly pathspecs after `-- .`.
func assertNotesGitCallsExclude(t *testing.T, f *runner.FakeRunner, diffRange string, pathspecs ...string) {
	t.Helper()
	tail := append([]string{diffRange, "--", "."}, pathspecs...)
	for _, head := range [][]string{
		{"diff", "--src-prefix=a/", "--dst-prefix=b/"},
		{"diff", "--name-status"},
		{"diff", "--numstat"},
	} {
		args := append(append([]string{}, head...), tail...)
		if !invokedWith(f, "git", args...) {
			t.Errorf("no `git %s` call; got %v", strings.Join(args, " "), commandLines(f.Invocations()))
		}
	}
}

func TestRelease_ReleaseDiffExclude_AddsToTheSharedListOnTheNotesDiffAndChangeMap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "diff_exclude = [\"*.lock\"]\n\n[release]\ndiff_exclude = [\"specs/\"]\n")
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

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude)*.lock", ":(exclude)specs/")
	prompt := stdinOf(t, f, "claude", leanClaudeArgs...)
	if !strings.Contains(prompt, exportDiff) {
		t.Errorf("notes call does not carry the src/export.go diff; got:\n%s", prompt)
	}
	for _, excluded := range []string{"app.lock", "specs/export.md"} {
		if strings.Contains(prompt, excluded) {
			t.Errorf("notes call names excluded path %q; got:\n%s", excluded, prompt)
		}
	}
}

func TestRelease_ReleaseDiffExclude_AppliesWithNoSharedList(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\ndiff_exclude = [\"specs/\"]\n")
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

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude)specs/")
}

func TestRelease_ReleaseDiffExclude_OnlyExcludedChangesGiveTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\ndiff_exclude = [\"specs/\"]\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut("")) // diff priorTag..HEAD: every change is under specs/
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "diff", "--src-prefix=a/", "--dst-prefix=b/", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)specs/") {
		t.Errorf("notes diff does not exclude specs/; got %v", commandLines(f.Invocations()))
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("claude was invoked for a range whose every change is excluded: %q", commandLine(inv))
		}
	}
	const want = "Maintenance release — no notable source changes"
	if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(want) {
		t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, terminatedAnnotationBody(want))
	}
}
