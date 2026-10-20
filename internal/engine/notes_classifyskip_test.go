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
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const notLoggedIn = "Not logged in · Please run /login"

func isChunkCallPrompt(prompt string) bool {
	return strings.Contains(prompt, "What follows is part ")
}

// classifySkipTransport fails every notes call whose prompt fails reports true with
// failure, answers the other notes calls with notes, and answers each classify call
// with classify, OTHER when unset. It records every prompt in arrival order.
type classifySkipTransport struct {
	fails    func(prompt string) bool
	failure  error
	classify func(ctx context.Context) (string, error)

	mu      sync.Mutex
	prompts []string
}

func (ct *classifySkipTransport) Generate(ctx context.Context, prompt string) (string, error) {
	ct.mu.Lock()
	ct.prompts = append(ct.prompts, prompt)
	ct.mu.Unlock()
	switch {
	case isClassifyPrompt(prompt):
		if ct.classify == nil {
			return "OTHER", nil
		}
		return ct.classify(ctx)
	case ct.fails(prompt):
		return "", ct.failure
	case strings.Contains(prompt, "What follows is part 1 of"):
		return apiPartNotes, nil
	case isChunkCallPrompt(prompt):
		return corePartNotes, nil
	default:
		return "merged notes", nil
	}
}

func (ct *classifySkipTransport) count(kind func(prompt string) bool) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	n := 0
	for _, p := range ct.prompts {
		if kind(p) {
			n++
		}
	}
	return n
}

func everyCall(string) bool { return true }

// isMergeCallPrompt reports whether prompt is a merge call's: a notes call carrying
// no diff.
func isMergeCallPrompt(prompt string) bool {
	return !isClassifyPrompt(prompt) && !strings.Contains(prompt, "diff --git ")
}

func TestRelease_NotesCallFails_ClassifiesOnlyWhenEitherStreamHoldsText(t *testing.T) {
	t.Parallel()

	failedWith := func(stdout, stderr string) runner.ScriptedCall {
		return runner.ScriptedCall{
			Result: runner.Result{Stdout: stdout, Stderr: stderr, ExitCode: 1},
			Err:    errors.New("exit status 1"),
		}
	}
	cases := []struct {
		name        string
		attempt     runner.ScriptedCall
		wantCalls   int
		wantMessage string
		wantOutput  string
	}{
		{
			name:        "an empty body, whitespace on stderr",
			attempt:     runner.ScriptedCall{Result: runner.Result{Stdout: "", Stderr: " \n\t\n"}},
			wantCalls:   2,
			wantMessage: "AI returned empty notes",
		},
		{
			name:        "a whitespace body, whitespace on stderr",
			attempt:     runner.ScriptedCall{Result: runner.Result{Stdout: "  \n", Stderr: "\n"}},
			wantCalls:   2,
			wantMessage: "AI returned empty notes",
		},
		{
			name:        "text only on stdout",
			attempt:     failedWith(notLoggedIn+"\n", ""),
			wantCalls:   3,
			wantMessage: "AI call failed",
			wantOutput:  notLoggedIn,
		},
		{
			name:        "text only on stderr",
			attempt:     runner.ScriptedCall{Result: runner.Result{Stdout: " \n", Stderr: "Credit balance is too low\n"}},
			wantCalls:   3,
			wantMessage: "AI call failed",
			wantOutput:  "Credit balance is too low",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release]\nai_command = \"tool-a\"\n")
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedNormalAINotes(f)
			f.SeedSequence("tool-a", tc.attempt, tc.attempt, ScriptedOut("OTHER\n"))
			rec := &presentertest.RecordingPresenter{}
			d := &deadlineRunner{FakeRunner: f}
			deps := newDeps(rec, f)
			deps.Runner = d

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			if len(d.calls) != tc.wantCalls {
				t.Fatalf("AI calls = %d, want %d", len(d.calls), tc.wantCalls)
			}
			if tc.wantCalls == 3 && !isClassifyPrompt(d.calls[2].stdin) {
				t.Errorf("third AI call =\n%.200s\nwant the classify call", d.calls[2].stdin)
			}
			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != tc.wantMessage || sf.Output != tc.wantOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, tc.wantMessage, tc.wantOutput)
			}
		})
	}
}

