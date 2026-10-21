package engine_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	regenFloorVersion   = "1.2.0"
	regenWriteNotesMove = "write the notes to a file and release with mint release regenerate " + regenFloorVersion + " --notes-file <file>"
)

func regenTidySavedNotesMove(path string) string {
	return "tidy the saved notes at " + path + ", filling in any files its marker lists, and release with mint release regenerate " + regenFloorVersion + " --notes-file " + path
}

// unattendedModes are the two ways a single-version regenerate runs with nobody to
// answer the floor's choice.
var unattendedModes = []struct {
	name             string
	yes              bool
	stdinInteractive bool
}{
	{name: "-y at a terminal", yes: true, stdinInteractive: true},
	{name: "stdin not a terminal, no -y"},
}

func newUnattendedRegenFloor(t *testing.T, yes, stdinInteractive bool) *regenChunkNotes {
	t.Helper()
	run := newRegenChunkNotes(t)
	run.deps.Yes = yes
	run.deps.StdinInteractive = stdinInteractive
	return run
}

func (run *regenChunkNotes) regenerateUnattended(ctx context.Context, cfg config.Config, produce freshProducer) error {
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.Tag = regenFloorTag
	req.VersionKey = regenFloorVersion
	req.DiffRange = regenFloorRange
	req.Config = cfg
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return produce(ctx, version.Resolution{Tag: regenFloorTag, PreviousTag: "v1.1.0"}, sinks)
	}
	return engine.RegenerateRun(ctx, run.deps, run.pub, run.root, req)
}

// assertRegenStopsWithNextMoves checks the run printed want as the notes stage's only
// next moves, after its ✗ line and followed by nothing, asked nothing, wrote nothing,
// named no forward-release-only move, and exited 1.
func assertRegenStopsWithNextMoves(t *testing.T, run *regenChunkNotes, err error, want []string) {
	t.Helper()
	assertExitsOne(t, err)
	got := nextMovesEvents(run.rec)
	if len(got) != 1 {
		t.Fatalf("NextMoves events = %+v, want one; kinds = %v", got, run.rec.Kinds())
	}
	if got[0].Stage != "notes" || !slices.Equal(got[0].Moves, want) {
		t.Errorf("NextMoves = %q %q, want notes %q", got[0].Stage, got[0].Moves, want)
	}
	for _, move := range got[0].Moves {
		if strings.Contains(move, "--no-ai") || strings.Contains(move, "on_notes_failure") {
			t.Errorf("move %q names a forward-release-only way out", move)
		}
	}
	failedAt := indexOfKind(run.rec, presentertest.KindStageFailed)
	movesAt := indexOfKind(run.rec, presentertest.KindNextMoves)
	if failedAt == -1 || failedAt >= movesAt {
		t.Errorf("want the next moves after the ✗ line; kinds = %v", run.rec.Kinds())
	}
	if movesAt != len(run.rec.Events)-1 {
		t.Errorf("want nothing after the next moves; kinds = %v", run.rec.Kinds())
	}
	if gates := promptGates(run.rec); len(gates) != 0 {
		t.Errorf("an unattended run showed prompts: %+v", gates)
	}
	assertNothingWritten(t, run)
}

