package commit_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/git"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// failLoudMessage is the EXACT spec fail-loud message the no-message-source guard
// must surface (lowercase, no trailing punctuation) — asserted byte-for-byte, never
// substring-matched.
const failLoudMessage = "no AI message and no interactive editor available"

// stageFailedMessages returns every recorded StageFailed Message, in order — the
// fail-loud surface narration the guard emits via the surface helper.
func stageFailedMessages(rec *presentertest.RecordingPresenter) []string {
	var msgs []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			msgs = append(msgs, ev.StageFailed.Message)
		}
	}
	return msgs
}

// assertFailLoudNoMutation asserts the standard fail-loud contract: a non-nil error,
// the exact spec message surfaced once, and NO editor launch, NO `git add`, NO
// `git commit` — the run never hangs and never commits an empty message.
func assertFailLoudNoMutation(t *testing.T, rec *presentertest.RecordingPresenter, er *editorRunner, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Run returned nil for a no-message-source fallback; want a non-zero fail-loud abort")
	}
	if err.Error() != failLoudMessage {
		t.Errorf("error = %q, want the exact spec message %q", err.Error(), failLoudMessage)
	}
	msgs := stageFailedMessages(rec)
	if len(msgs) != 1 || msgs[0] != failLoudMessage {
		t.Errorf("StageFailed messages = %v, want exactly one %q", msgs, failLoudMessage)
	}
	if len(er.launches) != 0 {
		t.Errorf("RunInteractive launched %d time(s); the guard must fire BEFORE any editor launch", len(er.launches))
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("fail-loud ran `git add` %v; nothing must be staged", adds)
	}
	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("fail-loud created %d commit(s); nothing must be committed", len(commits))
	}
}

// seedPreflightOnly scripts ONLY the non-empty preflight read (StagedOnly): the guard
// must fire before editor resolution, so neither `git var GIT_EDITOR` nor any
// staging/commit call is reached on a fail-loud run.
func seedPreflightOnly() *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --name-only (non-empty index)
	)
	return f
}

// seedAIPreflightOnly scripts the non-empty preflight read AND the L1 staged diff read
// (StagedOnly) for the AI/oversized paths, which read the L1 diff before reaching the
// fallback. The guard then fires before editor resolution.
func seedAIPreflightOnly() *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                         // git diff --cached --name-only (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work\n"}}, // git diff --cached -- . (L1)
	)
	return f
}

// failLoudDeps builds production-shaped Deps over an editorRunner with the given
// Yes/StdinInteractive guard inputs. A launchable editor is left SEEDED so a test can
// prove the guard fires before resolution/launch (the editor is present yet never
// reached). NoAI is parameterised; Transport is supplied when set.
func failLoudDeps(rec *presentertest.RecordingPresenter, er *editorRunner, mode commit.StagingMode, root string, yes, stdinInteractive, noAI bool, tr commit.Transport) commit.Deps {
	return commit.Deps{
		Presenter:        rec,
		Runner:           er,
		Mutator:          git.NewMutator(er, git.WithBackoff(func(int) {})),
		Transport:        tr,
		Root:             root,
		Staging:          mode,
		NoAI:             noAI,
		Yes:              yes,
		StdinInteractive: stdinInteractive,
	}
}

// TestRun_FallbackUnderYes_FailsLoud proves a fallback under -y fails loud with the
// exact spec message across ALL THREE converging triggers (--no-ai, AI-failure,
// oversized): no editor launch, no staging, no commit. A launchable editor is seeded
// to prove the guard fires regardless of editor availability.
func TestRun_FallbackUnderYes_FailsLoud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deps func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps
		seed func() *runner.FakeRunner
		root func(t *testing.T) string
	}{
		{
			name: "NoAI",
			seed: func() *runner.FakeRunner { return seedPreflightOnly() },
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				return failLoudDeps(rec, er, commit.StagedOnly, root, true, true, true, nil)
			},
		},
		{
			name: "AIFailure",
			seed: func() *runner.FakeRunner { return seedAIPreflightOnly() },
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}
				return failLoudDeps(rec, er, commit.StagedOnly, root, true, true, false, tr)
			},
		},
		{
			name: "Oversized",
			seed: func() *runner.FakeRunner { return seedAIPreflightOnly() },
			root: oversizedRoot,
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				tr := scriptedTransport("must never be returned (L2 was skipped)")
				return failLoudDeps(rec, er, commit.StagedOnly, root, true, true, false, tr)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: tt.seed(), saved: "feat: should never be saved\n"}

			err := commit.Run(context.Background(), tt.deps(rec, er, tt.root(t)))
			assertFailLoudNoMutation(t, rec, er, err)
		})
	}
}

