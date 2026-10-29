package engine

// This file is the single-version regenerate WRITE / PUSH / RECOVERY orchestration
// (task 5-9): it wires the already-shipped regenerate pieces (the resolved target,
// the source body, the create-or-update DISPATCH 5-7, and the changelog write+commit
// 5-8) into one sequence and owns the ORDERING + the recovery asymmetry:
//
//	gate/confirm → (fresh: write changelog commit) → push (PONR) → provider write
//
// It is the regenerate sibling of the forward release spine, adapted to the fact
// that NO tag is ever cut (tags are immutable):
//
//   - PUSH FORM is the PLAIN `git push origin HEAD` (no tag involved), NOT the
//     forward `git push --atomic origin HEAD {tag}`. This push is the POINT OF NO
//     RETURN for the changelog surface.
//   - RECOVERY (pre-PONR) is LIGHTER than the forward surgical unwind: there is no
//     tag to delete, so any pre-push failure just RESETS the local CHANGELOG commit
//     back to the clean start, leaving any notes file as it was (resetTo). A gate
//     abort lands before the commit, and a release-only TARGET (any source) makes no
//     commit, so neither has anything to reset.
//   - POST-PONR a provider create/update failure AFTER the changelog push is WARN
//     ONLY — the changelog is already public, so the run never unwinds; the user
//     re-heals the provider with `--target release`. This is the SAME warn-only
//     asymmetry the forward path uses post-push.
//   - --target both is NON-ATOMIC across surfaces: the changelog (commit + push) is
//     written FIRST, then the provider release; a provider failure after the
//     changelog push is the warn-only case, not a rollback.
//
// SOURCE GATING: fresh runs the full notes-review gate (y/n/e/r) BEFORE writing —
// backfilled notes are reviewable before they overwrite live surfaces; a supplied
// --notes-file body and a notes-floor body run the same gate without `r`; reuse is a
// simple two-choice confirm (no review gate — reuse generates no new notes to
// edit/regenerate). `-y` skips all of them (handled by the presenter, which returns
// the gate default).
//
// HISTORICAL DATE (carried from 5-8's review): the changelog write MUST keep the
// version's ORIGINAL historical release date in its `## [x.y.z] - <date>` header —
// rewriting it to today is a data-integrity regression AND would break the
// no-empty-commit guarantee. The date is recovered from the tag metadata via
// `git for-each-ref --format=%(creatordate:short) refs/tags/<tag>` (robust across
// annotated and lightweight tags), parsed with the SAME layout the forward changelog
// writer emits (YYYY-MM-DD), so the healed header matches existing sections exactly.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/runner"
)

// RegenerateSource names where a regenerate run sourced its notes body — the axis
// that drives the gating (gateRegenerate). It is the engine-level concrete enum so the
// orchestration never threads the cmd-layer flag types.
type RegenerateSource int

const (
	// RegenerateSourceFresh is the re-diff + AI path (5-6): freshly-generated notes
	// run the notes-review gate before they overwrite live surfaces.
	RegenerateSourceFresh RegenerateSource = iota
	// RegenerateSourceTag is the tag-annotation read path (5-5): deterministic,
	// parse-free — a simple confirm, no review gate.
	RegenerateSourceTag
	// RegenerateSourceRelease reads the EXISTING provider release body back (via the
	// publish.Publisher ReadReleaseBody seam) as the source — the provider-release
	// sibling of reuse, used to re-project the published release notes onto the
	// changelog. Like reuse it is deterministic (no AI), so it runs the simple confirm,
	// not the review gate.
	RegenerateSourceRelease
	// regenerateSourceSupplied is the finished body handed over with --notes-file. It
	// runs the review gate without `r`, since no AI wrote it. The run already holds
	// its body, so no body producer handles it.
	regenerateSourceSupplied
	// regenerateSourceFloor is the body chosen at the notes floor after the fresh
	// source's generation failed. Like a supplied body it runs the review gate without
	// `r`, since no AI wrote it.
	regenerateSourceFloor
)

