package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// chunkNotesRun is a release whose chunk notes save under a user cache dir of its own.
type chunkNotesRun struct {
	root, userCache string
	f               *runner.FakeRunner
	rec             *presentertest.RecordingPresenter
	deps            engine.ReleaseDeps
}

func newChunkNotesRun(t *testing.T, onNotesFailure string, transport func(ctx context.Context, prompt string) (string, error)) *chunkNotesRun {
	t.Helper()
	return newChunkNotesRunAt(t, t.TempDir(), onNotesFailure, transport)
}

// newChunkNotesRunAt is newChunkNotesRun from the repo at root.
func newChunkNotesRunAt(t *testing.T, root, onNotesFailure string, transport func(ctx context.Context, prompt string) (string, error)) *chunkNotesRun {
	t.Helper()
	run := &chunkNotesRun{root: root, userCache: t.TempDir(), f: runner.NewFakeRunner(), rec: &presentertest.RecordingPresenter{}}
	seedOverCapRelease(t, run.root, run.f, onNotesFailure)
	run.deps = newDeps(run.rec, run.f)
	run.deps.Transport = transportFunc(transport)
	run.deps.ChunkNotes = notescache.NewChunkNotes(run.chunkNotesBase())
	return run
}

func (run *chunkNotesRun) chunkNotesBase() string {
	return filepath.Join(run.userCache, "mint", "chunk-notes")
}

// savedPath is where the run's chunk notes for tag are saved: the repo root, its
// separators as dashes, under the chunk-notes base.
func (run *chunkNotesRun) savedPath(tag string) string {
	scope := strings.ReplaceAll(filepath.Clean(run.root), string(filepath.Separator), "-")
	return filepath.Join(run.chunkNotesBase(), scope, tag+".md")
}

func (run *chunkNotesRun) release(t *testing.T, ctx context.Context) error {
	t.Helper()
	return engine.Release(ctx, run.deps, priorTagNormalAIOptions())
}

func readSaved(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the saved chunk notes: %v", err)
	}
	return string(data)
}

func assertNoSavedChunkNotes(t *testing.T, base string) {
	t.Helper()
	var files []string
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if len(files) != 0 {
		t.Errorf("chunk notes saved to %v, want no file", files)
	}
}

type transportFunc func(ctx context.Context, prompt string) (string, error)

func (f transportFunc) Generate(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

// twoPartTransport answers a two-part chunked run's classify calls OTHER and its
// parts with their notes, unless fail answers the call with a failure first.
func twoPartTransport(fail func(prompt string) error) func(context.Context, string) (string, error) {
	return func(_ context.Context, prompt string) (string, error) {
		if err := fail(prompt); err != nil {
			return "", err
		}
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case strings.Contains(prompt, "What follows is part 1 of"):
			return apiPartNotes, nil
		case strings.Contains(prompt, "What follows is part 2 of"):
			return corePartNotes, nil
		default:
			return "merged notes", nil
		}
	}
}

// partOneSaved is the saved file of a two-part run whose second part, core/b.go's,
// failed: part 1's notes under the line naming the file with none.
const partOneSaved = "**Notes missing:** no notes were written for `core/b.go` — add them before releasing.\n\n" + apiPartNotes

const usageLimitOutput = "API Error: usage limit reached for this account"

func failsSecondPart(prompt string) error {
	if isSecondPartPrompt(prompt) {
		return &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
	}
	return nil
}

func failsMerge(prompt string) error {
	if isMergeCallPrompt(prompt) {
		return &ai.GenerationError{Stdout: "Invalid API key · Please run /login", ExitCode: 1}
	}
	return nil
}

func TestRelease_ChunkFailsForGood_Abort_SavesTheFinishedNotesAndNamesTheFileAfterTheOutput(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))

	err := run.release(t, t.Context())

	assertAbortNonZero(t, err)
	assertNoMutation(t, run.f)
	path := run.savedPath(nextTag)
	if got := readSaved(t, path); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want part 1's notes %q", got, partOneSaved)
	}
	sf := onlyStageFailureEvent(t, run.rec)
	if sf.Message != "AI call failed (part 2 of 2)" || sf.Output != usageLimitOutput {
		t.Errorf("StageFailure = %q with output %q, want the AI's failure unchanged", sf.Message, sf.Output)
	}
	if sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want the saved file's full path %q", sf.SavedNotes, path)
	}
}

