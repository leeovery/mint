package engine_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	readmeDiff = "diff --git a/README.md b/README.md\n" +
		"--- a/README.md\n" +
		"+++ b/README.md\n" +
		"@@ -1 +1 @@\n" +
		"-# Mint\n" +
		"+# Mint, now with exports\n"
	designADiff = "diff --git a/docs/design/a.md b/docs/design/a.md\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/docs/design/a.md\n" +
		"@@ -0,0 +1 @@\n" +
		"+# Design A\n"
	designSpecDiff = "diff --git a/docs/design/spec.md b/docs/design/spec.md\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/docs/design/spec.md\n" +
		"@@ -0,0 +1 @@\n" +
		"+# Spec\n"
)

// exportOnlyChangeMap is the Change Map's git answers once only src/export.go is left.
var exportOnlyChangeMap = []runner.ScriptedCall{
	ScriptedOut("M\tsrc/export.go\n"),
	ScriptedOut("1\t1\tsrc/export.go\n"),
}

// answeringGates answers the exclusion pauses with pauses in turn and the review gate
// with reviews in turn; every other gate, and any gate past its answers, takes its
// default.
func answeringGates(pauses, reviews []presenter.Choice) *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			queue := &reviews
			if isPauseGate(g) {
				queue = &pauses
			} else if !g.Has(presenter.ChoiceRegen) || !g.Has(presenter.ChoiceEdit) {
				return g.Default, nil
			}
			if len(*queue) == 0 {
				return g.Default, nil
			}
			choice := (*queue)[0]
			*queue = (*queue)[1:]
			return choice, nil
		},
	}
}

// answeredRelease runs an attended prior-tag release under config, git answering the
// notes calls with gitCalls in turn, claude answering the check with checkAnswer and
// then each of notesCalls in turn.
func answeredRelease(t *testing.T, rec *presentertest.RecordingPresenter, config string, gitCalls []runner.ScriptedCall, checkAnswer string, notesCalls ...string) (*runner.FakeRunner, string, error) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", gitCalls...)
	claude := []runner.ScriptedCall{ScriptedOut(checkAnswer)}
	for _, body := range notesCalls {
		claude = append(claude, ScriptedOut(body))
	}
	f.SeedSequence("claude", claude...)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	return f, root, engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())
}

// notesDiffCalls returns the argv of every notes diff git call, in order.
func notesDiffCalls(f *runner.FakeRunner) [][]string {
	var calls [][]string
	for _, inv := range invocationsNamed(f, "git") {
		if len(inv.Args) > 1 && inv.Args[0] == "diff" && inv.Args[1] == "--src-prefix=a/" {
			calls = append(calls, inv.Args)
		}
	}
	return calls
}

func notesDiffArgs(pathspecs ...string) []string {
	return append([]string{"diff", "--src-prefix=a/", "--dst-prefix=b/", priorTag + "..HEAD", "--", "."}, pathspecs...)
}

var builtinExcludes = []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml"}

// assertNoNotesCallCarries fails if any notes call names path.
func assertNoNotesCallCarries(t *testing.T, f *runner.FakeRunner, path string) {
	t.Helper()
	for i, inv := range notesInvocations(f, "claude") {
		if strings.Contains(inv.Stdin, path) {
			t.Errorf("notes call %d carries %s:\n%s", i, path, inv.Stdin)
		}
	}
}

func TestRelease_AnExcludeAnswer_LeavesItsFilesOutOfASinglePassRunsDiffAndChangeMap(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, _, err := answeredRelease(t, rec, "",
		withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...),
		"docs/design/: design documents\n", aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one single-pass call", len(notesCalls))
	}
	if !strings.Contains(notesCalls[0].Stdin, "\n\n"+exportGoDiff+"\n\n") {
		t.Errorf("the notes call does not carry the post-answer diff:\n%s", notesCalls[0].Stdin)
	}
	assertNoNotesCallCarries(t, f, "docs/design/")
}