// RegenerateTarget names which surface(s) a regenerate run writes — the axis that
// drives whether the changelog is committed + pushed and whether the provider
// release is written. It is the engine-level concrete enum (the cmd layer maps its
// own flag enum onto this).
type RegenerateTarget int

const (
	// RegenerateTargetRelease writes the provider release body only (no changelog
	// commit, no push of a commit).
	RegenerateTargetRelease RegenerateTarget = iota
	// RegenerateTargetChangelog writes CHANGELOG.md only (commit + push, no provider
	// write).
	RegenerateTargetChangelog
	// RegenerateTargetBoth writes the changelog (commit + push) FIRST, then the
	// provider release — non-atomic across surfaces.
	RegenerateTargetBoth
)

// writesChangelog reports whether the target commits + pushes the changelog.
func (t RegenerateTarget) writesChangelog() bool {
	return t == RegenerateTargetChangelog || t == RegenerateTargetBoth
}

// writesProvider reports whether the target writes the provider release.
func (t RegenerateTarget) writesProvider() bool {
	return t == RegenerateTargetRelease || t == RegenerateTargetBoth
}

// RegenerateWriteRequest carries the resolved per-version inputs the write path
// consumes. The body is already produced upstream (5-5 reuse read or 5-6 fresh AI),
// so this struct holds only what the write/push/recovery sequence needs.
type RegenerateWriteRequest struct {
	// Source selects the gate gateRegenerate runs.
	Source RegenerateSource
	// Target selects the surface(s) written: release, changelog, or both.
	Target RegenerateTarget
	// Tag is the canonical target tag (e.g. "v1.4.0") — the commit subject token, the
	// provider tag/title, and the ref the historical date is read from.
	Tag string
	// VersionKey is the bare x.y.z section key used in the `## [x.y.z] - date` header.
	VersionKey string
	// Body is the full regenerated notes body, used WHOLE for both the changelog
	// section and the provider release.
	Body string
	// Regenerator is the PER-RUN regenerate seam the fresh notes-review gate's `r`
	// choice consults with the user's one-time context: on chunked notes it re-merges
	// the version's own part notes in one call, otherwise it re-runs the fresh AI path
	// over the resolved `vX-1..vX` range (production binds RegenerateFreshRegenerator
	// here, the regenerate analogue of the forward path's perRunRegenerator). It is
	// meaningful ONLY for a fresh source (reuse has no review gate); a reuse run leaves
	// it nil. A wired deps.Regenerator OVERRIDES it (the test-injection seam), mirroring
	// the forward path's precedence.
	Regenerator Regenerator
	// KeepOut is the path the changelog commit leaves out, any staged edits to it
	// still staged afterwards.
	KeepOut git.Exclusion
	// LowerCap is the cap hinted after the notes, 0 for none.
	LowerCap int
	// regens narrates each `r` and hints after its notes; chunkNotes keeps the version's
	// chunk notes, saving those of a failed `r`. With either nil, an `r` is neither
	// narrated nor hinted and keeps nothing.
	regens     *regenStage
	chunkNotes *chunkNotesKeeper
	// node is the stage key of the node the review asks about, which its answer tails:
	// a --all version's own; "" for a single version's review node, which the answer
	// settles.
	node string
}

// reviewing is gate asked about the review: a single version's review node, which no
// stage event settles, so the gate carries what its answer settles it to, or the
// node req names, whose answer tails it.
func (req RegenerateWriteRequest) reviewing(gate presenter.Gate) presenter.Gate {
	if req.node != "" {
		gate.Stage = req.node
		return gate
	}
	gate.Stage, gate.Detail = stageReview, "accepted"
	return gate
}