func TestRelease_CapBelowTheClassifyPrompt_NoClassifyCallAndTheOriginalOutputShows(t *testing.T) {
	t.Parallel()

	// Two 20 KB streams trim to about 4 KB each, so the classify prompt is about 9 KB:
	// over a 6,000-byte cap that the single-pass notes call fits under.
	failure := &ai.GenerationError{
		Stdout:   strings.Repeat("stdout line from the tool\n", 800),
		Stderr:   strings.Repeat("stderr line from the tool\n", 800),
		ExitCode: 1,
	}
	root := t.TempDir()
	writeConfig(t, root, "max_prompt_bytes = 6000\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	rec := &presentertest.RecordingPresenter{}
	tr := &classifySkipTransport{fails: everyCall, failure: failure, classify: func(context.Context) (string, error) { return "SIZE", nil }}
	deps := newDeps(rec, f)
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if len(tr.prompts) != 1 || isChunkCallPrompt(tr.prompts[0]) {
		t.Fatalf("AI calls = %d, want only the single-pass notes call", len(tr.prompts))
	}
	if len(tr.prompts[0]) > 6000 {
		t.Fatalf("fixture: the single-pass prompt is %d bytes, over the 6,000-byte cap", len(tr.prompts[0]))
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != failure.Output() {
		t.Errorf("StageFailure = %q %q with output %.80q, want notes %q with the notes call's output verbatim", sf.Name, sf.Message, sf.Output, "AI call failed")
	}
}

func TestRelease_ClassifyCallFails_TheLineShowsTheNotesCallsOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"fails on both attempts", &ai.GenerationError{Stdout: "classify call broke\n", Stderr: "classify stderr\n", ExitCode: 1}},
		{"times out", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)},
		{"program not found", fmt.Errorf("%w: claude", ai.ErrCommandMissing)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedNormalAINotes(f)
			rec := &presentertest.RecordingPresenter{}
			tr := &classifySkipTransport{
				fails:    everyCall,
				failure:  &ai.GenerationError{Stdout: notLoggedIn + "\n", ExitCode: 1},
				classify: func(context.Context) (string, error) { return "", tc.err },
			}
			deps := newDeps(rec, f)
			deps.Transport = tr

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			if len(tr.prompts) != 2 || !isClassifyPrompt(tr.prompts[1]) {
				t.Fatalf("AI calls = %d, want the notes call then one classify call and nothing after", len(tr.prompts))
			}
			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != notLoggedIn {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, "AI call failed", notLoggedIn)
			}
		})
	}
}

func TestRelease_NotesCallTimesOutOrHasNoProgram_NoClassifyCall(t *testing.T) {
	t.Parallel()

	causes := []struct {
		name   string
		err    error
		phrase string
	}{
		{"times out", fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout), "AI timed out"},
		{"program not installed", fmt.Errorf("%w: claude", ai.ErrCommandMissing), "AI tool not installed"},
	}
	calls := []struct {
		name    string
		overCap bool
		fails   func(prompt string) bool
		suffix  string
	}{
		{name: "the single-pass call", fails: everyCall},
		{name: "a chunk call", overCap: true, fails: isSecondPartPrompt, suffix: " (part 2 of 2)"},
		{name: "the merge call", overCap: true, fails: isMergeCallPrompt, suffix: " (merging)"},
	}
	for _, call := range calls {
		for _, cause := range causes {
			t.Run(call.name+" "+cause.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				f := runner.NewFakeRunner()
				if call.overCap {
					seedOverCapRelease(t, root, f, "abort")
				} else {
					seedPriorTagReadGates(f, root, "main")
					seedNormalAINotes(f)
				}
				rec := &presentertest.RecordingPresenter{}
				tr := &classifySkipTransport{fails: call.fails, failure: cause.err}
				deps := newDeps(rec, f)
				deps.Transport = tr

				err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

				assertAbortNonZero(t, err)
				if n := tr.count(isClassifyPrompt); n != 0 {
					t.Errorf("classify calls = %d, want none", n)
				}
				sf := onlyStageFailureEvent(t, rec)
				if want := cause.phrase + call.suffix; sf.Name != "notes" || sf.Message != want {
					t.Errorf("StageFailure = %q %q, want notes %q", sf.Name, sf.Message, want)
				}
			})
		}
	}
}

// seedInterruptedRelease scripts a release under on_notes_failure = "fallback", over
// the two-part chunked diff when overCap is set, with the fallback and the mutation
// tail scripted in case the run wrongly reaches them.
func seedInterruptedRelease(t *testing.T, root string, f *runner.FakeRunner, overCap bool) {
	t.Helper()
	if overCap {
		seedOverCapRelease(t, root, f, "fallback")
	} else {
		writeConfig(t, root, "[release]\non_notes_failure = \"fallback\"\n")
		seedPriorTagReadGates(f, root, "main")
		seedNormalAINotes(f)
	}
	f.SeedSequence("git", ScriptedOut("subject one\n")) // log --format=%s
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
}

