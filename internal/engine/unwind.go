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
// exclusion answers: an answer is about the repo, not the release, so after the reset
// the answers are re-committed on the starting commit and narrated as kept, and their
// write to a .mint.toml git ignores, never committed, is left in place.
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
	"slices"
	"strings"

	"mint/internal/cmdline"
	"mint/internal/config"
	"mint/internal/fsutil"
	"mint/internal/git"
	"mint/internal/gitrepo"
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
	// answersEdit is the exclusion answers' write to .mint.toml while no commit holds it
	// and git is not found to ignore the file; the unwind reverts it.
	answersEdit *fileEdit
	// keptAnswers is the exclusion answers' commit, counted in Commits; the unwind
	// re-commits it on the starting commit after its reset.
	keptAnswers *answersCommit
	// answersLeft reports that the answers went to a .mint.toml git ignores, a write
	// the unwind leaves in place.
	answersLeft bool
	// written are the files the record stage wrote and no commit yet holds; with no
	// commit to reset they stay as written.
	written []string
	// created are the files the record stage created that neither a commit nor the
	// index holds, which a reset leaves untracked.
	created []string
	// hookRan reports that a preflight or pre_tag hook ran: what it did to the tree or
	// HEAD is unknown to mint, so the close claims nothing a hook could have undone.
	hookRan bool
	// wrote reports that mint began a write of its own: the exclusion answers or the
	// record stage.
	wrote bool
	// preTagStaged reports that mint staged what the pre_tag hook left and the commit
	// of it failed, leaving it staged.
	preTagStaged bool
}

// recordWrite tracks a file the record stage wrote, when it changed, and created, when
// the write made it.
func (m *MadeState) recordWrite(file string, changed, created bool) {
	if changed {
		m.written = append(m.written, file)
	}
	if created {
		m.created = append(m.created, file)
	}
}

// Unwind is the surgical pre-PONR recovery. Given the captured StartState and the
// tracked MadeState it:
//
//   - deletes the exact tag (local) when made.TagCreated — `git tag -d {start.Tag}`;
//   - reverts made.answersEdit, the exclusion answers' uncommitted write to .mint.toml,
//     unstaging it and restoring its content, or removing a file the write created;
//   - resets HEAD to the captured starting commit when made.Commits > 0 (see
//     resetTo), dropping exactly the N commits mint made and their working-tree
//     changes (changelog, version-file projection, hook artifacts) while start.KeepOut
//     keeps its content and staged state;
//   - re-commits the exclusion answers on the starting commit when made.keptAnswers
//     holds their commit and the reset succeeded;
//   - reports each undone item, the kept answers commit and where the repo ended up
//     via the Presenter's Unwound, the engine authoring every part, and tells the
//     run's close what mint did and where HEAD stands (unwoundTo), so the run closes
//     back at its start station saying so, or, where its reset never moved HEAD, at
//     the branch and commit HEAD stands on (strandedAt); with nothing to unwind it
//     tells the close what the run left behind (leftBehind).
//
// With zero mutations (no tag, no commits, no write) it is a no-op: it emits NO
// Unwound and issues NO git command. A reverted write alone emits no Unwound either.
// Either way it returns abort(reason) so the engine owns the non-zero exit; it emits
// NO StageFailed (a stage-failure caller surfaces that first).
//
// Its git mutations flow through the lock-resilient Mutator and are best-effort — a "tag
// not found" or a reset hiccup is not fatal, since the goal is that nothing mint made
// this run but the exclusion answers survives the abort; the abort still carries the
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
	commitsReset, headMoved, resetOK := resetCommitsIfMade(ctx, deps, start, made, restore)
	kept, keptOK := recommitAnswersIfKept(ctx, deps, made.keptAnswers, resetOK, restore)

	if tagDeleted || commitsReset {
		clean := tagOK && editOK && resetOK && keptOK
		event := unwindEvent(start.Tag, made, unwindOutcome{
			reset:       headMoved,
			deleted:     tagDeleted && tagOK,
			answersKept: kept,
			clean:       clean,
		})
		deps.Presenter.Unwound(event)
		// HEAD is back on the starting commit once the reset ran, or, with only a tag to
		// delete, when nothing but mint could have moved it; where the reset never moved
		// it, it stands on mint's commits.
		atStart := resetOK && (commitsReset || !made.hookRan)
		detail := unwoundTo(event, clean, atStart, made)
		if commitsReset && !headMoved {
			strandedAt(ctx, deps, commitsPhrase(made.Commits)+" not reset", detail)
		} else {
			deps.run.Unwound(detail)
		}
	} else {
		deps.run.Left(leftBehind(editOK, made))
	}
	return abort(reason)
}

