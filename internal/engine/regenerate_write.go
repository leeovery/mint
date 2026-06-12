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
//     tag to delete, so a gate abort (fresh) or any pre-push failure just RESETS the
//     local CHANGELOG commit (`git reset --hard {startingHEAD}`) back to the clean
//     start. Reuse-only and release-only runs make no commit, so there is nothing to
//     reset.
//   - POST-PONR a provider create/update failure AFTER the changelog push is WARN
//     ONLY — the changelog is already public, so the run never unwinds; the user
//     re-heals the provider with `--target release`. This is the SAME warn-only
//     asymmetry the forward path uses post-push.
//   - --target both is NON-ATOMIC across surfaces: the changelog (commit + push) is
//     written FIRST, then the provider release; a provider failure after the
//     changelog push is the warn-only case, not a rollback.
//
// SOURCE GATING: fresh runs the full notes-review gate (y/n/e/r) BEFORE writing —
// backfilled notes are reviewable before they overwrite live surfaces; reuse is a
// simple two-choice confirm (no review gate — reuse generates no new notes to
// edit/regenerate). `-y` skips both (handled by the presenter, which returns the
// gate default).
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
	"fmt"
	"strings"
	"time"

	"mint/internal/git"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/runner"
)

// RegenerateSource names where a regenerate run sourced its notes body — the axis
// that drives the gating: fresh notes are reviewable (the full y/n/e/r gate), reuse
// is deterministic (a simple confirm). It is the engine-level concrete enum so the
// orchestration never threads the cmd-layer flag types.
type RegenerateSource int

const (
	// RegenerateSourceFresh is the re-diff + AI path (5-6): freshly-generated notes
	// run the notes-review gate before they overwrite live surfaces.
	RegenerateSourceFresh RegenerateSource = iota
	// RegenerateSourceReuse is the tag-annotation read path (5-5): deterministic,
	// parse-free — a simple confirm, no review gate.
	RegenerateSourceReuse
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
	// Source selects the gating: fresh → notes-review gate, reuse → simple confirm.
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
	// choice consults: it re-runs the fresh AI path over the resolved `vX-1..vX` range
	// with the user's one-time context appended (production binds
	// notes.Generator.GenerateFromRangeWithContext here, the regenerate analogue of the
	// forward path's perRunRegenerator). It is meaningful ONLY for a fresh source (reuse
	// has no review gate); a reuse run leaves it nil. A wired deps.Regenerator OVERRIDES
	// it (the test-injection seam), mirroring the forward path's precedence.
	Regenerator Regenerator
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

	// Gate/confirm by source BEFORE any mutation. Fresh runs the notes-review gate
	// (the reviewed body may be edited/regenerated); reuse is a simple confirm.
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
			warnRegenerateProviderSkipped(p)
			return nil
		}
		// Regenerate's close carries no footer URL, so the dispatched release URL is
		// discarded here — only the forward release path threads it into RunResult.URL.
		if _, err := DispatchRelease(ctx, publisher, req.Tag, req.Tag, body); err != nil {
			if pushed {
				// Post-PONR: the changelog is already public, so never unwind — warn and
				// point at the heal path, exactly as the forward path's post-push failure.
				warnRegenerateProviderFailed(p, err)
				return nil
			}
			// Release-only: nothing was pushed, so this is a plain pre-PONR failure.
			return surface(p, "publish", err)
		}
	}

	return nil
}

// gateRegenerate runs the source-appropriate gate/confirm BEFORE any mutation and
// returns the body to write. Fresh runs the full notes-review gate (y/n/e/r), whose
// edit/regenerate choices may replace the body; reuse runs a simple two-choice
// confirm (no review gate). A decline aborts non-zero. `-y` skips both — the
// presenter returns the gate default (ChoiceYes), so the gate accepts immediately.
func gateRegenerate(ctx context.Context, deps ReleaseDeps, req RegenerateWriteRequest) (string, error) {
	p := deps.Presenter
	if req.Source == RegenerateSourceReuse {
		return reuseConfirm(p, req.Body)
	}
	// Fresh: the notes-review gate. A decline returns errGateAborted (a clean user
	// abort); the gate sits before the changelog commit, so there is nothing to reset.
	// The `r` choice consults the per-run regenerator (req.Regenerator), bound to this
	// version's fresh AI range — a wired deps.Regenerator OVERRIDES it (the test seam),
	// mirroring the forward path's perRunRegenerator precedence.
	return reviewGate(ctx, p, deps.Editor, regenerateRegenerator(deps, req.Regenerator), notes.KindNormalAI, req.Body, req.VersionKey)
}

