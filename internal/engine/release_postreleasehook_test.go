package engine_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
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
// silent no-op: no `sh` invocation occurs, no post_release stage is narrated, and
// the existing happy path proceeds normally to a successful finish.
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
	if _, ok := stageStarted(rec, "post_release"); ok {
		t.Errorf("an absent post_release hook narrated a StageStarted; kinds = %v", rec.Kinds())
	}
	if _, ok := stageSucceeded(rec, "post_release"); ok {
		t.Errorf("an absent post_release hook narrated a StageSucceeded; kinds = %v", rec.Kinds())
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("absent post_release run did not finish; last event = %v", fin.Kind)
	}
}

// hookTimingRunner records how many presenter events had been shown when each hook
// entry (`sh`) ran, then delegates to the wrapped FakeRunner, so a test can place
// the hook's invocation on the presenter timeline.
type hookTimingRunner struct {
	*runner.FakeRunner
	rec       *presentertest.RecordingPresenter
	shownAtSh []int
}

func (r *hookTimingRunner) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	if name == "sh" {
		r.shownAtSh = append(r.shownAtSh, len(r.rec.Events))
	}
	return r.FakeRunner.RunInDir(ctx, dir, env, name, args...)
}

// runPostRelease drives a real patch release with the given config and `sh`
// outcomes, returning the recorder and how many events had been shown when each
// hook entry ran.
func runPostRelease(t *testing.T, config string, sh ...runner.ScriptedCall) (*presentertest.RecordingPresenter, []int) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	f.SeedSequence("sh", sh...)
	rec := &presentertest.RecordingPresenter{}
	timed := &hookTimingRunner{FakeRunner: f, rec: rec}
	deps := newDeps(rec, f)
	deps.Runner = timed

	if err := engine.Release(t.Context(), deps, patchOptions()); err != nil {
		t.Fatalf("Release returned error %v, want nil (post_release is warn-only)", err)
	}
	if len(timed.shownAtSh) == 0 {
		t.Fatalf("the post_release hook never ran; kinds = %v", rec.Kinds())
	}
	return rec, timed.shownAtSh
}

// assertPostReleaseStageStarted checks the blocking post_release StageStarted
// payload, shown after the publish line and before the hook ran, and returns its
// position.
func assertPostReleaseStageStarted(t *testing.T, rec *presentertest.RecordingPresenter, shownAtSh int) int {
	t.Helper()
	start, ok := stageStarted(rec, "post_release")
	if !ok {
		t.Fatalf("no post_release StageStarted; kinds = %v", rec.Kinds())
	}
	want := presenter.StageStart{Name: "post_release", Key: "post_release", Blocking: true, Text: "running post_release hook…"}
	if start != want {
		t.Errorf("post_release StageStarted = %+v, want %+v", start, want)
	}
	startAt := indexOfStage(rec, presentertest.KindStageStarted, "post_release")
	if publishAt := indexOfStage(rec, presentertest.KindStageSucceeded, "publish"); publishAt == -1 || publishAt > startAt {
		t.Errorf("post_release StageStarted at %d, want after the publish line at %d", startAt, publishAt)
	}
	if startAt >= shownAtSh {
		t.Errorf("post_release StageStarted at %d, want before the hook ran (after %d events)", startAt, shownAtSh)
	}
	return startAt
}

// assertLastBeforeRunFinished checks the event at i is the one directly before the
// closing RunFinished.
func assertLastBeforeRunFinished(t *testing.T, rec *presentertest.RecordingPresenter, i int, what string) {
	t.Helper()
	last := len(rec.Events) - 1
	if rec.Events[last].Kind != presentertest.KindRunFinished || i != last-1 {
		t.Errorf("%s at %d, want directly before the closing RunFinished; kinds = %v", what, i, rec.Kinds())
	}
}

