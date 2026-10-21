package engine

// This file is the engine's SURGICAL pre-PONR auto-unwind — the precise recovery
// mint runs when a release fails or is aborted BEFORE the atomic push crosses the
// point of no return. It supersedes the Phase-1/2 best-effort, HEAD-probe reset: that
// reset inferred what to undo from a `git rev-parse HEAD` comparison, which is blunt
// (it cannot tell one commit from two and never names the count it dropped). This
// operation instead drives off what mint KNOWS it created this run — the exact
// captured StartState and the tracked MadeState — so it deletes exactly the tag it
// made (if any) and resets exactly the N commits it made, returning the repo to the
// exact clean starting state and REPORTING each undone item. The one exception is the
// exclusion answers commit: an answer is about the repo, not the release, so after the
// reset the answers are re-committed on the starting commit and narrated as kept.
//
// LOAD-BEARING INVARIANT: this is a PRE-PONR operation ONLY. It issues local-only
// mutations (tag-delete, reset, the answers re-commit) and never a `git push` or any
// publish, so it can never rewrite published history. After `git push --atomic`
// succeeds the tag is public and mint MUST NOT call Unwind — that asymmetry is the
// spine's (a publish failure post-PONR is warn-only). The operation refuses to do
// anything destructive to a remote: there is simply no push/publish path through it.
//
// All git mutations flow through the lock-resilient git.Mutator (task 4-1): a
// contended `.git` lock during the unwind is retried, a provably-stale lock cleared.
//
// WIRING: every review-gate abort and every pre-push failure trigger route through this
// operation from the spine (see surfaceAndUnwind / the review-gate path in release.go),
// passing the captured StartState and tracked MadeState. The gate-abort and a
// pre-push git failure are wired identically, so they produce an identical clean
// state and an identical Unwound summary.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mint/internal/fsutil"
	"mint/internal/git"
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
	// KeepOut is the path the reset leaves as it was, its content and staged state
	// untouched.
	KeepOut git.Exclusion
}

// MadeState is what mint actually created this run, tracked as the spine steps run
// (not inferred by probing git). It drives the surgical unwind: delete the tag iff
// TagCreated, reset iff Commits > 0, and reset by exactly Commits.
type MadeState struct {
	// Commits is the count of commits mint made before the tag, 0 to 3: the
	// hook-artifact, answers and bookkeeping commits, each optional. The unwind reports
	// this count and resets to StartState.HEAD when it is non-zero.
	Commits int
	// TagCreated reports whether mint created the annotated tag this run. When true the
	// unwind deletes it (local only — pre-PONR never pushed it).
	TagCreated bool
	// answersEdit is the exclusion answers' write to .mint.toml when no commit holds
	// it; the unwind reverts it.
	answersEdit *fileEdit
	// keptAnswers is the exclusion answers' commit, counted in Commits; the unwind
	// re-commits it on the starting commit after its reset.
	keptAnswers *answersCommit
}

