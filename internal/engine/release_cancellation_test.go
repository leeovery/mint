package engine_test

import (
	"context"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 10-3: a SIGINT/SIGTERM-cancelled context observed BEFORE the
// atomic push (the point of no return) is treated as a pre-PONR failure — it routes
// through the SAME surgical unwind (4-2) every other pre-push failure takes, so the
// bookkeeping commit is reset, a created tag deleted, and any --autostash WIP popped
// back on top of the clean state. The cancellation is caught in the window between the
// bookkeeping commit and the atomic push: the push (the PONR op) is NEVER issued, so
// the warn-only post-PONR contract is left untouched.
//
// The FakeRunner ignores ctx, so the cancellation is observable ONLY through the
// spine's explicit pre-PONR ctx.Err() check — exactly the production gap a bare
// context.Background() left open.

// TestRelease_CancelledBeforePush_Surgical_ResetsAndDeletesNothing proves a context
// cancelled before the atomic push routes through the surgical unwind: the bookkeeping
// commit mint made is reset to the captured starting sha, and because the cancellation
// is caught BEFORE the tag/push the push is never issued and no tag is deleted.
func TestRelease_CancelledBeforePush_Surgical_Resets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// Drive the spine through the bookkeeping commit and provider-detection read,
	// leaving it positioned at the conditional gh-auth gate / tag / push. The unwind
	// then resets the one tracked commit.
	seedHappyGitThroughCommit(f, root, "main", "v0.0.1")
	f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startingSHA
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// A context already cancelled (the SIGINT/SIGTERM signal already fired): the
	// FakeRunner ignores ctx, so the spine runs every read/commit normally and the
	// cancellation is caught only at the explicit pre-PONR gate before the push.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := engine.Release(ctx, newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	// The push (the PONR op) was NEVER issued — the cancellation was caught before it.
	if invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("a cancelled context crossed the PONR; the atomic push must never run; got %v", commandLines(f.Invocations()))
	}
	// The surgical unwind reset the one tracked bookkeeping commit back to the clean start.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("cancellation before the push did not run the surgical unwind reset; got %v", commandLines(f.Invocations()))
	}
	// No tag was created before the cancellation, so none is deleted.
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("cancellation deleted a tag though none was created")
	}
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_CancelledBeforePush_Autostash_UnwindsThenPops proves the cancellation
// path honours the load-bearing autostash ordering: on a pre-PONR cancellation mint
// FIRST runs the surgical unwind (reset back to the clean start), THEN pops the
// stashed WIP on top — and the deferred pop runs even though the parent context was
// cancelled (the recovery must survive cancellation).
func TestRelease_CancelledBeforePush_Autostash_UnwindsThenPops(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedAutostashThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		ScriptedOut(""), // unwind: reset --hard startingSHA
		stashPopped(),   // stash pop on TOP of the clean state
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := engine.Release(ctx, newDeps(rec, f), autostashOptions())
	assertAbortNonZero(t, err)

	// The push (the PONR op) was NEVER issued.
	if invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("a cancelled context crossed the PONR; the atomic push must never run; got %v", commandLines(f.Invocations()))
	}
	resetAt := indexOfCmd(f, "git", "reset", "--hard", startingSHA)
	popAt := indexOfCmd(f, "git", "stash", "pop")
	if resetAt == -1 {
		t.Fatalf("cancellation did not run the surgical unwind reset; got %v", commandLines(f.Invocations()))
	}
	if popAt == -1 {
		t.Fatalf("cancellation did not pop the autostashed WIP; got %v", commandLines(f.Invocations()))
	}
	// LOAD-BEARING: the unwind reset must precede the stash pop.
	if resetAt > popAt {
		t.Errorf("stash pop (at %d) ran BEFORE the unwind reset (at %d); the unwind MUST come first", popAt, resetAt)
	}
}