func TestRelease_MergeFails_SavesEveryPartsNotes(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsMerge))

	assertAbortNonZero(t, run.release(t, t.Context()))

	path := run.savedPath(nextTag)
	if got, want := readSaved(t, path), apiPartNotes+"\n\n"+corePartNotes; got != want {
		t.Errorf("saved chunk notes = %q, want every part's notes %q", got, want)
	}
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != path {
		t.Errorf("StageFailure.SavedNotes = %q, want %q", sf.SavedNotes, path)
	}
}

func TestRelease_ChunkFailsForGood_Fallback_SavesTheNotesAndShipsTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	const subjects = "subject one\nsubject two\n"
	run := newChunkNotesRun(t, "fallback", twoPartTransport(failsSecondPart))
	run.f.SeedSequence("git", ScriptedOut(subjects)) // log --format=%s
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)

	if err := run.release(t, t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := readSaved(t, run.savedPath(nextTag)); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want part 1's notes %q", got, partOneSaved)
	}
	if body := tagAnnotationBody(t, run.f, nextTag); body != subjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", body, subjects)
	}
}

func TestRelease_NoPartReturnedNotes_SavesNoFileAndNamesNone(t *testing.T) {
	t.Parallel()

	failsEveryPart := func(prompt string) error {
		if isChunkCallPrompt(prompt) {
			return &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
		}
		return nil
	}
	run := newChunkNotesRun(t, "abort", twoPartTransport(failsEveryPart))

	assertAbortNonZero(t, run.release(t, t.Context()))

	assertNoSavedChunkNotes(t, run.chunkNotesBase())
	if sf := onlyStageFailureEvent(t, run.rec); sf.SavedNotes != "" {
		t.Errorf("StageFailure.SavedNotes = %q, want none named", sf.SavedNotes)
	}
}

func TestRelease_ALaterFailureForTheSameTag_OverwritesTheSavedFile(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "abort", twoPartTransport(failsSecondPart))
	path, err := notescache.NewChunkNotes(run.chunkNotesBase()).Save(run.root, nextTag, "- an earlier run's notes\n- for parts long gone\n")
	if err != nil {
		t.Fatalf("pre-saving chunk notes: %v", err)
	}

	assertAbortNonZero(t, run.release(t, t.Context()))

	if got := readSaved(t, path); got != partOneSaved {
		t.Errorf("saved chunk notes = %q, want only the later run's %q", got, partOneSaved)
	}
}

func TestRelease_ALaterDryRunWritingTheNoteCache_LeavesTheSavedFileInPlace(t *testing.T) {
	t.Parallel()

	root, userCache := t.TempDir(), t.TempDir()
	const saved = "- the failed run's part notes\n"
	path, err := notescache.NewChunkNotes(filepath.Join(userCache, "mint", "chunk-notes")).Save(root, nextTag, saved)
	if err != nil {
		t.Fatalf("pre-saving chunk notes: %v", err)
	}
	opts := priorTagNormalAIOptions()
	opts.DryRun = true

	runDryRunNormalAI(t, root, filepath.Join(userCache, "mint", "cache"), priorTagDiff, "## Notes\n- previewed\n", opts)

	if got := readSaved(t, path); got != saved {
		t.Errorf("saved chunk notes after the dry run = %q, want them untouched %q", got, saved)
	}
}

func TestRelease_CtrlC_SavesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// cancelAt reports whether prompt is the call the Ctrl-C lands on.
		cancelAt func(prompt string) bool
	}{
		{name: "while chunk calls are in flight, part 1 finished", cancelAt: isSecondPartPrompt},
		{name: "during the merge call", cancelAt: isMergeCallPrompt},
		{name: "during the merge's classify call", cancelAt: isClassifyPrompt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cancels := func(prompt string) error {
				if tc.cancelAt(prompt) {
					cancel()
					return context.Canceled
				}
				return failsMerge(prompt)
			}
			run := newChunkNotesRun(t, "abort", twoPartTransport(cancels))

			err := run.release(t, ctx)

			if !errors.Is(err, context.Canceled) {
				t.Errorf("Release error = %v, want context.Canceled", err)
			}
			assertNoSavedChunkNotes(t, run.chunkNotesBase())
		})
	}
}
