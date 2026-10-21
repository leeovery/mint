package engine_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	nearCapPromptBytes = 1_499_800
	firstSinglePass    = "✨ Added\n- First attempt, one pass   \n"
	regenSinglePass    = "✨ Added\n- Regenerated, one pass   \n"
	regenMergeBody     = "✨ Added\n- Regenerated in parts, merged   \n"
	regenRemergeBody   = "✨ Added\n- Regenerated parts, re-merged   \n"
)

// overCapNudge is long enough that appending it to the instructions of a
// nearCapPromptBytes prompt takes the prompt over the default cap.
var overCapNudge = "lead with the export feature, " + strings.Repeat("and say why it matters to operators, ", 8)

var (
	threeFilePaths = []string{"api/a.go", "core/b.go", "web/c.go"}
	oneFilePaths   = []string{"api/a.go"}
)

// nearCapRelease is a release diff over paths with its Change Map's git answers.
type nearCapRelease struct {
	diff, nameStatus, numstat string
}

// newNearCapRelease builds a diff over paths whose composed single-pass prompt, under
// the default instructions, is exactly promptBytes.
func newNearCapRelease(t *testing.T, paths []string, promptBytes int) nearCapRelease {
	t.Helper()
	var nameStatus, numstat strings.Builder
	for _, path := range paths {
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("9000\t0\t" + path + "\n")
	}
	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut(nameStatus.String()), ScriptedOut(numstat.String()))
	changeMap, err := notes.NewAssembler(f, notes.ExcludeConfig{}).BuildChangeMapForRange(t.Context(), priorTag+"..HEAD", config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("building the Change Map: %v", err)
	}
	overhead := len(notes.ComposePrompt(notes.DefaultPrompt, changeMap, ""))
	return nearCapRelease{diff: diffOfSize(paths, promptBytes-overhead), nameStatus: nameStatus.String(), numstat: numstat.String()}
}

// seedGeneration scripts one full notes generation's git calls: the diff, then the
// Change Map's name-status and numstat.
func (r nearCapRelease) seedGeneration(f *runner.FakeRunner) {
	f.SeedSequence("git", ScriptedOut(r.diff), ScriptedOut(r.nameStatus), ScriptedOut(r.numstat))
}

// diffOfSize is a diff over paths of exactly size bytes, split evenly across them.
func diffOfSize(paths []string, size int) string {
	var b strings.Builder
	per := size / len(paths)
	for i, path := range paths {
		n := per
		if i == len(paths)-1 {
			n = size - per*(len(paths)-1)
		}
		header := "diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9000 @@\n"
		body := n - len(header)
		lines, extra := body/64, body%64
		b.WriteString(header)
		b.WriteString("+" + strings.Repeat("x", 62+extra) + "\n")
		b.WriteString(strings.Repeat("+"+strings.Repeat("x", 62)+"\n", lines-1))
	}
	return b.String()
}

// sizeCheckTransport answers single-pass calls, chunk and piece calls, and merge calls
// apart: single-pass calls with singlePass and merges with merges, each in turn. It
// records every prompt in arrival order.
type sizeCheckTransport struct {
	singlePass []string
	merges     []string

	mu      sync.Mutex
	prompts []string
	served  struct{ singlePass, merges int }
}

func (st *sizeCheckTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prompts = append(st.prompts, prompt)
	if m := partFraming.FindStringSubmatch(prompt); m != nil {
		return "- notes for part " + m[1] + " of " + m[2], nil
	}
	if strings.Contains(prompt, "diff --git ") {
		st.served.singlePass++
		return st.singlePass[st.served.singlePass-1], nil
	}
	st.served.merges++
	return st.merges[st.served.merges-1], nil
}

// regenPrompts is every prompt the run sent after the first attempt's.
func (st *sizeCheckTransport) regenPrompts() []string {
	return st.prompts[1:]
}

