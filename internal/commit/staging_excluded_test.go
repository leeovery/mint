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
// glob, so the real config.Load threads cfg.DiffExclude into the AI's L1 source ONLY.
// diff_exclude is an AI-context filter; the preflight is diff_exclude-blind, so this
// glob never touches the emptiness probe. Every other key stays at its default.
func writeDiffExclude(t *testing.T, dir, glob string) {
	t.Helper()
	body := "diff_exclude = [\"" + glob + "\"]\n"
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

// TestRun_StagedAllExcluded_CommitsViaEditorFallback proves a repo whose ONLY staged
// changes match a diff_exclude glob is STILL committed: the preflight (diff_exclude-blind)
// sees the dirty index and proceeds, the post-exclusion L1 diff is empty so the AI is
// never invoked, and commit routes to the $EDITOR fallback — the human-written save IS the
// accept, and the excluded file gets committed. It also pins the preflight probe argv as
// UNFILTERED (no :(exclude) pathspecs): diff_exclude must never decide whether the tree is
// dirty.
func TestRun_StagedAllExcluded_CommitsViaEditorFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	const saved = "chore: bump app.min.js\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "app.min.js\n"}}, // preflight probe (UNFILTERED): staged index is dirty
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},             // L1 staged diff (filtered): all excluded → empty
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},     // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	transport := scriptedTransport("must never be returned (all excluded)")
	deps := editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.StagedOnly, Transport: transport})

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The preflight probe must read the FULL would-be-staged set, NOT the post-exclusion
	// view: `git diff --cached --name-only -- .` with no :(exclude) pathspecs.
	gits := editorGitInvocations(er)
	if len(gits) == 0 {
		t.Fatal("no git invocations recorded; want the preflight probe first")
	}
	assertArgs(t, gits[0].Args, []string{"diff", "--cached", "--name-only", "--", "."})

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded diff must skip the AI and open the editor", transport.calls())
	}
	if len(er.launches) != 1 {
		t.Fatalf("editor launches = %d, want exactly 1 (the all-excluded fallback opens the editor)", len(er.launches))
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
	if adds := editorAddInvocations(er); len(adds) != 0 {
		t.Errorf("StagedOnly ran `git add` %v; it must commit the index exactly as staged", adds)
	}
}

// TestRun_AllModeAllExcluded_CommitsViaEditorFallback proves the same on the -a path: the
// preflight probe `git diff HEAD --name-only -- .` is UNFILTERED, so an all-excluded
// tracked change still passes preflight; the post-exclusion L1 diff is empty, so commit
// stages tracked changes (`git add -u`) and commits the human-written save.
func TestRun_AllModeAllExcluded_CommitsViaEditorFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	const saved = "chore: rebuild app.min.js\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M app.min.js\n"}}, // preflight probe (UNFILTERED): tracked change present
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                // L1 tracked diff (filtered): all excluded → empty
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},        // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -u (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	transport := scriptedTransport("must never be returned (all excluded -a)")
	deps := editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.All, Transport: transport})

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := editorGitInvocations(er)
	if len(gits) == 0 {
		t.Fatal("no git invocations recorded; want the -a preflight probe first")
	}
	assertArgs(t, gits[0].Args, []string{"diff", "HEAD", "--name-only", "--", "."})

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded -a diff must skip the AI", transport.calls())
	}
	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-u" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -u`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
}

// TestRun_AddAllModeAllExcluded_CommitsViaEditorFallback proves the same on the -A path,
// where the only change is an excluded UNTRACKED file: the preflight runs BOTH the tracked
// probe (`git diff HEAD --name-only -- .`) and the untracked probe (`git ls-files --others
// --exclude-standard -z -- .`), BOTH UNFILTERED, so the untracked file keeps the tree
// dirty and preflight proceeds; the post-exclusion L1 diff is empty (the untracked file is
// excluded), so commit stages everything (`git add -A`) and commits the human-written save.
func TestRun_AddAllModeAllExcluded_CommitsViaEditorFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	const saved = "chore: vendor vendor.min.js\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                  // preflight tracked probe (UNFILTERED): no tracked change
		runner.ScriptedCall{Result: runner.Result{Stdout: "vendor.min.js\x00"}}, // preflight untracked probe (UNFILTERED): untracked file present
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                  // L1 tracked diff (filtered): empty
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                  // L1 untracked enumeration (filtered): all excluded → empty
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},          // git var GIT_EDITOR
		runner.ScriptedCall{}, // git add -A (deferred staging on save)
		runner.ScriptedCall{}, // git commit -F -
	)
	er := &editorRunner{fake: f, saved: saved}
	transport := scriptedTransport("must never be returned (all excluded -A)")
	deps := editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.AddAll, Transport: transport})

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Both preflight probes are UNFILTERED — neither carries the :(exclude) tail.
	gits := editorGitInvocations(er)
	if len(gits) < 2 {
		t.Fatalf("git invocations = %v, want the tracked probe then the untracked probe", gits)
	}
	assertArgs(t, gits[0].Args, []string{"diff", "HEAD", "--name-only", "--", "."})
	assertArgs(t, gits[1].Args, []string{"ls-files", "--others", "--exclude-standard", "-z", "--", "."})

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded -A diff must skip the AI", transport.calls())
	}
	adds := editorAddInvocations(er)
	if len(adds) != 1 || adds[0].Args[len(adds[0].Args)-1] != "-A" {
		t.Fatalf("git add invocations = %v, want exactly one `git add -A`", adds)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying the saved body %q", commits, saved)
	}
}

// TestRun_AllExcludedUnderYes_FailsLoud proves an all-excluded diff on an UNATTENDED run
// (-y) fails loud rather than committing a blank or hanging: the editor fallback has no
// human to write a message, so the no-message-source guard fires — the SAME fail-loud
// behaviour as an oversized diff under -y. Nothing is staged or committed.
func TestRun_AllExcludedUnderYes_FailsLoud(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "app.min.js\n"}}, // preflight probe (UNFILTERED): dirty
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},             // L1 staged diff (filtered): all excluded → empty
	)
	er := &editorRunner{fake: f, saved: "must never be saved"}
	transport := scriptedTransport("must never be returned (all excluded -y)")
	deps := editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.StagedOnly, Transport: transport, Yes: true})

	err := commit.Run(context.Background(), deps)
	if err == nil {
		t.Fatal("Run returned nil for an all-excluded -y run; want a non-zero fail-loud abort")
	}
	if len(er.launches) != 0 {
		t.Errorf("editor launched %d time(s) under -y; the guard must fire before any launch", len(er.launches))
	}
	if commits := editorCommitInvocations(er); len(commits) != 0 {
		t.Errorf("all-excluded -y created %d commit(s); it must fail loud, not commit", len(commits))
	}
}

// TestRun_StagedNonExcludedChange_ReachesGenerate proves a repo with at least one
// NON-excluded staged change still passes preflight, reaches Generate, and commits
// normally even with diff_exclude configured: the preflight probe (unfiltered) is
// non-empty AND the post-exclusion L1 diff carries the non-excluded file, so the run
// proceeds to the AI exactly as before.
func TestRun_StagedNonExcludedChange_ReachesGenerate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeDiffExclude(t, root, "*.min.js")

	const message = "feat: a real, non-excluded change"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "src/app.go\n"}},                                // preflight probe (unfiltered): a file is staged
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/src/app.go b/src/app.go\n+work"}}, // L1 staged diff (filtered): the non-excluded file remains
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
