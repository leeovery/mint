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

// git's verbatim rejection stderr for a non-fast-forward push — the real text git
// emits, used to prove the pass-through is byte-for-byte verbatim (mint never
// reformats, summarises, or parses it).
const rejectedPushStderr = "To github.com:owner/repo.git\n" +
	" ! [rejected]        main -> main (non-fast-forward)\n" +
	"error: failed to push some refs to 'github.com:owner/repo.git'\n" +
	"hint: Updates were rejected because the tip of your current branch is behind\n" +
	"hint: its remote counterpart. Integrate the remote changes (e.g.\n" +
	"hint: 'git pull ...') before pushing again."

// git's verbatim stderr for a push with no upstream configured — carries git's own
// "set an upstream" hint, which must surface ONLY through the verbatim Output
// pass-through, never as mint-authored Message text.
const noUpstreamPushStderr = "fatal: The current branch main has no upstream branch.\n" +
	"To push the current branch and set the remote as upstream, use\n" +
	"\n" +
	"    git push --set-upstream origin main"

// git's verbatim stderr for a network push failure.
const networkPushStderr = "fatal: unable to access 'https://github.com/owner/repo.git/': " +
	"Could not resolve host: github.com"

// seedDiffThenCommitThenFailedPush scripts the gate-accept thread for an armed -p run
// whose push FAILS: the empty-index preflight read (non-empty), the L1 `git diff
// --cached` read, the `git commit -F -` success, then a `git push` returning the
// supplied git stderr with a non-zero exit (NOT a lock contention, so the git_safe
// wrapper surfaces it unchanged — exactly one push attempt).
func seedDiffThenCommitThenFailedPush(diff, pushStderr string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff --cached
		runner.ScriptedCall{}, // git commit -F - (succeeds)
		runner.ScriptedCall{ // git push FAILS with git's stderr (non-fast-forward / no-upstream / network)
			Result: runner.Result{Stderr: pushStderr},
			Err:    errExitOne,
		},
	)
	return r
}

// destructiveGitVerbs are the unwind operations the never-unwind invariant forbids on
// a push failure: a completed commit (and any pre-existing staging) must be left
// untouched — no reset, revert, restore, unstage, or amend ever runs.
var destructiveGitVerbs = []string{"reset", "revert", "restore", "rm", "checkout", "stash"}

// assertNoDestructiveGit scans EVERY recorded git invocation and fails if any matches a
// destructive unwind verb or `git commit --amend` — proving the failed push left the
// commit forward-only with no destructive cleanup of any kind.
func assertNoDestructiveGit(t *testing.T, invs []runner.Invocation) {
	t.Helper()
	for _, inv := range invs {
		if inv.Name != "git" || len(inv.Args) == 0 {
			continue
		}
		verb := inv.Args[0]
		for _, banned := range destructiveGitVerbs {
			if verb == banned {
				t.Errorf("a destructive `git %s` ran after a push failure (%v); the never-unwind invariant is absolute", verb, inv.Args)
			}
		}
		for _, arg := range inv.Args {
			if arg == "--amend" {
				t.Errorf("a `git commit --amend` ran after a push failure (%v); the commit must stay forward-only", inv.Args)
			}
		}
	}
}

// solePushWarn asserts EXACTLY one Warn fired, returns it, and proves it is the only
// failure-shaped narration — no StageFailed/Unwound accompanies the warn (the warn
// alone narrates a kept-commit push failure; the non-zero exit comes from the returned
// sentinel, not a surfaced stage failure).
func solePushWarn(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Warning {
	t.Helper()
	warns := warnEvents(rec)
	if len(warns) != 1 {
		t.Fatalf("Warn events = %d (%v), want exactly 1 generic push warn", len(warns), warns)
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			t.Errorf("a StageFailed fired on a push failure (%+v); the warn narrates — there is no surfaced stage failure", ev.StageFailed)
		}
		if ev.Kind == presentertest.KindUnwound {
			t.Errorf("an Unwound fired on a push failure; there is NO unwind path at all")
		}
	}
	return warns[0]
}

// TestRun_PushFailure_RejectedKeepsCommitAndWarns proves a rejected (non-fast-forward)
// push KEEPS the commit and emits the single generic warn: the commit invocation is
// present, the push was attempted, and exactly one KindWarn fires with Label "push",
// the generic message, and git's stderr verbatim in Output.
func TestRun_PushFailure_RejectedKeepsCommitAndWarns(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: rejected push"), t.TempDir())
	deps.Push = true

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-zero abort (the push failed)")
	}

	// The commit was created and kept (the push attempt followed it).
	findCommitInvocation(t, r)
	if pushes := pushInvocations(r); len(pushes) != 1 {
		t.Fatalf("git push invocations = %d, want exactly 1 (the failed push attempt)", len(pushes))
	}

	warn := solePushWarn(t, rec)
	if warn.Label != "push" {
		t.Errorf("warn Label = %q, want \"push\"", warn.Label)
	}
	if warn.Message == "" {
		t.Errorf("warn Message is empty; want the generic 'commit is in place; re-run the push' text")
	}
	if warn.Output != rejectedPushStderr {
		t.Errorf("warn Output = %q, want git's stderr VERBATIM %q", warn.Output, rejectedPushStderr)
	}
}

