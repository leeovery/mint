package engine_test

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// regenChunkNotes is a regenerate whose chunk notes save under a user cache dir of its
// own.
type regenChunkNotes struct {
	root, userCache string
	rec             *presentertest.RecordingPresenter
	pub             *fakePublisher
	deps            engine.ReleaseDeps
}

func newRegenChunkNotes(t *testing.T) *regenChunkNotes {
	t.Helper()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	run := &regenChunkNotes{root: t.TempDir(), userCache: t.TempDir(), rec: &presentertest.RecordingPresenter{}, pub: newFakePublisher()}
	run.deps = regenWriteDeps(run.rec, f)
	run.deps.ChunkNotes = notescache.NewChunkNotes(run.base())
	return run
}

func (run *regenChunkNotes) base() string {
	return filepath.Join(run.userCache, "mint", "chunk-notes")
}

func (run *regenChunkNotes) savedPath(tag string) string {
	scope := strings.ReplaceAll(filepath.Clean(run.root), string(filepath.Separator), "-")
	return filepath.Join(run.base(), scope, tag+".md")
}

// overCapFresh produces res's fresh notes over a two-file range diff that chunks into
// two parts under partCapBytes, answered by transport.
func overCapFresh(ctx context.Context, transport func(context.Context, string) (string, error), res version.Resolution, sinks engine.NotesSinks) (string, error) {
	cfg := freshCfg()
	cfg.MaxPromptBytes = partCapBytes
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, transportFunc(transport), "", cfg, res, sinks)
}

// underCapFresh produces res's fresh notes in one call under the default cap,
// answered by transport.
func underCapFresh(ctx context.Context, transport func(context.Context, string) (string, error), res version.Resolution, sinks engine.NotesSinks) (string, error) {
	notesGit := seedFreshGit(twoFileDiff(1_000), "M\tapi/a.go\nM\tcore/b.go\n", "16\t0\tapi/a.go\n16\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(ctx, notesGit, transportFunc(transport), "", freshCfg(), res, sinks)
}

type freshProducer func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error)

func (run *regenChunkNotes) regenerate(t *testing.T, tag string, produce freshProducer) error {
	t.Helper()
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.Tag = tag
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return produce(ctx, version.Resolution{Tag: tag, PreviousTag: "v1.1.0"}, sinks)
	}
	return engine.RegenerateRun(t.Context(), run.deps, run.pub, run.root, req)
}

// regenerateAll backfills v1.1.0, v1.2.0 and v1.3.0, producing each version listed in
// failing through produce and every other version's notes as they are.
func (run *regenChunkNotes) regenerateAll(t *testing.T, produce freshProducer, failing ...string) {
	t.Helper()
	versions := []version.Resolution{
		{Tag: "v1.1.0", PreviousTag: "v1.0.0"},
		{Tag: "v1.2.0", PreviousTag: "v1.1.0"},
		{Tag: "v1.3.0", PreviousTag: "v1.2.0"},
	}
	for _, res := range versions {
		run.pub.seedExists(res.Tag, true, nil)
	}
	req := batchReq(engine.RegenerateSourceFresh, versions, true)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		for _, tag := range failing {
			if res.Tag == tag {
				return produce(ctx, res, sinks)
			}
		}
		return "## fresh " + res.Tag + "\n", nil
	}
	if _, err := engine.RegenerateAll(t.Context(), run.deps, run.pub, run.root, req); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}
}

func overCapFailing(fail func(prompt string) error) freshProducer {
	return func(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
		return overCapFresh(ctx, twoPartTransport(fail), res, sinks)
	}
}

func failsEveryChunk(prompt string) error {
	if isChunkCallPrompt(prompt) {
		return &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
	}
	return nil
}

func singlePassFails(ctx context.Context, res version.Resolution, sinks engine.NotesSinks) (string, error) {
	failsNotesCalls := func(_ context.Context, prompt string) (string, error) {
		if isClassifyPrompt(prompt) {
			return "OTHER", nil
		}
		return "", &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
	}
	return underCapFresh(ctx, failsNotesCalls, res, sinks)
}

