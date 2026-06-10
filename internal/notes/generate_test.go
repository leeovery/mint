package notes_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// The production transport must satisfy the locally-defined notes.Transport seam:
// production wires the real *ai.Transport in, so this compile-time assertion guards
// the contract without coupling production notes code to the ai package.
var _ notes.Transport = (*ai.Transport)(nil)

// Compile-time assertion that the recording fake also satisfies the seam.
var _ notes.Transport = (*recordingTransport)(nil)

// recordingTransport is the recording fake for the notes.Transport seam: it
// captures every prompt it receives and returns a scripted body/error. It lets
// the generator tests assert the COMPOSED PROMPT (proving assemble + guard +
// change map + compose ran first) and the body passthrough WITHOUT scripting the
// real `claude` command through the runner — production wires the real
// ai.Transport, which itself goes through the CommandRunner seam.
type recordingTransport struct {
	body    string
	err     error
	prompts []string
}

// Generate records prompt and returns the scripted body/error. The signature
// matches notes.Transport so a *recordingTransport satisfies the seam.
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

// seedNormalPathGit scripts the three ordered git calls the normal path makes:
// AssembleDiff's `git diff` first, then BuildChangeMap's `--name-status`, then
// `--numstat`, IN THAT ORDER. The FakeRunner matches on command name only, so a
// SeedSequence is the seam for the three distinct `git` calls.
func seedNormalPathGit(t *testing.T, diff, nameStatus, numstat string) *runner.FakeRunner {
	t.Helper()
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: nameStatus}},
		runner.ScriptedCall{Result: runner.Result{Stdout: numstat}},
	)
	return r
}

// normalCfg is a config with a generous max_diff_lines ceiling and no
// prompt-control knobs, the common setup for the happy-path tests.
func normalCfg() config.Config {
	return config.Config{MaxDiffLines: 50000}
}

func TestGenerator_Generate_ReturnsValidatedAIBodyForPriorTagRelease(t *testing.T) {
	t.Parallel()

	// For a prior-tag release with a real diff, the normal path returns the AI body
	// the transport produced — a validated generation flows back to the caller.
	const body = "## TL;DR\n\nShipped the auth package.\n\n✨ Added\n- **Login**\n"
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	r := seedNormalPathGit(t, diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	transport := &recordingTransport{body: body}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v1.0.0", normalCfg())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}
}

func TestGenerator_Generate_UsesBodyWholeNoParsingOrSplitting(t *testing.T) {
	t.Parallel()

	// The body is used WHOLE: a multi-line body with section headers survives
	// verbatim — byte-identical to what the transport returned. No parsing,
	// splitting, label extraction, or per-sink reassembly happens.
	const body = "Top narrative line\n\n## Section A\nitem 1\nitem 2\n\n## Section B\nitem 3\n"
	diff := "diff --git a/api/handler.go b/api/handler.go\n@@ -1 +1 @@\n-old\n+new\n"
	r := seedNormalPathGit(t, diff, "M\tapi/handler.go\n", "5\t5\tapi/handler.go\n")
	transport := &recordingTransport{body: body}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v2.0.0", normalCfg())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want it passed through byte-identical %q", got, body)
	}
}

func TestGenerator_Generate_ValidGenerationPassesThroughUnchanged(t *testing.T) {
	t.Parallel()

	// A valid generation passes through UNCHANGED: leading/trailing newlines and
	// internal whitespace are part of the presentation body and must not be trimmed
	// or normalised by the generator.
	const body = "\n  ✨ Added\n  - feature with trailing space   \n\n"
	diff := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"
	r := seedNormalPathGit(t, diff, "M\tx.go\n", "1\t1\tx.go\n")
	transport := &recordingTransport{body: body}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v3.0.0", normalCfg())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want it unchanged %q", got, body)
	}
}

func TestGenerator_Generate_InvokesAssembleGuardChangeMapComposeTransportInOrder(t *testing.T) {
	t.Parallel()

	// ORDER: assemble -> guard -> change map -> compose -> transport. Two
	// independent assertions prove the sequence:
	//   1. The recording transport received a prompt that CONTAINS the diff text,
	//      the Change Map text, AND the default instructions — proving
	//      assemble + guard + changemap + compose all ran before the transport.
	//   2. The FakeRunner recorded the assemble `git diff` BEFORE the change map's
	//      `--name-status`/`--numstat`.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	r := seedNormalPathGit(t, diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	transport := &recordingTransport{body: "notes body"}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), "v1.0.0", normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	// 1. The composed prompt carries instructions, then the Change Map, then the
	// diff, in that order — the compose contract over the upstream pieces.
	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, diff) {
		t.Errorf("prompt missing the assembled diff (assemble did not feed compose):\n%s", prompt)
	}
	if !strings.Contains(prompt, "auth/") {
		t.Errorf("prompt missing the Change Map text (change map did not feed compose):\n%s", prompt)
	}
	if !strings.Contains(prompt, "TL;DR") {
		t.Errorf("prompt missing the default instructions (compose did not run):\n%s", prompt)
	}
	assertNormalPathGitOrder(t, r, "v1.0.0")
}

