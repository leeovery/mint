package engine_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 4-4: the --autostash escape hatch. When set, mint stashes
// (`git stash push --include-untracked`) BEFORE the clean-tree gate so a dirty tree
// passes, then restores the WIP (`git stash pop`) after — on success AND on
// abort/failure. The load-bearing ordering: on abort mint FIRST runs the surgical
// unwind (back to the clean starting state), THEN pops the stash on top. A pop
// conflict leaves the stash intact and warns; the WIP is never discarded. With no
// WIP to stash the flag is a no-op. Without the flag a dirty tree still aborts at
// the gate. All stash/pop ops flow through the FakeRunner (via the 4-1 wrapper).

// stashSaved is a successful `git stash push` that ACTUALLY stashed WIP: exit 0 with
// git's "Saved working directory" stdout, so the spine records that a pop is owed.
func stashSaved() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{Stdout: "Saved working directory and index state WIP on main: abc1234 prior\n"},
	}
}

// stashNothing models `git stash push` with nothing to stash: exit 0 with git's
// "No local changes to save" stdout — the no-WIP no-op signal (nothing to pop later).
func stashNothing() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{Stdout: "No local changes to save\n"},
	}
}

// stashPopped is a clean `git stash pop`: exit 0; git drops the stash entry itself.
func stashPopped() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{Stdout: "Dropped refs/stash@{0}\n"},
	}
}

// stashPopConflict models a conflicting `git stash pop`: a non-zero exit with a
// conflict message — git leaves the stash entry intact, so mint must NOT drop it.
func stashPopConflict() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{ExitCode: 1, Stderr: "CONFLICT (content): Merge conflict in file.go\n"},
		Err:    errors.New("exit status 1"),
	}
}

// autostashOptions is patchOptions with --autostash set.
func autostashOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, AutoStash: true}
}

// dirtyStatus is git's porcelain output for a dirty tree — what the clean-tree gate
// observes when autostash has NOT (yet) cleaned the tree.
const dirtyStatus = " M file.go\n"

// TestRelease_Autostash_StashesBeforeGate_DirtyTreePasses proves --autostash runs
// `git stash push --include-untracked` BEFORE the clean-tree gate, so the gate (which
// reads a now-clean porcelain) passes and the run proceeds through to a successful
// release — and the stash is popped afterward to restore the WIP.
func TestRelease_Autostash_StashesBeforeGate_DirtyTreePasses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut(""),              // tag --list (no tags)
		stashSaved(),                 // stash push --include-untracked (BEFORE the gate)
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (clean — WIP was stashed)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),          // rev-list left-right count (ahead only)
		ScriptedOut(""),              // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
		stashPopped(),                // stash pop (restore WIP after success)
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (dirty tree passes under --autostash)", err)
	}

	// The stash push ran with --include-untracked, BEFORE the clean-tree gate.
	if !invokedWith(f, "git", "stash", "push", "--include-untracked") {
		t.Errorf("--autostash did not run `git stash push --include-untracked`; got %v", commandLines(f.Invocations()))
	}
	pushAt := indexOfCmd(f, "git", "stash", "push", "--include-untracked")
	gateAt := indexOfCmd(f, "git", "status", "--porcelain")
	if pushAt == -1 || gateAt == -1 || pushAt > gateAt {
		t.Errorf("stash push (at %d) must precede the clean-tree gate (at %d)", pushAt, gateAt)
	}
	// The run finished successfully — the dirty tree passed the gate.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_Autostash_PopsAfterSuccessfulRelease proves the stash is popped after a
// successful release, restoring the WIP — and the pop runs through the FakeRunner.
func TestRelease_Autostash_PopsAfterSuccessfulRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAutostashHappyGit(f, root, "main", "v0.0.1")
	f.SeedSequence("git", stashPopped()) // stash pop after success
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil", err)
	}

	if !invokedWith(f, "git", "stash", "pop") {
		t.Errorf("--autostash did not pop the stash after a successful release; got %v", commandLines(f.Invocations()))
	}
	// The pop ran AFTER the push crossed the PONR.
	pushAt := indexOfCmd(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1")
	popAt := indexOfCmd(f, "git", "stash", "pop")
	if pushAt == -1 || popAt == -1 || popAt < pushAt {
		t.Errorf("stash pop (at %d) must follow the successful push (at %d)", popAt, pushAt)
	}
}

// TestRelease_Autostash_AbortUnwindsBeforePop proves the load-bearing ordering: on a
// pre-PONR abort mint FIRST runs the surgical unwind (reset/tag-delete back to the
// clean start), THEN pops the stash on top. The reset must precede the pop — popping
// before the unwind would apply WIP against mint's release commits.
func TestRelease_Autostash_AbortUnwindsBeforePop(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAutostashThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		pushRejected(),  // push --atomic origin HEAD v0.0.1 (rejected → pre-PONR abort)
		ScriptedOut(""), // unwind: tag -d v0.0.1
		ScriptedOut(""), // unwind: reset --hard startingSHA
		stashPopped(),   // stash pop on TOP of the clean state
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	assertAbortNonZero(t, err)

	// The surgical unwind ran AND the stash was popped.
	resetAt := indexOfCmd(f, "git", "reset", "--hard", startingSHA)
	popAt := indexOfCmd(f, "git", "stash", "pop")
	if resetAt == -1 {
		t.Fatalf("abort did not run the surgical unwind reset; got %v", commandLines(f.Invocations()))
	}
	if popAt == -1 {
		t.Fatalf("abort did not pop the stash; got %v", commandLines(f.Invocations()))
	}
	// LOAD-BEARING: the unwind reset must precede the stash pop.
	if resetAt > popAt {
		t.Errorf("stash pop (at %d) ran BEFORE the unwind reset (at %d); the unwind MUST come first", popAt, resetAt)
	}
	// The tag-delete (also part of the unwind) likewise precedes the pop.
	if tagDelAt := indexOfCmd(f, "git", "tag", "-d", "v0.0.1"); tagDelAt == -1 || tagDelAt > popAt {
		t.Errorf("unwind tag-delete (at %d) must precede the stash pop (at %d)", tagDelAt, popAt)
	}
}