// assertInterrupted fails unless the release aborted as a cancel, before any mutation
// and without building the fallback body.
func assertInterrupted(t *testing.T, err error, f *runner.FakeRunner) {
	t.Helper()
	assertAbortNonZero(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want it to carry context.Canceled", err)
	}
	assertNoMutation(t, f)
	if invokedWith(f, "git", "log", "--format=%s", priorTag+"..HEAD") {
		t.Errorf("an interrupted run built the fallback body; on_notes_failure must not apply")
	}
}

// cancellingClassify fails each notes call whose prompt fails reports true, with output,
// and lands the Ctrl-C during the classify call, after ready closes when ready is set.
func cancellingClassify(fails func(string) bool, cancel context.CancelFunc, ready <-chan struct{}) *classifySkipTransport {
	return &classifySkipTransport{
		fails:   fails,
		failure: &ai.GenerationError{Stdout: notLoggedIn + "\n", ExitCode: 1},
		classify: func(ctx context.Context) (string, error) {
			if ready != nil {
				<-ready
			}
			cancel()
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
}

func TestRelease_CtrlCDuringAClassifyCall_Fallback_AbortsWithNoFurtherCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		overCap bool
		fails   func(string) bool
	}{
		{name: "the single-pass call's", fails: everyCall},
		{name: "the merge call's", overCap: true, fails: isMergeCallPrompt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedInterruptedRelease(t, root, f, tc.overCap)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			tr := cancellingClassify(tc.fails, cancel, nil)
			deps := newDeps(&presentertest.RecordingPresenter{}, f)
			deps.Transport = tr

			err := engine.Release(ctx, deps, priorTagNormalAIOptions())

			assertInterrupted(t, err, f)
			if last := tr.prompts[len(tr.prompts)-1]; !isClassifyPrompt(last) {
				t.Errorf("last AI call =\n%.200s\nwant the classify call the Ctrl-C landed in", last)
			}
		})
	}
}

// heldChunkCalls holds each chunk call until the run is cancelled, landing the Ctrl-C
// itself when cancel is set and closing entered once the first arrives. It hands every
// other call to next, and with cancel unset a chunk call next fails too, and records
// every call there.
type heldChunkCalls struct {
	next    *classifySkipTransport
	cancel  context.CancelFunc
	entered chan struct{}
	once    sync.Once
}

func (hc *heldChunkCalls) Generate(ctx context.Context, prompt string) (string, error) {
	if !isChunkCallPrompt(prompt) || (hc.next.fails(prompt) && hc.cancel == nil) {
		return hc.next.Generate(ctx, prompt)
	}
	hc.next.mu.Lock()
	hc.next.prompts = append(hc.next.prompts, prompt)
	hc.next.mu.Unlock()
	hc.once.Do(func() {
		if hc.entered != nil {
			close(hc.entered)
		}
		if hc.cancel != nil {
			hc.cancel()
		}
	})
	<-ctx.Done()
	return "", ctx.Err()
}

func TestRelease_CtrlCDuringAChunkCallsClassifyCall_Fallback_AbortsWithNoFurtherCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedInterruptedRelease(t, root, f, true)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered := make(chan struct{})
	tr := cancellingClassify(isSecondPartPrompt, cancel, entered)
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	deps.Transport = &heldChunkCalls{next: tr, entered: entered}

	err := engine.Release(ctx, deps, priorTagNormalAIOptions())

	assertInterrupted(t, err, f)
	if len(tr.prompts) != 3 || tr.count(isClassifyPrompt) != 1 {
		t.Errorf("AI calls = %d, want the two chunk calls and the one classify call", len(tr.prompts))
	}
}

func TestRelease_CtrlCDuringAHalvingRoundsCall_Fallback_AbortsWithNoFurtherCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// About 10 KB a file under the default cap: one call, which halves with room for diff.
	writeConfig(t, root, "[release]\non_notes_failure = \"fallback\"\n")
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
		ScriptedOut("subject one\n"), // log --format=%s
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr := &classifySkipTransport{
		fails:    func(prompt string) bool { return !isChunkCallPrompt(prompt) },
		failure:  &ai.GenerationError{Stdout: modelRejectsSize + "\n", ExitCode: 1},
		classify: func(context.Context) (string, error) { return "SIZE", nil },
	}
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	deps.Transport = &heldChunkCalls{next: tr, cancel: cancel}

	err := engine.Release(ctx, deps, priorTagNormalAIOptions())

	assertInterrupted(t, err, f)
	if n := tr.count(isChunkCallPrompt); n == 0 {
		t.Fatal("chunk calls = 0, want the halving round's calls the Ctrl-C landed in")
	}
	if n := tr.count(isMergeCallPrompt); n != 0 {
		t.Errorf("merge calls = %d, want none after the Ctrl-C", n)
	}
	if n := tr.count(isClassifyPrompt); n != 1 {
		t.Errorf("classify calls = %d, want only the single-pass call's", n)
	}
}
