package engine_test

import (
	"context"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

type singlePassFailureCase struct {
	name        string
	failure     error
	wantMessage string
	wantOutput  string
}

func singlePassFailureCases() []singlePassFailureCase {
	return []singlePassFailureCase{
		{
			name:        "output on stdout",
			failure:     &ai.GenerationError{Stdout: "Not logged in · Please run /login\n", ExitCode: 1},
			wantMessage: "AI call failed",
			wantOutput:  "Not logged in · Please run /login",
		},
		{
			name:        "stdout empty, a message on stderr",
			failure:     &ai.GenerationError{Stderr: "error: connection refused\n", ExitCode: 1},
			wantMessage: "AI call failed",
			wantOutput:  "error: connection refused",
		},
		{
			name:        "whitespace body, whitespace on stderr",
			failure:     &ai.GenerationError{Stdout: "  \n", Stderr: "\t\n"},
			wantMessage: "AI returned empty notes",
		},
		{
			name:        "timeout",
			failure:     fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout),
			wantMessage: "AI timed out",
		},
		{
			name:        "program not installed",
			failure:     fmt.Errorf("%w: claude", ai.ErrCommandMissing),
			wantMessage: "AI tool not installed",
		},
	}
}

func TestRelease_SinglePassNotesFailure_TheLineNamesTheCause(t *testing.T) {
	t.Parallel()

	for _, tc := range singlePassFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedNormalAINotes(f)
			rec := &presentertest.RecordingPresenter{}
			deps := newDeps(rec, f)
			deps.Transport = &freshTransport{err: tc.failure}

			err := engine.Release(t.Context(), deps, priorTagNormalAIOptions())

			assertAbortNonZero(t, err)
			sf := onlyStageFailureEvent(t, rec)
			if sf.Name != "notes" || sf.Message != tc.wantMessage || sf.Output != tc.wantOutput {
				t.Errorf("StageFailure = %q %q with output %q, want notes %q with %q", sf.Name, sf.Message, sf.Output, tc.wantMessage, tc.wantOutput)
			}
		})
	}
}

func TestRegenerateRun_FreshSinglePassNotesFailure_TheLineNamesTheCause(t *testing.T) {
	t.Parallel()

	for _, tc := range singlePassFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}
			notesGit := seedFreshGit(priorTagDiff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
			req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
			req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ notes.Progress) (string, error) {
				return engine.RegenerateFreshBody(ctx, notesGit, &freshTransport{err: tc.failure}, dir, freshCfg(), version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, nil)
			}

			if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), newFakePublisher(), dir, req); err == nil {
				t.Fatal("RegenerateRun returned nil error for a failed notes call, want an abort")
			}

			sf := onlyStageFailureEvent(t, rec)
			if sf.Message != tc.wantMessage || sf.Output != tc.wantOutput {
				t.Errorf("StageFailure = %q with output %q, want %q with %q", sf.Message, sf.Output, tc.wantMessage, tc.wantOutput)
			}
		})
	}
}
