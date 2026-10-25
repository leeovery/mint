package engine_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	noRoomPhrase        = "no room for the diff under max_prompt_bytes"
	mergeTooLargePhrase = "part notes too large to merge"
)

// outsizedContext is operator text long enough that, with mint's own text around
// it, no diff fits under partCapBytes.
var outsizedContext = strings.Repeat("Project context the operator wrote at length.\n", 600)

// bigPartNotes is chunk notes large enough that two of them overflow a merge prompt
// under partCapBytes or the over-cap release's cap, as the default prompt's length
// changes.
var bigPartNotes = strings.Repeat(partNoteLine, partCap(halvableFileBytes)/len(partNoteLine))

const partNoteLine = "- a long note about this part\n"

// uncallableCase is a release over a two-file diff of about 10 KB a file whose notes
// cannot be written within its cap.
type uncallableCase struct {
	name   string
	config string
	// context, when set, is written to context.md at the repo root.
	context string
	// claude scripts the AI calls the run is expected to make.
	claude    []runner.ScriptedCall
	wantCalls int
	phrase    string
}

func uncallableCases() []uncallableCase {
	return []uncallableCase{
		{
			name:    "context leaves no room for diff",
			config:  partCapToml + "\n[release]\ncontext = \"context.md\"\n",
			context: outsizedContext,
			phrase:  noRoomPhrase,
		},
		{
			name:    "prompt override leaves no room for diff",
			config:  partCapToml + "\n[release]\nprompt = \"context.md\"\n",
			context: outsizedContext,
			phrase:  noRoomPhrase,
		},
		{
			name:   "cap below mint's own instructions",
			config: "max_prompt_bytes = 1000\n\n[release]\n",
			phrase: noRoomPhrase,
		},
		{
			name:      "merge prompt over the cap",
			config:    partCapToml + "\n[release]\n",
			claude:    []runner.ScriptedCall{ScriptedOut(bigPartNotes), ScriptedOut(bigPartNotes)},
			wantCalls: 2,
			phrase:    mergeTooLargePhrase,
		},
	}
}

func (tc uncallableCase) seedRelease(t *testing.T, root string, f *runner.FakeRunner, onNotesFailure string) {
	t.Helper()
	writeConfig(t, root, tc.config+"on_notes_failure = \""+onNotesFailure+"\"\n")
	if tc.context != "" {
		writeFile(t, root, "context.md", tc.context)
	}
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
	seedClaudeAfterCheck(f, tc.claude...)
}

// aiCallCount counts the notes calls, leaving out the exclusion check's.
func aiCallCount(f *runner.FakeRunner) int {
	return len(notesInvocations(f, "claude"))
}

func TestRelease_NotesCallCannotBeMade_Abort_StopsBeforeTheTagNamingTheCause(t *testing.T) {
	t.Parallel()

	for _, tc := range uncallableCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seedRelease(t, root, f, "abort")
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			assertNoMutation(t, f)
			if got := aiCallCount(f); got != tc.wantCalls {
				t.Errorf("AI calls = %d, want %d", got, tc.wantCalls)
			}
			sf := onlyStageFailureEvent(t, rec)
			if sf.Message != tc.phrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.phrase)
			}
			if sf.Output != "" {
				t.Errorf("StageFailure.Output = %q, want no AI output beneath the line", sf.Output)
			}
		})
	}
}

func TestRelease_NotesCallCannotBeMade_Fallback_ShipsTheCommitSubjects(t *testing.T) {
	t.Parallel()

	for _, tc := range uncallableCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			tc.seedRelease(t, root, f, "fallback")
			f.SeedSequence("git", ScriptedOut("subject one\nsubject two\n")) // log --format=%s
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := aiCallCount(f); got != tc.wantCalls {
				t.Errorf("AI calls = %d, want %d", got, tc.wantCalls)
			}
			if got := tagAnnotationBody(t, f, nextTag); got != "subject one\nsubject two\n" {
				t.Errorf("tag annotation body = %q, want the commit-subject fallback", got)
			}
		})
	}
}

// fixedNotesTransport answers every call with notes and records each prompt.
type fixedNotesTransport struct {
	mu      sync.Mutex
	notes   string
	prompts []string
}