// Unwind is the surgical pre-PONR recovery. Given the captured StartState and the
// tracked MadeState it:
//
//   - deletes the exact tag (local) when made.TagCreated — `git tag -d {start.Tag}`;
//   - reverts the exclusion answers' uncommitted write to .mint.toml, unstaging it and
//     restoring its content, or removing a file the write created;
//   - resets HEAD to the captured starting commit when made.Commits > 0 (see
//     resetTo), dropping exactly the N commits mint made and their working-tree
//     changes (changelog, version-file projection, hook artifacts) while start.KeepOut
//     keeps its content and staged state;
//   - re-commits the exclusion answers on the starting commit when made.keptAnswers
//     holds their commit and the reset succeeded;
//   - reports each undone item and the kept answers commit via the Presenter's
//     Unwound(Unwind{Summary}), the engine authoring the COMPLETE summary including
//     its "; repo clean" tail.
//
// With zero mutations (no tag, no commits, no write) it is a no-op: it emits NO
// Unwound and issues NO git command. A reverted write alone emits no Unwound either.
// Either way it returns abort(reason) so the engine owns the non-zero exit; it emits
// NO StageFailed (a stage-failure caller surfaces that first).
//
// Its git mutations flow through the lock-resilient Mutator and are best-effort — a "tag
// not found" or a reset hiccup is not fatal, since the goal is that nothing mint made
// this run but the answers commit survives the abort; the abort still carries the
// original reason. The operation NEVER pushes or publishes: it is pre-PONR recovery
// only.
func Unwind(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState, reason error) error {
	// The unwind is RECOVERY work that must run to completion even when the triggering
	// failure was the parent context's own cancellation (a SIGINT/SIGTERM pre-PONR
	// abort). Detaching from cancellation with context.WithoutCancel — while keeping any
	// values — means the local-only reset/tag-delete still execute and leave the repo
	// clean; without it the unwind's git commands would be killed by the same signal that
	// triggered the abort, stranding mint's commit(s) and tag.
	ctx = context.WithoutCancel(ctx)

	restore := answersRestoreStep(ctx, deps, made)
	tagDeleted, tagOK := deleteTagIfMade(ctx, deps, start, made)
	// Reverted before the reset, which then returns a tracked file to the starting
	// commit whatever the write found there.
	editOK := revertEditIfMade(ctx, deps, start, made.answersEdit)
	commitsReset, resetOK := resetCommitsIfMade(ctx, deps, start, made, restore)
	kept, keptOK := recommitAnswersIfKept(ctx, deps, made.keptAnswers, resetOK, restore)

	if tagDeleted || commitsReset {
		deps.Presenter.Unwound(unwindEvent(start.Tag, made, unwindOutcome{answersKept: kept, clean: tagOK && editOK && resetOK && keptOK}))
	}
	return abort(reason)
}

// recommitAnswersIfKept commits the exclusion answers again on the commit the reset
// returned to, reporting whether they were re-committed AND whether nothing failed; a
// failure warns with the manual step that restores them. A failed reset left the
// answers commit on the branch, so it is not re-committed.
func recommitAnswersIfKept(ctx context.Context, deps ReleaseDeps, answers *answersCommit, resetOK bool, restore string) (kept, ok bool) {
	if answers == nil || !resetOK {
		return false, true
	}
	if manual, output, err := answers.recommit(ctx, deps.Mutator, restore); err != nil {
		warnUnwindIncomplete(deps.Presenter, manual, output)
		return false, false
	}
	return true, true
}

// answersRestoreStep is the manual step that puts the answers commit back on top of the
// starting commit once a reset has dropped it, or "" when made holds no answers commit.
// It names the commit by sha, read while HEAD is still the tip mint built. Should that
// read fail it falls back to ORIG_HEAD, which every reset that moves HEAD sets to the
// tip it moved off, though a later reset, stash or merge moves it again.
func answersRestoreStep(ctx context.Context, deps ReleaseDeps, made MadeState) string {
	if made.keptAnswers == nil {
		return ""
	}
	below := ""
	if above := made.Commits - made.keptAnswers.position; above > 0 {
		below = fmt.Sprintf("~%d", above)
	}
	commit := "ORIG_HEAD" + below
	if res, err := deps.Mutator.Run(ctx, "git", "rev-parse", "HEAD"+below); err == nil && strings.TrimSpace(res.Stdout) != "" {
		commit = strings.TrimSpace(res.Stdout)
	}
	return "restore the exclusion answers commit on top (`git cherry-pick " + commit + "`)"
}

// deleteTagIfMade deletes the local tag mint created this run, returning whether a
// delete was issued AND whether it succeeded. Pre-PONR the tag was never pushed, so
// `git tag -d {tag}` (local only) fully removes it. The delete is a MUTATION, so it
// flows through the lock-resilient Mutator. A non-zero exit is no longer discarded: it
// is surfaced as a manual-cleanup Warn so the user is not told the repo is clean when
// the tag mint created still lives — the recovery DID NOT complete. The bool pair lets
// the caller both narrate the issued item and gate the "repo clean" tail.
func deleteTagIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState) (issued, ok bool) {
	if !made.TagCreated {
		return false, true
	}
	if _, err := deps.Mutator.Mutate(ctx, nil, "git", "tag", "-d", start.Tag); err != nil {
		warnUnwindIncomplete(deps.Presenter, "delete the leftover tag "+start.Tag+" (`git tag -d "+start.Tag+"`)", err.Error())
		return true, false
	}
	return true, true
}

