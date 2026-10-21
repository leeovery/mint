package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const designSuggestion = "docs/design/: design documents\n"

// suggestingTransport answers the exclusion check with check and every other call
// through notes, recording the other calls' prompts.
type suggestingTransport struct {
	check   string
	notes   notes.Transport
	mu      sync.Mutex
	prompts []string
}

func (st *suggestingTransport) Generate(ctx context.Context, prompt string) (string, error) {
	if isCheckPrompt(prompt) {
		return st.check, nil
	}
	st.mu.Lock()
	st.prompts = append(st.prompts, prompt)
	st.mu.Unlock()
	return st.notes.Generate(ctx, prompt)
}

func notesBody(body string, err error) notes.Transport {
	return transportFunc(func(context.Context, string) (string, error) { return body, err })
}

// suggestedRun is an unattended -y prior-tag release under config whose notes diff
// carries src/export.go, docs/design/a.md and docs/design/b.md (12,000 and 3,000
// bytes) and tmp/scratch.txt, the AI answering the check with check and the notes
// through notesCalls. The caller seeds what follows the notes.
type suggestedRun struct {
	root string
	f    *runner.FakeRunner
	rec  *presentertest.RecordingPresenter
	deps engine.ReleaseDeps
	tr   *suggestingTransport
}

func newSuggestedRun(t *testing.T, config, check string, notesCalls notes.Transport) *suggestedRun {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(exportGoDiff+
			fileDiffOfSize(t, "docs/design/a.md", 12_000)+
			fileDiffOfSize(t, "docs/design/b.md", 3_000)+
			fileDiffOfSize(t, "tmp/scratch.txt", 176)),
		ScriptedOut("M\tsrc/export.go\nM\tdocs/design/a.md\nM\tdocs/design/b.md\nM\ttmp/scratch.txt\n"),
		ScriptedOut("1\t1\tsrc/export.go\n1\t1\tdocs/design/a.md\n1\t1\tdocs/design/b.md\n1\t1\ttmp/scratch.txt\n"),
	)
	rec := &presentertest.RecordingPresenter{}
	tr := &suggestingTransport{check: check, notes: notesCalls}
	deps := newDeps(rec, f)
	deps.Yes = true
	deps.Transport = tr
	return &suggestedRun{root: root, f: f, rec: rec, deps: deps, tr: tr}
}

// shipping seeds the run through the push.
func (r *suggestedRun) shipping() *suggestedRun {
	seedRecordTagPush(r.f, r.root)
	r.f.Seed("gh", runner.Result{}, nil)
	return r
}

// release runs the release and fails unless .mint.toml is byte-for-byte as it was.
func (r *suggestedRun) release(t *testing.T) error {
	t.Helper()
	return r.releaseWith(t, priorTagNormalAIOptions())
}

// releaseWith runs the release under opts and fails unless .mint.toml is byte-for-byte
// as it was.
func (r *suggestedRun) releaseWith(t *testing.T, opts engine.ReleaseOptions) error {
	t.Helper()
	path := filepath.Join(r.root, ".mint.toml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	runErr := engine.Release(t.Context(), r.deps, opts)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(".mint.toml after the run: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf(".mint.toml = %q after the run, want it unchanged %q", after, before)
	}
	for _, inv := range invocationsNamed(r.f, "git") {
		if slices.Contains(inv.Args, ".mint.toml") && slices.Contains(inv.Args, "add") {
			t.Errorf("the run staged .mint.toml: %s", commandLine(inv))
		}
	}
	return runErr
}

func suggestionEvents(rec *presentertest.RecordingPresenter) []int {
	var at []int
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindExclusionSuggestions {
			at = append(at, i)
		}
	}
	return at
}

// onlySuggestions fails unless rec shows one ExclusionSuggestions event, and returns
// its index and payload.
func onlySuggestions(t *testing.T, rec *presentertest.RecordingPresenter) (int, presenter.ExclusionSuggestions) {
	t.Helper()
	at := suggestionEvents(rec)
	if len(at) != 1 {
		t.Fatalf("ExclusionSuggestions events = %d, want one; kinds = %v", len(at), rec.Kinds())
	}
	return at[0], rec.Events[at[0]].ExclusionSuggestions
}

