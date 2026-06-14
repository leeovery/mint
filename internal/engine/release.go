package engine

// This file is the engine's release ORCHESTRATOR — the Phase 1 walking-skeleton
// spine that threads every shipped unit (gitrepo, config, version, preflight,
// record, release, publish) into one runnable `mint release`. It owns ORDERING:
// the sequence in which the units run, when the presenter events fire, and the
// load-bearing point-of-no-return (PONR) asymmetry. The units themselves are
// unchanged — Release CALLS them; it never re-implements their logic.
//
// PONR ASYMMETRY (load-bearing): every failure in stages 1–8 BEFORE the atomic
// push succeeds aborts the run (surfaced via the presenter, non-zero exit) with
// nothing published. A publish failure AFTER a successful push is WARN-ONLY — the
// tag is already public, so the run does not unwind; it warns and exits 0.
//
// AUTO-UNWIND (Phase 2, made SURGICAL in Phase 4): a gate-no user abort and any
// POST-MUTATION pre-PONR failure share ONE recovery path — the surgical Unwind (task
// 4-2). The spine captures the clean StartState (HEAD + target tag) before the gate
// and TRACKS the MadeState as it proceeds (the count of commits mint made — an
// optional pre_tag artifact commit and/or the bookkeeping commit — and whether the
// annotated tag was created). On a gate-no or any pre-push failure it hands those
// captured inputs to Unwind, which resets EXACTLY the commits mint made and deletes the
// tag iff mint created it — no HEAD probe, no inference. The two triggers are
// deliberately identical: a declined gate and a rejected push with the same MadeState
// produce a byte-identical clean state and Unwound summary. With nothing made the
// surgical unwind no-ops (no reset, no Unwound). The PRE-mutation / preflight failures
// (before the StartState is captured) stay plain surface; there is nothing to unwind.
// All recovery mutations flow through the lock-resilient Mutator (4-1). --autostash
// pop-after ordering (4-4) layers on top of this wiring.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"mint/internal/aitransport"
	"mint/internal/config"
	"mint/internal/git"
	"mint/internal/gitrepo"
	"mint/internal/hooks"
	"mint/internal/notes"
	"mint/internal/notescache"
	"mint/internal/preflight"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/record"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

// releaseAction is the engine-supplied verb word for the start-of-run header.
const releaseAction = "releasing"

// Editor is the engine's edit seam: the `e` review-gate choice hands the current
// notes body to Edit and uses whatever it returns VERBATIM (no re-parse, no
// re-validation — a human edit is trusted). The interface is intentionally tiny so
// production can wire the real $EDITOR resolution (task 2-13) behind it while the
// gate-loop tests drive a scripted fake. It is defined HERE (the consumer) rather
// than where it is implemented, per the accept-interfaces convention.
type Editor interface {
	// Edit presents the current body to the user's editor and returns the saved
	// text. A non-nil error means no edit could be obtained (e.g. no editor on PATH);
	// the engine surfaces it and aborts rather than blocking on an unwired editor.
	Edit(ctx context.Context, current string) (string, error)
}

// Regenerator is the engine's regenerate seam: the `r` review-gate choice hands a
// ONE-TIME context line to Regenerate and uses the regenerated body it returns as
// the new notes body. The one-time context is TRANSIENT — it flows into this one
// AI call only and is NEVER persisted to [release].context. The interface is
// intentionally tiny so production (task 2-16) can wire the real AI path
// (notes.Generator.GenerateWithContext over the run's lastTag/cfg) behind it while
// the gate-loop tests drive a scripted fake. It is defined HERE (the consumer)
// rather than where it is implemented, per the accept-interfaces convention.
//
// It is consulted ONLY on `r`, which the gate offers ONLY for notes.KindNormalAI
// (the one path the AI produced the body), so the no-AI paths never reach it and
// may leave the seam nil.
type Regenerator interface {
	// Regenerate re-runs the normal AI path with oneTimeContext appended to the
	// prompt and returns the new body. An empty oneTimeContext is legal (regenerate
	// with no extra context). A non-nil error means the regeneration failed; the
	// engine surfaces it and aborts (fail-loud) rather than looping on a broken AI.
	Regenerate(ctx context.Context, oneTimeContext string) (string, error)
}

// regenContextPrompt is the AskLine label shown when the user chooses `r`: it asks
// for the one-time context nudge appended to the regeneration prompt. An empty
// answer is legal (regenerate with no extra context).
const regenContextPrompt = "Add one-time context for regeneration (optional):"

// ReleaseDeps bundles the orchestrator's injected seams so production wires the
// real implementations once (at the cmd entry point) and tests drive the whole
// spine with a RecordingPresenter + a single FakeRunner. The Releaser and
// Publisher each already hold the same runner; bundling them here keeps Release's
// signature short and the wiring explicit.
type ReleaseDeps struct {
	// Presenter is mint's single output/gate seam. Release emits lifecycle events
	// through it and never touches stdout/TTY directly.
	Presenter presenter.Presenter
	// Runner is the external-command seam the read-side units (gitrepo, version,
	// preflight, record reads, notes) issue git through unchanged — read-only git
	// calls do NOT go through the lock wrapper.
	Runner runner.CommandRunner
	// Mutator is the lock-resilient git MUTATION wrapper. Every git mutation the engine
	// drives — the record bookkeeping/artifact commits and the unwind's reset/tag-delete
	// — flows through it (retry on a contended lock, clear a provably-stale one). It is
	// constructed ONCE from the raw runner and shared with the Releaser (which wraps the
	// same Mutator for its tag + push). Read-only probes stay on Runner.
	Mutator *git.Mutator
	// Releaser performs the point-of-no-return tag + atomic push through the same Mutator.
	Releaser *release.Releaser
	// Editor is the OPTIONAL edit seam consulted ONLY on the `e` review-gate choice.
	// Runs that never reach `e` (every non-interactive and accept/abort path) may
	// leave it nil; production wires the real $EDITOR resolution (task 2-13). If `e`
	// is chosen with a nil Editor (a misconfiguration), the gate surfaces a clear
	// error and aborts rather than panicking.
	Editor Editor
	// Regenerator is the OPTIONAL regenerate seam consulted ONLY on the `r`
	// review-gate choice, which the gate offers ONLY for notes.KindNormalAI. Every
	// other path (the no-AI Kinds, every non-interactive and accept/abort path) may
	// leave it nil. In production it is left nil: Release builds a PER-RUN regenerator
	// closure that binds the run's lastTag + cfg to the resolved Generator (see
	// resolveBody). When non-nil it OVERRIDES that closure — the test-injection seam
	// the gate-loop `r` tests drive with a scripted fake. If `r` is chosen with neither
	// a wired Regenerator nor a Generator (a misconfiguration), the gate surfaces a
	// clear error and aborts rather than panicking.
	Regenerator Regenerator
	// Transport is the OPTIONAL AI transport seam the notes Generator hands its
	// composed prompt to. It exists so the prior-tag end-to-end tests can drive the
	// REAL notes path over the FakeRunner while still injecting a recording transport
	// where they need to script the AI body directly. When nil, Release builds the
	// production ai.Transport over deps.Runner once root is resolved, driving it with
	// the release verb's resolved command + timeout (cfg.AICommandFor / cfg.TimeoutFor
	// over the `[release] → shared → floor` chain) — config owns the default and the
	// blank-skip / no-deadline semantics, so production leaves it nil and gets the real
	// transport configured by the loaded schema.
	Transport notes.Transport
	// NoteCache is the dry-run note-cache seam — BOTH the WRITE side (task 4-7) and the
	// real-run REUSE/READ side (task 4-8). On a --dry-run that generated an AI note
	// preview, Release writes the generated body keyed by a hash of (the
	// post-diff_exclude diff + the computed version + the resolved prompt/context) with
	// a TTL stamp. On the REAL run Release recomputes the SAME key and Looks it up: a
	// live (within-TTL) match reuses the exact previewed bytes and SKIPS the AI; a miss
	// (or an expired entry) regenerates. The dry-run write is the SOLE filesystem side
	// effect of a dry run. When nil (the no-cache default used by tests that do not
	// assert caching) both the write and the reuse lookup are skipped (the real run
	// always generates); production wires a repo-path notescache.Store, and the cache
	// tests inject a temp-dir store so nothing lands in the real repo.
	NoteCache NoteCache
}

// NoteCache is the engine's dry-run note-cache seam: the WRITE side that persists a
// generated preview and the READ side that reuses it on the real run. It composes the
// segregated writer and reader interfaces, defined HERE (the consumer) per the
// accept-interfaces convention; notescache.Store satisfies it.
type NoteCache interface {
	NoteCacheWriter
	NoteCacheReader
}

// NoteCacheWriter is the engine's dry-run note-cache WRITE seam: it persists a
// generated note body under a precomputed key, scoped to a repo root, with its own
// TTL stamp. It is defined HERE (the consumer) per the accept-interfaces
// convention; notescache.Store satisfies it.
type NoteCacheWriter interface {
	// Write persists body under key for repoRoot. A non-nil error means the cache
	// entry could not be written; the dry run surfaces it (the cache write is the
	// dry run's only side effect, so a failure to perform it is worth reporting).
	Write(repoRoot, key, body string) error
	// Prune deletes every cached note for repoRoot except keepKey's, bounding the
	// per-project cache to the current diff's note so stale entries never accumulate.
	// Best-effort housekeeping — there is no error to handle (a leftover entry is
	// harmless; the cache is only an optimization).
	Prune(repoRoot, keepKey string)
}

// NoteCacheReader is the engine's real-run note-cache REUSE seam (task 4-8): it looks
// up a previously-written preview by its precomputed key, reporting found ONLY for a
// live (within-TTL) entry. The TTL check lives behind the seam (the store owns the
// clock), so an EXPIRED entry yields found=false and the real run regenerates rather
// than ever reusing a stale preview. It is defined HERE (the consumer) per the
// accept-interfaces convention; notescache.Store satisfies it.
type NoteCacheReader interface {
	// Lookup returns the cached entry for (repoRoot, key) and whether a FRESH one
	// exists. found is true ONLY for an entry within TTL; an absent or expired entry is
	// (zero, false, nil). A non-nil error is a genuine read/decode failure.
	Lookup(repoRoot, key string) (notescache.Entry, bool, error)
}

