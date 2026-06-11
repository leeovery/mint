package commit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// The production transport must satisfy commit's locally-defined Transport seam:
// production wires the real *ai.Transport in, so this compile-time assertion guards
// the contract without coupling production commit code to the ai package's concretions.
var _ commit.Transport = (*ai.Transport)(nil)

// Compile-time assertion that the recording fake also satisfies the seam.
var _ commit.Transport = (*recordingTransport)(nil)

// recordingTransport is the recording fake for the commit.Transport seam: it
// captures every prompt it receives and returns a scripted body/error. It lets the
// generator tests assert the COMPOSED PROMPT (proving L1 + size guard + compose ran
// first) and the body passthrough WITHOUT scripting the real `claude` command
// through the runner — production wires the real ai.Transport, which itself goes
// through the CommandRunner seam.
type recordingTransport struct {
	body    string
	err     error
	prompts []string
}

// Generate records prompt and returns the scripted body/error. The signature
// matches commit.Transport so a *recordingTransport satisfies the seam.
func (rt *recordingTransport) Generate(_ context.Context, prompt string) (string, error) {
	rt.prompts = append(rt.prompts, prompt)
	return rt.body, rt.err
}

// calls reports how many times Generate was invoked — the load-bearing count for
// "the AI is NEVER called" on the too-large-diff guard.
func (rt *recordingTransport) calls() int {
	return len(rt.prompts)
}

// lastPrompt returns the most recent prompt the transport received, failing the
// test if it was never called.
func (rt *recordingTransport) lastPrompt(t *testing.T) string {
	t.Helper()
	if len(rt.prompts) == 0 {
		t.Fatal("transport was never called; no prompt recorded")
	}
	return rt.prompts[len(rt.prompts)-1]
}

// assertArgs fails unless got equals want element-for-element — the exact-argv
// assertion for commit's staged-diff invocation.
func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q (full argv %v)", i, got[i], want[i], got)
		}
	}
}

// containsArg reports whether args contains arg.
func containsArg(args []string, arg string) bool {
	for _, a := range args {
		if a == arg {
			return true
		}
	}
	return false
}

// seedStagedDiff returns a FakeRunner whose single `git` call (commit's L1 staged
// diff) returns diff. The FakeRunner matches on command name only, so a name-keyed
// Seed is sufficient for the one git invocation commit's glue makes.
func seedStagedDiff(diff string) *runner.FakeRunner {
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)
	return r
}

// normalCfg is a config with a generous max_diff_lines ceiling and no
// prompt-control knobs or diff_exclude globs — the common happy-path setup.
func normalCfg() config.Config {
	return config.Config{MaxDiffLines: 50000}
}

// gitInvocation returns the single recorded `git` call, failing the test if commit's
// glue made anything other than exactly one git call.
func gitInvocation(t *testing.T, r *runner.FakeRunner) runner.Invocation {
	t.Helper()
	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("expected exactly one git invocation, got %d: %+v", len(invs), invs)
	}
	return invs[0]
}

func TestGenerator_Generate_ObtainsStagedDiffViaGitDiffCached(t *testing.T) {
	t.Parallel()

	// L1 source is the STAGED diff: commit's glue must invoke `git diff --cached`
	// (staged-only), scoped to the worktree (`-- .`). The fixed baseline argv (no
	// diff_exclude configured) is exactly `git diff --cached -- .`.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "feat: add api"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := gitInvocation(t, r)
	if inv.Name != "git" {
		t.Fatalf("invoked %q, want git", inv.Name)
	}
	want := []string{"diff", "--cached", "--", "."}
	assertArgs(t, inv.Args, want)
}

