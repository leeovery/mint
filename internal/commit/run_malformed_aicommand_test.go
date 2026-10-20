package commit_test

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// TestRun_InvalidConfig_AbortsAtConfigLoad proves a malformed ai_command at any level, a
// non-positive max_prompt_bytes or the removed max_diff_lines stops `mint commit` at
// config load: the load error naming the key and cause is surfaced once, and no command
// runs — no preflight read, no AI call, no commit.
func TestRun_InvalidConfig_AbortsAtConfigLoad(t *testing.T) {
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
			name:   "lone backslash in shared ai_command",
			config: "ai_command = 'claude -p \\'\n",
			want:   "invalid .mint.toml: ai_command ends in a lone backslash",
		},
		{
			name:   "empty first word in commit.ai_command",
			config: "[commit]\nai_command = \"'' --flag\"\n",
			want:   "invalid .mint.toml: commit.ai_command names no program",
		},
		{
			name:   "zero max_prompt_bytes",
			config: "max_prompt_bytes = 0\n",
			want:   "invalid .mint.toml: max_prompt_bytes must be a positive number of bytes",
		},
		{
			name:   "negative max_prompt_bytes",
			config: "max_prompt_bytes = -1\n",
			want:   "invalid .mint.toml: max_prompt_bytes must be a positive number of bytes",
		},
		{
			name:   "removed max_diff_lines",
			config: "max_diff_lines = 50000\n",
			want:   "invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeMintToml(t, root, tt.config)
			r := runner.NewFakeRunner()
			rec := &presentertest.RecordingPresenter{}

			err := commit.Run(context.Background(), realTransportDeps(rec, r, root))
			if err == nil {
				t.Fatal("Run returned nil, want a config-error abort")
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err.Error(), tt.want)
			}
			if msgs := stageFailedMessages(rec); len(msgs) != 1 || msgs[0] != tt.want {
				t.Errorf("StageFailed messages = %q, want exactly [%q]", msgs, tt.want)
			}
			if invs := r.Invocations(); len(invs) != 0 {
				t.Errorf("%d command(s) ran despite a config error, want none: %v", len(invs), invs)
			}
		})
	}
}

// TestRun_BlankCommitAICommand_RunsSharedCommand proves a blank [commit].ai_command is
// not malformed: config loads and the blank level falls through to the shared command.
func TestRun_BlankCommitAICommand_RunsSharedCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "ai_command = 'llm -m gpt-4o'\n[commit]\nai_command = '  '\n")
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("llm", runner.Result{Stdout: "feat: shared command drove the call"}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if got := aiInvocationStdin(r, "llm", "-m", "gpt-4o"); got == "" {
		t.Error("shared `llm -m gpt-4o` was not invoked with a prompt on stdin")
	}
}

// TestRun_EveryAICommandLevelBlank_RunsShippedDefault proves blank values at every level
// load and resolve to the shipped default command.
func TestRun_EveryAICommandLevelBlank_RunsShippedDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintToml(t, root, "ai_command = ''\n[release]\nai_command = ' '\n[commit]\nai_command = ''\n")
	r := seedAIDiffThenCommit("diff --git a/x b/x\n+work")
	r.Seed("claude", runner.Result{Stdout: "feat: default drove the call"}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := commit.Run(context.Background(), realTransportDeps(rec, r, root)); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if got := aiInvocationStdin(r, "claude", leanClaudeArgs...); got == "" {
		t.Error("shipped default claude command was not invoked with a prompt on stdin")
	}
}
