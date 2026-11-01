package engine

// This file is the engine's SURGICAL pre-PONR auto-unwind — the precise recovery
// mint runs when a release fails or is aborted BEFORE the atomic push crosses the
// point of no return. It supersedes the Phase-1/2 best-effort, HEAD-probe reset: that
// reset inferred what to undo from a `git rev-parse HEAD` comparison, which is blunt
// (it cannot tell one commit from two and never names the count it dropped). This
// operation instead drives off what mint KNOWS it created this run — the exact
// captured StartState and the tracked MadeState — so it deletes exactly the tag it
// made (if any), puts back exactly the files it wrote or staged that no commit holds and
// resets exactly the N commits it made, undoing exactly what the run changed, keeping
// every change the user made meanwhile, and REPORTING each undone item. Where a change
// of the user's touches a path mint's commits changed, the reset refuses and the unwind
// says so with the manual step, losing nothing. The one exception is the exclusion
// answers: an answer is about the repo, not the release, so after the reset the
// answers are re-committed on the starting commit and narrated as kept, and their
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
	"mint/internal/record"
)

// StartState is the exact clean ref state captured at the START of the mutating
// portion of a run — after preflight has confirmed a clean tree and a resolvable
// HEAD, and BEFORE any commit or tag. It is the unambiguous target the surgical
// unwind resets back to.
type StartState struct {
	// HEAD is the starting commit sha (`git rev-parse HEAD`). The unwind resets to
	// THIS exact sha, not a relative HEAD~N, so HEAD provably returns to the starting
	// commit regardless of how many commits mint made.
	HEAD string
	// Tag is the target tag this run computes (e.g. "v0.0.1"). It is captured here so
	// the unwind deletes exactly the tag mint would have created.
	Tag string
	// TagExisted records whether the target tag already existed at capture time. The
	// spine confirms it did NOT (preflight's tag-free gates), so this is false on the
	// normal path; it is captured to document the starting fact and to never delete a
	// pre-existing tag mint did not create.
	TagExisted bool
	// KeepOut is the path the unwind never reads as a change of mint's.
	KeepOut git.Exclusion
	// Root is the repo's root, under which the unwind puts back the files mint wrote.
	Root string
}

// MadeState is what mint actually created this run, tracked as the spine steps run
// (not inferred by probing git). It drives the surgical unwind: delete the tag iff
// TagCreated, put back exactly the files mint wrote or staged that no commit holds,
// and reset to the starting commit iff mint committed.
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
	// written are the files the record stage wrote and no commit yet holds.
	written []string
	// created are the files among written that the record stage's write created.
	created []string
	// recordStaged reports that the record stage staged written and the commit of
	// them failed, so git holds the files it created in the index.
	recordStaged bool
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

// restored are the files the unwind returns to HEAD's content: those the record stage
// wrote that were there before it.
func (m MadeState) restored() []string {
	return slices.DeleteFunc(slices.Clone(m.written), func(file string) bool {
		return slices.Contains(m.created, file)
	})
}

// recordFiles are the record stage's files to put back: tracked, those git holds in
// HEAD or the index, and untracked, those it created and never staged.
func (m MadeState) recordFiles() (tracked, untracked []string) {
	if m.recordStaged {
		return m.written, nil
	}
	return m.restored(), m.created
}

// restores reports whether the tree or the index holds what mint wrote or staged and
// no commit holds, which the unwind puts back.
func (m MadeState) restores() bool {
	return m.preTagStaged || len(m.written) > 0
}

// writesLeft is what of m's uncommitted writes is still in the tree: the record
// stage's files among paths, and pre_tag's staged changes where preTag says so.
func (m MadeState) writesLeft(paths []string, preTag bool) MadeState {
	among := func(files []string) []string {
		return slices.DeleteFunc(slices.Clone(files), func(file string) bool { return !slices.Contains(paths, file) })
	}
	return MadeState{written: among(m.written), created: among(m.created), preTagStaged: preTag}
}