func TestRegenerateRun_NotesFloorNextMoves_UnattendedFailurePrintsThemBeneathTheOutput(t *testing.T) {
	t.Parallel()

	for _, mode := range unattendedModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, mode.yes, mode.stdinInteractive)

			err := run.regenerateUnattended(t.Context(), freshCfg(), singlePassFailsUnder(""))

			sf := onlyStageFailureEvent(t, run.rec)
			if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != usageLimitOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", usageLimitOutput)
			}
			assertRegenStopsWithNextMoves(t, run, err, []string{regenWriteNotesMove})
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_FallbackStopsTheSameWayWithoutTheCommitSubjects(t *testing.T) {
	t.Parallel()

	for _, mode := range unattendedModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, mode.yes, mode.stdinInteractive)
			run.seedSubjects()
			cfg := freshCfg()
			cfg.Release.OnNotesFailure = "fallback"

			err := run.regenerateUnattended(t.Context(), cfg, singlePassFailsUnder("fallback"))

			assertRegenStopsWithNextMoves(t, run, err, []string{regenWriteNotesMove})
			if invokedWith(run.deps.Runner.(*runner.FakeRunner), "git", "log", "--format=%s", regenFloorRange) {
				t.Errorf("the commit-subject list was built for a stopped regenerate")
			}
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_SavedChunkNotesAreTheNotesFileRouteForThisVersion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		fail func(prompt string) error
	}{
		{name: "a part failed", fail: failsSecondPart},
		{name: "the merge failed", fail: failsMerge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, true, true)

			err := run.regenerateUnattended(t.Context(), freshCfg(), overCapFailing(tc.fail))

			assertRegenStopsWithNextMoves(t, run, err, []string{regenTidySavedNotesMove(run.savedPath(regenFloorTag))})
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_NoSavedFileNamesNoPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		chunkNotes engine.ChunkNotesSaver
		produce    freshProducer
		warnings   int
	}{
		{name: "a single-pass call failed", produce: singlePassFails},
		{name: "the wave finished no part", produce: overCapFailing(failsEveryChunk)},
		{name: "the save failed", chunkNotes: failingChunkNotes{err: errChunkNotesWrite}, produce: overCapFailing(failsSecondPart), warnings: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, true, true)
			if tc.chunkNotes != nil {
				run.deps.ChunkNotes = tc.chunkNotes
			}

			err := run.regenerateUnattended(t.Context(), freshCfg(), tc.produce)

			assertRegenStopsWithNextMoves(t, run, err, []string{regenWriteNotesMove})
			if got := len(warnings(run.rec)); got != tc.warnings {
				t.Errorf("warnings = %d, want %d", got, tc.warnings)
			}
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_TimeoutAddsRaiseTheReleaseTimeout(t *testing.T) {
	t.Parallel()

	run := newUnattendedRegenFloor(t, true, true)
	timesOut := func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		return underCapFresh(ctx, func(context.Context, string) (string, error) { return "", errTimedOut }, res, sinks)
	}

	err := run.regenerateUnattended(t.Context(), freshCfg(), timesOut)

	if sf := onlyStageFailureEvent(t, run.rec); sf.Message != timeoutPhrase {
		t.Errorf("StageFailure.Message = %q, want %q", sf.Message, timeoutPhrase)
	}
	assertRegenStopsWithNextMoves(t, run, err, []string{regenWriteNotesMove, raiseTimeoutMove})
}

func TestRegenerateRun_NotesFloorNextMoves_HalvingRunsOutAddsTheSizeMoves(t *testing.T) {
	t.Parallel()

	for _, tc := range halvingOutCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, true, true)
			tr := &firstPartOverflowTransport{}
			halvesOut := func(ctx context.Context, res version.Resolution, _ engine.NotesSinks) (string, error) {
				return tc.produce(ctx, t, tr, res)
			}

			err := run.regenerateUnattended(t.Context(), freshCfg(), halvesOut)

			if sf := onlyStageFailureEvent(t, run.rec); sf.Message != tc.wantPhrase(t, tr) {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.wantPhrase(t, tr))
			}
			assertRegenStopsWithNextMoves(t, run, err, withSizeMoves(regenWriteNotesMove))
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_OtherFailureWithTheRepoOwnReleaseCommandAddsTheLeanCommandPointer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		aiCommand string
		want      []string
	}{
		{name: "the shipped command", want: []string{regenWriteNotesMove}},
		{name: "the repo's own command", aiCommand: "mytool --print", want: []string{regenWriteNotesMove, leanCommandMove}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newUnattendedRegenFloor(t, true, true)
			cfg := freshCfg()
			cfg.AICommand = tc.aiCommand

			err := run.regenerateUnattended(t.Context(), cfg, singlePassFailsUnder(""))

			assertRegenStopsWithNextMoves(t, run, err, tc.want)
		})
	}
}

func TestRegenerateRun_NotesFloorNextMoves_CtrlCPrintsNone(t *testing.T) {
	t.Parallel()

	run := newUnattendedRegenFloor(t, true, true)
	ctx, cancel := context.WithCancel(t.Context())
	cancelled := func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		return underCapFresh(ctx, func(ctx context.Context, _ string) (string, error) {
			cancel()
			return "", ctx.Err()
		}, res, sinks)
	}

	if err := run.regenerateUnattended(ctx, freshCfg(), cancelled); err == nil {
		t.Fatal("RegenerateRun returned nil error, want a cancel")
	}

	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("a cancelled regenerate printed next moves: %+v", got)
	}
}

func TestRegenerateRun_NotesFloorNextMoves_AttendedRunGetsTheChoiceInstead(t *testing.T) {
	t.Parallel()

	run := newAttendedRegenFloor(t, &fakeEditor{})

	assertExitsOne(t, run.regenerateAttended(t.Context(), singlePassFailsUnder("")))

	if got := nextMovesEvents(run.rec); len(got) != 0 {
		t.Errorf("an attended regenerate printed next moves: %+v", got)
	}
	if got := len(floorGates(run.rec)); got != 1 {
		t.Errorf("floor gates = %d, want 1", got)
	}
}
