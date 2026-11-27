//go:build unix

package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// stopSignalling is a RecordingPresenter that closes stopping at its first
// StageStopping, so a test can act once the runner has sent the hook SIGTERM.
type stopSignalling struct {
	*presentertest.RecordingPresenter
	stopping chan struct{}
	once     bool
}

func (p *stopSignalling) StageStopping(s presenter.StageStop) {
	p.RecordingPresenter.StageStopping(s)
	if !p.once {
		p.once = true
		close(p.stopping)
	}
}

// The whole chain, through the real graceful runner and the hooks runner: a pre_tag
// hook that ignores its SIGTERM, killed by the second press, says mint is forcing it
// before its stage fails, and the failure is marked forced, naming the entry.
func TestRelease_ForcedPreTagHook_ThroughTheGracefulRunner(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const entry = `trap "" TERM; : > ready; while :; do sleep 0.05; done`
	writeConfig(t, root, "[release.hooks]\npre_tag = '"+entry+"'\n")
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	rec := &stopSignalling{RecordingPresenter: &presentertest.RecordingPresenter{}, stopping: make(chan struct{})}
	hooksRunner := runner.NewGracefulExecRunner(time.Minute)
	deps := newDeps(rec.RecordingPresenter, f)
	deps.Presenter = rec
	deps.Hooks = hooksRunner
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = engine.Release(ctx, deps, patchOptions())
	}()
	waitForFile(t, filepath.Join(root, "ready"))
	cancel()
	select {
	case <-rec.stopping:
	case <-time.After(30 * time.Second):
		t.Fatal("the runner never said it was waiting for the hook to stop")
	}
	if !hooksRunner.KillStopping() {
		t.Fatal("KillStopping found no hook stopping")
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the release had not returned 30s after the second press")
	}

	want := []string{"waiting for the pre_tag hook to stop", "forcing the pre_tag hook to stop"}
	if got := stoppingTexts(rec.RecordingPresenter, "pre_tag"); !slices.Equal(got, want) {
		t.Errorf("StageStopping texts = %q, want %q", got, want)
	}
	assertFailedForced(t, rec.RecordingPresenter, "pre_tag", entry, true)
}

// waitForFile waits for path to exist.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}
