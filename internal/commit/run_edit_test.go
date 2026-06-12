package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// editDeps assembles production-shaped Deps for an interactive AI-path run over an
// editorRunner: the recording presenter (scripted gate answers), the editorRunner as
// the read/interactive seam, the lock-resilient git Mutator (git_safe) as the
// staging+commit sink wrapping the SAME runner, and the scripted transport. The run is
// interactive (StdinInteractive true, no -y) so the `e` gate action is reachable.
func editDeps(rec *presentertest.RecordingPresenter, er *editorRunner, tr commit.Transport, root string) commit.Deps {
	// The run is interactive (StdinInteractive defaults true, no -y) so the `e` gate
	// action is reachable. Staging is left at its StagedOnly zero value.
	return editorDeps(rec, er, editorDepsOptions{Transport: tr, Root: root})
}

// seedEditThenAccept scripts the git thread for an `e`-then-`y` AI-path run on a
// FakeRunner: the empty-index preflight read (non-empty), the L1 staged diff, the
// `git var GIT_EDITOR` resolution (one per `e` press, hence editorResolutions copies),
// then the `git commit -F -` sink. The order matches the engine's call sequence.
func seedEditThenAccept(diff, editor string, editorResolutions int) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	calls := []runner.ScriptedCall{
		{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
		{Result: runner.Result{Stdout: diff}},  // git diff --cached (L1)
	}
	for i := 0; i < editorResolutions; i++ {
		calls = append(calls, runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}) // git var GIT_EDITOR
	}
	calls = append(calls, runner.ScriptedCall{}) // git commit -F -
	f.SeedSequence("git", calls...)
	return f
}

// findEditorResolution fails unless at least one `git var GIT_EDITOR` was recorded —
// proof the `e` path resolves the editor via the consumed 3-1 ResolveEditor, not a
// parallel resolver.
func findEditorResolution(t *testing.T, er *editorRunner) {
	t.Helper()
	for _, inv := range editorGitInvocations(er) {
		if len(inv.Args) >= 2 && inv.Args[0] == "var" && inv.Args[1] == "GIT_EDITOR" {
			return
		}
	}
	t.Fatal("no `git var GIT_EDITOR` recorded; the `e` path must resolve the editor via 3-1 ResolveEditor")
}

// TestRun_GateOffersEditAlongsideYesNo proves the interactive AI-path gate declares the
// e (edit) choice alongside y/n. The recorder captures the gate the engine handed to
// Prompt; the assertion reads the declared choice set via Has (nothing hardcodes y/n/e).
func TestRun_GateOffersEditAlongsideYesNo(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}
	er := &editorRunner{fake: seedDiffThenCommit("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport("feat: gate offers edit"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	idx := indexOfKind(rec.Kinds(), presentertest.KindPrompt)
	if idx < 0 {
		t.Fatalf("no Prompt recorded; kinds = %v", rec.Kinds())
	}
	ev, _ := rec.At(idx)
	gate := ev.Prompt
	for _, want := range []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit} {
		if !gate.Has(want) {
			t.Errorf("gate does not offer %q; want y/n/e all present (keys %v)", want, gate.Keys())
		}
	}
}

// TestRun_EditOpensEditorPreFilledWithCurrentMessage proves pressing e opens the editor
// pre-filled with the CURRENT generated message (not an empty/template buffer). The
// double captures the temp-file contents at launch, before its own save-back.
func TestRun_EditOpensEditorPreFilledWithCurrentMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: the generated message\n\nwith a body line\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{"feat: edited\n"},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(er.preFills) != 1 {
		t.Fatalf("editor launch count = %d, want exactly 1 (one `e` press)", len(er.preFills))
	}
	if er.preFills[0] != generated {
		t.Errorf("editor pre-fill = %q, want the current generated message %q (not empty)", er.preFills[0], generated)
	}
}