// regenerateRegenerator selects the Regenerator the fresh regenerate gate's `r` choice
// consults, mirroring the forward path's perRunRegenerator precedence: a wired
// deps.Regenerator OVERRIDES everything (the test-injection seam), otherwise the
// per-run regenerator the cmd layer bound to this version's fresh AI range is used. A
// reuse run never reaches this (it runs the simple confirm, not the review gate), so a
// nil per-run regenerator on a fresh run is the misconfiguration the gate surfaces.
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
func reuseConfirm(p presenter.Presenter, body string) (string, error) {
	choice, err := ReviewDecision(p, presenter.ReuseConfirmGate())
	if err != nil {
		return "", err
	}
	if choice == presenter.ChoiceNo {
		return "", abort(errGateAborted)
	}
	return body, nil
}

// writeAndPushChangelog writes the version's regenerated section (under its ORIGINAL
// historical date) into a single CHANGELOG commit, then pushes it plain. It returns
// whether the push crossed the point of no return. Any pre-push failure — the
// historical-date read, the write/commit, or the push itself — RESETS the local
// CHANGELOG commit back to the captured clean start (a plain commit reset; no tag is
// ever involved) and is surfaced as an abort. A reset is issued only when a commit
// actually landed; a write that nets no change makes no commit and pushes nothing.
func writeAndPushChangelog(ctx context.Context, deps ReleaseDeps, root string, req RegenerateWriteRequest, body string) (pushed bool, err error) {
	p := deps.Presenter

	// Capture the clean starting HEAD before any mutation so the recovery resets to
	// exactly here — the regenerate analogue of the forward path's StartState capture.
	startingHEAD, err := resolveHEAD(ctx, deps.Runner)
	if err != nil {
		return false, surface(p, "record", err)
	}

	// Recover the version's ORIGINAL historical date from the tag metadata so the
	// healed `## [x.y.z] - <date>` header keeps its original date, not today.
	date, err := readHistoricalDate(ctx, deps.Runner, req.Tag)
	if err != nil {
		return false, surface(p, "record", err)
	}

	committed, err := RegenerateChangelog(ctx, deps.Mutator, root, req.VersionKey, req.Tag, date, body)
	if err != nil {
		// A failed write/stage/commit makes no commit, but a partial commit could have
		// landed before a later step; reset iff one did, then abort. (RegenerateChangelog
		// returns committed=false on a failed stage/commit, so the reset no-ops there.)
		return false, resetAndAbort(ctx, deps, startingHEAD, committed, "record", err)
	}
	if !committed {
		// No net change → no commit → nothing to push. Reaching here with a no-op write
		// is not a failure; the surface is simply byte-for-byte unchanged.
		return false, nil
	}

	// The changelog commit landed: push it (the point of no return) via the shared
	// regenerate push tail. A successful push crosses the PONR (return pushed=true); a
	// pre-push failure is already routed through the helper's reset-on-abort.
	if err := pushChangelogCommit(ctx, deps, startingHEAD); err != nil {
		return false, err
	}
	return true, nil
}

