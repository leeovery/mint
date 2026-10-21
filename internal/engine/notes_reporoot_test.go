package engine_test

import (
	"context"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// The notes diff and both Change Map reads carry cwd-relative pathspecs (`-- .` and
// the :(exclude) globs the exclusion check records root-relative), so each runs at the
// repo root whatever directory mint started in.

const subTestsDiff = "diff --git a/sub/tests/t_test.go b/sub/tests/t_test.go\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/sub/tests/t_test.go\n" +
	"@@ -0,0 +1 @@\n" +
	"+package tests\n"

// notesReads returns every notes diff and Change Map git invocation, in order.
func notesReads(f *runner.FakeRunner) []runner.Invocation {
	var reads []runner.Invocation
	for _, inv := range invocationsNamed(f, "git") {
		if len(inv.Args) < 2 || inv.Args[0] != "diff" {
			continue
		}
		switch inv.Args[1] {
		case "--src-prefix=a/", "--name-status", "--numstat":
			reads = append(reads, inv)
		}
	}
	return reads
}

// notesReadArgv is the diff then the two Change Map reads over diffRange and pathspecs.
func notesReadArgv(diffRange string, pathspecs ...string) [][]string {
	tail := append([]string{diffRange, "--", "."}, pathspecs...)
	return [][]string{
		slices.Concat([]string{"diff", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", "--no-ext-diff"}, tail),
		slices.Concat([]string{"diff", "--name-status"}, tail),
		slices.Concat([]string{"diff", "--numstat"}, tail),
	}
}

// assertNotesReadsAt fails unless the recorded notes reads are exactly want, in order,
// each run with root as its working directory.
func assertNotesReadsAt(t *testing.T, reads []runner.Invocation, root string, want [][]string) {
	t.Helper()
	if len(reads) != len(want) {
		t.Fatalf("notes reads = %d, want %d: %v", len(reads), len(want), reads)
	}
	for i, inv := range reads {
		if inv.Dir != root {
			t.Errorf("notes read %d (`git %v`) Dir = %q, want the repo root %q", i, inv.Args, inv.Dir, root)
		}
		if !slices.Equal(inv.Args, want[i]) {
			t.Errorf("notes read %d argv = %q, want %q", i, inv.Args, want[i])
		}
	}
}

func TestRelease_AnExcludeAnswerAndAnR_ReadTheNotesDiffAndChangeMapAtTheRepoRoot(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes})
	rec.NextLines = []string{"lead with the export feature"}
	f, root, err := answeredRelease(t, rec, "",
		withAnswersCommit(slices.Concat(
			[]runner.ScriptedCall{ScriptedOut(exportGoDiff + subTestsDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap,
			[]runner.ScriptedCall{ScriptedOut(exportGoDiff)}, exportOnlyChangeMap,
		)...),
		"sub/tests/: test files\n", aiBody, aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	answered := notesReadArgv(priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)sub/tests/")...)
	want := slices.Concat(
		notesReadArgv(priorTag+"..HEAD", builtinExcludes...)[:1],
		answered,
		answered,
	)
	assertNotesReadsAt(t, notesReads(f), root, want)
}

func TestRegenerateRun_FreshSourceAndAnR_ReadTheRangeDiffAndChangeMapAtTheRepoRoot(t *testing.T) {
	t.Parallel()

	const root = "/work/repo"
	notesGit := runner.NewFakeRunner()
	pass := []runner.ScriptedCall{ScriptedOut(exportGoDiff), exportOnlyChangeMap[0], exportOnlyChangeMap[1]}
	notesGit.SeedSequence("git", slices.Concat(pass, pass)...)
	tr := &freshTransport{body: aiBody}
	cfg := freshCfg()
	cfg.Release.DiffExclude = []string{"sub/tests/"}
	res := version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGit, tr, root, cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGit, tr, root, cfg, res, run)
	}

	if err := engine.RegenerateRun(t.Context(), regenWriteDeps(rec, f), pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	pass1 := notesReadArgv("v1.3.0.."+regenRunTag, append(slices.Clone(builtinExcludes), ":(exclude)sub/tests/")...)
	assertNotesReadsAt(t, notesReads(notesGit), root, slices.Concat(pass1, pass1))
}
