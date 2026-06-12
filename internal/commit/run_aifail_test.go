package commit_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// aiFailDeps assembles production-shaped Deps for an AI-generate-failure run over an
// editorRunner: the recording presenter, the editorRunner as the read/interactive
// seam, and the lock-resilient git Mutator (git_safe) as the staging+commit sink
// wrapping the SAME editorRunner. The supplied transport returns the sentinel under
// test so the generate step fails; NoAI is FALSE (this is the AI path failing, not
// --no-ai). Root is a TempDir so config.Load reads no real repo config.
func aiFailDeps(rec *presentertest.RecordingPresenter, er *editorRunner, tr commit.Transport, mode commit.StagingMode, root string) commit.Deps {
	// These tests exercise the TTY editor-fallback path (a TTY stdin, no -y), so the
	// no-message-source fail-loud guard (task 3-5) does NOT fire and the AI failure
	// reaches the editor (StdinInteractive defaults true). The guard's own
	// preconditions live in run_failloud_test.go.
	return editorDeps(rec, er, editorDepsOptions{Transport: tr, Root: root, Staging: mode})
}

// failTransport is a Transport whose Generate ALWAYS returns the wrapped sentinel,
// recording how many times it was invoked. It stands in for the L2 transport having
// exhausted its OWN behaviour (the one bad-content retry happens INSIDE the real
// transport; commit consumes the typed failure, it never re-runs the transport). The
// call count proves commit calls Generate exactly once and adds no retry of its own.
type failTransport struct {
	err   error
	calls int
}

func (f *failTransport) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "", f.err
}

// seedAIFailFallback scripts the git thread for an AI-generate-failure that falls
// back to the editor under the DEFAULT (staged-only) mode: the empty-index preflight
// read (non-empty), the L1 staged diff read, the `git var GIT_EDITOR` resolution,
// then the `git commit -F -` on a non-empty save. No `git add` runs under the default
// mode. The transport (not the runner) supplies the generate failure.
func seedAIFailFallback(editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff --cached -- . (L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}},                 // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
	)
	return f
}

// TestRun_AIFailure_GenerationFailed_RoutesToEditor proves an ai.ErrGenerationFailed
// (the transport's bad content surviving its one retry) routes to the editor fallback
// — a RunInteractive launch is recorded and the saved body is committed — rather than
// aborting like the bare-path generate-failure surface.
func TestRun_AIFailure_GenerationFailed_RoutesToEditor(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message after AI failed\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: saved}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor, not an abort", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the AI failure routes to the editor)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_AIFailure_Timeout_RoutesToEditor proves an ai.ErrTimeout routes to the
// editor fallback immediately (the transport never retried it), not an abort.
func TestRun_AIFailure_Timeout_RoutesToEditor(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message after AI timed out\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: saved}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrTimeout)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor, not an abort", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (a timeout routes to the editor)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_AIFailure_CommandMissing_RoutesToEditor proves an ai.ErrCommandMissing
// (the AI binary not on PATH) routes to the editor fallback immediately (never
// retried), not an abort.
func TestRun_AIFailure_CommandMissing_RoutesToEditor(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message; AI binary missing\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: saved}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrCommandMissing)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor, not an abort", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (a missing AI binary routes to the editor)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_AIFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits proves the editor
// fallback on the AI-failure path reuses save-as-accept UNCHANGED from --no-ai: a
// non-empty save under -a applies `git add -u` then commits the saved body, in that
// order.
func TestRun_AIFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits(t *testing.T) {
	t.Parallel()

	const saved = "feat: staged tracked then committed after AI failure\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff HEAD --name-only (preflight, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff HEAD -- . (L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},                    // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -u (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.All, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-u" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -u`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
	assertAddBeforeCommit(t, er)
}

// TestRun_AIFailure_EmptySave_TrueNoOp proves an empty/aborted editor on the
// AI-failure path is a true no-op: no `git add`, no `git commit`, a non-zero abort.
func TestRun_AIFailure_EmptySave_TrueNoOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		saved     string
		launchErr error
	}{
		{name: "WhitespaceOnlySave", saved: "  \n\t\n"},
		{name: "AbortedEditor", launchErr: errExitOne},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			f := runner.NewFakeRunner()
			// Only the preflight read, the L1 diff, and the editor resolution are scripted;
			// staging/commit must never be reached on an empty/aborted save.
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},
			)
			er := &editorRunner{fake: f, saved: tt.saved, launchErr: tt.launchErr}
			tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}

			err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir()))
			if err == nil {
				t.Fatal("Run returned nil for an empty/aborted editor; want a non-zero no-op abort")
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("empty/aborted editor ran `git add` %v; an empty save is a true no-op", adds)
			}
			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("empty/aborted editor created %d commit(s); an empty save is a true no-op", len(commits))
			}
		})
	}
}

// TestRun_AIFailure_EditorBufferIsEmptyTemplate proves the editor opens with an EMPTY
// buffer on the AI-failure path — NO synthetic stub and NO re-show of a partial
// message. The double captures the temp-file contents at launch (before its save-back).
func TestRun_AIFailure_EditorBufferIsEmptyTemplate(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: "feat: human message\n"}
	var opened string
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if opened != "" {
		t.Errorf("editor opened with buffer %q; the AI-failure buffer must be empty (no synthetic stub, no partial re-show)", opened)
	}
}

// TestRun_AIFailure_OversizedDiff_RoutesViaOversizedBranch proves the oversized-skip
// route is DISTINCT from the transport-failure route: a generate step surfacing
// notes.ErrDiffTooLarge routes through the OVERSIZED branch (3-4) — it emits the
// oversized note and falls back to the editor — NOT through the noteless transport-
// failure branch (3-3). The note's presence is the discriminator: the AI-failure branch
// never emits it. (The full oversized boundary is exercised end-to-end through the real
// Generator in run_oversized_test.go; this asserts only the branch boundary.)
func TestRun_AIFailure_OversizedDiff_RoutesViaOversizedBranch(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message for an oversized diff\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: saved}
	tr := &failTransport{err: fmt.Errorf("commit size guard: %w", notes.ErrDiffTooLarge)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v; an oversized diff falls back to the editor, it does not abort", err)
	}
	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (oversized routes to the editor)", len(er.launches))
	}
	// The oversized NOTE is the discriminator: it proves the oversized branch ran, not the
	// noteless transport-failure branch.
	gotNote := false
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Message == "diff too large to summarise — opening editor" {
			gotNote = true
		}
	}
	if !gotNote {
		t.Errorf("kinds = %v, want a Warn carrying the oversized note (proving the oversized branch, not the noteless AI-failure branch)", rec.Kinds())
	}
}

// TestRun_AIFailure_TransportNotReRun proves commit consumes the transport's one
// bad-content retry rather than re-implementing it: the injected transport is called
// EXACTLY ONCE — commit routes the typed failure to the editor without re-running the
// transport itself.
func TestRun_AIFailure_TransportNotReRun(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: "feat: human message\n"}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}

	if err := commit.Run(context.Background(), aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls != 1 {
		t.Errorf("transport.Generate called %d times; commit must consume the transport's own retry and call it exactly once (no re-run)", tr.calls)
	}
}
