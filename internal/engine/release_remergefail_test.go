package engine_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/cmdline"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

var sixPartCapToml = partCapToml + "\n"

// sixPartSaved is what the wave of a six-part run under gateAbortTransport left in
// its parts' notes: each one, in chunk order, a blank line between.
var sixPartSaved = func() string {
	parts := make([]string, len(sixPartPaths))
	for i := range parts {
		parts[i] = fmt.Sprintf("- notes for What follows is part %d of %d", i+1, len(sixPartPaths))
	}
	return strings.Join(parts, "\n\n")
}()

type countedRegen struct {
	regen func(context.Context, string) (string, error)

	mu sync.Mutex
	n  int
}

func (c *countedRegen) call(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return c.regen(ctx, prompt)
}

func (c *countedRegen) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// remergeFailRun is an attended release of six-part chunked notes whose pre_tag hook
// made the artifact commit, whose chunk notes save under a user cache dir of its own.
type remergeFailRun struct {
	root, userCache string
	f               *runner.FakeRunner
	rec             *presentertest.RecordingPresenter
	tr              *gateAbortTransport
	regen           *countedRegen
	deps            engine.ReleaseDeps
}

func newRemergeFailRun(t *testing.T, config string, rec *presentertest.RecordingPresenter, classify string, regen func(context.Context, string) (string, error)) *remergeFailRun {
	t.Helper()
	root, f := gateAbortRelease(t, sixPartCapToml+config+preTagHookConfig, seedSixPartNotes)
	run := &remergeFailRun{root: root, userCache: t.TempDir(), f: f, rec: rec, regen: &countedRegen{regen: regen}}
	run.tr = &gateAbortTransport{nudge: exportNudge, classify: classify, regen: run.regen.call}
	run.deps = attendedDeps(rec, f)
	run.deps.Transport = run.tr
	run.deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(run.userCache))
	return run
}

func (run *remergeFailRun) release(ctx context.Context) error {
	return engine.Release(ctx, run.deps, priorTagNormalAIOptions())
}

func (run *remergeFailRun) savedPath() string {
	return chunkNotesPathIn(run.userCache, run.root, nextTag)
}

func chunkNotesBaseIn(userCache string) string {
	return filepath.Join(userCache, "mint", "chunk-notes")
}

// chunkNotesPathIn is where the chunk notes of root's tag save under userCache: the
// repo path, its separators as dashes, under the chunk-notes base.
func chunkNotesPathIn(userCache, root, tag string) string {
	scope := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	return filepath.Join(chunkNotesBaseIn(userCache), scope, tag+".md")
}

// releaseSavedNotesMoves is the failure's naming of the file at path with the command
// that releases it.
func releaseSavedNotesMoves(path string) []presenter.NextMoves {
	return []presenter.NextMoves{{
		Stage: "regenerate",
		Moves: []presenter.NextMove{{
			Lead:    "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with",
			Command: "mint release --notes-file " + cmdline.Quote(path),
		}},
	}}
}

// assertSavedAndNamed fails unless the six parts' notes sit in the file at path and
// the next moves name it with the release command, ahead of the unwind.
func assertSavedAndNamed(t *testing.T, rec *presentertest.RecordingPresenter, path string) {
	t.Helper()
	if got := readSaved(t, path); got != sixPartSaved {
		t.Errorf("saved chunk notes = %q, want every part's notes %q", got, sixPartSaved)
	}
	if got, want := nextMovesEvents(rec), releaseSavedNotesMoves(path); !slices.EqualFunc(got, want, nextMovesEqual) {
		t.Errorf("next moves = %+v, want %+v", got, want)
	}
	if moves, unwound := indexOfKind(rec, presentertest.KindNextMoves), indexOfKind(rec, presentertest.KindUnwound); moves > unwound {
		t.Errorf("event kinds = %v, want the saved file named before the unwind", rec.Kinds())
	}
}

func nextMovesEqual(a, b presenter.NextMoves) bool {
	return a.Stage == b.Stage && slices.Equal(a.Moves, b.Moves)
}

func TestRelease_ReMergeFailsWithOutput_ShowsTheMergesFailureAndSavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	run := newRemergeFailRun(t, "", regenOnce(exportNudge), "OTHER", failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))

	assertAbortNonZero(t, run.release(t.Context()))

	sf := onlyStageFailureEvent(t, run.rec)
	if sf.Message != "AI call failed (merging)" || sf.Output != loginOutput {
		t.Errorf("✗ line = %q with output %q, want %q with the re-merge's output %q verbatim", sf.Message, sf.Output, "AI call failed (merging)", loginOutput)
	}
	if sf.SavedNotes != run.savedPath() {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, run.savedPath())
	}
	assertSavedAndNamed(t, run.rec, run.savedPath())
}

