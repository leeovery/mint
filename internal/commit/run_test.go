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

// The lock-resilient git Mutator must satisfy commit's locally-defined Mutator
// sink seam: production wires *git.Mutator (git_safe) as the mutation sink, so this
// compile-time assertion guards the contract — every commit mutation (stage, commit,
// push) MUST go through the lock-resilient wrapper, never the raw runner.
var _ commit.Mutator = (*git.Mutator)(nil)

// The in-test proof commit.Run reports through the AS-BUILT presenter seam — Run
// accepts presenter.Presenter, so the shipped recorder is a legal argument with no
// commit-defined presenter interface or fake in sight.
var _ presenter.Presenter = (*presentertest.RecordingPresenter)(nil)

// seedDiffThenCommit returns a FakeRunner scripting the bare-commit thread's three
// git invocations IN ORDER: the empty-index preflight (`git diff --cached
// --name-only`) reports a non-empty staged index so preflight passes, the L1
// `git diff --cached` read returns diff on stdout, then the `git commit -F -`
// mutation returns a clean success. The FakeRunner matches on command name only, so
// a SeedSequence keyed on "git" distinguishes the same-binary calls (a plain Seed
// could not — all three are `git`).
func seedDiffThenCommit(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},  // git diff --cached
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
		Mutator:   git.NewMutator(r),
		Transport: tr,
		Root:      root,
	}
}

// gitInvocationsOf returns only the `git` invocations in invs, in order. It takes an
// already-extracted invocation slice so ONE helper serves both the raw-runner
// (r.Invocations()) and editorRunner (er.fake.Invocations()) sources.
func gitInvocationsOf(invs []runner.Invocation) []runner.Invocation {
	var gits []runner.Invocation
	for _, inv := range invs {
		if inv.Name == "git" {
			gits = append(gits, inv)
		}
	}
	return gits
}

// gitVerbInvocations returns only the `git <verb> …` invocations in invs (Name=="git"
// AND Args[0]==verb), in order — the shared filter behind the add/commit/push
// per-verb helpers across both the raw-runner and editorRunner sources.
func gitVerbInvocations(invs []runner.Invocation, verb string) []runner.Invocation {
	var matched []runner.Invocation
	for _, inv := range gitInvocationsOf(invs) {
		if len(inv.Args) > 0 && inv.Args[0] == verb {
			matched = append(matched, inv)
		}
	}
	return matched
}

// gitInvocations returns only the recorded `git` invocations, in order — the spine
// of the staged-only / commit-sink assertions.
func gitInvocations(r *runner.FakeRunner) []runner.Invocation {
	return gitInvocationsOf(r.Invocations())
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
		presentertest.KindPrompt,
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
// lock-resilient git_safe wrapper, not the raw runner: the wired Mutator is the
// *git.Mutator, and a lock-contended commit is RETRIED (the raw runner would
// surface the first failure). Seeding a stale-lock contention on the first commit
// attempt and a success on the second shows the retry — proof the sink is git_safe.
func TestRun_CommitCreatedViaGitSafe(t *testing.T) {
	t.Parallel()

	const message = "chore: wire commit sink"
	rec := &presentertest.RecordingPresenter{}

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                       // git diff --cached --name-only (non-empty index)
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
		Mutator:   git.NewMutator(r, git.WithBackoff(func(int) {})),
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
// invocations are the empty-index preflight read, the L1 `git diff --cached` read,
// and the `git commit` mutation — NO `git add` (staging is Phase 2). The exact
// three-call git argv is the assertion.
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
	if len(gits) != 3 {
		t.Fatalf("git invocations = %d (%v), want exactly 3 (preflight + diff + commit)", len(gits), gits)
	}
	if gits[0].Args[0] != "diff" {
		t.Errorf("first git call = %v, want the empty-index preflight read (`git diff …`)", gits[0].Args)
	}
	if gits[1].Args[0] != "diff" {
		t.Errorf("second git call = %v, want the staged-diff read (`git diff …`)", gits[1].Args)
	}
	if gits[2].Args[0] != "commit" {
		t.Errorf("third git call = %v, want the commit mutation (`git commit …`)", gits[2].Args)
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
		Mutator:   git.NewMutator(r),
		Transport: &recordingTransport{err: errExitOne},
		Root:      t.TempDir(),
	}

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil error, want a generate-failure abort")
	}

	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("a `git commit` ran despite generate failure: %v", commits)
	}
	if !containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("kinds = %v, want a StageFailed", rec.Kinds())
	}
}