// TestRun_FallbackUnderNonTTYStdin_FailsLoud proves a fallback under non-TTY stdin
// (StdinInteractive=false, Yes=false) fails loud across all three triggers — gated on
// the threaded startup-resolved StdinInteractive signal, NOT a separate probe. A
// launchable editor is seeded to prove the guard never reaches OpenEditor.
func TestRun_FallbackUnderNonTTYStdin_FailsLoud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deps func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps
		seed func() *runner.FakeRunner
		root func(t *testing.T) string
	}{
		{
			name: "NoAI",
			seed: func() *runner.FakeRunner { return seedPreflightOnly() },
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				return failLoudDeps(rec, er, commit.StagedOnly, root, false, false, true, nil)
			},
		},
		{
			name: "AIFailure",
			seed: func() *runner.FakeRunner { return seedAIPreflightOnly() },
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				tr := &failTransport{err: fmt.Errorf("generating commit message: %w", ai.ErrGenerationFailed)}
				return failLoudDeps(rec, er, commit.StagedOnly, root, false, false, false, tr)
			},
		},
		{
			name: "Oversized",
			seed: func() *runner.FakeRunner { return seedAIPreflightOnly() },
			root: oversizedRoot,
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				tr := scriptedTransport("must never be returned (L2 was skipped)")
				return failLoudDeps(rec, er, commit.StagedOnly, root, false, false, false, tr)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: tt.seed(), saved: "feat: should never be saved\n"}

			err := commit.Run(context.Background(), tt.deps(rec, er, tt.root(t)))
			assertFailLoudNoMutation(t, rec, er, err)
		})
	}
}

// TestRun_FallbackNonTTYStdin_NeverReachesEditorResolution proves the guard is gated on
// the THREADED StdinInteractive — not the editor's launchability: a fully launchable
// editor is seeded, yet a non-TTY (StdinInteractive=false) run fails loud WITHOUT
// resolving or launching it (no `git var GIT_EDITOR`, no RunInteractive). This pins the
// guard to the same startup-resolved stdin determination the gate uses, with no
// separate /dev/tty probe and no isatty re-implementation.
func TestRun_FallbackNonTTYStdin_NeverReachesEditorResolution(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedPreflightOnly(), saved: "feat: launchable but never reached\n"}

	err := commit.Run(context.Background(), failLoudDeps(rec, er, commit.StagedOnly, t.TempDir(), false, false, true, nil))
	assertFailLoudNoMutation(t, rec, er, err)

	// The guard fired before editor resolution: only the preflight read ran — no
	// `git var GIT_EDITOR` resolution call.
	for _, inv := range editorGitInvocations(er) {
		if len(inv.Args) >= 2 && inv.Args[0] == "var" && inv.Args[1] == "GIT_EDITOR" {
			t.Errorf("a `git var GIT_EDITOR` resolution ran; the guard must fire BEFORE editor resolution")
		}
	}
}

