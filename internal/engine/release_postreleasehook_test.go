package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// postReleaseWarnMessage is the exact warn message mint emits when a post_release
// hook exits non-zero: the tag is already public, so the failure cannot abort —
// it only warns. The text is fixed by the spec (Hooks Failure behaviour).
const postReleaseWarnMessage = "post_release hook failed; tag is already published"

// firstWarn returns the first recorded Warn event's payload, failing the test if
// none fired — post_release's only observable failure signal is a Warn.
func firstWarn(t *testing.T, rec *presentertest.RecordingPresenter) presentertest.Event {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			return ev
		}
	}
	t.Fatalf("no Warn event recorded; kinds = %v", rec.Kinds())
	return presentertest.Event{}
}

// TestRelease_PostReleaseHook_RunsAfterProviderRelease proves the post_release hook
// runs at Stage 7, AFTER the provider release is created: the `sh -c "notify.sh"`
// invocation appears after the `gh release create` (publish) on the FakeRunner
// timeline, and the run reaches a successful RunFinished.
func TestRelease_PostReleaseHook_RunsAfterProviderRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	f.Seed("sh", runner.Result{}, nil) // post_release hook exits zero
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The hook ran exactly once as `sh -c "notify.sh"`.
	if got := countSh(f); got != 1 {
		t.Errorf("sh ran %d times, want exactly 1 (the post_release hook)", got)
	}
	hookAt := indexOfCmd(f, "sh", "-c", "notify.sh")
	if hookAt == -1 {
		t.Fatalf("post_release hook `sh -c notify.sh` never ran; got %v", commandLines(f.Invocations()))
	}
	// The hook must run AFTER the provider release is created (publish).
	publishAt := indexOfCmd(f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag")
	if publishAt == -1 {
		t.Fatalf("provider release create never ran; got %v", commandLines(f.Invocations()))
	}
	if hookAt < publishAt {
		t.Errorf("post_release hook ran at %d, before the provider release create at %d", hookAt, publishAt)
	}

	// The run reached the successful end-of-run line, and no warn/unwound fired.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish with a zero-exit post_release hook; last event = %v", fin.Kind)
	}
	if recorded(rec, presentertest.KindWarn) {
		t.Errorf("a zero-exit post_release hook surfaced a Warn; it must complete silently")
	}
}

// TestRelease_PostReleaseHook_NonZeroWarnsOnly proves a non-zero post_release hook
// WARNS ONLY and the run still completes: the recorder shows a Warn (label
// "post_release", the fixed message), Release returns NIL (no abort), the run still
// finishes (RunFinished), and NO Unwound fires — the tag is already public.
func TestRelease_PostReleaseHook_NonZeroWarnsOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// The post_release hook exits non-zero AFTER the tag is public.
	f.Seed("sh", runner.Result{ExitCode: 1, Stderr: "notify boom"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned error %v, want nil (warn-only post-PONR)", err)
	}

	warn := firstWarn(t, rec)
	if warn.Warn.Label != "post_release" {
		t.Errorf("Warn.Label = %q, want %q", warn.Warn.Label, "post_release")
	}
	if warn.Warn.Message != postReleaseWarnMessage {
		t.Errorf("Warn.Message = %q, want %q", warn.Warn.Message, postReleaseWarnMessage)
	}
	// No abort signals: no StageFailed, no Unwound; the run still finishes.
	if recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("non-zero post_release surfaced a StageFailed; it must warn only")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("non-zero post_release unwound; it must never unwind a public tag")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish after a warn-only post_release failure; last event = %v", fin.Kind)
	}
}

// TestRelease_PostReleaseHook_NonZeroDoesNotUnwind proves a non-zero post_release
// hook does NOT unwind the published release: after the failing hook there is no
// `git reset --hard`, no `git tag -d`, and no Unwound event — the tag stays public.
func TestRelease_PostReleaseHook_NonZeroDoesNotUnwind(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = \"notify.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("sh", runner.Result{ExitCode: 1}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned error %v, want nil (warn-only post-PONR)", err)
	}

	// No reset and no tag deletion: the published release is never unwound.
	if firstIndexWithPrefix(f, "git reset --hard") != -1 {
		t.Errorf("a `git reset --hard` ran after a failing post_release hook; the public tag must not unwind")
	}
	if firstIndexWithPrefix(f, "git tag -d") != -1 {
		t.Errorf("a `git tag -d` ran after a failing post_release hook; the public tag must not unwind")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("a failing post_release hook emitted an Unwound; the public tag must not unwind")
	}
}

// TestRelease_PostReleaseHook_ArrayStopsAtFirstFailureAndWarns proves an array
// post_release hook stops at the FIRST non-zero entry and warns (not aborts): with
// two entries the first exits non-zero, so only one `sh` runs (the second never
// executes), a single Warn fires, and the run still completes successfully (nil).
func TestRelease_PostReleaseHook_ArrayStopsAtFirstFailureAndWarns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = [\"a\", \"b\"]\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// The first `sh` entry exits non-zero; the second is seeded zero but must never run.
	f.SeedSequence("sh",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")},
		runner.ScriptedCall{Result: runner.Result{}},
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned error %v, want nil (warn-only post-PONR)", err)
	}

	// Exactly one entry ran: the first failed, so the second was never reached.
	if got := countSh(f); got != 1 {
		t.Errorf("sh ran %d times, want exactly 1 (array stops at the first non-zero entry)", got)
	}
	if invokedWith(f, "sh", "-c", "b") {
		t.Errorf("the second array entry ran despite the first failing")
	}
	// A single warn, no abort, and a successful finish.
	if got := countKind(rec, presentertest.KindWarn); got != 1 {
		t.Errorf("Warn count = %d, want exactly 1 for an array post_release first-failure", got)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("array post_release first-failure did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PostReleaseHook_AbsentSkipped proves an absent post_release hook is a
// silent no-op: no `sh` invocation occurs and the existing happy path proceeds
// normally to a successful finish.
func TestRelease_PostReleaseHook_AbsentSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .mint.toml — no [release.hooks].post_release.
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("an absent post_release hook ran `sh`; got %v", commandLines(f.Invocations()))
	}
	if recorded(rec, presentertest.KindWarn) {
		t.Errorf("an absent post_release hook surfaced a Warn")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("absent post_release run did not finish; last event = %v", fin.Kind)
	}
}