func TestRelease_ReMergeTimesOut_ShowsTheTimeoutAtTheMergeAndSavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	run := newRemergeFailRun(t, "", regenOnce(exportNudge), "OTHER", failsWith(fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)))

	assertAbortNonZero(t, run.release(t.Context()))

	if sf := onlyStageFailureEvent(t, run.rec); sf.Message != "AI timed out (merging)" || sf.SavedNotes != run.savedPath() {
		t.Errorf("✗ line = %q saving %q, want %q saving %q", sf.Message, sf.SavedNotes, "AI timed out (merging)", run.savedPath())
	}
	assertSavedAndNamed(t, run.rec, run.savedPath())
}

func TestRelease_ReMergeTooLarge_FailsAsPartNotesTooLargeToMergeAndSavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	overCapNudge := strings.Repeat("lead with the export feature ", 1_000)
	for _, tc := range []struct {
		name      string
		nudge     string
		classify  string
		wantCalls int
	}{
		{name: "the nudge brings the merge prompt over the cap", nudge: overCapNudge, classify: "OTHER", wantCalls: 0},
		{name: "the re-merge call is classified SIZE", nudge: exportNudge, classify: "SIZE", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newRemergeFailRun(t, "", regenOnce(tc.nudge), tc.classify, failsWith(&ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}))
			run.tr.nudge = tc.nudge

			assertAbortNonZero(t, run.release(t.Context()))

			if got := run.regen.calls(); got != tc.wantCalls {
				t.Errorf("re-merge calls = %d, want %d", got, tc.wantCalls)
			}
			if run.tr.classifs > tc.wantCalls {
				t.Errorf("classify calls = %d, want no more than the re-merge's", run.tr.classifs)
			}
			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != "part notes too large to merge" || sf.SavedNotes != run.savedPath() {
				t.Errorf("✗ line = %q saving %q, want %q saving %q", sf.Message, sf.SavedNotes, "part notes too large to merge", run.savedPath())
			}
			assertSavedAndNamed(t, run.rec, run.savedPath())
		})
	}
}

func TestRelease_StdinClosedAtTheNudgeOnChunkedNotes_MakesNoCallAndSavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices:   []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
	}
	run := newRemergeFailRun(t, "", rec, "OTHER", failsWith(errors.New("unexpected re-merge")))

	err := run.release(t.Context())

	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	if want := len(sixPartPaths) + 1; run.tr.calls != want {
		t.Errorf("AI calls = %d, want only the first run's six chunk calls and its merge (%d)", run.tr.calls, want)
	}
	assertSavedAndNamed(t, run.rec, run.savedPath())
	assertUnwoundToStart(t, run.rec, run.f, err)
}

func TestRelease_FailedReMerge_UnwindsToTheStartLeavingTheSavedFile(t *testing.T) {
	t.Parallel()

	closedAtNudge := func() *presentertest.RecordingPresenter {
		return &presentertest.RecordingPresenter{
			NextChoices:   []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
			AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
		}
	}
	failing := func() *presentertest.RecordingPresenter { return regenOnce(exportNudge) }
	for _, config := range []string{"", "[release]\non_notes_failure = \"fallback\"\n\n"} {
		for _, tc := range []struct {
			name     string
			rec      func() *presentertest.RecordingPresenter
			classify string
			regen    func(context.Context, string) (string, error)
		}{
			{name: "fails with output", rec: failing, classify: "OTHER", regen: failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1})},
			{name: "times out", rec: failing, classify: "OTHER", regen: failsWith(fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout))},
			{name: "is classified SIZE", rec: failing, classify: "SIZE", regen: failsWith(&ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1})},
			{name: "stdin closes at the nudge", rec: closedAtNudge, classify: "OTHER", regen: failsWith(errors.New("unexpected re-merge"))},
		} {
			t.Run(fmt.Sprintf("%s %q", tc.name, config), func(t *testing.T) {
				t.Parallel()

				run := newRemergeFailRun(t, config, tc.rec(), tc.classify, tc.regen)

				err := run.release(t.Context())

				assertUnwoundToStart(t, run.rec, run.f, err)
				if got := readSaved(t, run.savedPath()); got != sixPartSaved {
					t.Errorf("saved chunk notes after the unwind = %q, want %q", got, sixPartSaved)
				}
				if countCmd(run.f, "git", "log", "--format=%s", priorTag+"..HEAD") != 0 {
					t.Errorf("a failed r built the commit-subject list")
				}
				if gates := floorGates(run.rec); len(gates) != 0 {
					t.Errorf("a failed r was offered the c / e / a choice: %+v", gates)
				}
				if got := warnMessages(run.rec); len(got) != 0 {
					t.Errorf("warnings = %q, want none", got)
				}
			})
		}
	}
}

