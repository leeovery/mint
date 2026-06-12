package commit_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// sequencedTransport is a Transport whose Generate returns a SCRIPTED (body, err)
// pair per call, popping the next step on each invocation: it lets a test drive a
// SUCCESSFUL initial generate followed by a FAILING regeneration (the r-failure
// path), which neither the always-succeed recordingTransport nor the always-fail
// failTransport can express. The recorded call count proves commit consumes the
// transport's own one retry (the real transport owns it) and never re-runs it.
type sequencedTransport struct {
	steps   []transportStep
	calls   int
	prompts []string
}

type transportStep struct {
	body string
	err  error
}

// Generate records the prompt and returns the next scripted step. Calling beyond the
// scripted steps is a test-authoring error (the transport must be scripted for every
// expected Generate), so it returns a clear error rather than panicking silently.
func (s *sequencedTransport) Generate(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if s.calls >= len(s.steps) {
		s.calls++
		return "", fmt.Errorf("sequencedTransport: unexpected Generate call #%d (only %d scripted)", s.calls, len(s.steps))
	}
	step := s.steps[s.calls]
	s.calls++
	return step.body, step.err
}

// regenFailDeps assembles production-shaped Deps for an interactive AI-path run whose
// regeneration FAILS and must route to the $EDITOR fallback: the recording presenter
// (scripted gate answers + AskLine lines), the editorRunner as the read/interactive
// seam (git reads delegated to its embedded FakeRunner; RunInteractive simulates the
// editor save), the lock-resilient git Mutator (git_safe) over the SAME editorRunner,
// and the sequenced transport. The run is interactive (StdinInteractive true, no -y) so
// the `r` gate action is reachable AND the editor fallback's no-message-source guard
// does not fire.
func regenFailDeps(rec *presentertest.RecordingPresenter, er *editorRunner, tr commit.Transport, mode commit.StagingMode, root string) commit.Deps {
	return commit.Deps{
		Presenter:        rec,
		Runner:           er,
		Committer:        git.NewMutator(er, git.WithBackoff(func(int) {})),
		Transport:        tr,
		Root:             root,
		Staging:          mode,
		StdinInteractive: true,
	}
}

// seedRegenFailFallback scripts the git thread for an r-regeneration-failure that
// falls back to the editor under the DEFAULT (staged-only) mode: the empty-index
// preflight read (non-empty), the initial L1 staged diff, the regeneration L1 staged
// diff (regenerateMessage re-reads the diff before the transport fails), the
// `git var GIT_EDITOR` resolution for the fallback, then the `git commit -F -` on a
// non-empty save. No `git add` runs under the default mode.
func seedRegenFailFallback(editor string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff --cached (initial L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff --cached (regeneration L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}},                 // git var GIT_EDITOR (fallback)
		runner.ScriptedCall{}, // git commit -F -
	)
	return f
}

// regenSucceedThenFail scripts a transport that succeeds on the initial generate and
// then FAILS the regeneration with the given AI-transport sentinel (after the
// transport's own one retry, which is internal to the real transport).
func regenSucceedThenFail(initial string, failErr error) *sequencedTransport {
	return &sequencedTransport{steps: []transportStep{
		{body: initial},
		{err: failErr},
	}}
}

// TestRun_RegenFailure_RoutesToEditorFallback proves an r regeneration that fails
// after the engine's one retry routes to the 3-3 editor fallback (the SAME path a
// first-generation AI failure takes): a RunInteractive launch is recorded and the
// saved body is committed, rather than the interim surface-abort with the old
// "regenerate" StageFailed.
func TestRun_RegenFailure_RoutesToEditorFallback(t *testing.T) {
	t.Parallel()

	const saved = "feat: human message after regeneration failed\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{"steer the re-roll"},
	}
	er := &editorRunner{fake: seedRegenFailFallback("myedit"), saved: saved}
	tr := regenSucceedThenFail("feat: initial generated", fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed))

	if err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v; an r failure must fall back to the editor, not abort", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the r failure routes to the editor)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
	// The old interim behaviour surfaced a "regenerate" StageFailed; the fallback path
	// must NOT abort that way.
	if containsKind(rec.Kinds(), presentertest.KindStageFailed) {
		t.Errorf("r failure emitted a StageFailed; it must route to the editor fallback, not abort: %v", rec.Kinds())
	}
}

