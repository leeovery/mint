package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/cmdline"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// twoPartSaved is what a two-part run under twoPartTransport left in its parts' notes.
const twoPartSaved = apiPartNotes + "\n\n" + corePartNotes

// countedTransport counts the calls it passes on to answer.
type countedTransport struct {
	answer func(context.Context, string) (string, error)

	mu sync.Mutex
	n  int
}

func (c *countedTransport) Generate(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.answer(ctx, prompt)
}

func (c *countedTransport) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// failsNudgedMerge fails a merge call carrying exportNudge, the `r`'s re-merge.
func failsNudgedMerge(prompt string) error {
	if isMergeCallPrompt(prompt) && strings.Contains(prompt, exportNudge) {
		return failsWithLogin()
	}
	return nil
}

// chunkedFresh is a fresh two-part chunked run over partCapBytes whose `r`
// re-merge fails, its calls counted.
func chunkedFresh() (*runner.FakeRunner, *countedTransport, config.Config) {
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	cfg := freshCfg()
	cfg.MaxPromptBytes = partCapBytes
	return notesGit, &countedTransport{answer: twoPartTransport(failsNudgedMerge)}, cfg
}

// singlePassFresh scripts the git reads of a fresh single-pass run and of one `r` that re-diffs its range.
func singlePassFresh() *runner.FakeRunner {
	notesGit := runner.NewFakeRunner()
	seedTwoFileGenerations(notesGit, halvableFileBytes, 2)
	return notesGit
}

// regenFailRun is `mint release regenerate v1.4.0 --source fresh --target both` at a
// terminal without -y, its chunk notes saving under a user cache dir of its own.
type regenFailRun struct {
	root, userCache string
	f               *runner.FakeRunner
	rec             *presentertest.RecordingPresenter
	pub             *fakePublisher
	deps            engine.ReleaseDeps
}

func newRegenFailRun(t *testing.T, rec *presentertest.RecordingPresenter) *regenFailRun {
	t.Helper()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                    // status --porcelain (clean)
		ScriptedOut(regenRunReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                    // fetch --tags
		ScriptedOut("0\t1"),                // rev-list left-right count (ahead only)
	)
	f.Seed("gh", runner.Result{}, nil)
	run := &regenFailRun{root: t.TempDir(), userCache: t.TempDir(), f: f, rec: rec, pub: newFakePublisher()}
	run.pub.seedExists(regenRunTag, true, nil)
	run.deps = regenWriteDeps(rec, f)
	run.deps.StdinInteractive = true
	run.deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(run.userCache))
	return run
}

func (run *regenFailRun) regenerate(t *testing.T, notesGit *runner.FakeRunner, tr notes.Transport, cfg config.Config) error {
	t.Helper()
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetBoth))
	req.ReleaseBranch = regenRunReleaseBranch
	freshRunOf(&req, notesGit, tr, cfg, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
	return engine.RegenerateRun(t.Context(), run.deps, run.pub, run.root, req)
}

func (run *regenFailRun) savedPath() string {
	return chunkNotesPathIn(run.userCache, run.root, regenRunTag)
}

// assertWroteNothing fails unless the regenerate left the changelog and the provider
// release untouched.
func (run *regenFailRun) assertWroteNothing(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(run.root, "CHANGELOG.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("CHANGELOG.md stat err = %v, want it never written", err)
	}
	for _, inv := range run.f.Invocations() {
		if slices.Contains(inv.Args, "commit") || slices.Contains(inv.Args, "push") {
			t.Errorf("git %v ran, want no changelog commit or push", inv.Args)
		}
	}
	if len(run.pub.dispatched) != 0 {
		t.Errorf("provider writes = %d, want none", len(run.pub.dispatched))
	}
}

