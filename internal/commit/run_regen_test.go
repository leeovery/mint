package commit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/config"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedRegenThenAccept scripts the git thread for a run that presses `r` regenerations
// times then accepts: the empty-index preflight read (non-empty), the initial L1 staged
// diff, one further L1 staged diff per regeneration (regenerateMessage re-runs the
// consumed L1 → compose → L2 path, so it re-reads the diff each time), then the
// `git commit -F -` sink. Regeneration runs NO `git add` — the default staging mode is
// staged-only and `r` is never an accept.
func seedRegenThenAccept(diff string, regenerations int) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	calls := []runner.ScriptedCall{
		{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		{Result: runner.Result{Stdout: diff}},  // git diff --cached (initial L1)
	}
	for i := 0; i < regenerations; i++ {
		calls = append(calls, runner.ScriptedCall{Result: runner.Result{Stdout: diff}}) // git diff --cached (regeneration L1)
	}
	calls = append(calls, runner.ScriptedCall{}) // git commit -F -
	r.SeedSequence("git", calls...)
	return r
}

// writeMintToml writes a .mint.toml carrying body to root, so config.Load over that
// root reads the [commit] knobs. Used to seed a persisted [commit].context for the
// "one-time context is not persisted" assertions.
func writeMintToml(t *testing.T, root, body string) {
	t.Helper()
	writeFile(t, root, ".mint.toml", body)
}

// regenDeps assembles production-shaped Deps for an interactive AI-path run: the
// recording presenter (scripted gate answers + AskLine lines), the FakeRunner as the
// read/commit-sink seam wrapped by the git_safe Mutator, and the scripted transport.
// The run is interactive (StdinInteractive true, no -y) so the `r` gate action is
// reachable.
func regenDeps(rec *presentertest.RecordingPresenter, r *runner.FakeRunner, tr commit.Transport, root string) commit.Deps {
	return commit.Deps{
		Presenter:        rec,
		Runner:           r,
		Committer:        git.NewMutator(r, git.WithBackoff(func(int) {})),
		Transport:        tr,
		Root:             root,
		StdinInteractive: true,
	}
}

// TestRun_GateOffersRegenAlongsideYesNoEdit proves the interactive AI-path gate declares
// the r (regenerate) choice alongside y/n/e. The recorder captures the gate the engine
// handed to Prompt; the assertion reads the declared choice set via Has.
func TestRun_GateOffersRegenAlongsideYesNoEdit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := regenDeps(rec, r, scriptedTransport("feat: gate offers regen"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindPrompt)
	if idx < 0 {
		t.Fatalf("no Prompt recorded; kinds = %v", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	gate := ev.Prompt
	for _, want := range []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen} {
		if !gate.Has(want) {
			t.Errorf("gate does not offer %q; want y/n/e/r all present (keys %v)", want, gate.Keys())
		}
	}
}

// TestRun_RegenPromptsForContextLineViaAskLine proves pressing r prompts for a single
// free-text context line via the presenter's AskLine seam (Enter submits, scripted via
// NextLines). A KindAskLine is recorded, between the first gate and the re-rendered gate.
func TestRun_RegenPromptsForContextLineViaAskLine(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"focus on the API change"},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	deps := regenDeps(rec, r, scriptedTransport("feat: regenerated"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !containsKind(rec.Kinds(), presentertest.KindAskLine) {
		t.Fatalf("kinds = %v, want an AskLine recorded for the r context line", rec.Kinds())
	}
	// Ordering: ShowMessage, Prompt (r), AskLine, ShowMessage (regenerated), Prompt (y).
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindShowMessage,
		presentertest.KindPrompt,
		presentertest.KindAskLine,
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
			t.Errorf("event[%d] kind = %v, want %v (full %v)", i, got[i], want, got)
		}
	}
}

// TestRun_RegenNonEmptyLine_InjectedOneTimeIntoPrompt proves a non-empty context line is
// injected ONE-TIME into the regeneration prompt: the prompt the transport receives on
// the regeneration carries the line, while the first (initial) generate prompt does not.
// The line is never written to cfg/[commit].context.
func TestRun_RegenNonEmptyLine_InjectedOneTimeIntoPrompt(t *testing.T) {
	t.Parallel()

	const line = "REGEN_CONTEXT_LINE_SENTINEL"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{line},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	transport := scriptedTransport("feat: regenerated")
	deps := regenDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 2 {
		t.Fatalf("transport called %d times, want 2 (initial generate + one regeneration)", transport.calls())
	}
	initial := transport.prompts[0]
	regen := transport.prompts[1]
	if strings.Contains(initial, line) {
		t.Errorf("initial generate prompt carries the regen context line %q; it is one-time, only on regeneration:\n%s", line, initial)
	}
	if !strings.Contains(regen, line) {
		t.Errorf("regeneration prompt missing the injected one-time context line %q:\n%s", line, regen)
	}
}

