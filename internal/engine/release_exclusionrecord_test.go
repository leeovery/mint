package engine_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// excludeThenKeepCalls is git's notes answers for a release changing src/export.go,
// docs/design/ and test/fixtures/, once with every path and once docs/design/ is out.
var excludeThenKeepCalls = []runner.ScriptedCall{
	ScriptedOut(exportGoDiff + designDocDiff + fixturesDiff),
	ScriptedOut(exportGoDiff + fixturesDiff),
	ScriptedOut("M\tsrc/export.go\nA\ttest/fixtures/export.json\n"),
	ScriptedOut("1\t1\tsrc/export.go\n1\t0\ttest/fixtures/export.json\n"),
}

func TestRelease_ExcludeAndKeepAnswers_JoinTheReleaseTablesExistingLists(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	_, root, err := answeredRelease(t, rec,
		"[release]\ndiff_exclude = [\"specs/\"]\ndiff_keep = [\"docs/api/\"]\n",
		excludeThenKeepCalls,
		"docs/design/: design documents\ntest/fixtures/: test data\n", aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("loading the answered .mint.toml: %v", err)
	}
	if want := []string{"specs/", "docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if want := []string{"docs/api/", "test/fixtures/"}; !slices.Equal(cfg.Release.DiffKeep, want) {
		t.Errorf("[release].diff_keep = %q, want %q", cfg.Release.DiffKeep, want)
	}
	if len(cfg.DiffExclude) != 0 {
		t.Errorf("shared diff_exclude = %q, want it untouched and unset", cfg.DiffExclude)
	}
}

func TestRelease_AKeepAnswerWithNoConfigFile_CreatesItHoldingOnlyTheReleaseTable(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceKeep}, nil)
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(exportGoDiff+fixturesDiff),
		ScriptedOut("M\tsrc/export.go\nA\ttest/fixtures/export.json\n"),
		ScriptedOut("1\t1\tsrc/export.go\n1\t0\ttest/fixtures/export.json\n"),
	)
	f.SeedSequence("claude", ScriptedOut("test/fixtures/: test data\n"), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	if err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("reading .mint.toml: %v", err)
	}
	if want := "[release]\ndiff_keep = [\"test/fixtures/\"]\n"; string(got) != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
}