// TestRun_PushFailure_ExitsNonZeroCommitInPlace proves a push failure EXITS NON-ZERO
// (Run returns a non-nil error) while leaving the commit in place — the commit
// invocation is recorded.
func TestRun_PushFailure_ExitsNonZeroCommitInPlace(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: exit non-zero"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-nil error mapping to a non-zero exit")
	}

	// The commit stays in place — exactly one commit, no destructive cleanup.
	if commits := commitInvocations(r); len(commits) != 1 {
		t.Fatalf("git commit invocations = %d, want exactly 1 (the commit stays in place)", len(commits))
	}
}

// TestRun_PushFailure_NoUpstreamSameGenericWarn proves a no-upstream push failure emits
// the SAME generic warn (no per-cause message); the no-upstream hint appears ONLY via
// the verbatim Output pass-through (git's stderr), not mint-authored Message text.
func TestRun_PushFailure_NoUpstreamSameGenericWarn(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", noUpstreamPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: no upstream"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a no-upstream push failure; want a non-zero abort")
	}

	warn := solePushWarn(t, rec)
	if warn.Label != "push" {
		t.Errorf("warn Label = %q, want \"push\" (no per-cause label)", warn.Label)
	}
	// The Message is the generic mint-authored text — it must NOT contain git's own
	// upstream hint; that hint travels only in Output.
	if strings.Contains(strings.ToLower(warn.Message), "upstream") {
		t.Errorf("warn Message %q mentions upstream; the upstream hint must come ONLY from git's verbatim Output, not mint-authored text", warn.Message)
	}
	if warn.Output != noUpstreamPushStderr {
		t.Errorf("warn Output = %q, want git's no-upstream stderr VERBATIM %q", warn.Output, noUpstreamPushStderr)
	}
}

// TestRun_PushFailure_NetworkSameGenericWarn proves a network push failure emits the
// SAME generic warn — the message and label match the rejected/no-upstream cases (no
// cause classification), with git's network stderr passed through verbatim.
func TestRun_PushFailure_NetworkSameGenericWarn(t *testing.T) {
	t.Parallel()

	// Capture the rejected-cause warn message to prove the network-cause warn matches it
	// byte-for-byte (one generic warn for all causes, no per-cause classification).
	rejectedRec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	rejectedR := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	rejectedDeps := newCommitDeps(rejectedRec, rejectedR, scriptedTransport("feat: rejected"), t.TempDir())
	rejectedDeps.Push = true
	_ = commit.Run(context.Background(), rejectedDeps)
	rejectedWarn := solePushWarn(t, rejectedRec)

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", networkPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: network"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a network push failure; want a non-zero abort")
	}

	warn := solePushWarn(t, rec)
	if warn.Label != rejectedWarn.Label || warn.Message != rejectedWarn.Message {
		t.Errorf("network warn (%q/%q) differs from rejected warn (%q/%q); mint must emit ONE generic warn for all causes (no classification)",
			warn.Label, warn.Message, rejectedWarn.Label, rejectedWarn.Message)
	}
	if warn.Output != networkPushStderr {
		t.Errorf("warn Output = %q, want git's network stderr VERBATIM %q", warn.Output, networkPushStderr)
	}
}

// TestRun_PushFailure_StderrVerbatimOutOnly proves git's stderr is passed through
// VERBATIM as Warning.Output (the seam's out-only pass-through) and that the warn is
// the only thing emitted — no StageFailed. The Output is byte-identical to the seeded
// git stderr.
func TestRun_PushFailure_StderrVerbatimOutOnly(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: verbatim stderr"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-zero abort")
	}

	warn := solePushWarn(t, rec)
	if warn.Output != rejectedPushStderr {
		t.Errorf("warn Output = %q, want git's stderr passed through byte-for-byte VERBATIM %q", warn.Output, rejectedPushStderr)
	}
}

// TestRun_PushFailure_NeverUnwinds proves a failed push runs NO destructive git: no
// reset/revert/restore/unstage/amend across ALL recorded invocations — the never-unwind
// invariant is absolute.
func TestRun_PushFailure_NeverUnwinds(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: never unwind"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-zero abort")
	}

	assertNoDestructiveGit(t, r.Invocations())
}