// stageAndCommitChangelog is the SHARED regenerate stage+commit half both the
// single-version (RegenerateChangelog) and batch (commitAndPushRebuild) paths invoke to
// land a CHANGELOG commit: it runs `git -C {root} add CHANGELOG.md` then
// `git -C {root} commit -m {subject}` through the lock-resilient Mutator, parameterised
// ONLY by subject so each path keeps its own commit subject. A failed step short-circuits
// (a failed stage never reaches the commit) and the returned error names which step
// failed (`staging …` / `committing …`) so each caller can apply its own distinct
// error-recovery to the returned error — the single-version path wraps it with the tag,
// the batch path routes it through resetAndAbort. Keeping the stage/commit idiom here
// means the two regenerate paths can no longer drift on the stage/commit half (the mirror
// of pushChangelogCommit owning the push/recovery half).
func stageAndCommitChangelog(ctx context.Context, m *git.Mutator, root, subject string) error {
	if _, err := m.Mutate(ctx, nil, "git", "-C", root, "add", record.ChangelogFileName); err != nil {
		return fmt.Errorf("staging %s: %w", record.ChangelogFileName, err)
	}
	if _, err := m.Mutate(ctx, nil, "git", "-C", root, "commit", "-m", subject); err != nil {
		return fmt.Errorf("committing %q: %w", subject, err)
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
// captured startingHEAD — both call sites reach this only after a commit landed, so the
// reset always applies. Keeping the push form, PONR narration, and reset wiring here
// means the two regenerate paths can no longer drift on push form or recovery.
func pushChangelogCommit(ctx context.Context, deps ReleaseDeps, startingHEAD string) error {
	pushDone := emitBlockingStageStarted(deps.Presenter, "push", "pushing to origin…")
	if _, err := deps.Mutator.Mutate(ctx, nil, "git", "push", "origin", "HEAD"); err != nil {
		return resetAndAbort(ctx, deps, startingHEAD, true, "push", fmt.Errorf("pushing regenerated changelog: %w", err))
	}
	pushDone()
	return nil
}

// resetAndAbort handles a pre-PONR changelog failure: it surfaces the failed stage,
// resets the local CHANGELOG commit back to the captured starting HEAD when a commit
// landed (a plain `git reset --hard` through the lock-resilient Mutator — no tag,
// best-effort), and returns the abort. With no commit made the reset is skipped.
func resetAndAbort(ctx context.Context, deps ReleaseDeps, startingHEAD string, committed bool, stage string, cause error) error {
	deps.Presenter.StageFailed(presenter.StageFailure{Name: stage, Message: failureMessage(cause)})
	if committed {
		// The reset is RECOVERY that must run even when the abort was the parent context's
		// own cancellation (a SIGINT/SIGTERM pre-PONR abort). Detaching with
		// context.WithoutCancel keeps the local-only `git reset --hard` from being killed by
		// the same signal, so the changelog commit is rolled back and the repo left clean —
		// the regenerate sibling of the forward Unwind's cancellation resilience.
		//
		// A failed reset is no longer swallowed: it surfaces a manual-cleanup Warn naming
		// the exact `git reset --hard {startingHEAD}` the user must run, so the leftover
		// CHANGELOG commit is not silently left in place.
		if _, err := deps.Mutator.Mutate(context.WithoutCancel(ctx), nil, "git", "reset", "--hard", startingHEAD); err != nil {
			warnUnwindIncomplete(deps.Presenter, "reset HEAD back to "+startingHEAD+" (`git reset --hard "+startingHEAD+"`) to drop the leftover changelog commit", err)
		}
	}
	return abort(cause)
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
// second warn names which surface was skipped as a result.
func warnRegenerateProviderSkipped(p presenter.Presenter) {
	p.Warn(presenter.Warning{
		Label:   "publish skipped",
		Message: "provider could not be resolved; the release surface was not written",
	})
}

// warnRegenerateProviderFailed emits the post-PONR warn-only notice when the provider
// create/update fails AFTER the changelog has already been pushed: the changelog is
// public, so the run must NOT unwind — it warns and points at the `--target release`
// re-heal, mirroring the forward path's post-push warn-only handling. It rides the
// Warn seam (no failure state), so the run finishes successfully.
func warnRegenerateProviderFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "publish failed",
		Message: "changelog is already pushed; re-heal the provider release with --target release",
		Output:  cause.Error(),
	})
}
