package engine_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// lsTree is the answers commit's read of rev's files.
func lsTree(rev string) []string {
	return []string{"ls-tree", "-r", "-z", "--name-only", "--full-tree", rev}
}

// treeReads counts the answers commit's ls-tree reads.
func treeReads(f *runner.FakeRunner) int {
	n := 0
	for _, inv := range invocationsNamed(f, "git") {
		if len(inv.Args) > 0 && inv.Args[0] == "ls-tree" {
			n++
		}
	}
	return n
}

// notesCallsAskingNothing is git's answers for a release changing src/export.go alone
// whose check asks nothing: the notes diff, then the Change Map's reads.
func notesCallsAskingNothing() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut(exportGoDiff),
		ScriptedOut("M\tsrc/export.go\n"),
		ScriptedOut("1\t1\tsrc/export.go\n"),
	}
}

// droppingRelease runs a release under config, git listing tree as HEAD's and the
// prior tag's files, answering the notes calls with gitCalls in turn, and claude the
// check with checkAnswer and then the notes with aiBody.
func droppingRelease(t *testing.T, deps func(*presentertest.RecordingPresenter, *runner.FakeRunner) engine.ReleaseDeps, rec *presentertest.RecordingPresenter, cfg string, tree []string, gitCalls []runner.ScriptedCall, checkAnswer string) (*runner.FakeRunner, string, error) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, cfg)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedTrees(f, tree...)
	f.SeedSequence("git", gitCalls...)
	f.SeedSequence("claude", ScriptedOut(checkAnswer), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	return f, root, engine.Release(t.Context(), deps(rec, f), priorTagNormalAIOptions())
}

func readConfigFile(t *testing.T, root string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("reading .mint.toml: %v", err)
	}
	return string(got)
}

func answersPayload(t *testing.T, rec *presentertest.RecordingPresenter) presenter.ExclusionAnswers {
	t.Helper()
	for _, e := range rec.Events {
		if e.Kind == presentertest.KindExclusionAnswersRecorded {
			return e.ExclusionAnswers
		}
	}
	t.Fatal("no answers line was narrated")
	return presenter.ExclusionAnswers{}
}

func TestRelease_ACheckAskingNothing_DropsTheReleaseEntriesMatchingNoFileInTheAnswersCommit(t *testing.T) {
	t.Parallel()

	const cfg = "# kept for the CI\ndiff_exclude = [\".github/\"]\n\n" +
		"[release]\ndiff_exclude = [\"specs/\", \".github/workflows/\"]\ndiff_keep = [\"docs/api/\", \".goreleaser.yaml\"]\n"
	rec := answeringGates(nil, nil)
	f, root, err := droppingRelease(t, attendedDeps, rec, cfg,
		[]string{"src/export.go", "specs/a.md", "docs/api/index.md"},
		withAnswersCommit(notesCallsAskingNothing()...), "none")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := "# kept for the CI\ndiff_exclude = [\".github/\"]\n\n" +
		"[release]\ndiff_exclude = [\"specs/\"]\ndiff_keep = [\"docs/api/\"]\n"
	if got := readConfigFile(t, root); got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	if got := commitSubjectsIn(f, root); !slices.Contains(got, answersSubject(nextTag)) {
		t.Errorf("commits = %q, want the answers commit among them", got)
	}
	if got := pauseGates(rec); len(got) != 0 {
		t.Errorf("the run paused: %v", pauseQuestions(got))
	}
	wantPayload := presenter.ExclusionAnswers{Stage: "notes", Key: "check", File: ".mint.toml", Table: "[release]",
		Dropped: []string{".github/workflows/", ".goreleaser.yaml"}}
	if got := answersPayload(t, rec); !reflect.DeepEqual(got, wantPayload) {
		t.Errorf("answers payload = %+v, want %+v", got, wantPayload)
	}
}

func TestRelease_AnswersAndDroppedEntries_GoInOneAnswersCommit(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	f, root, err := droppingRelease(t, attendedDeps, rec, "[release]\ndiff_exclude = [\"old/\"]\n",
		[]string{"src/export.go", "docs/design/a.md", "test/fixtures/export.json"},
		excludeThenKeepCalls, designAndFixturesAnswer)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got, want := readConfigFile(t, root), "[release]\ndiff_exclude = [\"docs/design/\"]\ndiff_keep = [\"test/fixtures/\"]\n"; got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	subjects := commitSubjectsIn(f, root)
	if n := len(slices.DeleteFunc(slices.Clone(subjects), func(s string) bool { return s != answersSubject(nextTag) })); n != 1 {
		t.Errorf("answers commits = %d in %q, want one", n, subjects)
	}
	payload := answersPayload(t, rec)
	if !payload.Answered || !slices.Equal(payload.Dropped, []string{"old/"}) {
		t.Errorf("answers payload = %+v, want it answered with old/ dropped", payload)
	}
}

func TestRelease_AnEntryWhoseFilesThisReleaseDeletes_IsKept(t *testing.T) {
	t.Parallel()

	const cfg = "[release]\ndiff_exclude = [\"specs/\"]\n"
	root := t.TempDir()
	writeConfig(t, root, cfg)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedCall("git", lsTree("HEAD"), ScriptedOut("src/export.go\x00"))
	f.SeedCall("git", lsTree(priorTag), ScriptedOut("src/export.go\x00specs/a.md\x00"))
	f.SeedSequence("git", notesCallsAskingNothing()...)
	f.SeedSequence("claude", ScriptedOut("none"), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	if err := engine.Release(t.Context(), attendedDeps(answeringGates(nil, nil), f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := readConfigFile(t, root); got != cfg {
		t.Errorf(".mint.toml = %q, want it as it was, %q", got, cfg)
	}
	if got := commitSubjectsIn(f, root); slices.Contains(got, answersSubject(nextTag)) {
		t.Errorf("commits = %q, want no answers commit", got)
	}
}

func TestRelease_WithNoReleaseLists_ReadsNoTreeAndLeavesTheSharedListAlone(t *testing.T) {
	t.Parallel()

	const cfg = "diff_exclude = [\".github/\"]\n"
	f, root, err := droppingRelease(t, attendedDeps, answeringGates(nil, nil), cfg,
		[]string{"src/export.go"}, notesCallsAskingNothing(), "none")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	if n := treeReads(f); n != 0 {
		t.Errorf("ls-tree reads = %d, want none", n)
	}
	if got := readConfigFile(t, root); got != cfg {
		t.Errorf(".mint.toml = %q, want it as it was, %q", got, cfg)
	}
}

func TestRelease_Unattended_ReadsNoTreeAndDropsNothing(t *testing.T) {
	t.Parallel()

	const cfg = "[release]\ndiff_exclude = [\".github/workflows/\"]\n"
	f, root, err := droppingRelease(t, newDeps, &presentertest.RecordingPresenter{}, cfg,
		[]string{"src/export.go"}, notesCallsAskingNothing(), "none")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	if n := treeReads(f); n != 0 {
		t.Errorf("ls-tree reads = %d, want none", n)
	}
	if got := readConfigFile(t, root); got != cfg {
		t.Errorf(".mint.toml = %q, want it as it was, %q", got, cfg)
	}
}