// TestRun_FallbackOnTTY_NoLaunchableEditor_FailsLoud proves a fallback on a TTY
// (StdinInteractive=true, Yes=false) where NO editor in git's chain is launchable fails
// loud with the SAME spec message — there is no message to fall back to. The not-
// launchable signal is consumed from 3-1 (ErrNoEditor via a failed `git var GIT_EDITOR`).
func TestRun_FallbackOnTTY_NoLaunchableEditor_FailsLoud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		seed func() *runner.FakeRunner
		deps func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps
		root func(t *testing.T) string
	}{
		{
			name: "NoAI_ErrNoEditor",
			seed: func() *runner.FakeRunner {
				f := runner.NewFakeRunner()
				f.SeedSequence("git",
					runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                                              // preflight (non-empty)
					runner.ScriptedCall{Result: runner.Result{Stderr: "terminal is dumb", ExitCode: 128}, Err: errExitOne}, // git var GIT_EDITOR fails → ErrNoEditor
				)
				return f
			},
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				return failLoudDeps(rec, er, commit.StagedOnly, root, false, true, true, nil)
			},
		},
		{
			name: "NoAI_CommandNotFoundOnLaunch",
			seed: func() *runner.FakeRunner {
				f := runner.NewFakeRunner()
				f.SeedSequence("git",
					runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},            // preflight (non-empty)
					runner.ScriptedCall{Result: runner.Result{Stdout: "ghost-editor\n"}}, // git var GIT_EDITOR resolves
				)
				return f
			},
			root: func(t *testing.T) string { return t.TempDir() },
			deps: func(rec *presentertest.RecordingPresenter, er *editorRunner, root string) commit.Deps {
				return failLoudDeps(rec, er, commit.StagedOnly, root, false, true, true, nil)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: tt.seed(), saved: "feat: never saved\n"}
			// The CommandNotFound-on-launch case resolves an editor but the binary is
			// missing at launch — simulate the not-found launch failure.
			if tt.name == "NoAI_CommandNotFoundOnLaunch" {
				er.launchErr = wrapNotFound("ghost-editor")
			}

			err := commit.Run(context.Background(), tt.deps(rec, er, tt.root(t)))

			if err == nil {
				t.Fatal("Run returned nil for a no-launchable-editor fallback; want a non-zero fail-loud abort")
			}
			if err.Error() != failLoudMessage {
				t.Errorf("error = %q, want the exact spec message %q", err.Error(), failLoudMessage)
			}
			msgs := stageFailedMessages(rec)
			if len(msgs) != 1 || msgs[0] != failLoudMessage {
				t.Errorf("StageFailed messages = %v, want exactly one %q", msgs, failLoudMessage)
			}
			if adds := editorAddInvocations(er); len(adds) != 0 {
				t.Errorf("fail-loud ran `git add` %v; nothing must be staged", adds)
			}
			if commits := editorCommitInvocations(er); len(commits) != 0 {
				t.Errorf("fail-loud created %d commit(s); nothing must be committed", len(commits))
			}
		})
	}
}

// TestRun_FallbackOnTTY_AbortedEditor_StaysTrueNoOp proves the guard is NARROW: on a
// legitimate interactive run (TTY stdin, no -y), an aborted/quit editor — which
// OpenEditor reports as a no-op (ok=false, nil err), NOT the not-launchable signal — is
// NOT rewritten into the no-message-source fail-loud. The user reached the editor and
// chose to abort, so it stays a true no-op (errEditorNoOp), distinct from "there was
// never a message source".
func TestRun_FallbackOnTTY_AbortedEditor_StaysTrueNoOp(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},      // preflight (non-empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}}, // git var GIT_EDITOR resolves
	)
	// errExitOne is a launched-but-failed run (a quit/abort), which OpenEditor swallows as
	// a no-op signal rather than a launch error — so the run reaches the editor and aborts.
	er := &editorRunner{fake: f, launchErr: errExitOne}

	err := commit.Run(context.Background(), failLoudDeps(rec, er, commit.StagedOnly, t.TempDir(), false, true, true, nil))
	if err == nil {
		t.Fatal("Run returned nil for an aborted editor; want a non-zero no-op abort")
	}
	if err.Error() == failLoudMessage {
		t.Errorf("aborted editor surfaced the no-message-source message; an aborted editor on a TTY is a true no-op, not a no-message-source fail-loud")
	}
	if len(er.launches) != 1 {
		t.Errorf("RunInteractive launched %d time(s); a TTY run with a launchable editor must reach the editor", len(er.launches))
	}
	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("aborted editor created %d commit(s); an abort is a true no-op", len(commits))
	}
}

// TestRun_NoAI_TTY_LaunchableEditor_SaveAsAcceptStillWorks is the regression guard:
// with StdinInteractive=true and Yes=false and a launchable editor seeded, the existing
// save-as-accept happy path still commits the saved body — the new guard does NOT fire
// on a legitimate interactive run.
func TestRun_NoAI_TTY_LaunchableEditor_SaveAsAcceptStillWorks(t *testing.T) {
	t.Parallel()

	const saved = "feat: interactive save still accepted\n"
	rec := &presentertest.RecordingPresenter{}
	er := &editorRunner{fake: seedNoAIDefault("myedit"), saved: saved}

	deps := failLoudDeps(rec, er, commit.StagedOnly, t.TempDir(), false, true, true, nil)
	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error on a legitimate interactive run: %v", err)
	}

	if len(er.launches) != 1 {
		t.Fatalf("RunInteractive launch count = %d, want exactly 1 (the interactive editor still opens)", len(er.launches))
	}
	if got := editorCommitInvocations(er); len(got) != 1 || got[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", got, saved)
	}
}

// oversizedRoot writes a tiny max_diff_lines ceiling into a TempDir so the real
// Generator's size guard fires on the seeded over-limit L1 diff, routing to the
// oversized fallback. Shared by the fail-loud trigger tables.
func oversizedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	body := "max_diff_lines = " + strconv.Itoa(1) + "\n"
	if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
	return root
}