// putBackOf is what of m's uncommitted writes the unwind put back, left being what
// is still in the tree.
func (m MadeState) putBackOf(left MadeState) MadeState {
	notLeft := func(files, leftFiles []string) []string {
		return slices.DeleteFunc(slices.Clone(files), func(file string) bool { return slices.Contains(leftFiles, file) })
	}
	return MadeState{
		written:      notLeft(m.written, left.written),
		created:      notLeft(m.created, left.created),
		preTagStaged: m.preTagStaged && !left.preTagStaged,
	}
}

// Unwind is the surgical pre-PONR recovery. Given the captured StartState and the
// tracked MadeState it:
//
//   - deletes the exact tag (local) when made.TagCreated — `git tag -d {start.Tag}`;
//   - reverts made.answersEdit, the exclusion answers' uncommitted write to .mint.toml,
//     unstaging it and restoring its content, or removing a file the write created;
//   - puts back exactly what mint wrote or staged that no commit holds (see
//     restoreWritesIfMade): the record stage's files and pre_tag's changes it staged
//     return to HEAD's content, in the index and the work tree, and the files it
//     created are removed, while every other path stays as it is and HEAD stays where
//     it stands;
//   - resets HEAD to the captured starting commit when made.Commits > 0 (see
//     resetTo), taking back exactly what the N commits mint made changed and keeping
//     every other change in the tree, a notes file's included, or, where the user
//     changed a path those commits changed, refusing and changing nothing;
//   - re-commits the exclusion answers on the starting commit when made.keptAnswers
//     holds their commit and the reset succeeded, then puts the index back as it
//     held the user's changes before the reset (stagedIndex);
//   - reports each undone item, the kept answers commit and where the repo ended up
//     via the Presenter's Unwound, the engine authoring every part, and tells the
//     run's close what mint did and where HEAD stands (unwoundTo), so the run closes
//     back at its start station saying so, or, where its reset never moved HEAD, at
//     the branch and commit HEAD stands on (strandedAt), or, where its
//     put-back left something, saying what mint left unrestored (notRestored); with
//     nothing to unwind it tells the close what the run left behind (leftBehind).
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
	// Reverted before the reset, which would keep the write as a change of the user's.
	editOK := revertEditIfMade(ctx, deps, start, made.answersEdit)
	// Put back before the reset, which moves HEAD and HEAD's content with it.
	restored, left := restoreWritesIfMade(ctx, deps, start, made)
	restoreOK := !left.restores()
	reset, resetOK, staged := resetIfMade(ctx, deps, start, made, restore)
	kept, keptOK := recommitAnswersIfKept(ctx, deps, made.keptAnswers, resetOK, restore)
	restagedOK := restageIfReset(ctx, deps, staged)

	if tagDeleted || restored || reset {
		clean := tagOK && editOK && restoreOK && resetOK && keptOK && restagedOK
		event := unwindEvent(start.Tag, made, unwindOutcome{
			reset:       resetOK && reset,
			put:         made.putBackOf(left),
			deleted:     tagDeleted && tagOK,
			answersKept: kept,
			clean:       clean,
		})
		deps.Presenter.Unwound(event)
		// HEAD is back on the starting commit once the reset ran, or, with no commit to
		// reset, when nothing but mint could have moved it; where the reset never moved
		// it, it stands on mint's commits.
		atStart := resetOK && (reset || !made.hookRan)
		switch {
		case reset && !resetOK:
			strandedAt(ctx, deps, commitsPhrase(made.Commits)+" not reset", unwoundTo(event, clean, atStart))
		case !restoreOK:
			deps.run.Unwound(notRestored(left))
		default:
			deps.run.Unwound(unwoundTo(event, clean, atStart))
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
	answersLeftInPlace = "your exclusion answers kept in " + config.FileName
	// answersKept is what the unwind says it kept of the exclusion answers: plain's
	// line where it re-committed them, the rail's row too where git ignores the
	// .mint.toml that holds them.
	answersKept = "kept your exclusion answers in " + config.FileName
	// answersRecommitted is the one commit past the start the close names where the
	// unwind re-committed the answers.
	answersRecommitted = "+ 1 commit (the " + config.FileName + " answers)"
	notCommitted       = " written, not committed"
	preTagStaged       = "pre_tag's changes staged, not committed"
	// noClaim is the detail of a run stopped once mint began a write of its own, no
	// hook having run, with nothing of it left to name. Once a hook has run even
	// "nothing committed" is beyond what mint knows, so the detail is nothingPushed.
	noClaim = nothingCommitted + " · " + nothingPushed
)

// unwoundTo is where an unwind left the repo, as the run's close states it: one
// commit past the start, the answers', when it re-committed them, else back where it
// started when atStart says HEAD is, then that recovery needs a hand when it does.
// That it kept the answers is the unwind's row's to say, not the close's. An unwind
// runs only before the push, so it always pushed nothing.
func unwoundTo(event presenter.Unwind, clean, atStart bool) string {
	var parts []string
	switch {
	case event.Kept != "":
		parts = append(parts, answersRecommitted)
	case atStart:
		parts = append(parts, backWhereItStarted)
	}
	if !clean {
		parts = append(parts, manualCleanup)
	}
	return strings.Join(append(parts, nothingPushed), " · ")
}

// notRestored is where an unwind left the repo when its put-back left part of what
// mint wrote or staged: that recovery needs a hand, then left, what is still there
// that no commit holds, then that it pushed nothing.
func notRestored(left MadeState) string {
	return strings.Join(slices.Concat([]string{manualCleanup}, uncommitted(left), []string{nothingPushed}), " · ")
}

// uncommitted is what mint wrote or staged that no commit holds, as a close names it:
// pre_tag's changes it staged, then the files the record stage wrote.
func uncommitted(made MadeState) []string {
	var parts []string
	if made.preTagStaged {
		parts = append(parts, preTagStaged)
	}
	if len(made.written) > 0 {
		parts = append(parts, strings.Join(made.written, " + ")+notCommitted)
	}
	return parts
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
// behind: a failed revert of the answers' write and the answers an ignored .mint.toml
// keeps, then that it pushed nothing. With none of those it is nothingPushed once a
// hook has run, noClaim once mint began a write, and "" (the close's "nothing
// changed") when neither has.
func leftBehind(editOK bool, made MadeState) string {
	var parts []string
	if !editOK {
		parts = append(parts, manualCleanup)
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
	if manual, output, err := answers.recommit(ctx, deps.Mutator, deps.Presenter, restore); err != nil {
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
	return []manualStep{{does: "put your exclusion answers back on top", commands: []string{"git cherry-pick " + commit}}}
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

// resetIfMade resets HEAD to the captured starting commit when mint made any commits
// this run (see resetTo), returning whether a reset was issued, whether it moved HEAD
// back, and what the index held of the user's, for restageIfReset to put back. It
// uses the captured sha (not HEAD~N), so exactly mint's commits are taken back, and
// every other change in the tree is kept. A failure is surfaced as a manual-cleanup
// Warn naming the steps left so the user is not told the repo is clean when mint's
// commit(s) still live — the recovery DID NOT complete.
func resetIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState, restore []manualStep) (issued, ok bool, staged stagedIndex) {
	if made.Commits <= 0 {
		return false, true, stagedIndex{}
	}
	staged, manual, output, err := resetTo(ctx, deps.Mutator, resetTarget{HEAD: start.HEAD, Root: start.Root}, "")
	if err != nil {
		// The manual reset drops the answers commit along with mint's other commits.
		warnUnwindIncomplete(deps.Presenter, output, append(manual, restore...)...)
		return true, false, stagedIndex{}
	}
	return true, true, staged
}

// restageIfReset puts the index back as it held the user's changes before the reset
// (stagedIndex.steps), each step attempted whatever another's outcome, reporting
// whether all succeeded; each that fails warns with the manual step that does it.
func restageIfReset(ctx context.Context, deps ReleaseDeps, index stagedIndex) bool {
	ok := true
	for _, step := range index.steps() {
		if res, err := deps.Mutator.Mutate(ctx, record.PathspecInput(step.paths), "git", append(step.args, pathspecsOnStdin...)...); err != nil {
			warnUnwindIncomplete(deps.Presenter, failedGitOutput(res.Stderr, err), step.manual)
			ok = false
		}
	}
	return ok
}

// restoreWritesIfMade puts back what mint wrote or staged that no commit holds, and
// touches no other path, returning whether it ran and what of it is left: the record
// stage's files, and the paths of pre_tag's changes mint staged, which it reads from
// the index, leaving start.KeepOut out. A file of the record stage's git holds in
// neither HEAD nor the index, one mint wrote over that git ignores, is left and
// warned, and never stops the rest being put back. HEAD stays where it stands, so a
// commit the pre_tag hook made itself stays too.
func restoreWritesIfMade(ctx context.Context, deps ReleaseDeps, start StartState, made MadeState) (issued bool, left MadeState) {
	if !made.restores() {
		return false, MadeState{}
	}
	tracked, untracked := made.recordFiles()
	unrestored := putBack(ctx, deps, start.Root, tracked, untracked, made.recordStaged)
	preTagLeft := made.preTagStaged && !restorePreTag(ctx, deps, start)
	return true, made.writesLeft(unrestored, preTagLeft)
}

// restorePreTag returns the paths of pre_tag's changes mint staged, read from the
// index with start.KeepOut left out, to HEAD, reporting whether it did.
func restorePreTag(ctx context.Context, deps ReleaseDeps, start StartState) bool {
	staged, output, err := stagedPaths(ctx, deps.Mutator, start.Root, start.KeepOut)
	if err != nil {
		warnUnwindIncomplete(deps.Presenter, output, manualStep{does: "unstage and remove the pre_tag changes mint staged"})
		return false
	}
	return restoreToHEAD(ctx, deps, start.Root, staged)
}

// putBack returns tracked, files mint wrote over under root, to HEAD's content, and
// removes untracked, files it created that git never held, returning those it could
// not put back. Where mint never staged the files, those git holds in neither HEAD
// nor the index are left (knownToGit) and never stop the rest.
func putBack(ctx context.Context, deps ReleaseDeps, root string, tracked, untracked []string, staged bool) (left []string) {
	if !staged {
		tracked, left = knownToGit(ctx, deps, root, tracked)
	}
	if !restoreToHEAD(ctx, deps, root, tracked) {
		left = append(left, tracked...)
	}
	return append(left, removeCreated(ctx, deps, root, untracked)...)
}

// knownToGit splits paths, files mint wrote over in the repo at root, into those git
// can restore, held in HEAD or the index, and the rest, which it warns are left as mint
// wrote them, git holding no copy to put back. A failed read counts them all known,
// for the restore to try.
func knownToGit(ctx context.Context, deps ReleaseDeps, root string, paths []string) (known, unknown []string) {
	if len(paths) == 0 {
		return nil, nil
	}
	args := append([]string{"-C", root, "ls-files", "-z", "--cached", "--with-tree=HEAD", "--"}, literalPathspecs(paths)...)
	res, err := deps.Mutator.Run(ctx, "git", args...)
	if err != nil {
		return paths, nil
	}
	listed := strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 })
	for _, path := range paths {
		if slices.Contains(listed, path) {
			known = append(known, path)
		} else {
			unknown = append(unknown, path)
		}
	}
	if len(unknown) > 0 {
		warnUnwindIncomplete(deps.Presenter, "", manualStep{
			does: "put back " + strings.Join(unknown, " + ") + ", which mint wrote over and git holds no copy of",
		})
	}
	return known, unknown
}

// restoreToHEAD returns paths, which git holds in HEAD or the index in the repo at
// root, to HEAD's content in the index and the work tree, removing those HEAD lacks.
// It touches no other path and moves no HEAD. A failure is warned with its manual
// step; it reports whether it succeeded.
func restoreToHEAD(ctx context.Context, deps ReleaseDeps, root string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	args := append([]string{"-C", root, "restore", "--source=HEAD", "--staged", "--worktree"}, pathspecsOnStdin...)
	if res, err := deps.Mutator.Mutate(ctx, record.PathspecInput(paths), "git", args...); err != nil {
		warnUnwindIncomplete(deps.Presenter, failedGitOutput(res.Stderr, err), manualStep{
			does:     "restore " + strings.Join(paths, " + ") + " to HEAD",
			commands: []string{"git restore --source=HEAD --staged --worktree -- " + quoteAll(literalPathspecs(paths))},
		})
		return false
	}
	return true
}

// removeCreated removes paths, files under root mint created, from the index and the
// disk, returning those it could not, each warned with its manual step. The index is
// cleared whatever mint believes of it: a `git add` that fails part-way, on an ignored
// path beside them, has already staged the others.
func removeCreated(ctx context.Context, deps ReleaseDeps, root string, paths []string) (left []string) {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"-C", root, "rm", "--cached", "-q", "--ignore-unmatch"}, pathspecsOnStdin...)
	if res, err := deps.Mutator.Mutate(ctx, record.PathspecInput(paths), "git", args...); err != nil {
		warnUnwindIncomplete(deps.Presenter, failedGitOutput(res.Stderr, err), manualStep{
			does:     "unstage " + strings.Join(paths, " + ") + ", which mint created",
			commands: []string{"git -C " + cmdline.Quote(root) + " rm --cached -q --ignore-unmatch -- " + quoteAll(literalPathspecs(paths))},
		})
		left = slices.Clone(paths)
	}
	for _, path := range paths {
		full := filepath.Join(root, path)
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			warnUnwindIncomplete(deps.Presenter, err.Error(), manualStep{does: "remove " + full + ", which mint created"})
			if !slices.Contains(left, path) {
				left = append(left, path)
			}
		}
	}
	return left
}

