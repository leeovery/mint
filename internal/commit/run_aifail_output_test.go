package commit_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// These proofs ride the REAL ai.Transport (deps.Transport nil) over the FakeRunner, so
// the AI tool's failure is scripted as the `claude` subprocess itself failing on both
// attempts — the shape a claude too old for the lean flags produces.

const unknownOptionOutput = "error: unknown option '--safe-mode'\n"

func seedClaudeFailing(f *runner.FakeRunner, stdout, stderr string) {
	f.Seed("claude", runner.Result{Stdout: stdout, Stderr: stderr, ExitCode: 1}, errors.New("exit status 1"))
}

func stageFailures(rec *presentertest.RecordingPresenter) []presenter.StageFailure {
	var out []presenter.StageFailure
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			out = append(out, ev.StageFailed)
		}
	}
	return out
}

func assertWarnings(t *testing.T, got, want []presenter.Warning) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("warnings = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("warning[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func assertStageFailures(t *testing.T, got, want []presenter.StageFailure) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("stage failures = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stage failure[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestRun_AIFailure_Attended_WarnCarriesToolOutputThenOpensEditor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
		stderr string
		want   string
	}{
		{name: "StdoutMessage", stdout: unknownOptionOutput, want: "error: unknown option '--safe-mode'"},
		{name: "StderrBehindWhitespaceStdout", stdout: " \n\t\n", stderr: "Credit balance is too low\n", want: "Credit balance is too low"},
		{name: "StderrBehindEmptyStdout", stderr: unknownOptionOutput, want: "error: unknown option '--safe-mode'"},
		{name: "BothStreams", stdout: "Prompt is too long\n", stderr: "warning: retrying\n", want: "Prompt is too long\n\nwarning: retrying"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const saved = "feat: human message after AI failed\n"
			f := seedAIFailFallback("myedit")
			seedClaudeFailing(f, tt.stdout, tt.stderr)
			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: f, saved: saved}

			if err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Root: t.TempDir()})); err != nil {
				t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor", err)
			}

			assertWarnings(t, warnEvents(rec), []presenter.Warning{{
				Label:   "ai",
				Message: "could not generate a message, opening editor",
				Key:     "message",
				Output:  tt.want,
			}})
			if len(er.launches) != 1 {
				t.Fatalf("editor launches = %d, want 1 (the editor opens after the warning)", len(er.launches))
			}
			if commits := editorCommitInvocations(er); len(commits) != 1 || commits[0].Stdin != saved {
				t.Fatalf("commit invocations = %v, want exactly one carrying %q", commits, saved)
			}
		})
	}
}

func TestRun_AIFailure_Unattended_FailLoudCarriesToolOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opts   editorDepsOptions
		stdout string
		stderr string
		want   string
	}{
		{name: "UnderYes", opts: editorDepsOptions{Yes: true}, stdout: unknownOptionOutput, want: "error: unknown option '--safe-mode'"},
		{name: "NonTTYStdin", opts: editorDepsOptions{NonInteractiveStdin: true}, stdout: unknownOptionOutput, want: "error: unknown option '--safe-mode'"},
		{name: "UnderYes_StderrBehindWhitespaceStdout", opts: editorDepsOptions{Yes: true}, stdout: "  \n", stderr: "Credit balance is too low\n", want: "Credit balance is too low"},
		{name: "NonTTYStdin_StderrBehindWhitespaceStdout", opts: editorDepsOptions{NonInteractiveStdin: true}, stdout: "  \n", stderr: "Credit balance is too low\n", want: "Credit balance is too low"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := seedAIPreflightOnly()
			seedClaudeFailing(f, tt.stdout, tt.stderr)
			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: f, saved: "feat: should never be saved\n"}
			opts := tt.opts
			opts.Root = t.TempDir()

			err := commit.Run(context.Background(), editorDeps(rec, er, opts))

			assertFailLoudNoMutation(t, rec, er, err)
			assertStageFailures(t, stageFailures(rec), []presenter.StageFailure{{
				Name:    "editor",
				Key:     "message",
				Message: failLoudMessage,
				Output:  tt.want,
			}})
			assertWarnings(t, warnEvents(rec), nil)
		})
	}
}

