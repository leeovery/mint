package notes_test

import (
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// TestSelector_CacheInputs_NormalAIPath proves the normal-AI path surfaces the
// EXACT cache-key inputs alongside the body: the post-diff_exclude diff fed to the
// AI and the resolved prompt/context instructions. These feed the dry-run note
// cache key (post-diff_exclude diff + version + prompt/context); the version is
// the caller's, not the selector's.
func TestSelector_CacheInputs_NormalAIPath(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	const aiBody = "## TL;DR\n\nShipped the auth package.\n"
	transport := &recordingTransport{body: aiBody}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: false}
	body, kind, inputs, err := sel.SelectBodyWithCacheInputs(t.Context(), state, config.Config{Release: abortRel(), MaxDiffLines: 50000})
	if err != nil {
		t.Fatalf("SelectBodyWithCacheInputs returned unexpected error: %v", err)
	}
	if body != aiBody {
		t.Errorf("body = %q, want the AI body %q", body, aiBody)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI", kind)
	}
	if !inputs.Cacheable {
		t.Errorf("inputs.Cacheable = false, want true on the normal-AI path")
	}
	// The diff is the post-diff_exclude diff that fed the AI — byte-for-byte the
	// assembled diff, NOT a re-assembled or HEAD-derived value.
	if inputs.Diff != diff {
		t.Errorf("inputs.Diff = %q, want the assembled post-exclusion diff %q", inputs.Diff, diff)
	}
	// The instructions are the resolved prompt (default prompt here, no override/context).
	if inputs.Instructions != notes.DefaultPrompt {
		t.Errorf("inputs.Instructions = %q, want the resolved default prompt", inputs.Instructions)
	}
}

// TestSelector_CacheInputs_NonAIPathsNotCacheable proves the non-AI paths (first
// release, degenerate, --no-ai) report Cacheable=false: there is no stochastic AI
// body to cache, so the dry-run cache write is skipped for them.
func TestSelector_CacheInputs_NonAIPathsNotCacheable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state notes.SelectState
		seed  func(*runner.FakeRunner)
	}{
		{
			name:  "first release",
			state: notes.SelectState{FirstRelease: true},
			seed:  func(*runner.FakeRunner) {},
		},
		{
			name:  "degenerate diff",
			state: notes.SelectState{LastTag: "v1.0.0"},
			seed: func(r *runner.FakeRunner) {
				r.Seed("git", runner.Result{Stdout: "   \n"}, nil) // whitespace-only diff.
			},
		},
		{
			name:  "no-ai",
			state: notes.SelectState{LastTag: "v1.0.0", NoAI: true},
			seed: func(r *runner.FakeRunner) {
				r.SeedSequence("git",
					runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/a.go b/a.go\n+x\n"}},
					runner.ScriptedCall{Result: runner.Result{Stdout: "Add feature\n"}},
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transport := &recordingTransport{body: "must never be returned"}
			r := runner.NewFakeRunner()
			tt.seed(r)
			sel := newSelector(t, r, transport)

			_, _, inputs, err := sel.SelectBodyWithCacheInputs(t.Context(), tt.state, config.Config{Release: abortRel()})
			if err != nil {
				t.Fatalf("SelectBodyWithCacheInputs returned unexpected error: %v", err)
			}
			if inputs.Cacheable {
				t.Errorf("inputs.Cacheable = true on the %s path, want false (no AI body to cache)", tt.name)
			}
		})
	}
}