// ReleaseOptions carries the per-run parsed inputs. Bump selects the version
// segment to increment; Now is the injected release date (the production caller
// passes time.Now(), tests pass a fixed time) so the changelog header is
// deterministic — Release never calls time.Now() itself.
type ReleaseOptions struct {
	// Bump selects which version segment Next increments (default BumpPatch). It is
	// IGNORED when SetVersion is set — the two are mutually exclusive (the CLI rejects
	// combining --set-version with a bump flag before reaching the engine).
	Bump version.Bump
	// SetVersion is the raw --set-version value (e.g. "2.0.0" or "v2.0.0"): when
	// non-empty it PINS the next version outright, bypassing Next. The engine parses
	// it as strict 3-part SemVer (reusing the tag-grammar parser, prefix-tolerant per
	// regenerate's normalisation) and gates it strictly-greater than the current
	// latest tag — a backwards/equal jump is rejected even if the target tag is free.
	// On success the run's bump kind is explicit, so MINT_BUMP renders "explicit".
	// Empty (the default) selects the Bump path.
	SetVersion string
	// Now is the injected release timestamp used for the changelog date.
	Now time.Time
	// NotesBody is a TEST-INJECTION OVERRIDE for the resolved notes body. In
	// PRODUCTION it is empty: Release resolves the body via the notes-path precedence
	// (SelectBody). When NON-EMPTY it bypasses the selector and is used verbatim
	// alongside NotesKind — the seam the body-distribution / gate-loop tests drive to
	// pin a specific body without scripting the whole notes engine. Whatever value
	// resolves (override or selector), it flows WHOLE to the tag annotation, the
	// CHANGELOG section, and the provider release — no parsing, no splitting, no
	// per-sink reassembly.
	NotesBody string
	// NotesKind names which precedence path produced NotesBody — used ONLY alongside
	// the NotesBody test-injection override. When NotesBody is empty (production),
	// Release ignores this and uses the Kind SelectBody returns. It selects the review
	// gate variant: notes.KindNormalAI offers the four-choice y/n/e/r gate (the only
	// path with an AI to regenerate), while EVERY other Kind (first-release,
	// degenerate, --no-ai, fallback) offers the y/n/e variant with no `r`.
	NotesKind notes.Kind
	// NoAI is the --no-ai flag: a DELIBERATE skip of the AI path (after the
	// first-release and degenerate guards). It is threaded into the selector's
	// SelectState so the precedence routes to the commit-subject fallback body and
	// never calls the AI. It is irrelevant when NotesBody overrides the selector.
	NoAI bool
	// AutoStash is the --autostash escape hatch: when set, mint runs `git stash push
	// --include-untracked` BEFORE the clean-tree gate (so a dirty tree passes the gate)
	// and restores the WIP with `git stash pop` afterward — on success AND on
	// abort/failure. On an abort the restore is layered ON TOP of the surgical unwind:
	// the unwind returns the repo to its clean starting state first, THEN the pop applies
	// the WIP, so the WIP is never popped against mint's release commits. A no-WIP run is
	// a no-op (nothing stashed → nothing popped); a pop conflict leaves the stash intact
	// and warns rather than discarding the user's work. It is OPT-IN because the release
	// mutates the tree, so popping unrelated WIP can conflict — opting in is the user
	// asserting it is safe. All stash/pop ops flow through the lock-resilient Mutator.
	AutoStash bool
	// AnyBranch is the --any-branch escape hatch: when set, the on-release-branch
	// preflight gate is SKIPPED ENTIRELY (not evaluated — no `git rev-parse
	// --abbrev-ref HEAD` is issued) so a deliberate off-branch release proceeds. Every
	// OTHER gate (clean tree, tag-free local/remote, remote sync, gh auth) still runs
	// unchanged — this flag weakens ONLY the branch gate, nothing else. Without the flag
	// the branch gate runs exactly as the Phase 1 default (aborting off-branch). The
	// bypass is reported via the Presenter (a Warn) so an off-branch release is visible.
	// It composes with --autostash and the rest with no interaction.
	AnyBranch bool
	// DryRun is the --dry-run flag: a READ-ONLY run that prints the full plan and
	// touches nothing. When active the read-only stages run NORMALLY (preflight gates,
	// version determination, notes generation/preview), but every MUTATION is skipped:
	// each configured lifecycle hook (preflight/pre_tag/post_release) is reported-and-
	// skipped rather than run (the env still renders MINT_DRY_RUN=1 even though no hook
	// consumes it), and — at the dry-run boundary after the gate (4-7a) — the
	// version-file projection, the changelog write, the bookkeeping commit, the
	// annotated tag, the atomic push, and the provider release are ALL skipped so a dry
	// run NEVER reaches the lock-resilient Mutator and the repo is byte-for-byte
	// unchanged. The -d/--dry-run flag is wired in production (cmd/mint) and sets this
	// field. The ONE intentional side effect (task 4-7): after the notes PREVIEW is
	// generated, the generated AI note is WRITTEN to the gitignored/temp dry-run cache
	// (keyed by the post-diff_exclude diff + version + prompt/context) so the
	// subsequent real run can REUSE it (reuse itself is task 4-8).
	DryRun bool
}

