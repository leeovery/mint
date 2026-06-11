package notes_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// This file pins the ARBITRARY-RANGE half of the assembly engine — the methods the
// regenerate fresh source (task 5-6) drives with a `vX-1..vX` range instead of the
// forward path's `last_tag..HEAD`. The SAME exclusion tiers (built-in CHANGELOG.md,
// configured diff_exclude globs, strategy-aware version_file) ride on the range
// calls; only the range argv differs. Exclusion is PATH-based (`:(exclude)`
// pathspecs), so a range carrying mint's bookkeeping commit is reproduced exactly as
// the forward source view — the commit is never subtracted as a commit.

// wantRangeArgs is the exact `git diff` argv AssembleRange must issue for a full git
// range: `git diff {range} -- . ':(exclude)CHANGELOG.md'` — the range substituted
// verbatim (NOT a tag with `..HEAD` appended).
func wantRangeArgs(rng string) []string {
	return []string{"diff", rng, "--", ".", ":(exclude)CHANGELOG.md"}
}

func TestAssembler_AssembleRange_DiffsArbitraryRange(t *testing.T) {
	t.Parallel()

	// The range is substituted VERBATIM into the git diff argv — `v1.3.0..v1.4.0`,
	// NOT `v1.4.0..HEAD`. This is the regenerate fresh diff base from 5-3's DiffRange.
	diff := "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	got, err := a.AssembleRange(t.Context(), "v1.3.0..v1.4.0")
	if err != nil {
		t.Fatalf("AssembleRange returned unexpected error: %v", err)
	}

	if got != diff {
		t.Errorf("diff = %q, want %q", got, diff)
	}
	assertGitDiffInvocation(t, r, wantRangeArgs("v1.3.0..v1.4.0"))
}

