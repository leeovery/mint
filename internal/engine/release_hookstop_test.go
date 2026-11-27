package engine_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// hookEnd is how a hook the run is interrupted during ends.
type hookEnd int

const (
	// exited: the hook had exited when the run was interrupted, so it got no SIGTERM.
	exited hookEnd = iota
	// stopped: the hook was sent SIGTERM and stopped on it.
	stopped
	// forced: the hook was sent SIGTERM, and the second press killed it.
	forced
)

// interruptedHook stands in for a hook the run is interrupted during: each entry
// cancels the run, then returns the cancel, as the hook runner does once the hook has
// stopped. Where the hook was still running, the runner sent it SIGTERM and gave the
// stop notice first; a hook that had exited gets none. Where the second press killed
// it, the runner gave the kill notice too and the error says it forced the hook.
type interruptedHook struct {
	*runner.FakeRunner
	cancel context.CancelFunc
	end    hookEnd
}

func (h *interruptedHook) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	res, _ := h.FakeRunner.RunInDir(ctx, dir, env, name, args...)
	h.cancel()
	if h.end >= stopped {
		runner.NoticeStop(ctx)
	}
	if h.end == forced {
		runner.NoticeKill(ctx)
		return res, fmt.Errorf("running %q: %w: %w", name, runner.ErrForced, ctx.Err())
	}
	return res, fmt.Errorf("running %q: %w", name, ctx.Err())
}

// interruptedRelease runs a release whose hook at point, through hook, is interrupted
// and ends as end says.
func interruptedRelease(t *testing.T, point string, seedGit func(f *runner.FakeRunner, root string), end hookEnd) (*presentertest.RecordingPresenter, *runner.FakeRunner, *interruptedHook) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\n"+point+" = \"press run\"\n")
	f := runner.NewFakeRunner()
	seedGit(f, root)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	hook := &interruptedHook{FakeRunner: runner.NewFakeRunner(), cancel: cancel, end: end}
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

			rec, f, hook := interruptedRelease(t, tt.point, tt.seedGit, stopped)

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
	}, exited)

	if recorded(rec, presentertest.KindStageStopping) {
		t.Errorf("a hook sent no SIGTERM was said to be stopping; kinds = %v", rec.Kinds())
	}
	failedAt := stageEventAt(rec, presentertest.KindStageFailed, "preflight")
	if failedAt == -1 || !rec.Events[failedAt].StageFailed.Interrupted {
		t.Errorf("want the preflight stage failed as interrupted; kinds = %v", rec.Kinds())
	}
}

// stoppingTexts is what each StageStopping on key's stage said, in order.
func stoppingTexts(rec *presentertest.RecordingPresenter, key string) []string {
	var texts []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageStopping && ev.StageStopping.Key == key {
			texts = append(texts, ev.StageStopping.Text)
		}
	}
	return texts
}

// A hook the second press forced says so on its stage after the wait, before the
// stage's failure, which keeps it: a ✗ marked forced before the push, a warning
// saying so for post_release. A hook that stopped on its SIGTERM says neither.
func TestRelease_ForcedHook_SaysTheSecondPressForcedIt(t *testing.T) {
	t.Parallel()

	points := []struct {
		point   string
		seedGit func(f *runner.FakeRunner, root string)
	}{
		{point: "preflight", seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGit(f, root, "main", "v0.0.1")
		}},
		{point: "pre_tag", seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGitThroughGate(f, root, "main", "v0.0.1")
		}},
		{point: "post_release", seedGit: func(f *runner.FakeRunner, root string) {
			seedHappyGit(f, root, "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)
		}},
	}
	ends := []struct {
		name string
		end  hookEnd
		// forced is whether the stage says the hook was forced to stop.
		forced bool
	}{
		{name: "forced", end: forced, forced: true},
		{name: "stopped on its SIGTERM", end: stopped},
	}
	for _, pt := range points {
		for _, e := range ends {
			t.Run(pt.point+"/"+e.name, func(t *testing.T) {
				t.Parallel()

				rec, _, _ := interruptedRelease(t, pt.point, pt.seedGit, e.end)

				want := []string{"waiting for the " + pt.point + " hook to stop"}
				if e.forced {
					want = append(want, "forcing the "+pt.point+" hook to stop")
				}
				if got := stoppingTexts(rec, pt.point); !slices.Equal(got, want) {
					t.Errorf("StageStopping texts = %q, want %q", got, want)
				}
				if pt.point == "post_release" {
					assertPostReleaseWarned(t, rec, e.forced)
					return
				}
				assertFailedForced(t, rec, pt.point, "press run", e.forced)
			})
		}
	}
}

// assertFailedForced fails unless point's stage failed as interrupted on entry, after
// every StageStopping on it, marked Forced as forced says.
func assertFailedForced(t *testing.T, rec *presentertest.RecordingPresenter, point, entry string, forced bool) {
	t.Helper()
	failedAt := stageEventAt(rec, presentertest.KindStageFailed, point)
	if failedAt == -1 {
		t.Fatalf("the %s stage never failed; kinds = %v", point, rec.Kinds())
	}
	assertStoppingBefore(t, rec, point, failedAt)
	got := rec.Events[failedAt].StageFailed
	if !got.Interrupted {
		t.Errorf("the %s failure is not marked interrupted: %+v", point, got)
	}
	if got.Forced != forced {
		t.Errorf("the %s failure's Forced = %v, want %v", point, got.Forced, forced)
	}
	if got.HookEntry != entry {
		t.Errorf("the %s failure's HookEntry = %q, want the entry that ran, %q", point, got.HookEntry, entry)
	}
}

// assertPostReleaseWarned fails unless the post_release hook's warning, after every
// StageStopping on its stage, says it was forced to stop where forced, or failed.
func assertPostReleaseWarned(t *testing.T, rec *presentertest.RecordingPresenter, forced bool) {
	t.Helper()
	warnedAt := stageEventAt(rec, presentertest.KindWarn, "post_release")
	if warnedAt == -1 {
		t.Fatalf("the post_release hook never warned; kinds = %v", rec.Kinds())
	}
	assertStoppingBefore(t, rec, "post_release", warnedAt)
	want := "post_release hook failed; tag is already published"
	if forced {
		want = "post_release hook forced to stop; tag is already published"
	}
	if got := rec.Events[warnedAt].Warn.Message; got != want {
		t.Errorf("post_release warning = %q, want %q", got, want)
	}
}

// assertStoppingBefore fails unless every StageStopping on point's stage comes before
// the event at end.
func assertStoppingBefore(t *testing.T, rec *presentertest.RecordingPresenter, point string, end int) {
	t.Helper()
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageStopping && ev.StageStopping.Key == point && i > end {
			t.Errorf("StageStopping %q at %d comes after the %s stage ended at %d", ev.StageStopping.Text, i, point, end)
		}
	}
}