// Release runs the Phase 1 first-release spine in strict order and returns nil on
// success. Any pre-push failure returns an *AbortError carrying a non-zero exit
// code (the failure is surfaced through the presenter first). A publish failure
// AFTER a successful push is warn-only — it surfaces a Warn and returns nil,
// because the tag is already public and mint never unwinds post-PONR.
//
// --autostash (4-4) layers into Stage 2: when set, mint stashes the working tree
// BEFORE the clean-tree gate (so the gate observes a clean tree) and restores it with
// a DEFERRED `git stash pop` that runs when Release returns. Because the surgical
// unwind runs inline before any abort return and the deferred pop runs after, the
// load-bearing unwind-then-pop ordering holds for EVERY abort path by construction: on
// a pre-PONR abort the repo is already back at its clean starting state by the time the
// pop applies the WIP on top. A no-WIP run is a no-op (nothing stashed → no deferred
// pop); a pop conflict warns and leaves the stash intact (never discarded).
func Release(ctx context.Context, deps ReleaseDeps, opts ReleaseOptions) error {
	p := deps.Presenter

	// Stage 1 — resolve root, load config, resolve branch, compute the tag.
	root, err := gitrepo.ResolveRoot(ctx, deps.Runner)
	if err != nil {
		return surface(p, "preflight", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return surface(p, "config", err)
	}

	releaseBranch, err := gitrepo.ResolveReleaseBranch(ctx, deps.Runner, cfg)
	if err != nil {
		return surface(p, "preflight", err)
	}

	current, err := version.CurrentVersion(ctx, deps.Runner, cfg.Release.TagPrefix)
	if err != nil {
		return surface(p, "version", err)
	}
	// Resolve the next version and its bump KIND from one of two paths: --set-version
	// PINS an explicit version (parsed strict + gated strictly-greater than current),
	// otherwise the bump flag COMPUTES it from current. A --set-version failure is a
	// pre-mutation "version" abort — nothing is tagged yet, so it is a plain surface
	// with nothing to unwind. The resolved bumpKind flows to every hook env so a pinned
	// run renders MINT_BUMP=explicit.
	next, bumpKind, err := resolveNextVersion(current, cfg.Release.TagPrefix, opts)
	if err != nil {
		return surface(p, "version", err)
	}
	tag := next.String(cfg.Release.TagPrefix)
	// versionKey is the bare next version (no tag prefix) — the changelog/notes key, the
	// RunStarted header version, AND the third cache-key component. It is computed HERE at
	// Stage 1 (the version is resolved) so RunStarted can lead the run; resolveBody later
	// reuses it to recompute the dry-run cache key for the real-run reuse lookup.
	versionKey := next.String("")

	// RunStarted OPENS the run: the brand header renders first, with every stage line
	// beneath it (the spec worked example, the presenter golden transcript). Its payload
	// depends ONLY on Stage-1 facts — the project name, the resolved version, the action
	// verb, and the commit_prefix leaf — none of which depend on the notes body, the
	// pre_tag hook, or preflight, so it leads the version/preflight/pre_tag/notes stage
	// events that follow. (ShowPlan/ShowNotes still fire after the notes body resolves.)
	p.RunStarted(presenter.RunInfo{
		Project: projectName(root),
		Version: versionKey,
		Action:  releaseAction,
		Leaf:    cfg.Release.CommitPrefix,
		DryRun:  opts.DryRun,
	})

	// Stage 1 — confirm the version before any work. A dry run takes no action, so it
	// asks for NOTHING: it skips this gate (and the notes-review gate below), previews,
	// and prints a "no changes made" summary. It still STATES the version it would cut,
	// as a read-only stage line, so the preview is faithful. A real run confirms here;
	// the gate runs before preflight and any mutation, so a "no" aborts cleanly with
	// nothing to unwind. -y auto-accepts (inside the presenter).
	if opts.DryRun {
		detail, sentence := versionPreview(current, next, cfg.Release.TagPrefix, bumpKind)
		emitGateSucceeded(p, "version", detail, sentence)
	} else {
		choice, derr := ReviewDecision(p, versionConfirmGate(current, next, cfg.Release.TagPrefix, bumpKind))
		if derr != nil {
			return derr
		}
		if choice == presenter.ChoiceNo {
			return abort(errReleaseDeclined)
		}
	}

	// Stage 2 — --autostash escape hatch: stash the working tree BEFORE the clean-tree
	// gate so a dirty tree passes (opt-in; without the flag a dirty tree still aborts at
	// the gate below). The DEFERRED pop restores the WIP when Release returns — on
	// success on top of the released state, on abort on top of the surgically-unwound
	// clean state. Deferring is load-bearing: the surgical unwind runs inline before any
	// abort return, so by the time this deferred pop fires the repo is already back at
	// its clean starting state — unwind-then-pop holds for every abort path. A no-WIP
	// tree stashes nothing (no pop is owed and none is deferred); a pop conflict warns
	// and leaves the stash intact.
	if opts.AutoStash {
		if autostashPush(ctx, deps) {
			defer autostashPop(ctx, deps)
		}
	}

	// Stage 2 — --any-branch escape hatch: report the branch-gate bypass so a
	// deliberate off-branch release is visible in the run. The skip itself happens in
	// runPreflight (the on-branch gate is not evaluated); this rides the Warn seam
	// (mirroring --autostash) which does not set failure state.
	if opts.AnyBranch {
		warnAnyBranchBypass(p)
	}

	// Stage 2 — preflight. Fetch first, then cheap local gates, then network gates.
	if err := runPreflight(ctx, deps.Runner, releaseBranch, tag, opts.AnyBranch); err != nil {
		return surface(p, "preflight", err)
	}

	// Stage 2 — project preflight hook. After mint's built-in gates pass and BEFORE
	// any mutation (and before startingHEAD is even captured), the project's optional
	// preflight hook runs for project-specific gates/validation. An absent hook
	// (cfg.Release.Hooks.Preflight == nil) is a no-op — the runner returns nil for a
	// nil/empty value. A non-zero exit (for an array, the first non-zero entry) aborts
	// the whole release cleanly: it is surfaced as a "preflight" StageFailed. Because
	// this precedes all mutation there is nothing to unwind — a plain surface, not the
	// auto-unwind path.
	if err := runPreflightHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun); err != nil {
		return surface(p, "preflight", err)
	}
	// Stage 2 narration: the preflight gates are read-only (no spinner), so emit a
	// completion line only — no StageStarted. Fired once both the built-in gates and
	// the project preflight hook have passed (Stage 2 complete).
	emitGateSucceeded(p, "preflight", preflightDetail(releaseBranch, tag, opts.AnyBranch), "Preflight passed: "+preflightDetail(releaseBranch, tag, opts.AnyBranch))

	// Capture the clean starting state NOW: preflight has just confirmed the tree is
	// clean and HEAD is resolvable, and nothing has mutated yet, so this HEAD is the
	// unambiguous reset target the surgical unwind returns to. It is captured BEFORE
	// the gate and any mutation so a gate-abort or any pre-PONR failure can reset back
	// to exactly here. The target tag is captured alongside it (and confirmed NOT to
	// pre-exist by preflight's tag-free gates) so the unwind deletes exactly the tag
	// mint would create. Failing to resolve HEAD is a plain preflight failure (no
	// mutation to unwind yet).
	startingHEAD, err := resolveHEAD(ctx, deps.Runner)
	if err != nil {
		return surface(p, "preflight", err)
	}
	start := StartState{HEAD: startingHEAD, Tag: tag, TagExisted: false}

	// Track what mint actually makes this run (NOT inferred by probing git): each
	// commit count is bumped as the step that made it runs, and TagCreated is set when
	// the annotated tag is created. The surgical unwind drives off this state — it
	// resets exactly made.Commits and deletes the tag iff made.TagCreated.
	made := MadeState{}

	// Stage 3 — project prep. The optional pre_tag hook builds/generates artifacts
	// (e.g. a knowledge bundle) and may dirty the tree; mint then commits whatever it
	// left dirty as its OWN `chore(release): pre-tag artifacts for {tag}` commit, kept
	// distinct from the bookkeeping commit. This runs AFTER startingHEAD so the
	// artifact commit is covered by the auto-unwind, and BEFORE notes so they generate
	// at the post-hook HEAD. An absent hook is a no-op (no prep, no artifact commit); a
	// non-zero exit aborts cleanly before any notes/tag/push — routed through the
	// surgical unwind so mint's OWN artifact commit (when one landed before the hook
	// step failed) is reset back to startingHEAD. The committed signal is folded into
	// made.Commits the moment the artifact commit lands, so it is tracked even if a
	// LATER stage fails.
	artifactCommitted, err := runPreTagHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun)
	if artifactCommitted {
		made.Commits++
	}
	if err != nil {
		return surfaceAndUnwind(ctx, deps, "pre_tag", start, made, err)
	}

	// Stage 4 — notes body. resolveBody runs the notes-path PRECEDENCE (SelectBody)
	// to pick the body + Kind for this run, building the selector from deps.Runner now
	// that root is resolved. opts.NotesBody is a test-injection override that bypasses
	// the selector. An on_notes_failure=abort failure aborts here. Nothing is tagged
	// yet, but a pre_tag artifact commit may already be in made, so this routes through
	// the surgical unwind (not a plain surface) — exactly like every other pre-push
	// failure — so that artifact commit is reset back to the clean start. Whatever
	// resolves flows WHOLE to every active sink below — no parsing, no per-sink reassembly.
	// (versionKey was computed at Stage 1 so RunStarted could lead the run; resolveBody
	// reuses it to recompute the dry-run cache key for the real-run reuse lookup.)
	//
	// Stage 4 is BLOCKING: generating notes can call the AI (~60s). Narrate it with a
	// blocking StageStarted (spinner) and a StageSucceeded carrying the engine-measured
	// Elapsed once the body resolves.
	notesDone := emitBlockingStageStarted(p, "notes", "generating release notes…", "Generated release notes")
	body, kind, generator, err := resolveBody(ctx, deps, root, cfg, current, versionKey, opts)
	if err != nil {
		return surfaceAndUnwind(ctx, deps, "notes", start, made, err)
	}
	notesDone("generated")

	// Show the plan, then the notes — identical in a dry run and a real run up to here;
	// the two diverge at the DRY-RUN BOUNDARY immediately below.
	p.ShowPlan(buildPlan(tag, cfg.Release.Publish))
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})

	// DRY-RUN BOUNDARY: a dry run takes no action, so it asks for nothing — it stops
	// HERE, before the review gate, having shown the full preview (version, plan, notes),
	// and prints the "no changes made" close-out instead of prompting, recording,
	// tagging, pushing, or publishing. The working tree is byte-for-byte unchanged
	// (hooks were skipped-and-reported above; the notes were generated read-only).
	if opts.DryRun {
		return finishDryRun(ctx, deps, cfg, root, versionKey, current, next, tag, bumpKind)
	}

	// The review gate is the FINAL gate of a real release: accepting it proceeds straight
	// through Record → tag → push → publish (there is no further confirmation), which is
	// why ReleaseReviewGate's wording says "release", not "accept". It may first EDIT (the
	// `e` choice) or REGENERATE (`r`, normal-AI only) the body; capture the returned final
	// body and thread IT to every downstream sink. The gate stays positioned BEFORE any
	// mutation (before Record). gateForKind selects the variant (y/n/e vs y/n/e/r); the
	// per-run regenerator binds this run's lastTag + cfg to the generator so an `r` re-runs
	// the SAME AI path SelectBody took.
	regenerator := perRunRegenerator(deps, generator, current.String(cfg.Release.TagPrefix), cfg)
	body, err = reviewGate(ctx, p, deps.Editor, regenerator, gateForKind(kind), body, versionKey)
	if err != nil {
		// A gate-no is a clean USER abort: route it through the SAME surgical unwind a
		// pre-push failure takes (reset any pre_tag artifact commit tracked in made;
		// narrate an identical Unwound). Every OTHER reviewGate error already surfaced its
		// own StageFailed and is returned as-is. (A dry run never reaches this gate — it
		// returned at the boundary above — so there is no DryRun branch here.)
		if errors.Is(err, errGateAborted) {
			return Unwind(ctx, deps, start, made, errGateAborted)
		}
		return err
	}

	// Stage 5 — record: project the version file, write the changelog, then fold BOTH
	// into one bookkeeping commit.
	//
	// The version-file projection runs FIRST (deliberate ordering): in embedded mode a
	// pattern that matches nothing is a fail-loud abort, and projecting first means that
	// mismatch fires BEFORE the changelog file is touched — so a clean abort leaves no
	// partial dirty changelog behind. The projection is filesystem-only (no git), so it
	// adds no git calls; its change is staged by CommitBookkeeping below. An empty
	// version_file is tag-only and no-ops (versionChanged false).
	versionChanged, err := record.ProjectVersionFile(root, cfg.Release.VersionFile, cfg.Release.VersionPattern, versionKey)
	if err != nil {
		// Post-mutation, pre-PONR: route through the surgical unwind so any pre_tag
		// artifact commit already in made is reset back to the clean start. The
		// version-file projection is a filesystem write that made no commit, so made is
		// unchanged here; the unwind's reset (when made.Commits>0) discards the projection
		// along with the artifact commit. No tag exists yet (made.TagCreated false).
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}

	// When changelog=false WriteChangelog no-ops (Changed:false). When the version file
	// also nets no change, CommitBookkeeping makes NO commit — the tag then points at the
	// existing HEAD (or any pre_tag artifact commit) and STILL carries the full body via
	// TagAndPush. Nothing durable is lost.
	writeResult, err := record.WriteChangelog(root, versionKey, opts.Now, body, cfg.Release.Changelog)
	if err != nil {
		// Post-mutation, pre-PONR: route through the surgical unwind so any artifact commit
		// (and a half-written changelog discarded by the reset) is rolled back to the clean
		// start — the abort path is identical to the pre-push failure path. No tag yet.
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}

	// ONE folded bookkeeping commit stages the changelog and the version file together —
	// the version file is never given its own separate commit. Kept DISTINCT from the
	// pre_tag artifact commit (3-3), which precedes it. It commits IFF something net-
	// changed (the changelog and/or the version file); the SHARED record predicate —
	// the same rule CommitBookkeeping's no-op branch consumes — tells the spine whether
	// HEAD just moved, so made.Commits is bumped exactly when a commit landed —
	// tracked, not inferred by probing git.
	bookkeepingCommitted := record.BookkeepingWillCommit(cfg.Release.VersionFile, writeResult.Changed, versionChanged)
	if err := record.CommitBookkeeping(ctx, deps.Mutator, root, cfg.Release.CommitPrefix, tag, cfg.Release.VersionFile, writeResult.Changed, versionChanged); err != nil {
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}
	if bookkeepingCommitted {
		made.Commits++
	}
	// Stage 5 narration: the ✓ line names WHAT was just recorded (the artifacts the
	// bookkeeping commit carried) rather than a bare stage codeword, so the user can
	// follow the run without knowing mint's stage taxonomy.
	emitGateSucceeded(p, "record", recordDetail(writeResult.Changed, versionChanged, bookkeepingCommitted), recordSentence(writeResult.Changed, versionChanged, bookkeepingCommitted))

	// Stage 6/7 — resolve the publishing driver, then run its conditional gh gate,
	// only when publishing and BEFORE the tag.
	//
	// The Phase 1/2 hardwired "always GitHub when publish=true" selection is gone:
	// the driver is now AUTO-DETECTED from the release remote's host (overridable by
	// [release].provider), so a github.com remote — HTTPS or SSH — resolves to the
	// GitHub driver behind the Publisher interface, and a future GitLab/Gitea driver
	// slots in with no change here. The resolved publisher is held as the interface
	// type and used UNCHANGED below for CreateRelease — the orchestrator never names a
	// concrete driver.
	//
	// The gh install/auth gate is conditional on a driver actually being SELECTED and
	// on publishing proceeding: it runs only for the resolved driver, before the tag,
	// so a missing/unauthenticated gh never strands a pushed tag waiting on a release
	// it cannot create.
	//
	// SAFE DOWNGRADE (4-10): an UNRESOLVED provider (non-github.com host, unsupported
	// value, no remote, or an unparseable SSH URL) is NOT an abort. resolvePublisher
	// returns ErrProviderUnresolved (a nil Publisher); the spine WARNS loudly — naming
	// the reason — and DOWNGRADES the run to tag + push ONLY: publisher stays nil, so
	// the gh gate below is skipped (it gates a selected driver, of which there is none)
	// and the Stage-7 CreateRelease is skipped too. The annotated tag and the atomic
	// push still happen, so the pushed tag is never stranded (publishing was simply
	// never attempted). mint NEVER silently assumes GitHub for an unresolved provider.
	// This is DISTINCT from publish=false: an explicit opt-out is a SILENT tag + push
	// (this whole block is skipped), not a warned downgrade. Any OTHER resolution error
	// remains a pre-PONR "preflight" abort routed through the surgical unwind.
	var publisher publish.Publisher
	if cfg.Release.Publish {
		resolved, err := resolvePublisher(ctx, deps, cfg)
		switch {
		case errors.Is(err, publish.ErrProviderUnresolved):
			warnPublishDowngraded(p, err)
		case err != nil:
			return surfaceAndUnwind(ctx, deps, "preflight", start, made, err)
		default:
			publisher = resolved
		}

		// The gh gate gates ONLY an actually-selected driver: on a downgrade publisher is
		// nil, so it is never reached — keeping the pushed tag from being stranded.
		if publisher != nil {
			if err := preflight.CheckGhAuth(ctx, deps.Runner); err != nil {
				// The bookkeeping commit may have moved HEAD; the surgical unwind resets exactly
				// the commits in made. No tag yet (made.TagCreated false).
				return surfaceAndUnwind(ctx, deps, "preflight", start, made, err)
			}
		}
	}

	// Stage 6 — tag + atomic push.
	//
	// COMMIT-GRAPH INVARIANT (assembled across stages 3 and 5): this run produces UP
	// TO TWO commits before the tag — an OPTIONAL pre_tag hook-artifact commit
	// (`chore(release): pre-tag artifacts for {tag}`, Stage 3, made only when the hook
	// dirtied the tree) followed by an OPTIONAL release-bookkeeping commit
	// (`{commit_prefix} Release {tag}`, Stage 5, made only when the changelog and/or
	// version file netted a change). The annotated tag conceptually points at the
	// release-bookkeeping commit; because TagAndPush creates it with `git tag -a {tag}`
	// (which tags the CURRENT HEAD) and HEAD EQUALS the bookkeeping commit whenever one
	// was made, the tag is correct by construction — no explicit target is passed. When
	// only the hook-artifact commit was made the tag sits on it; when neither commit was
	// made the tag sits on the pre-existing HEAD. NO-OP SAFETY: zero, one, or two commits
	// are all valid and no empty commit is ever created (CommitDirtyTree and
	// CommitBookkeeping each commit only when there is a real change). The single atomic
	// push (`git push --atomic origin HEAD {tag}`) sends whatever commits HEAD carries
	// together with the tag — the one point of no return for the whole graph.
	//
	// A nil error means the atomic push succeeded and PointOfNoReturnCrossed is set:
	// from here the tag is public, so any later failure is warn-only and the run must
	// NOT unwind.
	//
	// LAST PRE-PONR GATE: a SIGINT/SIGTERM cancellation observed in the window between
	// the bookkeeping commit and the atomic push (Ctrl-C with no subprocess running, so
	// no command-level error surfaces it) is caught HERE and routed through the SAME
	// surgical unwind every pre-push failure takes — resetting the tracked commit(s),
	// deleting nothing (no tag yet), and letting the deferred autostash pop apply on top
	// of the clean state. The push is NOT attempted, so the warn-only post-PONR contract
	// is never entered. A cancellation DURING a pre-PONR subprocess (the AI call, a hook)
	// already surfaces as that command's error and routes through the same unwind above;
	// this gate closes the remaining no-subprocess gap.
	if err := ctx.Err(); err != nil {
		return surfaceAndUnwind(ctx, deps, "tag", start, made, err)
	}

	// Stage 6 is BLOCKING: the tag + atomic push round-trips the network. Narrate it
	// with a blocking StageStarted (spinner) and a StageSucceeded carrying the
	// engine-measured Elapsed once the push crosses the PONR. On failure the
	// StageFailed surfaced by surfaceAndUnwind narrates the stage instead, so no
	// StageSucceeded fires.
	pushDone := emitBlockingStageStarted(p, "push", "pushing branch and tag to origin…", "Pushed branch + "+tag+" atomically")
	if _, err := deps.Releaser.TagAndPush(ctx, tag, cfg.Release.CommitPrefix, body); err != nil {
		// Pre-PONR failure: route through the surgical unwind. A push REJECTION means the
		// local tag WAS created (TagAndPush wraps it in release.ErrPushRejected), so the
		// unwind must delete it; a tag-CREATION failure leaves no tag, so only the tracked
		// commits are reset. Either way the reset is driven by made.Commits — the exact
		// count mint made this run — not a HEAD probe.
		made.TagCreated = errors.Is(err, release.ErrPushRejected)
		return surfaceAndUnwind(ctx, deps, "tag", start, made, err)
	}
	pushDone("branch + " + tag + " pushed atomically")

	// Stage 7 — publish. Post-PONR: a publish failure is WARN-ONLY (the tag is
	// already public); the run does not unwind and exits successfully. The release is
	// created through the resolved Publisher INTERFACE (whichever driver detection
	// picked), never a concrete type. It is gated on a publisher actually being
	// SELECTED, not merely on publish=true: a safe downgrade (provider unresolved)
	// leaves publisher nil and already warned above, so publishing is skipped here —
	// the run is tag + push only.
	releaseURL := ""
	if publisher != nil {
		// CreateRelease returns the published release URL (parsed from the driver's
		// stdout); thread it into RunResult.URL so the success footer renders the real
		// URL. A publish FAILURE is warn-only and leaves releaseURL empty (no bogus URL
		// in the footer); a downgrade leaves publisher nil so releaseURL stays empty too.
		url, err := publisher.CreateRelease(ctx, tag, tag, body)
		if err != nil {
			warnPublishFailed(p, err)
		} else {
			releaseURL = url
			// Post-PONR success narration: the release exists on the provider now.
			emitGateSucceeded(p, "publish", "release published", "Published the release")
		}
	}

	// Stage 7 — post_release hook. Post-PONR follow-ups (notifications, tap
	// repository_dispatch). It runs UNCONDITIONALLY: reaching here means the push
	// already crossed the PONR, so the tag is public whether publish=true or
	// publish=false, and post-release follow-ups apply either way. Like the publish
	// failure above, a non-zero exit is WARN-ONLY — it does NOT abort or unwind; the
	// run still reaches RunFinished and returns nil. This is the ONLY hook point whose
	// failure is non-fatal (preflight and pre_tag abort); the array stop-on-first-
	// failure SEQUENCING is identical across points — only the CONSEQUENCE differs.
	if err := runPostReleaseHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun); err != nil {
		warnPostReleaseFailed(p, err)
	}

	p.RunFinished(presenter.RunResult{
		Project: projectName(root),
		Version: versionKey,
		URL:     releaseURL,
		Leaf:    cfg.Release.CommitPrefix,
	})
	return nil
}