// regenerateSavedNotesMoves is the failure's naming of the file at path with
// version's regenerate that puts it on the version.
func regenerateSavedNotesMoves(version, path string) []presenter.NextMoves {
	return []presenter.NextMoves{{
		Stage: "regenerate",
		Moves: []presenter.NextMove{{
			Lead:    "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with",
			Command: "mint release regenerate " + version + " --notes-file " + cmdline.Quote(path),
		}},
	}}
}

func assertNamedWithRegenerate(t *testing.T, rec *presentertest.RecordingPresenter, version, path string) {
	t.Helper()
	if got, want := nextMovesEvents(rec), regenerateSavedNotesMoves(version, path); !slices.EqualFunc(got, want, nextMovesEqual) {
		t.Errorf("next moves = %+v, want %+v", got, want)
	}
}

func assertAbortExitsOne(t *testing.T, err error) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", err)
	}
}

func closedAtNudge() *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{
		NextChoices:   []presenter.Choice{presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
	}
}

func regenOnceAtTheGate() *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceRegen}, NextLines: []string{exportNudge}}
}

// ladderChunkedRegen answers a single-pass first attempt, then an `r` whose
// single-pass call overflows and halves into a chunked run that fails for good after
// some parts finish.
func ladderChunkedRegen() *chunkedRegenTransport {
	return &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, part: partFails(2, failsWithLogin()), classify: sizeForOverflow}
}

func TestRegenerateRun_ReMergeFails_SavesEveryPartsNotesAndNamesThemWithTheVersionsRegenerate(t *testing.T) {
	t.Parallel()

	run := newRegenFailRun(t, regenOnceAtTheGate())
	notesGit, tr, cfg := chunkedFresh()

	err := run.regenerate(t, notesGit, tr, cfg)

	assertAbortExitsOne(t, err)
	path := run.savedPath()
	if got := readSaved(t, path); got != twoPartSaved {
		t.Errorf("saved chunk notes = %q, want every part's notes unmarked %q", got, twoPartSaved)
	}
	sf := onlyStageFailureEvent(t, run.rec)
	if sf.Name != "regenerate" || sf.Message != "AI call failed (merging)" || sf.Output != loginOutput {
		t.Errorf("StageFailure = %+v, want the re-merge's failure", sf)
	}
	if sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
	}
	assertNamedWithRegenerate(t, run.rec, regenRunVersionKey, path)
	if got := warnings(run.rec); len(got) != 0 {
		t.Errorf("warnings = %+v, want none", got)
	}
	run.assertWroteNothing(t)
}

func TestRegenerateRun_RegenTheLadderChunkedFailsForGood_SavesTheFinishedPartsWithTheMarker(t *testing.T) {
	t.Parallel()

	run := newRegenFailRun(t, regenOnceAtTheGate())

	err := run.regenerate(t, singlePassFresh(), ladderChunkedRegen(), freshCfg())

	assertAbortExitsOne(t, err)
	path := run.savedPath()
	want := "**Notes missing:** no notes were written for `api/a.go` (partly) — add them before releasing.\n\n" +
		regenPartNotes(1, 4) + "\n\n" + regenPartNotes(3, 4) + "\n\n" + regenPartNotes(4, 4)
	if got := readSaved(t, path); got != want {
		t.Errorf("saved chunk notes = %q, want %q", got, want)
	}
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
	}
	assertNamedWithRegenerate(t, run.rec, regenRunVersionKey, path)
	run.assertWroteNothing(t)
}

func TestRegenerateRun_StdinClosedAtTheNudgeOnChunkedNotes_MakesNoCallAndSavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	run := newRegenFailRun(t, closedAtNudge())
	notesGit, tr, cfg := chunkedFresh()

	err := run.regenerate(t, notesGit, tr, cfg)

	assertAbortExitsOne(t, err)
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	if got := tr.calls(); got != 3 {
		t.Errorf("AI calls = %d, want only the first run's two chunk calls and its merge", got)
	}
	path := run.savedPath()
	if got := readSaved(t, path); got != twoPartSaved {
		t.Errorf("saved chunk notes = %q, want %q", got, twoPartSaved)
	}
	assertNamedWithRegenerate(t, run.rec, regenRunVersionKey, path)
	run.assertWroteNothing(t)
}