func TestRelease_AnExcludeAnswer_LeavesItsFilesOutOfEveryCallOfAChunkedRun(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, _, err := answeredRelease(t, rec, "max_prompt_bytes = 20000\n",
		withAnswersCommit(
			ScriptedOut(twoFileDiff(10_000)+designDocDiff),
			ScriptedOut(twoFileDiff(10_000)),
			ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
			ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
		),
		"docs/design/: design documents\n", "- api notes", "- core notes", chunkedMergeBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
	if got := len(notesInvocations(f, "claude")); got != 3 {
		t.Fatalf("notes calls = %d, want two chunk calls and the merge", got)
	}
	assertNoNotesCallCarries(t, f, "docs/design/")
	if got := tagAnnotationBody(t, f, nextTag); got != chunkedMergeBody {
		t.Errorf("tag annotation body = %q, want the merge output %q", got, chunkedMergeBody)
	}
}

func TestRelease_AnExcludeAnswer_TakesOutAFileTheKeepListCovers(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, _, err := answeredRelease(t, rec, "[release]\ndiff_keep = [\"docs/design/spec.md\"]\n",
		withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designSpecDiff + designADiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...),
		"docs/design/: design documents\n", aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := [][]string{notesDiffArgs(builtinExcludes...), notesDiffArgs(append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)}
	if got := notesDiffCalls(f); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("notes diff calls = %q, want %q", got, want)
	}
	assertNoNotesCallCarries(t, f, "docs/design/spec.md")
}

func TestRelease_ExcludingTheTranscriptThatPutItOverTheCap_GivesOneSinglePassCall(t *testing.T) {
	t.Parallel()

	transcriptDiff := "diff --git a/notes/session.jsonl b/notes/session.jsonl\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/notes/session.jsonl\n" +
		"@@ -0,0 +1,30000 @@\n" +
		strings.Repeat("+"+strings.Repeat("x", 62)+"\n", 1_900_000/64)
	if before := len(exportGoDiff + transcriptDiff); before <= config.DefaultMaxPromptBytes {
		t.Fatalf("the diff before the answer is %d bytes, want it over the %d-byte cap", before, config.DefaultMaxPromptBytes)
	}
	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, _, err := answeredRelease(t, rec, "",
		withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + transcriptDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...),
		"notes/session.jsonl: an agent session transcript\n", aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one single-pass call", len(notesCalls))
	}
	if prompt := notesCalls[0].Stdin; !singlePassCall(prompt) || len(prompt) > config.DefaultMaxPromptBytes {
		t.Errorf("the notes call is not one under-cap single-pass call (%d bytes)", len(prompt))
	}
	assertNoNotesCallCarries(t, f, "notes/session.jsonl")
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the single-pass notes %q", got, aiBody)
	}
}

func TestRelease_ExcludeAnswersLeavingNoPath_GiveTheMaintenanceBodyWithNoNotesCall(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, _, err := answeredRelease(t, rec, "",
		withAnswersCommit(ScriptedOut(designDocDiff), ScriptedOut("")),
		"docs/design/: design documents\n")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if notesCallMade(f) {
		t.Errorf("a notes call was made for a diff the answers left empty")
	}
	const want = "Maintenance release — no notable source changes"
	if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(want) {
		t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, terminatedAnnotationBody(want))
	}
}

// cachedAnsweredRelease runs a release as answeredRelease does, over a cache holding
// the notes a dry run wrote for the diff before any answer.
func cachedAnsweredRelease(t *testing.T, rec *presentertest.RecordingPresenter, gitCalls []runner.ScriptedCall, notesCalls ...string) *runner.FakeRunner {
	t.Helper()
	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, exportGoDiff+designDocDiff, "1.2.4", cachedReuseBody)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", gitCalls...)
	f.SeedSequence("claude", append([]runner.ScriptedCall{ScriptedOut("docs/design/: design documents\n")}, scriptedOuts(notesCalls)...)...)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	deps.StdinInteractive = true
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	return f
}