func TestGenerator_Generate_AIInputIsExactlyChangeMapAndDiffAndPrompt(t *testing.T) {
	t.Parallel()

	// The AI input is EXACTLY instructions + Change Map + capped diff — and NO
	// commit messages. The composed prompt equals ComposePrompt over the resolved
	// instructions, the built Change Map, and the assembled diff; nothing else is
	// smuggled in (no `git log`, no commit subjects).
	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	const nameStatus = "M\tcore/run.go\n"
	const numstat = "3\t3\tcore/run.go\n"
	r := seedNormalPathGit(t, diff, nameStatus, numstat)
	transport := &recordingTransport{body: "body"}
	root := t.TempDir()

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, root)
	if _, err := gen.Generate(t.Context(), "v4.0.0", normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	// Reconstruct the expected prompt from the same building blocks the generator
	// must use — exact equality proves the AI input is those three parts and only
	// those three parts.
	instructions, err := notes.ResolveInstructions(root, config.Release{})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}
	changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus, numstat)).
		BuildChangeMap(t.Context(), "v4.0.0")
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	want := notes.ComposePrompt(instructions, changeMap, diff)

	if got := transport.lastPrompt(t); got != want {
		t.Errorf("AI input mismatch:\n got: %q\nwant: %q", got, want)
	}

	// No commit-message channel: only git calls were made, never `git log` for
	// subjects, and certainly no other binary.
	for _, inv := range r.Invocations() {
		if inv.Name != "git" {
			t.Errorf("unexpected non-git invocation %q (AI input must be diff-derived only)", inv.Name)
		}
		if len(inv.Args) > 0 && inv.Args[0] == "log" {
			t.Errorf("commit messages leaked into the AI input via %v", inv.Args)
		}
	}
}

func TestGenerator_Generate_TooLargeDiff_SurfacesNotesFailureWithoutCallingAI(t *testing.T) {
	t.Parallel()

	// A post-exclusion diff over max_diff_lines surfaces ErrDiffTooLarge as a typed
	// notes failure (matchable via errors.Is) and the AI is NEVER called — the
	// guard short-circuits before any transport call.
	diff := "line1\nline2\nline3\nline4\nline5\n" // 5 lines, ceiling is 2.
	r := seedNormalPathGit(t, diff, "A\tbig.go\n", "5\t0\tbig.go\n")
	transport := &recordingTransport{body: "must never be returned"}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v5.0.0", config.Config{MaxDiffLines: 2})
	if err == nil {
		t.Fatal("Generate returned nil error for an over-ceiling diff, want ErrDiffTooLarge")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
	if got != "" {
		t.Errorf("body = %q, want empty when the guard fails", got)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 (AI must not run on an over-ceiling diff)", transport.calls())
	}
}

func TestGenerator_Generate_TransportTimeout_SurfacesTypedFailureCausePreserved(t *testing.T) {
	t.Parallel()

	// A transport timeout surfaces as a typed notes failure with the CAUSE
	// preserved — errors.Is(returned, ai.ErrTimeout) holds. The generator does NOT
	// decide abort-vs-fallback; it only surfaces the typed cause.
	diff := "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	r := seedNormalPathGit(t, diff, "M\ta.go\n", "1\t1\ta.go\n")
	transport := &recordingTransport{err: ai.ErrTimeout}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v6.0.0", normalCfg())
	if err == nil {
		t.Fatal("Generate returned nil error on a transport timeout, want ai.ErrTimeout surfaced")
	}
	if !errors.Is(err, ai.ErrTimeout) {
		t.Errorf("error = %v, want it to match ai.ErrTimeout (cause preserved)", err)
	}
	if got != "" {
		t.Errorf("body = %q, want empty on a transport failure", got)
	}
}

func TestGenerator_Generate_TransportNotesFailure_SurfacesTypedFailureCausePreserved(t *testing.T) {
	t.Parallel()

	// A transport bad-content failure surfaces as a typed notes failure with the
	// cause preserved — errors.Is(returned, ai.ErrNotesFailure) holds.
	diff := "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	r := seedNormalPathGit(t, diff, "M\ta.go\n", "1\t1\ta.go\n")
	transport := &recordingTransport{err: ai.ErrNotesFailure}

	gen := notes.NewGenerator(notes.NewAssembler(r), transport, t.TempDir())
	got, err := gen.Generate(t.Context(), "v7.0.0", normalCfg())
	if err == nil {
		t.Fatal("Generate returned nil error on a transport notes failure, want ai.ErrNotesFailure surfaced")
	}
	if !errors.Is(err, ai.ErrNotesFailure) {
		t.Errorf("error = %v, want it to match ai.ErrNotesFailure (cause preserved)", err)
	}
	if got != "" {
		t.Errorf("body = %q, want empty on a transport failure", got)
	}
}

// assertNormalPathGitOrder fails unless the three git calls were recorded in the
// normal-path order: AssembleDiff's `git diff` first (no --name-status/--numstat
// selector), then BuildChangeMap's --name-status, then --numstat — proving
// assemble ran before the change map.
func assertNormalPathGitOrder(t *testing.T, r *runner.FakeRunner, lastTag string) {
	t.Helper()

	invs := r.Invocations()
	if len(invs) != 3 {
		t.Fatalf("git invocations = %d, want 3 (assemble diff, name-status, numstat)", len(invs))
	}
	assertGitArgv(t, invs[0], wantArgs(lastTag))
	assertGitArgv(t, invs[1], wantNameStatusArgs(lastTag))
	assertGitArgv(t, invs[2], wantNumstatArgs(lastTag))
}