// errGateAborted is the cause for a clean gate-no abort: the user declined the
// review gate, so the run stops with a non-zero exit but no underlying failure.
var errGateAborted = errors.New("release aborted at the review gate")

// errReleaseDeclined is the cause for a clean decline at the version-confirmation
// gate: the user said no to the bump before any work began. Nothing has mutated, so
// there is nothing to unwind — it is a plain non-zero abort.
var errReleaseDeclined = errors.New("release declined at the version gate")

// versionConfirmGate builds the version-confirmation gate shown before any work: it
// states the current→next version and the bump kind and asks the user to confirm.
// A first release (current 0.0.0) reads "Release {next} (first release)?"; otherwise
// "Release {current} → {next} ({bump})?". Subject/AcceptEcho drive the -y auto-accept
// echo ("version: {next} (-y)"). It declares y/n only (no edit/regenerate) and
// defaults to yes (a bare Enter proceeds).
func versionConfirmGate(current, next version.SemVer, prefix string, bumpKind version.Bump) presenter.Gate {
	nextDisplay := next.String(prefix)
	question := fmt.Sprintf("Release %s → %s (%s)?", current.String(prefix), nextDisplay, hookBump(bumpKind))
	if current.String("") == "0.0.0" {
		question = fmt.Sprintf("Release %s (first release)?", nextDisplay)
	}
	return presenter.Gate{
		Question:   question,
		Subject:    "version",
		AcceptEcho: nextDisplay,
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "yes"},
			{Key: presenter.ChoiceNo, Action: "no"},
		},
		Default: presenter.ChoiceYes,
	}
}

// versionPreview builds the dry-run "version" stage line's (detail, sentence): the
// read-only counterpart to versionConfirmGate, stating the version a real run would
// cut without asking for consent (a dry run prompts for nothing). detail is the terse
// plain value ("v1.4.0 (minor bump)"); sentence is the pretty narration
// ("Would release v1.4.0 (minor bump from v1.2.0)"). A first release (current 0.0.0)
// reads "(first release)" with no from-version, mirroring the gate's first-release
// wording.
func versionPreview(current, next version.SemVer, prefix string, bumpKind version.Bump) (detail, sentence string) {
	nextDisplay := next.String(prefix)
	if current.String("") == "0.0.0" {
		return fmt.Sprintf("%s (first release)", nextDisplay),
			fmt.Sprintf("Would release %s (first release)", nextDisplay)
	}
	bump := hookBump(bumpKind)
	return fmt.Sprintf("%s (%s bump)", nextDisplay, bump),
		fmt.Sprintf("Would release %s (%s bump from %s)", nextDisplay, bump, current.String(prefix))
}

