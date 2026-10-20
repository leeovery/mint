package engine_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// isClassifyPrompt reports whether prompt is a classify call's, by its opening sentence.
func isClassifyPrompt(prompt string) bool {
	return strings.HasPrefix(prompt, "A command asked to write release notes has failed.")
}

// aiCall is one AI-tool invocation as the runner received it.
type aiCall struct {
	name      string
	stdin     string
	remaining time.Duration
}

// deadlineRunner is the FakeRunner with every non-git RunWith call's program, stdin
// and remaining per-attempt deadline recorded, since the FakeRunner drops the context.
type deadlineRunner struct {
	*runner.FakeRunner

	mu    sync.Mutex
	calls []aiCall
}

func (d *deadlineRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	if name == "git" {
		return d.FakeRunner.RunWith(ctx, stdin, name, args...)
	}
	var in []byte
	if stdin != nil {
		in, _ = io.ReadAll(stdin)
	}
	var remaining time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		remaining = time.Until(deadline)
	}
	d.mu.Lock()
	d.calls = append(d.calls, aiCall{name: name, stdin: string(in), remaining: remaining})
	d.mu.Unlock()
	return d.FakeRunner.RunWith(ctx, strings.NewReader(string(in)), name, args...)
}

const perVerbConfig = "[release]\nai_command = \"tool-a\"\ntimeout = 30\n\n[commit]\nai_command = \"tool-b\"\n"

// seedToolAFailsThenClassified scripts tool-a failing both notes attempts with output,
// then answering the classify call OTHER.
func seedToolAFailsThenClassified(f *runner.FakeRunner) {
	failed := runner.ScriptedCall{
		Result: runner.Result{Stdout: "Not logged in · Please run /login\n", ExitCode: 1},
		Err:    errors.New("exit status 1"),
	}
	f.SeedSequence("tool-a", failed, failed, ScriptedOut("OTHER\n"))
}

// assertClassifiedByReleaseCommand fails unless the AI tool was tool-a throughout,
// with the classify call third, under a 30-second per-attempt deadline.
func assertClassifiedByReleaseCommand(t *testing.T, d *deadlineRunner) {
	t.Helper()
	if len(d.calls) != 3 {
		t.Fatalf("AI calls = %d, want two notes attempts then one classify call", len(d.calls))
	}
	for i, call := range d.calls {
		if call.name != "tool-a" {
			t.Errorf("AI call %d ran %q, want the [release].ai_command %q", i, call.name, "tool-a")
		}
	}
	classify := d.calls[2]
	if !isClassifyPrompt(classify.stdin) || !strings.Contains(classify.stdin, "Not logged in · Please run /login") {
		t.Errorf("third AI call stdin =\n%.400s\nwant the classify prompt carrying the failed call's output", classify.stdin)
	}
	if classify.remaining > 30*time.Second || classify.remaining < 25*time.Second {
		t.Errorf("classify call deadline remaining = %v, want about the [release].timeout of 30s", classify.remaining)
	}
}

func TestRelease_NotesCallFails_ClassifyRunsTheReleaseCommandAndTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, perVerbConfig)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	seedToolAFailsThenClassified(f)
	rec := &presentertest.RecordingPresenter{}
	d := &deadlineRunner{FakeRunner: f}
	deps := newDeps(rec, f)
	deps.Runner = d

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	assertClassifiedByReleaseCommand(t, d)
}

func TestRegenerateFreshBody_NotesCallFails_ClassifyRunsTheReleaseCommandAndTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, perVerbConfig)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	f := seedFreshGit(priorTagDiff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	seedToolAFailsThenClassified(f)
	d := &deadlineRunner{FakeRunner: f}

	_, err = engine.RegenerateFreshBody(t.Context(), d, nil, root, cfg, version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, engine.NotesSinks{})

	if !errors.Is(err, ai.ErrGenerationFailed) {
		t.Fatalf("RegenerateFreshBody error = %v, want the notes call's generation failure", err)
	}
	assertClassifiedByReleaseCommand(t, d)
}

// classifiedOtherTransport fails every notes call with failure and answers a classify
// call OTHER, recording the classify prompts.
type classifiedOtherTransport struct {
	failure  error
	classify []string
}

func (ct *classifiedOtherTransport) Generate(_ context.Context, prompt string) (string, error) {
	if isClassifyPrompt(prompt) {
		ct.classify = append(ct.classify, prompt)
		return "OTHER", nil
	}
	return "", ct.failure
}

func TestRelease_ClassifiedOther_TheLineShowsTheNotesCallsOutputVerbatim(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	rec := &presentertest.RecordingPresenter{}
	tr := &classifiedOtherTransport{failure: &ai.GenerationError{Stdout: "Not logged in · Please run /login\n", ExitCode: 1}}
	deps := newDeps(rec, f)
	deps.Transport = tr

	err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	if len(tr.classify) != 1 {
		t.Fatalf("classify calls = %d, want one", len(tr.classify))
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Name != "notes" || sf.Message != "AI call failed" || sf.Output != "Not logged in · Please run /login" {
		t.Errorf("StageFailure = %q %q with output %q, want notes %q with the notes call's output verbatim", sf.Name, sf.Message, sf.Output, "AI call failed")
	}
}