// skipWarning is tag's skip after a call that failed with the usage limit, its cause
// phrase phrase.
func skipWarning(tag, phrase string) presenter.Warning {
	return presenter.Warning{Label: "skipped " + tag, Message: phrase, Output: usageLimitOutput}
}

func TestRegenerateRun_FreshChunkedRunFails_SavesTheFinishedNotesUnderTheVersionsTagAndNamesThePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		fail        func(prompt string) error
		wantSaved   string
		wantMessage string
	}{
		{
			name:        "a part fails, so the file opens with the missing-notes marker",
			fail:        failsSecondPart,
			wantSaved:   partOneSaved,
			wantMessage: "AI call failed (part 2 of 2)",
		},
		{
			name:        "the merge fails, so the file holds every part's notes unmarked",
			fail:        failsMerge,
			wantSaved:   apiPartNotes + "\n\n" + corePartNotes,
			wantMessage: "AI call failed (merging)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newRegenChunkNotes(t)

			if err := run.regenerate(t, "v1.2.0", overCapFailing(tc.fail)); err == nil {
				t.Fatal("RegenerateRun returned nil error, want an abort")
			}

			path := run.savedPath("v1.2.0")
			if got := readSaved(t, path); got != tc.wantSaved {
				t.Errorf("saved chunk notes = %q, want %q", got, tc.wantSaved)
			}
			sf := onlyStageFailureEvent(t, run.rec)
			if sf.Name != "notes" || sf.Message != tc.wantMessage {
				t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, tc.wantMessage)
			}
			if sf.SavedNotes != path {
				t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
			}
			if len(run.pub.dispatched) != 0 {
				t.Errorf("provider writes = %d, want none for an aborted regenerate", len(run.pub.dispatched))
			}
		})
	}
}

// savedSkipCause is version's skip cause, its cause phrase phrase, when its chunk
// notes were saved at path.
func savedSkipCause(phrase, version, path string) string {
	return phrase + "; part notes saved to " + path + " — tidy them and put them on it with mint release regenerate " + version + " --notes-file " + path
}

func TestRegenerateAll_FreshChunkedRunsFail_EachSkipNamesItsOwnSavedFileAndCommand(t *testing.T) {
	t.Parallel()

	run := newRegenChunkNotes(t)

	run.regenerateAll(t, overCapFailing(failsSecondPart), "v1.1.0", "v1.3.0")

	p1, p3 := run.savedPath("v1.1.0"), run.savedPath("v1.3.0")
	for _, path := range []string{p1, p3} {
		if got := readSaved(t, path); got != partOneSaved {
			t.Errorf("%s saved chunk notes = %q, want %q", path, got, partOneSaved)
		}
	}
	if got := dispatchedTags(run.pub); len(got) != 1 || got[0] != "v1.2.0" {
		t.Errorf("provider writes = %v, want only v1.2.0", got)
	}
	cause1 := savedSkipCause("AI call failed (part 2 of 2)", "1.1.0", p1)
	cause3 := savedSkipCause("AI call failed (part 2 of 2)", "1.3.0", p3)
	want := "1 regenerated, 2 skipped: v1.1.0 (" + cause1 + "), v1.3.0 (" + cause3 + ")"
	if got := finishEvent(t, run.rec).Summary; got != want {
		t.Errorf("RunFinished.Summary = %q, want %q", got, want)
	}
	wantWarns := []presenter.Warning{
		{Label: "skipped v1.1.0", Message: cause1, Output: usageLimitOutput},
		{Label: "skipped v1.3.0", Message: cause3, Output: usageLimitOutput},
	}
	if got := plainWarnings(run.rec); !slices.Equal(got, wantWarns) {
		t.Errorf("warnings = %+v, want %+v", got, wantWarns)
	}
}