func TestRegenerateRun_SinglePassRegenFailsWithoutChunking_SavesAndNamesNoFile(t *testing.T) {
	t.Parallel()

	run := newRegenFailRun(t, regenOnceAtTheGate())
	tr := &chunkedRegenTransport{nudge: exportNudge, singlePass: func(context.Context) (string, error) { return "", failsWithLogin() }, classify: answers("OTHER")}

	err := run.regenerate(t, singlePassFresh(), tr, freshCfg())

	assertAbortExitsOne(t, err)
	assertNoSavedChunkNotes(t, chunkNotesBaseIn(run.userCache))
	if sf := onlyStageFailureEvent(t, run.rec); sf.Message != "AI call failed" || sf.SavedNotes != "" {
		t.Errorf("StageFailure = %+v, want the single-pass failure naming no file", sf)
	}
	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("next moves = %+v, want no saved file named", got)
	}
	run.assertWroteNothing(t)
}

func TestRegenerateAll_RegenOnAChunkedVersionFails_SavesItsFileNamesItsRegenerateAndEndsTheBatch(t *testing.T) {
	t.Parallel()

	versions := []version.Resolution{
		{Tag: "v1.2.0", PreviousTag: "v1.1.0"},
		{Tag: "v1.3.0", PreviousTag: "v1.2.0"},
		{Tag: "v1.4.0", PreviousTag: "v1.3.0"},
	}
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	for _, res := range versions {
		pub.seedExists(res.Tag, true, nil)
	}
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		NextLines:   []string{exportNudge},
	}
	root, userCache := t.TempDir(), t.TempDir()
	deps := regenWriteDeps(rec, f)
	deps.StdinInteractive = true
	deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(userCache))
	notesGit, tr, cfg := chunkedFresh()
	req := batchReq(engine.RegenerateSourceFresh, versions, false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		if res.Tag != "v1.3.0" {
			return "## fresh " + res.Tag + "\n", nil
		}
		return engine.RegenerateFreshBody(ctx, notesGit, tr, "", cfg, res, sinks)
	}
	req.ProduceRegenerator = func(_ engine.RegenerateSource, res version.Resolution, run engine.RegenRun) engine.Regenerator {
		return engine.RegenerateFreshRegenerator(notesGit, tr, "", cfg, res, run)
	}

	_, err := engine.RegenerateAll(t.Context(), deps, pub, root, req)

	assertAbortExitsOne(t, err)
	path := chunkNotesPathIn(userCache, root, "v1.3.0")
	if got := readSaved(t, path); got != twoPartSaved {
		t.Errorf("v1.3.0 saved chunk notes = %q, want %q", got, twoPartSaved)
	}
	assertNamedWithRegenerate(t, rec, "1.3.0", path)
	if got := dispatchedTags(pub); !slices.Equal(got, []string{"v1.2.0"}) {
		t.Errorf("provider writes = %v, want only v1.2.0", got)
	}
	if got := runStartedVersions(rec); !slices.Equal(got, []string{"1.2.0", "1.3.0"}) {
		t.Errorf("versions started = %v, want v1.4.0 never processed", got)
	}
	if got := warnings(rec); len(got) != 0 {
		t.Errorf("warnings = %+v, want no skip", got)
	}
	if i := indexOfKind(rec, presentertest.KindRunFinished); i != -1 {
		t.Errorf("event kinds = %v, want the batch ended with no summary", rec.Kinds())
	}
}