// TestRun_EditResolvesEditorVia31ResolveEditor proves the e path resolves the editor
// through the consumed 3-1 ResolveEditor — a `git var GIT_EDITOR` call is recorded, not
// a parallel resolver.
func TestRun_EditResolvesEditorVia31ResolveEditor(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{"feat: edited\n"},
	}
	deps := editDeps(rec, er, scriptedTransport("feat: resolve via 3-1"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	findEditorResolution(t, er)
}

// TestRun_EditHandoffBracketedBySuspendResume proves the editor hand-off on the e path
// is bracketed by the presenter's SuspendSpinner / ResumeSpinner (OpenEditor brackets
// internally; reusing it satisfies the requirement).
func TestRun_EditHandoffBracketedBySuspendResume(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{"feat: edited\n"},
	}
	deps := editDeps(rec, er, scriptedTransport("feat: bracketed"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	kinds := rec.Kinds()
	suspendIdx := indexOfKind(kinds, presentertest.KindSuspendSpinner)
	resumeIdx := indexOfKind(kinds, presentertest.KindResumeSpinner)
	if suspendIdx < 0 || resumeIdx < 0 {
		t.Fatalf("kinds = %v, want both a SuspendSpinner and a ResumeSpinner bracketing the editor hand-off", kinds)
	}
	if suspendIdx >= resumeIdx {
		t.Errorf("SuspendSpinner at %d, ResumeSpinner at %d; suspend must precede resume", suspendIdx, resumeIdx)
	}
}

// TestRun_EditNonEmptySaveLoopsBack_NotSaveAsAccept proves a non-empty save LOOPS BACK
// to the gate rather than committing immediately: the engine re-calls ShowMessage (with
// the refreshed body) then Prompt, and the commit carries the EDITED body — committed
// only after the subsequent y, not by the save itself. The recorded presenter ordering
// is ShowMessage, Prompt, [edit], ShowMessage, Prompt; the commit body is the edit.
func TestRun_EditNonEmptySaveLoopsBack_NotSaveAsAccept(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	const edited = "feat: hand-edited\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{edited},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Loop-back re-render ordering: ShowMessage, Prompt (first gate), then after the
	// edit ShowMessage, Prompt (re-rendered gate), then the commit on accept.
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindStageStarted,
		presentertest.KindStageSucceeded,
		presentertest.KindShowMessage,
		presentertest.KindPrompt,
		presentertest.KindSuspendSpinner,
		presentertest.KindResumeSpinner,
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

	// The first ShowMessage carries the generated body; the second carries the edit.
	first, _ := rec.At(3)
	if first.ShowMessage.Body != generated {
		t.Errorf("first ShowMessage body = %q, want the generated message %q", first.ShowMessage.Body, generated)
	}
	second, _ := rec.At(7)
	if second.ShowMessage.Body != edited {
		t.Errorf("re-rendered ShowMessage body = %q, want the edited message %q", second.ShowMessage.Body, edited)
	}

	// Not save-as-accept: exactly one commit, and it carries the EDITED body (the save
	// did not stage/commit; only the subsequent y did). No `git add` on the default mode.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 {
		t.Fatalf("git commit invocations = %d (%v), want exactly 1 (the save is not an accept)", len(commits), commits)
	}
	if commits[0].Stdin != edited {
		t.Errorf("commit body = %q, want the edited body verbatim %q", commits[0].Stdin, edited)
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("the `e` save ran `git add` %v; a non-empty save loops back, it does not stage", adds)
	}
}

// TestRun_EditedMessageUsedVerbatim_NoAIReprocessing proves the edited message is used
// VERBATIM with no AI reprocessing: the transport is called exactly ONCE (the initial
// generate), never again for the edit, and the committed body equals the edited text
// byte-for-byte.
func TestRun_EditedMessageUsedVerbatim_NoAIReprocessing(t *testing.T) {
	t.Parallel()

	const edited = "fix: verbatim edit\n\nbody untouched by AI\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{edited},
	}
	transport := scriptedTransport("feat: original generated")
	deps := editDeps(rec, er, transport, t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 1 {
		t.Errorf("transport called %d times; the edit must NOT reprocess through the AI (want exactly 1 initial generate)", transport.calls())
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != edited {
		t.Fatalf("commit invocations = %v, want exactly one carrying the edited body verbatim %q", commits, edited)
	}
}

// TestRun_MultiLineEditedBodyPreservedThroughLoopBack proves a multi-line edited body
// survives the loop-back intact: the re-rendered ShowMessage and the committed body are
// the multi-line edit byte-for-byte.
func TestRun_MultiLineEditedBodyPreservedThroughLoopBack(t *testing.T) {
	t.Parallel()

	const edited = "feat: multi-line subject\n\nFirst body paragraph that explains the why.\n\nSecond paragraph with more detail.\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{edited},
	}
	deps := editDeps(rec, er, scriptedTransport("feat: generated"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	second, _ := rec.At(7)
	if second.ShowMessage.Body != edited {
		t.Errorf("re-rendered ShowMessage body = %q, want the multi-line edit intact %q", second.ShowMessage.Body, edited)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != edited {
		t.Fatalf("commit invocations = %v, want exactly one carrying the multi-line edit verbatim %q", commits, edited)
	}
}

// TestRun_ReRenderedGateStillOffersYesNoEdit proves the gate re-rendered after an edit
// still offers y/n/e (r is added in 4-4). The recorder captures both Prompt gates; the
// second (post-edit) must declare the same choice set.
func TestRun_ReRenderedGateStillOffersYesNoEdit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{"feat: edited\n"},
	}
	deps := editDeps(rec, er, scriptedTransport("feat: generated"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	var prompts []presenter.Gate
	for i, k := range rec.Kinds() {
		if k == presentertest.KindPrompt {
			ev, _ := rec.At(i)
			prompts = append(prompts, ev.Prompt)
		}
	}
	if len(prompts) != 2 {
		t.Fatalf("Prompt count = %d, want 2 (first gate + re-rendered gate after edit)", len(prompts))
	}
	for _, want := range []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit} {
		if !prompts[1].Has(want) {
			t.Errorf("re-rendered gate does not offer %q; want y/n/e all present (keys %v)", want, prompts[1].Keys())
		}
	}
}

// TestRun_EditingAgainPreFillsWithNowEditedMessage proves a second e pre-fills the
// editor with the NOW-edited message (the result of the first edit), not the original
// generated message. Scripts e, e, y: the second OpenEditor pre-fill must equal the
// first edit's result.
func TestRun_EditingAgainPreFillsWithNowEditedMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	const firstEdit = "feat: first edit\n"
	const secondEdit = "feat: second edit\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 2),
		saves: []string{firstEdit, secondEdit},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(er.preFills) != 2 {
		t.Fatalf("editor launch count = %d, want exactly 2 (two `e` presses)", len(er.preFills))
	}
	if er.preFills[0] != generated {
		t.Errorf("first edit pre-fill = %q, want the generated message %q", er.preFills[0], generated)
	}
	if er.preFills[1] != firstEdit {
		t.Errorf("second edit pre-fill = %q, want the now-edited (first edit) message %q", er.preFills[1], firstEdit)
	}

	// The final commit carries the second edit (the last verbatim refinement).
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != secondEdit {
		t.Fatalf("commit invocations = %v, want exactly one carrying the second edit %q", commits, secondEdit)
	}
}
