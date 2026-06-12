package commit_test

import (
	"context"
	"os"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// noAIDeps assembles production-shaped Deps for a --no-ai run over an editorRunner:
// the recording presenter, the editorRunner as the read/interactive seam, and the
// lock-resilient git Mutator (git_safe) as the staging+commit sink wrapping the SAME
// editorRunner. NoAI is set; the transport is left nil (the --no-ai path must never
// reach it). Root is a TempDir so config.Load reads no real repo config.
func noAIDeps(rec *presentertest.RecordingPresenter, er *editorRunner, mode commit.StagingMode, root string) commit.Deps {
	// NoAI is set; the transport is left nil (the --no-ai path must never reach it). The
	// interactive editor-fallback tests exercise the TTY path: a TTY stdin and no -y
	// (StdinInteractive defaults true), so the no-message-source fail-loud guard (task
	// 3-5) does NOT fire and the editor opens. The guard's own preconditions are covered
	// in run_failloud_test.go.
	return editorDeps(rec, er, editorDepsOptions{Root: root, Staging: mode, NoAI: true})
}

// editorGitInvocations returns the recorded `git` invocations made through the
// editorRunner's embedded FakeRunner, in order.
func editorGitInvocations(er *editorRunner) []runner.Invocation {
	return gitInvocationsOf(er.fake.Invocations())
}

func editorAddInvocations(er *editorRunner) []runner.Invocation {
	return gitVerbInvocations(er.fake.Invocations(), "add")
}

func editorCommitInvocations(er *editorRunner) []runner.Invocation {
	return gitVerbInvocations(er.fake.Invocations(), "commit")
}

// seedNoAIDefault scripts the --no-ai default-mode git thread on a FakeRunner: the
// empty-index preflight read (non-empty), the `git var GIT_EDITOR` resolution
// (returning editor), then the `git commit -F -` on a non-empty save. No `git add`
// runs under the default mode.
func seedNoAIDefault(editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
	)
	return f
}

// TestRun_NoAI_SkipsAI_NoTransportCall proves --no-ai skips L3 generate entirely: the
// transport is never reached (it is nil in noAIDeps — a call would panic) and no
// `claude`/ai_command invocation is recorded among the runner calls.
func TestRun_NoAI_SkipsAI_NoTransportCall(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: "feat: human message\n"}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, inv := range er.fake.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("a `claude` AI invocation was recorded under --no-ai: %v", inv)
		}
	}
	// No ShowMessage/Prompt gate is rendered on the --no-ai path (the AI message render
	// + Continue? gate are AI-path-only).
	if containsKind(rec.Kinds(), presentertest.KindPrompt) {
		t.Errorf("kinds = %v, want NO Prompt gate on the --no-ai path", rec.Kinds())
	}
}

// TestRun_NoAI_OpensEditorViaMint_NotGitCommit proves mint opens the editor itself: a
// direct editor launch is recorded (RunInteractive on the resolved editor) and the
// only `git commit` is mint's own `commit -F -` sink — there is NO bare `git commit`
// delegation that would open the editor for mint.
func TestRun_NoAI_OpensEditorViaMint_NotGitCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: "feat: human message\n"}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (mint opens the editor itself)", len(er.launches))
	}
	if er.launches[0].Name != "myedit" {
		t.Errorf("launched editor = %q, want the resolved %q", er.launches[0].Name, "myedit")
	}

	// mint's only `git commit` is the `-F -` sink (the body piped via stdin), NOT a bare
	// interactive `git commit` that would open the editor for mint.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 {
		t.Fatalf("git commit invocations = %d (%v), want exactly 1 (mint's own `commit -F -` sink)", len(commits), commits)
	}
	if !containsArg(commits[0].Args, "-F") || !containsArg(commits[0].Args, "-") {
		t.Errorf("commit argv = %v, want `commit -F -` (mint pipes the saved body; it does not delegate to an interactive git commit)", commits[0].Args)
	}
}

// TestRun_NoAI_EditorBufferIsEmptyTemplate proves the editor opens with an EMPTY
// buffer — no synthetic stub message is inserted. The double captures the temp-file
// contents at launch (before its own save-back).
func TestRun_NoAI_EditorBufferIsEmptyTemplate(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: "feat: human message\n"}
	var opened string
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if opened != "" {
		t.Errorf("editor opened with buffer %q; the --no-ai buffer must be empty (no synthetic stub)", opened)
	}
}

