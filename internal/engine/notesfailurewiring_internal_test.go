package engine

// White-box proofs for Task 2-3: WIRING notesFailureOutput(cause) into BOTH notes
// StageFailed surfacing sites so claude's captured output reaches StageFailure.Output.
//
//   - surfaceAndUnwind (forward release) — the post-mutation, pre-PONR notes stage.
//   - surface (regenerate single-version/interactive) — the shorter-chain notes path.
//
// Both build presenter.StageFailure{Name, Message} and must now ALSO set
// Output: notesFailureOutput(cause). The two paths must render IDENTICALLY for the same
// cause (Acceptance Criteria #3). Only the generation-failed CARRIER populates Output;
// the other three causes (timeout / command-missing / diff-too-large) yield "" so the
// ✗ line stands alone. These assert Output POPULATION via the RecordingPresenter, not the
// rendered stream — the pinned presenter test already covers stream placement.

import (
	"errors"
	"fmt"
	"testing"

	"mint/internal/ai"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// recordedSurfaceAndUnwind drives surfaceAndUnwind with a zero MadeState (nothing made →
// Unwind no-ops, never touching the Mutator) and returns the single recorded
// StageFailure so a test can inspect its Name, Message, and Output. It mirrors the
// forward-release notes surfacing path exactly.
func recordedSurfaceAndUnwind(t *testing.T, cause error) presenter.StageFailure {
	t.Helper()
	rec := &presentertest.RecordingPresenter{}
	deps := ReleaseDeps{Presenter: rec}
	_ = surfaceAndUnwind(t.Context(), deps, "notes", StartState{}, MadeState{}, cause)
	return onlyStageFailure(t, rec)
}

// recordedSurface drives the regenerate single-version/interactive surface(p, "notes",
// cause) path and returns the single recorded StageFailure.
func recordedSurface(t *testing.T, cause error) presenter.StageFailure {
	t.Helper()
	rec := &presentertest.RecordingPresenter{}
	_ = surface(rec, "notes", cause)
	return onlyStageFailure(t, rec)
}

// onlyStageFailure extracts the single recorded StageFailed payload, failing the test
// if there is not exactly one.
//
// An intentional byte-identical twin, onlyStageFailureEvent, lives in the external
// engine_test package (release_priortag_test.go). The duplication cannot be collapsed
// into one func: this copy is in white-box package engine and is invisible to the
// external package, exporting a test helper into the production package is forbidden,
// and relocating tests across the package boundary is out of scope. Edit BOTH together
// if the "exactly one StageFailed" contract or the RecordingPresenter event shape changes.
func onlyStageFailure(t *testing.T, rec *presentertest.RecordingPresenter) presenter.StageFailure {
	t.Helper()
	var found *presenter.StageFailure
	for i := range rec.Events {
		if rec.Events[i].Kind == presentertest.KindStageFailed {
			if found != nil {
				t.Fatalf("recorded more than one StageFailed; kinds = %v", rec.Kinds())
			}
			f := rec.Events[i].StageFailed
			found = &f
		}
	}
	if found == nil {
		t.Fatalf("no StageFailed recorded; kinds = %v", rec.Kinds())
	}
	return *found
}

const conciseGenMessage = "AI returned empty/invalid notes after retry"

// TestSurfaceAndUnwind_PopulatesOutputWithCapturedStdout proves the forward-release notes
// failure path feeds claude's captured stdout into StageFailure.Output while the top-line
// Message stays the concise phrase (no nested chain, no "failed", no leading "notes").
func TestSurfaceAndUnwind_PopulatesOutputWithCapturedStdout(t *testing.T) {
	t.Parallel()

	cause := wrapNotesAbort(t, &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1})

	sf := recordedSurfaceAndUnwind(t, cause)

	if sf.Output != "Prompt is too long" {
		t.Errorf("StageFailure.Output = %q, want %q", sf.Output, "Prompt is too long")
	}
	if sf.Message != conciseGenMessage {
		t.Errorf("StageFailure.Message = %q, want the concise phrase %q", sf.Message, conciseGenMessage)
	}
}