// TestRun_PushFailure_CommitForwardOnly proves the commit remains in place and
// forward-only after a failed push: the ONLY git mutations recorded are the commit and
// the failed push attempt — no rewrite (no second commit, no amend, no destructive
// verb).
func TestRun_PushFailure_CommitForwardOnly(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: forward only"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-zero abort")
	}

	// Exactly one commit and one push attempt; no second commit (no rewrite).
	if commits := commitInvocations(r); len(commits) != 1 {
		t.Errorf("git commit invocations = %d, want exactly 1 (forward-only, no rewrite)", len(commits))
	}
	if pushes := pushInvocations(r); len(pushes) != 1 {
		t.Errorf("git push invocations = %d, want exactly 1 (the single failed attempt)", len(pushes))
	}
	assertNoDestructiveGit(t, r.Invocations())
}

// TestRun_PushFailure_NoUpstreamHintOnlyInOutput proves the no-upstream hint comes from
// git's pass-through, not mint-authored text: the generic Message contains neither
// "upstream" nor "set an upstream", while git's hint is present in Output.
func TestRun_PushFailure_NoUpstreamHintOnlyInOutput(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", noUpstreamPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: hint in output"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a no-upstream push failure; want a non-zero abort")
	}

	warn := solePushWarn(t, rec)
	lower := strings.ToLower(warn.Message)
	if strings.Contains(lower, "upstream") {
		t.Errorf("warn Message %q contains 'upstream'; the hint must be mint-free and surface only via git's Output", warn.Message)
	}
	if strings.Contains(lower, "set an upstream") {
		t.Errorf("warn Message %q contains 'set an upstream'; that is git's illustrative pass-through, never mint-authored", warn.Message)
	}
	// git's own hint IS present in the verbatim Output.
	if !strings.Contains(warn.Output, "set the remote as upstream") {
		t.Errorf("warn Output %q is missing git's upstream hint; the hint must survive verbatim in Output", warn.Output)
	}
}

// TestRun_PushFailure_EditorAcceptSameGenericWarn proves the SAME generic warn fires for
// the EDITOR save-as-accept push failure (--no-ai → non-empty save → -p armed → push
// fails), proving the single shared push step: the commit is kept and the same Label
// "push" warn with git's stderr verbatim is emitted.
func TestRun_PushFailure_EditorAcceptSameGenericWarn(t *testing.T) {
	t.Parallel()

	const saved = "feat: editor save then failed push\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F - (succeeds)
		runner.ScriptedCall{ // git push FAILS with git's rejection stderr
			Result: runner.Result{Stderr: rejectedPushStderr},
			Err:    errExitOne,
		},
	)
	er := &editorRunner{fake: f, saved: saved}
	deps := noAIDeps(rec, er, commit.StagedOnly, t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on an editor-accept failed push; want a non-zero abort")
	}

	// The save-as-accept commit was created and kept.
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	if pushes := editorPushInvocations(er); len(pushes) != 1 {
		t.Fatalf("git push invocations = %d, want exactly 1 (the failed push attempt)", len(pushes))
	}

	warn := solePushWarn(t, rec)
	if warn.Label != "push" {
		t.Errorf("editor-path warn Label = %q, want \"push\" (the same single shared step)", warn.Label)
	}
	if warn.Output != rejectedPushStderr {
		t.Errorf("editor-path warn Output = %q, want git's stderr VERBATIM %q", warn.Output, rejectedPushStderr)
	}
	assertNoDestructiveGit(t, editorGitInvocations(er))
}

// TestRun_PushFailure_RunFinishedStillFiresWithOneWarn locks "the warn does not
// suppress the close-out" on the path where it matters — the FAILED push: the commit
// is the run's success, so RunFinished still fires alongside exactly ONE generic push
// warn (and no StageFailed/Unwound), even though Run returns non-nil for the exit code.
func TestRun_PushFailure_RunFinishedStillFiresWithOneWarn(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", rejectedPushStderr)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: warn does not suppress close-out"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil on a failed push; want a non-zero abort (the push failed)")
	}

	if !hasKind(rec, presentertest.KindRunFinished) {
		t.Errorf("RunFinished did not fire on a failed push; the commit IS the run's success, so the close-out must still run")
	}
	solePushWarn(t, rec)
}

// TestRun_PushSuccess_StillFinishesZeroNoWarn is the 5-2/5-3 happy-path regression: a
// SUCCESSFUL push still returns nil (exit 0), fires RunFinished, and emits NO warn —
// the warn-don't-unwind handling never triggers on success.
func TestRun_PushSuccess_StillFinishesZeroNoWarn(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport("feat: clean push"), t.TempDir())
	deps.Push = true

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error on a successful push: %v", err)
	}

	if warns := warnEvents(rec); len(warns) != 0 {
		t.Errorf("Warn events = %d (%v) on a successful push; want none", len(warns), warns)
	}
	if !hasKind(rec, presentertest.KindRunFinished) {
		t.Errorf("RunFinished did not fire on a successful push; the success close-out must run")
	}
}

// hasKind reports whether any recorded event matches kind.
func hasKind(rec *presentertest.RecordingPresenter, kind presentertest.EventKind) bool {
	for _, ev := range rec.Events {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}