func TestRegenerateRun_FailedRegensChunkNotesCannotBeSaved_KeepsTheFailureAndWarnsWithTheWriteError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		rec  func() *presentertest.RecordingPresenter
		run  func() (*runner.FakeRunner, notes.Transport, config.Config)
		// failed is the ✗ line the failure shows, or nil for none.
		failed *presenter.StageFailure
	}{
		{
			name: "the re-merge fails",
			rec:  regenOnceAtTheGate,
			run: func() (*runner.FakeRunner, notes.Transport, config.Config) {
				notesGit, tr, cfg := chunkedFresh()
				return notesGit, tr, cfg
			},
			failed: &presenter.StageFailure{Name: "regenerate", Key: "notes", Message: "AI call failed (merging)", Output: loginOutput},
		},
		{
			name: "the ladder-chunked r fails for good",
			rec:  regenOnceAtTheGate,
			run: func() (*runner.FakeRunner, notes.Transport, config.Config) {
				return singlePassFresh(), ladderChunkedRegen(), freshCfg()
			},
			failed: &presenter.StageFailure{Name: "regenerate", Key: "notes", Message: "AI call failed (part 2 of 4)", Output: loginOutput},
		},
		{
			name: "stdin closes at the nudge",
			rec:  closedAtNudge,
			run: func() (*runner.FakeRunner, notes.Transport, config.Config) {
				notesGit, tr, cfg := chunkedFresh()
				return notesGit, tr, cfg
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newRegenFailRun(t, tc.rec())
			run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}
			notesGit, tr, cfg := tc.run()

			assertAbortExitsOne(t, run.regenerate(t, notesGit, tr, cfg))

			if tc.failed != nil {
				if sf := untimed(onlyStageFailureEvent(t, run.rec)); sf != *tc.failed {
					t.Errorf("StageFailure = %+v, want the failure unchanged %+v", sf, *tc.failed)
				}
				assertWarnedAfterTheFailure(t, run.rec)
			}
			assertOnlyWarning(t, run.rec, unsavedChunkNotesWarning(errChunkNotesWrite.Error()))
			if got := nextMovesEvents(run.rec); len(got) != 0 {
				t.Errorf("next moves = %+v, want no saved file named", got)
			}
		})
	}
}

// failsAfterTheFirstMerge answers the first merge regenMergeBody and fails every later
// one with the login failure.
func failsAfterTheFirstMerge() func(context.Context) (string, error) {
	var mu sync.Mutex
	var merges int
	return func(context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		merges++
		if merges == 1 {
			return regenMergeBody, nil
		}
		return "", failsWithLogin()
	}
}

func TestRegenerateRun_RegenFailsOnChunkedNotesAnEarlierRegenWrote_SavesThatRegensPartNotes(t *testing.T) {
	t.Parallel()

	ladderThenReMerge := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceRegen},
		NextLines:   []string{exportNudge, exportNudge},
	}
	ladderThenClosed := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceRegen},
		AskLineResult: func() func(string) (string, error) {
			var asked int
			return func(string) (string, error) {
				asked++
				if asked == 1 {
					return exportNudge, nil
				}
				return "", presenter.ErrInputClosed
			}
		}(),
	}
	for _, tc := range []struct {
		name string
		rec  *presentertest.RecordingPresenter
	}{
		{name: "the second r's re-merge fails", rec: ladderThenReMerge},
		{name: "stdin closes at the second r's nudge", rec: ladderThenClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newRegenFailRun(t, tc.rec)
			tr := &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, classify: sizeForOverflow, merge: failsAfterTheFirstMerge()}

			assertAbortExitsOne(t, run.regenerate(t, singlePassFresh(), tr, freshCfg()))

			path := run.savedPath()
			want := strings.Join([]string{regenPartNotes(1, 4), regenPartNotes(2, 4), regenPartNotes(3, 4), regenPartNotes(4, 4)}, "\n\n")
			if got := readSaved(t, path); got != want {
				t.Errorf("saved chunk notes = %q, want the earlier r's part notes unmarked %q", got, want)
			}
			assertNamedWithRegenerate(t, run.rec, regenRunVersionKey, path)
			run.assertWroteNothing(t)
		})
	}
}
