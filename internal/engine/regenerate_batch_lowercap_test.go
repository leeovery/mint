package engine_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// versionCalls scripts one version's notes generation as its notes stage sees it:
// the largest diff-carrying call that succeeded, whether a part was halved, and the
// error the generation ends with, nil when the notes are written.
type versionCalls struct {
	largest int
	halved  bool
	err     error
}

func lowerCapBatchVersions() []version.Resolution {
	return []version.Resolution{
		{Tag: "v1.1.0", PreviousTag: "v1.0.0"},
		{Tag: "v1.2.0", PreviousTag: "v1.1.0"},
		{Tag: "v1.3.0", PreviousTag: "v1.2.0"},
	}
}

// lowerCapBatch is a fresh `--all` over v1.1.0, v1.2.0 and v1.3.0 whose versions
// generate as calls scripts them, a version it leaves out writing its notes in one
// unhalved call.
type lowerCapBatch struct {
	rec *presentertest.RecordingPresenter
	pub *fakePublisher
	req engine.BatchRegenerateRequest
}

func newLowerCapBatch(yes bool, calls map[string]versionCalls) *lowerCapBatch {
	b := &lowerCapBatch{rec: &presentertest.RecordingPresenter{}, pub: newFakePublisher()}
	for _, res := range lowerCapBatchVersions() {
		b.pub.seedExists(res.Tag, true, nil)
	}
	b.req = batchReq(engine.RegenerateSourceFresh, lowerCapBatchVersions(), yes)
	b.req.ProduceBody = func(_ context.Context, _ engine.RegenerateSource, res version.Resolution, _ string, sinks engine.NotesSinks) (string, error) {
		c, ok := calls[res.Tag]
		if !ok {
			c = versionCalls{largest: 1_000}
		}
		sinks.Progress.CallSucceeded(c.largest)
		if c.halved {
			sinks.Progress.Halved(0, 2)
			sinks.Progress.CallSucceeded(c.largest / 2)
		}
		if c.err != nil {
			return "", c.err
		}
		if c.halved {
			sinks.Progress.Merged(3)
		}
		return "## fresh " + res.Tag + "\n", nil
	}
	return b
}

func (b *lowerCapBatch) run(t *testing.T) error {
	t.Helper()
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	_, err := engine.RegenerateAll(t.Context(), batchDeps(b.rec, f), b.pub, t.TempDir(), b.req)
	return err
}

// assertOneBatchEndHint fails unless rec shows one lower-the-cap hint naming bytes,
// after every version's notes and every gate, straight before the close of the run
// when the batch ran to its end.
func assertOneBatchEndHint(t *testing.T, rec *presentertest.RecordingPresenter, bytes int) {
	t.Helper()
	at := lowerCapHints(rec)
	if len(at) != 1 {
		t.Fatalf("lower-the-cap hints = %d, want one", len(at))
	}
	i := at[0]
	if got, want := rec.Events[i].Warn, lowerCapWarning(bytes); got != want {
		t.Errorf("hint = %+v, want %+v", got, want)
	}
	for _, ev := range rec.Events[i+1:] {
		if ev.Kind != presentertest.KindRunFinished {
			t.Errorf("event %s follows the hint, want only the run's close", ev.Kind)
		}
	}
}

func TestRegenerateAll_TwoVersionsWrittenAfterHalving_OneHintAtTheBatchEnd(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		yes     bool
		choices []presenter.Choice
	}{
		{name: "at a terminal", choices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceYes}},
		{name: "under -y", yes: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := newLowerCapBatch(tc.yes, map[string]versionCalls{
				"v1.1.0": {largest: 700_000, halved: true},
				"v1.3.0": {largest: 1_260_000, halved: true},
			})
			b.rec.NextChoices = tc.choices

			if err := b.run(t); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			assertOneBatchEndHint(t, b.rec, 1_260_000)
			if got := finishEvent(t, b.rec).Summary; got != "3 regenerated" {
				t.Errorf("RunFinished.Summary = %q, want %q", got, "3 regenerated")
			}
		})
	}
}