// TestSurface_PopulatesOutputIdenticallyToForwardPath proves the regenerate surface path
// renders IDENTICALLY to the forward-release path for the same generation-failed carrier
// (Acceptance Criteria #3) — both Message AND Output match.
func TestSurface_PopulatesOutputIdenticallyToForwardPath(t *testing.T) {
	t.Parallel()

	carrier := &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}

	// Forward release builds the longer abortError chain; regenerate surfaces the shorter
	// "generating notes: %w" chain. Both must collapse to the identical Message/Output.
	forward := recordedSurfaceAndUnwind(t, wrapNotesAbort(t, carrier))
	regen := recordedSurface(t, fmt.Errorf("generating notes: %w", carrier))

	if regen.Output != "Prompt is too long" {
		t.Errorf("surface StageFailure.Output = %q, want %q", regen.Output, "Prompt is too long")
	}
	if regen.Message != conciseGenMessage {
		t.Errorf("surface StageFailure.Message = %q, want %q", regen.Message, conciseGenMessage)
	}
	if regen.Output != forward.Output {
		t.Errorf("surface Output %q != surfaceAndUnwind Output %q (paths must render identically)", regen.Output, forward.Output)
	}
	if regen.Message != forward.Message {
		t.Errorf("surface Message %q != surfaceAndUnwind Message %q (paths must render identically)", regen.Message, forward.Message)
	}
}

// TestNotesSurfacing_NonCarrierCausesYieldEmptyOutput proves the other three causes
// (timeout / command-missing / diff-too-large) render the concise phrase with an EMPTY
// Output — the ✗ line stands alone — identically on both surfacing paths.
func TestNotesSurfacing_NonCarrierCausesYieldEmptyOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		sentinel error
		message  string
	}{
		{"timeout", ai.ErrTimeout, "AI timed out"},
		{"command missing", ai.ErrCommandMissing, "AI tool not installed"},
		{"diff too large", notes.ErrDiffTooLarge, "diff too large"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cause := wrapNotesAbort(t, tc.sentinel)

			fwd := recordedSurfaceAndUnwind(t, cause)
			reg := recordedSurface(t, cause)

			if fwd.Output != "" {
				t.Errorf("surfaceAndUnwind Output = %q, want \"\" for %s", fwd.Output, tc.name)
			}
			if reg.Output != "" {
				t.Errorf("surface Output = %q, want \"\" for %s", reg.Output, tc.name)
			}
			if fwd.Message != tc.message {
				t.Errorf("surfaceAndUnwind Message = %q, want %q", fwd.Message, tc.message)
			}
			if reg.Message != tc.message {
				t.Errorf("surface Message = %q, want %q", reg.Message, tc.message)
			}
		})
	}
}

// TestResetAndAbort_NoOutputForGitFailure proves the third StageFailure builder
// (resetAndAbort, regenerate's changelog record/push path) is UNCHANGED: a git push
// failure surfaces the concise/cause.Error() fallback Message with an EMPTY Output (the
// AI carrier never reaches it; notesFailureOutput would return "" there anyway).
func TestResetAndAbort_NoOutputForGitFailure(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	cause := fmt.Errorf("pushing regenerated changelog: %w", errors.New("git push failed: exit status 128"))

	// committed=false so no reset mutation runs — the Mutator is never touched.
	_ = resetAndAbort(t.Context(), ReleaseDeps{Presenter: rec}, "startsha", false, "push", cause)

	sf := onlyStageFailure(t, rec)
	if sf.Output != "" {
		t.Errorf("resetAndAbort StageFailure.Output = %q, want \"\" (git failure carries no claude output)", sf.Output)
	}
	if sf.Message != cause.Error() {
		t.Errorf("resetAndAbort StageFailure.Message = %q, want the cause.Error() fallback %q", sf.Message, cause.Error())
	}
}
