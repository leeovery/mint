package commit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// writeDiffExclude writes a .mint.toml into dir setting only a single diff_exclude
// glob, so the real config.Load threads cfg.DiffExclude into both the preflight
// emptiness probe and the L1 source — proving they read ONE exclusion-filtered
// source. Every other key stays at its default.
func writeDiffExclude(t *testing.T, dir, glob string) {
	t.Helper()
	body := "diff_exclude = [\"" + glob + "\"]\n"
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

// TestRun_StagedAllExcluded_FailsLoudNoAINoMutation proves a repo whose ONLY staged
// changes match a diff_exclude glob fails loud with the empty-staging message and
// mutates nothing — the AI is NEVER invoked on the empty post-exclusion diff. The
// preflight probe now carries the same :(exclude) pathspecs the L1 source uses, so it
// measures the POST-exclusion would-be-staged set: empty → fail loud BEFORE generate.
func TestRun_StagedAllExcluded_FailsLoudNoAINoMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	// The post-exclusion staged probe `git diff --cached --name-only -- . :(exclude)*.min.js`
	// is empty (the only staged file is excluded), then `git status --porcelain` reports
	// the (excluded) staged change exists → the no-changes-staged guidance.
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},               // git diff --cached --name-only -- . :(exclude)*.min.js (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "A app.min.js\n"}}, // git status --porcelain (the excluded change still shows)
	)
	transport := scriptedTransport("must never be returned (all excluded)")
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.StagedOnly

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an all-excluded staged set; want a non-zero fail-loud abort")
	}
	if err.Error() != noChangesStagedMessage {
		t.Errorf("error = %q, want the existing no-changes-staged guidance %q", err.Error(), noChangesStagedMessage)
	}

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded staged set must short-circuit before any AI call", transport.calls())
	}
	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("all-excluded staged set ran `git add` %v; it must never stage", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("all-excluded staged set created %d commit(s); it must never commit", len(commits))
	}
}

// TestRun_PreflightProbeCarriesExcludePathspecs proves the preflight emptiness probe
// now carries the SAME :(exclude) pathspecs the AI's L1 source uses (exact argv), so
// the emptiness verdict and the L1 diff read one exclusion-filtered source. The first
// git invocation is the preflight probe and it must carry `-- . :(exclude)*.min.js`.
func TestRun_PreflightProbeCarriesExcludePathspecs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},               // preflight probe (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "A app.min.js\n"}}, // git status --porcelain
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned"), root)
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an all-excluded staged set; want a non-zero abort")
	}

	gits := gitInvocations(r)
	if len(gits) == 0 {
		t.Fatal("no git invocations recorded; want the preflight emptiness probe first")
	}
	// The preflight probe must read the SAME exclusion-filtered source the L1 staged
	// diff reads: `git diff --cached --name-only -- . :(exclude)*.min.js`.
	want := []string{"diff", "--cached", "--name-only", "--", ".", ":(exclude)*.min.js"}
	assertArgs(t, gits[0].Args, want)
}

// TestRun_AllModeAllExcluded_FailsLoudNoAI proves the same all-excluded scenario on the
// deferred-staging -a path likewise fails loud and never calls the AI: the -a tracked
// probe `git diff HEAD --name-only -- . :(exclude)*.min.js` is empty (the only tracked
// change is excluded), and `git status --porcelain` reports the change exists.
func TestRun_AllModeAllExcluded_FailsLoudNoAI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                // git diff HEAD --name-only -- . :(exclude)*.min.js (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: " M app.min.js\n"}}, // git status --porcelain (the excluded change exists)
	)
	transport := scriptedTransport("must never be returned (all excluded -a)")
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.All

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an all-excluded -a set; want a non-zero fail-loud abort")
	}
	// -a with the only change excluded leaves a non-clean tree whose tracked diff is
	// empty post-exclusion — the existing All-mode message points at -A/--add-all.
	if err.Error() != noTrackedChangesMessage {
		t.Errorf("error = %q, want the existing -a guidance %q", err.Error(), noTrackedChangesMessage)
	}

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded -a set must short-circuit before any AI call", transport.calls())
	}
	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("all-excluded -a set ran `git add` %v; it must never stage", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("all-excluded -a set created %d commit(s); it must never commit", len(commits))
	}
}