// stagedPaths reads the paths the index holds changes to, keepOut left out, in the
// repo at root, or the one the working directory is in where root is empty, returning
// git's output where the read fails.
func stagedPaths(ctx context.Context, m *git.Mutator, root string, keepOut git.Exclusion) (paths []string, output string, err error) {
	args := append(atRoot(root, "diff", "--cached", "--name-only", "-z", "--no-renames"), keepOut.Pathspec()...)
	res, err := m.Run(ctx, "git", args...)
	if err != nil {
		return nil, failedGitOutput(res.Stderr, err), err
	}
	return strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 }), "", nil
}

// atRoot is a git step's args run in the repo at root, or in the one the working
// directory is in where root is empty.
func atRoot(root string, args ...string) []string {
	if root == "" {
		return args
	}
	return append([]string{"-C", root}, args...)
}

// pathspecsOnStdin has git read a step's pathspecs from stdin (record.PathspecInput),
// so no count of paths meets the argument limit.
var pathspecsOnStdin = []string{"--pathspec-from-file=-", "--pathspec-file-nul"}

// literalPathspecs are paths as root-anchored literal pathspecs, as a manual step
// prints them.
func literalPathspecs(paths []string) []string {
	specs := make([]string, len(paths))
	for i, path := range paths {
		specs[i] = git.LiteralPathspec(path)
	}
	return specs
}

