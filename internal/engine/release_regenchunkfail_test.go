package engine_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// fourPartNudge leaves a chunk call of a nearCapPromptBytes release over fourPartPaths
// room for one file but not two, so the `r` carrying it goes as four parts.
var fourPartNudge = strings.Repeat("lead with the export feature. ", 30_000)

// chunkedRegenTransport answers a release's first attempt firstSinglePass, and the
// calls of its `r`, those carrying nudge: the single-pass call through singlePass,
// each chunk call through part, handed its part number and count, and the merge
// through merge. Unset, they answer regenSinglePass, the part's notes and
// regenMergeBody. A classify call is answered classify of its prompt.
type chunkedRegenTransport struct {
	nudge      string
	singlePass func(ctx context.Context) (string, error)
	part       func(ctx context.Context, part, parts int) (string, error)
	merge      func(ctx context.Context) (string, error)
	classify   func(prompt string) string

	mu       sync.Mutex
	calls    int
	classifs int
}

func (ct *chunkedRegenTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	ct.mu.Lock()
	ct.calls++
	ct.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		ct.mu.Lock()
		ct.classifs++
		ct.mu.Unlock()
		return ct.classify(prompt), nil
	case !strings.HasPrefix(prompt, nudgedInstructions(ct.nudge)):
		return firstSinglePass, nil
	case partFraming.MatchString(prompt):
		m := partFraming.FindStringSubmatch(prompt)
		var part, parts int
		_, _ = fmt.Sscan(m[1], &part)
		_, _ = fmt.Sscan(m[2], &parts)
		if ct.part == nil {
			return regenPartNotes(part, parts), nil
		}
		return ct.part(ctx, part, parts)
	case strings.Contains(prompt, "diff --git "):
		if ct.singlePass == nil {
			return regenSinglePass, nil
		}
		return ct.singlePass(ctx)
	default:
		if ct.merge == nil {
			return regenMergeBody, nil
		}
		return ct.merge(ctx)
	}
}

func (ct *chunkedRegenTransport) aiCalls() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.calls
}

func regenPartNotes(part, parts int) string {
	return fmt.Sprintf("- r notes for part %d of %d", part, parts)
}

// partFails is a chunk answer failing part failed with err, every other part with
// its notes.
func partFails(failed int, err error) func(context.Context, int, int) (string, error) {
	return func(_ context.Context, part, parts int) (string, error) {
		if part == failed {
			return "", err
		}
		return regenPartNotes(part, parts), nil
	}
}

func answers(answer string) func(string) string {
	return func(string) string { return answer }
}

// sizeForOverflow answers SIZE for a classify call reading regenOverflowOutput, else
// OTHER.
func sizeForOverflow(prompt string) string {
	if strings.Contains(prompt, regenOverflowOutput) {
		return "SIZE"
	}
	return "OTHER"
}

func overflows(context.Context) (string, error) {
	return "", &ai.GenerationError{Stdout: regenOverflowOutput + "\n", ExitCode: 1}
}

func failsWithLogin() error { return &ai.GenerationError{Stdout: loginOutput, ExitCode: 1} }

// chunkedRegenRun is an attended release whose pre_tag hook made the artifact commit,
// of single-pass notes whose review gate takes one `r` over tr, its chunk notes saving
// under a user cache dir of its own.
type chunkedRegenRun struct {
	root, userCache string
	f               *runner.FakeRunner
	rec             *presentertest.RecordingPresenter
	tr              *chunkedRegenTransport
	deps            engine.ReleaseDeps
}

func newChunkedRegenRun(t *testing.T, config string, release nearCapRelease, tr *chunkedRegenTransport) *chunkedRegenRun {
	t.Helper()
	root, f := gateAbortRelease(t, config+preTagHookConfig, func(f *runner.FakeRunner) {
		release.seedGeneration(f)
		release.seedGeneration(f)
	})
	run := &chunkedRegenRun{root: root, userCache: t.TempDir(), f: f, rec: regenOnce(tr.nudge), tr: tr}
	run.deps = attendedDeps(run.rec, f)
	run.deps.Transport = tr
	run.deps.ChunkNotes = notescache.NewChunkNotes(chunkNotesBaseIn(run.userCache))
	return run
}