func TestGenerator_Generate_DoesNotComputeWouldBeStagedDiff(t *testing.T) {
	t.Parallel()

	// Phase 1 is staged-only: the L1 argv must NOT carry git's -a/-A working-tree
	// flags (the would-be-staged source is Phase 2). The single git call is the bare
	// `git diff --cached` and nothing that would compute an unstaged worktree diff.
	r := seedStagedDiff("diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n")
	transport := &recordingTransport{body: "fix: x"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := gitInvocation(t, r)
	for _, arg := range inv.Args {
		switch arg {
		case "-a", "--all", "-A", "--add-all":
			t.Errorf("L1 argv carries a would-be-staged flag %q (Phase 1 is staged-only): %v", arg, inv.Args)
		}
	}
	// Positive guard: the staged-only source is `--cached`.
	if !containsArg(inv.Args, "--cached") {
		t.Errorf("L1 argv does not request the staged diff (--cached): %v", inv.Args)
	}
}

func TestGenerator_Generate_DiffExcludeMapsToExcludePathspecs(t *testing.T) {
	t.Parallel()

	// Each cfg.DiffExclude glob becomes a :(exclude)<glob> pathspec on the staged-diff
	// invocation, in config order, appended AFTER `-- .`. Commit does NOT inherit
	// release's CHANGELOG.md / version_file tiers — ONLY cfg.DiffExclude maps here.
	globs := []string{"skills/**/knowledge.cjs", "*.min.js"}
	r := seedStagedDiff("diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n")
	transport := &recordingTransport{body: "chore: bundle"}

	cfg := normalCfg()
	cfg.DiffExclude = globs

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := gitInvocation(t, r)
	want := []string{"diff", "--cached", "--", ".", ":(exclude)skills/**/knowledge.cjs", ":(exclude)*.min.js"}
	assertArgs(t, inv.Args, want)

	// Commit must not inherit release's hardwired exclusions.
	if containsArg(inv.Args, ":(exclude)CHANGELOG.md") {
		t.Errorf("commit L1 inherited release's CHANGELOG.md exclude (it must not): %v", inv.Args)
	}
}

func TestGenerator_Generate_ExcludedFilesNeverReachThePrompt(t *testing.T) {
	t.Parallel()

	// diff_exclude removes excluded files BEFORE generation: git performs the
	// exclusion via the :(exclude) pathspec, so the excluded path is absent from the
	// post-exclusion diff git returns — and therefore absent from the prompt the
	// transport receives. The fake returns a post-exclusion diff (git already dropped
	// the bundle); the assertion is that whatever git returns is exactly what reaches
	// the prompt, with the exclude pathspec actually issued.
	postExclusionDiff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	r := seedStagedDiff(postExclusionDiff)
	transport := &recordingTransport{body: "feat: api"}

	cfg := normalCfg()
	cfg.DiffExclude = []string{"dist/bundle.js"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	// The exclude pathspec was issued to git (the exclusion happens at L1).
	inv := gitInvocation(t, r)
	if !containsArg(inv.Args, ":(exclude)dist/bundle.js") {
		t.Errorf("excluded glob was not mapped to a :(exclude) pathspec: %v", inv.Args)
	}

	// The prompt carries exactly git's post-exclusion diff; the excluded path never
	// reaches it.
	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, postExclusionDiff) {
		t.Errorf("prompt missing the post-exclusion diff:\n%s", prompt)
	}
	if strings.Contains(prompt, "dist/bundle.js") {
		t.Errorf("excluded path reached the prompt:\n%s", prompt)
	}
}

func TestGenerator_Generate_MaxDiffLinesGuardAppliedBeforeTransport(t *testing.T) {
	t.Parallel()

	// The max_diff_lines guard runs at L1 AFTER diff_exclude and BEFORE any L2 call:
	// an over-ceiling post-exclusion diff short-circuits — the transport is NEVER
	// called — and the failure is the consumed notes.ErrDiffTooLarge sentinel.
	diff := "line a\nline b\nline c\n" // 3 lines
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "must never be returned (AI was called)"}

	cfg := normalCfg()
	cfg.MaxDiffLines = 2 // 3 > 2 → over ceiling

	gen := commit.NewGenerator(r, transport, t.TempDir())
	_, err := gen.Generate(t.Context(), cfg)
	if err == nil {
		t.Fatal("Generate returned nil error for an over-ceiling diff, want notes.ErrDiffTooLarge")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times; the guard must short-circuit L2 entirely", transport.calls())
	}
}

func TestGenerator_Generate_GuardCountsPostExclusionDiff(t *testing.T) {
	t.Parallel()

	// The guard counts the POST-exclusion diff (git's returned stdout, after the
	// :(exclude) pathspecs), not a pre-exclusion count: a diff within the ceiling
	// passes through to the transport even with diff_exclude configured.
	diff := "line a\nline b\n" // 2 lines, == ceiling (inclusive boundary passes)
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "feat: thing"}

	cfg := normalCfg()
	cfg.MaxDiffLines = 2
	cfg.DiffExclude = []string{"*.min.js"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), cfg); err != nil {
		t.Fatalf("Generate returned unexpected error for an at-ceiling diff: %v", err)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times; an at-ceiling diff must reach L2", transport.calls())
	}
}

func TestGenerator_Generate_ReturnsValidatedBodyWhole(t *testing.T) {
	t.Parallel()

	// For a real staged diff the glue returns the transport's validated body WHOLE:
	// no parsing, splitting, or trimming — byte-identical, ready for the commit sink.
	const body = "feat: add login\n\nWire the auth package into the router so users can\nauthenticate at /login.\n"
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: body}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	got, err := gen.Generate(t.Context(), normalCfg())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want it passed through byte-identical %q", got, body)
	}
}

func TestGenerator_Generate_FeedsComposedPromptWithDefaultInstructionsAndDiff(t *testing.T) {
	t.Parallel()

	// The transport receives the COMPOSED prompt: the default commit instructions
	// (proving compose ran) followed by the staged diff (proving L1 fed compose),
	// in that order.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "feat: auth"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, "Conventional Commits") {
		t.Errorf("prompt missing the default commit instructions (compose did not run):\n%s", prompt)
	}
	if !strings.Contains(prompt, diff) {
		t.Errorf("prompt missing the staged diff (L1 did not feed compose):\n%s", prompt)
	}
	// Order: instructions before the diff.
	assertOrder(t, prompt, "Conventional Commits", diff)
}

