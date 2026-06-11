package commit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// errExitOne models a non-zero git exit accompanying a populated Result (matching
// the real runner's contract) — used to script the lock-contention failure the
// git_safe sink must retry past.
var errExitOne = errors.New("exit status 1")

// The lock-resilient git Mutator must satisfy commit's locally-defined Committer
// sink seam: production wires *git.Mutator (git_safe) as the commit sink, so this
// compile-time assertion guards the contract — the bare commit MUST go through the
// lock-resilient wrapper, never the raw runner.
var _ commit.Committer = (*git.Mutator)(nil)

// The in-test proof commit.Run reports through the AS-BUILT presenter seam — Run
// accepts presenter.Presenter, so the shipped recorder is a legal argument with no
// commit-defined presenter interface or fake in sight.
var _ presenter.Presenter = (*presentertest.RecordingPresenter)(nil)

// seedDiffThenCommit returns a FakeRunner scripting the bare-commit thread's two
// git invocations IN ORDER: the L1 `git diff --cached` read returns diff on stdout,
// then the `git commit -F -` mutation returns a clean success. The FakeRunner
// matches on command name only, so a SeedSequence keyed on "git" distinguishes the
// two same-binary calls (a plain Seed could not — both are `git`).
func seedDiffThenCommit(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}}, // git diff --cached
		runner.ScriptedCall{}, // git commit -F -
	)
	return r
}

// scriptedTransport returns a recordingTransport that yields body for every prompt,
// standing in for the AI so no real `claude` is ever spawned.
func scriptedTransport(body string) *recordingTransport {
	return &recordingTransport{body: body}
}

// newCommitDeps assembles the production-shaped Deps for a bare run over a single
// FakeRunner: the recording presenter, the runner backing L1's staged-diff read,
// the lock-resilient git Mutator (git_safe) as the commit sink wrapping the SAME
// runner, and the scripted transport in place of the real ai.Transport. This is the
// end-to-end harness — the REAL Generator + REAL Mutator thread, driven with no real
// git/claude.
func newCommitDeps(rec *presentertest.RecordingPresenter, r *runner.FakeRunner, tr commit.Transport, root string) commit.Deps {
	return commit.Deps{
		Presenter: rec,
		Runner:    r,
		Committer: git.NewMutator(r),
		Transport: tr,
		Root:      root,
	}
}

// gitInvocations returns only the recorded `git` invocations, in order — the spine
// of the staged-only / commit-sink assertions.
func gitInvocations(r *runner.FakeRunner) []runner.Invocation {
	var git []runner.Invocation
	for _, inv := range r.Invocations() {
		if inv.Name == "git" {
			git = append(git, inv)
		}
	}
	return git
}

// TestRun_BareCommit_GeneratesAndCommitsConventionalMessage drives the whole bare
// thread end-to-end (real Generator + real Mutator over one FakeRunner, scripted
// transport): the AI-inferred conventional-commits message is generated from the
// staged diff and the commit is created carrying that body verbatim.
func TestRun_BareCommit_GeneratesAndCommitsConventionalMessage(t *testing.T) {
	t.Parallel()

	const message = "feat: add staged-diff commit thread"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	if got := commitInv.Stdin; got != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", got, message)
	}
}

// TestRun_NarratesThroughRecordingPresenter proves the thread reports through the
// shipped presenter seam end-to-end: the run opens with RunStarted, shows the minted
// message verbatim via ShowMessage, and closes with RunFinished — recorded on the
// RecordingPresenter with no commit-defined presenter or fake in sight.
func TestRun_NarratesThroughRecordingPresenter(t *testing.T) {
	t.Parallel()

	const message = "feat: narrate the commit thread"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindShowMessage,
		presentertest.KindRunFinished,
	}
	got := rec.Kinds()
	if len(got) != len(wantKinds) {
		t.Fatalf("event kinds = %v, want %v", got, wantKinds)
	}
	for i, want := range wantKinds {
		if got[i] != want {
			t.Errorf("event[%d] kind = %v, want %v", i, got[i], want)
		}
	}

	msg, ok := rec.At(1)
	if !ok {
		t.Fatal("no ShowMessage event recorded")
	}
	if msg.ShowMessage.Body != message {
		t.Errorf("shown message body = %q, want the minted body verbatim %q", msg.ShowMessage.Body, message)
	}
}

// TestRun_InferredTypeAppears_NoScopeByDefault proves the AI-inferred type lands in
// the committed message and no "(scope)" is emitted on the default bare path. The
// generator passes the body through verbatim, so the committed bytes are exactly the
// AI body — type present, scope absent.
func TestRun_InferredTypeAppears_NoScopeByDefault(t *testing.T) {
	t.Parallel()

	const message = "fix: handle empty staged diff"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	subject := firstLine(commitInv.Stdin)
	if !strings.HasPrefix(subject, "fix:") {
		t.Errorf("subject = %q, want it to carry the AI-inferred type %q", subject, "fix:")
	}
	if strings.Contains(subject, "(") {
		t.Errorf("subject = %q carries a scope; scope is off by default", subject)
	}
}