func scriptedOuts(bodies []string) []runner.ScriptedCall {
	calls := make([]runner.ScriptedCall, len(bodies))
	for i, body := range bodies {
		calls[i] = ScriptedOut(body)
	}
	return calls
}

func TestRelease_AnExcludeAnswerAfterADryRunCachedItsNotes_WritesFreshNotesWithoutOfferingTheCache(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f := cachedAnsweredRelease(t, rec,
		withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...),
		aiBody)

	if reuseConfirmPrompted(rec) {
		t.Errorf("the cached notes were offered after an Exclude answer changed the diff")
	}
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one fresh call", len(notesCalls))
	}
	assertNoNotesCallCarries(t, f, "docs/design/")
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the fresh notes %q", got, aiBody)
	}
}

func TestRelease_EveryPauseAnsweredKeepAfterADryRunCachedItsNotes_OffersTheCachedNotes(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceKeep}, nil)
	f := cachedAnsweredRelease(t, rec, withAnswersCommit(ScriptedOut(exportGoDiff+designDocDiff)))

	if !reuseConfirmPrompted(rec) {
		t.Errorf("the cached notes were not offered when every pause was answered Keep")
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made; the cached notes should have been used")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != cachedReuseBody {
		t.Errorf("tag annotation body = %q, want the cached notes %q", got, cachedReuseBody)
	}
}

func TestRelease_AnRAfterAnExcludeAnswer_WritesItsNotesWithoutTheExcludedFiles(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes})
	rec.NextLines = []string{"lead with the export feature"}
	const regenBody = "✨ Added\n- Regenerated without the design docs   \n"
	f, _, err := answeredRelease(t, rec, "",
		withAnswersCommit(slices.Concat(
			[]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap,
			[]runner.ScriptedCall{ScriptedOut(exportGoDiff)}, exportOnlyChangeMap,
		)...),
		"docs/design/: design documents\n", aiBody, regenBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	answered := notesDiffArgs(append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
	want := [][]string{notesDiffArgs(builtinExcludes...), answered, answered}
	if got := notesDiffCalls(f); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("notes diff calls = %q, want %q", got, want)
	}
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 2 {
		t.Fatalf("notes calls = %d, want the first attempt and the r", len(notesCalls))
	}
	if !strings.Contains(notesCalls[1].Stdin, "lead with the export feature") {
		t.Errorf("the second notes call is not the r's")
	}
	assertNoNotesCallCarries(t, f, "docs/design/")
	if got := tagAnnotationBody(t, f, nextTag); got != regenBody {
		t.Errorf("tag annotation body = %q, want the r's notes %q", got, regenBody)
	}
}

func TestRelease_AKeepAnswerAfterAnExcludeAnswer_NeverBringsBackAFileTheExcludeTookOut(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	f, _, err := answeredRelease(t, rec, "",
		withAnswersCommit(
			ScriptedOut(exportGoDiff+readmeDiff+designADiff),
			ScriptedOut(exportGoDiff+readmeDiff),
			ScriptedOut("M\tsrc/export.go\nM\tREADME.md\n"),
			ScriptedOut("1\t1\tsrc/export.go\n1\t1\tREADME.md\n"),
		),
		"docs/design/: design documents\n*.md: documentation\n", aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(pauseGates(rec)); got != 2 {
		t.Fatalf("pauses = %d, want the Exclude and the Keep", got)
	}
	want := [][]string{notesDiffArgs(builtinExcludes...), notesDiffArgs(append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)}
	if got := notesDiffCalls(f); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("notes diff calls = %q, want %q", got, want)
	}
	prompt := notesStdinOf(t, f, "claude", leanClaudeArgs...)
	if !strings.Contains(prompt, readmeDiff) {
		t.Errorf("the notes call does not carry README.md:\n%s", prompt)
	}
	assertNoNotesCallCarries(t, f, "docs/design/a.md")
}