// TestRun_GateEnterAccepts_CreatesCommit proves a bare-Enter accept (the recorder
// returns the gate's Default — ChoiceYes — when no answer is scripted) renders the
// message then the gate via Prompt and proceeds to create the commit carrying the
// minted body verbatim.
func TestRun_GateEnterAccepts_CreatesCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: accept on enter"
	rec := &presentertest.RecordingPresenter{} // no NextChoices → Prompt returns gate Default (ChoiceYes)
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}

// TestRun_GateYesAccepts_CreatesCommit proves an explicit y answer (scripted via
// NextChoices) accepts the gate and creates the commit.
func TestRun_GateYesAccepts_CreatesCommit(t *testing.T) {
	t.Parallel()

	const message = "feat: accept on y"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}

// TestRun_GateNoAborts_MutatesNothing proves an n abort is a true no-op: no
// `git commit` runs (nothing mutated), and the abort emits NO StageFailed failure
// narration (a deliberate user decline is not a failure).
func TestRun_GateNoAborts_MutatesNothing(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}
	r := runner.NewFakeRunner()
	// Only the staged-diff read is scripted; the commit must never be reached.
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: declined"), t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on gate-no; want a non-zero abort")
	}

	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("gate-no created %d commit(s); abort must mutate nothing", len(commits))
	}
	if containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("gate-no emitted a StageFailed; a clean abort emits no failure narration: %v", rec.Kinds())
	}
}

// TestRun_DashYAutoAccepts_CallsPromptAndCommits proves the -y skip is
// presenter-internal: the engine STILL calls Prompt (a KindPrompt is recorded) and
// the auto-accepted gate proceeds to create the commit. The recorder returns the
// gate Default for an unscripted Prompt, modelling the real presenter's -y echo +
// Default return.
func TestRun_DashYAutoAccepts_CallsPromptAndCommits(t *testing.T) {
	t.Parallel()

	const message = "feat: unattended accept"
	rec := &presentertest.RecordingPresenter{} // unscripted → Default (ChoiceYes), modelling the -y skip
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !containsKind(rec.Kinds(), presentertest.KindPrompt) {
		t.Errorf("kinds = %v, want a Prompt call even under -y (the skip is presenter-internal)", rec.Kinds())
	}
	// A recorded commit proves the auto-accepted gate proceeded to the mutation.
	findCommitInvocation(t, r)
}

// TestRun_NonTTYWithoutDashY_FailsLoudNoCommit proves the forbidden combination
// (Prompt returns ErrNotInteractive) maps to a non-zero abort with NO commit, and
// the underlying sentinel survives in the error chain (errors.Is). The presenter
// has ALREADY rendered the failure line, so the engine adds NO further StageFailed.
func TestRun_NonTTYWithoutDashY_FailsLoudNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(presenter.Gate) (presenter.Choice, error) {
			return "", presenter.ErrNotInteractive
		},
	}
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: unattended"), t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on the non-TTY-without-y forbidden combo; want a non-zero abort")
	}
	if !errors.Is(err, presenter.ErrNotInteractive) {
		t.Errorf("error = %v, want errors.Is(..., ErrNotInteractive) preserved in the chain", err)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("forbidden combo created %d commit(s); it must not commit", len(commits))
	}
	if containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("forbidden combo emitted a StageFailed; the presenter already rendered the failure line: %v", rec.Kinds())
	}
}

// TestRun_InputClosed_SurfacedNoCommit proves EOF mid-input (Prompt returns
// ErrInputClosed, which the presenter does NOT render) is surfaced by the engine: a
// non-zero abort preserving the sentinel, with NO commit. Because the presenter
// rendered nothing, commit narrates the failure itself (a StageFailed).
func TestRun_InputClosed_SurfacedNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		PromptResult: func(presenter.Gate) (presenter.Choice, error) {
			return "", presenter.ErrInputClosed
		},
	}
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/x b/x\n+work"}, nil)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: eof"), t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on ErrInputClosed; want a non-zero abort")
	}
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("error = %v, want errors.Is(..., ErrInputClosed) preserved in the chain", err)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("ErrInputClosed created %d commit(s); it must not commit", len(commits))
	}
	if !containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("ErrInputClosed emitted no StageFailed; the presenter renders nothing, so the engine must surface it: %v", rec.Kinds())
	}
}

