package notes_test

import (
	"errors"
	"testing"

	"mint/internal/notes"
	"mint/internal/runner"
)

// wantArgs is the exact git argv AssembleDiff must issue for a given last tag:
// `git diff {lastTag}..HEAD -- . ':(exclude)CHANGELOG.md'`. CHANGELOG.md is the
// built-in non-configurable always-exclude, applied by git via the :(exclude)
// pathspec — mint does no Go-side text stripping.
func wantArgs(lastTag string) []string {
	return []string{"diff", lastTag + "..HEAD", "--", ".", ":(exclude)CHANGELOG.md"}
}

// wantArgsWithExcludes is wantArgs plus one :(exclude)<glob> entry per configured
// diff_exclude glob, appended AFTER the built-in :(exclude)CHANGELOG.md in config
// order. Each glob is carried verbatim into the pathspec — git, not Go, matches it.
func wantArgsWithExcludes(lastTag string, globs ...string) []string {
	args := wantArgs(lastTag)
	for _, g := range globs {
		args = append(args, ":(exclude)"+g)
	}
	return args
}

// assertGitDiffInvocation fails unless exactly one git call was recorded with the
// exact expected argv. The exclude pathspec must match byte-for-byte — it is the
// load-bearing assertion that git, not Go, performs the exclusion.
func assertGitDiffInvocation(t *testing.T, r *runner.FakeRunner, want []string) {
	t.Helper()

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	got := invs[0]
	if got.Name != "git" {
		t.Errorf("command = %q, want %q", got.Name, "git")
	}
	if len(got.Args) != len(want) {
		t.Fatalf("args = %v, want %v", got.Args, want)
	}
	for i := range want {
		if got.Args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q (full argv %v)", i, got.Args[i], want[i], got.Args)
		}
	}
}

func TestAssembler_AssembleDiff_DiffsLastTagToHEAD(t *testing.T) {
	t.Parallel()

	// The diff base is last_tag..HEAD: AssembleDiff must invoke git diff over that
	// range and return git's stdout verbatim.
	diff := "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v1.2.3")
	if err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	if got != diff {
		t.Errorf("diff = %q, want %q", got, diff)
	}
	assertGitDiffInvocation(t, r, wantArgs("v1.2.3"))
}

func TestAssembler_AssembleDiff_ExcludesChangelogViaPathspec(t *testing.T) {
	t.Parallel()

	// CHANGELOG.md is excluded by git via the :(exclude)CHANGELOG.md pathspec, not
	// by Go text stripping. The argv must carry that exact pathspec.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, nil)
	if _, err := a.AssembleDiff(t.Context(), "v0.9.0"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgs("v0.9.0"))
}

func TestAssembler_AssembleDiff_ConfiguredGlob_AppliedOnTopOfChangelog(t *testing.T) {
	t.Parallel()

	// A configured diff_exclude glob becomes a :(exclude)<glob> pathspec entry IN
	// ADDITION to the built-in :(exclude)CHANGELOG.md. The argv must carry BOTH, in
	// order (CHANGELOG.md first, then the configured glob).
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, []string{"skills/**/knowledge.cjs"})
	if _, err := a.AssembleDiff(t.Context(), "v0.9.0"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgsWithExcludes("v0.9.0", "skills/**/knowledge.cjs"))
}

func TestAssembler_AssembleDiff_MultipleGlobs_AllAppliedInOrder(t *testing.T) {
	t.Parallel()

	// Multiple diff_exclude globs ALL apply, each as its own :(exclude)<glob> entry,
	// in config order, after the built-in CHANGELOG.md exclusion.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	globs := []string{"skills/**/knowledge.cjs", "*.min.js"}
	a := notes.NewAssembler(r, globs)
	if _, err := a.AssembleDiff(t.Context(), "v1.5.0"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgsWithExcludes("v1.5.0", globs...))
}

func TestAssembler_AssembleDiff_GlobMatchingNothing_IsHarmless(t *testing.T) {
	t.Parallel()

	// A glob that matches no path is HARMLESS: mint does NO Go-side matching, so the
	// pathspec simply carries the glob and git no-ops it. The assertion is structural —
	// the glob rides in the argv unchanged; nothing special-cases a no-match glob.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-x\n+y\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)

	a := notes.NewAssembler(r, []string{"no/such/path/**"})
	got, err := a.AssembleDiff(t.Context(), "v2.1.0")
	if err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	if got != diff {
		t.Errorf("diff = %q, want git output passed through verbatim %q", got, diff)
	}
	assertGitDiffInvocation(t, r, wantArgsWithExcludes("v2.1.0", "no/such/path/**"))
}