// The parts of the detail a release that stopped short closes at. They state only
// mint's own actions (what it committed, reset, wrote, staged, created or kept, and
// that it pushed nothing) and where HEAD stands. They never describe how clean the
// tree is, which the Unwound line reports, or what a hook did, which mint cannot see.
const (
	backWhereItStarted = "back where it started"
	manualCleanup      = "manual cleanup required"
	nothingCommitted   = "nothing committed"
	nothingPushed      = "nothing pushed"
	answersLeftInPlace = "the exclusion answers kept in " + config.FileName
	notCommitted       = " written, not committed"
	leftUntracked      = " left untracked"
	preTagStaged       = "pre_tag's changes staged, not committed"
	// noClaim is the detail of a run stopped once mint began a write of its own, no
	// hook having run, with nothing of it left to name. Once a hook has run even
	// "nothing committed" is beyond what mint knows, so the detail is nothingPushed.
	noClaim = nothingCommitted + " · " + nothingPushed
)

// unwoundTo is where an unwind left the repo, as the run's close states it: on the
// kept answers commit when it re-committed them, else back where it started when
// atStart says HEAD is, then that recovery needs a hand when it does, the files the
// record stage created and the reset left untracked, and the answers an ignored
// .mint.toml keeps. An unwind runs only before the push, so it always pushed nothing.
func unwoundTo(event presenter.Unwind, clean, atStart bool, made MadeState) string {
	var parts []string
	switch {
	case event.Kept != "":
		parts = append(parts, event.Kept)
	case atStart:
		parts = append(parts, backWhereItStarted)
	}
	if !clean {
		parts = append(parts, manualCleanup)
	}
	if len(made.created) > 0 {
		parts = append(parts, strings.Join(made.created, " + ")+leftUntracked)
	}
	if made.answersLeft {
		parts = append(parts, answersLeftInPlace)
	}
	return strings.Join(append(parts, nothingPushed), " · ")
}

// strandedAt tells the run's close that a recovery reset never moved HEAD off what mint
// committed, notReset naming it: the close names the branch HEAD stands on and its
// commit, read for display, in place of the start station HEAD never got back to, then
// notReset and detail, the rest of what it states. A HEAD git can't read leaves the
// start station, notReset saying how far HEAD is past it.
func strandedAt(ctx context.Context, deps ReleaseDeps, notReset, detail string) {
	head := gitrepo.ResolveHead(ctx, deps.Runner)
	deps.run.StoppedAt(head.Branch)
	parts := []string{notReset, detail}
	if head.ShortSHA != "" {
		parts = slices.Insert(parts, 0, head.ShortSHA)
	}
	deps.run.Unwound(strings.Join(parts, " · "))
}

// leftBehind is what a release that stopped short with nothing to unwind left
// behind: a failed revert of the answers' write, pre_tag's changes left staged, the
// files the record stage wrote and no commit holds, and the answers an ignored
// .mint.toml keeps, then that it pushed nothing. With none of those it is
// nothingPushed once a hook has run, noClaim once mint began a write, and "" (the
// close's "nothing changed") when neither has.
func leftBehind(editOK bool, made MadeState) string {
	var parts []string
	if !editOK {
		parts = append(parts, manualCleanup)
	}
	if made.preTagStaged {
		parts = append(parts, preTagStaged)
	}
	if len(made.written) > 0 {
		parts = append(parts, strings.Join(made.written, " + ")+notCommitted)
	}
	if made.answersLeft {
		parts = append(parts, answersLeftInPlace)
	}
	switch {
	case len(parts) > 0:
		return strings.Join(append(parts, nothingPushed), " · ")
	case made.hookRan:
		return nothingPushed
	case made.wrote:
		return noClaim
	default:
		return ""
	}
}