func TestRun_RegenFailure_WarnCarriesRegenerationOutput(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message after regeneration failed\n"
	f := seedRegenFailFallback("myedit")
	f.SeedSequence("claude",
		runner.ScriptedCall{Result: runner.Result{Stdout: "feat: initial generated\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "first attempt\n", ExitCode: 1}, Err: errors.New("exit status 1")},
		runner.ScriptedCall{Result: runner.Result{Stdout: unknownOptionOutput, ExitCode: 1}, Err: errors.New("exit status 1")},
	)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{""},
	}
	er := &editorRunner{fake: f, saved: saved}

	if err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Root: t.TempDir()})); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor", err)
	}

	assertWarnings(t, warnEvents(rec), []presenter.Warning{{
		Label:   "ai",
		Message: "could not generate a message, opening editor",
		Key:     "message",
		Output:  "error: unknown option '--safe-mode'",
	}})
	if len(er.launches) != 1 {
		t.Fatalf("editor launches = %d, want 1 (the editor opens after the warning)", len(er.launches))
	}
	if commits := editorCommitInvocations(er); len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying %q", commits, saved)
	}
}

func TestRun_NonAIFallbackTriggers_CarryNoAIOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		noAI     bool
		root     func(t *testing.T) string
		l1Diff   string
		attended []presenter.Warning
	}{
		{
			name: "NoAI",
			noAI: true,
			root: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name:     "Oversized",
			root:     oversizedRoot,
			l1Diff:   "diff --git a/x b/x\n+work\n",
			attended: []presenter.Warning{{Label: "diff", Message: oversizedNote, Key: "message"}},
		},
		{
			name: "AllExcluded",
			root: func(t *testing.T) string {
				dir := t.TempDir()
				writeDiffExclude(t, dir, "*.min.js")
				return dir
			},
			attended: []presenter.Warning{{Label: "diff", Message: "all changes are excluded from the AI context — opening editor", Key: "message"}},
		},
	}

	seed := func(noAI bool, l1Diff string, tail ...runner.ScriptedCall) *runner.FakeRunner {
		f := runner.NewFakeRunner()
		calls := []runner.ScriptedCall{scriptedHead(), {Result: runner.Result{Stdout: "x\n"}}}
		if !noAI {
			calls = append(calls, runner.ScriptedCall{Result: runner.Result{Stdout: l1Diff}})
		}
		f.SeedSequence("git", append(calls, tail...)...)
		// A failing claude that must never be reached: if it were, its output would leak.
		seedClaudeFailing(f, unknownOptionOutput, "")
		return f
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name+"/Attended", func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			f := seed(tt.noAI, tt.l1Diff,
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
				runner.ScriptedCall{}, // git commit -F -
				scriptedShortHead(),   // git rev-parse --short HEAD (the end station)
			)
			er := &editorRunner{fake: f, saved: "chore: human message\n"}
			deps := editorDeps(rec, er, editorDepsOptions{Root: tt.root(t), NoAI: tt.noAI})

			if err := commit.Run(context.Background(), deps); err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}
			assertWarnings(t, warnEvents(rec), tt.attended)
			assertStageFailures(t, stageFailures(rec), nil)
		})
		t.Run(tt.name+"/Unattended", func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: seed(tt.noAI, tt.l1Diff), saved: "feat: should never be saved\n"}
			deps := editorDeps(rec, er, editorDepsOptions{Root: tt.root(t), NoAI: tt.noAI, Yes: true})

			err := commit.Run(context.Background(), deps)

			assertFailLoudNoMutation(t, rec, er, err)
			assertStageFailures(t, stageFailures(rec), []presenter.StageFailure{{Name: "editor", Key: "message", Message: failLoudMessage}})
			assertWarnings(t, warnEvents(rec), nil)
		})
	}
}

func TestRun_AIFailure_MakesNoClassifyCall_EditorFallbackRuns(t *testing.T) {
	t.Parallel()

	f := seedAIFailFallback("myedit")
	seedClaudeFailing(f, "Prompt is too long\n", "")
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: f, saved: "feat: human message after AI failed\n"}

	if err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Root: t.TempDir()})); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor", err)
	}

	invs := invocationsNamed(f, "claude")
	if len(invs) != 2 {
		t.Fatalf("claude invocations = %d, want only the commit prompt's two attempts", len(invs))
	}
	if invs[0].Stdin == "" || invs[1].Stdin != invs[0].Stdin {
		t.Errorf("claude stdins = %q, %q; want the same commit prompt on both attempts", invs[0].Stdin, invs[1].Stdin)
	}
	if len(er.launches) != 1 {
		t.Errorf("editor launches = %d, want 1", len(er.launches))
	}
}
