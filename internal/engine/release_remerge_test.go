package engine_test

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	sixPartMergeBody  = "✨ Added\n- Six parts, merged once   \n"
	firstRemergeBody  = "✨ Added\n- The export feature, first   \n"
	secondRemergeBody = "✨ Added\n- Fixes, grouped   \n"
	exportNudge       = "lead with the export feature"
	fixesNudge        = "group the fixes"
)

func seedSixPartNotes(f *runner.FakeRunner) {
	var nameStatus, numstat strings.Builder
	for _, path := range sixPartPaths {
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("156\t0\t" + path + "\n")
	}
	f.SeedSequence("git", ScriptedOut(sixFileDiff(10_000)), ScriptedOut(nameStatus.String()), ScriptedOut(numstat.String()))
}

// seedSixPartAI scripts the AI's answers: six chunk calls' notes, the merge output,
// then one output per re-merge.
func seedSixPartAI(f *runner.FakeRunner, remerges ...string) {
	calls := make([]runner.ScriptedCall, 0, len(sixPartPaths)+1+len(remerges))
	for i := range sixPartPaths {
		calls = append(calls, ScriptedOut(fmt.Sprintf("- notes written for chunk call %d", i+1)))
	}
	calls = append(calls, ScriptedOut(sixPartMergeBody))
	for _, body := range remerges {
		calls = append(calls, ScriptedOut(body))
	}
	seedClaudeAfterCheck(f, calls...)
}

func claudeCalls(f *runner.FakeRunner) []runner.Invocation {
	return notesInvocations(f, "claude")
}

// withNudge is mergePrompt, which carries the default instructions, with nudge
// appended to those instructions.
func withNudge(t *testing.T, mergePrompt, nudge string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(mergePrompt, notes.DefaultPrompt)
	if !ok {
		t.Fatalf("the merge prompt does not open with the release's instructions")
	}
	return notes.DefaultPrompt + "\n\n" + nudge + rest
}

func TestRelease_RegenOnChunkedNotes_MakesOneMergeCallCarryingTheNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, partCapToml)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedSixPartNotes(f)
	seedSixPartAI(f, firstRemergeBody)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	calls := claudeCalls(f)
	if len(calls) != 8 {
		t.Fatalf("AI calls = %d, want six chunk calls, the merge and one re-merge", len(calls))
	}
	for i, inv := range calls {
		if !slices.Equal(inv.Args, leanClaudeArgs) {
			t.Errorf("AI call %d argv = %v, want the release's ai_command %v", i+1, inv.Args, leanClaudeArgs)
		}
	}
	firstMerge, remerge := calls[6].Stdin, calls[7].Stdin
	if want := withNudge(t, firstMerge, exportNudge); remerge != want {
		t.Errorf("re-merge prompt = %q, want the first merge's prompt with the nudge in its instructions %q", remerge, want)
	}
	if strings.Contains(remerge, "diff --git ") {
		t.Errorf("the re-merge call carries diff")
	}
	assertPartsInOrder(t, remerge, len(sixPartPaths))
	assertMergeFraming(t, remerge, len(sixPartPaths))
	if got := gitDiffCalls(f); got != 1 {
		t.Errorf("git diff assemblies = %d, want the first generation's only", got)
	}
}