// quoteAll quotes each of args for the shell, joined by spaces.
func quoteAll(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = cmdline.Quote(arg)
	}
	return strings.Join(quoted, " ")
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
// in the repo at Root, whose index the reset reads.
type resetTarget struct {
	HEAD string
	Root string
}

// stagedIndex is what the index held of the user's ahead of a reset, for restage to
// put back after it: the tree the index was written as and the paths whose content it
// staged, the paths whose deletion it staged, and the paths marked intent-to-add,
// which the tree doesn't hold.
type stagedIndex struct {
	tree    string
	staged  []string
	deleted []string
	intents []string
}

// readIndex reads what the index holds that HEAD doesn't, in the repo at root, from
// one porcelain status, which never takes the index lock: content staged, a deletion
// staged, and intent-to-add marks. Edits only in the work tree are left out, as the
// reset keeps them.
func readIndex(ctx context.Context, m *git.Mutator, root string) (index stagedIndex, output string, err error) {
	res, err := m.Run(ctx, "git", atRoot(root, "--no-optional-locks", "status", "--porcelain=v2", "-z", "--untracked-files=no", "--no-renames")...)
	if err != nil {
		return stagedIndex{}, failedGitOutput(res.Stderr, err), err
	}
	for _, entry := range strings.FieldsFunc(res.Stdout, func(r rune) bool { return r == 0 }) {
		// An ordinary entry: "1 XY sub mH mI mW hH hI path", X the index's status.
		fields := strings.SplitN(entry, " ", 9)
		if len(fields) < 9 || fields[0] != "1" {
			continue
		}
		switch xy, path := fields[1], fields[8]; {
		case xy[0] == 'D':
			index.deleted = append(index.deleted, path)
		case xy[0] != '.':
			index.staged = append(index.staged, path)
		case xy[1] == 'A':
			index.intents = append(index.intents, path)
		}
	}
	return index, "", nil
}

