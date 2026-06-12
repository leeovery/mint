package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedEditNoEditorThenAccept scripts the AI-path git thread for an `e`-then-`y` run
// where the editor RESOLUTION fails (3-1 ErrNoEditor via a non-zero `git var
// GIT_EDITOR`): the empty-index preflight read (non-empty), the L1 staged diff, the
// FAILED `git var GIT_EDITOR` (the not-launchable signal under `e`), then — because
// `e` degrades gracefully and the subsequent `y` accepts the UNEDITED message — the
// `git commit -F -` sink. No editor is ever launched (resolution failed first), so no
// second resolution is scripted.
func seedEditNoEditorThenAccept(diff string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                                              // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},                                               // git diff --cached (L1)
		runner.ScriptedCall{Result: runner.Result{Stderr: "terminal is dumb", ExitCode: 128}, Err: errExitOne}, // git var GIT_EDITOR fails → ErrNoEditor
		runner.ScriptedCall{}, // git commit -F - (the subsequent y accepts the unedited message)
	)
	return f
}

// seedEditMissingBinaryThenAccept scripts the AI-path git thread for an `e`-then-`y`
// run where the editor RESOLVES but its binary is MISSING at launch (3-1
// runner.ErrCommandNotFound, surfaced by OpenEditor from RunInteractive): preflight
// (non-empty), the L1 staged diff, the `git var GIT_EDITOR` resolution (succeeds),
// then the `git commit -F -` sink (the subsequent y accepts the unedited message).
// The not-found launch itself is scripted on the editorRunner's launchErr.
func seedEditMissingBinaryThenAccept(diff, editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},          // git diff --cached (L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}, // git var GIT_EDITOR resolves
		runner.ScriptedCall{}, // git commit -F - (the subsequent y accepts the unedited message)
	)
	return f
}

// editorWarnings returns every recorded Warn payload, in order — the graceful-degrade
// narration the `e` not-launchable path emits via the consumed Presenter Warn.
func editorWarnings(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	return warns
}

// TestRun_EditNotLaunchable_WarnsAndReRenders proves the `e` graceful-degrade: a
// not-launchable editor (3-1 ErrNoEditor via a failed `git var GIT_EDITOR`) under `e`
// WARNS then RE-RENDERS the gate (Warn, then a SECOND ShowMessage with the UNEDITED
// body, then a SECOND Prompt) — NOT the 3-5 fail-loud. A subsequent `y` accepts the
// unchanged message. The recorded ordering is the contract.
func TestRun_EditNotLaunchable_WarnsAndReRenders(t *testing.T) {
	t.Parallel()

	const generated = "feat: the generated message\n\nwith a body line\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{fake: seedEditNoEditorThenAccept("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// On a RESOLUTION failure (ErrNoEditor), OpenEditor returns BEFORE the
	// SuspendSpinner/ResumeSpinner bracket (the bracket wraps the launch, which is never
	// reached), so no spinner events are recorded — the warn follows the first gate
	// directly, then the re-render.
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindShowMessage, // first render
		presentertest.KindPrompt,      // first gate → e
		presentertest.KindWarn,        // editor could not launch
		presentertest.KindShowMessage, // re-render with the UNEDITED body
		presentertest.KindPrompt,      // re-rendered gate → y
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

	// A warn fired (graceful-degrade), not a StageFailed (fail-loud).
	if warns := editorWarnings(rec); len(warns) != 1 {
		t.Errorf("Warn count = %d, want exactly 1 (the could-not-launch graceful-degrade)", len(warns))
	}
	if msgs := stageFailedMessages(rec); len(msgs) != 0 {
		t.Errorf("StageFailed messages = %v, want none; `e` degrades gracefully, it does not fail loud", msgs)
	}
}

// TestRun_EditNotLaunchable_PreservesUneditedMessageVerbatim proves the unedited
// message survives the `e` not-launchable warn VERBATIM: the re-rendered ShowMessage
// and the body committed on the final `y` are the original generated message
// byte-for-byte (e treated as a no-op).
func TestRun_EditNotLaunchable_PreservesUneditedMessageVerbatim(t *testing.T) {
	t.Parallel()

	const generated = "feat: verbatim subject\n\nbody untouched by the no-op e\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{fake: seedEditNoEditorThenAccept("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The re-rendered ShowMessage carries the UNEDITED generated body. The sequence on a
	// resolution-failure not-launchable signal is RunStarted, ShowMessage, Prompt, Warn,
	// ShowMessage (the re-render), so the re-rendered ShowMessage is event[4].
	second, ok := rec.At(4)
	if !ok || second.Kind != presentertest.KindShowMessage {
		t.Fatalf("event[4] = %+v, want the re-rendered ShowMessage", second)
	}
	if second.ShowMessage.Body != generated {
		t.Errorf("re-rendered ShowMessage body = %q, want the unedited generated message %q", second.ShowMessage.Body, generated)
	}

	// The committed body is the unedited generated message verbatim.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the unedited message verbatim %q", commits, generated)
	}
}

