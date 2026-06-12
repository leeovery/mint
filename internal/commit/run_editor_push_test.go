package commit_test

import (
	"context"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// editorPushInvocations returns every recorded `git push` invocation made through the
// editorRunner's embedded FakeRunner, in order — the SAME shared auto-push step the
// gate-accept path runs (pushAfterCommit), reused by the editor save-as-accept path.
func editorPushInvocations(er *editorRunner) []runner.Invocation {
	return gitVerbInvocations(er.fake.Invocations(), "push")
}

// editorGitIndexOf returns the position (within the ordered editorRunner `git`
// invocations) of the first call whose first arg equals verb, or -1 — used to assert
// the strict stage -> commit -> push ordering on the editor save-as-accept path.
func editorGitIndexOf(er *editorRunner, verb string) int {
	for i, inv := range editorGitInvocations(er) {
		if len(inv.Args) > 0 && inv.Args[0] == verb {
			return i
		}
	}
	return -1
}

// seedNoAIDefaultThenPush scripts the --no-ai default-mode git thread WITH an armed
// push: the empty-index preflight read (non-empty), the `git var GIT_EDITOR`
// resolution, the `git commit -F -` on a non-empty save, then the `git push` that
// follows a successful save-as-accept commit. No `git add` runs under the default mode.
func seedNoAIDefaultThenPush(editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
		runner.ScriptedCall{}, // git push
	)
	return f
}

// TestRun_NoAI_PushArmed_EditorSaveCommitsThenPushes proves a non-empty editor save
// (save-as-accept) commits THEN pushes when -p is armed: exactly one `git push` runs,
// strictly after the commit, on the same git_safe runner the commit flows through.
func TestRun_NoAI_PushArmed_EditorSaveCommitsThenPushes(t *testing.T) {
	t.Parallel()

	const saved = "feat: editor save then push\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefaultThenPush("myedit"), saved: saved}
	deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	pushes := editorPushInvocations(er)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1 after the save-as-accept commit", len(pushes), pushes)
	}
	commitIdx := editorGitIndexOf(er, "commit")
	pushIdx := editorGitIndexOf(er, "push")
	if commitIdx >= pushIdx {
		t.Errorf("commit at %d, push at %d; the push must run strictly AFTER the commit", commitIdx, pushIdx)
	}
}

// TestRun_NoAI_PushArmed_AddAll_EndToEnd proves `mint commit -Ap --no-ai` runs
// stage -> commit -> push end-to-end on a non-empty editor save: no AI/transport call
// (transport is nil under noAIDeps), then `git add -A`, `git commit -F -`, `git push`,
// in that order, all via the git_safe Mutator.
func TestRun_NoAI_PushArmed_AddAll_EndToEnd(t *testing.T) {
	t.Parallel()

	const saved = "feat: stage commit push end to end\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff HEAD --name-only (preflight tracked, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -A (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
		runner.ScriptedCall{}, // git push
	)
	er := &editorRunner{fake: f, saved: saved}
	deps := noAIDeps(rec, er, commit.AddAll, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// No AI: the transport is nil under noAIDeps (a call would panic), and no `claude`
	// invocation is recorded.
	for _, inv := range er.fake.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("a `claude` AI invocation was recorded under --no-ai: %v", inv)
		}
	}

	// stage -> commit -> push, each exactly once and in order.
	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-A" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -A`", adds)
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	if pushes := editorPushInvocations(er); len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1", len(pushes), pushes)
	}

	addIdx := editorGitIndexOf(er, "add")
	commitIdx := editorGitIndexOf(er, "commit")
	pushIdx := editorGitIndexOf(er, "push")
	if addIdx >= commitIdx || commitIdx >= pushIdx {
		t.Errorf("git order add=%d commit=%d push=%d; want add < commit < push", addIdx, commitIdx, pushIdx)
	}
}

// TestRun_NoAI_PushArmed_SingleSharedPushStep proves the editor-path push reuses the
// SINGLE shared push step — exactly ONE push invocation is recorded, never a second /
// parallel push call.
func TestRun_NoAI_PushArmed_SingleSharedPushStep(t *testing.T) {
	t.Parallel()

	const saved = "feat: single shared push\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefaultThenPush("myedit"), saved: saved}
	deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if pushes := editorPushInvocations(er); len(pushes) != 1 {
		t.Fatalf("git push invocations = %d; the editor path must reuse the single shared push step (exactly 1, no parallel push)", len(pushes))
	}
}

// TestRun_NoAI_PushArmed_PlainPushNoUpstreamArgs proves the editor-path push is a PLAIN
// `git push` — argv exactly ["push"], no upstream/remote/branch args, no -u — and
// carries no stdin (push has no body), identical to the gate-path push (5-2).
func TestRun_NoAI_PushArmed_PlainPushNoUpstreamArgs(t *testing.T) {
	t.Parallel()

	const saved = "feat: plain push from editor\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefaultThenPush("myedit"), saved: saved}
	deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	pushes := editorPushInvocations(er)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1", len(pushes), pushes)
	}
	if got := pushes[0].Args; len(got) != 1 || got[0] != "push" {
		t.Errorf("push argv = %v, want exactly [\"push\"] (no upstream/remote/branch args, no -u)", got)
	}
	if pushes[0].Stdin != "" {
		t.Errorf("push stdin = %q, want empty (push carries no body)", pushes[0].Stdin)
	}
}

// TestRun_AIFailure_PushArmed_EditorSaveCommitsThenPushes proves an AI-failure (3-3)
// editor drop with -p commits THEN pushes on a non-empty save: a transport failure
// routes to the editor, the non-empty save commits, then the armed push follows.
func TestRun_AIFailure_PushArmed_EditorSaveCommitsThenPushes(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message after AI failed, then push\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff --cached -- . (L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},                    // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
		runner.ScriptedCall{}, // git push
	)
	er := &editorRunner{fake: f, saved: saved}
	tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}
	deps := aiFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v; want a fall-back to the editor then push, not an abort", err)
	}

	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	pushes := editorPushInvocations(er)
	if len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1 after the AI-failure save-as-accept commit", len(pushes), pushes)
	}
	commitIdx := editorGitIndexOf(er, "commit")
	pushIdx := editorGitIndexOf(er, "push")
	if commitIdx >= pushIdx {
		t.Errorf("commit at %d, push at %d; the push must run strictly AFTER the commit", commitIdx, pushIdx)
	}
}

// TestRun_NoAI_PushUnarmed_EditorSaveCommitsNoPush proves that with -p UNARMED the
// editor save commits but does NOT push — the shared push step is a no-op when
// disarmed.
func TestRun_NoAI_PushUnarmed_EditorSaveCommitsNoPush(t *testing.T) {
	t.Parallel()

	const saved = "feat: editor save, no push when unarmed\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: saved}
	// Push left at the zero value (false): no push.

	if err := commit.Run(context.Background(), noAIDeps(rec, er, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := editorCommitInvocations(er); len(got) != 1 {
		t.Fatalf("commit invocations = %d, want exactly 1 (the save still commits)", len(got))
	}
	if pushes := editorPushInvocations(er); len(pushes) != 0 {
		t.Errorf("unarmed editor save created %d push(es); no push without -p", len(pushes))
	}
}