// resetCommitsIfMade resets HEAD to the captured starting commit when mint made any
// commits this run, returning whether a reset was issued AND whether it succeeded. It
// uses the captured sha (not HEAD~N) so the tree returns to the EXACT starting state,
// discarding mint's release commits and their working-tree changes, all but
// start.KeepOut's. A non-zero exit is no longer discarded: it is surfaced as a
// manual-cleanup Warn naming the steps left so the user is not told the repo is clean
// when mint's commit(s) still live — the recovery DID NOT complete.
func resetCommitsIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState, restore string) (issued, ok bool) {
	if made.Commits <= 0 {
		return false, true
	}
	if manual, err := resetTo(ctx, deps.Mutator, resetTarget{HEAD: start.HEAD, KeepOut: start.KeepOut}); err != nil {
		// The manual reset drops the answers commit along with mint's other commits.
		if restore != "" {
			manual += ", then " + restore
		}
		warnUnwindIncomplete(deps.Presenter, manual, err.Error())
		return true, false
	}
	return true, true
}

// fileEdit is a write mint made to path under root, with what path held before it.
type fileEdit struct {
	root, path string
	existed    bool
	content    []byte
	mode       fs.FileMode
}

// beforeEdit records path under root as it is now, ahead of a write to it.
func beforeEdit(root, path string) (*fileEdit, error) {
	edit := &fileEdit{root: root, path: path}
	full := filepath.Join(root, path)
	info, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return edit, nil
	}
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	edit.existed, edit.content, edit.mode = true, content, info.Mode().Perm()
	return edit, nil
}

// revertEditIfMade returns edit's path to its index entry at the starting commit and
// its content before the write, reporting whether both succeeded; each is attempted
// whatever the other's outcome. A nil edit is a no-op.
func revertEditIfMade(ctx context.Context, deps ReleaseDeps, start StartState, edit *fileEdit) bool {
	if edit == nil {
		return true
	}
	unstaged := unstageEdit(ctx, deps, start, edit)
	restored := restoreEditContent(deps.Presenter, edit)
	return unstaged && restored
}

func unstageEdit(ctx context.Context, deps ReleaseDeps, start StartState, edit *fileEdit) bool {
	res, err := deps.Mutator.Mutate(ctx, nil, "git", "-C", edit.root, "reset", "-q", start.HEAD, "--", edit.path)
	if err == nil {
		return true
	}
	output := res.Stderr
	if output == "" {
		output = err.Error()
	}
	warnUnwindIncomplete(deps.Presenter, "unstage "+edit.path+" (`git -C "+edit.root+" reset -q "+start.HEAD+" -- "+edit.path+"`)", output)
	return false
}

func restoreEditContent(p presenter.Presenter, edit *fileEdit) bool {
	full := filepath.Join(edit.root, edit.path)
	if !edit.existed {
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			warnUnwindIncomplete(p, "remove "+full+", which mint created", err.Error())
			return false
		}
		return true
	}
	if err := fsutil.WriteFile(full, edit.content, edit.mode); err != nil {
		warnUnwindIncomplete(p, "undo mint's edit to "+full, err.Error())
		return false
	}
	return true
}

// resetTarget is where a recovery reset returns the repo: HEAD at the starting commit,
// with KeepOut left as it was.
type resetTarget struct {
	HEAD    string
	KeepOut git.Exclusion
}

