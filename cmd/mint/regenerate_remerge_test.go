package main

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/runner"
	"mint/internal/version"
)

const (
	remergeNudge = "lead with the export feature"
	remergedBody = "✨ Added\n- The export feature, re-merged   \n"
)

// remergeCfg gives every ai_command layer its own command, so the call's argv names
// the layer it resolved through.
func remergeCfg() config.Config {
	releaseCmd, commitCmd := "release-ai --notes", "commit-ai --message"
	return config.Config{
		MaxPromptBytes: config.DefaultMaxPromptBytes,
		AICommand:      "shared-ai",
		Release:        config.Release{TagPrefix: "v", AICommand: &releaseCmd},
		Commit:         config.Commit{AICommand: &commitCmd},
	}
}

// assertOneReleaseRemerge fails unless regen's `r` with the nudge makes exactly one
// call, through the release ai_command, carrying the nudge and no diff, and returns
// its output byte-identical.
func assertOneReleaseRemerge(t *testing.T, f *runner.FakeRunner, regen engine.Regenerator) {
	t.Helper()
	if regen == nil {
		t.Fatal("fresh regenerator = nil")
	}
	got, err := regen.Regenerate(t.Context(), remergeNudge)
	if err != nil {
		t.Fatalf("Regenerate returned unexpected error: %v", err)
	}
	if got != remergedBody {
		t.Errorf("body = %q, want the re-merge output %q", got, remergedBody)
	}
	invs := f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %v, want exactly one re-merge call", invs)
	}
	if invs[0].Name != "release-ai" || !slices.Equal(invs[0].Args, []string{"--notes"}) {
		t.Errorf("re-merge argv = %s %v, want the release ai_command release-ai [--notes]", invs[0].Name, invs[0].Args)
	}
	if !strings.Contains(invs[0].Stdin, remergeNudge) {
		t.Errorf("re-merge prompt does not carry the nudge %q", remergeNudge)
	}
	if strings.Contains(invs[0].Stdin, "diff --git ") {
		t.Errorf("re-merge prompt carries diff")
	}
}

func remergeRunner() *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.Seed("release-ai", runner.Result{Stdout: remergedBody}, nil)
	return f
}

func TestNewRegenerateRegeneratorProducer_ChunkedNotes_RegenReMergesOnceThroughTheReleaseCommand(t *testing.T) {
	t.Parallel()

	f := remergeRunner()
	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	regen := newRegenerateRegeneratorProducer(f, remergeCfg(), t.TempDir(), res)(engine.RegenerateSourceFresh, engine.RegenRun{Merged: func() *notes.MergeInputs { return &notes.MergeInputs{} }})

	assertOneReleaseRemerge(t, f, regen)
}

func TestNewBatchRegeneratorProducer_ChunkedNotes_RegenReMergesOnceThroughTheReleaseCommand(t *testing.T) {
	t.Parallel()

	f := remergeRunner()
	res := version.Resolution{Tag: "v1.3.0", PreviousTag: "v1.2.0"}
	regen := newBatchRegeneratorProducer(f, remergeCfg(), t.TempDir())(engine.RegenerateSourceFresh, res, engine.RegenRun{Merged: func() *notes.MergeInputs { return &notes.MergeInputs{} }})

	assertOneReleaseRemerge(t, f, regen)
}