func TestRegenerateAll_LargerCallInAVersionWithNoHalving_SetsTheHintsExactValue(t *testing.T) {
	t.Parallel()

	b := newLowerCapBatch(true, map[string]versionCalls{
		"v1.1.0": {largest: 700_000, halved: true},
		"v1.2.0": {largest: 1_300_001},
		"v1.3.0": {largest: 1_260_000, halved: true},
	})

	if err := b.run(t); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	assertOneBatchEndHint(t, b.rec, 1_300_001)
}

func TestRegenerateAll_LargerCallInAVersionThatFailed_SetsTheHintsValue(t *testing.T) {
	t.Parallel()

	b := newLowerCapBatch(true, map[string]versionCalls{
		"v1.1.0": {largest: 700_000, halved: true},
		"v1.2.0": {largest: 1_300_001, err: &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}},
	})

	if err := b.run(t); err != nil {
		t.Fatalf("RegenerateAll returned unexpected error: %v", err)
	}

	assertOneBatchEndHint(t, b.rec, 1_300_001)
}

func TestRegenerateAll_NoVersionsNotesWrittenAfterHalving_NoHint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		calls map[string]versionCalls
	}{
		{name: "no version halved", calls: map[string]versionCalls{"v1.2.0": {largest: 1_260_000}}},
		{name: "the only halving was in a version that failed and was skipped", calls: map[string]versionCalls{
			"v1.2.0": {largest: 1_260_000, halved: true, err: &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := newLowerCapBatch(true, tc.calls)

			if err := b.run(t); err != nil {
				t.Fatalf("RegenerateAll returned unexpected error: %v", err)
			}

			assertNoLowerCapHint(t, b.rec)
		})
	}
}

func TestRegenerateAll_BatchEndsEarlyAfterNotesWrittenAfterHalving_HintStillPrintsOnceOnTheWayOut(t *testing.T) {
	t.Parallel()

	failedRegen := &ai.GenerationError{Stdout: usageLimitOutput, ExitCode: 1}
	for _, tc := range []struct {
		name    string
		yes     bool
		calls   map[string]versionCalls
		choices []presenter.Choice
		regen   engine.Regenerator
		wantErr error
	}{
		{
			name:    "declined at the halved version's gate",
			calls:   map[string]versionCalls{"v1.1.0": {largest: 1_260_000, halved: true}},
			choices: []presenter.Choice{presenter.ChoiceNo},
		},
		{
			name:    "declined at a later version's gate",
			calls:   map[string]versionCalls{"v1.1.0": {largest: 1_260_000, halved: true}},
			choices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes, presenter.ChoiceNo},
		},
		{
			name:    "a failed r at a later version's gate",
			calls:   map[string]versionCalls{"v1.1.0": {largest: 1_260_000, halved: true}},
			choices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
			regen:   &fakeRegenerator{err: failedRegen},
			wantErr: failedRegen,
		},
		{
			name: "a Ctrl-C at a later version",
			yes:  true,
			calls: map[string]versionCalls{
				"v1.1.0": {largest: 1_260_000, halved: true},
				"v1.2.0": {largest: 1_000, err: context.Canceled},
			},
			wantErr: context.Canceled,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := newLowerCapBatch(tc.yes, tc.calls)
			b.rec.NextChoices = tc.choices
			b.rec.NextLines = []string{"lead with the fix"}
			if tc.regen != nil {
				b.req.ProduceRegenerator = func(engine.RegenerateSource, version.Resolution, engine.RegenRun) engine.Regenerator {
					return tc.regen
				}
			}

			err := b.run(t)

			assertAbortNonZero(t, err)
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("RegenerateAll error = %v, want one matching %v", err, tc.wantErr)
			}
			assertOneBatchEndHint(t, b.rec, 1_260_000)
			if at := lowerCapHints(b.rec); at[0] != len(b.rec.Events)-1 {
				t.Errorf("the hint is not the batch's last event")
			}
		})
	}
}