// resetTo returns HEAD, the index and the work tree to target.HEAD as
// `git reset --hard` does, except that target.KeepOut keeps its content and staged
// state. On failure it returns the manual step that finishes the reset.
//
// With a path to keep, `git reset --hard` cannot be used, as it takes no pathspec: the
// soft reset moves HEAD alone, then restore rewrites the index and work tree everywhere
// but the excluded path, removing tracked paths the starting commit lacks.
func resetTo(ctx context.Context, m *git.Mutator, target resetTarget) (manual string, err error) {
	sha := target.HEAD
	if target.KeepOut == "" {
		if _, err := m.Mutate(ctx, nil, "git", "reset", "--hard", sha); err != nil {
			return "reset HEAD back to " + sha + " (`git reset --hard " + sha + "`)", err
		}
		return "", nil
	}

	pathspec := target.KeepOut.Pathspec()
	restoreArgs := append([]string{"restore", "--source=" + sha, "--staged", "--worktree"}, pathspec...)
	restoreLine := "`git restore --source=" + sha + " --staged --worktree " + pathspec[0] + " '" + pathspec[1] + "'`"
	keeping := ", leaving " + string(target.KeepOut) + " as it is ("
	if _, err := m.Mutate(ctx, nil, "git", "reset", "--soft", sha); err != nil {
		return "reset HEAD back to " + sha + keeping + "`git reset --soft " + sha + "` then " + restoreLine + ")", err
	}
	if _, err := m.Mutate(ctx, nil, "git", restoreArgs...); err != nil {
		return "restore the index and work tree to " + sha + keeping + restoreLine + ")", err
	}
	return "", nil
}

// warnUnwindIncomplete emits the manual-cleanup notice when a recovery step failed
// mid-unwind: the repo is NOT clean, so the user must finish the recovery by hand.
// action names the exact manual step; the failing step's error rides the Warn's Output
// verbatim, mirroring the engine's other captured-output warns (warnPublishFailed et al).
func warnUnwindIncomplete(p presenter.Presenter, action, output string) {
	p.Warn(presenter.Warning{
		Label:   "unwind incomplete",
		Message: "automatic recovery failed; manually " + action,
		Output:  output,
	})
}

// unwindOutcome is how an unwind ended: whether the exclusion answers were
// re-committed, and whether EVERY recovery mutation succeeded.
type unwindOutcome struct {
	answersKept bool
	clean       bool
}

// unwindEvent builds the Unwound payload whose engine-authored Summary names each
// undone item and the kept answers commit. Only a clean outcome carries the
// "; repo clean" tail, so the narration never calls the repo clean after a failed step.
func unwindEvent(tag string, made MadeState, outcome unwindOutcome) presenter.Unwind {
	return presenter.Unwind{Summary: surgicalSummary(tag, made, outcome)}
}

// surgicalSummary authors the engine-owned Unwound Summary describing what the unwind
// ATTEMPTED to undo — the count of commits reset and the deleted tag — then the kept
// answers commit when it was re-committed, and its trailing status tail. When every
// recovery mutation succeeded the tail is "; repo clean"; when one FAILED the tail
// becomes "; manual cleanup required" so the summary never falsely claims the repo is
// clean. It is ASCII and semicolon-joined (matching the
// existing "…; repo clean" style) so the plain presenter stays byte-pure; the presenter
// renders it VERBATIM and synthesises no part of it. It never leads with a
// "Reverted:"-style prefix — the presenter prefixes the line with "unwound", so a prefix
// here would double-label.
//
// It is only ever called when something was undone (a commit reset and/or a tag
// deleted), so the zero/zero case is unreachable here; the operation no-ops before
// reaching it.
func surgicalSummary(tag string, made MadeState, outcome unwindOutcome) string {
	tail := summaryTail(outcome.clean)
	if outcome.answersKept {
		tail = "; kept the answers commit on the starting commit" + tail
	}
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

// summaryTail selects the Unwound summary's trailing status: the "; repo clean"
// confirmation when all recovery succeeded, or the "; manual cleanup required" notice
// when a recovery step failed (the failure was already warned with the exact manual
// step).
func summaryTail(clean bool) string {
	if clean {
		return "; repo clean"
	}
	return "; manual cleanup required"
}

// commitsPhrase renders the reset-commit count with correct singular/plural grammar:
// "1 commit" for one, "{n} commits" otherwise.
func commitsPhrase(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}