func TestAssembler_AssembleRange_AlwaysExcludesChangelog(t *testing.T) {
	t.Parallel()

	// CHANGELOG.md is ALWAYS excluded from the regenerate diff via the :(exclude)
	// pathspec — exactly as the forward path. The argv carries the exact pathspec.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	if _, err := a.AssembleRange(t.Context(), "v1.0.0..v1.1.0"); err != nil {
		t.Fatalf("AssembleRange returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantRangeArgs("v1.0.0..v1.1.0"))
}

func TestAssembler_AssembleRange_PlainModeExcludesVersionFile(t *testing.T) {
	t.Parallel()

	// PLAIN mode (version_file set, NO version_pattern): the strategy excludes the
	// whole-file version. The range argv must carry :(exclude)CHANGELOG.md AND
	// :(exclude)<version_file>, in that order — the SAME strategy decision the forward
	// path computes, now consumed over the regenerate range.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{VersionFile: "release.txt"})
	if _, err := a.AssembleRange(t.Context(), "v2.0.0..v2.1.0"); err != nil {
		t.Fatalf("AssembleRange returned unexpected error: %v", err)
	}

	want := append(wantRangeArgs("v2.0.0..v2.1.0"), ":(exclude)release.txt")
	assertGitDiffInvocation(t, r, want)
}

func TestAssembler_AssembleRange_EmbeddedModeDoesNotExcludeVersionFile(t *testing.T) {
	t.Parallel()

	// EMBEDDED mode (version_file + version_pattern): the version line is in real
	// source we WANT in the notes, so the strategy does NOT exclude it. The range argv
	// carries the built-in :(exclude)CHANGELOG.md but NO :(exclude)<version_file>.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/main.go b/main.go\n"}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{
		VersionFile:    "main.go",
		VersionPattern: `RELEASE_VERSION="{version}"`,
	})
	if _, err := a.AssembleRange(t.Context(), "v2.0.0..v2.1.0"); err != nil {
		t.Fatalf("AssembleRange returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantRangeArgs("v2.0.0..v2.1.0"))
}

func TestAssembler_AssembleRange_PathExclusionNotCommitExclusion(t *testing.T) {
	t.Parallel()

	// The regenerate range ALREADY CONTAINS mint's bookkeeping commit. Exclusion is
	// PATH-based (:(exclude) pathspecs), NOT commit-based: the argv carries ONLY the
	// path exclusions (CHANGELOG.md + plain version_file) over the FULL range — there
	// is no attempt to filter/drop the bookkeeping commit. Path exclusion is exactly
	// what reproduces the forward source view.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{VersionFile: "release.txt"})
	if _, err := a.AssembleRange(t.Context(), "v1.4.0..v1.5.0"); err != nil {
		t.Fatalf("AssembleRange returned unexpected error: %v", err)
	}

	// The full range is diffed verbatim; only path pathspecs are appended. No revision
	// surgery (no `^{commit}`, no `--not`, no extra ranges) appears in the argv.
	want := append(wantRangeArgs("v1.4.0..v1.5.0"), ":(exclude)release.txt")
	assertGitDiffInvocation(t, r, want)
}

func TestAssembler_AssembleDiff_StillForwardRangeAfterRefactor(t *testing.T) {
	t.Parallel()

	// The forward AssembleDiff is preserved BYTE-IDENTICAL after the range refactor:
	// it still issues `git diff {lastTag}..HEAD ...`. AssembleDiff is the thin wrapper
	// that builds the forward range and delegates to the shared range path.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, notes.ExcludeConfig{})
	if _, err := a.AssembleDiff(t.Context(), "v1.2.3"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgs("v1.2.3"))
}

func TestAssembler_BuildChangeMapForRange_RidesSameExcludesOverRange(t *testing.T) {
	t.Parallel()

	// The Change Map for an arbitrary range runs the SAME two git calls
	// (--name-status, --numstat) over the range with the SAME post-exclusion set the
	// diff uses (CHANGELOG.md + strategy version_file). The map is computed AFTER
	// exclusion, so an excluded path never appears.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)

	a := notes.NewAssembler(r, notes.ExcludeConfig{VersionFile: "release.txt"})
	got, err := a.BuildChangeMapForRange(t.Context(), "v1.3.0..v1.4.0")
	if err != nil {
		t.Fatalf("BuildChangeMapForRange returned unexpected error: %v", err)
	}

	if got == "" {
		t.Fatal("change map is empty, want a non-empty salience preamble")
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("recorded %d git calls, want 2 (name-status then numstat)", len(invs))
	}
	wantNameStatus := []string{"diff", "--name-status", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)release.txt"}
	wantNumstat := []string{"diff", "--numstat", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)release.txt"}
	assertArgvEqual(t, invs[0].Args, wantNameStatus)
	assertArgvEqual(t, invs[1].Args, wantNumstat)
}

// assertArgvEqual fails unless got equals want element-for-element.
func assertArgvEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestGenerator_GenerateFromRange_AssemblesRangePrependsChangeMapRunsAI(t *testing.T) {
	t.Parallel()

	// The fresh-source generator over a range: it assembles the range diff, computes
	// the Change Map AFTER exclusion, prepends the map to the AI input, and runs the
	// transport. The composed prompt carries the Change Map BEFORE the diff, proving
	// the prepend ordering matches the forward path.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	body := "## TL;DR\n\nShipped auth.\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	transport := &recordingTransport{body: body}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	got, err := gen.GenerateFromRange(t.Context(), "v1.3.0..v1.4.0", config.Config{MaxDiffLines: 50000})
	if err != nil {
		t.Fatalf("GenerateFromRange returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}

	prompt := transport.lastPrompt(t)
	mapIdx := indexOf(prompt, "New package: auth/")
	diffIdx := indexOf(prompt, "diff --git a/auth/login.go")
	if mapIdx < 0 {
		t.Fatalf("prompt missing the Change Map; got:\n%s", prompt)
	}
	if diffIdx < 0 {
		t.Fatalf("prompt missing the diff; got:\n%s", prompt)
	}
	if mapIdx >= diffIdx {
		t.Errorf("Change Map (idx %d) must be prepended BEFORE the diff (idx %d)", mapIdx, diffIdx)
	}

	// The diff git call ranged over v1.3.0..v1.4.0, NOT last_tag..HEAD.
	firstArgs := r.Invocations()[0].Args
	assertArgvEqual(t, firstArgs, wantRangeArgs("v1.3.0..v1.4.0"))
}

// indexOf returns the byte index of sub in s, or -1.
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestGenerator_GenerateFromRange_AppliesMaxDiffLinesGuard(t *testing.T) {
	t.Parallel()

	// The max_diff_lines guard is REUSED unchanged: an over-ceiling range diff returns
	// ErrDiffTooLarge (wrapped) and the AI is NEVER called — exactly as the forward
	// path. The guard runs on the post-exclusion range diff.
	diff := "line1\nline2\nline3\nline4\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)
	transport := &recordingTransport{body: "should not be produced"}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	_, err := gen.GenerateFromRange(t.Context(), "v1.0.0..v1.1.0", config.Config{MaxDiffLines: 2})
	if err == nil {
		t.Fatal("GenerateFromRange returned nil error, want ErrDiffTooLarge")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 — the AI must NOT run on an over-ceiling diff", transport.calls())
	}
}

func TestGenerator_GenerateFromRange_SurfacesTransportFailure(t *testing.T) {
	t.Parallel()

	// AI validation/retry behaves as the forward path: a typed transport failure is
	// surfaced (wrapped, errors.Is still matches) rather than swallowed. The fresh
	// single-mode path keeps the failure SURFACED so 5-12 can intercept for --all.
	diff := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "M\tx.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t1\tx.go\n"}},
	)
	transport := &recordingTransport{err: errTransportFailure}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	_, err := gen.GenerateFromRange(t.Context(), "v1.0.0..v1.1.0", config.Config{MaxDiffLines: 50000})
	if err == nil {
		t.Fatal("GenerateFromRange returned nil error, want the transport failure surfaced")
	}
	if !errors.Is(err, errTransportFailure) {
		t.Errorf("error = %v, want it to wrap the transport failure", err)
	}
}

