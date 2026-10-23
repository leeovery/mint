package engine_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	fourPartMergeBody = "✨ Added\n- Four parts, merged once   \n"
	cachedRemergeBody = "✨ Added\n- The export feature, re-merged from the cache   \n"
	reuseQuestion     = "Cached notes found for this diff — use them?"
)

var fourPartPaths = []string{"a/1.go", "b/2.go", "c/3.go", "d/4.go"}

// fourPartDiff is a range diff of four files of about 10,000 bytes of fill each, which
// chunks into four parts under partCapBytes.
func fourPartDiff(fill string) string {
	var b strings.Builder
	for _, path := range fourPartPaths {
		b.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		b.WriteString(strings.Repeat("+"+strings.Repeat(fill, 63)+"\n", 10_000/64))
	}
	return b.String()
}

// seedFourPartNotes scripts the notes assembly of diff: the diff, then the Change Map's
// name-status and numstat.
func seedFourPartNotes(f *runner.FakeRunner, diff string) {
	var nameStatus, numstat strings.Builder
	for _, path := range fourPartPaths {
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("156\t0\t" + path + "\n")
	}
	f.SeedSequence("git", ScriptedOut(diff), ScriptedOut(nameStatus.String()), ScriptedOut(numstat.String()))
}

// chunkedCacheRepo is one repo whose runs share a notes cache under cacheBase.
type chunkedCacheRepo struct {
	root, cacheBase string
}

func newChunkedCacheRepo(t *testing.T) chunkedCacheRepo {
	t.Helper()
	repo := chunkedCacheRepo{root: t.TempDir(), cacheBase: t.TempDir()}
	writeConfig(t, repo.root, partCapToml)
	return repo
}

func (repo chunkedCacheRepo) deps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, tr notes.Transport, now time.Time) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.Transport = tr
	deps.NoteCache = notescache.NewStore(repo.cacheBase, func() time.Time { return now })
	return deps
}

// dryRun runs `mint release --dry-run` over diff, its notes written by tr.
func (repo chunkedCacheRepo) dryRun(t *testing.T, diff string, tr notes.Transport) {
	t.Helper()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, repo.root, "main")
	seedFourPartNotes(f, diff)
	f.SeedSequence("git", ScriptedOut(githubRemoteURL))
	opts := priorTagNormalAIOptions()
	opts.DryRun = true
	if err := engine.Release(t.Context(), repo.deps(&presentertest.RecordingPresenter{}, f, tr, cacheWriteClock), opts); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}
}

// reusingRun runs a real release over diff that is offered cached notes, answering its
// gates with choices and its `r` nudges with nudges.
func (repo chunkedCacheRepo) reusingRun(t *testing.T, diff string, tr notes.Transport, choices []presenter.Choice, nudges ...string) (*presentertest.RecordingPresenter, *runner.FakeRunner) {
	t.Helper()
	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, repo.root, diff)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: choices, NextLines: nudges}
	if err := engine.Release(t.Context(), repo.deps(rec, f, tr, realRunClock), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	return rec, f
}

func (repo chunkedCacheRepo) entry(t *testing.T, diff string) notescache.Entry {
	t.Helper()
	return readCacheEntry(t, repo.cacheBase, repo.root, cachedKey(diff, "1.2.4"))
}

func (repo chunkedCacheRepo) cacheDir(diff string) string {
	store := notescache.NewStore(repo.cacheBase, time.Now)
	return filepath.Dir(store.EntryPath(repo.root, cachedKey(diff, "1.2.4")))
}

func gateQuestions(rec *presentertest.RecordingPresenter) []string {
	var questions []string
	for _, g := range promptGates(rec) {
		questions = append(questions, g.Question)
	}
	return questions
}