func (run *chunkedRegenRun) release(ctx context.Context) error {
	return engine.Release(ctx, run.deps, priorTagNormalAIOptions())
}

func (run *chunkedRegenRun) savedPath() string {
	return chunkNotesPathIn(run.userCache, run.root, nextTag)
}

// assertNamesNoFile fails unless the run saved no chunk notes and its failure names
// none.
func (run *chunkedRegenRun) assertNamesNoFile(t *testing.T) {
	t.Helper()
	assertNoSavedChunkNotes(t, chunkNotesBaseIn(run.userCache))
	for _, e := range run.rec.Events {
		if e.Kind == presentertest.KindStageFailed && e.StageFailed.SavedNotes != "" {
			t.Errorf("StageFailure.SavedNotes = %q, want no saved file named", e.StageFailed.SavedNotes)
		}
	}
	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("next moves = %+v, want no saved file named", got)
	}
}

// assertSavedAs fails unless the file the run saved holds want and its ✗ line and
// next moves name it.
func (run *chunkedRegenRun) assertSavedAs(t *testing.T, want string) {
	t.Helper()
	path := run.savedPath()
	if got := readSaved(t, path); got != want {
		t.Errorf("saved chunk notes = %q, want %q", got, want)
	}
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
	}
	assertSavedAndNamedBeforeUnwind(t, run.rec, path)
}

func assertSavedAndNamedBeforeUnwind(t *testing.T, rec *presentertest.RecordingPresenter, path string) {
	t.Helper()
	if got, want := nextMovesEvents(rec), releaseSavedNotesMoves(path); len(got) != 1 || !nextMovesEqual(got[0], want[0]) {
		t.Errorf("next moves = %+v, want %+v", got, want)
	}
	if moves, unwound := indexOfKind(rec, presentertest.KindNextMoves), indexOfKind(rec, presentertest.KindUnwound); moves > unwound {
		t.Errorf("event kinds = %v, want the saved file named before the unwind", rec.Kinds())
	}
}

func notesMissingFor(path string) string {
	return "**Notes missing:** no notes were written for `" + path + "` — add them before releasing."
}

func TestRelease_RegenOverTheCapWhosePartFailsForGood_SavesTheOtherPartsNotesAndNamesTheFile(t *testing.T) {
	t.Parallel()

	release := newNearCapRelease(t, fourPartPaths, nearCapPromptBytes)
	run := newChunkedRegenRun(t, "", release, &chunkedRegenTransport{nudge: fourPartNudge, part: partFails(3, failsWithLogin()), classify: answers("OTHER")})

	assertAbortNonZero(t, run.release(t.Context()))

	want := presenter.StageFailure{Name: "regenerate", Key: "notes", Message: "AI call failed (part 3 of 4)", Output: loginOutput, SavedNotes: run.savedPath()}
	if sf := untimed(onlyStageFailureEvent(t, run.rec)); sf != want {
		t.Errorf("StageFailure = %+v, want %+v", sf, want)
	}
	run.assertSavedAs(t, notesMissingFor("c/3.go")+"\n\n"+regenPartNotes(1, 4)+"\n\n"+regenPartNotes(2, 4)+"\n\n"+regenPartNotes(4, 4))
}

func TestRelease_RegenHalvedWhosePartFailsForGood_SavesTheOtherPartsNotesAndNamesTheFile(t *testing.T) {
	t.Parallel()

	release := newNearCapRelease(t, threeFilePaths, 400_000)
	run := newChunkedRegenRun(t, "", release, &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, part: partFails(2, failsWithLogin()), classify: sizeForOverflow})

	assertAbortNonZero(t, run.release(t.Context()))

	if sf := onlyStageFailureEvent(t, run.rec); sf.Message != "AI call failed (part 2 of 3)" || sf.Output != loginOutput {
		t.Errorf("✗ line = %q with output %q, want %q with part 2's output %q", sf.Message, sf.Output, "AI call failed (part 2 of 3)", loginOutput)
	}
	run.assertSavedAs(t, notesMissingFor("core/b.go")+"\n\n"+regenPartNotes(1, 3)+"\n\n"+regenPartNotes(3, 3))
}

