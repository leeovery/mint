package engine_test

// This file pins task 6-4: the consolidation/rewiring that routes the engine's AI
// transport through the ONE validated config.Config and proves a bad config aborts
// the run BEFORE any pipeline work begins.
//
//   - ai_command rewiring: the documented top-level ai_command key MUST drive the
//     notes AI invocation (its name + args reach the runner). The pinned default
//     (claude -p --model sonnet) still applies on a zero-config run.
//   - up-front abort: an unknown-key / bad-type config error MUST abort in Stage 1
//     (config load) BEFORE version determination, preflight, or notes — so NONE of the
//     pipeline git stages are reached.
//
// The provider-VALUE carve-out (provider="gitlab" loads clean and downgrades, NOT a
// config error) is pinned end-to-end in release_downgrade_test.go and at the config
// level in internal/config; it is deliberately NOT re-asserted here.

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// TestRelease_AICommand_ConfigValueDrivesTransport proves the documented top-level
// ai_command key drives the AI invocation: a custom command in .mint.toml is the
// command the notes transport runs (name + args reach the runner with the composed
// prompt on stdin), NOT the default `claude -p --model sonnet`. It rides the real prior-tag normal-AI
// path so the transport is the production ai.Transport over the FakeRunner.
func TestRelease_AICommand_ConfigValueDrivesTransport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "ai_command = \"mybot gen --json\"\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	// The configured ai_command's BINARY (mybot) is the runner key — the transport
	// whitespace-splits "mybot gen --json" into name + args. Seeding `claude` here would
	// never be reached; an unseeded `mybot` would error the run, so the seed proves the
	// config value (not the default) is what gets invoked.
	f.Seed("mybot", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The configured ai_command — its binary AND its args — drove the AI call, with the
	// composed prompt piped on stdin. The default `claude -p --model sonnet` was never invoked.
	if got := stdinOf(t, f, "mybot", "gen", "--json"); got == "" {
		t.Errorf("configured ai_command %q was not invoked with a prompt on stdin", "mybot gen --json")
	}
	if invokedWith(f, "claude", "-p", "--model", "sonnet") {
		t.Errorf("default `claude -p --model sonnet` was invoked despite a configured ai_command; config value did not drive the transport")
	}

	// The body the configured command returned still reaches the sinks unchanged.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want AI body %q", got, aiBody)
	}
}

// TestRelease_AICommand_DefaultDrivesTransport proves a zero-config run still uses the
// documented default `claude -p --model sonnet`: with no ai_command set, the transport
// invokes the pinned default with the composed prompt on stdin — the consolidation
// preserves the default exactly.
func TestRelease_AICommand_DefaultDrivesTransport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := stdinOf(t, f, "claude", "-p", "--model", "sonnet"); got == "" {
		t.Errorf("default `claude -p --model sonnet` was not invoked with a prompt on stdin")
	}
}

// TestRelease_BadConfig_AbortsBeforePipelineWork proves the validation sequencing: a
// bad config (an unknown key OR a bad type) aborts the run in Stage 1 (config load),
// BEFORE version determination, preflight, or notes — so NONE of those pipeline git
// stages are reached. The run resolves the repo root (the only git call that precedes
// config.Load) and then aborts; the release-branch resolution, the version `tag
// --list`, the preflight `fetch`/gates, and the notes assembly are all proven absent
// on the recorded git timeline, and the run aborts non-zero.
func TestRelease_BadConfig_AbortsBeforePipelineWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "unknown top-level key", config: "not_a_real_key = true\n"},
		{name: "unknown release key", config: "[release]\nbogus = 1\n"},
		{name: "bad type max_diff_lines", config: "max_diff_lines = \"lots\"\n"},
		{name: "bad type publish", config: "[release]\npublish = \"yes\"\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			// Seed ONLY the root resolution — the single git call that precedes config.Load.
			// Every later pipeline git call (symbolic-ref, tag --list, fetch, gates, diff) is
			// deliberately UNSEEDED: if the run reached any of them the FakeRunner would still
			// record the invocation, which the assertions below catch.
			f.Seed("git", runner.Result{Stdout: root}, nil)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
			if err == nil {
				t.Fatalf("Release returned nil, want a config-error abort for %q", tt.config)
			}

			// The ONLY git call the run made was the pre-config root resolution. No pipeline
			// stage (version / preflight / notes) was reached.
			gitCmds := gitInvocations(f)
			if len(gitCmds) != 1 || gitCmds[0] != "git rev-parse --show-toplevel" {
				t.Fatalf("git timeline = %v, want only the pre-config `git rev-parse --show-toplevel` (config error must abort before pipeline work)", gitCmds)
			}

			// Explicitly assert the version, preflight, and notes stages were NOT reached.
			for _, forbidden := range []string{
				"git symbolic-ref --short refs/remotes/origin/HEAD", // release-branch resolution
				"git tag --list",   // version determination
				"git fetch --tags", // preflight
			} {
				if containsCommand(gitCmds, forbidden) {
					t.Errorf("pipeline stage reached despite a config error: %q was invoked", forbidden)
				}
			}

			// The abort surfaced as a config StageFailed (not a version/preflight failure),
			// proving the config load is what aborted the run.
			if !hasStageFailed(rec, "config") {
				t.Errorf("no `config` StageFailed recorded; the abort did not come from config load (stages = %v)", stageFailures(rec))
			}
		})
	}
}

// gitInvocations returns the recorded "git" command lines (name + args) in order.
func gitInvocations(f *runner.FakeRunner) []string {
	var out []string
	for _, inv := range f.Invocations() {
		if inv.Name == "git" {
			out = append(out, commandLine(inv))
		}
	}
	return out
}

// containsCommand reports whether cmds contains an exact command line.
func containsCommand(cmds []string, want string) bool {
	for _, c := range cmds {
		if c == want {
			return true
		}
	}
	return false
}

// hasStageFailed reports whether a StageFailed event with the given stage name was
// recorded.
func hasStageFailed(rec *presentertest.RecordingPresenter, name string) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed && ev.StageFailed.Name == name {
			return true
		}
	}
	return false
}

// stageFailures returns the names of every recorded StageFailed event (for assertion
// diagnostics).
func stageFailures(rec *presentertest.RecordingPresenter) []string {
	var out []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			out = append(out, ev.StageFailed.Name)
		}
	}
	return out
}
