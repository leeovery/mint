package commit_test

import (
	"context"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// Task 5-5 LOCKS two invariants around the shared push step (5-2/5-3):
//
//  1. Push is gated on commit success — a run that committed nothing performs NO push
//     even with -p armed. Two such no-op cases: a gate `n` abort (covered for -p in
//     run_push_test.go) and an empty/aborted editor save (locked HERE under -p).
//  2. No pre-push / remote-sync gate exists — the Preflight & Safety drops
//     (clean-working-tree, on-release-branch, remote-in-sync, no pre-push gate) stay
//     dropped, so a -p run attempts the push DIRECTLY with no behind/diverged precheck.
//     This is what lets `mint commit -Apy` run unattended end-to-end.
//
// These are invariant-confirmation + locking tests, NOT new push machinery: the push
// step is 5-2/5-3 and the failure warn is 5-4 (re-tested elsewhere, not here).

// gitArgVerbs returns the first arg of every recorded `git` invocation, in order — the
// spine of the "exact git invocation set, no probe" assertions on the -p happy path.
func gitArgVerbs(r *runner.FakeRunner) []string {
	var verbs []string
	for _, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 {
			verbs = append(verbs, inv.Args[0])
		}
	}
	return verbs
}

// assertNoRemoteSyncProbe fails if any recorded git verb/argv is a pre-push / remote-sync
// probe (fetch, rev-list, an @{upstream}/--count behind-ahead probe, ls-remote). The
// dropped gates are deliberately absent, so a -p run must contain NONE of these before
// (or after) the push — it attempts the push DIRECTLY.
func assertNoRemoteSyncProbe(t *testing.T, invs []runner.Invocation) {
	t.Helper()
	for _, inv := range invs {
		if inv.Name != "git" {
			continue
		}
		for _, a := range inv.Args {
			switch a {
			case "fetch", "rev-list", "ls-remote", "--count":
				t.Errorf("recorded a remote-sync/pre-push probe `git %v`; the dropped gates must stay dropped (no fetch/rev-list/upstream behind-ahead precheck)", inv.Args)
			}
			if strings.Contains(a, "@{upstream}") || strings.Contains(a, "@{u}") {
				t.Errorf("recorded an upstream behind/ahead probe `git %v`; no remote-sync precheck must run", inv.Args)
			}
		}
	}
}

// TestRun_NoAI_PushArmed_EmptyEditorSave_NoPushNoCommit proves the Phase 3 editor true
// no-op short-circuits the ENTIRE accept-and-push tail under -p: an empty (whitespace-
// only) save produces no staging, no commit, AND no push even with -p armed. The
// errEditorNoOp branch returns BEFORE pushAfterCommit is reachable.
func TestRun_NoAI_PushArmed_EmptyEditorSave_NoPushNoCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		saved string
	}{
		{name: "EmptyString", saved: ""},
		{name: "WhitespaceOnly", saved: "  \n\t\n  "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			f := runner.NewFakeRunner()
			// Only the preflight read + editor resolution are scripted; staging, commit, and
			// push must never be reached on an empty save.
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff --cached --name-only (non-empty index)
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
			)
			er := &editorRunner{fake: f, saved: tt.saved}
			deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
			deps.Push = true

			err := commit.Run(context.Background(), deps)
			if err == nil {
				t.Fatal("Run returned nil for an empty editor save under -p; want a non-zero no-op abort")
			}

			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("empty editor save created %d commit(s); a true no-op commits nothing", len(commits))
			}
			if pushes := editorPushInvocations(er); len(pushes) != 0 {
				t.Errorf("empty editor save with -p created %d push(es); no commit means no push", len(pushes))
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("empty editor save ran `git add` %v; a true no-op stages nothing", adds)
			}
		})
	}
}

// TestRun_NoAI_PushArmed_AbortedEditor_NoPushNoCommit proves an aborted/quit editor
// (RunInteractive returns a non-not-found error → ok=false) is a true no-op under -p:
// no staging, no commit, no push. The errEditorNoOp branch returns before the push step.
func TestRun_NoAI_PushArmed_AbortedEditor_NoPushNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
	)
	er := &editorRunner{fake: f, launchErr: errExitOne}
	deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
	deps.Push = true

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an aborted editor under -p; want a non-zero no-op abort")
	}

	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("aborted editor created %d commit(s); a true no-op commits nothing", len(commits))
	}
	if pushes := editorPushInvocations(er); len(pushes) != 0 {
		t.Errorf("aborted editor with -p created %d push(es); no commit means no push", len(pushes))
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("aborted editor ran `git add` %v; a true no-op stages nothing", adds)
	}
}

// TestRun_NoAI_PushArmed_AddAll_EmptyEditorSave_NoPushNoCommit proves the no-op short-
// circuit holds even with the headline `-Ap --no-ai` bundle: an empty save under -A
// performs NO `git add -A`, NO commit, and NO push. The deferred staging is part of the
// accept-and-push tail the empty save short-circuits — abort leaves the index untouched.
func TestRun_NoAI_PushArmed_AddAll_EmptyEditorSave_NoPushNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff HEAD --name-only (preflight tracked, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
	)
	er := &editorRunner{fake: f, saved: "   \n"} // whitespace-only save
	deps := noAIDeps(rec, er, commit.AddAll, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an empty -Ap save; want a non-zero no-op abort")
	}

	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("empty -Ap save ran `git add` %v; the deferred staging is part of the short-circuited tail", adds)
	}
	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("empty -Ap save created %d commit(s); a true no-op commits nothing", len(commits))
	}
	if pushes := editorPushInvocations(er); len(pushes) != 0 {
		t.Errorf("empty -Ap save created %d push(es); no commit means no push", len(pushes))
	}
}