// resetTo moves HEAD back to target.HEAD as `git reset --keep` does: it takes back in
// the index and the work tree exactly what the commits since target.HEAD changed, and
// keeps every other change in the work tree, a notes file's included, refusing and
// changing nothing where the work tree or the index changes a path those commits
// changed. As --keep also returns every other index entry to target.HEAD, and drops
// intent-to-add marks, what the index holds is read first (readIndex), the index
// written as a tree where it stages content, and the returned stagedIndex restages it
// all. On failure HEAD has not moved, and it returns the manual steps that finish the
// reset (keepSteps, the reset's step saying why) and the failing git step's own output.
func resetTo(ctx context.Context, m *git.Mutator, target resetTarget, why string) (index stagedIndex, manual []manualStep, output string, err error) {
	sha := target.HEAD
	index, output, err = readIndex(ctx, m, target.Root)
	if err != nil {
		return stagedIndex{}, keepSteps(sha, why, nil), output, err
	}
	keep := keepSteps(sha, why, index.intents)
	if len(index.staged) > 0 {
		res, err := m.Mutate(ctx, nil, "git", "write-tree")
		if err != nil {
			return stagedIndex{}, keep, failedGitOutput(res.Stderr, err), err
		}
		index.tree = strings.TrimSpace(res.Stdout)
	}
	if res, err := m.Mutate(ctx, nil, "git", "reset", "--keep", sha); err != nil {
		return stagedIndex{}, keep, failedGitOutput(res.Stderr, err), err
	}
	return index, nil, "", nil
}