func TestRelease_ChunkedRegenWhoseMergeFails_SavesEveryPartsNotesWithNoMarker(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		release func(t *testing.T) nearCapRelease
		tr      *chunkedRegenTransport
		parts   int
	}{
		{
			name:    "over the cap",
			release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, fourPartPaths, nearCapPromptBytes) },
			tr:      &chunkedRegenTransport{nudge: fourPartNudge, classify: answers("OTHER")},
			parts:   4,
		},
		{
			name:    "halved",
			release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, threeFilePaths, 400_000) },
			tr:      &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, classify: sizeForOverflow},
			parts:   3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.tr.merge = func(context.Context) (string, error) { return "", failsWithLogin() }
			run := newChunkedRegenRun(t, "", tc.release(t), tc.tr)

			assertAbortNonZero(t, run.release(t.Context()))

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != "AI call failed (merging)" || sf.Output != loginOutput {
				t.Errorf("✗ line = %q with output %q, want %q with the merge's output %q", sf.Message, sf.Output, "AI call failed (merging)", loginOutput)
			}
			var want []string
			for part := 1; part <= tc.parts; part++ {
				want = append(want, regenPartNotes(part, tc.parts))
			}
			run.assertSavedAs(t, strings.Join(want, "\n\n"))
		})
	}
}

func TestRelease_ChunkedRegenWhoseWaveFailsBeforeAnyPartFinishes_SavesAndNamesNoFile(t *testing.T) {
	t.Parallel()

	release := newNearCapRelease(t, fourPartPaths, nearCapPromptBytes)
	everyPartFails := func(context.Context, int, int) (string, error) { return "", failsWithLogin() }
	run := newChunkedRegenRun(t, "", release, &chunkedRegenTransport{nudge: fourPartNudge, part: everyPartFails, classify: answers("OTHER")})

	assertAbortNonZero(t, run.release(t.Context()))

	onlyStageFailureEvent(t, run.rec)
	run.assertNamesNoFile(t)
}

func TestRelease_SinglePassRegenFailingWithoutChunking_SavesAndNamesNoFile(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		failure error
		message string
	}{
		{name: "classified OTHER", failure: failsWithLogin(), message: "AI call failed"},
		{name: "timed out", failure: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), message: "AI timed out"},
		{name: "program not installed", failure: fmt.Errorf("%w: claude", ai.ErrCommandMissing), message: "AI tool not installed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			release := newNearCapRelease(t, threeFilePaths, 400_000)
			failure := tc.failure
			run := newChunkedRegenRun(t, "", release, &chunkedRegenTransport{
				nudge:      exportNudge,
				singlePass: func(context.Context) (string, error) { return "", failure },
				classify:   answers("OTHER"),
			})

			assertAbortNonZero(t, run.release(t.Context()))

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != tc.message {
				t.Errorf("✗ line = %q, want %q", sf.Message, tc.message)
			}
			run.assertNamesNoFile(t)
		})
	}
}