func TestGenerator_Generate_AppliesCommitPromptKnobs(t *testing.T) {
	t.Parallel()

	// The glue resolves commit's OWN prompt knobs: a [commit].context injects into
	// the default prompt and reaches the transport — proving ResolveInstructions
	// (commit's composer, not release's) is the source of the instructions half.
	const ctxText = "CONTEXT_KNOB_SENTINEL"
	diff := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "chore: x"}

	cfg := normalCfg()
	cfg.Commit = config.Commit{Context: ctxText}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, ctxText) {
		t.Errorf("injected [commit].context did not reach the prompt:\n%s", prompt)
	}
}

func TestGenerator_Generate_SurfacesDiffTooLargeDistinctFromGenerationFailure(t *testing.T) {
	t.Parallel()

	// The oversized-diff failure is DISTINGUISHABLE from a generation failure via
	// errors.Is: notes.ErrDiffTooLarge matches and ai.ErrGenerationFailed does NOT,
	// so Phase 3 can route oversized vs generation-failure.
	diff := "a\nb\nc\nd\n" // 4 lines
	r := seedStagedDiff(diff)
	transport := &recordingTransport{body: "unused"}

	cfg := normalCfg()
	cfg.MaxDiffLines = 1

	gen := commit.NewGenerator(r, transport, t.TempDir())
	_, err := gen.Generate(t.Context(), cfg)
	if err == nil {
		t.Fatal("Generate returned nil error for an over-ceiling diff")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want notes.ErrDiffTooLarge", err)
	}
	if errors.Is(err, ai.ErrGenerationFailed) {
		t.Errorf("oversized-diff error must NOT match ai.ErrGenerationFailed: %v", err)
	}
}

func TestGenerator_Generate_SurfacesTransportFailuresTyped(t *testing.T) {
	t.Parallel()

	// A transport failure surfaces with its TYPED cause preserved (wrapped with %w so
	// errors.Is still matches) and distinguishable from the oversized-diff case — one
	// subtest per transport sentinel.
	cases := []struct {
		name string
		err  error
	}{
		{"generation failed", ai.ErrGenerationFailed},
		{"timeout", ai.ErrTimeout},
		{"command missing", ai.ErrCommandMissing},
	}
	diff := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := seedStagedDiff(diff)
			transport := &recordingTransport{err: tc.err}

			gen := commit.NewGenerator(r, transport, t.TempDir())
			_, err := gen.Generate(t.Context(), normalCfg())
			if err == nil {
				t.Fatalf("Generate returned nil error, want %v", tc.err)
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("error = %v, want it to match %v", err, tc.err)
			}
			// Distinguishable from the oversized-diff case.
			if errors.Is(err, notes.ErrDiffTooLarge) {
				t.Errorf("transport failure must NOT match notes.ErrDiffTooLarge: %v", err)
			}
		})
	}
}

func TestGenerator_Generate_ConsumesL2OneRetryNotReimplemented(t *testing.T) {
	t.Parallel()

	// The L2 one-retry is CONSUMED, not reimplemented: wiring the REAL ai.Transport
	// over a FakeRunner that scripts a bad (empty) first attempt then a good second
	// attempt, the validated body comes back — proving the retry happened INSIDE the
	// transport. Commit's glue makes exactly ONE git (L1) call and does not re-run the
	// transport itself; the two `claude` calls are the transport's own retry.
	const good = "feat: add thing"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"}, nil)
	// First `claude` attempt returns empty (bad content → retryable); the second
	// returns the good body. This is the ai.Transport's internal retry path.
	r.SeedSequence("claude",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},
		runner.ScriptedCall{Result: runner.Result{Stdout: good}},
	)

	transport := ai.NewTransport(r, ai.Config{AICommand: "claude -p"})
	gen := commit.NewGenerator(r, transport, t.TempDir())

	got, err := gen.Generate(t.Context(), normalCfg())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	if got != good {
		t.Errorf("body = %q, want the good second-attempt body %q", got, good)
	}

	// Exactly one git (L1) call and two claude calls (the transport's own retry) —
	// commit's glue did not re-run the transport itself.
	gitCalls, claudeCalls := 0, 0
	for _, inv := range r.Invocations() {
		switch inv.Name {
		case "git":
			gitCalls++
		case "claude":
			claudeCalls++
		}
	}
	if gitCalls != 1 {
		t.Errorf("git called %d times, want exactly 1 (commit's single L1 call)", gitCalls)
	}
	if claudeCalls != 2 {
		t.Errorf("claude called %d times, want 2 (the transport's consumed one-retry)", claudeCalls)
	}
}

func TestGenerator_Generate_SurfacesL1GitError(t *testing.T) {
	t.Parallel()

	// A non-zero git exit on the L1 staged-diff call surfaces as an error (never
	// mistaken for an empty diff), and the transport is never reached.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: not a git repository", ExitCode: 128}, errors.New("exit status 128"))
	transport := &recordingTransport{body: "must never be returned"}

	gen := commit.NewGenerator(r, transport, t.TempDir())
	_, err := gen.Generate(t.Context(), normalCfg())
	if err == nil {
		t.Fatal("Generate returned nil error for a failing git call, want non-nil")
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an L1 failure must short-circuit L2", transport.calls())
	}
}