// keepSteps are the manual steps that reset HEAD back to sha keeping the user's
// changes, the reset's step saying why: stash them, reset, then pop the stash with
// --index, which brings back what was staged, a staged deletion included, as it was.
// The pop is offered only for a stash that saved something, as a reset that failed
// for another reason may leave nothing to stash, and a pop then would apply some older
// stash. --index applies the staged part strictly, so where a staged change meets the
// lines mint's commits wrote, the very case that makes --keep refuse, git answers
// "conflicts in index. Try without --index." and keeps the stash; a plain pop then
// brings the changes back, the one to mint's lines as a merge conflict. git can't
// stash a file marked intent-to-add, so intents, the files the index read found
// marked, are unmarked first and marked again last.
func keepSteps(sha, why string, intents []string) []manualStep {
	var steps []manualStep
	if len(intents) > 0 {
		steps = append(steps, manualStep{
			does:     "unmark " + strings.Join(intents, " + ") + ", which git can't stash",
			commands: []string{"git rm --cached -q -- " + quoteAll(literalPathspecs(intents))},
		})
	}
	steps = append(steps,
		manualStep{does: "stash your changes", commands: []string{"git stash"}},
		manualStep{does: "reset HEAD back to " + sha, commands: []string{"git reset --keep " + sha}, why: why},
		manualStep{does: "bring your changes back, staged as they were, if the stash saved any", commands: []string{"git stash pop --index"}},
		manualStep{
			does:     `if git answers "conflicts in index", bring them back instead, not staged as they were, and any to mint's lines as a conflict to resolve`,
			commands: []string{"git stash pop"},
		},
	)
	if len(intents) > 0 {
		steps = append(steps, manualStep{
			does:     "mark " + strings.Join(intents, " + ") + " as intended to add again",
			commands: []string{"git add --intent-to-add -- " + quoteAll(literalPathspecs(intents))},
		})
	}
	return steps
}

// restageStep is one git step of a restage: the paths it names on stdin, its args, and
// the manual step that does it by hand.
type restageStep struct {
	paths  []string
	args   []string
	manual manualStep
}

