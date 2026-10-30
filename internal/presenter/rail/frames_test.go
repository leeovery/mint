package rail_test

import (
	"fmt"
	"strings"
	"testing"

	"mint/internal/presenter/frametest"
	"mint/internal/presenter/rail"
)

// TestFrames holds each screen, at each width and at each colour level kept, to its
// golden frame in testdata/frames, cell for cell. `go test ./internal/presenter/rail
// -run Frames -update` writes them again.
func TestFrames(t *testing.T) {
	t.Parallel()
	screens := map[string]func() rail.Run{
		"running": running,
		"review":  review,
		"done":    done,
		"failed":  failed,
		"commit":  commit,
		"regen":   regenerated,
		"dry-run": dryRun,
		"init":    initialised,
	}
	drawn := map[string]string{}
	for name, screen := range screens {
		for _, width := range []int{100, 80, 60} {
			drawn[fmt.Sprintf("%s-%d", name, width)] = frame(screen(), width, rail.TrueColour)
		}
	}
	drawn["failed-100-256"] = frame(failed(), 100, rail.Colours256)
	drawn["failed-100-16"] = frame(failed(), 100, rail.Colours16)
	frametest.Check(t, "testdata/frames", drawn)
}

func frame(r rail.Run, width int, level rail.Level) string {
	return strings.Join(rail.NewLayout(width, level).Draw(r), "\n")
}
