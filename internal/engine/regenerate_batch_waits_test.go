package engine_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// manualClock is a clock that moves only when a test moves it.
type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// slowEditor is an editor the person spends a while in.
type slowEditor struct {
	clock *manualClock
	spent time.Duration
}

func (e slowEditor) Edit(context.Context, string) (string, error) {
	e.clock.advance(e.spent)
	return "## edited\n", nil
}

// slowRegenerator writes the notes again, which takes mint a while.
type slowRegenerator struct {
	clock *manualClock
	work  time.Duration
}

func (r slowRegenerator) Regenerate(context.Context, string) (string, error) {
	r.clock.advance(r.work)
	return "## regenerated\n", nil
}

// TestRegenerateAll_AVersionsTimeCountsOnlyMintsWork pins that a --all version's
// node, which goes on past its review to write the release, times only mint's work,
// as a release's notes node does: its notes and an r's new notes count, while the
// waits on its questions, the context typed for the r and the editor don't.
func TestRegenerateAll_AVersionsTimeCountsOnlyMintsWork(t *testing.T) {
	t.Parallel()

	const (
		notesWork = 2 * time.Second
		regenWork = 3 * time.Second
	)
	clock := &manualClock{now: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
	answers := []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceRegen, presenter.ChoiceYes}
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(presenter.Gate) (presenter.Choice, error) {
			clock.advance(30 * time.Second)
			c := answers[0]
			answers = answers[1:]
			return c, nil
		},
		AskLineResult: func(string) (string, error) {
			clock.advance(10 * time.Second)
			return "shorter", nil
		},
	}
	deps := batchDeps(rec, runner.NewFakeRunner())
	deps.Clock = clock.Now
	deps.Editor = slowEditor{clock: clock, spent: 20 * time.Second}
	deps.Regenerator = slowRegenerator{clock: clock, work: regenWork}
	pub := newFakePublisher()
	pub.seedExists(batchV2Tag, true, nil)
	req := batchReq(engine.RegenerateSourceFresh, []version.Resolution{{Tag: batchV2Tag, PreviousTag: batchV1Tag}}, false)
	req.ProduceBody = func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error) {
		clock.advance(notesWork)
		return "## fresh\n", nil
	}

	if _, err := engine.RegenerateAll(t.Context(), deps, pub, t.TempDir(), req); err != nil {
		t.Fatalf("RegenerateAll: %v", err)
	}

	var settled *presenter.StageSuccess
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Key == batchV2Tag && !ev.StageSucceeded.Ongoing {
			settled = &ev.StageSucceeded
		}
	}
	if settled == nil {
		t.Fatalf("the version's node never settled; kinds = %v", rec.Kinds())
	}
	if want := notesWork + regenWork; settled.Elapsed != want {
		t.Errorf("the version's node took %v, want %v: its notes and the r's, none of the waits", settled.Elapsed, want)
	}
}

// TestRegenerateAll_InputClosedAtAVersionsReviewFailsItWithItsTime pins that a
// version whose input closes at its review fails with its node's time, as every
// other failure of a version does, the wait on the question left out.
func TestRegenerateAll_InputClosedAtAVersionsReviewFailsItWithItsTime(t *testing.T) {
	t.Parallel()

	const notesWork = 2 * time.Second
	clock := &manualClock{now: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
	rec := &presentertest.RecordingPresenter{PromptResult: func(presenter.Gate) (presenter.Choice, error) {
		clock.advance(30 * time.Second)
		return "", presenter.ErrInputClosed
	}}
	deps := batchDeps(rec, runner.NewFakeRunner())
	deps.Clock = clock.Now
	req := batchReq(engine.RegenerateSourceFresh, []version.Resolution{{Tag: batchV2Tag, PreviousTag: batchV1Tag}}, false)
	req.ProduceBody = func(context.Context, engine.RegenerateSource, version.Resolution, string, engine.NotesSinks) (string, error) {
		clock.advance(notesWork)
		return "## fresh\n", nil
	}

	_, err := engine.RegenerateAll(t.Context(), deps, newFakePublisher(), t.TempDir(), req)

	want := presenter.StageFailure{Name: "review", Key: batchV2Tag, Message: presenter.ErrInputClosed.Error(), Elapsed: notesWork}
	assertClosedOn(t, rec, err, 1, untimed(want))
	if got := onlyStageFailureEvent(t, rec); got != want {
		t.Errorf("StageFailed = %+v, want %+v: the node's time, none of the wait", got, want)
	}
}