// TestRun_RegenFailure_ReusesThe33EntryPoint proves the r-failure reuses the 3-3 entry
// point (no parallel failure handler): the fallback path observably matches 3-3 — the
// editor is resolved via `git var GIT_EDITOR`, then launched via RunInteractive — exactly
// as the first-generation AI failure does.
func TestRun_RegenFailure_ReusesThe33EntryPoint(t *testing.T) {
	t.Parallel()

	const saved = "feat: reuses the shared fallback\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{""},
	}
	er := &editorRunner{fake: seedRegenFailFallback("myedit"), saved: saved}
	tr := regenSucceedThenFail("feat: initial generated", fmt.Errorf("generating commit message: %w", ai.ErrTimeout))

	if err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The editor is resolved via `git var GIT_EDITOR` (3-1) before any launch — the
	// hallmark of the shared runEditorFallback path.
	var resolvedEditor bool
	for _, inv := range editorGitInvocations(er) {
		if len(inv.Args) >= 2 && inv.Args[0] == "var" && inv.Args[1] == "GIT_EDITOR" {
			resolvedEditor = true
		}
	}
	if !resolvedEditor {
		t.Errorf("git invocations = %v, want a `git var GIT_EDITOR` resolution (the shared 3-3 fallback's 3-1 step)", editorGitInvocations(er))
	}
	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the shared fallback launches the resolved editor)", len(er.launches))
	}
	if er.launches[0].Name != "myedit" {
		t.Errorf("launched editor = %q, want the resolved %q", er.launches[0].Name, "myedit")
	}
}

// TestRun_RegenFailure_EditorBufferIsEmptyTemplate proves the fallback editor opens
// EMPTY/template on the r-failure path — NO synthetic stub and NO re-show of the pre-r
// message. The double captures the temp-file contents at launch (before its save-back).
func TestRun_RegenFailure_EditorBufferIsEmptyTemplate(t *testing.T) {
	t.Parallel()

	const preR = "feat: the message shown BEFORE the user pressed r"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{"give me a better one"},
	}
	er := &editorRunner{fake: seedRegenFailFallback("myedit"), saved: "feat: human message\n"}
	var opened string
	er.onLaunch = func(path string) {
		b, _ := os.ReadFile(path)
		opened = string(b)
	}
	tr := regenSucceedThenFail(preR, fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed))

	if err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if opened != "" {
		t.Errorf("editor opened with buffer %q; the r-failure buffer must be empty (no synthetic stub, no re-show of the pre-r message)", opened)
	}
}

// TestRun_RegenFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits proves save-as-accept
// is unchanged on the r-failure path: a non-empty save under -a applies `git add -u`
// then commits the saved body verbatim, in that order.
func TestRun_RegenFailure_NonEmptySaveUnderAll_AddsTrackedThenCommits(t *testing.T) {
	t.Parallel()

	const saved = "feat: staged tracked then committed after regen failure\n"
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{"another angle"},
	}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff HEAD --name-only (preflight, non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff HEAD -- . (initial L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff HEAD -- . (regeneration L1)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},                    // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -u (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	tr := regenSucceedThenFail("feat: initial generated", fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed))

	if err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.All, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-u" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -u`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
	assertAddBeforeCommit(t, er)
}

// TestRun_RegenFailure_EmptySave_TrueNoOp proves an empty/aborted editor on the
// r-failure path is a true no-op: no `git add`, no `git commit`, a non-zero abort.
func TestRun_RegenFailure_EmptySave_TrueNoOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		saved     string
		launchErr error
	}{
		{name: "WhitespaceOnlySave", saved: "  \n\t\n"},
		{name: "AbortedEditor", launchErr: errExitOne},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{
				NextChoices: []presenter.Choice{presenter.ChoiceRegen},
				NextLines:   []string{"re-roll please"},
			}
			f := runner.NewFakeRunner()
			// Only the preflight read, the two L1 diffs, and the editor resolution are
			// scripted; staging/commit must never be reached on an empty/aborted save.
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}},
				runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},
			)
			er := &editorRunner{fake: f, saved: tt.saved, launchErr: tt.launchErr}
			tr := regenSucceedThenFail("feat: initial generated", fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed))

			err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir()))
			if err == nil {
				t.Fatal("Run returned nil for an empty/aborted editor on the r-failure path; want a non-zero no-op abort")
			}
			// The fallback editor MUST have launched — proving the no-op is the editor
			// fallback's empty-save no-op (3-2), not the old interim surface-abort that never
			// reached the editor.
			if len(er.launches) != 1 {
				t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the r failure must reach the editor fallback)", len(er.launches))
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("empty/aborted editor ran `git add` %v; an empty save is a true no-op", adds)
			}
			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("empty/aborted editor created %d commit(s); an empty save is a true no-op", len(commits))
			}
		})
	}
}

// TestRun_RegenFailure_EngineOneRetryConsumed proves the engine's one retry is consumed
// on the r-failure path (the transport owns it) and the commit code does not re-run it:
// the transport is called EXACTLY TWICE (the initial generate + the single failing
// regeneration) — commit routes the typed failure to the editor without re-running the
// transport itself.
func TestRun_RegenFailure_EngineOneRetryConsumed(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceRegen},
		NextLines:   []string{"add more detail"},
	}
	er := &editorRunner{fake: seedRegenFailFallback("myedit"), saved: "feat: human message\n"}
	tr := regenSucceedThenFail("feat: initial generated", fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed))

	if err := commit.Run(context.Background(), regenFailDeps(rec, er, tr, commit.StagedOnly, t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if tr.calls != 2 {
		t.Errorf("transport.Generate called %d times; want exactly 2 (initial generate + ONE failing regeneration) — commit must consume the transport's own retry, not re-run it", tr.calls)
	}
}
