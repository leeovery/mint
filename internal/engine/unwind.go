package engine

// This file is the engine's SURGICAL pre-PONR auto-unwind — the precise recovery
// mint runs when a release fails or is aborted BEFORE the atomic push crosses the
// point of no return. It supersedes the Phase-1/2 best-effort, HEAD-probe reset: that
// reset inferred what to undo from a `git rev-parse HEAD` comparison, which is blunt
// (it cannot tell one commit from two and never names the count it dropped). This
// operation instead drives off what mint KNOWS it created this run — the exact
// captured StartState and the tracked MadeState — so it deletes exactly the tag it
// made (if any) and resets exactly the N commits it made, returning the repo to the
// exact clean starting state and REPORTING each undone item.
//
// LOAD-BEARING INVARIANT: this is a PRE-PONR operation ONLY. It issues local-only
// mutations (tag-delete, reset --hard) and never a `git push` or any publish, so it
// can never rewrite published history. After `git push --atomic` succeeds the tag is
// public and mint MUST NOT call Unwind — that asymmetry is the spine's (a publish
// failure post-PONR is warn-only). The operation refuses to do anything destructive
// to a remote: there is simply no push/publish path through it.
//
// All git mutations flow through the lock-resilient git.Mutator (task 4-1): a
// contended `.git` lock during the unwind is retried, a provably-stale lock cleared.
//
// WIRING NOTE (task 4-3): rewiring the gate-abort and pre-push failure triggers from
// the legacy best-effort reset onto THIS operation is a separate task. This task
// builds and proves the operation against the full range of pre-PONR mutation states
// (0/1/2 commits; tag created or not).

import (
	"context"
	"fmt"

	"mint/internal/presenter"
)

// StartState is the exact clean ref state captured at the START of the mutating
// portion of a run — after preflight has confirmed a clean tree and a resolvable
// HEAD, and BEFORE any commit or tag. It is the unambiguous target the surgical
// unwind resets back to.
type StartState struct {
	// HEAD is the starting commit sha (`git rev-parse HEAD`). The unwind resets to
	// THIS exact sha, not a relative HEAD~N, so the result is provably the exact
	// starting state regardless of how many commits mint made.
	HEAD string
	// Tag is the target tag this run computes (e.g. "v0.0.1"). It is captured here so
	// the unwind deletes exactly the tag mint would have created.
	Tag string
	// TagExisted records whether the target tag already existed at capture time. The
	// spine confirms it did NOT (preflight's tag-free gates), so this is false on the
	// normal path; it is captured to document the starting fact and to never delete a
	// pre-existing tag mint did not create.
	TagExisted bool
}

// MadeState is what mint actually created this run, tracked as the spine steps run
// (not inferred by probing git). It drives the surgical unwind: delete the tag iff
// TagCreated, reset iff Commits > 0, and reset by exactly Commits.
type MadeState struct {
	// Commits is the count of commits mint made before the tag — 0, 1 (a lone
	// hook-artifact or bookkeeping commit), or 2 (hook-artifact + bookkeeping). The
	// unwind reports this count and resets to StartState.HEAD when it is non-zero.
	Commits int
	// TagCreated reports whether mint created the annotated tag this run. When true the
	// unwind deletes it (local only — pre-PONR never pushed it).
	TagCreated bool
}

// Unwind is the surgical pre-PONR recovery. Given the captured StartState and the
// tracked MadeState it:
//
//   - deletes the exact tag (local) when made.TagCreated — `git tag -d {start.Tag}`;
//   - resets HEAD to the captured starting commit when made.Commits > 0 —
//     `git reset --hard {start.HEAD}` — dropping exactly the N commits mint made and
//     their working-tree changes (changelog, version-file projection, hook artifacts);
//   - reports each undone item via the Presenter's Unwound(Unwind{Summary}), the
//     engine authoring the COMPLETE summary including its "; repo clean" tail.
//
// With zero mutations (no tag, no commits) it is a no-op: it emits NO Unwound and
// issues NO git command. Either way it returns abort(reason) so the engine owns the
// non-zero exit; it emits NO StageFailed (a stage-failure caller surfaces that first).
//
// Both mutations flow through the lock-resilient Mutator and are best-effort — a "tag
// not found" or a reset hiccup is not fatal, since the goal is that nothing mint made
// this run survives the abort; the abort still carries the original reason. The
// operation NEVER pushes or publishes: it is pre-PONR recovery only.
func Unwind(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState, reason error) error {
	tagDeleted := deleteTagIfMade(ctx, deps, start, made)
	commitsReset := resetCommitsIfMade(ctx, deps, start, made)

	if tagDeleted || commitsReset {
		deps.Presenter.Unwound(unwindEvent(start.Tag, made))
	}
	return abort(reason)
}

// deleteTagIfMade deletes the local tag mint created this run, returning whether a
// delete was issued. Pre-PONR the tag was never pushed, so `git tag -d {tag}` (local
// only) fully removes it. The delete is a MUTATION, so it flows through the
// lock-resilient Mutator; a "tag not found" non-zero is not fatal.
func deleteTagIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState) bool {
	if !made.TagCreated {
		return false
	}
	_, _ = deps.Mutator.Mutate(ctx, nil, "git", "tag", "-d", start.Tag)
	return true
}

// resetCommitsIfMade resets HEAD to the captured starting commit when mint made any
// commits this run, returning whether a reset was issued. It uses the captured sha
// (not HEAD~N) so the tree returns to the EXACT starting state, discarding mint's
// release commits and their working-tree changes. The reset is a MUTATION, so it
// flows through the lock-resilient Mutator; a reset hiccup is best-effort.
func resetCommitsIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState) bool {
	if made.Commits <= 0 {
		return false
	}
	_, _ = deps.Mutator.Mutate(ctx, nil, "git", "reset", "--hard", start.HEAD)
	return true
}

// unwindEvent builds the Unwound payload whose engine-authored Summary names each
// undone item.
func unwindEvent(tag string, made MadeState) presenter.Unwind {
	return presenter.Unwind{Summary: surgicalSummary(tag, made)}
}

// surgicalSummary authors the engine-owned Unwound Summary describing what the unwind
// undid — the count of commits reset and the deleted tag — INCLUDING the trailing "; repo
// clean" tail. It is ASCII and semicolon-joined (matching the existing "…; repo clean"
// style) so the plain presenter stays byte-pure; the presenter renders it VERBATIM and
// synthesises no part of it. It never leads with a "Reverted:"-style prefix — the
// presenter prefixes the line with "unwound", so a prefix here would double-label.
//
// It is only ever called when something was undone (a commit reset and/or a tag
// deleted), so the zero/zero case is unreachable here; the operation no-ops before
// reaching it.
func surgicalSummary(tag string, made MadeState) string {
	const tail = "; repo clean"
	reset := made.Commits > 0
	deleted := made.TagCreated
	switch {
	case reset && deleted:
		return fmt.Sprintf("reset %s and deleted tag %s%s", commitsPhrase(made.Commits), tag, tail)
	case reset:
		return "reset " + commitsPhrase(made.Commits) + tail
	default:
		return "deleted tag " + tag + tail
	}
}

// commitsPhrase renders the reset-commit count with correct singular/plural grammar:
// "1 commit" for one, "{n} commits" otherwise.
func commitsPhrase(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}
