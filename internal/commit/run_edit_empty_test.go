package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// promptGates returns every gate the engine handed to Prompt, in order — the spine of
// the re-render / "still offers y/n/e" assertions across an `e` empty-save loop-back.
func promptGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for i, k := range rec.Kinds() {
		if k == presentertest.KindPrompt {
			ev, _ := rec.At(i)
			gates = append(gates, ev.Prompt)
		}
	}
	return gates
}

// showMessageBodies returns the Body of every ShowMessage the engine rendered, in
// order — used to assert the message re-rendered after an empty `e` save is the PRIOR
// (unchanged) body, not a refreshed/blank one.
func showMessageBodies(rec *presentertest.RecordingPresenter) []string {
	var bodies []string
	for i, k := range rec.Kinds() {
		if k == presentertest.KindShowMessage {
			ev, _ := rec.At(i)
			bodies = append(bodies, ev.ShowMessage.Body)
		}
	}
	return bodies
}

// TestRun_EditEmptySaveDiscardsEditReRendersWithPriorMessage proves an empty `e` save
// DISCARDS the edit and re-renders the gate with the PRIOR message preserved unchanged:
// the recorder shows ShowMessage(generated) → Prompt → [e, empty save] →
// ShowMessage(generated, SAME body) → Prompt, then the final `y` commits the PRIOR
// (unedited) body verbatim. This is the empty-save counterpart to 4-1's non-empty
// loop-back — distinct from the Phase 3 fallback's empty=abort: a message already
// exists, so the empty save is a discard + re-render, NOT a no-op abort.
func TestRun_EditEmptySaveDiscardsEditReRendersWithPriorMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n\nwith a body line\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{""}, // empty save under `e`
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Re-render ordering: the empty save loops back to a fresh ShowMessage + Prompt.
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
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

	// Both ShowMessage renders carry the PRIOR (generated) body unchanged — the empty
	// save discarded the edit, it did not adopt a blank/refreshed message.
	bodies := showMessageBodies(rec)
	if len(bodies) != 2 {
		t.Fatalf("ShowMessage count = %d, want 2 (first render + re-render after empty save)", len(bodies))
	}
	for i, body := range bodies {
		if body != generated {
			t.Errorf("ShowMessage[%d] body = %q, want the prior generated message preserved %q", i, body, generated)
		}
	}

	// The final `y` commits the PRIOR (unedited) body verbatim.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the prior body verbatim %q", commits, generated)
	}
}

// TestRun_EditWhitespaceOnlySaveTreatedAsEmpty proves a whitespace-only `e` save is
// treated as empty per the 3-2 editor contract (whitespace-only / no content = empty;
// NO #-comment stripping) and preserves the prior message the same way an empty-string
// save does. Table-driven over the two whitespace shapes.
func TestRun_EditWhitespaceOnlySaveTreatedAsEmpty(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"

	tests := []struct {
		name string
		save string
	}{
		{name: "SpacesAndNewline", save: "   \n"},
		{name: "TabsSpacesNewlines", save: "  \n\t\n  "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{
				NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
			}
			er := &editorRunner{
				fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
				saves: []string{tt.save},
			}
			deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

			if err := commit.Run(context.Background(), deps); err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}

			// The re-rendered ShowMessage carries the prior body unchanged.
			bodies := showMessageBodies(rec)
			if len(bodies) != 2 {
				t.Fatalf("ShowMessage count = %d, want 2 (first render + re-render)", len(bodies))
			}
			if bodies[1] != generated {
				t.Errorf("re-rendered ShowMessage body = %q, want the prior message preserved %q", bodies[1], generated)
			}

			// The whitespace-only save discarded the edit; the commit carries the prior body.
			commits := editorCommitInvocations(er)
			if len(commits) != 1 || commits[0].Stdin != generated {
				t.Fatalf("commit invocations = %v, want exactly one carrying the prior body verbatim %q", commits, generated)
			}
		})
	}
}

// TestRun_EditQuitWithNoContentPreservesPriorMessage proves quitting the `e` editor
// with NO content (an aborted editor, ok=false) preserves the prior message: the gate
// re-renders and the run CONTINUES (a final `y` commits the prior body). Distinct from
// the Phase 3 fallback where an aborted editor is a no-op abort — under `e` a message
// already exists, so an aborted editor is a discard + re-render.
func TestRun_EditQuitWithNoContentPreservesPriorMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	// launchErr models a launched-but-failed editor (a quit/abort, e.g. `:cq`): OpenEditor
	// swallows it as ok=false with no error, so the `e` branch discards and loops back.
	er := &editorRunner{
		fake:      seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		launchErr: errExitOne,
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v; an aborted editor under `e` re-renders, it does not abort", err)
	}

	// The gate re-rendered (run continued): a second ShowMessage + Prompt, then a commit.
	bodies := showMessageBodies(rec)
	if len(bodies) != 2 {
		t.Fatalf("ShowMessage count = %d, want 2 (first render + re-render after the aborted editor)", len(bodies))
	}
	if bodies[1] != generated {
		t.Errorf("re-rendered ShowMessage body = %q, want the prior message preserved %q", bodies[1], generated)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the prior body verbatim %q", commits, generated)
	}
}