// steps are the git steps that put the index back as s holds it, touching no work
// tree: the content staged, from the tree it was written as, which holds each of
// those paths; the deletions staged, by a removal that passes over a path the reset
// already took out of the index, one mint's commits created; and the intent-to-add
// marks. Each kind is a step of its own, so one failing never stops the rest.
func (s stagedIndex) steps() []restageStep {
	var steps []restageStep
	if len(s.staged) > 0 {
		steps = append(steps, restageStep{
			paths: s.staged,
			args:  []string{"restore", "--source=" + s.tree, "--staged"},
			manual: manualStep{
				does:     "stage your changes to " + strings.Join(s.staged, " + ") + " again",
				commands: []string{"git restore --source=" + s.tree + " --staged -- " + quoteAll(literalPathspecs(s.staged))},
			},
		})
	}
	if len(s.deleted) > 0 {
		steps = append(steps, restageStep{
			paths: s.deleted,
			args:  []string{"rm", "--cached", "-q", "--ignore-unmatch"},
			manual: manualStep{
				does:     "stage your deletion of " + strings.Join(s.deleted, " + ") + " again",
				commands: []string{"git rm --cached -q --ignore-unmatch -- " + quoteAll(literalPathspecs(s.deleted))},
			},
		})
	}
	if len(s.intents) > 0 {
		steps = append(steps, restageStep{
			paths: s.intents,
			args:  []string{"add", "--intent-to-add"},
			manual: manualStep{
				does:     "mark " + strings.Join(s.intents, " + ") + " as intended to add again",
				commands: []string{"git add --intent-to-add -- " + quoteAll(literalPathspecs(s.intents))},
			},
		})
	}
	return steps
}

// unwindOutcome is how an unwind ended: whether its reset dropped mint's commits, what
// it put back of what mint wrote or staged and no commit held, whether its tag delete
// succeeded, whether the exclusion answers were re-committed, and whether EVERY
// recovery mutation succeeded.
type unwindOutcome struct {
	reset, deleted bool
	put            MadeState
	answersKept    bool
	clean          bool
}

// unwindEvent builds the Unwound payload: what the unwind undid, the exclusion
// answers it kept, and where it left the repo. Only a clean outcome
// reads "repo clean", so the narration never calls the repo clean after a failed step.
// Each part is ASCII, so the plain presenter's joined line stays byte-pure, and none
// leads with a "Reverted:"-style prefix — the presenter prefixes the line with
// "unwound", so a prefix here would double-label.
func unwindEvent(tag string, made MadeState, outcome unwindOutcome) presenter.Unwind {
	event := presenter.Unwind{Undone: undone(tag, made, outcome), Repo: repoState(outcome.clean), Key: stageUnwind}
	if outcome.answersKept {
		event.Kept = answersKept
	}
	if outcome.answersKept || made.answersLeft {
		event.KeptRow = answersKept
	}
	return event
}

// undone names what the unwind undid: the commits reset, what it restored and the tag
// deleted, each only where its step succeeded, so a step that failed is never
// claimed; "" where none did.
func undone(tag string, made MadeState, outcome unwindOutcome) string {
	var parts []string
	if outcome.reset {
		parts = append(parts, "reset "+commitsPhrase(made.Commits))
	}
	parts = append(parts, restoredParts(outcome.put)...)
	if outcome.deleted {
		parts = append(parts, "deleted tag "+tag)
	}
	return joinAnd(parts)
}

// restoredParts names what the unwind put back of what mint wrote or staged and no
// commit held: pre_tag's changes it staged, the files the record stage wrote over,
// and those it created.
func restoredParts(made MadeState) []string {
	var parts []string
	if made.preTagStaged {
		parts = append(parts, "unstaged and removed pre_tag's changes")
	}
	if restored := made.restored(); len(restored) > 0 {
		parts = append(parts, "restored "+strings.Join(restored, " + "))
	}
	if len(made.created) > 0 {
		parts = append(parts, "removed "+strings.Join(made.created, " + "))
	}
	return parts
}

// joinAnd joins parts as a sentence lists them: "a", "a and b", "a, b and c".
func joinAnd(parts []string) string {
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	last := len(parts) - 1
	return strings.Join(parts[:last], ", ") + " and " + parts[last]
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