// TestRun_PushArmed_NoRemoteSyncProbeBeforePush proves no pre-push / remote-sync gate
// runs on the gate-accept happy path with -p: the recorded git invocations are EXACTLY
// [diff(--name-only preflight), diff(cached L1), commit, push] — there is NO fetch, NO
// rev-list/--count, NO @{upstream} behind-ahead probe, and NO ls-remote. The only
// "remote" op is the plain `git push`, attempted directly with no behind/diverged
// precheck (the dropped gates stay dropped).
func TestRun_PushArmed_NoRemoteSyncProbeBeforePush(t *testing.T) {
	t.Parallel()

	const message = "feat: push with no pre-push gate"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// No remote-sync precheck anywhere in the recorded git calls.
	assertNoRemoteSyncProbe(t, r.Invocations())

	// The exact git verb sequence: diff, diff, commit, push — and nothing else. A pre-push
	// gate would insert a fetch/rev-list BEFORE the push; its absence proves the push is
	// attempted directly after the commit.
	gotVerbs := gitArgVerbs(r)
	wantVerbs := []string{"diff", "diff", "commit", "push"}
	if len(gotVerbs) != len(wantVerbs) {
		t.Fatalf("git verbs = %v, want exactly %v (no fetch/rev-list/upstream probe before the push)", gotVerbs, wantVerbs)
	}
	for i, want := range wantVerbs {
		if gotVerbs[i] != want {
			t.Errorf("git verb[%d] = %q, want %q (full %v)", i, gotVerbs[i], want, gotVerbs)
		}
	}
}

// TestRun_PushArmed_PushAttemptedDirectlyAfterCommit proves the push is attempted with
// NO intervening behind/diverged precheck: the commit is IMMEDIATELY followed by the
// push in the recorded git order (push index == commit index + 1), so nothing runs
// between them — no remote-sync probe blocks the push attempt.
func TestRun_PushArmed_PushAttemptedDirectlyAfterCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: push directly after commit"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitIdx := indexOfGitArgs(r, "commit")
	pushIdx := indexOfGitArgs(r, "push")
	if commitIdx < 0 || pushIdx < 0 {
		t.Fatalf("git calls = %v, want both a commit and a push", gitInvocations(r))
	}
	if pushIdx != commitIdx+1 {
		t.Errorf("commit at %d, push at %d; the push must follow the commit IMMEDIATELY (no intervening behind/diverged precheck)", commitIdx, pushIdx)
	}
}

// TestRun_DashApy_RunsUnattendedEndToEnd proves `mint commit -Apy` runs unattended
// end-to-end on the AI path: -y auto-accepts the gate (presenter-internal, no blocking
// read), then the deferred `git add -A`, the `git commit -F -`, and the `git push` run
// in that order — add < commit < push — with NO interactive AskLine/blocking read and
// no remote-sync precheck. StdinInteractive is left false: the -y gate auto-accept does
// not require an interactive stdin (it is presenter-internal).
func TestRun_DashApy_RunsUnattendedEndToEnd(t *testing.T) {
	t.Parallel()

	const message = "feat: unattended add-all push"
	rec := &presentertest.RecordingPresenter{} // unscripted Prompt → gate Default (ChoiceYes), modelling the -y skip
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                       // git diff HEAD --name-only (preflight tracked, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // git diff HEAD -- . (read-only -A tracked source)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                          // git ls-files --others (no untracked files for the -A source)
		runner.ScriptedCall{}, // git add -A (deferred staging on accept)
		runner.ScriptedCall{}, // git commit -F -
		runner.ScriptedCall{}, // git push
	)
	deps := newCommitDeps(rec, f, scriptedTransport(message), t.TempDir())
	deps.Staging = commit.AddAll
	deps.Push = true
	deps.Yes = true
	// StdinInteractive deliberately left false: the gate's -y auto-accept is presenter-
	// internal and does not require an interactive stdin (mirrors the editor-fallback
	// guard, which is the only consumer of StdinInteractive — and the AI path never
	// reaches it).

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v; `mint commit -Apy` must run unattended end-to-end", err)
	}

	// No interactive blocking read: -y is presenter-internal, so no AskLine free-text read
	// occurs (AskLine is the gate's `r` line-read, never reached under -y).
	if containsKind(rec.Kinds(), presentertest.KindAskLine) {
		t.Errorf("kinds = %v, want NO AskLine; -y must not trigger an interactive blocking read", rec.Kinds())
	}

	// stage -> commit -> push, each exactly once and in order.
	adds := addInvocations(f)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-A" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -A`", adds)
	}
	if got := commitInvocations(f); len(got) != 1 || got[0].Stdin != message {
		t.Fatalf("commit invocations = %v, want exactly one carrying the minted body %q", got, message)
	}
	if pushes := pushInvocations(f); len(pushes) != 1 {
		t.Fatalf("git push invocations = %d (%v), want exactly 1", len(pushes), pushes)
	}

	addIdx := indexOfGitArgs(f, "add")
	commitIdx := indexOfGitArgs(f, "commit")
	pushIdx := indexOfGitArgs(f, "push")
	if addIdx >= commitIdx || commitIdx >= pushIdx {
		t.Errorf("git order add=%d commit=%d push=%d; want add < commit < push", addIdx, commitIdx, pushIdx)
	}

	// No remote-sync precheck blocked the unattended push.
	assertNoRemoteSyncProbe(t, f.Invocations())
}