// TestRun_MessageThenGateThenCommit_Ordering proves the strict pre-mutation
// ordering: ShowMessage (the minted message) renders BEFORE Prompt (the gate), and
// the gate fires BEFORE any commit mutation. The recorder's Kinds() captures the
// presenter ordering; the FakeRunner shows the commit ran only after the gate.
func TestRun_MessageThenGateThenCommit_Ordering(t *testing.T) {
	t.Parallel()

	const message = "feat: ordered"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport(message), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	msgIdx := indexOfKind(rec.Kinds(), presentertest.KindShowMessage)
	promptIdx := indexOfKind(rec.Kinds(), presentertest.KindPrompt)
	if msgIdx < 0 || promptIdx < 0 {
		t.Fatalf("kinds = %v, want both a ShowMessage and a Prompt", rec.Kinds())
	}
	if msgIdx >= promptIdx {
		t.Errorf("ShowMessage at %d, Prompt at %d; the message must render before the gate (kinds %v)", msgIdx, promptIdx, rec.Kinds())
	}
	// The gate is the only path to a commit, so a recorded commit proves the gate
	// preceded the mutation.
	findCommitInvocation(t, r)
}

// TestRun_GateLiteral_CommitSubjectAndChoices proves Run builds its OWN commit Gate
// literal — Subject "message" (so the -y echo reads "message: accepted (-y)", NOT
// "notes: …"), AcceptEcho "accepted", Default ChoiceYes, and the y/n/e/r choice set in
// that order with the spec's action labels — NOT a reused NotesReviewGate/
// ReuseConfirmGate (whose Subject is "notes"). The e (edit) action was added in task
// 4-1; r (regenerate) in 4-4. The recorder captures the gate the engine handed to
// Prompt.
func TestRun_GateLiteral_CommitSubjectAndChoices(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport("feat: gate literal"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindPrompt)
	if idx < 0 {
		t.Fatalf("no Prompt recorded; kinds = %v", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	gate := ev.Prompt
	if gate.Subject != "message" {
		t.Errorf("gate Subject = %q, want %q (so the -y echo reads \"message: accepted (-y)\", not notes)", gate.Subject, "message")
	}
	if gate.AcceptEcho != "accepted" {
		t.Errorf("gate AcceptEcho = %q, want %q", gate.AcceptEcho, "accepted")
	}
	if gate.Default != presenter.ChoiceYes {
		t.Errorf("gate Default = %q, want ChoiceYes", gate.Default)
	}
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen}
	gotKeys := gate.Keys()
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("gate keys = %v, want %v (4-1 added e; 4-4 added r)", gotKeys, wantKeys)
	}
	for i, want := range wantKeys {
		if gotKeys[i] != want {
			t.Errorf("gate key[%d] = %q, want %q", i, gotKeys[i], want)
		}
	}
}

// nothingToCommitMessage is git's exact clean-tree line the empty-index preflight
// must surface verbatim (spec: Staging Model → Empty-staging handling).
const nothingToCommitMessage = "nothing to commit, working tree clean"

// TestRun_NotAGitRepository_FailsLoudNoAINoCommit proves the not-a-git-repo path is
// caught at preflight (root resolution) BEFORE any AI call or commit: Root is left
// empty so Run resolves it via gitrepo.ResolveRoot, and `git rev-parse
// --show-toplevel` is seeded to fail. The run aborts non-zero, the transport is
// never called, and no `git commit` runs — with no panic.
func TestRun_NotAGitRepository_FailsLoudNoAINoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// `git rev-parse --show-toplevel` fails → gitrepo.ResolveRoot returns ErrNotARepository.
	r.Seed("git", runner.Result{Stderr: "fatal: not a git repository"}, errExitOne)
	transport := scriptedTransport("must never be returned (not a repo)")
	deps := commit.Deps{
		Presenter: rec,
		Runner:    r,
		Mutator:   git.NewMutator(r),
		Transport: transport,
		// Root left empty: Run resolves it via gitrepo.ResolveRoot.
	}

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for a non-git-repo; want a non-zero preflight abort")
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times; not-a-git-repo must short-circuit before any AI call", transport.calls())
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("not-a-git-repo created %d commit(s); it must never commit", len(commits))
	}
}