// TestRun_CommitCreatedViaGitSafe proves the commit mutation flows through the
// lock-resilient git_safe wrapper, not the raw runner: the wired Committer is the
// *git.Mutator, and a lock-contended commit is RETRIED (the raw runner would
// surface the first failure). Seeding a stale-lock contention on the first commit
// attempt and a success on the second shows the retry — proof the sink is git_safe.
func TestRun_CommitCreatedViaGitSafe(t *testing.T) {
	t.Parallel()

	const message = "chore: wire commit sink"
	rec := &presentertest.RecordingPresenter{}

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // git diff --cached
		runner.ScriptedCall{ // git commit attempt 1: lock contention
			Result: runner.Result{Stderr: "fatal: Unable to create '/nope/.git/index.lock': File exists\nAnother git process seems to be running"},
			Err:    errExitOne,
		},
		runner.ScriptedCall{}, // git commit attempt 2: succeeds after the wrapper retries
	)

	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		// A no-op backoff keeps the retry deterministic and never sleeps.
		Committer: git.NewMutator(r, git.WithBackoff(func(int) {})),
		Transport: scriptedTransport(message),
		Root:      t.TempDir(),
	}

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Two commit attempts prove the lock-resilient retry ran — a raw runner would have
	// surfaced the first lock failure and never retried.
	commits := commitInvocations(r)
	if len(commits) != 2 {
		t.Fatalf("git commit invocations = %d, want 2 (the lock retry proves git_safe)", len(commits))
	}
}

// TestRun_MessageCarriesNoBranding proves the committed message is the plain
// conventional-commit body — commit does NOT prepend release's commit_prefix (🌿)
// or any branding. The body is committed verbatim, so a branded byte would only
// appear if the orchestrator added it.
func TestRun_MessageCarriesNoBranding(t *testing.T) {
	t.Parallel()

	const message = "docs: describe the commit flow"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit body = %q, want exactly the generated message %q (no branding)", commitInv.Stdin, message)
	}
	if strings.Contains(commitInv.Stdin, "🌿") {
		t.Errorf("commit body %q carries the release commit_prefix; commit is unbranded", commitInv.Stdin)
	}
}

// TestRun_BarePathRunsNoGitAdd proves the bare path is STAGED-ONLY: the only git
// invocations are the L1 `git diff --cached` read and the `git commit` mutation —
// NO `git add` (staging is Phase 2). The exact two-call git argv is the assertion.
func TestRun_BarePathRunsNoGitAdd(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport("chore: thread"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 && inv.Args[0] == "add" {
			t.Errorf("bare path ran `git add %v`; the bare path is staged-only", inv.Args)
		}
	}

	gits := gitInvocations(r)
	if len(gits) != 2 {
		t.Fatalf("git invocations = %d (%v), want exactly 2 (diff + commit)", len(gits), gits)
	}
	if gits[0].Args[0] != "diff" {
		t.Errorf("first git call = %v, want the staged-diff read (`git diff …`)", gits[0].Args)
	}
	if gits[1].Args[0] != "commit" {
		t.Errorf("second git call = %v, want the commit mutation (`git commit …`)", gits[1].Args)
	}
}

// TestRun_GeneratedBodyUsedVerbatim proves a multi-line generated body (subject +
// blank line + wrapped body) is committed BYTE-FOR-BYTE — no trimming, re-wrapping,
// or reformatting between generate and the commit sink.
func TestRun_GeneratedBodyUsedVerbatim(t *testing.T) {
	t.Parallel()

	const message = "feat: add commit thread\n\nWire the L3 generate step to the git_safe\ncommit sink so a bare run mints and commits.\n"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit body = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
	// The body is piped via stdin (-F -), so the mutation argv must select stdin.
	if !containsArg(commitInv.Args, "-F") || !containsArg(commitInv.Args, "-") {
		t.Errorf("commit argv = %v, want `commit -F -` (the body piped via stdin)", commitInv.Args)
	}
}

// TestRun_GenerateFailure_AbortsWithoutCommitting proves a failed generation
// surfaces a StageFailed and aborts BEFORE the commit sink — no `git commit` runs, so
// a broken AI can never produce an unminted/empty commit on the bare path.
func TestRun_GenerateFailure_AbortsWithoutCommitting(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// Only the staged-diff read is scripted; the commit must never be reached.
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		Committer: git.NewMutator(r),
		Transport: &recordingTransport{err: errExitOne},
		Root:      t.TempDir(),
	}

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil error, want a generate-failure abort")
	}

	for _, inv := range r.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "commit" {
			t.Errorf("a `git commit` ran despite generate failure: %v", inv.Args)
		}
	}
	if !containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("kinds = %v, want a StageFailed", rec.Kinds())
	}
}

// containsKind reports whether kinds contains want.
func containsKind(kinds []presentertest.EventKind, want presentertest.EventKind) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

// findCommitInvocation returns the recorded `git commit` invocation, failing the
// test if none ran.
func findCommitInvocation(t *testing.T, r *runner.FakeRunner) runner.Invocation {
	t.Helper()
	for _, inv := range r.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "commit" {
			return inv
		}
	}
	t.Fatal("no `git commit` invocation recorded; the commit was never created")
	return runner.Invocation{}
}

// commitInvocations returns every recorded `git commit` invocation, in order — the
// count proves the lock-resilient retry behaviour of the git_safe sink.
func commitInvocations(r *runner.FakeRunner) []runner.Invocation {
	var commits []runner.Invocation
	for _, inv := range r.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "commit" {
			commits = append(commits, inv)
		}
	}
	return commits
}

// firstLine returns the first line of s (the conventional-commit subject).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