// recommitAnswersIfKept commits the exclusion answers again on the commit the reset
// returned to, reporting whether they were re-committed AND whether nothing failed; a
// failure warns with the manual steps that restore them. A failed reset left the
// answers commit on the branch, so it is not re-committed.
func recommitAnswersIfKept(ctx context.Context, deps ReleaseDeps, answers *answersCommit, resetOK bool, restore []manualStep) (kept, ok bool) {
	if answers == nil || !resetOK {
		return false, true
	}
	if manual, output, err := answers.recommit(ctx, deps.Mutator, restore); err != nil {
		warnUnwindIncomplete(deps.Presenter, output, manual...)
		return false, false
	}
	return true, true
}

// answersRestoreStep is the manual step that puts the answers commit back on top of the
// starting commit once a reset has dropped it, or none when made holds no answers
// commit. It names the commit by sha, read while HEAD is still the tip mint built.
// Should that read fail it falls back to ORIG_HEAD, which every reset that moves HEAD
// sets to the tip it moved off, though a later reset, stash or merge moves it again.
func answersRestoreStep(ctx context.Context, deps ReleaseDeps, made MadeState) []manualStep {
	if made.keptAnswers == nil {
		return nil
	}
	below := ""
	if above := made.Commits - made.keptAnswers.position; above > 0 {
		below = fmt.Sprintf("~%d", above)
	}
	commit := "ORIG_HEAD" + below
	if res, err := deps.Mutator.Run(ctx, "git", "rev-parse", "HEAD"+below); err == nil && strings.TrimSpace(res.Stdout) != "" {
		commit = strings.TrimSpace(res.Stdout)
	}
	return []manualStep{{does: "restore the exclusion answers commit on top", commands: []string{"git cherry-pick " + commit}}}
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
	if res, err := deps.Mutator.Mutate(ctx, nil, "git", "tag", "-d", start.Tag); err != nil {
		warnUnwindIncomplete(deps.Presenter, failedGitOutput(res.Stderr, err), manualStep{does: "delete the leftover tag " + start.Tag, commands: []string{"git tag -d " + start.Tag}})
		return true, false
	}
	return true, true
}

// resetCommitsIfMade resets HEAD to the captured starting commit when mint made any
// commits this run, returning whether a reset was issued, whether HEAD moved back, and
// whether the whole reset succeeded. It uses the captured sha (not HEAD~N) so the tree
// returns to the EXACT starting state, discarding mint's release commits and their
// working-tree changes, all but start.KeepOut's. A non-zero exit is no longer discarded: it is surfaced as a
// manual-cleanup Warn naming the steps left so the user is not told the repo is clean
// when mint's commit(s) still live — the recovery DID NOT complete.
func resetCommitsIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState, restore []manualStep) (issued, headMoved, ok bool) {
	if made.Commits <= 0 {
		return false, false, true
	}
	headMoved, manual, output, err := resetTo(ctx, deps.Mutator, resetTarget{HEAD: start.HEAD, KeepOut: start.KeepOut})
	if err != nil {
		// The manual reset drops the answers commit along with mint's other commits.
		warnUnwindIncomplete(deps.Presenter, output, append([]manualStep{manual}, restore...)...)
		return true, headMoved, false
	}
	return true, true, true
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
	warnUnwindIncomplete(deps.Presenter, failedGitOutput(res.Stderr, err), manualStep{
		does:     "unstage " + edit.path,
		commands: []string{"git -C " + cmdline.Quote(edit.root) + " reset -q " + start.HEAD + " -- " + cmdline.Quote(edit.path)},
	})
	return false
}