// TestRun_EmptyStagedIndex_FailsLoudWithGitMessage proves an empty staged index
// fails loud with git's exact clean-tree line. The preflight read (`git diff
// --cached --name-only`) reports an empty index (empty stdout), so the run aborts
// before generate, surfacing the message verbatim through the presenter.
func TestRun_EmptyStagedIndex_FailsLoudWithGitMessage(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// Empty stdout from the preflight read = nothing staged.
	r.Seed("git", runner.Result{Stdout: ""}, nil)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned (empty index)"), t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an empty staged index; want a non-zero fail-loud abort")
	}
	if err.Error() != nothingToCommitMessage {
		t.Errorf("error = %q, want git's exact clean-tree line %q", err.Error(), nothingToCommitMessage)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindStageFailed)
	if idx < 0 {
		t.Fatalf("kinds = %v, want a StageFailed narrating the empty-index abort", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	if ev.StageFailed.Message != nothingToCommitMessage {
		t.Errorf("StageFailed.Message = %q, want git's exact clean-tree line %q", ev.StageFailed.Message, nothingToCommitMessage)
	}
}

// TestRun_EmptyStagedIndex_NoAIInvoked proves the AI is NEVER invoked on an empty
// staged diff: the preflight short-circuits BEFORE generate, so the transport
// records zero calls and no commit runs.
func TestRun_EmptyStagedIndex_NoAIInvoked(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil) // empty index
	transport := scriptedTransport("must never be returned (empty index)")
	deps := newCommitDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an empty staged index; want a non-zero abort")
	}

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an empty staged diff must short-circuit before any AI call", transport.calls())
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("empty index created %d commit(s); it must never commit", len(commits))
	}
}

// TestRun_NonEmptyStagedIndex_ProceedsToGeneration proves a non-empty staged index
// passes preflight and proceeds to generation and the commit: the preflight read
// reports a non-empty index, so the transport IS called and the commit is created.
func TestRun_NonEmptyStagedIndex_ProceedsToGeneration(t *testing.T) {
	t.Parallel()

	const message = "feat: proceed past preflight"
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 1 {
		t.Errorf("transport called %d times; a non-empty staged index must reach generation", transport.calls())
	}
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}

// TestRun_PreflightRunsBeforeGenerate proves the empty-index preflight read runs
// BEFORE the L1 staged-diff read that feeds generation: the first git invocation is
// the preflight read, and only then does the generation diff run. The recorded git
// order is the assertion.
func TestRun_PreflightRunsBeforeGenerate(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	transport := scriptedTransport("feat: ordered preflight")
	deps := newCommitDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := gitInvocations(r)
	if len(gits) < 2 {
		t.Fatalf("git invocations = %v, want at least the preflight read then the generation diff", gits)
	}
	// The preflight read is the empty-index check (`git diff --cached --name-only`);
	// the second is generation's L1 diff. Both are `git diff`, distinguished by the
	// preflight carrying --name-only and running first.
	if !containsArg(gits[0].Args, "--name-only") {
		t.Errorf("first git call = %v, want the empty-index preflight read (`git diff --cached --name-only`)", gits[0].Args)
	}
	if containsArg(gits[1].Args, "--name-only") {
		t.Errorf("second git call = %v, want generation's L1 diff (not the --name-only preflight read)", gits[1].Args)
	}
}

// indexOfKind returns the index of the first occurrence of want in kinds, or -1.
func indexOfKind(kinds []presentertest.EventKind, want presentertest.EventKind) int {
	for i, k := range kinds {
		if k == want {
			return i
		}
	}
	return -1
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

// findCommitInvocation returns the first recorded `git commit` invocation, failing the
// test if none ran.
func findCommitInvocation(t *testing.T, r *runner.FakeRunner) runner.Invocation {
	t.Helper()
	commits := commitInvocations(r)
	if len(commits) == 0 {
		t.Fatal("no `git commit` invocation recorded; the commit was never created")
	}
	return commits[0]
}

// commitInvocations returns every recorded `git commit` invocation, in order — the
// count proves the lock-resilient retry behaviour of the git_safe sink.
func commitInvocations(r *runner.FakeRunner) []runner.Invocation {
	return gitVerbInvocations(r.Invocations(), "commit")
}

// firstLine returns the first line of s (the conventional-commit subject).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