// TestRelease_Autostash_PopConflict_KeepsStashAndWarns proves a pop CONFLICT leaves
// the stash intact (NO `git stash drop`) and warns via the presenter — the WIP is
// never discarded.
func TestRelease_Autostash_PopConflict_KeepsStashAndWarns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAutostashHappyGit(f, root, "main", "v0.0.1")
	f.SeedSequence("git", stashPopConflict()) // stash pop conflicts after success
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// A clean release; only the restoring pop conflicts. The run still succeeds (the
	// tag is public) but warns that the WIP could not be restored cleanly.
	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (pop conflict is warn-only after a successful release)", err)
	}

	// NEVER drop the stash on a conflict — the WIP must be preserved.
	if invokedWith(f, "git", "stash", "drop") {
		t.Errorf("pop conflict ran `git stash drop`; the WIP must be preserved, never discarded")
	}
	// A warn was surfaced pointing the user at the preserved stash.
	if !popConflictWarnRecorded(rec) {
		t.Errorf("pop conflict did not warn that the WIP is preserved in git stash; warns = %v", warnMessages(rec))
	}
}

// TestRelease_Autostash_NoWIP_IsNoOp proves --autostash with nothing to stash is a
// no-op: the push reports "No local changes to save", so nothing was stashed and
// NOTHING is popped afterward.
func TestRelease_Autostash_NoWIP_IsNoOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut(""),              // tag --list
		stashNothing(),               // stash push --include-untracked (NOTHING to stash)
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (already clean)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),          // rev-list left-right count
		ScriptedOut(""),              // ls-remote --tags
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture clean start)
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil", err)
	}

	// The push ran, but because nothing was stashed, NO pop is issued.
	if !invokedWith(f, "git", "stash", "push", "--include-untracked") {
		t.Errorf("--autostash did not run the stash push probe; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "stash", "pop") {
		t.Errorf("--autostash popped though nothing was stashed (no-WIP no-op violated); got %v", commandLines(f.Invocations()))
	}
}

// TestRelease_NoAutostash_DirtyTreeStillAborts proves the Phase 1 behaviour is
// preserved: WITHOUT --autostash a dirty tree aborts at the clean-tree gate and NO
// stash is taken.
func TestRelease_NoAutostash_DirtyTreeStillAborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(dirtyStatus),   // status --porcelain (DIRTY — gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	assertAbortNonZero(t, err)

	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("dirty tree without --autostash did not surface a StageFailed")
	}
	// No stash was taken — autostash is opt-in.
	if invokedWith(f, "git", "stash", "push", "--include-untracked") {
		t.Errorf("a stash was taken without --autostash; the escape hatch must be opt-in")
	}
	assertNoMutation(t, f)
}

// TestRelease_Autostash_PopConflictAfterAbort_KeepsStashAndWarns proves the conflict
// rule holds on the ABORT path too: after the unwind, a conflicting pop leaves the
// stash intact (no drop) and warns — the WIP is never discarded even on failure.
func TestRelease_Autostash_PopConflictAfterAbort_KeepsStashAndWarns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAutostashThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		pushRejected(),     // push rejected → pre-PONR abort
		ScriptedOut(""),    // unwind: tag -d v0.0.1
		ScriptedOut(""),    // unwind: reset --hard startingSHA
		stashPopConflict(), // restoring pop conflicts on top of the clean state
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), autostashOptions())
	assertAbortNonZero(t, err)

	if invokedWith(f, "git", "stash", "drop") {
		t.Errorf("abort-path pop conflict ran `git stash drop`; the WIP must be preserved")
	}
	if !popConflictWarnRecorded(rec) {
		t.Errorf("abort-path pop conflict did not warn that the WIP is preserved; warns = %v", warnMessages(rec))
	}
	// The unwind still ran (reset + tag-delete) before the conflicting pop.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("abort path did not run the surgical unwind before the pop")
	}
}

// popConflictWarnRecorded reports whether any recorded Warn tells the user their WIP
// is preserved in git stash — the pop-conflict guidance.
func popConflictWarnRecorded(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && strings.Contains(ev.Warn.Message, "git stash") {
			return true
		}
	}
	return false
}

// seedAutostashHappyGit scripts the full happy git timeline for an --autostash run:
// the read stages, the stash push (WIP saved), then the rest of the clean spine
// through the atomic push. The caller seeds the trailing stash pop and gh calls.
func seedAutostashHappyGit(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		stashSaved(),                         // stash push --include-untracked (WIP saved)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean — WIP stashed)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag}
		ScriptedOut("0\t1"),                  // rev-list left-right count
		ScriptedOut(""),                      // ls-remote --tags
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture clean start)
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// seedAutostashThroughTag scripts an --autostash run through the annotated tag,
// leaving the spine positioned at the atomic push. The caller seeds the push outcome
// and whatever the unwind + pop need next.
func seedAutostashThroughTag(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		stashSaved(),                         // stash push --include-untracked (WIP saved)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean — WIP stashed)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag}
		ScriptedOut("0\t1"),                  // rev-list left-right count
		ScriptedOut(""),                      // ls-remote --tags
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture clean start)
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
	)
}