// TestRelease_PostReleaseHook_NarratesAsBlockingStage proves a configured
// post_release hook narrates as pre_tag does: a blocking StageStarted (the pretty
// spinner) before the hook runs, then a blocking StageSucceeded carrying the
// engine-measured Elapsed, and the rail its command, once it exits zero, directly
// before RunFinished.
func TestRelease_PostReleaseHook_NarratesAsBlockingStage(t *testing.T) {
	t.Parallel()

	rec, shownAtSh := runPostRelease(t, "[release.hooks]\npost_release = \"notify.sh\"\n", runner.ScriptedCall{})

	assertPostReleaseStageStarted(t, rec, shownAtSh[0])
	done, ok := stageSucceeded(rec, "post_release")
	if !ok {
		t.Fatalf("no post_release StageSucceeded after a zero-exit hook; kinds = %v", rec.Kinds())
	}
	if done.Elapsed < 0 {
		t.Errorf("post_release StageSucceeded.Elapsed = %v, want a non-negative engine-measured duration", done.Elapsed)
	}
	done.Elapsed = 0
	want := presenter.StageSuccess{Name: "post_release", Key: "post_release", Detail: "hook completed", Row: "notify.sh", Sentence: "Ran the post_release hook", Blocking: true}
	if done != want {
		t.Errorf("post_release StageSucceeded = %+v, want %+v", done, want)
	}
	doneAt := indexOfStage(rec, presentertest.KindStageSucceeded, "post_release")
	if doneAt < shownAtSh[0] {
		t.Errorf("post_release StageSucceeded at %d, want after the hook ran (after %d events)", doneAt, shownAtSh[0])
	}
	assertLastBeforeRunFinished(t, rec, doneAt, "post_release StageSucceeded")
}

// TestRelease_PostReleaseHook_FailureWarnsInPlaceOfTheSuccessLine proves a failing
// post_release hook starts its stage but never succeeds it: the first array entry
// exits non-zero, no post_release StageSucceeded fires, the warn-only Warn carrying
// the hook's stderr verbatim directly follows the StageStarted (the pretty Warn
// stops the spinner), and the run still finishes.
func TestRelease_PostReleaseHook_FailureWarnsInPlaceOfTheSuccessLine(t *testing.T) {
	t.Parallel()

	rec, shownAtSh := runPostRelease(t, "[release.hooks]\npost_release = [\"a\", \"b\"]\n",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1, Stderr: "notify boom\n"}, Err: errors.New("exit status 1")},
		runner.ScriptedCall{},
	)

	startAt := assertPostReleaseStageStarted(t, rec, shownAtSh[0])
	if len(shownAtSh) != 1 {
		t.Errorf("sh ran %d times, want 1 (the array stops at the first non-zero entry)", len(shownAtSh))
	}
	if _, ok := stageSucceeded(rec, "post_release"); ok {
		t.Errorf("a failing post_release hook emitted a StageSucceeded; kinds = %v", rec.Kinds())
	}
	warnAt := startAt + 1
	if warnAt >= len(rec.Events) || rec.Events[warnAt].Kind != presentertest.KindWarn {
		t.Fatalf("no Warn directly after the post_release StageStarted at %d; kinds = %v", startAt, rec.Kinds())
	}
	want := presenter.Warning{Label: "post_release", Message: postReleaseWarnMessage, Key: "post_release", Output: "notify boom\n"}
	if got := rec.Events[warnAt].Warn; got != want {
		t.Errorf("Warn = %+v, want %+v", got, want)
	}
	assertLastBeforeRunFinished(t, rec, warnAt, "post_release Warn")
}

// TestRelease_PostReleaseHook_EmptyValueBehavesAsAbsent proves an explicitly empty
// post_release ("" or []) is treated as no hook at all: nothing runs and no
// post_release stage or Warn is narrated.
func TestRelease_PostReleaseHook_EmptyValueBehavesAsAbsent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, value string }{
		{name: "empty string", value: `""`},
		{name: "empty array", value: `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release.hooks]\npost_release = "+tc.value+"\n")
			f := runner.NewFakeRunner()
			seedHappyGit(f, root, "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if shInvoked(f) {
				t.Errorf("an empty post_release ran `sh`; got %v", commandLines(f.Invocations()))
			}
			if _, ok := stageStarted(rec, "post_release"); ok {
				t.Errorf("an empty post_release narrated a StageStarted; kinds = %v", rec.Kinds())
			}
			if _, ok := stageSucceeded(rec, "post_release"); ok {
				t.Errorf("an empty post_release narrated a StageSucceeded; kinds = %v", rec.Kinds())
			}
			if recorded(rec, presentertest.KindWarn) {
				t.Errorf("an empty post_release surfaced a Warn; kinds = %v", rec.Kinds())
			}
		})
	}
}
