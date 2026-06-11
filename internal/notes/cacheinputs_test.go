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

// TestSelector_Reuse_KeyMatch_SkipsAI proves the reuse hook short-circuits the AI on
// the normal-AI path: when the hook reports reused=true, the cached body is returned
// as KindNormalAI and the transport is NEVER called. The hook receives the resolved
// post-exclusion diff and instructions so the engine can recompute the cache key.
func TestSelector_Reuse_KeyMatch_SkipsAI(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	const cachedBody = "REUSED previewed body.\n"
	transport := &recordingTransport{body: "must never be returned (AI was called)"}
	r := runner.NewFakeRunner()
	// Only the degenerate-check diff is assembled on reuse — NO Change Map calls, since
	// the AI is skipped.
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{Stdout: diff}})
	sel := newSelector(t, r, transport)

	var gotDiff, gotInstructions string
	reuse := func(d, instr string) (string, bool, error) {
		gotDiff, gotInstructions = d, instr
		return cachedBody, true, nil
	}

	state := notes.SelectState{LastTag: "v1.0.0"}
	body, kind, inputs, err := sel.SelectBodyWithReuse(t.Context(), state, config.Config{Release: abortRel(), MaxDiffLines: 50000}, reuse)
	if err != nil {
		t.Fatalf("SelectBodyWithReuse returned unexpected error: %v", err)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 (a key match must skip the AI)", transport.calls())
	}
	if body != cachedBody {
		t.Errorf("body = %q, want the reused cached body %q", body, cachedBody)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI (reuse preserves the AI kind)", kind)
	}
	if !inputs.Cacheable || inputs.Diff != diff {
		t.Errorf("inputs = %+v, want Cacheable with the resolved diff %q", inputs, diff)
	}
	// The hook received the exact cache-key inputs the engine hashes.
	if gotDiff != diff {
		t.Errorf("reuse hook diff = %q, want the post-exclusion diff %q", gotDiff, diff)
	}
	if gotInstructions != notes.DefaultPrompt {
		t.Errorf("reuse hook instructions = %q, want the resolved default prompt", gotInstructions)
	}
}

// TestSelector_Reuse_KeyMiss_Generates proves a reuse miss (reused=false) falls
// through to the AI: the transport IS called and its fresh body is returned, never a
// stale cached one.
func TestSelector_Reuse_KeyMiss_Generates(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	const freshBody = "FRESH AI body.\n"
	transport := &recordingTransport{body: freshBody}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	sel := newSelector(t, r, transport)

	reuse := func(string, string) (string, bool, error) { return "", false, nil } // a miss.

	state := notes.SelectState{LastTag: "v1.0.0"}
	body, kind, _, err := sel.SelectBodyWithReuse(t.Context(), state, config.Config{Release: abortRel(), MaxDiffLines: 50000}, reuse)
	if err != nil {
		t.Fatalf("SelectBodyWithReuse returned unexpected error: %v", err)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times, want 1 (a miss must regenerate)", transport.calls())
	}
	if body != freshBody {
		t.Errorf("body = %q, want the freshly generated body %q", body, freshBody)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI", kind)
	}
}

// TestSelector_Reuse_NilHook_AlwaysGenerates proves a nil reuse hook (the dry-run
// write path) is byte-identical to the always-generate path: the AI is called and the
// cache inputs surface unchanged.
func TestSelector_Reuse_NilHook_AlwaysGenerates(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	const aiBody = "## TL;DR\n\nShipped auth.\n"
	transport := &recordingTransport{body: aiBody}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0"}
	body, kind, inputs, err := sel.SelectBodyWithReuse(t.Context(), state, config.Config{Release: abortRel(), MaxDiffLines: 50000}, nil)
	if err != nil {
		t.Fatalf("SelectBodyWithReuse returned unexpected error: %v", err)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times, want 1 (a nil hook always generates)", transport.calls())
	}
	if body != aiBody {
		t.Errorf("body = %q, want the AI body %q", body, aiBody)
	}
	if kind != notes.KindNormalAI || !inputs.Cacheable || inputs.Diff != diff {
		t.Errorf("kind/inputs = %v/%+v, want KindNormalAI with cacheable diff %q", kind, inputs, diff)
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
