package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// TestRelease_MalformedAICommand_AbortsAtConfigLoad proves a malformed ai_command stops
// a release at config load, before any pipeline step and with no AI call — including a
// [commit].ai_command, which release itself never runs. The ✗ config line carries the
// load error naming the level and the cause.
func TestRelease_MalformedAICommand_AbortsAtConfigLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "unterminated quote in release.ai_command",
			config: "[release]\nai_command = 'claude --system-prompt \"Be terse'\n",
			want:   "invalid .mint.toml: release.ai_command has an unterminated quote",
		},
		{
			name:   "empty first word in commit.ai_command",
			config: "[commit]\nai_command = \"'' --flag\"\n",
			want:   "invalid .mint.toml: commit.ai_command names no program",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			// Only the root resolution precedes config.Load; every later call stays unseeded
			// so reaching one would still be recorded and caught below.
			f.Seed("git", runner.Result{Stdout: root}, nil)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
			// The returned error is the engine's abort wrapper; the load error itself is
			// what the ✗ config line renders, asserted exactly below.
			if err == nil {
				t.Fatal("Release returned nil, want a config-error abort")
			}
			if !strings.HasSuffix(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to carry %q", err.Error(), tt.want)
			}

			if gitCmds := gitInvocations(f); len(gitCmds) != 1 || gitCmds[0] != "git rev-parse --show-toplevel" {
				t.Errorf("git timeline = %v, want only the pre-config `git rev-parse --show-toplevel`", gitCmds)
			}
			for _, inv := range f.Invocations() {
				if inv.Name != "git" {
					t.Errorf("non-git command %q invoked despite a config error; no AI call may run", commandLine(inv))
				}
			}

			var messages []string
			for _, ev := range rec.Events {
				if ev.Kind == presentertest.KindStageFailed && ev.StageFailed.Name == "config" {
					messages = append(messages, ev.StageFailed.Message)
				}
			}
			if len(messages) != 1 || messages[0] != tt.want {
				t.Errorf("config StageFailed messages = %q, want exactly [%q] (stages = %v)", messages, tt.want, stageFailures(rec))
			}
		})
	}
}