// resolveNextVersion picks this run's next version and its bump KIND from one of two
// mutually-exclusive paths:
//
//   - --set-version (opts.SetVersion non-empty) PINS the version: the value is parsed
//     as strict 3-part SemVer (version.ParseSemVer, reusing the tag-grammar parser and
//     tolerating a leading prefix like regenerate's <version>) and then gated
//     STRICTLY-GREATER than current. The strictly-greater gate sits ON TOP of the
//     free-tag preflight check: a backwards or equal jump is rejected here even when
//     the target tag does not exist, because a lower/equal version sorts at-or-below
//     "latest" and corrupts tag-as-truth. There is deliberately no --force downgrade
//     override (YAGNI). On success the kind is version.BumpExplicit, so MINT_BUMP
//     renders "explicit".
//   - otherwise the version is COMPUTED with version.Next(current, opts.Bump) and the
//     kind is opts.Bump (patch/minor/major) unchanged.
//
// A returned error is a pre-mutation "version" failure for the caller to surface.
func resolveNextVersion(current version.SemVer, prefix string, opts ReleaseOptions) (version.SemVer, version.Bump, error) {
	if opts.SetVersion == "" {
		return version.Next(current, opts.Bump), opts.Bump, nil
	}

	pinned, err := version.ParseSemVer(opts.SetVersion, prefix)
	if err != nil {
		return version.SemVer{}, 0, err
	}
	if !pinned.GreaterThan(current) {
		return version.SemVer{}, 0, fmt.Errorf(
			"--set-version %s must be greater than the current latest version %s",
			pinned.String(""), current.String(""),
		)
	}
	return pinned, version.BumpExplicit, nil
}

// resolveBody resolves this run's notes body + Kind via the notes-path precedence
// (SelectBody), returning the generator it built so the caller can bind a per-run
// regenerator for the gate's `r` choice. It is the single Stage-4 body decision:
//
//   - opts.NotesBody is a TEST-INJECTION OVERRIDE: when non-empty it bypasses the
//     selector and returns (opts.NotesBody, opts.NotesKind, nil generator, nil). The
//     gate's `r` is never reached for the no-AI Kinds these overrides use, so a nil
//     generator is fine; a normal-AI override test supplies its own deps.Regenerator.
//   - otherwise the selector is built from deps.Runner (now that root is resolved) and
//     SelectBody runs the precedence over the run's SelectState. A SelectBody error is
//     an on_notes_failure=abort notes failure: it returns the error for the caller to
//     surface and abort on, BEFORE any mutation.
//
// FirstRelease is detected by comparing current to the zero SemVer — a tagless repo
// resolves to {0,0,0}. An actual v0.0.0 tag is therefore treated as a first release;
// that edge is acceptable for Phase 2 (a real v0.0.0 release is not a meaningful case
// to support, and the selector simply records "Initial release." for it).
func resolveBody(ctx context.Context, deps ReleaseDeps, root string, cfg config.Config, current version.SemVer, versionKey string, opts ReleaseOptions) (string, notes.Kind, *notes.Generator, error) {
	if opts.NotesBody != "" {
		return opts.NotesBody, opts.NotesKind, nil, nil
	}

	// One Assembler (the single git seam) is shared by the Generator and the Selector
	// so the degenerate-check diff and the AI path range over the same git, exactly as
	// NewSelector documents. The consolidated ExcludeConfig threads BOTH the configured
	// diff_exclude globs AND the strategy-aware version_file decision here, so the per-run
	// diff assembly and the Change Map apply the union of tiers ON TOP OF CHANGELOG.md.
	// On the forward path the version_file rule is inert (notes generate at Stage 4
	// precedes the version write at Stage 5, so the file is unchanged at notes time) — the
	// decision is still computed so the regenerate path (Phase 5) inherits it correctly.
	assembler := notes.NewAssembler(deps.Runner, notes.ExcludeConfig{
		Globs:          cfg.DiffExclude,
		VersionFile:    cfg.Release.VersionFile,
		VersionPattern: cfg.Release.VersionPattern,
	})
	generator := notes.NewGenerator(assembler, aiTransport(deps, cfg), root)
	selector := notes.NewSelector(generator, assembler, deps.Runner, root)

	state := notes.SelectState{
		FirstRelease: current == version.SemVer{},
		LastTag:      current.String(cfg.Release.TagPrefix),
		NoAI:         opts.NoAI,
	}
	// SelectBodyWithReuse runs the precedence and surfaces, for the normal-AI path, the
	// post-diff_exclude diff + resolved prompt/context the cache key hashes (assembled
	// ONCE here). It consults the reuse hook BEFORE the AI — a live cache match skips the
	// AI and reuses the cached body. The hook differs by mode: a real run PROMPTS
	// (use/regenerate) on a match, a dry run reuses SILENTLY (a dry run asks for nothing).
	// reused records whether the body came from the cache so the write below skips a
	// no-op re-write.
	reused := false
	body, kind, cacheInputs, err := selector.SelectBodyWithReuse(ctx, state, cfg, cacheReuse(deps, root, versionKey, opts.DryRun, &reused))
	if err != nil {
		return "", kind, nil, err
	}
	// Cache a freshly-generated, cacheable (normal-AI) body so a LATER run for the SAME
	// diff can reuse it (a dry run caches its preview so a following real run reuses it).
	// A reused body is already cached, so it is not re-written; a write failure is
	// warn-only (the cache is an optimization, never load-bearing).
	if cacheInputs.Cacheable && deps.NoteCache != nil {
		key := notescache.Key(cacheInputs.Diff, versionKey, cacheInputs.Instructions)
		if !reused {
			if werr := deps.NoteCache.Write(root, key, body); werr != nil {
				warnNoteCacheFailed(deps.Presenter, werr)
			}
		}
		// Housekeeping: keep only the current diff's note, pruning entries left by earlier
		// runs (older diffs/versions) so the per-project cache never accumulates.
		deps.NoteCache.Prune(root, key)
	}
	return body, kind, generator, nil
}

// cacheReuse builds the notes.ReuseFunc the selector consults on the NORMAL-AI path to
// reuse a previously cached note (typically one a prior dry run wrote), or nil when no
// cache is wired. The hook recomputes the SAME cache key the entry was written under
// (the post-diff_exclude diff + the bare version + the resolved prompt/context) and
// Looks it up:
//
//   - a clean MISS, an expired entry, or a READ/DECODE error (Lookup found=false, or a
//     non-nil err from a corrupt/partial entry or a permissions glitch) → return
//     reused=false so the selector regenerates fresh via the AI. There is nothing to
//     reuse, so this is SILENT — the "generating release notes…" spinner is the only
//     narration needed (a changed diff is self-evident: the code changed). A stale or
//     corrupt body is never shipped (the AI regenerates; the fresh write overwrites a
//     corrupt entry). The cache is purely an optimization, so a read failure must NEVER
//     abort the release.
//   - a live (within-TTL) match → reuse the cached body (reused=true), so the selector
//     SKIPS the AI. The two modes differ ONLY here: a real run PROMPTS first
//     (use/regenerate — the spinner is suspended around reuseConfirmGate, so "regenerate"
//     re-runs the AI under a live spinner; under -y the gate auto-accepts "use"), while a
//     dry run reuses SILENTLY (a dry run asks for nothing — it is a no-action preview).
func cacheReuse(deps ReleaseDeps, root, versionKey string, dryRun bool, reused *bool) notes.ReuseFunc {
	// Guard: a nil NoteCache (no cache wired) yields a nil hook — always generate.
	if deps.NoteCache == nil {
		return nil
	}
	return func(diff, instructions string) (string, bool, error) {
		key := notescache.Key(diff, versionKey, instructions)
		entry, found, err := deps.NoteCache.Lookup(root, key)
		if err != nil || !found {
			return "", false, nil
		}
		// A live entry matches this exact diff. A dry run reuses it without asking — a
		// preview prompts for nothing.
		if dryRun {
			*reused = true
			return entry.Body, true, nil
		}
		// A real run asks use/regenerate: the spinner is suspended for the prompt and
		// resumed after (so "regenerate" runs the AI under a live spinner; "use" closes
		// out moments later). Under -y the gate auto-accepts the default — use the cache.
		pr := deps.Presenter
		pr.SuspendSpinner()
		choice, derr := ReviewDecision(pr, reuseConfirmGate())
		pr.ResumeSpinner()
		if derr != nil {
			return "", false, derr
		}
		if choice == presenter.ChoiceRegen {
			return "", false, nil
		}
		*reused = true
		return entry.Body, true, nil
	}
}

// reuseConfirmGate is the real-run prompt shown when a dry-run preview matches the
// current diff: use the cached notes, or regenerate fresh. Default is "use" (a bare
// Enter, and the -y auto-accept, reuse the preview). It declares y/r only — there is
// no edit here (the notes-review gate downstream owns editing).
func reuseConfirmGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Cached notes found for this diff — use them?",
		Subject:    "notes",
		AcceptEcho: "reused",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceYes, Action: "use cached"},
			{Key: presenter.ChoiceRegen, Action: "regenerate"},
		},
		Default: presenter.ChoiceYes,
	}
}

// aiTransport resolves the AI transport the notes Generator hands its prompt to:
// the injected deps.Transport when set (the test seam), else the production
// ai.Transport over the run's runner — so production leaves deps.Transport nil and
// gets the real transport. The production construction goes through the shared
// aitransport.New helper with config.VerbRelease, so the release verb's per-key
// resolution supplies BOTH the concrete command and the per-attempt deadline (the
// `[release] → shared → floor` chain, and the never-zero-by-omission timeout contract,
// live once in that helper). The injected-transport short-circuit is this site's own
// (the deps wrapper type differs per site) and stays here — only the construction
// expression is shared.
func aiTransport(deps ReleaseDeps, cfg config.Config) notes.Transport {
	if deps.Transport != nil {
		return deps.Transport
	}
	return aitransport.New(deps.Runner, cfg, config.VerbRelease)
}

// regeneratorFunc adapts a plain regenerate closure to the Regenerator seam so the
// per-run AI regenerator (which must bind this run's lastTag + cfg, known only at run
// time) can be expressed without a dedicated type at the call site.
type regeneratorFunc func(ctx context.Context, oneTimeContext string) (string, error)

func (f regeneratorFunc) Regenerate(ctx context.Context, oneTimeContext string) (string, error) {
	return f(ctx, oneTimeContext)
}

// perRunRegenerator selects the Regenerator the gate's `r` choice consults. A wired
// deps.Regenerator OVERRIDES everything (the test-injection seam). Otherwise, when a
// generator was built (the normal-AI selector path), it returns a per-run closure
// binding lastTag + cfg to generator.GenerateWithContext — so an `r` re-runs the SAME
// AI path SelectBody took, with the one-time context appended. When neither is present
// (an override body, or a no-AI path with no generator), it returns nil; `r` is only
// offered for KindNormalAI, so the nil is never consulted on the no-AI paths.
func perRunRegenerator(deps ReleaseDeps, generator *notes.Generator, lastTag string, cfg config.Config) Regenerator {
	if deps.Regenerator != nil {
		return deps.Regenerator
	}
	if generator == nil {
		return nil
	}
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		return generator.GenerateWithContext(ctx, lastTag, cfg, oneTimeContext)
	})
}