func TestAssembler_AssembleDiff_ForceAddedTrackedFileMatchingGlob_ExcludedByPathspec(t *testing.T) {
	t.Parallel()

	// A force-added (gitignored-but-tracked) file matching a configured glob is STILL
	// excluded — git applies the :(exclude) pathspec to it like any tracked path. mint
	// does NO Go re-filtering; the load-bearing assertion is that the glob rides in the
	// argv, so git is the one performing the exclusion.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	a := notes.NewAssembler(r, []string{"dist/bundle.js"})
	if _, err := a.AssembleDiff(t.Context(), "v3.0.0"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgsWithExcludes("v3.0.0", "dist/bundle.js"))
}

func TestAssembler_AssembleDiff_AbsentDiffExclude_ExcludesOnlyChangelog(t *testing.T) {
	t.Parallel()

	// With no configured globs (nil), AssembleDiff must reproduce EXACTLY the Phase-2
	// behaviour: the only exclude is :(exclude)CHANGELOG.md, with no extra entries.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "diff --git a/api.go b/api.go\n"}, nil)

	a := notes.NewAssembler(r, nil)
	if _, err := a.AssembleDiff(t.Context(), "v0.8.0"); err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	assertGitDiffInvocation(t, r, wantArgs("v0.8.0"))
}

func TestAssembler_AssembleDiff_ChangelogOnlyChange_ReturnsEmptyDiff(t *testing.T) {
	t.Parallel()

	// When the only modification in the range is CHANGELOG.md, git's :(exclude)
	// pathspec filters it out, so git returns an empty post-exclusion diff. That
	// empty result is NOT an error — it feeds the degenerate path downstream.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v2.0.0")
	if err != nil {
		t.Fatalf("AssembleDiff returned unexpected error on empty diff: %v", err)
	}

	if got != "" {
		t.Errorf("diff = %q, want empty string", got)
	}
	assertGitDiffInvocation(t, r, wantArgs("v2.0.0"))
}

func TestAssembler_AssembleDiff_ForceAddedGitignoredFile_PassesGitOutputThrough(t *testing.T) {
	t.Parallel()

	// A gitignored-but-force-added file is tracked, so git includes it in the
	// commit-to-commit diff. mint does NO Go-side path re-filtering: whatever git
	// outputs flows through unchanged. The assembler is a passthrough over git's
	// stdout, so the force-added file's hunk survives verbatim.
	diff := "diff --git a/dist/bundle.js b/dist/bundle.js\n@@ -0,0 +1 @@\n+console.log(1)\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	if got != diff {
		t.Errorf("diff = %q, want git output passed through verbatim %q", got, diff)
	}
}

func TestAssembler_AssembleDiff_ReturnsPostExclusionDiffText(t *testing.T) {
	t.Parallel()

	// The assembler returns the RAW post-exclusion diff text (git's stdout) for
	// downstream layering — no Change Map preamble, no max_diff_lines cap added
	// here. The returned text equals git's stdout byte-for-byte.
	diff := "diff --git a/x.go b/x.go\nindex 111..222 100644\n--- a/x.go\n+++ b/x.go\n@@ -1,2 +1,2 @@\n a\n-b\n+c\n"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: diff}, nil)

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v3.1.4")
	if err != nil {
		t.Fatalf("AssembleDiff returned unexpected error: %v", err)
	}

	if got != diff {
		t.Errorf("diff = %q, want post-exclusion text unchanged %q", got, diff)
	}
}

func TestAssembler_AssembleDiff_CommandNotFound_SurfacesDistinguishableError(t *testing.T) {
	t.Parallel()

	// A missing git binary is reported as a distinguishable condition matched with
	// errors.Is(runner.ErrCommandNotFound), mirroring the sibling packages, so the
	// caller can tell an absent tool apart from a git that ran and failed.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v1.0.0")
	if err == nil {
		t.Fatalf("AssembleDiff returned nil error, want a command-not-found condition")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match runner.ErrCommandNotFound", err)
	}
	if got != "" {
		t.Errorf("diff = %q, want empty on failure", got)
	}
}

func TestAssembler_AssembleDiff_GitFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A git that runs and exits non-zero unexpectedly (e.g. a bad range) is a real
	// error — NOT a normal empty diff — and must be surfaced rather than swallowed.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{
		Stderr:   "fatal: bad revision 'v9.9.9..HEAD'\n",
		ExitCode: 128,
	}, errors.New("exit status 128"))

	a := notes.NewAssembler(r, nil)
	got, err := a.AssembleDiff(t.Context(), "v9.9.9")
	if err == nil {
		t.Fatalf("AssembleDiff returned nil error, want the git failure surfaced")
	}
	if errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, must not be classified as command-not-found", err)
	}
	if got != "" {
		t.Errorf("diff = %q, want empty on failure", got)
	}
}
