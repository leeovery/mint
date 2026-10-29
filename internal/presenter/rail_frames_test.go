package presenter_test

import (
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// TestRailFrames holds the golden runs, whole runs driven by fixed events through a
// virtual terminal 100 columns wide, colour off, to their frames in testdata/frames,
// cell for cell: each run's whole screen once it ends, and two screens mid-run, the
// notes running with the plan below and the review's question up. `go test
// ./internal/presenter -run Frames -update` writes them again.
func TestRailFrames(t *testing.T) {
	t.Parallel()
	drawn := map[string]string{}

	in := answers("y\n", "e\n", "y\n")
	r := newRailTerm(t, 100, 40, in)
	in.asked = func(n int) {
		if n == 3 {
			drawn["review-100"] = r.frame(t)
		}
	}
	tour(r, r.clock, func() {
		r.ticks.tick(t)
		drawn["running-100"] = r.frame(t)
	})
	drawn["done-100"] = r.frame(t)

	for name, run := range map[string]func(presenter.Presenter, *fakeClock){
		"dry-run-100": dryRun,
		"commit-100":  committed,
		"init-100":    initialised,
		"version-100": versioned,
	} {
		r := newRailTerm(t, 100, 40, answers("y\n"))
		run(r, r.clock)
		drawn[name] = r.frame(t)
	}

	failed := newRailTerm(t, 100, 40, answers())
	failed.WithYes(true)
	failedRelease(failed, failed.clock)
	drawn["failed-100"] = failed.frame(t)

	frametest.Check(t, "testdata/frames", drawn)
}