// resolveHEAD reads the current commit SHA via `git rev-parse HEAD` through the
// runner seam (every git op goes through the seam). It captures the clean starting
// state before the gate — the exact sha the surgical unwind resets back to. The
// surgical unwind no longer probes HEAD (it drives off the tracked MadeState), so
// this is the run's single rev-parse HEAD.
func resolveHEAD(ctx context.Context, r runner.CommandRunner) (string, error) {
	res, err := r.Run(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// resolvePublisher selects the publishing driver for this run: it reads the
// release remote's URL through the runner seam and hands it (with the optional
// [release].provider override) to publish.ResolvePublisher, which parses the host
// across the HTTPS/SSH URL forms and picks the driver. It returns the driver
// behind the Publisher interface so the orchestrator never names a concrete type.
//
// A missing remote (`git remote get-url origin` exits non-zero) is treated as an
// EMPTY remote URL rather than a hard error here, so the resolver's no-remote
// outcome flows through the SAME ErrProviderUnresolved path as a non-matching
// host — one unresolved sentinel the spine downgrades on (warn + tag + push only).
func resolvePublisher(ctx context.Context, deps ReleaseDeps, cfg config.Config) (publish.Publisher, error) {
	return publish.ResolvePublisher(RemoteURL(ctx, deps.Runner), cfg.Release.Provider, deps.Runner)
}

// ResolvePublisher is the SHARED publisher-resolution entry the regenerate cmd paths
// (single-version and --all) call instead of discarding the resolver error. It performs
// the SAME safe-downgrade branching the forward engine.Release Stage-6 applies, so the
// regenerate paths can never again pass a nil Publisher down to DispatchRelease and crash:
//
//   - provider RESOLVED: returns the driver behind the Publisher interface, nil error.
//   - provider UNRESOLVED (ErrProviderUnresolved — a non-github.com host, an unsupported
//     value, no remote, or an unparseable SSH URL): WARNS loudly (naming the reason) and
//     returns a nil Publisher with NO error — the run proceeds DOWNGRADED, exactly as the
//     forward path downgrades to tag + push only. The downstream regenerate write nil-
//     guards the publisher so the provider surface is simply skipped.
//   - any OTHER resolution error: surfaces it as a pre-mutation regenerate abort (the
//     "preflight" stage, matching the regenerate preflight failures) and returns it so the
//     cmd layer maps it to a non-zero exit.
//
// It mirrors the forward spine's choice precisely (warn-and-downgrade vs abort) so the
// forward and regenerate paths handle an unresolvable provider identically.
func ResolvePublisher(ctx context.Context, deps ReleaseDeps, cfg config.Config) (publish.Publisher, error) {
	publisher, err := resolvePublisher(ctx, deps, cfg)
	switch {
	case errors.Is(err, publish.ErrProviderUnresolved):
		warnPublishDowngraded(deps.Presenter, err)
		return nil, nil
	case err != nil:
		return nil, surface(deps.Presenter, regeneratePreflightStage, err)
	default:
		return publisher, nil
	}
}

// RemoteURL reads the release remote's URL via `git remote get-url origin` through
// the runner seam. A non-zero exit (no `origin` remote configured) yields the empty
// string, which the resolver treats as "no remote" — an unresolved outcome rather
// than a fatal git error, so the no-remote case joins the other unresolved cases.
//
// It is exported as the single owned reader the cmd regenerate path also calls, so
// the forward and regenerate publisher resolution share one "empty == unresolved,
// downgrade rather than fail" implementation rather than copied literals.
func RemoteURL(ctx context.Context, r runner.CommandRunner) string {
	res, err := r.Run(ctx, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

// surfaceAndUnwind handles a post-mutation, pre-PONR STAGE failure: it surfaces the
// StageFailed first (so the failed stage is shown) and then routes through the SAME
// surgical Unwind (4-2) the gate-no path takes, so the abort path after a commit is
// identical to the pre-push failure path — exactly the tracked commits are reset and
// the tag deleted iff made.TagCreated. It is the stage-failure sibling of the gate-no
// path, which calls Unwind directly with no StageFailed.
func surfaceAndUnwind(ctx context.Context, deps ReleaseDeps, stage string, start StartState, made MadeState, cause error) error {
	deps.Presenter.StageFailed(presenter.StageFailure{
		Name:    stage,
		Message: failureMessage(cause),
	})
	return Unwind(ctx, deps, start, made, cause)
}

// runPreflight runs the Stage 2 gate chain in the spec's order: fetch (read-only,
// refreshes tags + upstream refs), then the cheap local gates, then the network
// gates. The first failure short-circuits and is returned for the caller to
// surface and abort on. The conditional gh gate is run separately by Release
// (only when publishing, after the bookkeeping commit and before the tag).
//
// anyBranch is the --any-branch escape hatch: it is passed to RunLocalGates, which
// SKIPS the on-release-branch gate when it is true (the gate is not evaluated). It
// affects ONLY the branch gate — fetch, clean-tree, tag-free, and remote-sync all
// run unchanged.
func runPreflight(ctx context.Context, r runner.CommandRunner, releaseBranch, tag string, anyBranch bool) error {
	if err := preflight.Fetch(ctx, r); err != nil {
		return err
	}
	if err := preflight.RunLocalGates(ctx, r, releaseBranch, tag, anyBranch); err != nil {
		return err
	}
	if err := preflight.CheckRemoteSync(ctx, r, releaseBranch); err != nil {
		return err
	}
	return preflight.CheckTagFreeRemote(ctx, r, tag)
}

// dryRunLabel is the Presenter Warning label every dry-run hook-skip notice rides.
// The skip notices use the existing Warn seam (the available out-of-band, non-failure
// notice): a Warn does NOT set failure state or suppress the success line, so a
// reported skip leaves the run otherwise intact. No NEW presenter event is added.
const dryRunLabel = "dry-run"

// reportHookSkipped emits the dry-run hook-skip notice for a CONFIGURED point that
// was not invoked because dry-run is active. The message is "skipping {point} hook"
// (point = preflight / pre_tag / post_release), kept identical across the three call
// sites so the convention is consistent. It rides the Warn seam (see dryRunLabel).
func reportHookSkipped(p presenter.Presenter, point string) {
	p.Warn(presenter.Warning{Label: dryRunLabel, Message: "skipping " + point + " hook"})
}

// runPreflightHook runs the project's optional preflight hook through the shared
// hooks runner. It assembles the MINT_* env from the run's computed versions
// (NewVersion = the bare version being released, PreviousVersion = the bare prior
// latest, VersionTag = the full prefixed tag), the bump kind, and the dryRun mode,
// then runs the configured [release.hooks].preflight value from the repo root. An
// absent value (nil) is a no-op — the runner returns nil. A non-zero exit (the first
// non-zero entry for an array) returns a non-nil error the caller surfaces and aborts
// on.
//
// DRY-RUN: when dryRun is active AND the hook is configured (non-nil), the hook is
// NOT invoked — no `sh -c …` reaches the runner; instead the skip is REPORTED via the
// presenter and nil is returned. An ABSENT hook stays a silent no-op (no run, no
// report). The env is still assembled with MINT_DRY_RUN=1 for consistency even though
// the skipped hook never consumes it.
func runPreflightHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool) error {
	if dryRun {
		if cfg.Release.Hooks.Preflight != nil {
			reportHookSkipped(deps.Presenter, "preflight")
		}
		return nil
	}
	env := buildHookEnv(current, next, tag, bump, dryRun)
	return hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.Preflight, root, env)
}

// runPreTagHook runs the project's optional Stage-3 pre_tag hook and then applies
// the artifact-commit interplay rule. The hook (build/generate artifacts) runs
// through the shared hooks runner with the same MINT_* env as the preflight hook. An
// absent value (nil) is a no-op — the runner returns nil, no artifact commit is
// considered, and committed is false. A non-zero exit (the first non-zero entry for an
// array) returns a non-nil error the caller routes through the surgical unwind.
//
// On hook SUCCESS, mint commits whatever the hook left dirty as its OWN commit
// (subject `chore(release): pre-tag artifacts for {tag}` — a FIXED chore prefix, NOT
// the configurable commit_prefix), via record.CommitDirtyTree: a clean tree (empty
// `git status --porcelain`) commits nothing — which covers a hook that built nothing,
// a hook that made its own commit, and gitignored-only outputs alike. The returned
// committed reports whether an artifact commit actually landed, so the caller can fold
// it into MadeState for the surgical unwind (CommitDirtyTree returns committed=true
// ONLY after a successful commit, so committed is never true alongside a non-nil
// error). A commit failure is surfaced for the caller to unwind. The interplay rule
// applies ONLY after a hook actually ran: an absent hook skips the artifact-commit
// probe entirely, so the existing no-hook spine is untouched.
//
// DRY-RUN: when dryRun is active AND the hook is configured, the hook is NOT invoked
// and — crucially — the artifact-commit step is SKIPPED TOO: because the hook never
// ran, the tree was not dirtied by mint, so NO porcelain probe and NO
// `chore(release): pre-tag artifacts for {tag}` commit must be produced (committed is
// false). The skip is reported via the presenter and the function returns immediately.
// An ABSENT hook stays a silent no-op (no run, no report, no probe).
func runPreTagHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool) (bool, error) {
	if cfg.Release.Hooks.PreTag == nil {
		return false, nil
	}

	if dryRun {
		// Report the skip and return BEFORE the porcelain probe / artifact commit: the
		// hook did not run, so the tree carries no mint-made changes to commit.
		//
		// NOTE-CACHE INTERPLAY (task 4-7): the dry-run note cache is keyed by the
		// POST-diff_exclude diff, so it is invariant to hook artifacts that fall under
		// diff_exclude (the normal case) — reuse holds even though this hook is skipped.
		// If a pre_tag hook changes a NON-excluded (real source) path, the dry-run
		// (hook-skipped) and real (post-hook) diffs genuinely differ, so the key
		// correctly misses and the real run regenerates. The cache WRITE happens at the
		// dry-run boundary (finishDryRun), not here.
		reportHookSkipped(deps.Presenter, "pre_tag")
		return false, nil
	}

	// Stage 3 is BLOCKING: the hook may build/generate for a while, so narrate it
	// with a blocking StageStarted (which animates the spinner) and a StageSucceeded
	// carrying the engine-measured Elapsed once both the hook and its artifact commit
	// land. The events fire ONLY here — when a hook is configured and actually runs —
	// so a hookless run (returned above) narrates no pre_tag stage.
	done := emitBlockingStageStarted(deps.Presenter, "pre_tag", "running pre_tag hook…", "Ran the pre_tag hook")
	env := buildHookEnv(current, next, tag, bump, dryRun)
	if err := hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PreTag, root, env); err != nil {
		return false, err
	}
	committed, err := record.CommitDirtyTree(ctx, deps.Mutator, root, pretagArtifactSubject(tag))
	if err != nil {
		return committed, err
	}
	done("hook completed")
	return committed, nil
}

