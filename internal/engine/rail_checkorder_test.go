package engine_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/runner"
)

// TestRelease_RailNarrow_TheCheckPrintsAboveTheFailureThatEndsTheRun pins that the
// check's row settles as its call ends, so a terminal too narrow for a region, which
// prints the run as it happens, shows it above the ✗ of a run its pauses end: the
// answers' commit failing, or input closing at the pause.
func TestRelease_RailNarrow_TheCheckPrintsAboveTheFailureThatEndsTheRun(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		answers string
		// notes is git's calls after the notes diff.
		notes []runner.ScriptedCall
	}{
		{
			name:    "the answers' commit failing",
			answers: "y\ne\n",
			notes:   slices.Concat(commitFails.git, []runner.ScriptedCall{ScriptedOut(""), ScriptedOut("")}), // then the unwind
		},
		{name: "input closed at the pause", answers: "y\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := newRailWorldSized(t, 50, 40, "", tc.answers, false)
			w.deps.Transport = railAI(designDocAnswer, nil)
			w.readGates(false)
			w.designNotes(tc.notes...)

			out, err := w.release(t, nil)

			if err == nil {
				t.Fatal("Release returned nil, want the run to fail")
			}
			lines := strings.Split(out, "\n")
			at := func(text string) int {
				return slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, text) })
			}
			if flagged, failed := at("1 flagged"), at("✗"); flagged == -1 || failed == -1 || flagged > failed {
				t.Errorf("the terminal shows\n%s\nwant the check's 1 flagged above the ✗", out)
			}
		})
	}
}