func restoreEditContent(p presenter.Presenter, edit *fileEdit) bool {
	full := filepath.Join(edit.root, edit.path)
	if !edit.existed {
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			warnUnwindIncomplete(p, err.Error(), manualStep{does: "remove " + full + ", which mint created"})
			return false
		}
		return true
	}
	if err := fsutil.WriteFile(full, edit.content, edit.mode); err != nil {
		warnUnwindIncomplete(p, err.Error(), manualStep{does: "undo mint's edit to " + full})
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
// state, reporting whether HEAD moved back. On failure it returns the manual step that
// finishes the reset and the failing git step's own output; HEAD has moved back where
// the soft reset went through and only the restore after it failed.
//
// With a path to keep, `git reset --hard` cannot be used, as it takes no pathspec: the
// soft reset moves HEAD alone, then restore rewrites the index and work tree everywhere
// but the excluded path, removing tracked paths the starting commit lacks.
func resetTo(ctx context.Context, m *git.Mutator, target resetTarget) (headMoved bool, manual manualStep, output string, err error) {
	sha := target.HEAD
	if target.KeepOut == "" {
		if res, err := m.Mutate(ctx, nil, "git", "reset", "--hard", sha); err != nil {
			return false, manualStep{does: "reset HEAD back to " + sha, commands: []string{"git reset --hard " + sha}}, failedGitOutput(res.Stderr, err), err
		}
		return true, manualStep{}, "", nil
	}

	pathspec := target.KeepOut.Pathspec()
	restoreArgs := append([]string{"restore", "--source=" + sha, "--staged", "--worktree"}, pathspec...)
	restoreLine := "git restore --source=" + sha + " --staged --worktree " + pathspec[0] + " " + cmdline.Quote(pathspec[1])
	keeping := ", leaving " + string(target.KeepOut) + " as it is"
	if res, err := m.Mutate(ctx, nil, "git", "reset", "--soft", sha); err != nil {
		step := manualStep{does: "reset HEAD back to " + sha + keeping, commands: []string{"git reset --soft " + sha, restoreLine}}
		return false, step, failedGitOutput(res.Stderr, err), err
	}
	if res, err := m.Mutate(ctx, nil, "git", restoreArgs...); err != nil {
		step := manualStep{does: "restore the index and work tree to " + sha + keeping, commands: []string{restoreLine}}
		return true, step, failedGitOutput(res.Stderr, err), err
	}
	return true, manualStep{}, "", nil
}

// unwindOutcome is how an unwind ended: whether its reset and its tag delete each
// succeeded, whether the exclusion answers were re-committed, and whether EVERY
// recovery mutation succeeded.
type unwindOutcome struct {
	reset, deleted bool
	answersKept    bool
	clean          bool
}

// unwindEvent builds the Unwound payload: what the unwind undid, the kept answers
// commit when it was re-committed, and where it left the repo. Only a clean outcome
// reads "repo clean", so the narration never calls the repo clean after a failed step.
// Each part is ASCII, so the plain presenter's joined line stays byte-pure, and none
// leads with a "Reverted:"-style prefix — the presenter prefixes the line with
// "unwound", so a prefix here would double-label.
func unwindEvent(tag string, made MadeState, outcome unwindOutcome) presenter.Unwind {
	event := presenter.Unwind{Undone: undone(tag, made.Commits, outcome), Repo: repoState(outcome.clean), Key: stageUnwind}
	if outcome.answersKept {
		event.Kept = "kept the answers commit on the starting commit"
	}
	return event
}

// undone names what the unwind undid: the commits reset and the tag deleted, each only
// where its step succeeded, so a step that failed is never claimed; "" where none did.
func undone(tag string, commits int, outcome unwindOutcome) string {
	switch {
	case outcome.reset && outcome.deleted:
		return fmt.Sprintf("reset %s and deleted tag %s", commitsPhrase(commits), tag)
	case outcome.reset:
		return "reset " + commitsPhrase(commits)
	case outcome.deleted:
		return "deleted tag " + tag
	}
	return ""
}

// repoState is where the unwind left the repo: "repo clean" when all recovery
// succeeded, or "manual cleanup required" when a recovery step failed (the failure
// was already warned with the exact manual step).
func repoState(clean bool) string {
	if clean {
		return "repo clean"
	}
	return manualCleanup
}

// commitsPhrase renders the reset-commit count with correct singular/plural grammar:
// "1 commit" for one, "{n} commits" otherwise.
func commitsPhrase(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}