// runPostReleaseHook runs the project's optional Stage-7 post_release hook through
// the shared hooks runner with the same MINT_* env as the other points (reusing
// buildHookEnv). An absent value (nil) is a no-op — the runner returns nil. A
// non-zero exit (the first non-zero entry for an array — the stop-on-first-failure
// SEQUENCING is identical to the other points) returns a non-nil error. UNLIKE
// preflight/pre_tag, the CONSEQUENCE here is warn-only: the caller does NOT abort or
// unwind, because by Stage 7 the push has crossed the PONR and the tag is public.
//
// DRY-RUN: when dryRun is active AND the hook is configured, the hook is NOT invoked;
// the skip is REPORTED via the presenter and nil is returned (the caller's warn-only
// branch is irrelevant — there is no failure). An ABSENT hook stays a silent no-op.
func runPostReleaseHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool) error {
	if dryRun {
		if cfg.Release.Hooks.PostRelease != nil {
			reportHookSkipped(deps.Presenter, "post_release")
		}
		return nil
	}
	env := buildHookEnv(current, next, tag, bump, dryRun)
	return hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PostRelease, root, env)
}

// pretagArtifactSubject builds the FIXED subject for the pre_tag artifact commit. It
// uses a constant `chore(release):` prefix — NOT the configurable commit_prefix —
// because the artifact commit is project content (e.g. a rebuilt bundle), distinct
// from the release bookkeeping commit (`{commit_prefix} Release {tag}`).
func pretagArtifactSubject(tag string) string {
	return "chore(release): pre-tag artifacts for " + tag
}

// buildHookEnv assembles the shared MINT_* hook environment from the run's computed
// versions (NewVersion = the bare version being released, PreviousVersion = the bare
// prior latest, VersionTag = the full prefixed tag), the bump kind, and the run's
// dryRun mode. The preflight and pre_tag points share it so they inject an identical
// env. dryRun renders to MINT_DRY_RUN=1/0; the builder stays correct even though the
// hooks are SKIPPED (and so never consume the env) under dry-run.
func buildHookEnv(current, next version.SemVer, tag string, bump version.Bump, dryRun bool) hooks.HookEnv {
	return hooks.NewHookEnv(next.String(""), current.String(""), tag, hookBump(bump), dryRun)
}

// preflightDetail summarises WHICH gates just passed so the preflight ✓ line tells
// the user what was actually checked, not a stage codeword: the clean tree, the
// branch gate (or its --any-branch bypass), the free tag, and the remote sync.
func preflightDetail(releaseBranch, tag string, anyBranch bool) string {
	branch := "on " + releaseBranch
	if anyBranch {
		branch = "branch gate bypassed"
	}
	return fmt.Sprintf("tree clean, %s, %s free, origin in sync", branch, tag)
}

// recordDetail names what the record stage just committed — the changelog and/or
// the version file — or says plainly that nothing needed recording (a tag-only
// release with the changelog disabled).
func recordDetail(changelogChanged, versionChanged, committed bool) string {
	if !committed {
		return "nothing to record"
	}
	var parts []string
	if changelogChanged {
		parts = append(parts, "CHANGELOG.md")
	}
	if versionChanged {
		parts = append(parts, "version file")
	}
	return strings.Join(parts, " + ") + " committed"
}

// recordSentence is the pretty past-tense narration for the record gate: it names
// what the bookkeeping commit carried, or says plainly nothing needed recording.
func recordSentence(changelogChanged, versionChanged, committed bool) string {
	if !committed {
		return "Nothing to record"
	}
	var parts []string
	if changelogChanged {
		parts = append(parts, "CHANGELOG.md")
	}
	if versionChanged {
		parts = append(parts, "version file")
	}
	return "Committed " + strings.Join(parts, " + ")
}

// hookBump maps the engine's version.Bump onto the hooks package's Bump so the
// MINT_BUMP variable reflects how the version was chosen. A --set-version run
// carries version.BumpExplicit and renders to "explicit"; an unmapped value falls
// back to patch.
func hookBump(bump version.Bump) hooks.Bump {
	switch bump {
	case version.BumpExplicit:
		return hooks.BumpExplicit
	case version.BumpMinor:
		return hooks.BumpMinor
	case version.BumpMajor:
		return hooks.BumpMajor
	default:
		return hooks.BumpPatch
	}
}

// buildPlan assembles the Phase 1 plan steps in execution order — commit, tag,
// push, and publish when publishing. The presenter renders these verbatim; the
// engine owns the verbs and targets.
func buildPlan(tag string, publish bool) presenter.Plan {
	steps := []presenter.PlanStep{
		{Verb: "commit", Target: "bookkeeping"},
		{Verb: "tag", Target: tag},
	}
	if publish {
		steps = append(steps, presenter.PlanStep{Verb: "publish", Target: tag})
	}
	return presenter.Plan{Steps: steps}
}

// finishDryRun completes a --dry-run at the dry-run boundary: the version line, plan,
// and notes have all been shown read-only above, and the notes were already
// generated-and-cached in resolveBody (so a following real run can reuse the preview).
// It reports the post_release hook skip (the only hook point the real spine reaches
// AFTER this boundary) and prints the "no changes made" close-out, returning nil
// WITHOUT any mutation. No commit, tag, push, or provider release is reached, so the
// working tree is byte-for-byte unchanged (apart from the user-level note cache). The
// preflight and pre_tag hook skips already fired above this point.
func finishDryRun(ctx context.Context, deps ReleaseDeps, cfg config.Config, root, versionKey string, current, next version.SemVer, tag string, bumpKind version.Bump) error {
	p := deps.Presenter

	// The notes were already generated-and-cached (or reused) in resolveBody, exactly as
	// a real run does — a dry run is not a separate path. So nothing cache-related happens
	// here; the dry run only reports the post_release hook skip and the close-out.
	//
	// Report the post_release hook skip too: in a real run it fires at Stage 7 (after
	// the boundary the dry run stops at), so reusing the 3-11 skip-and-report path here
	// keeps all three hook points consistently reported under the full dry run. It runs
	// no `sh` (dryRun=true) and returns nil; an absent hook stays a silent no-op.
	if err := runPostReleaseHook(ctx, deps, cfg, root, current, next, tag, bumpKind, true); err != nil {
		return err
	}

	p.RunFinished(presenter.RunResult{
		Project: projectName(root),
		Version: versionKey,
		Leaf:    cfg.Release.CommitPrefix,
		DryRun:  true,
	})
	return nil
}

// errEditorUnavailable is the cause surfaced when the `e` choice is taken but no
// Editor seam was wired — a misconfiguration that should never reach production
// (task 2-13 wires the editor). It is surfaced rather than panicked so the spine
// fails loud and clean before any mutation.
var errEditorUnavailable = errors.New("edit chosen but no editor is configured")

// errRegeneratorUnavailable is the cause surfaced when the `r` choice is taken but
// no Regenerator seam was wired — a misconfiguration that should never reach
// production (the gate offers `r` only for KindNormalAI, which task 2-16 wires with
// a Regenerator). It is surfaced rather than panicked so the spine fails loud and
// clean before any mutation.
var errRegeneratorUnavailable = errors.New("regenerate chosen but no regenerator is configured")