func assertSuggested(t *testing.T, got presenter.ExclusionSuggestions, want ...presenter.ExclusionSuggestion) {
	t.Helper()
	if got.Stage != "notes" || got.Table != "[release]" {
		t.Errorf("suggestions stage %q table %q, want notes under [release]", got.Stage, got.Table)
	}
	if !reflect.DeepEqual(got.Suggestions, want) {
		t.Errorf("suggestions =\n %+v\nwant\n %+v", got.Suggestions, want)
	}
}

var designSuggested = presenter.ExclusionSuggestion{
	Glob:    "docs/design/",
	Reason:  "design documents",
	Matches: "2 files, 15 KB",
	Line:    `diff_exclude = ["docs/design/"]`,
}

func TestRelease_Unattended_PausesOnNothingAndTheNotesCarryTheFlaggedFiles(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "", designSuggestion, notesBody(aiBody, nil)).shipping()

	if err := run.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if gates := pauseGates(run.rec); len(gates) != 0 {
		t.Errorf("an unattended run paused: %+v", gates)
	}
	if len(run.tr.prompts) != 1 {
		t.Fatalf("notes calls = %d, want one", len(run.tr.prompts))
	}
	for _, path := range []string{"docs/design/a.md", "docs/design/b.md"} {
		if !strings.Contains(run.tr.prompts[0], "diff --git a/"+path+" b/"+path+"\n") {
			t.Errorf("the notes call does not carry %s's diff", path)
		}
	}
	assertNotesGitCallsExclude(t, run.f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml")
}

func TestRelease_Unattended_PrintsEachSuggestionAfterTheNotesWithTheLineThatExcludesIt(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "", designSuggestion, notesBody(aiBody, nil)).shipping()

	if err := run.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	at, got := onlySuggestions(t, run.rec)
	assertSuggested(t, got, designSuggested)
	if shown := indexOfKind(run.rec, presentertest.KindShowNotes); shown == -1 || shown+1 != at {
		t.Errorf("want the suggestions straight after the notes; kinds = %v", run.rec.Kinds())
	}
}

func TestRelease_Unattended_EachLineIsTheReleaseKeyAsAnExcludeAnswerWouldLeaveIt(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config string
		want   []presenter.ExclusionSuggestion
	}{
		{
			name: "key unset",
			want: []presenter.ExclusionSuggestion{
				designSuggested,
				{Glob: "tmp/", Reason: "scratch files", Matches: "1 file, 176 B", Line: `diff_exclude = ["tmp/"]`},
			},
		},
		{
			name:   "key set",
			config: "[release]\ndiff_exclude = [\"specs/\"]\n",
			want: []presenter.ExclusionSuggestion{
				{Glob: "docs/design/", Reason: "design documents", Matches: "2 files, 15 KB", Line: `diff_exclude = ["specs/", "docs/design/"]`},
				{Glob: "tmp/", Reason: "scratch files", Matches: "1 file, 176 B", Line: `diff_exclude = ["specs/", "tmp/"]`},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, tc.config, designSuggestion+"tmp/: scratch files\n", notesBody(aiBody, nil)).shipping()

			if err := run.release(t); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			_, got := onlySuggestions(t, run.rec)
			assertSuggested(t, got, tc.want...)
		})
	}
}

func TestRelease_Unattended_APrintedLineUnderReleaseLoadsAsTheKeyWithTheGlobAppended(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "[release]\ndiff_exclude = [\"specs/\"]\n", designSuggestion, notesBody(aiBody, nil)).shipping()

	if err := run.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	_, got := onlySuggestions(t, run.rec)
	if len(got.Suggestions) != 1 {
		t.Fatalf("suggestions = %+v, want one", got.Suggestions)
	}
	root := t.TempDir()
	writeConfig(t, root, got.Table+"\n"+got.Suggestions[0].Line+"\n")
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("loading the printed line: %v", err)
	}
	if want := []string{"specs/", "docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
}

func TestRelease_Unattended_ASuggestionMatchingAKeptFileNamesItAsOnTheKeepList(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "[release]\ndiff_keep = [\"docs/design/b.md\"]\n", designSuggestion, notesBody(aiBody, nil)).shipping()

	if err := run.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	_, got := onlySuggestions(t, run.rec)
	assertSuggested(t, got, presenter.ExclusionSuggestion{
		Glob:    "docs/design/",
		Reason:  "design documents",
		Matches: "2 files, 15 KB; on the keep list: docs/design/b.md",
		Line:    `diff_exclude = ["docs/design/"]`,
	})
}