func TestRelease_FailedRegenTheLadderMadeChunkedOrNot_AbortsBeforeAnyTagWhateverOnNotesFailureHolds(t *testing.T) {
	t.Parallel()

	for _, config := range []string{"", "[release]\non_notes_failure = \"abort\"\n\n", "[release]\non_notes_failure = \"fallback\"\n\n"} {
		for _, tc := range []struct {
			name    string
			release func(t *testing.T) nearCapRelease
			tr      func() *chunkedRegenTransport
		}{
			{
				name:    "a part of an r over the cap fails",
				release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, fourPartPaths, nearCapPromptBytes) },
				tr: func() *chunkedRegenTransport {
					return &chunkedRegenTransport{nudge: fourPartNudge, part: partFails(3, failsWithLogin()), classify: answers("OTHER")}
				},
			},
			{
				name:    "a part of a halved r fails",
				release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, threeFilePaths, 400_000) },
				tr: func() *chunkedRegenTransport {
					return &chunkedRegenTransport{nudge: exportNudge, singlePass: overflows, part: partFails(2, failsWithLogin()), classify: sizeForOverflow}
				},
			},
			{
				name:    "a chunked r's merge fails",
				release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, fourPartPaths, nearCapPromptBytes) },
				tr: func() *chunkedRegenTransport {
					return &chunkedRegenTransport{nudge: fourPartNudge, merge: func(context.Context) (string, error) { return "", failsWithLogin() }, classify: answers("OTHER")}
				},
			},
			{
				name:    "a single-pass r fails",
				release: func(t *testing.T) nearCapRelease { return newNearCapRelease(t, threeFilePaths, 400_000) },
				tr: func() *chunkedRegenTransport {
					return &chunkedRegenTransport{nudge: exportNudge, singlePass: func(context.Context) (string, error) { return "", failsWithLogin() }, classify: answers("OTHER")}
				},
			},
		} {
			t.Run(fmt.Sprintf("%s %q", tc.name, config), func(t *testing.T) {
				t.Parallel()

				run := newChunkedRegenRun(t, config, tc.release(t), tc.tr())

				err := run.release(t.Context())

				assertUnwoundToStart(t, run.rec, run.f, err)
				var abort *engine.AbortError
				if errors.As(err, &abort) && abort.ExitCode != 1 {
					t.Errorf("abort ExitCode = %d, want 1", abort.ExitCode)
				}
				if gates := floorGates(run.rec); len(gates) != 0 {
					t.Errorf("a failed r was offered the c / e / a choice: %+v", gates)
				}
				if countCmd(run.f, "git", "log", "--format=%s", priorTag+"..HEAD") != 0 {
					t.Errorf("a failed r built the commit-subject list")
				}
			})
		}
	}
}

func TestRelease_CtrlCDuringAChunkedRegensCalls_AbortsAsACancelSavingNothing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var others sync.WaitGroup
	others.Add(3)
	var cancelled sync.Once
	var callsAtCancel int
	tr := &chunkedRegenTransport{nudge: fourPartNudge, classify: answers("OTHER")}
	tr.part = func(ctx context.Context, part, parts int) (string, error) {
		if part != 3 {
			others.Done()
			return regenPartNotes(part, parts), nil
		}
		others.Wait()
		cancelled.Do(func() {
			callsAtCancel = tr.aiCalls()
			cancel()
		})
		return "", ctx.Err()
	}
	run := newChunkedRegenRun(t, "[release]\non_notes_failure = \"fallback\"\n\n", newNearCapRelease(t, fourPartPaths, nearCapPromptBytes), tr)

	err := run.release(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if got := tr.aiCalls(); got != callsAtCancel {
		t.Errorf("AI calls = %d, want none after the cancel's %d", got, callsAtCancel)
	}
	run.assertNamesNoFile(t)
	if countCmd(run.f, "git", "log", "--format=%s", priorTag+"..HEAD") != 0 {
		t.Errorf("a cancelled r built the commit-subject list")
	}
	assertUnwoundToStart(t, run.rec, run.f, err)
}

func TestRelease_ChunkedRegensNotesCannotBeSaved_KeepsTheFailureAndWarnsWithTheWriteError(t *testing.T) {
	t.Parallel()

	release := newNearCapRelease(t, fourPartPaths, nearCapPromptBytes)
	run := newChunkedRegenRun(t, "", release, &chunkedRegenTransport{nudge: fourPartNudge, part: partFails(3, failsWithLogin()), classify: answers("OTHER")})
	run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}

	err := run.release(t.Context())

	assertUnwoundToStart(t, run.rec, run.f, err)
	want := presenter.StageFailure{Name: "regenerate", Key: "notes", Message: "AI call failed (part 3 of 4)", Output: loginOutput}
	if sf := untimed(onlyStageFailureEvent(t, run.rec)); sf != want {
		t.Errorf("StageFailure = %+v, want the failure unchanged %+v", sf, want)
	}
	assertWarnedAfterTheFailure(t, run.rec)
	assertOnlyWarning(t, run.rec, unsavedChunkNotesWarning(errChunkNotesWrite.Error()))
	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("next moves = %+v, want no saved file named", got)
	}
}