// RegenerateWrite runs the single-version regenerate write/push/recovery sequence for
// one resolved request: gate/confirm by source, then (for a changelog/both target)
// write the historical-dated CHANGELOG commit and push it plain (the point of no
// return), then (for a release/both target) write the provider release. It returns
// nil on success, an *AbortError on a gate decline or any pre-push failure (after
// resetting any CHANGELOG commit it made), and nil with a Warn on a post-push
// provider failure (warn-only — the changelog is already public).
//
// publisher is the resolved provider driver behind the Publisher seam; it is consumed
// only for a release/both target (a changelog-only run leaves it unused and may pass
// nil). root is the repo root (the changelog lives at root/CHANGELOG.md).
func RegenerateWrite(ctx context.Context, deps ReleaseDeps, publisher publish.Publisher, root string, req RegenerateWriteRequest) error {
	p := deps.Presenter

	// Gate/confirm by source BEFORE any mutation.
	body, err := gateRegenerate(ctx, deps, req)
	if err != nil {
		return err
	}

	// Changelog surface: write the historical-dated commit, then push it plain. The
	// push is the point of no return; a gate abort already returned above, and any
	// pre-push failure here resets the commit back to the captured clean start.
	pushed := false
	if req.Target.writesChangelog() {
		ponr, err := writeAndPushChangelog(ctx, deps, root, req, body)
		if err != nil {
			return err
		}
		pushed = ponr
	}

	// Provider surface: create-or-update via the per-version DISPATCH (5-7). For
	// --target both this runs AFTER the changelog push, so a failure here is the
	// post-PONR warn-only case (the changelog is already public). For --target
	// release there is no push, so a failure is surfaced as a plain abort.
	//
	// NIL-GUARD: a downgraded run (the provider could not be resolved — a non-github /
	// no-remote origin) carries a nil publisher. Skip the provider write with a warn
	// rather than dereferencing nil in DispatchRelease — mirroring the forward path,
	// which downgrades to tag + push only and never attempts the provider write. The
	// changelog (for --target both) was already written above; the provider surface is
	// simply skipped.
	if req.Target.writesProvider() {
		if publisher == nil {
			warnRegenerateProviderSkipped(p, stagePublish)
			return nil
		}
		// Regenerate's close carries no footer URL, so the dispatched release URL is
		// discarded here — only the forward release path threads it into RunResult.URL.
		if _, err := DispatchRelease(ctx, publisher, req.Tag, req.Tag, body); err != nil {
			if pushed {
				// Post-PONR: the changelog is already public, so never unwind — warn and
				// point at the heal path, exactly as the forward path's post-push failure.
				warnRegenerateProviderFailed(p, err, releaseName(publisher), regenerateMoveCommand(req.Tag, req.Source, RegenerateTargetRelease))
				return nil
			}
			// Release-only: nothing was pushed, so this is a plain pre-PONR failure. The
			// provider write ran, so the close names only what mint did.
			deps.run.Left(noClaim)
			return surface(p, stagePublish, err)
		}
		p.StageSucceeded(presenter.Settled(stagePublish, releaseName(publisher)+" written"))
	}

	return nil
}