// TestRun_NoAI_NonEmptySaveUnderAll_AddsTrackedThenCommits proves a non-empty save
// under -a applies `git add -u` then commits the saved body, in that order.
func TestRun_NoAI_NonEmptySaveUnderAll_AddsTrackedThenCommits(t *testing.T) {
	t.Parallel()

	const saved = "feat: staged tracked then committed\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff HEAD --name-only (preflight, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -u (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.All, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-u" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -u`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 {
		t.Fatalf("git commit invocations = %d, want exactly 1", len(commits))
	}
	if commits[0].Stdin != saved {
		t.Errorf("commit body = %q, want the saved buffer verbatim %q", commits[0].Stdin, saved)
	}
	assertAddBeforeCommit(t, er)
}

// TestRun_NoAI_NonEmptySaveUnderAddAll_AddsEverythingThenCommits proves a non-empty
// save under -A applies `git add -A` then commits, in that order.
func TestRun_NoAI_NonEmptySaveUnderAddAll_AddsEverythingThenCommits(t *testing.T) {
	t.Parallel()

	const saved = "feat: staged everything then committed\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff HEAD --name-only (preflight tracked, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -A (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.AddAll, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-A" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -A`", adds)
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	assertAddBeforeCommit(t, er)
}

// TestRun_NoAI_NonEmptySaveUnderDefault_CommitsIndexUnchanged proves a non-empty save
// under the default mode commits the existing index with NO `git add`.
func TestRun_NoAI_NonEmptySaveUnderDefault_CommitsIndexUnchanged(t *testing.T) {
	t.Parallel()

	const saved = "feat: default mode commits the index\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: saved}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("default mode ran `git add` %v under --no-ai; StagedOnly stages nothing", adds)
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// TestRun_NoAI_WhitespaceOnlySave_TrueNoOp proves a whitespace-only save is treated as
// empty — a true no-op: no `git add`, no `git commit`, no mutation. The non-zero abort
// tells the caller the commit did not happen.
func TestRun_NoAI_WhitespaceOnlySave_TrueNoOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		saved string
	}{
		{name: "EmptyString", saved: ""},
		{name: "SpacesAndNewlines", saved: "  \n\t\n  "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			f := runner.NewFakeRunner()
			// Only the preflight read + editor resolution are scripted; staging/commit must
			// never be reached on an empty save.
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},
			)
			er := &editorRunner{fake: f, saved: tt.saved}

			err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir()))
			if err == nil {
				t.Fatal("Run returned nil for a whitespace-only save; want a non-zero no-op abort")
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("empty save ran `git add` %v; an empty save is a true no-op", adds)
			}
			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("empty save created %d commit(s); an empty save is a true no-op", len(commits))
			}
		})
	}
}

// TestRun_NoAI_AbortedEditor_TrueNoOp proves an aborted/quit editor (RunInteractive
// returns a non-not-found error) is a true no-op: no `git add`, no `git commit`.
func TestRun_NoAI_AbortedEditor_TrueNoOp(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},
	)
	er := &editorRunner{fake: f, launchErr: errExitOne}

	err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir()))
	if err == nil {
		t.Fatal("Run returned nil for an aborted editor; want a non-zero no-op abort")
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("aborted editor ran `git add` %v; an abort is a true no-op", adds)
	}
	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("aborted editor created %d commit(s); an abort is a true no-op", len(commits))
	}
}

// TestRun_NoAI_NoContinueGate proves NO Continue? gate is rendered on the --no-ai path
// (the editor save IS the accept event; the gate is AI-path-only).
func TestRun_NoAI_NoContinueGate(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: "feat: no gate here\n"}

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if containsKind(rec.Kinds(), presentertest.KindPrompt) {
		t.Errorf("kinds = %v, want NO Prompt (no Continue? gate on the --no-ai path)", rec.Kinds())
	}
	if containsKind(rec.Kinds(), presentertest.KindShowMessage) {
		t.Errorf("kinds = %v, want NO ShowMessage (no AI message render on the --no-ai path)", rec.Kinds())
	}
}

// assertAddBeforeCommit fails unless the deferred `git add` ran BEFORE the commit
// mutation in the recorded git order.
func assertAddBeforeCommit(t *testing.T, er *editorRunner) {
	t.Helper()
	addIdx, commitIdx := -1, -1
	for i, inv := range editorGitInvocations(er) {
		if len(inv.Args) == 0 {
			continue
		}
		if inv.Args[0] == "add" && addIdx < 0 {
			addIdx = i
		}
		if inv.Args[0] == "commit" && commitIdx < 0 {
			commitIdx = i
		}
	}
	if addIdx < 0 || commitIdx < 0 {
		t.Fatalf("git calls = %v, want both an add and a commit", editorGitInvocations(er))
	}
	if addIdx >= commitIdx {
		t.Errorf("git add at %d, commit at %d; staging must run BEFORE the commit", addIdx, commitIdx)
	}
}
