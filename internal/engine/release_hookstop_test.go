package engine_test

import (
	"context"
	"fmt"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// interruptedHook stands in for a hook the run is interrupted during: each entry
// cancels the run, then returns the cancel, as the hook runner does once the hook has
// stopped. Where the hook was still running, terminated, the runner sent it SIGTERM
// and gave the stop notice first; a hook that had exited gets none.
type interruptedHook struct {
	*runner.FakeRunner
	cancel     context.CancelFunc
	terminated bool
}

func (h *interruptedHook) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	res, _ := h.FakeRunner.RunInDir(ctx, dir, env, name, args...)
	h.cancel()
	if h.terminated {
		runner.NoticeStop(ctx)
	}
	return res, fmt.Errorf("running %q: %w", name, ctx.Err())
}

// interruptedRelease runs a release whose hook at point, through hook, is interrupted.
func interruptedRelease(t *testing.T, point string, seedGit func(f *runner.FakeRunner, root string), terminated bool) (*presentertest.RecordingPresenter, *runner.FakeRunner, *interruptedHook) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\n"+point+" = \"press run\"\n")
	f := runner.NewFakeRunner()
	seedGit(f, root)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	hook := &interruptedHook{FakeRunner: runner.NewFakeRunner(), cancel: cancel, terminated: terminated}
	hook.Seed("sh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	deps := newDeps(rec, f)
	deps.Hooks = hook

	_ = engine.Release(ctx, deps, patchOptions())
	return rec, f, hook
}

// stageEventAt is the index of the first event of kind keyed key, or -1.
func stageEventAt(rec *presentertest.RecordingPresenter, kind presentertest.EventKind, key string) int {
	for i, ev := range rec.Events {
		if ev.Kind != kind {
			continue
		}
		switch {
		case kind == presentertest.KindStageStarted && ev.StageStarted.Key == key,
			kind == presentertest.KindStageStopping && ev.StageStopping.Key == key,
			kind == presentertest.KindStageFailed && ev.StageFailed.Key == key,
			kind == presentertest.KindWarn && ev.Warn.Key == key:
			return i
		}
	}
	return -1
}

// A run interrupted while a hook runs, once the runner has sent the hook SIGTERM, says
// on the hook's stage that it waits for the hook to stop, after the stage started and
// before the hook's failure ends it: a ✗ for a hook before the push, a warning for
// post_release. The hook runs through the hooks seam, never the runner git's reads use.
func TestRelease_InterruptedHook_SaysItWaitsForTheHookToStop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		point   string
		seedGit func(f *runner.FakeRunner, root string)
		// ends is the event the hook's failure ends its stage with.
		ends presentertest.EventKind
	}{
		{point: "preflight", ends: presentertest.KindStageFailed, seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGit(f, root, "main", "v0.0.1")
		}},
		{point: "pre_tag", ends: presentertest.KindStageFailed, seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGitThroughGate(f, root, "main", "v0.0.1")
		}},
		{point: "post_release", ends: presentertest.KindWarn, seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGit(f, root, "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.point, func(t *testing.T) {
			t.Parallel()

			rec, f, hook := interruptedRelease(t, tt.point, tt.seedGit, true)

			if n := len(hook.Invocations()); n != 1 {
				t.Fatalf("the hooks seam ran %d commands, want the one hook entry", n)
			}
			if shInvoked(f) {
				t.Errorf("the hook ran on the runner git's reads use; got %v", commandLines(f.Invocations()))
			}
			if n := countKind(rec, presentertest.KindStageStopping); n != 1 {
				t.Fatalf("StageStopping events = %d, want 1; kinds = %v", n, rec.Kinds())
			}
			stoppingAt := stageEventAt(rec, presentertest.KindStageStopping, tt.point)
			want := presenter.StageStop{Name: tt.point, Key: tt.point, Text: "waiting for the " + tt.point + " hook to stop"}
			if got := rec.Events[stoppingAt].StageStopping; got != want {
				t.Errorf("StageStopping = %+v, want %+v", got, want)
			}
			if startedAt := stageEventAt(rec, presentertest.KindStageStarted, tt.point); startedAt == -1 || startedAt > stoppingAt {
				t.Errorf("the %s stage started at %d, want before its StageStopping at %d", tt.point, startedAt, stoppingAt)
			}
			if endedAt := stageEventAt(rec, tt.ends, tt.point); endedAt < stoppingAt {
				t.Errorf("the %s stage ended (%v) at %d, want after its StageStopping at %d", tt.point, tt.ends, endedAt, stoppingAt)
			}
		})
	}
}

// A hook that ends with the run still going says nothing about stopping.
func TestRelease_HookThatEndsOnItsOwn_SaysNothingAboutStopping(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"scripts/check.sh\"\npre_tag = \"build.sh\"\npost_release = \"notify.sh\"\n")
	f := runner.NewFakeRunner()
	seedPreTagClean(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countSh(f); got != 3 {
		t.Fatalf("hook entries run = %d, want 3", got)
	}
	if recorded(rec, presentertest.KindStageStopping) {
		t.Errorf("a run no one interrupted said it waits for a hook to stop; kinds = %v", rec.Kinds())
	}
}

// A hook the interrupt found already exited, which the runner sent no SIGTERM, has
// nothing to wait for: its stage fails as interrupted and says nothing about stopping.
func TestRelease_InterruptedHookThatHadExited_SaysNothingAboutStopping(t *testing.T) {
	t.Parallel()

	rec, _, _ := interruptedRelease(t, "preflight", func(f *runner.FakeRunner, root string) {
		seedHappyGit(f, root, "main", "v0.0.1")
	}, false)

	if recorded(rec, presentertest.KindStageStopping) {
		t.Errorf("a hook sent no SIGTERM was said to be stopping; kinds = %v", rec.Kinds())
	}
	failedAt := stageEventAt(rec, presentertest.KindStageFailed, "preflight")
	if failedAt == -1 || !rec.Events[failedAt].StageFailed.Interrupted {
		t.Errorf("want the preflight stage failed as interrupted; kinds = %v", rec.Kinds())
	}
}