// gateRegenerate runs the source-appropriate gate/confirm BEFORE any mutation and
// returns the body to write. Fresh runs the full notes-review gate (y/n/e/r), whose
// edit/regenerate choices may replace the body; a supplied or floor body runs the same
// gate without `r`; reuse runs a simple two-choice confirm (no review gate). A decline
// aborts non-zero. Under `-y` the presenter returns each gate's default (ChoiceYes),
// so the gate accepts immediately.
func gateRegenerate(ctx context.Context, deps ReleaseDeps, req RegenerateWriteRequest) (string, error) {
	p := deps.Presenter
	// Show the notes BEFORE the gate so the user reviews the body they are accepting —
	// the regenerate mirror of the forward path's ShowNotes-then-reviewGate adjacency
	// (release.go). reviewGate re-shows only AFTER an edit/regenerate choice; the INITIAL
	// render is the caller's, so without this the fresh gate (and the reuse confirm) would
	// prompt over an empty preview. Both the single-version (RegenerateWrite) and batch
	// (gatePerVersion) fresh/reuse paths route through here, so one render covers both.
	p.ShowNotes(presenter.Notes{Version: req.VersionKey, Body: req.Body})
	hintLowerCap(p, req.LowerCap)
	switch req.Source {
	case RegenerateSourceFresh:
		// Fresh: the notes-review gate. A decline returns errGateAborted (a clean user
		// abort); the gate sits before the changelog commit, so there is nothing to reset.
		// The `r` choice consults the per-run regenerator (req.Regenerator), bound to this
		// version's chunk notes or fresh AI range — a wired deps.Regenerator OVERRIDES it
		// (the test seam), mirroring the forward path's perRunRegenerator precedence.
		return reviewFresh(ctx, deps, req)
	case regenerateSourceSupplied, regenerateSourceFloor:
		return reviewGate(ctx, deps, reviewRegen{}, req.reviewing(presenter.NotesReviewGateNoRegen()), req.Body, req.VersionKey)
	default:
		// Deterministic source (tag annotation or provider release): the simple
		// two-choice confirm — there are no freshly-generated notes to edit/regenerate.
		return reuseConfirm(p, req)
	}
}

// reviewFresh runs the fresh notes-review gate. An `r` that fails saves the version's
// chunk notes, named with the version's regenerate that puts them on it.
func reviewFresh(ctx context.Context, deps ReleaseDeps, req RegenerateWriteRequest) (string, error) {
	regenerator := regenerateRegenerator(deps, req.Regenerator)
	if req.regens == nil || req.chunkNotes == nil {
		return reviewGate(ctx, deps, reviewRegen{regenerator: regenerator}, req.reviewing(presenter.NotesReviewGate()), req.Body, req.VersionKey)
	}
	regen := req.regens.review(regenerator, req.chunkNotes)
	return req.chunkNotes.reviewGate(ctx, deps, regen, req.reviewing(presenter.NotesReviewGate()), req.Body, req.VersionKey, regenerateRerun(req.VersionKey, req.Target))
}

// regenerateRegenerator selects the Regenerator the fresh regenerate gate's `r` choice
// consults, mirroring the forward path's perRunRegenerator precedence: a wired
// deps.Regenerator OVERRIDES everything (the test-injection seam), otherwise the
// per-run regenerator the cmd layer bound to this version's chunk notes or fresh AI
// range is used. A reuse run never reaches this (it runs the simple confirm, not the
// review gate), so a nil per-run regenerator on a fresh run is the misconfiguration
// the gate surfaces.
func regenerateRegenerator(deps ReleaseDeps, perRun Regenerator) Regenerator {
	if deps.Regenerator != nil {
		return deps.Regenerator
	}
	return perRun
}

// reuseConfirm runs the deterministic reuse confirm: the two-choice ReuseConfirmGate
// (y/n), with no review loop because reuse generates no new notes to edit/regenerate.
// ChoiceYes proceeds with the unchanged body; ChoiceNo is a clean abort; a Prompt
// read failure is already an *AbortError carrying a non-zero exit.
func reuseConfirm(p presenter.Presenter, req RegenerateWriteRequest) (string, error) {
	choice, err := ReviewDecision(p, req.reviewing(presenter.ReuseConfirmGate()))
	if err != nil {
		return "", err
	}
	if choice == presenter.ChoiceNo {
		return "", abort(errGateAborted)
	}
	return req.Body, nil
}

