package engine

// This file is the engine's --autostash escape hatch — the opt-in WIP stash/restore
// that lets a release run from a DIRTY tree. When enabled, mint stashes the working
// tree (`git stash push --include-untracked`) BEFORE the clean-tree gate, so the gate
// observes a clean tree, then restores the WIP (`git stash pop`) afterward — on
// success AND on abort/failure.
//
// LOAD-BEARING ORDERING: on a pre-PONR abort the surgical unwind (4-2) runs FIRST,
// returning the repo to its exact clean starting state, and the pop is layered ON TOP
// of that. Because the unwind runs inside the spine and the pop runs after the spine
// returns, the unwind-then-pop ordering holds for every abort path by construction —
// popping before the unwind would apply WIP against mint's release commits.
//
// NO-WIP NO-OP: `git stash push` with a clean tree saves nothing and prints "No local
// changes to save"; mint detects that and records that NOTHING was stashed, so no pop
// is owed. CONFLICT SAFETY: a conflicting `git stash pop` exits non-zero and git
// leaves the stash entry intact; mint NEVER runs `git stash drop` on a conflict —
// it leaves the stash in place and WARNS so the user can resolve it manually. A clean
// pop drops the stash entry itself (git does this), so mint issues no drop.
//
// All stash/pop git ops are MUTATIONS and flow through the lock-resilient git.Mutator
// (4-1): a contended `.git` lock is retried, a provably-stale lock cleared.

import (
	"context"
	"strings"

	"mint/internal/presenter"
)

// noChangesToStash is git's stdout marker for `git stash push` when the tree is clean
// (nothing to stash). It is the no-WIP signal: when the push reports it, mint stashed
// nothing and owes no pop.
const noChangesToStash = "No local changes to save"

// autostashPush runs `git stash push --include-untracked` through the lock-resilient
// Mutator BEFORE the clean-tree gate, returning whether anything was actually stashed.
// A clean tree yields "No local changes to save" (stashed=false) — the no-WIP no-op,
// where no pop is later owed. A push error is also treated as nothing-stashed
// (stashed=false): mint never owes a pop it cannot safely make, and the subsequent
// clean-tree gate still guards correctness (a dirty tree that failed to stash simply
// aborts at the gate as it would without --autostash).
func autostashPush(ctx context.Context, deps ReleaseDeps) bool {
	res, err := deps.Mutator.Mutate(ctx, nil, "git", "stash", "push", "--include-untracked")
	if err != nil {
		return false
	}
	return !strings.Contains(res.Stdout, noChangesToStash)
}

// autostashPop restores the WIP with `git stash pop` through the lock-resilient
// Mutator. It is called AFTER the spine returns — on success, on top of the released
// state; on abort, on top of the surgically-unwound clean state — so the unwind-then-
// pop ordering holds for every abort path by construction.
//
// A clean pop succeeds and git drops the stash entry itself, so mint issues no drop.
// A CONFLICTING pop exits non-zero and git leaves the stash intact; mint NEVER runs
// `git stash drop` — it leaves the stash in place and warns (warnPopConflict) so the
// user's WIP is preserved and they can resolve it manually.
func autostashPop(ctx context.Context, deps ReleaseDeps) {
	if _, err := deps.Mutator.Mutate(ctx, nil, "git", "stash", "pop"); err != nil {
		warnPopConflict(deps.Presenter)
	}
}

// warnPopConflict emits the pop-conflict warning: the restoring `git stash pop` could
// not apply cleanly, so the WIP is left in the stash for manual resolution. It rides
// the existing Warn seam (mirroring warnPublishFailed) — a Warn does not set failure
// state, so a release that otherwise succeeded still reports success; an aborted run
// keeps its abort. The message names `git stash` so the user knows exactly where their
// preserved work lives.
func warnPopConflict(p presenter.Presenter) {
	p.Warn(presenter.Warning{
		Label:   "autostash",
		Message: "could not restore stashed changes cleanly — your work is preserved in git stash; resolve manually",
	})
}