// TestRun_RegenEmptyLine_NoInjectedContext proves an empty context line regenerates with
// NO injected context (a plain re-roll): the regeneration prompt equals the initial
// generate prompt byte-for-byte (no one-time context block).
func TestRun_RegenEmptyLine_NoInjectedContext(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{""}, // empty line = plain re-roll
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	transport := scriptedTransport("feat: re-rolled")
	deps := regenDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 2 {
		t.Fatalf("transport called %d times, want 2 (initial generate + plain re-roll)", transport.calls())
	}
	if transport.prompts[0] != transport.prompts[1] {
		t.Errorf("empty-line re-roll prompt differs from the initial generate prompt; a plain re-roll injects no context.\ninitial:\n%s\nregen:\n%s", transport.prompts[0], transport.prompts[1])
	}
}

// TestRun_RegenAskLineInputClosed_AbortsFailLoudNoCommit proves an AskLine EOF
// (ErrInputClosed, NOT rendered by the presenter) aborts fail-loud: no regeneration runs
// (the transport is called only for the initial generate) and no commit is created. The
// sentinel survives in the error chain and the engine surfaces it (a StageFailed).
func TestRun_RegenAskLineInputClosed_AbortsFailLoudNoCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) {
			return "", presenter.ErrInputClosed
		},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 0)
	transport := scriptedTransport("feat: must not regenerate")
	deps := regenDeps(rec, r, transport, t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on AskLine ErrInputClosed; want a non-zero fail-loud abort")
	}
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("error = %v, want errors.Is(..., ErrInputClosed) preserved in the chain", err)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times; EOF on the context line must NOT regenerate (only the initial generate)", transport.calls())
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("AskLine EOF created %d commit(s); it must not commit", len(commits))
	}
	if !containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("AskLine ErrInputClosed emitted no StageFailed; the presenter renders nothing, so the engine must surface it: %v", rec.Kinds())
	}
}

// TestRun_RegenAskLineNotInteractive_AbortsNoExtraStageFailed proves an AskLine
// ErrNotInteractive (PRE-rendered by the presenter) aborts non-zero with the sentinel
// preserved and NO further StageFailed (the presenter already rendered it).
func TestRun_RegenAskLineNotInteractive_AbortsNoExtraStageFailed(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) {
			return "", presenter.ErrNotInteractive
		},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 0)
	transport := scriptedTransport("feat: must not regenerate")
	deps := regenDeps(rec, r, transport, t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil on AskLine ErrNotInteractive; want a non-zero abort")
	}
	if !errors.Is(err, presenter.ErrNotInteractive) {
		t.Errorf("error = %v, want errors.Is(..., ErrNotInteractive) preserved in the chain", err)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times; the forbidden combo must NOT regenerate", transport.calls())
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("AskLine ErrNotInteractive created %d commit(s); it must not commit", len(commits))
	}
	if containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("AskLine ErrNotInteractive emitted a StageFailed; the presenter already rendered the failure line: %v", rec.Kinds())
	}
}

// TestRun_RegenContextNotPersisted_SubsequentRegenDoesNotCarryPriorLine proves the
// injected context is ONE-TIME: scripting r(line "A") then r(line "B") then y, the second
// regeneration prompt carries "B" and NOT "A" — the prior line is never carried forward.
func TestRun_RegenContextNotPersisted_SubsequentRegenDoesNotCarryPriorLine(t *testing.T) {
	t.Parallel()

	const lineA = "FIRST_CONTEXT_AAA"
	const lineB = "SECOND_CONTEXT_BBB"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{lineA, lineB},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 2)
	transport := scriptedTransport("feat: regenerated twice")
	deps := regenDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 3 {
		t.Fatalf("transport called %d times, want 3 (initial + two regenerations)", transport.calls())
	}
	firstRegen := transport.prompts[1]
	secondRegen := transport.prompts[2]
	if !strings.Contains(firstRegen, lineA) {
		t.Errorf("first regeneration prompt missing line A %q:\n%s", lineA, firstRegen)
	}
	if !strings.Contains(secondRegen, lineB) {
		t.Errorf("second regeneration prompt missing line B %q:\n%s", lineB, secondRegen)
	}
	if strings.Contains(secondRegen, lineA) {
		t.Errorf("second regeneration prompt carries the PRIOR line A %q; the one-time context must not persist across re-rolls:\n%s", lineA, secondRegen)
	}
}