// writeAndPushChangelog writes the version's regenerated section (under its ORIGINAL
// historical date) into a single CHANGELOG commit, then pushes it plain. It returns
// whether the push crossed the point of no return. Any pre-push failure — the
// historical-date read, the write/commit, or the push itself — RESETS the local
// CHANGELOG commit back to the captured clean start (a plain commit reset; no tag is
// ever involved) and is surfaced as an abort. A reset is issued only when a commit
// actually landed; a write that nets no change makes no commit and pushes nothing.
// The rail draws it all, rewrite to push, on the one CHANGELOG.md node.
func writeAndPushChangelog(ctx context.Context, deps ReleaseDeps, root string, req RegenerateWriteRequest, body string) (pushed bool, err error) {
	p := deps.Presenter
	node := startChangelogNode(deps, "rewriting "+record.ChangelogFileName+"…")

	// Capture the clean starting HEAD before any mutation so the recovery resets to
	// exactly here — the regenerate analogue of the forward path's StartState capture.
	startingHEAD, err := resolveHEAD(ctx, deps.Runner)
	if err != nil {
		return false, surface(p, stageRecord, err)
	}
	start := resetTarget{HEAD: startingHEAD, KeepOut: req.KeepOut}

	// Recover the version's ORIGINAL historical date from the tag metadata so the
	// healed `## [x.y.z] - <date>` header keeps its original date, not today.
	date, err := readHistoricalDate(ctx, deps.Runner, req.Tag)
	if err != nil {
		return false, surface(p, stageRecord, err)
	}

	// The write may leave CHANGELOG.md changed, so the close claims no more than the
	// recovery below can name.
	deps.run.Left(noClaim)
	committed, err := RegenerateChangelog(ctx, deps.Mutator, root, req.VersionKey, req.Tag, date, body, req.KeepOut)
	if err != nil {
		// A failed write/stage/commit makes no commit, but a partial commit could have
		// landed before a later step; reset iff one did, then abort. (RegenerateChangelog
		// returns committed=false on a failed stage/commit, so the reset no-ops there.)
		return false, resetAndAbort(ctx, deps, start, committed, stageFailure(stageRecord, err), err)
	}
	if !committed {
		// No net change → no commit → nothing to push. Reaching here with a no-op write
		// is not a failure; the surface is simply byte-for-byte unchanged.
		node.succeeded(presenter.StageSuccess{Detail: "unchanged · nothing to push"})
		return false, nil
	}

	// The changelog commit landed: push it (the point of no return) via the shared
	// regenerate push tail. A successful push crosses the PONR (return pushed=true); a
	// pre-push failure is already routed through the helper's reset-on-abort.
	if err := pushChangelogCommit(ctx, deps, start, node); err != nil {
		return false, err
	}
	node.succeeded(presenter.StageSuccess{Detail: "rewritten · committed and pushed", Commit: regenerateChangelogSubject(req.Tag)})
	return true, nil
}

// startChangelogNode starts the CHANGELOG.md node a regenerate's changelog write runs
// on, doing text, Quiet: plain has no line for the write, only for its push.
func startChangelogNode(deps ReleaseDeps, text string) *blockingStage {
	return startQuietStep(deps.Presenter, deps.clock(), stepOf{name: stageRecord, key: stageRecord}, text)
}

// stageAndCommitChangelog is the SHARED regenerate stage+commit half both the
// single-version (RegenerateChangelog) and batch (commitAndPushRebuild) paths invoke to
// land a CHANGELOG commit: it runs `git -C {root} add CHANGELOG.md` then commits the
// index under subject through the lock-resilient Mutator, parameterised by subject so
// each path keeps its own commit subject. A keepOut path is left out of the commit, any
// staged edits to it still staged afterwards (record.CommitStaged). A failed step
// short-circuits (a failed stage never reaches the commit) and the returned error names
// which step failed (`staging …` / `committing …`) so each caller can apply its own
// distinct error-recovery to the returned error — the single-version path wraps it with
// the tag, the batch path routes it through resetAndAbort. Keeping the stage/commit
// idiom here means the two regenerate paths can no longer drift on the stage/commit half
// (the mirror of pushChangelogCommit owning the push/recovery half).
func stageAndCommitChangelog(ctx context.Context, m *git.Mutator, root, subject string, keepOut git.Exclusion) error {
	if res, err := m.Mutate(ctx, nil, "git", "-C", root, "add", record.ChangelogFileName); err != nil {
		return fmt.Errorf("staging %s: %w", record.ChangelogFileName, record.NewGitError(res.Stderr, err))
	}
	committed, err := record.CommitStaged(ctx, m, root, subject, keepOut)
	if err != nil {
		return fmt.Errorf("committing %q: %w", subject, err)
	}
	if !committed {
		return fmt.Errorf("committing %q: %w", subject, record.ErrNothingStaged)
	}
	return nil
}