func TestRelease_UnderYes_TheOutputIsTheSameWhetherStdinIsATerminalOrNot(t *testing.T) {
	t.Parallel()

	runs := map[bool]*suggestedRun{}
	for _, interactive := range []bool{true, false} {
		run := newSuggestedRun(t, "", designSuggestion, notesBody(aiBody, nil)).shipping()
		run.deps.StdinInteractive = interactive
		if err := run.release(t); err != nil {
			t.Fatalf("Release (stdin a terminal: %v) returned unexpected error: %v", interactive, err)
		}
		runs[interactive] = run
	}

	tty, piped := runs[true].rec, runs[false].rec
	if !slices.Equal(tty.Kinds(), piped.Kinds()) {
		t.Errorf("event kinds differ:\n terminal: %v\n     pipe: %v", tty.Kinds(), piped.Kinds())
	}
	_, ttySuggestions := onlySuggestions(t, tty)
	_, pipedSuggestions := onlySuggestions(t, piped)
	if !reflect.DeepEqual(ttySuggestions, pipedSuggestions) {
		t.Errorf("suggestions differ:\n terminal: %+v\n     pipe: %+v", ttySuggestions, pipedSuggestions)
	}
	if !reflect.DeepEqual(promptGates(tty), promptGates(piped)) {
		t.Errorf("gates differ:\n terminal: %+v\n     pipe: %+v", promptGates(tty), promptGates(piped))
	}
}

func TestRelease_Unattended_Fallback_PrintsTheSuggestionsAfterTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, fallbackConfig, designSuggestion, notesBody("", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	run.shipping()

	if err := run.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	at, got := onlySuggestions(t, run.rec)
	assertSuggested(t, got, designSuggested)
	shown := indexOfKind(run.rec, presentertest.KindShowNotes)
	if shown == -1 || run.rec.Events[shown].ShowNotes.Body != commitSubjects {
		t.Fatalf("want the commit-subject list shown; kinds = %v", run.rec.Kinds())
	}
	if shown+1 != at {
		t.Errorf("want the suggestions straight after the commit-subject list; kinds = %v", run.rec.Kinds())
	}
}

func TestRelease_Unattended_AtTheFloor_PrintsTheSuggestionsWithTheNextMovesAndExits1(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "[release]\non_notes_failure = \"abort\"\n", designSuggestion,
		notesBody("", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))

	err := run.release(t)

	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort with exit 1", err)
	}
	at, got := onlySuggestions(t, run.rec)
	assertSuggested(t, got, designSuggested)
	if moves := indexOfKind(run.rec, presentertest.KindNextMoves); moves == -1 || moves+1 != at {
		t.Errorf("want the suggestions straight after the next moves; kinds = %v", run.rec.Kinds())
	}
	if shown := indexOfKind(run.rec, presentertest.KindShowNotes); shown != -1 {
		t.Errorf("notes were shown; kinds = %v", run.rec.Kinds())
	}
	assertNoTag(t, run.f)
}

func TestRelease_Unattended_NotesWrittenAfterHalving_PrintTheLowerCapHintThenTheSuggestions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHalvedRelease(root, f)
	rec := &presentertest.RecordingPresenter{}
	halving := &sizeOverflowTransport{}
	deps := newDeps(rec, f)
	deps.Yes = true
	deps.Transport = &suggestingTransport{check: "core/: core internals\n", notes: halving}

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	halving.assertHalved(t)
	hints := lowerCapHints(rec)
	if len(hints) != 1 {
		t.Fatalf("lower-the-cap hints = %d, want one", len(hints))
	}
	at, got := onlySuggestions(t, rec)
	if len(got.Suggestions) != 1 || got.Suggestions[0].Glob != "core/" {
		t.Errorf("suggestions = %+v, want core/", got.Suggestions)
	}
	shown := indexOfKind(rec, presentertest.KindShowNotes)
	if shown == -1 || hints[0] != shown+1 || at != hints[0]+1 {
		t.Errorf("want the notes, the hint, then the suggestions; kinds = %v", rec.Kinds())
	}
}
