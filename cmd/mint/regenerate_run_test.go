package main

import (
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/runner"
	"mint/internal/version"
)

// TestRegenerateRunAxes maps the cmd-layer source/target selection onto the engine's
// optional axis types for the interactive default flow (task 5-10). A supplied source
// flag (SourceSet) maps to a present engine source so the source prompt is skipped; an
// unset source maps to the engine UNSET so the prompt asks. The target mirrors this
// off targetUnset. The mapping is what hands the engine the "ask vs skip" decision.
func TestRegenerateRunAxes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		req        regenerateRequest
		wantSource engine.OptionalRegenerateSource
		wantTarget engine.OptionalRegenerateTarget
	}{
		{
			name:       "no flags map to unset both axes (ask both)",
			req:        regenerateRequest{Source: sourceFresh, SourceSet: false, Target: targetUnset},
			wantSource: engine.SourceUnset(),
			wantTarget: engine.TargetUnset(),
		},
		{
			name:       "explicit fresh maps to a present fresh source (skip the question)",
			req:        regenerateRequest{Source: sourceFresh, SourceSet: true, Target: targetUnset},
			wantSource: engine.SourceOf(engine.RegenerateSourceFresh),
			wantTarget: engine.TargetUnset(),
		},
		{
			name:       "reuse maps to a present reuse source",
			req:        regenerateRequest{Source: sourceReuse, SourceSet: true, Target: targetRelease},
			wantSource: engine.SourceOf(engine.RegenerateSourceReuse),
			wantTarget: engine.TargetOf(engine.RegenerateTargetRelease),
		},
		{
			name:       "target changelog maps to a present changelog target",
			req:        regenerateRequest{Source: sourceFresh, SourceSet: true, Target: targetChangelog},
			wantSource: engine.SourceOf(engine.RegenerateSourceFresh),
			wantTarget: engine.TargetOf(engine.RegenerateTargetChangelog),
		},
		{
			name:       "target both maps to a present both target",
			req:        regenerateRequest{Source: sourceFresh, SourceSet: true, Target: targetBoth},
			wantSource: engine.SourceOf(engine.RegenerateSourceFresh),
			wantTarget: engine.TargetOf(engine.RegenerateTargetBoth),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotSource, gotTarget := regenerateRunAxes(tt.req)
			if gotSource != tt.wantSource {
				t.Errorf("source axis = %+v, want %+v", gotSource, tt.wantSource)
			}
			if gotTarget != tt.wantTarget {
				t.Errorf("target axis = %+v, want %+v", gotTarget, tt.wantTarget)
			}
		})
	}
}

// TestNewRegenerateBodyProducer_Reuse proves the single-version body producer binds its
// fixed Resolution and routes the reuse source through the SHARED Resolution-keyed
// dispatch (newBatchBodyProducer), reading the tag annotation body verbatim. The reuse
// read is git-only, so it is exercisable with the FakeRunner without an AI transport.
// Together with the batch reuse test (TestNewBatchBodyProducer_Reuse) this proves both the
// single-bound and batch-threaded routes hit the same body dispatch.
func TestNewRegenerateBodyProducer_Reuse(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "## reuse body\n"}, nil)

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	produce := newRegenerateBodyProducer(f, config.Config{}, t.TempDir(), res)
	body, err := produce(t.Context(), engine.RegenerateSourceReuse)
	if err != nil {
		t.Fatalf("produce returned unexpected error: %v", err)
	}
	if body != "## reuse body\n" {
		t.Errorf("reuse body = %q, want the verbatim tag annotation body", body)
	}
}

// TestNewRegenerateRegeneratorProducer proves the single-version regenerator producer
// returns NO regenerator for a reuse source (reuse runs the simple confirm, no review
// gate) and a non-nil one for a fresh source (backing the gate's `r` choice so it never
// aborts with errRegeneratorUnavailable).
func TestNewRegenerateRegeneratorProducer(t *testing.T) {
	t.Parallel()

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	produce := newRegenerateRegeneratorProducer(runner.NewFakeRunner(), config.Config{MaxDiffLines: 50000}, t.TempDir(), res)

	if got := produce(engine.RegenerateSourceReuse); got != nil {
		t.Errorf("reuse regenerator = %v, want nil (reuse has no review gate)", got)
	}
	if got := produce(engine.RegenerateSourceFresh); got == nil {
		t.Error("fresh regenerator = nil, want a non-nil per-run regenerator for the `r` choice")
	}
}