// releaseWithRegens runs a real prior-tag release over tr whose review gate takes
// one `r` per nudge, then accepts.
func releaseWithRegens(t *testing.T, root string, f *runner.FakeRunner, tr notes.Transport, deps func(*presentertest.RecordingPresenter, *runner.FakeRunner) engine.ReleaseDeps, nudges ...string) *presentertest.RecordingPresenter {
	t.Helper()
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	choices := []presenter.Choice{presenter.ChoiceYes}
	for range nudges {
		choices = append(choices, presenter.ChoiceRegen)
	}
	rec := &presentertest.RecordingPresenter{NextChoices: append(choices, presenter.ChoiceYes), NextLines: nudges}
	d := deps(rec, f)
	d.Transport = tr
	if err := engine.Release(t.Context(), d, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	return rec
}

// nudgedInstructions is the default instructions with nudge appended, as every call
// of an `r` carrying nudge opens.
func nudgedInstructions(nudge string) string {
	return notes.DefaultPrompt + "\n\n" + nudge + "\n\n"
}

// assertChunkedRegen fails unless regen, an `r`'s calls in arrival order, are chunk
// calls none bigger than the cap, then one merge call over their notes, each opening
// with the release's instructions carrying nudge.
func assertChunkedRegen(t *testing.T, regen []string, nudge string) {
	t.Helper()
	if len(regen) < 3 {
		t.Fatalf("r's AI calls = %d, want chunk calls then one merge call", len(regen))
	}
	chunks, merge := regen[:len(regen)-1], regen[len(regen)-1]
	for i, prompt := range chunks {
		if !partFraming.MatchString(prompt) {
			t.Errorf("r's call %d is not a chunk call", i+1)
		}
		if len(prompt) > config.DefaultMaxPromptBytes {
			t.Errorf("chunk call %d = %d bytes, over the cap", i+1, len(prompt))
		}
		if !strings.HasPrefix(prompt, nudgedInstructions(nudge)) {
			t.Errorf("chunk call %d does not carry the nudge in the release's instructions", i+1)
		}
	}
	if partFraming.MatchString(merge) || strings.Contains(merge, "diff --git ") {
		t.Errorf("r's last call is not the merge")
	}
	if !strings.HasPrefix(merge, nudgedInstructions(nudge)) {
		t.Errorf("the merge call does not carry the nudge in the release's instructions")
	}
	assertPartsInOrder(t, merge, len(chunks))
}

// assertReMergeWithOnlyTheNewNudge fails unless remerge is merge, a chunked `r`'s
// merge call carrying firstNudge, with only newNudge in its instructions.
func assertReMergeWithOnlyTheNewNudge(t *testing.T, merge, remerge, firstNudge, newNudge string) {
	t.Helper()
	rest, ok := strings.CutPrefix(merge, nudgedInstructions(firstNudge))
	if !ok || partFraming.MatchString(merge) {
		t.Fatalf("the chunked r's last call is not a merge opening with its nudged instructions")
	}
	if want := nudgedInstructions(newNudge) + rest; remerge != want {
		t.Errorf("re-merge prompt = %q, want the chunked r's merge with only the new nudge %q", remerge, want)
	}
}

func TestRelease_RegenOnSinglePassNotes_WithinTheCap_MakesOneSinglePassCallWithTheNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, 400_000)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass, regenSinglePass}}

	releaseWithRegens(t, root, f, tr, newDeps, exportNudge)

	if got := len(tr.prompts); got != 2 {
		t.Fatalf("AI calls = %d, want the first attempt and one single-pass call for the r", got)
	}
	if want := withNudge(t, tr.prompts[0], exportNudge); tr.prompts[1] != want {
		t.Errorf("r's prompt is not the first attempt's prompt with the nudge appended to its instructions")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenSinglePass {
		t.Errorf("tag annotation body = %q, want the r's notes %q", got, regenSinglePass)
	}
}

func TestRelease_RegenWhoseNudgeTakesThePromptOverTheCap_WritesTheNotesInPartsCarryingTheNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, nearCapPromptBytes)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody}}

	rec := releaseWithRegens(t, root, f, tr, newDeps, overCapNudge)

	if got := len(tr.prompts[0]); got != nearCapPromptBytes {
		t.Fatalf("first attempt's prompt = %d bytes, want %d", got, nearCapPromptBytes)
	}
	if got := len(withNudge(t, tr.prompts[0], overCapNudge)); got <= config.DefaultMaxPromptBytes {
		t.Fatalf("the nudged single-pass prompt = %d bytes, want it over the cap", got)
	}
	assertChunkedRegen(t, tr.regenPrompts(), overCapNudge)
	if got, want := shownBodies(rec), []string{firstSinglePass, regenMergeBody}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the first attempt's then the merge output %q", got, want)
	}
	var reviewAsks int
	for _, g := range promptGates(rec) {
		if g.Question == presenter.ReleaseReviewGate().Question {
			reviewAsks++
		}
	}
	if reviewAsks != 2 {
		t.Errorf("review gate asked %d times, want once before the r and once after", reviewAsks)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenMergeBody {
		t.Errorf("tag annotation body = %q, want the merge output %q", got, regenMergeBody)
	}
}