// pushChangelogCommit is the SHARED regenerate push/recovery tail both the
// single-version (writeAndPushChangelog) and batch (commitAndPushRebuild) paths invoke
// after producing their respective CHANGELOG commit. It performs the PLAIN
// `git push origin HEAD` — NOT the forward `--atomic … {tag}` (no tag is involved) —
// which is the regenerate POINT OF NO RETURN. The push round-trips the network, so it
// is narrated as a BLOCKING stage: a blocking StageStarted (spinner) before the push
// and a StageSucceeded carrying the engine-measured Elapsed once it crosses the PONR.
// A pre-push failure routes through resetAndAbort, which surfaces a "push" StageFailed
// (so no StageSucceeded fires) and resets the local CHANGELOG commit back to the
// captured starting HEAD — both call sites reach this only after a commit landed, so the
// reset always applies. Keeping the push form, PONR narration, and reset wiring here
// means the two regenerate paths can no longer drift on push form or recovery. The
// push is a step of the CHANGELOG.md node it runs on, its failure timed as the node.
func pushChangelogCommit(ctx context.Context, deps ReleaseDeps, start resetTarget, node *blockingStage) error {
	push := startStep(deps.Presenter, deps.clock(), stepOf{name: stagePush, key: node.step.key, ongoing: true}, "pushing to origin…", deps.clock()())
	push.sentence = "Pushed to origin"
	if res, err := deps.Mutator.Mutate(ctx, nil, "git", "push", "origin", "HEAD"); err != nil {
		cause := fmt.Errorf("pushing regenerated changelog: %w", record.NewGitError(res.Stderr, err))
		failure := push.failure(cause)
		failure.Elapsed = node.elapsed()
		return resetAndAbort(ctx, deps, start, true, failure, cause)
	}
	push.done("pushed to origin")
	return nil
}

// resetAndAbort handles a pre-PONR changelog failure: it surfaces failure, resets the
// local CHANGELOG commit back to start when a commit landed (resetTo — no tag,
// best-effort), narrates the reset as the rail's unwind, Quiet as plain has no line
// for it, tells the run's close where that left the repo, and returns the abort on
// cause. With no commit made the reset is skipped, and a CHANGELOG.md rewritten but
// not committed is what the run leaves; a write that failed before that leaves the
// close claiming nothing. The ✗ line carries the failure's message, the failing git
// step's stderr verbatim beneath it.
func resetAndAbort(ctx context.Context, deps ReleaseDeps, start resetTarget, committed bool, failure presenter.StageFailure, cause error) error {
	deps.Presenter.StageFailed(failure)
	var uncommitted *uncommittedChangelog
	if errors.As(cause, &uncommitted) {
		deps.run.Left(record.ChangelogFileName + notCommitted + " · " + nothingPushed)
	}
	if committed {
		resetChangelogCommit(ctx, deps, start)
	}
	return abort(cause)
}