// reviewGate runs the notes review gate as the engine-owned re-entry LOOP and
// returns the (possibly edited or regenerated) FINAL body the caller threads to
// every sink. Rendering stays in the presenter; this owns only the semantics.
//
// The gate is supplied by the caller: release passes gateForKind(kind)
// (ReleaseReviewGate's y/n/e/r for normal-AI, FirstReleaseReviewGate's y/n/e
// otherwise), regenerate passes presenter.NotesReviewGate() (its own wording). The
// loop is gate-agnostic — it acts on the choice keys the supplied gate declares, and
// the SAME gate is re-rendered on every iteration.
//
// Each pass reads one decision and acts on it:
//
//   - a Prompt error (ErrNotInteractive / ErrInputClosed) is already an
//     *AbortError carrying a non-zero exit; it is returned as-is (ErrNotInteractive
//     is pre-rendered by the presenter, ErrInputClosed is surfaced via the abort).
//   - ChoiceYes (also the bare-Enter default) accepts: the loop RETURNS the current
//     body so the spine proceeds to Record with exactly the reviewed text. Under -y
//     the presenter returns the gate Default (ChoiceYes), so the loop accepts
//     immediately and `r` is never reached — regenerate is interactive-only.
//   - ChoiceNo aborts: it returns a clean errGateAborted cause WITHOUT emitting an
//     Unwound — Release routes the gate-no through the shared auto-unwind (which
//     authors the Unwound and resets whatever mint made), so a user-abort and a
//     pre-push failure share one path. The gate sits BEFORE any mutation, so the
//     unwind finds nothing to reset; the run still exits non-zero.
//   - ChoiceEdit edits: the editor seam returns the saved text, which REPLACES the
//     body VERBATIM — no re-parse, no re-validation (a human edit is trusted;
//     structural validation only ever guards untrusted AI output). The notes are
//     re-shown and the loop re-prompts. A return-to-gate signal
//     (ErrEditorReturnToGate — no editor could be launched, already reported by the
//     launcher) re-presents the gate with the body UNCHANGED, no re-render and no
//     abort. Any OTHER editor-seam failure (a launched-but-failed editor, the
//     nil-editor misconfiguration) is surfaced and aborts rather than blocking the
//     spine.
//   - ChoiceRegen regenerates (only reachable on the four-choice gate): see
//     regenerateBody. It reads a one-time context line via AskLine, re-runs the AI,
//     REPLACES the body with the regenerated text, re-shows the notes, and LOOPS so
//     the user may regenerate again or settle on y/n/e.
func reviewGate(ctx context.Context, p presenter.Presenter, editor Editor, regen Regenerator, gate presenter.Gate, body, versionKey string) (string, error) {
	for {
		choice, err := ReviewDecision(p, gate)
		if err != nil {
			return "", err
		}

		switch choice {
		case presenter.ChoiceYes:
			// Accept (also the bare-Enter default): proceed with the reviewed body.
			return body, nil
		case presenter.ChoiceNo:
			// Abort on gate-no: return a CLEAN cause and let Release route it through the
			// shared auto-unwind (which authors the Unwound and owns the non-zero exit) —
			// the gate no longer emits its own Unwound, so the user-abort and the pre-push
			// failure share one reset/narration path.
			return "", abort(errGateAborted)
		case presenter.ChoiceEdit:
			// Narrate the edit as a BLOCKING stage so the editor's own
			// SuspendSpinner/ResumeSpinner bracket (editor.go) suspends a GENUINELY ACTIVE
			// spinner over its $EDITOR hand-off. The StageStarted fires BEFORE editBody so
			// the spinner is live when the launcher suspends it; the matching
			// StageSucceeded fires once the edit completes (or returns to the gate) to stop
			// it, while a genuine edit failure surfaces a StageFailed (which stops it) and
			// emits no StageSucceeded.
			editDone := emitBlockingStageStarted(p, "edit", "applying edited notes…", "Applied edited notes")
			edited, eerr := editBody(ctx, editor, body)
			switch {
			case errors.Is(eerr, ErrEditorReturnToGate):
				// No editor could be launched: the launcher already reported the problem
				// via the presenter. Close the edit stage (stopping the spinner) and
				// RE-PRESENT the gate with the body UNCHANGED — this is not a failure, so do
				// not surface or abort, and do not re-render.
				editDone("notes updated")
				continue
			case eerr != nil:
				// A genuine edit failure (a launched-but-failed editor, an IO error, or
				// the nil-editor misconfiguration): surface and abort.
				return "", surface(p, "edit", eerr)
			}
			editDone("notes updated")
			// Use the edited text VERBATIM — no re-parse, no re-validation — then
			// re-render the notes and loop back to re-prompt.
			body = edited
			p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})
		case presenter.ChoiceRegen:
			// Regenerate (only reachable on the four-choice gate): re-run the AI with a
			// one-time context line, replace the body, re-show, and loop. Any failure
			// (closed input, nil seam, regenerate error) is surfaced and aborts.
			regenerated, rerr := regenerateBody(ctx, p, regen, versionKey)
			if rerr != nil {
				return "", rerr
			}
			body = regenerated
		default:
			// The gate declares only its own choice set and the presenter returns a
			// member of it; any other choice is a contract violation. Fail loud rather
			// than spin the loop forever.
			return "", surface(p, "review", errUnexpectedChoice(choice))
		}
	}
}

// gateForKind selects the review-gate variant for the precedence path that
// produced the body: notes.KindNormalAI gets the four-choice y/n/e/r gate (the one
// path with an AI to regenerate), and EVERY other Kind (first-release, degenerate,
// --no-ai, fallback) gets the y/n/e gate with no `r` — offering `r` there would be
// meaningless (no AI to nudge) and, under --no-ai, would contradict the flag.
func gateForKind(kind notes.Kind) presenter.Gate {
	if kind == notes.KindNormalAI {
		return presenter.ReleaseReviewGate()
	}
	return FirstReleaseReviewGate()
}

// regenerateBody runs the `r` regenerate step and returns the regenerated body.
// It reads a one-time context line via the presenter's AskLine input seam (the
// engine NEVER reads stdin directly), an EMPTY answer being legal (regenerate with
// no extra context), then hands that line to the Regenerator. Each failure mode is
// fail-loud and surfaced before any mutation:
//
//   - AskLine's ErrInputClosed / ErrNotInteractive abort (the read failed; the
//     engine owns surfacing ErrInputClosed). Any AskLine error — including a
//     defensively-handled ErrNotInteractive, which `r` being interactive-only should
//     make unreachable — is wrapped by the generic abort; there is no dedicated branch.
//   - a nil Regenerator (a misconfiguration that should never reach production for
//     KindNormalAI) surfaces a clean error and aborts rather than panicking.
//   - a Regenerator failure is surfaced and aborts. Richer handling could re-present
//     the gate on a regenerate failure, but surface+abort keeps the path
//     deterministic.
//
// On success the regenerated body is re-shown (so the user reviews it) and
// returned; the caller replaces the body and loops the gate.
func regenerateBody(ctx context.Context, p presenter.Presenter, regen Regenerator, versionKey string) (string, error) {
	line, err := p.AskLine(regenContextPrompt)
	if err != nil {
		// A closed/non-interactive input on the one-time-context read is fail-loud: the
		// presenter leaves ErrInputClosed unrendered, so the abort (and any closing
		// narration) is the engine's to surface — done via abort, mirroring ReviewDecision.
		return "", abort(err)
	}

	if regen == nil {
		// `r` was offered (KindNormalAI) but no Regenerator was wired — a
		// misconfiguration. Surface a clean failure and abort rather than panicking.
		return "", surface(p, "regenerate", errRegeneratorUnavailable)
	}

	regenerated, rerr := regen.Regenerate(ctx, line)
	if rerr != nil {
		// A regenerate failure is fail-loud: surface and abort.
		return "", surface(p, "regenerate", rerr)
	}

	// Re-show the regenerated notes so the user reviews them before the gate re-prompts.
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: regenerated})
	return regenerated, nil
}

// errUnexpectedChoice builds the cause for a review-gate choice outside the gate's
// declared y/n/e set — a presenter-contract violation the loop refuses to ignore.
func errUnexpectedChoice(choice presenter.Choice) error {
	return fmt.Errorf("unexpected review-gate choice %q", choice)
}

// editBody obtains the edited notes from the editor seam. A nil editor on the `e`
// path is a misconfiguration (task 2-13 wires the real one); rather than
// panicking it returns errEditorUnavailable so the gate surfaces a clear failure.
func editBody(ctx context.Context, editor Editor, current string) (string, error) {
	if editor == nil {
		return "", errEditorUnavailable
	}
	return editor.Edit(ctx, current)
}

// warnPublishDowngraded emits the LOUD pre-tag warning when the publishing provider
// could not be resolved and the run is downgrading to tag + push only. It NAMES the
// reason (an unsupported provider value, an unrecognised host, no remote, or an
// unparseable remote) so the user can see WHY publishing was skipped. It rides the
// Warn seam (which does not set failure state), so the run proceeds to a normal
// tag + push and finishes successfully — distinct from publish=false, which is a
// SILENT opt-out with no warning at all.
func warnPublishDowngraded(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "publish skipped",
		Message: "provider could not be resolved (" + downgradeReason(cause) + "); downgrading to tag + push only",
	})
}

// downgradeReason extracts the human-readable cause from a *publish.UnresolvedError
// so the downgrade warning can name it; a cause that is not an UnresolvedError (it
// always is on this path) falls back to its Error() text rather than rendering empty.
func downgradeReason(cause error) string {
	var unresolved *publish.UnresolvedError
	if errors.As(cause, &unresolved) {
		return unresolved.Reason()
	}
	return cause.Error()
}

// warnPublishFailed emits the post-PONR warn-only event: by the time it runs the
// push has already crossed the point of no return, so the tag is public and mint
// must NOT unwind. It warns and points at the heal path; the run still finishes
// successfully.
func warnPublishFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "publish failed",
		Message: "tag is already published; heal with regenerate --reuse",
		Output:  cause.Error(),
	})
}

// warnPostReleaseFailed emits the post-PONR warn-only event for a failed
// post_release hook: by Stage 7 the push has crossed the point of no return, so the
// tag is public and mint must NOT unwind. It warns (the spec-fixed message) and the
// run still finishes successfully. When the failure carries a *hooks.HookError its
// captured output (the failing entry's stderr) is rendered beneath the warn line, as
// warnPublishFailed does for the provider error; otherwise the Output is empty.
func warnPostReleaseFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   "post_release",
		Message: "post_release hook failed; tag is already published",
		Output:  hookFailureOutput(cause),
	})
}

// warnNoteCacheFailed emits the warn-only notice when the dry-run note cache write
// fails: the preview was already shown and the dry run made no destructive change,
// so the run still finishes successfully — the only consequence is that the
// subsequent real run regenerates the notes instead of reusing the cached preview.
func warnNoteCacheFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Label:   dryRunLabel,
		Message: "could not cache notes preview; the real run will regenerate",
		Output:  cause.Error(),
	})
}

// warnAnyBranchBypass emits the --any-branch observable signal: the on-release-branch
// gate was bypassed for this run, so a release running off the release branch is
// visible rather than silent. It rides the existing Warn seam (mirroring
// warnPopConflict) — a Warn does not set failure state, so the release proceeds and
// finishes normally; this is informational, not an abort.
func warnAnyBranchBypass(p presenter.Presenter) {
	p.Warn(presenter.Warning{
		Label:   "any-branch",
		Message: "release-branch gate bypassed (--any-branch); releasing from the current branch",
	})
}

// hookFailureOutput extracts the failing hook entry's captured stderr from a
// *hooks.HookError so the warn can render it verbatim; a cause that is not a
// HookError (or that captured no stderr) yields the empty string — the common case,
// which renders no output block.
func hookFailureOutput(cause error) string {
	var hookErr *hooks.HookError
	if errors.As(cause, &hookErr) {
		return hookErr.Result.Stderr
	}
	return ""
}

// surface renders a stage failure through the presenter and returns the engine's
// typed abort carrying a non-zero exit code. It is the single pre-PONR failure
// path: every stage-1–8 error flows through here so the failure is both shown and
// terminal. A *GateError surfaces its actionable Message; any other error
// surfaces its Error() text.
func surface(p presenter.Presenter, stage string, cause error) error {
	p.StageFailed(presenter.StageFailure{
		Name:    stage,
		Message: failureMessage(cause),
	})
	return abort(cause)
}

// failureMessage extracts the display message for a stage failure: a preflight
// *GateError carries an actionable, display-ready Message; everything else falls
// back to the wrapped error text.
func failureMessage(cause error) string {
	var gate *preflight.GateError
	if errors.As(cause, &gate) {
		return gate.Message()
	}
	return cause.Error()
}

// projectName derives the project label from the repo root's final path segment.
// Phase 1 has no configured project name, so the directory basename is the natural
// stand-in for the brand/header lines.
func projectName(root string) string {
	return filepath.Base(root)
}
