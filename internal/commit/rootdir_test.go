package commit_test

// Root-anchoring regression tests: every read-side git call (the emptiness probes,
// the L1 diff sources, the untracked enumeration + addition diffs, and the tree-state
// probe) must run with the repo ROOT as its working directory. The shared `-- .`
// selector and the ls-files enumeration are cwd-relative, so without the anchoring a
// `cd sub && mint commit -a` would PREVIEW only the subtree while the accept-time
// mutations (`git add -u`/`-A`, the whole-index `git commit`) stay repo-wide — a
// reviewed message describing only part of what gets committed. The FakeRunner records
// the working directory per invocation, so the anchoring is asserted directly.

import (
	"context"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// readVerbs are the git subcommands commit issues READ-ONLY (and must anchor at root).
var readVerbs = map[string]bool{"diff": true, "ls-files": true, "status": true}

// assertReadsAnchoredAt asserts every read-side git invocation recorded by r ran with
// dir as its working directory, and that at least minReads such reads happened (so a
// route change cannot silently drain the assertion).
func assertReadsAnchoredAt(t *testing.T, r *runner.FakeRunner, dir string, minReads int) {
	t.Helper()
	reads := 0
	for _, inv := range gitInvocationsOf(r.Invocations()) {
		if len(inv.Args) == 0 || !readVerbs[inv.Args[0]] {
			continue
		}
		reads++
		if inv.Dir != dir {
			t.Errorf("git %v ran in dir %q, want the repo root %q (cwd-relative `-- .` must scope whole-tree)", inv.Args, inv.Dir, dir)
		}
	}
	if reads < minReads {
		t.Errorf("recorded %d read-side git invocations, want at least %d (harness drift?)", reads, minReads)
	}
}

// TestRun_BareCommit_ReadsAnchoredAtRepoRoot proves the bare path's preflight probe and
// L1 staged-diff read run from the resolved repo root.
func TestRun_BareCommit_ReadsAnchoredAtRepoRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rec := &presentertest.RecordingPresenter{}
	r := seedDiffThenCommit("diff --git a/x b/x\n+work")
	deps := newCommitDeps(rec, r, scriptedTransport("feat: anchored"), root)

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// The bare path issues two reads: the emptiness probe and the L1 staged diff.
	assertReadsAnchoredAt(t, r, root, 2)
}

// TestRun_AddAll_ReadsAnchoredAtRepoRoot proves the -A path's probes, tracked diff,
// untracked enumeration, and per-file addition diffs ALL run from the repo root — the
// mode where an unanchored cwd is most damaging (ls-files paths and `-- .` both scope
// to the invocation directory).
func TestRun_AddAll_ReadsAnchoredAtRepoRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                                 // tracked probe (empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "new.go\x00"}},                       // untracked probe (non-empty → proceed)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},                                 // L1 tracked diff (empty)
		runner.ScriptedCall{Result: runner.Result{Stdout: "new.go\x00"}},                       // L1 untracked enumeration (-z)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/new.go b/new.go\n+x"}}, // addition diff
		runner.ScriptedCall{}, // git add -A
		runner.ScriptedCall{}, // git commit -F -
	)
	deps := newCommitDeps(rec, r, scriptedTransport("feat: anchored add-all"), root)
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Two probes + the tracked diff + the enumeration + one addition diff = five reads.
	assertReadsAnchoredAt(t, r, root, 5)
}