// TestRun_AllModeAllExcludedProbeCarriesExcludePathspecs proves the -a preflight probe
// carries the SAME :(exclude) pathspecs the -a L1 source uses (exact argv).
func TestRun_AllModeAllExcludedProbeCarriesExcludePathspecs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                // -a preflight probe (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: " M app.min.js\n"}}, // git status --porcelain
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned"), root)
	deps.Staging = commit.All

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an all-excluded -a set; want a non-zero abort")
	}

	gits := gitInvocations(r)
	if len(gits) == 0 {
		t.Fatal("no git invocations recorded; want the -a preflight emptiness probe first")
	}
	want := []string{"diff", "HEAD", "--name-only", "--", ".", ":(exclude)*.min.js"}
	assertArgs(t, gits[0].Args, want)
}

// TestRun_AddAllModeAllExcluded_FailsLoudNoAI proves the same all-excluded scenario on
// the -A path likewise fails loud and never calls the AI: BOTH the -A tracked probe and
// the untracked probe carry the :(exclude) pathspecs, so an all-excluded untracked set
// is also recognised as empty. With every change excluded the tree's would-be-staged set
// is empty and `git status --porcelain` is clean of NON-excluded changes here (the only
// untracked file is excluded, but status itself still shows it) → the clean-tree line
// fires only when status is empty; an excluded-untracked change keeps status non-empty,
// so -A defensively falls back to the clean-tree message.
func TestRun_AddAllModeAllExcluded_FailsLoudNoAI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                   // git diff HEAD --name-only -- . :(exclude)*.min.js (tracked all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                   // git ls-files --others --exclude-standard -- . :(exclude)*.min.js (untracked all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "?? vendor.min.js\n"}}, // git status --porcelain (the excluded untracked file still shows)
	)
	transport := scriptedTransport("must never be returned (all excluded -A)")
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.AddAll

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an all-excluded -A set; want a non-zero fail-loud abort")
	}
	// -A's empty would-be-staged set defensively falls back to the clean-tree message
	// (the existing emptyStagingError AddAll branch).
	if err.Error() != nothingToCommitMessage {
		t.Errorf("error = %q, want the existing -A defensive clean-tree line %q", err.Error(), nothingToCommitMessage)
	}

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded -A set must short-circuit before any AI call", transport.calls())
	}
	if adds := addInvocations(r); len(adds) != 0 {
		t.Errorf("all-excluded -A set ran `git add` %v; it must never stage", adds)
	}
	if commits := commitInvocations(r); len(commits) != 0 {
		t.Errorf("all-excluded -A set created %d commit(s); it must never commit", len(commits))
	}
}

// TestRun_AddAllModeAllExcludedUntrackedProbeCarriesExcludePathspecs proves the -A
// untracked probe (`git ls-files --others`) also honours the SAME :(exclude) pathspecs
// the -A L1 untracked enumeration uses (exact argv), so an all-excluded untracked set is
// measured the same way the AI path measures it.
func TestRun_AddAllModeAllExcludedUntrackedProbeCarriesExcludePathspecs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                   // tracked probe (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                   // untracked probe (all excluded → empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "?? vendor.min.js\n"}}, // git status --porcelain
	)
	deps := newCommitDeps(rec, r, scriptedTransport("must never be returned"), root)
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err == nil {
		t.Fatal("Run returned nil for an all-excluded -A set; want a non-zero abort")
	}

	gits := gitInvocations(r)
	if len(gits) < 2 {
		t.Fatalf("git invocations = %v, want the tracked probe then the untracked probe", gits)
	}
	wantTracked := []string{"diff", "HEAD", "--name-only", "--", ".", ":(exclude)*.min.js"}
	assertArgs(t, gits[0].Args, wantTracked)
	wantUntracked := []string{"ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude)*.min.js"}
	assertArgs(t, gits[1].Args, wantUntracked)
}

// TestRun_StagedNonExcludedChange_ReachesGenerate proves a repo with at least one
// NON-excluded staged change still passes preflight, reaches Generate, and commits
// normally even with diff_exclude configured: the post-exclusion preflight probe is
// non-empty, so the run proceeds exactly as before.
func TestRun_StagedNonExcludedChange_ReachesGenerate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	const message = "feat: a real, non-excluded change"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "src/app.go\n"}},                                // preflight probe: a non-excluded file remains
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/src/app.go b/src/app.go\n+work"}}, // L1 staged diff
		runner.ScriptedCall{}, // git commit -F -
	)
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if transport.calls() != 1 {
		t.Errorf("transport called %d times; a non-excluded staged change must reach generation", transport.calls())
	}
	commitInv := findCommitInvocation(t, r)
	if commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want the generated body verbatim %q", commitInv.Stdin, message)
	}
}
