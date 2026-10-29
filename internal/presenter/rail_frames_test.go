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

	narrow(t, drawn)

	frametest.Check(t, "testdata/frames", drawn)
}

// narrow draws the cached-notes release 50 columns wide, where there is no region
// and the run prints as it happens: the whole run, and the screen as each of its
// questions waits, the pause and the cached notes asked under the notes still
// running, then the review.
func narrow(t *testing.T, drawn map[string]string) {
	in := answers("y\n", "e\n", "y\n", "y\n")
	r := newRailTerm(t, 50, 40, in)
	in.asked = func(n int) {
		if name, ok := map[int]string{2: "narrow-pause-50", 3: "narrow-cache-50", 4: "narrow-review-50"}[n]; ok {
			drawn[name] = r.frame(t)
		}
	}
	cachedRelease(r, r.clock)
	drawn["narrow-50"] = r.frame(t)
}