func TestRegenerate_NoChunkNotesToKeep_SavesNoFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		produce freshProducer
		// phrases are the skip's possible cause phrases: either part of a wave whose
		// parts all fail can be the one that ends it.
		phrases []string
	}{
		{name: "the single-pass call failed without halving", produce: singlePassFails, phrases: []string{"AI call failed"}},
		{name: "the wave produced no notes", produce: overCapFailing(failsEveryChunk), phrases: []string{"AI call failed (part 1 of 2)", "AI call failed (part 2 of 2)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name+", single version", func(t *testing.T) {
			t.Parallel()

			run := newRegenChunkNotes(t)

			if err := run.regenerate(t, "v1.2.0", tc.produce); err == nil {
				t.Fatal("RegenerateRun returned nil error, want an abort")
			}

			assertNoSavedChunkNotes(t, run.base())
			if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != "" {
				t.Errorf("StageFailure.SavedNotes = %q, want none named", sf.SavedNotes)
			}
			if got := plainWarnings(run.rec); len(got) != 0 {
				t.Errorf("warnings = %+v, want none", got)
			}
		})
		t.Run(tc.name+", --all", func(t *testing.T) {
			t.Parallel()

			run := newRegenChunkNotes(t)

			run.regenerateAll(t, tc.produce, "v1.2.0")

			assertNoSavedChunkNotes(t, run.base())
			got := plainWarnings(run.rec)
			if len(got) != 1 || !slices.ContainsFunc(tc.phrases, func(phrase string) bool { return got[0] == skipWarning("v1.2.0", phrase) }) {
				t.Errorf("warnings = %+v, want only v1.2.0's skip reading one of %q", got, tc.phrases)
			}
			summary := finishEvent(t, run.rec).Summary
			if !slices.ContainsFunc(tc.phrases, func(phrase string) bool { return summary == "2 regenerated, 1 skipped: v1.2.0 ("+phrase+")" }) {
				t.Errorf("RunFinished.Summary = %q, want v1.2.0 skipped reading one of %q and naming no file", summary, tc.phrases)
			}
		})
	}
}

func TestRegenerateRun_ChunkNotesCannotBeWritten_KeepsTheAIsFailureAndWarnsWithTheErrorVerbatim(t *testing.T) {
	t.Parallel()

	run := newRegenChunkNotes(t)
	run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}

	if err := run.regenerate(t, "v1.2.0", overCapFailing(failsSecondPart)); err == nil {
		t.Fatal("RegenerateRun returned nil error, want an abort")
	}

	sf := onlyStageFailureEvent(t, run.rec)
	if sf.Message != "AI call failed (part 2 of 2)" || sf.Output != usageLimitOutput {
		t.Errorf("StageFailure = %q with output %q, want the AI's failure unchanged", sf.Message, sf.Output)
	}
	if sf.SavedNotes != "" {
		t.Errorf("StageFailure.SavedNotes = %q, want no path named", sf.SavedNotes)
	}
	assertOnlyWarning(t, run.rec, unsavedChunkNotesWarning(errChunkNotesWrite.Error()))
	assertWarnedAfterTheFailure(t, run.rec)
}

func TestRegenerateAll_ChunkNotesCannotBeWritten_WarnsAfterTheSkipAndTheBatchCarriesOn(t *testing.T) {
	t.Parallel()

	run := newRegenChunkNotes(t)
	run.deps.ChunkNotes = failingChunkNotes{err: errChunkNotesWrite}

	run.regenerateAll(t, overCapFailing(failsSecondPart), "v1.1.0")

	want := []presenter.Warning{skipWarning("v1.1.0", "AI call failed (part 2 of 2)"), unsavedChunkNotesWarning(errChunkNotesWrite.Error())}
	if got := plainWarnings(run.rec); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("warnings = %+v, want %+v", got, want)
	}
	if got := dispatchedTags(run.pub); len(got) != 2 || got[0] != "v1.2.0" || got[1] != "v1.3.0" {
		t.Errorf("provider writes = %v, want v1.2.0 and v1.3.0 with v1.1.0 skipped", got)
	}
	want2 := "2 regenerated, 1 skipped: v1.1.0 (AI call failed (part 2 of 2))"
	if got := finishEvent(t, run.rec).Summary; got != want2 {
		t.Errorf("RunFinished.Summary = %q, want %q naming no file", got, want2)
	}
}
