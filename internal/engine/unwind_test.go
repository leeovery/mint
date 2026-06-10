package engine_test

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// errUnwindReason is the fixed cause threaded into the surgical Unwind so the returned
// abort carries it (the engine owns the non-zero exit; the cause is the original
// failure or the gate-abort sentinel).
var errUnwindReason = errors.New("pre-PONR failure")

// startState builds the captured StartState the surgical unwind resets back to:
// the clean starting HEAD and the target tag, which did NOT exist at capture time.
func startState(head, tag string) engine.StartState {
	return engine.StartState{HEAD: head, Tag: tag, TagExisted: false}
}

// TestUnwind_TwoCommitsAndTag_ResetsBothAndDeletesTag proves the surgical unwind for
// the maximal pre-PONR state — two commits made (hook-artifact + bookkeeping) AND the
// annotated tag created. It deletes the exact tag and resets HEAD to the captured
// starting sha (dropping exactly the two commits), narrating both undone items with
// the engine-authored "repo clean" tail.
func TestUnwind_TwoCommitsAndTag_ResetsBothAndDeletesTag(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 2, TagCreated: true}

	err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason)

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("unwind did not delete the local tag; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("unwind did not reset to the captured starting sha; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundSummary(t, rec), "reset 2 commits and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
}

// TestUnwind_OneBookkeepingCommitAndTag_ResetsToStart proves the one-commit + tag
// state: a single bookkeeping commit and the created tag. The unwind deletes the tag
// and resets to the captured starting sha (NOT HEAD~1), and the summary names the
// single reset commit.
func TestUnwind_OneBookkeepingCommitAndTag_ResetsToStart(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 1, TagCreated: true}

	err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason)

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("unwind did not delete the local tag; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("unwind did not reset to the captured starting sha; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundSummary(t, rec), "reset 1 commit and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
}

// TestUnwind_CommitsButTagNotCreated_DeletesNoTag proves the tag-not-yet-created
// state (e.g. a pre-tag failure or a gate abort after the bookkeeping commit): the
// unwind resets the commit(s) but issues NO `git tag -d`, and the summary names only
// the reset.
func TestUnwind_CommitsButTagNotCreated_DeletesNoTag(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 1, TagCreated: false}

	err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason)

	assertAbortNonZero(t, err)
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("unwind deleted a tag though none was created; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("unwind did not reset to the captured starting sha; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
}

// TestUnwind_TagOnlyNoCommits_DeletesTagOnly proves the tag-created-but-no-commits
// state (zero commits made — the NEITHER graph — yet the tag was created before the
// push was rejected): the unwind deletes the tag and issues NO reset, and the
// summary names only the deleted tag.
func TestUnwind_TagOnlyNoCommits_DeletesTagOnly(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 0, TagCreated: true}

	err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason)

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("unwind did not delete the local tag; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("unwind reset despite zero commits made; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundSummary(t, rec), "deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
}

// TestUnwind_ZeroMutations_NoOp proves the zero-mutation case: no commits and no tag.
// The unwind issues NO git mutation and emits NO Unwound event — there is nothing to
// undo — yet it still returns the non-zero abort (the run failed; only the recovery
// is a no-op).
func TestUnwind_ZeroMutations_NoOp(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 0, TagCreated: false}

	err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason)

	assertAbortNonZero(t, err)
	if len(f.Invocations()) != 0 {
		t.Errorf("zero-mutation unwind issued git commands; got %v", commandLines(f.Invocations()))
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("zero-mutation unwind emitted an Unwound; nothing was undone")
	}
}

// TestUnwind_ResetsToExactCapturedSha proves the reset targets the captured starting
// sha VERBATIM — not a relative HEAD~N — so the result is provably the exact starting
// state regardless of how many commits mint made.
func TestUnwind_ResetsToExactCapturedSha(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 2, TagCreated: false}

	if err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason); err == nil {
		t.Fatalf("Unwind returned nil error, want an abort")
	}

	// The reset names the captured sha exactly, never a HEAD~N relative ref.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Fatalf("unwind did not reset to the exact captured sha %q; got %v", startingSHA, commandLines(f.Invocations()))
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) >= 3 && inv.Args[0] == "reset" && inv.Args[1] == "--hard" {
			if inv.Args[2] != startingSHA {
				t.Errorf("reset target = %q, want the captured sha %q (no HEAD~N)", inv.Args[2], startingSHA)
			}
		}
	}
}

// TestUnwind_ReportsEachUndoneItem proves the engine-authored Unwound Summary names
// each undone item — the reset commit count AND the deleted tag — and carries the
// "repo clean" tail, rendered verbatim by the presenter with no "Reverted:"-style
// prefix (the presenter prefixes the line with "unwound" itself).
func TestUnwind_ReportsEachUndoneItem(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 2, TagCreated: true}

	if err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason); err == nil {
		t.Fatalf("Unwind returned nil error, want an abort")
	}

	summary := unwoundSummary(t, rec)
	if got, want := summary, "reset 2 commits and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	// The summary must not lead with a label the presenter already prefixes.
	if len(summary) >= len("Reverted") && summary[:len("Reverted")] == "Reverted" {
		t.Errorf("Unwound.Summary leads with a redundant prefix: %q", summary)
	}
}

// TestUnwind_NeverPushesOrPublishes proves the surgical unwind is a pre-PONR recovery
// operation ONLY: it issues local-only mutations (tag-delete, reset) and NEVER a
// `git push` or any publish — the operation can never rewrite published history.
func TestUnwind_NeverPushesOrPublishes(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""), // tag -d v0.0.1
		ScriptedOut(""), // reset --hard startsha
	)
	rec := &presentertest.RecordingPresenter{}

	start := startState(startingSHA, "v0.0.1")
	made := engine.MadeState{Commits: 2, TagCreated: true}

	if err := engine.Unwind(t.Context(), newDeps(rec, f), start, made, errUnwindReason); err == nil {
		t.Fatalf("Unwind returned nil error, want an abort")
	}

	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) > 0 && inv.Args[0] == "push" {
			t.Errorf("surgical unwind issued a `git push` (post-PONR op); got %v", commandLines(f.Invocations()))
		}
		if inv.Name == "gh" {
			t.Errorf("surgical unwind invoked `gh` (publish); got %v", commandLines(f.Invocations()))
		}
	}
}