func (ft *fixedNotesTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.prompts = append(ft.prompts, prompt)
	return ft.notes, nil
}

func (ft *fixedNotesTransport) calls() int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return len(ft.prompts)
}

// uncallableRegenerateCase is a fresh regenerate over a two-file diff of about 10 KB
// a file whose notes cannot be written within its cap.
type uncallableRegenerateCase struct {
	name      string
	maxBytes  int
	context   string
	notes     string
	wantCalls int
	phrase    string
}

func uncallableRegenerateCases() []uncallableRegenerateCase {
	return []uncallableRegenerateCase{
		{name: "context leaves no room for diff", maxBytes: partCapBytes, context: outsizedContext, phrase: noRoomPhrase},
		{name: "cap below mint's own instructions", maxBytes: 1_000, phrase: noRoomPhrase},
		{name: "merge prompt over the cap", maxBytes: partCapBytes, notes: bigPartNotes, wantCalls: 2, phrase: mergeTooLargePhrase},
	}
}

func (tc uncallableRegenerateCase) produce(t *testing.T, tr *fixedNotesTransport, res version.Resolution) (string, error) {
	t.Helper()
	dir := t.TempDir()
	cfg := freshCfg()
	cfg.MaxPromptBytes = tc.maxBytes
	if tc.context != "" {
		writeFile(t, dir, "context.md", tc.context)
		cfg.Release.Context = "context.md"
	}
	notesGit := seedFreshGit(twoFileDiff(10_000), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	return engine.RegenerateFreshBody(t.Context(), notesGit, tr, dir, cfg, res, engine.NotesSinks{})
}

func TestRegenerateRun_FreshNotesCallCannotBeMade_AbortsNamingTheCause(t *testing.T) {
	t.Parallel()

	for _, tc := range uncallableRegenerateCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			f.Seed("gh", runner.Result{}, nil)
			pub := newFakePublisher()
			rec := &presentertest.RecordingPresenter{}
			tr := &fixedNotesTransport{notes: tc.notes}
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
			req.ProduceBody = func(context.Context, engine.RegenerateSource, engine.NotesSinks) (string, error) {
				return tc.produce(t, tr, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"})
			}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, t.TempDir(), req); err == nil {
				t.Fatal("RegenerateRun returned nil error, want an abort")
			}

			if got := tr.calls(); got != tc.wantCalls {
				t.Errorf("AI calls = %d, want %d", got, tc.wantCalls)
			}
			sf := onlyStageFailureEvent(t, rec)
			if sf.Message != tc.phrase {
				t.Errorf("StageFailure.Message = %q, want %q", sf.Message, tc.phrase)
			}
			if sf.Output != "" {
				t.Errorf("StageFailure.Output = %q, want no AI output beneath the line", sf.Output)
			}
			if len(pub.dispatched) != 0 {
				t.Errorf("provider writes = %d, want none for an aborted regenerate", len(pub.dispatched))
			}
		})
	}
}

func TestRegenerateAll_FreshNotesCallCannotBeMade_SkipsThatVersionAndCarriesOn(t *testing.T) {
	t.Parallel()

	for _, tc := range uncallableRegenerateCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(batchV1Tag, true, nil)
			pub.seedExists(batchV2Tag, true, nil)
			pub.seedExists(batchV3Tag, true, nil)
			rec := &presentertest.RecordingPresenter{}
			tr := &fixedNotesTransport{notes: tc.notes}
			req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
			req.ProduceBody = func(_ context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
				if res.Tag == batchV2Tag {
					return tc.produce(t, tr, res)
				}
				return "## fresh " + res.Tag + "\n", nil
			}

			if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			if got := tr.calls(); got != tc.wantCalls {
				t.Errorf("AI calls for %s = %d, want %d", batchV2Tag, got, tc.wantCalls)
			}
			if got := dispatchedTags(pub); len(got) != 2 || got[0] != batchV1Tag || got[1] != batchV3Tag {
				t.Errorf("provider writes = %v, want %s and %s with %s skipped", got, batchV1Tag, batchV3Tag, batchV2Tag)
			}
			assertSkippedV2(t, rec, pub, tc.phrase, "")
		})
	}
}
