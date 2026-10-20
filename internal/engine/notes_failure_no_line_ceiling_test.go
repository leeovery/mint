package engine_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// overOldLineCeilingDiff is a one-file diff of 50,001 added lines: past the removed
// 50,000-line ceiling, far under the default max_prompt_bytes.
func overOldLineCeilingDiff() string {
	return "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1,50001 @@\n" + strings.Repeat("+x\n", 50001)
}

func TestRelease_NotesFailureOverOldLineCeiling_CarriesAICausePhrase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(overOldLineCeilingDiff()),
		ScriptedOut("A\tauth/login.go\n"),
		ScriptedOut("50001\t0\tauth/login.go\n"),
	)
	f.Seed("claude", runner.Result{Stdout: "Prompt is too long", ExitCode: 1}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	sf := onlyStageFailureEvent(t, rec)
	if sf.Message != "AI call failed" {
		t.Errorf("StageFailure.Message = %q, want the AI failure's phrase %q", sf.Message, "AI call failed")
	}
	if sf.Output != "Prompt is too long" {
		t.Errorf("StageFailure.Output = %q, want the AI's captured output", sf.Output)
	}
}

func TestRegenerateRun_FreshNotesFailureOverOldLineCeiling_CarriesAICausePhrase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{}

	notesGit := seedFreshGit(overOldLineCeilingDiff(), "A\tauth/login.go\n", "50001\t0\tauth/login.go\n")
	tr := &freshTransport{err: fmt.Errorf("transport: %w", ai.ErrGenerationFailed)}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease), false)
	req.ProduceBody = func(ctx context.Context, _ engine.RegenerateSource, _ engine.NotesSinks) (string, error) {
		return engine.RegenerateFreshBody(ctx, notesGit, tr, dir, freshCfg(), version.Resolution{Tag: regenRunTag, PreviousTag: "v1.3.0"}, engine.NotesSinks{})
	}

	if err := engine.RegenerateRun(t.Context(), freshRunDeps(rec, f), pub, dir, req); err == nil {
		t.Fatal("RegenerateRun returned nil error for a failed notes call, want an abort")
	}

	if tr.calls() != 1 {
		t.Errorf("transport called %d times, want the single-pass call", tr.calls())
	}
	sf := onlyStageFailureEvent(t, rec)
	if sf.Message != "AI returned empty notes" {
		t.Errorf("StageFailure.Message = %q, want the AI failure's phrase %q", sf.Message, "AI returned empty notes")
	}
}

func TestRegenerateAll_NotesFailureSkip_NeverReadsDiffTooLarge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		reason string
	}{
		{name: "AI call failed", err: fmt.Errorf("generating notes: %w", ai.ErrGenerationFailed), reason: "AI returned empty notes"},
		{name: "AI timed out", err: fmt.Errorf("generating notes: %w", ai.ErrTimeout), reason: "AI timed out"},
		{name: "size sentinel", err: fmt.Errorf("generating notes: %w", notes.ErrDiffTooLarge), reason: "notes generation failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			pub := newFakePublisher()
			pub.seedExists(batchV1Tag, true, nil)
			pub.seedExists(batchV2Tag, true, nil)
			pub.seedExists(batchV3Tag, true, nil)
			rec := &presentertest.RecordingPresenter{}

			req := batchReq(engine.RegenerateSourceFresh, threeVersions(), true)
			req.ProduceBody = func(_ context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, _ engine.NotesSinks) (string, error) {
				if res.Tag == batchV2Tag {
					return "", tt.err
				}
				return "## fresh " + res.Tag + "\n", nil
			}

			if _, err := engine.RegenerateAll(t.Context(), batchDeps(rec, f), pub, t.TempDir(), req); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			var skips []string
			for _, ev := range rec.Events {
				if ev.Kind == presentertest.KindWarn && ev.Warn.Label == "skipped "+batchV2Tag {
					skips = append(skips, ev.Warn.Message)
				}
			}
			if len(skips) != 1 || skips[0] != tt.reason {
				t.Errorf("skip warnings for %s = %q, want exactly [%q]", batchV2Tag, skips, tt.reason)
			}
			summary := finishEvent(t, rec).Summary
			if want := "2 regenerated, 1 skipped: " + batchV2Tag + " (" + tt.reason + ")"; summary != want {
				t.Errorf("RunFinished.Summary = %q, want %q", summary, want)
			}
			for _, text := range append(skips, summary) {
				if strings.Contains(text, "diff too large") {
					t.Errorf("%q reads diff too large", text)
				}
			}
		})
	}
}