var errTransportFailure = errors.New("ai notes generation failed")

func TestGenerator_GenerateFromRangeWithContext_AppendsOneTimeContextOverRange(t *testing.T) {
	t.Parallel()

	// The regenerate fresh `r` path: GenerateFromRangeWithContext re-runs the fresh AI
	// path over the resolved `vX-1..vX` range with the user's one-time context APPENDED
	// to the instructions — the range counterpart of GenerateWithContext.
	const oneTime = "Lead with the new auth package; downplay the refactor."
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	transport := &recordingTransport{body: "regenerated body"}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	got, err := gen.GenerateFromRangeWithContext(t.Context(), "v1.3.0..v1.4.0", config.Config{MaxDiffLines: 50000}, oneTime)
	if err != nil {
		t.Fatalf("GenerateFromRangeWithContext returned unexpected error: %v", err)
	}

	if got != "regenerated body" {
		t.Errorf("body = %q, want the AI body %q", got, "regenerated body")
	}
	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, oneTime) {
		t.Errorf("prompt missing the one-time context %q:\n%s", oneTime, prompt)
	}
	// The diff git call ranged over v1.3.0..v1.4.0, NOT last_tag..HEAD.
	assertArgvEqual(t, r.Invocations()[0].Args, wantRangeArgs("v1.3.0..v1.4.0"))
}

func TestGenerator_GenerateFromRangeWithContext_EmptyContextMatchesGenerateFromRange(t *testing.T) {
	t.Parallel()

	// An EMPTY one-time context produces a BYTE-IDENTICAL prompt to GenerateFromRange,
	// so the no-context path is exactly the plain range path.
	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	seed := func() *runner.FakeRunner {
		r := runner.NewFakeRunner()
		r.SeedSequence("git",
			runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
			runner.ScriptedCall{Result: runner.Result{Stdout: "M\tcore/run.go\n"}},
			runner.ScriptedCall{Result: runner.Result{Stdout: "3\t3\tcore/run.go\n"}},
		)
		return r
	}
	root := t.TempDir()

	plainTransport := &recordingTransport{body: "body"}
	plainGen := notes.NewGenerator(notes.NewAssembler(seed(), notes.ExcludeConfig{}), plainTransport, root)
	if _, err := plainGen.GenerateFromRange(t.Context(), "v1.0.0..v1.1.0", config.Config{MaxDiffLines: 50000}); err != nil {
		t.Fatalf("GenerateFromRange returned unexpected error: %v", err)
	}

	emptyTransport := &recordingTransport{body: "body"}
	emptyGen := notes.NewGenerator(notes.NewAssembler(seed(), notes.ExcludeConfig{}), emptyTransport, root)
	if _, err := emptyGen.GenerateFromRangeWithContext(t.Context(), "v1.0.0..v1.1.0", config.Config{MaxDiffLines: 50000}, ""); err != nil {
		t.Fatalf("GenerateFromRangeWithContext returned unexpected error: %v", err)
	}

	if got, want := emptyTransport.lastPrompt(t), plainTransport.lastPrompt(t); got != want {
		t.Errorf("empty-context prompt differs from GenerateFromRange:\n got: %q\nwant: %q", got, want)
	}
}