// resetChangelogCommit resets the changelog commit a failure left back to start.
//
// The reset is RECOVERY that must run even when the abort was the parent context's
// own cancellation (a SIGINT/SIGTERM pre-PONR abort). Detaching with
// context.WithoutCancel keeps the local-only reset from being killed by the same
// signal, so the changelog commit is rolled back and the repo left clean — the
// regenerate sibling of the forward Unwind's cancellation resilience.
//
// A failed reset is not swallowed: it surfaces a manual-cleanup Warn naming the step
// the user must run, and why: where HEAD has already left the commit, only the
// commit's changes are left to drop.
func resetChangelogCommit(ctx context.Context, deps ReleaseDeps, start resetTarget) {
	moved, manual, output, err := resetTo(context.WithoutCancel(ctx), deps.Mutator, start)
	event := presenter.Unwind{Key: stageUnwind, Quiet: true}
	if moved {
		event.Undone = "reset the changelog commit"
	}
	if err != nil {
		manual.why = "to drop the leftover changelog commit"
		if moved {
			manual.why = "to drop the changelog commit's leftover changes"
		}
		warnUnwindIncomplete(deps.Presenter, output, manual)
	}
	deps.Presenter.Unwound(event)
	if err != nil {
		deps.run.Unwound(manualCleanup + " · " + nothingPushed)
		return
	}
	deps.run.Unwound(backWhereItStarted + " · " + nothingPushed)
}

// readHistoricalDate recovers the version's ORIGINAL release date from the tag
// metadata via `git for-each-ref --format=%(creatordate:short) refs/tags/<tag>`
// through the read seam. creatordate is used (over taggerdate) because it resolves
// robustly for BOTH annotated tags (the tagger date) and lightweight tags (the
// commit date), so a tag mint did not create still yields a date. The :short form
// emits YYYY-MM-DD, parsed with the same layout the forward changelog writer uses so
// the healed header matches existing sections. A read failure or an unparseable date
// is surfaced (the date is load-bearing for changelog integrity, so it fails loud
// rather than silently falling back to today).
func readHistoricalDate(ctx context.Context, r runner.CommandRunner, tag string) (time.Time, error) {
	res, err := r.Run(ctx, "git", "for-each-ref", "--format=%(creatordate:short)", "refs/tags/"+tag)
	if err != nil {
		return time.Time{}, fmt.Errorf("reading original date for tag %s: %w", tag, err)
	}
	raw := strings.TrimSpace(res.Stdout)
	if raw == "" {
		return time.Time{}, fmt.Errorf("tag %s has no resolvable creation date", tag)
	}
	date, err := time.Parse(record.ChangelogDateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing original date %q for tag %s: %w", raw, tag, err)
	}
	return date, nil
}

// warnRegenerateProviderSkipped emits the downgrade notice when a provider-writing
// target runs with NO resolved publisher (the provider could not be resolved — a
// non-github / no-remote origin). The provider surface is skipped rather than the run
// dereferencing a nil Publisher; this rides the Warn seam (no failure state), so the run
// finishes successfully — the regenerate analogue of the forward path's tag + push only
// downgrade. The cmd layer already warned the downgrade REASON at resolve time; this
// second warn names which surface was skipped as a result. key is the node it ends:
// the release's own, or a --all version's.
func warnRegenerateProviderSkipped(p presenter.Presenter, key string) {
	p.Warn(presenter.Warning{
		Key:     key,
		Label:   "publish skipped",
		Message: "provider could not be resolved; the release surface was not written",
	})
}

// warnRegenerateProviderFailed emits the post-PONR warn-only notice when the provider
// create/update fails AFTER the changelog has already been pushed: the changelog is
// public, so the run must NOT unwind — it warns and points at the `--target release`
// re-heal, mirroring the forward path's post-push warn-only handling. It rides the
// Warn seam (no failure state), so the run finishes successfully. The rail's row says
// what happened, and heal, the regenerate that writes release again, goes under next.
func warnRegenerateProviderFailed(p presenter.Presenter, cause error, release, heal string) {
	const pushed = "changelog is already pushed"
	p.Warn(presenter.Warning{
		Key:     stagePublish,
		Label:   "publish failed",
		Message: pushed + "; re-heal the provider release with --target release",
		Row:     pushed,
		Output:  cause.Error(),
	})
	p.NextMoves(presenter.NextMoves{Stage: stagePublish, Quiet: true, Moves: []presenter.NextMove{
		{Lead: "Re-heal the " + release + ":", Command: heal},
	}})
}