func TestRelease_ReMergeFailsOnNotesReusedFromTheCache_SavesTheCachedPartNotes(t *testing.T) {
	t.Parallel()

	const dryRunParts = "- dry notes for part 1 of 4\n\n- dry notes for part 2 of 4\n\n- dry notes for part 3 of 4\n\n- dry notes for part 4 of 4"
	const abortedRunParts = "- aborted notes for part 1 of 4\n\n- aborted notes for part 2 of 4\n\n- aborted notes for part 3 of 4\n\n- aborted notes for part 4 of 4"
	for _, tc := range []struct {
		name  string
		cache func(t *testing.T, repo chunkedCacheRepo, diff string)
		want  string
	}{
		{
			name: "a dry run cached them",
			cache: func(t *testing.T, repo chunkedCacheRepo, diff string) {
				repo.dryRun(t, diff, &mergeTransport{label: "dry", firstMerge: fourPartMergeBody})
			},
			want: dryRunParts,
		},
		{
			name: "an earlier real run aborted at the review gate cached them",
			cache: func(t *testing.T, repo chunkedCacheRepo, diff string) {
				f := runner.NewFakeRunner()
				seedPriorTagReadGates(f, repo.root, "main")
				seedFourPartNotes(f, diff)
				rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}}
				tr := &mergeTransport{label: "aborted", firstMerge: fourPartMergeBody}
				assertAbortNonZero(t, engine.Release(t.Context(), repo.deps(rec, f, tr, cacheWriteClock), priorTagNormalAIOptions()))
			},
			want: abortedRunParts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newChunkedCacheRepo(t)
			diff := fourPartDiff("x")
			tc.cache(t, repo, diff)
			f := runner.NewFakeRunner()
			seedRealRunReuseGit(f, repo.root, diff)
			rec := &presentertest.RecordingPresenter{
				NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceRegen},
				NextLines:   []string{exportNudge},
			}
			userCache := t.TempDir()
			tr := &gateAbortTransport{nudge: exportNudge, classify: "OTHER", regen: failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1})}
			deps := repo.deps(rec, f, tr, realRunClock)
			deps.StdinInteractive = true
			deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(userCache))

			assertAbortNonZero(t, engine.Release(t.Context(), deps, priorTagNormalAIOptions()))

			path := chunkNotesPathIn(userCache, repo.root, nextTag)
			if got := readSaved(t, path); got != tc.want {
				t.Errorf("saved chunk notes = %q, want the cached part notes %q", got, tc.want)
			}
			if sf := onlyStageFailureEvent(t, rec); sf.SavedNotes != path {
				t.Errorf("StageFailure.SavedNotes = %q, want %q", sf.SavedNotes, path)
			}
			if got, want := nextMovesEvents(rec), releaseSavedNotesMoves(path); !slices.EqualFunc(got, want, nextMovesEqual) {
				t.Errorf("next moves = %+v, want %+v", got, want)
			}
		})
	}
}

func TestRelease_FailedReMergesNotesCannotBeSaved_KeepsTheFailureAndWarnsWithTheWriteError(t *testing.T) {
	t.Parallel()

	closedAtNudge := &presentertest.RecordingPresenter{
		NextChoices:   []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) { return "", presenter.ErrInputClosed },
	}
	for _, tc := range []struct {
		name string
		rec  *presentertest.RecordingPresenter
		// failed is the ✗ line the failure shows, or nil for none.
		failed *presenter.StageFailure
	}{
		{
			name:   "the re-merge fails with output",
			rec:    regenOnce(exportNudge),
			failed: &presenter.StageFailure{Name: "regenerate", Key: "notes", Message: "AI call failed (merging)", Output: loginOutput},
		},
		{name: "stdin closes at the nudge", rec: closedAtNudge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newRemergeFailRun(t, "", tc.rec, "OTHER", failsWith(&ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))
			run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}

			err := run.release(t.Context())

			assertUnwoundToStart(t, run.rec, run.f, err)
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

func TestRelease_CtrlCDuringTheReMerge_AbortsAsACancelAndSavesNoFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	run := newRemergeFailRun(t, "", regenOnce(exportNudge), "OTHER", func(ctx context.Context, _ string) (string, error) {
		cancel()
		return "", ctx.Err()
	})

	err := run.release(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	assertNoSavedChunkNotes(t, chunkNotesBaseIn(run.userCache))
	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("next moves = %+v, want none", got)
	}
}