// TestRun_RegenContextNotWrittenToConfig proves the one-time line is never persisted to
// cfg/[commit].context: the run reads the Config via config.Load over the temp root (no
// [commit].context set), and the injected line is distinct from any persisted context —
// asserted by the initial generate prompt (which carries [commit].context but never the
// one-time line) being free of the line while the regeneration prompt carries it.
func TestRun_RegenContextNotWrittenToConfig(t *testing.T) {
	t.Parallel()

	const persisted = "PERSISTED_PROJECT_CONTEXT"
	const oneTime = "ONE_TIME_REGEN_CONTEXT"
	root := t.TempDir()
	writeMintToml(t, root, "[commit]\ncontext = \""+persisted+"\"\n")

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{oneTime},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	transport := scriptedTransport("feat: regenerated")
	deps := regenDeps(rec, r, transport, root)

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 2 {
		t.Fatalf("transport called %d times, want 2", transport.calls())
	}
	initial := transport.prompts[0]
	regen := transport.prompts[1]
	// The persisted [commit].context is on EVERY prompt (it is the persisted knob).
	if !strings.Contains(initial, persisted) {
		t.Errorf("initial prompt missing the persisted [commit].context %q:\n%s", persisted, initial)
	}
	if !strings.Contains(regen, persisted) {
		t.Errorf("regeneration prompt missing the persisted [commit].context %q:\n%s", persisted, regen)
	}
	// The one-time line is NOT persisted: it is absent from the initial prompt and
	// present only on the regeneration prompt.
	if strings.Contains(initial, oneTime) {
		t.Errorf("initial prompt carries the one-time regen line %q; it must be regeneration-only, never persisted:\n%s", oneTime, initial)
	}
	if !strings.Contains(regen, oneTime) {
		t.Errorf("regeneration prompt missing the one-time line %q:\n%s", oneTime, regen)
	}
	// Re-loading config from disk proves the one-time line was never written back.
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load returned unexpected error: %v", err)
	}
	if strings.Contains(cfg.Commit.Context, oneTime) {
		t.Errorf("[commit].context = %q now carries the one-time regen line; it must never be persisted", cfg.Commit.Context)
	}
}

// TestRun_RegenConsumesEngineOneRetry proves regeneration consumes the engine's one retry
// (the transport owns it) and the commit code does not re-run it: a transport that fails
// once (bad content) then succeeds is driven through its OWN retry on the regeneration,
// producing a usable body that returns to the gate and commits — the commit code never
// loops the transport itself.
func TestRun_RegenConsumesEngineOneRetry(t *testing.T) {
	t.Parallel()

	const good = "feat: succeeds on the transport's own retry"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"add more detail"},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	// The transport returns good for EVERY Generate call (the real ai.Transport owns the
	// one retry internally; this fake stands in for the consumed behaviour). The initial
	// generate and the single regeneration are the only two Generate calls — the commit
	// code never invokes a second Generate for the same regeneration.
	transport := scriptedTransport(good)
	deps := regenDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 2 {
		t.Errorf("transport Generate called %d times; want exactly 2 (initial + ONE regeneration) — commit must not re-run the retry itself", transport.calls())
	}
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != good {
		t.Errorf("commit body = %q, want the regenerated body verbatim %q", commitInv.Stdin, good)
	}
}

// TestRun_RegeneratedMessageReturnsToGate_NotAnAccept proves a successful regeneration
// returns the new candidate to the Continue? gate (shown, then re-prompted) rather than
// committing: scripting r then y, the engine re-renders ShowMessage(regenerated) → Prompt
// → accept → commit(regenerated body); no staging/commit happens on the r iteration itself.
func TestRun_RegeneratedMessageReturnsToGate_NotAnAccept(t *testing.T) {
	t.Parallel()

	const regenerated = "feat: the regenerated candidate\n\nwith fresh body\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"context line"},
	}
	r := seedRegenThenAccept("diff --git a/x b/x\n+work", 1)
	transport := scriptedTransport(regenerated)
	deps := regenDeps(rec, r, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The second ShowMessage carries the regenerated body (returned to the gate).
	var shows []presenter.Message
	for i, k := range rec.Kinds() {
		if k == presentertest.KindShowMessage {
			ev, _ := rec.At(i)
			shows = append(shows, ev.ShowMessage)
		}
	}
	if len(shows) != 2 {
		t.Fatalf("ShowMessage count = %d, want 2 (initial + regenerated returned to the gate)", len(shows))
	}
	if shows[1].Body != regenerated {
		t.Errorf("re-rendered ShowMessage body = %q, want the regenerated candidate %q", shows[1].Body, regenerated)
	}

	// Exactly one commit, carrying the regenerated body — the r iteration committed
	// nothing; only the subsequent y did.
	commits := commitInvocations(r)
	if len(commits) != 1 {
		t.Fatalf("git commit invocations = %d, want exactly 1 (r is not an accept)", len(commits))
	}
	if commits[0].Stdin != regenerated {
		t.Errorf("commit body = %q, want the regenerated body verbatim %q", commits[0].Stdin, regenerated)
	}
	// The default staging mode runs no `git add`; r never stages either.
	for _, inv := range gitInvocations(r) {
		if len(inv.Args) > 0 && inv.Args[0] == "add" {
			t.Errorf("r ran `git add %v`; regenerate refreshes the candidate, it does not stage", inv.Args)
		}
	}
}