// TestRun_EditRepeatedThenEmptySavePreservesLastNonEmptyCandidate proves that a
// non-empty edit followed by an empty `e` save preserves the message current BEFORE
// that empty save — the LAST non-empty candidate, not the original generated message.
// Scripts e(save "X") → e(empty save) → y: the committed body is "X" and the second
// `e`'s pre-fill was "X" (the empty save discarded only itself, keeping the prior X).
func TestRun_EditRepeatedThenEmptySavePreservesLastNonEmptyCandidate(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	const firstEdit = "feat: first edit X\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 2),
		saves: []string{firstEdit, ""}, // first edit saves X, second is an empty save
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The second `e` pre-fills with the last non-empty candidate (X), not the original.
	if len(er.preFills) != 2 {
		t.Fatalf("editor launch count = %d, want exactly 2 (two `e` presses)", len(er.preFills))
	}
	if er.preFills[0] != generated {
		t.Errorf("first edit pre-fill = %q, want the generated message %q", er.preFills[0], generated)
	}
	if er.preFills[1] != firstEdit {
		t.Errorf("second edit pre-fill = %q, want the last non-empty candidate %q", er.preFills[1], firstEdit)
	}

	// The empty save discarded only itself: the committed body is the last non-empty edit X.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != firstEdit {
		t.Fatalf("commit invocations = %v, want exactly one carrying the last non-empty candidate %q", commits, firstEdit)
	}
}

// TestRun_EditEmptySaveOnFirstEditPreservesGeneratedMessage proves an empty `e` save on
// the FIRST edit preserves the ORIGINAL generated message: scripts e(empty) → y; the
// committed body equals the generated message (e is never a message source, so an empty
// first edit falls back to the generated candidate it discarded back to).
func TestRun_EditEmptySaveOnFirstEditPreservesGeneratedMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: the original generated message\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{""},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the original generated message %q", commits, generated)
	}
}

// TestRun_EditEmptySaveNeverCommits proves `e` is never a message source: NO commit
// occurs on the empty-save iteration. The only commit happens on the final `y`, and it
// carries the prior body — the empty save itself committed nothing.
func TestRun_EditEmptySaveNeverCommits(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{""},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Exactly one commit, on the final `y` — the empty `e` save committed nothing, and it
	// ran no `git add` either (the default mode stages nothing, and `e` never stages).
	commits := editorCommitInvocations(er)
	if len(commits) != 1 {
		t.Fatalf("git commit invocations = %d (%v), want exactly 1 (e is never a message source)", len(commits), commits)
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("the empty `e` save ran `git add` %v; e never stages or commits", adds)
	}
}

// TestRun_EditEmptySaveIsNotAnAbort proves an empty `e` save is NOT treated as an
// abort: the run CONTINUES (the gate re-renders) and Run does NOT emit a terminal
// abort. A no-op abort (errEditorNoOp / errGateAborted) returns a non-nil error with NO
// further gate render and NO commit; here Run returns nil (the subsequent `y` accepts),
// the gate re-renders a second time still offering y/n/e, and the prior body commits.
func TestRun_EditEmptySaveIsNotAnAbort(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:  seedEditThenAccept("diff --git a/x b/x\n+work", "myedit", 1),
		saves: []string{""},
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	// An abort would surface a non-nil error and stop BEFORE re-rendering/committing; the
	// run continuing to a clean nil is the observable proof the empty save is not an abort.
	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned %v; an empty `e` save is a discard + re-render, not an abort", err)
	}

	// The run continued: a second gate render that still offers y/n/e (r arrives in 4-4).
	gates := promptGates(rec)
	if len(gates) != 2 {
		t.Fatalf("Prompt count = %d, want 2 (first gate + re-rendered gate after the empty save)", len(gates))
	}
	for _, want := range []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit} {
		if !gates[1].Has(want) {
			t.Errorf("re-rendered gate does not offer %q; want y/n/e all present (keys %v)", want, gates[1].Keys())
		}
	}

	// The run reached the commit (an abort never would): exactly one, carrying the prior body.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Errorf("commit invocations = %v, want exactly one carrying the prior body %q (an abort would commit nothing)", commits, generated)
	}
}