func TestRelease_RealRunAfterAChunkedDryRun_UsingTheCachedNotesShowsThemByteIdenticalWithNoAICall(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	diff := fourPartDiff("x")
	dry := &mergeTransport{label: "dry", firstMerge: fourPartMergeBody}
	repo.dryRun(t, diff, dry)
	if got := len(dry.prompts); got != 5 {
		t.Fatalf("dry-run AI calls = %d, want four chunk calls and the merge", got)
	}
	live := &mergeTransport{label: "real", firstMerge: "unexpected merge"}

	rec, f := repo.reusingRun(t, diff, live, []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes})

	if got := gateQuestions(rec); !slices.Contains(got, reuseQuestion) {
		t.Errorf("gates asked %q, want the cached-notes prompt %q", got, reuseQuestion)
	}
	if got, want := shownBodies(rec), []string{fourPartMergeBody}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the dry run's body %q", got, want)
	}
	if got := len(live.prompts); got != 0 {
		t.Errorf("AI calls = %d, want none: the cached notes were used", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != fourPartMergeBody {
		t.Errorf("tag annotation body = %q, want the dry run's body %q", got, fourPartMergeBody)
	}
}

func TestRelease_RegenOnNotesReusedFromAChunkedDryRun_MakesOneMergeCallOverTheDryRunsParts(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	diff := fourPartDiff("x")
	dry := &mergeTransport{label: "dry", firstMerge: fourPartMergeBody}
	repo.dryRun(t, diff, dry)
	live := &mergeTransport{label: "real", firstMerge: cachedRemergeBody}

	_, f := repo.reusingRun(t, diff, live,
		[]presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge)

	if got := len(live.prompts); got != 1 {
		t.Fatalf("AI calls = %d, want the one re-merge", got)
	}
	if want := withNudge(t, dry.merges[0], exportNudge); live.prompts[0] != want {
		t.Errorf("re-merge prompt = %q, want the dry run's merge prompt with the nudge in its instructions %q", live.prompts[0], want)
	}
	assertPartsInOrder(t, live.prompts[0], len(fourPartPaths))
	assertMergeFraming(t, live.prompts[0], len(fourPartPaths))
	if got := tagAnnotationBody(t, f, nextTag); got != cachedRemergeBody {
		t.Errorf("tag annotation body = %q, want the re-merge output %q", got, cachedRemergeBody)
	}
}

func TestRelease_RegenOnNotesReusedFromAChunkedRunAbortedAtTheReviewGate_MakesOnlyTheMergeCall(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	diff := fourPartDiff("x")
	aborted := &mergeTransport{label: "aborted", firstMerge: fourPartMergeBody}
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, repo.root, "main")
	seedFourPartNotes(f, diff)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}}
	assertAbortNonZero(t, engine.Release(t.Context(), repo.deps(rec, f, aborted, cacheWriteClock), priorTagNormalAIOptions()))
	next := &mergeTransport{label: "next", firstMerge: cachedRemergeBody}

	nextRec, _ := repo.reusingRun(t, diff, next,
		[]presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes}, exportNudge)

	if got := gateQuestions(nextRec); !slices.Contains(got, reuseQuestion) {
		t.Errorf("gates asked %q, want the cached-notes prompt %q", got, reuseQuestion)
	}
	if got := len(next.prompts); got != 1 {
		t.Fatalf("AI calls = %d, want the one re-merge", got)
	}
	if want := withNudge(t, aborted.merges[0], exportNudge); next.prompts[0] != want {
		t.Errorf("re-merge prompt = %q, want the aborted run's merge prompt with the nudge in its instructions %q", next.prompts[0], want)
	}
}

func TestRelease_ChunkedEntryOlderThanTheTTL_IsNotOfferedAndTheNotesAreWrittenAfresh(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	diff := fourPartDiff("x")
	repo.dryRun(t, diff, &mergeTransport{label: "dry", firstMerge: fourPartMergeBody})
	const freshBody = "✨ Added\n- Written afresh   \n"
	fresh := &mergeTransport{label: "fresh", firstMerge: freshBody}
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, repo.root, "main")
	seedFourPartNotes(f, diff)
	seedRecordTagPush(f, repo.root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	expired := cacheWriteClock.Add(notescache.TTL + time.Minute)

	if err := engine.Release(t.Context(), repo.deps(rec, f, fresh, expired), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := gateQuestions(rec); slices.Contains(got, reuseQuestion) {
		t.Errorf("gates asked %q, want no cached-notes prompt for an expired entry", got)
	}
	if got := len(fresh.prompts); got != 5 {
		t.Errorf("AI calls = %d, want four chunk calls and the merge", got)
	}
	entry := repo.entry(t, diff)
	if entry.Body != freshBody || !entry.WrittenAt.Equal(expired) {
		t.Errorf("cache entry = (%q, %v), want the fresh notes written at %v", entry.Body, entry.WrittenAt, expired)
	}
	raw := readFileString(t, filepath.Join(repo.cacheDir(diff), cachedKey(diff, "1.2.4")+".json"))
	if strings.Contains(raw, "dry notes") || !strings.Contains(raw, "fresh notes for part 4 of 4") {
		t.Errorf("cache entry = %s, want only the fresh run's part notes", raw)
	}
}

func TestRelease_WritingAChunkedEntry_LeavesTheWorkTreeAndIndexUntouched(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	diff := fourPartDiff("x")
	before := treeSnapshot(t, repo.root)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, repo.root, "main")
	seedFourPartNotes(f, diff)
	f.SeedSequence("git", ScriptedOut(githubRemoteURL))
	opts := priorTagNormalAIOptions()
	opts.DryRun = true

	err := engine.Release(t.Context(), repo.deps(&presentertest.RecordingPresenter{}, f, &mergeTransport{label: "dry", firstMerge: fourPartMergeBody}, cacheWriteClock), opts)
	if err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}

	if entry := repo.entry(t, diff); entry.Body != fourPartMergeBody {
		t.Fatalf("cache entry body = %q, want the chunked notes %q", entry.Body, fourPartMergeBody)
	}
	if after := treeSnapshot(t, repo.root); !maps.Equal(before, after) {
		t.Errorf("work tree after the write = %v, want it unchanged %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	assertNoMutation(t, f)
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, "add") {
			t.Errorf("the run staged: %q", commandLine(inv))
		}
	}
}