func TestRelease_RegenOnChunkedNotes_ReShowsTheReMergeAndShipsItByteIdentical(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, partCapToml)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedSixPartNotes(f)
	seedSixPartAI(f, firstRemergeBody)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	var shown []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowNotes {
			shown = append(shown, ev.ShowNotes.Body)
		}
	}
	if want := []string{sixPartMergeBody, firstRemergeBody}; !slices.Equal(shown, want) {
		t.Errorf("notes shown = %q, want the merge output then the re-merge output %q", shown, want)
	}
	var reviewQuestions int
	for _, g := range promptGates(rec) {
		if g.Question == presenter.ReleaseReviewGate().Question {
			reviewQuestions++
		}
	}
	if reviewQuestions != 2 {
		t.Errorf("review gate asked %d times, want once before the r and once after", reviewQuestions)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != firstRemergeBody {
		t.Errorf("tag annotation body = %q, want the re-merge output %q", got, firstRemergeBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != firstRemergeBody {
		t.Errorf("CHANGELOG body = %q, want the re-merge output %q", got, firstRemergeBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != firstRemergeBody {
		t.Errorf("provider release body = %q, want the re-merge output %q", got, firstRemergeBody)
	}
}

func TestRelease_SecondRegenOnChunkedNotes_ReMergesTheWavesNotesWithOnlyItsOwnNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, partCapToml)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedSixPartNotes(f)
	seedSixPartAI(f, firstRemergeBody, secondRemergeBody)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge, fixesNudge},
	}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	calls := claudeCalls(f)
	if len(calls) != 9 {
		t.Fatalf("AI calls = %d, want six chunk calls, the merge and two re-merges", len(calls))
	}
	second := calls[8].Stdin
	if want := withNudge(t, calls[6].Stdin, fixesNudge); second != want {
		t.Errorf("second re-merge prompt = %q, want the first merge's prompt with only its own nudge %q", second, want)
	}
	if strings.Contains(second, exportNudge) {
		t.Errorf("the second re-merge carries the first nudge")
	}
	if strings.Contains(second, firstRemergeBody) {
		t.Errorf("the second re-merge carries the first re-merge's output")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != secondRemergeBody {
		t.Errorf("tag annotation body = %q, want the second re-merge output %q", got, secondRemergeBody)
	}
}

const halvedRemergeBody = "✨ Added\n- Three parts, re-merged   \n"

// halvedToThreeTransport fails the single-pass call with an overflow's output,
// answers its classify call SIZE, writes each chunk call's notes naming its part,
// and answers the first merge halvedNotesBody and every later one halvedRemergeBody.
type halvedToThreeTransport struct {
	mu      sync.Mutex
	prompts []string
	chunks  int
	merges  []string
}

var partFraming = regexp.MustCompile(`What follows is part (\d+) of (\d+)`)

func (ht *halvedToThreeTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	ht.mu.Lock()
	defer ht.mu.Unlock()
	ht.prompts = append(ht.prompts, prompt)
	switch {
	case isClassifyPrompt(prompt):
		return "SIZE", nil
	case partFraming.MatchString(prompt):
		ht.chunks++
		m := partFraming.FindStringSubmatch(prompt)
		return "- notes for part " + m[1] + " of " + m[2], nil
	case strings.Contains(prompt, "diff --git "):
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	}
	ht.merges = append(ht.merges, prompt)
	if len(ht.merges) == 1 {
		return halvedNotesBody, nil
	}
	return halvedRemergeBody, nil
}

func TestRelease_RegenOnHalvedSinglePassNotes_MakesOneMergeCallOverTheThreeParts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	var diff strings.Builder
	for _, path := range []string{"api/a.go", "core/b.go", "web/c.go"} {
		diff.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		diff.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", halvableFileBytes/64))
	}
	f.SeedSequence("git",
		ScriptedOut(diff.String()),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\nM\tweb/c.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n156\t0\tweb/c.go\n"),
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}
	tr := &halvedToThreeTransport{}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if tr.chunks != 3 || len(tr.merges) != 2 {
		t.Fatalf("chunk calls = %d, merge calls = %d, want a halved run of three parts and one re-merge", tr.chunks, len(tr.merges))
	}
	if got := len(tr.prompts); got != 7 {
		t.Errorf("AI calls = %d, want the single-pass call, its classify call, three chunk calls, the merge and one re-merge", got)
	}
	if want := withNudge(t, tr.merges[0], exportNudge); tr.merges[1] != want {
		t.Errorf("re-merge prompt = %q, want the first merge's prompt with the nudge in its instructions %q", tr.merges[1], want)
	}
	assertPartsInOrder(t, tr.merges[1], 3)
	if got := tagAnnotationBody(t, f, nextTag); got != halvedRemergeBody {
		t.Errorf("tag annotation body = %q, want the re-merge output %q", got, halvedRemergeBody)
	}
}

// assertMergeFraming fails unless mergePrompt carries the merge framing over n
// parts, spelled out because the exact wording is the contract: one set of notes in
// the format above, the parts' nothing-to-list lines left out, with no ranking rule,
// no headline and no nothing-to-list body of mint's own.
func assertMergeFraming(t *testing.T, mergePrompt string, n int) {
	t.Helper()
	framing := fmt.Sprintf("This release's diff was too large to read in one pass, so it was read in %d parts and release notes were written for each part. "+
		"Those notes follow, in part order. From them, write one set of release notes for the whole release, in the format above: "+
		"combine related changes across parts into single bullets and drop duplicates. "+
		"Leave out every line in which a part said it holds nothing for the notes. "+
		"Only where every part returned such a line, write what the format above gives a release with nothing to list; never do so while any part listed a change.", n)
	if !strings.Contains(mergePrompt, "\n\n"+framing+"\n\n") {
		t.Errorf("the merge prompt does not carry the merge framing %q", framing)
	}
	if strings.Contains(mergePrompt, "rank by the Change Map") {
		t.Errorf("the merge prompt asks to rank by the Change Map")
	}
}

// assertPartsInOrder fails unless mergePrompt carries the notes headings of parts 1
// to n, in that order, and no more.
func assertPartsInOrder(t *testing.T, mergePrompt string, n int) {
	t.Helper()
	at := 0
	for k := 1; k <= n; k++ {
		heading := fmt.Sprintf("Notes for part %d of %d:", k, n)
		i := strings.Index(mergePrompt[at:], heading)
		if i < 0 {
			t.Fatalf("the merge prompt does not carry %q after the parts before it", heading)
		}
		at += i + len(heading)
	}
	if got := strings.Count(mergePrompt, "Notes for part "); got != n {
		t.Errorf("parts' notes carried = %d, want %d", got, n)
	}
}

// gitDiffCalls counts the run's diff assemblies.
func gitDiffCalls(f *runner.FakeRunner) int {
	var n int
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 1 && inv.Args[0] == "diff" && inv.Args[1] == "--src-prefix=a/" {
			n++
		}
	}
	return n
}