// TestRun_EditNotLaunchable_NoEditorLaunched proves no editor is launched when the
// not-launchable signal fires via RESOLUTION failure (3-1 ErrNoEditor): RunInteractive
// is never called, so nothing was opened — the graceful-degrade short-circuits at the
// resolution stage, exactly as OpenEditor surfaces it.
func TestRun_EditNotLaunchable_NoEditorLaunched(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{fake: seedEditNoEditorThenAccept("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport("feat: generated"), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s); a resolution-failure not-launchable signal must launch NO editor", len(er.launches))
	}
}

// TestRun_EditNotLaunchable_DistinctFromFallbackFailLoud proves the `e`
// not-launchable graceful-degrade is DISTINCT from the 3-5 fallback fail-loud: a
// message already exists at the gate, so `e` warns + re-renders and a subsequent `y`
// commits — the run returns nil and NEVER surfaces errNoMessageSource (the fail-loud
// spec message). Contrast with the fallback path, which DOES fail loud (covered in
// run_failloud_test.go).
func TestRun_EditNotLaunchable_DistinctFromFallbackFailLoud(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{fake: seedEditNoEditorThenAccept("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport("feat: generated"), t.TempDir())

	err := commit.Run(context.Background(), deps)
	if err != nil {
		t.Fatalf("Run returned %v; the `e` not-launchable path degrades gracefully and a final y commits, so it must return nil", err)
	}

	// The fail-loud spec message must NOT have been surfaced.
	for _, msg := range stageFailedMessages(rec) {
		if msg == failLoudMessage {
			t.Errorf("the `e` path surfaced the fail-loud message %q; a message exists, so `e` degrades gracefully", failLoudMessage)
		}
	}
}

// TestRun_EditNotLaunchable_GateRemainsUsable proves the gate stays fully usable after
// the not-launchable warn: the re-rendered gate offers the same y/n/e choices and a
// subsequent y commits the unchanged message. (r arrives in 4-4; assert against the
// choices that exist now.)
func TestRun_EditNotLaunchable_GateRemainsUsable(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{fake: seedEditNoEditorThenAccept("diff --git a/x b/x\n+work")}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

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
		t.Fatalf("Prompt count = %d, want 2 (first gate + re-rendered gate after the warn)", len(prompts))
	}
	for _, want := range []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit} {
		if !prompts[1].Has(want) {
			t.Errorf("re-rendered gate does not offer %q; want y/n/e all present after the warn (keys %v)", want, prompts[1].Keys())
		}
	}

	// A subsequent y commits the unchanged message.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the unchanged message %q", commits, generated)
	}
}

// TestRun_EditMissingBinary_WarnsAndReRenders proves the SAME graceful-degrade fires
// for the OTHER not-launchable signal: a resolved editor whose binary is MISSING at
// launch (3-1 runner.ErrCommandNotFound, surfaced by OpenEditor). The editor resolves
// and is launched once (the not-found is detected AT launch), then a warn + re-render
// follows and a subsequent y commits the unchanged message — NOT fail-loud.
func TestRun_EditMissingBinary_WarnsAndReRenders(t *testing.T) {
	t.Parallel()

	const generated = "feat: generated\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}
	er := &editorRunner{
		fake:      seedEditMissingBinaryThenAccept("diff --git a/x b/x\n+work", "ghost-editor"),
		launchErr: wrapNotFound("ghost-editor"),
	}
	deps := editDeps(rec, er, scriptedTransport(generated), t.TempDir())

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The editor resolved and was launched once (the not-found surfaces AT launch).
	if len(er.launches) != 1 {
		t.Errorf("RunInteractive launched %d time(s); a missing binary is detected at launch (want exactly 1)", len(er.launches))
	}
	// Graceful-degrade: a warn fired, no fail-loud.
	if warns := editorWarnings(rec); len(warns) != 1 {
		t.Errorf("Warn count = %d, want exactly 1 (the could-not-launch graceful-degrade)", len(warns))
	}
	if msgs := stageFailedMessages(rec); len(msgs) != 0 {
		t.Errorf("StageFailed messages = %v, want none; a missing binary under `e` degrades gracefully", msgs)
	}
	// The subsequent y commits the unchanged message verbatim.
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Fatalf("commit invocations = %v, want exactly one carrying the unchanged message %q", commits, generated)
	}
}