func TestRelease_ALaterRunWithAChangedDiff_LeavesNothingOfTheEarlierChunkedEntryInTheCache(t *testing.T) {
	t.Parallel()

	repo := newChunkedCacheRepo(t)
	earlier, later := fourPartDiff("x"), fourPartDiff("y")
	repo.dryRun(t, earlier, &mergeTransport{label: "earlier", firstMerge: fourPartMergeBody})

	repo.dryRun(t, later, &mergeTransport{label: "later", firstMerge: fourPartMergeBody})

	dir := repo.cacheDir(later)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the cache dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		if got := readFileString(t, filepath.Join(dir, e.Name())); strings.Contains(got, "earlier notes") {
			t.Errorf("%s holds the earlier entry's chunk notes", e.Name())
		}
	}
	if want := []string{cachedKey(later, "1.2.4") + ".json"}; !slices.Equal(names, want) {
		t.Errorf("cache dir = %q, want only the later run's entry %q", names, want)
	}
}

func TestRelease_RegenOnNotesReusedFromASinglePassRun_ReRunsTheAIPathFromTheSizeCheck(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(priorTagDiff))
	seedNormalAINotes(f)
	seedRecordTagPush(f, root)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{exportNudge},
	}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	calls := claudeCalls(f)
	if len(calls) != 1 {
		t.Fatalf("AI calls = %d, want the one single-pass call", len(calls))
	}
	prompt := calls[0].Stdin
	if !strings.HasPrefix(prompt, notes.DefaultPrompt+"\n\n"+exportNudge) || !strings.Contains(prompt, priorTagDiff) {
		t.Errorf("r's prompt = %q, want the single-pass prompt over the diff with the nudge in its instructions", prompt)
	}
	if got := gitDiffCalls(f); got != 2 {
		t.Errorf("git diff assemblies = %d, want the reuse check's and r's", got)
	}
	if got, want := shownBodies(rec), []string{cachedReuseBody, aiBody}; !slices.Equal(got, want) {
		t.Errorf("notes shown = %q, want the cached body then r's %q", got, want)
	}
}

func TestRegenerateRun_FreshChunkedNotes_NeitherOfferNorWriteCachedNotes(t *testing.T) {
	t.Parallel()

	cacheBase := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	tr := &mergeTransport{label: "v1.4.0", firstMerge: fivePartMergeBody}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	freshRunOf(&req, seedFivePartFreshGit(), tr, chunkingCfg(), version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
	deps := regenWriteDeps(rec, f)
	deps.NoteCache = notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })

	if err := engine.RegenerateRun(t.Context(), deps, pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got := len(tr.merges); got != 1 {
		t.Fatalf("merge calls = %d, want the chunked run's merge", got)
	}
	if got := gateQuestions(rec); slices.Contains(got, reuseQuestion) {
		t.Errorf("gates asked %q, want no cached-notes prompt", got)
	}
	assertEmptyDir(t, cacheBase)
}

func TestRegenerateAll_FreshChunkedNotes_NeitherOfferNorWriteCachedNotes(t *testing.T) {
	t.Parallel()

	cacheBase := t.TempDir()
	versions := []version.Resolution{
		{Tag: "v1.2.0", PreviousTag: "v1.1.0"},
		{Tag: "v1.3.0", PreviousTag: "v1.2.0"},
	}
	transports := map[string]*mergeTransport{
		"v1.2.0": {label: "v1.2.0", firstMerge: "✨ Added\n- v1.2.0 merged   \n"},
		"v1.3.0": {label: "v1.3.0", firstMerge: "✨ Added\n- v1.3.0 merged   \n"},
	}
	notesGits := map[string]*runner.FakeRunner{"v1.2.0": seedFivePartFreshGit(), "v1.3.0": seedFivePartFreshGit()}
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	for _, res := range versions {
		pub.seedExists(res.Tag, true, nil)
	}
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes}}
	cfg := chunkingCfg()
	req := batchReq(engine.RegenerateSourceFresh, versions, false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGits[res.Tag], transports[res.Tag], "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, res version.Resolution, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGits[res.Tag], transports[res.Tag], "", cfg, res, run)
	}
	deps := regenWriteDeps(rec, f)
	deps.NoteCache = notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })

	if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	for tag, tr := range transports {
		if got := len(tr.merges); got != 1 {
			t.Errorf("%s merge calls = %d, want its chunked run's merge", tag, got)
		}
	}
	if got := gateQuestions(rec); slices.Contains(got, reuseQuestion) {
		t.Errorf("gates asked %q, want no cached-notes prompt", got)
	}
	assertEmptyDir(t, cacheBase)
}

// treeSnapshot maps every file under root, by its path relative to root, to its
// contents.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = readFileString(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 0 {
		t.Errorf("%s holds %d entries, want none", dir, len(entries))
	}
}