func TestRelease_RegenOverTheCapWithAFileTooBigForOneChunkCall_CarriesTheNudgeInEveryPieceCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, oneFilePaths, nearCapPromptBytes)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody}}

	releaseWithRegens(t, root, f, tr, newDeps, overCapNudge)

	regen := tr.regenPrompts()
	var pieces int
	for i, prompt := range regen[:len(regen)-1] {
		if !strings.Contains(prompt, "of the diff for api/a.go, a file too large to read in one part") {
			t.Errorf("r's call %d is not a piece call of api/a.go", i+1)
			continue
		}
		pieces++
		if len(prompt) > config.DefaultMaxPromptBytes {
			t.Errorf("piece call %d = %d bytes, over the cap", i+1, len(prompt))
		}
		if !strings.HasPrefix(prompt, nudgedInstructions(overCapNudge)) {
			t.Errorf("piece call %d does not carry the nudge in the release's instructions", i+1)
		}
	}
	if pieces < 2 {
		t.Errorf("piece calls = %d, want the file cut into pieces", pieces)
	}
	if !strings.HasPrefix(regen[len(regen)-1], nudgedInstructions(overCapNudge)) {
		t.Errorf("the merge call does not carry the nudge in the release's instructions")
	}
}

func TestRelease_RegenAfterAnRegenTheLadderChunked_ReMergesThatRegensPartsWithOnlyTheNewNudge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, nearCapPromptBytes)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass}, merges: []string{regenMergeBody, regenRemergeBody}}

	releaseWithRegens(t, root, f, tr, newDeps, overCapNudge, fixesNudge)

	regen := tr.regenPrompts()
	remerge := regen[len(regen)-1]
	assertChunkedRegen(t, regen[:len(regen)-1], overCapNudge)
	assertReMergeWithOnlyTheNewNudge(t, regen[len(regen)-2], remerge, overCapNudge, fixesNudge)
	if strings.Contains(remerge, "diff --git ") {
		t.Errorf("the re-merge call carries diff")
	}
	if got := gitDiffCalls(f); got != 2 {
		t.Errorf("git diff assemblies = %d, want the first attempt's and the chunked r's only", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenRemergeBody {
		t.Errorf("tag annotation body = %q, want the re-merge output %q", got, regenRemergeBody)
	}
}

func TestRelease_RegenWithAnEmptyNudgeAfterARealRunCachedItsNotes_CallsTheAIWithoutOfferingTheCache(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	release := newNearCapRelease(t, threeFilePaths, 400_000)
	release.seedGeneration(f)
	release.seedGeneration(f)
	tr := &sizeCheckTransport{singlePass: []string{firstSinglePass, regenSinglePass}}
	now := func() time.Time { return realRunClock }
	withCache := func(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
		return newDepsWithRealRunCache(rec, f, cacheBase, now)
	}

	rec := releaseWithRegens(t, root, f, tr, withCache, "")

	if got := readCacheEntry(t, cacheBase, root, cachedKey(release.diff, "1.2.4")).Body; got != firstSinglePass {
		t.Errorf("cached body = %q, want the real run's first notes %q", got, firstSinglePass)
	}
	if got := len(tr.prompts); got != 2 {
		t.Fatalf("AI calls = %d, want the first attempt and the r's call", got)
	}
	if tr.prompts[1] != tr.prompts[0] {
		t.Errorf("the empty-nudge r's prompt differs from the first attempt's")
	}
	for _, g := range promptGates(rec) {
		if g.Question == "Cached notes found for this diff — use them?" {
			t.Errorf("the r offered the cached notes")
		}
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenSinglePass {
		t.Errorf("tag annotation body = %q, want the r's notes %q", got, regenSinglePass)
	}
}
