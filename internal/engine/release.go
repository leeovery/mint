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
// optional pre_tag artifact commit, answers commit and bookkeeping commit — and whether
// the annotated tag was created). On a gate-no or any pre-push failure it hands those
// captured inputs to Unwind, which resets EXACTLY the commits mint made and deletes the
// tag iff mint created it — no HEAD probe, no inference — then re-commits the exclusion
// answers on the starting commit when their commit had landed, and leaves their
// uncommitted write to a .mint.toml git ignores in place.
// The two triggers are deliberately identical: a declined gate and a rejected push with
// the same MadeState produce a byte-identical state and Unwound summary. With nothing made the
// surgical unwind no-ops (no reset, no Unwound). The PRE-mutation / preflight failures
// (before the StartState is captured) stay plain surface; there is nothing to unwind.
// All recovery mutations flow through the lock-resilient Mutator (4-1). --autostash
// pop-after ordering (4-4) layers on top of this wiring.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mint/internal/ai"
	"mint/internal/aitransport"
	"mint/internal/bytesize"
	"mint/internal/cmdline"
	"mint/internal/config"
	"mint/internal/fsutil"
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
	"mint/internal/runclose"
	"mint/internal/runner"
	"mint/internal/version"
)

// releaseAction is the engine-supplied verb word for the start-of-run header, and
// releaseCommand the command words of the rail's header.
const (
	releaseAction  = "releasing"
	releaseCommand = "release"
)

// firstRelease is the start station's name for a release with no previous tag, the
// first-release wording the version gate uses.
const firstRelease = "first release"

// releaseStart is the station a release starts from: the previous tag, or the first
// release, at the branch and commit HEAD stands at, or on a branch with no commits
// yet; with HEAD unread, at no detail.
func releaseStart(current version.SemVer, prefix string, head gitrepo.Head) presenter.Station {
	station := presenter.Station{Name: current.String(prefix)}
	if current == (version.SemVer{}) {
		station.Name = firstRelease
	}
	switch {
	case head.Unborn():
		station.Detail = head.Branch + " · " + head.Commit()
	case head != (gitrepo.Head{}):
		station.Detail = head.Branch + " @ " + head.ShortSHA
	}
	return station
}

// Editor is the engine's edit seam: an `e` choice hands a notes body to Edit and
// uses the text it returns VERBATIM, never re-parsed — a human edit is trusted.
type Editor interface {
	// Edit presents the current body to the user's editor and returns the saved
	// text. A non-nil error means no edit could be obtained: ErrEditorReturnToGate (no
	// editor could be launched, already reported) sends the engine back to the gate or
	// choice it came from; any other error is surfaced and aborts.
	Edit(ctx context.Context, current string) (string, error)
}

// Regenerator is the engine's regenerate seam: the `r` review-gate choice hands a
// ONE-TIME context line to Regenerate and uses the regenerated body it returns as
// the new notes body. The one-time context is TRANSIENT — it flows into this one
// regeneration only and is NEVER persisted to [release].context. The interface is
// intentionally tiny so production can wire the real AI path behind it (a re-merge
// of chunked notes, else notes.Generator.GenerateWithContext over the run's
// lastTag/cfg) while the gate-loop tests drive a scripted fake. It is defined HERE (the consumer)
// rather than where it is implemented, per the accept-interfaces convention.
//
// It is consulted ONLY on `r`, which the gate offers ONLY for notes.KindNormalAI
// (the one path the AI produced the body), so the no-AI paths never reach it and
// may leave the seam nil.
type Regenerator interface {
	// Regenerate re-runs the AI with oneTimeContext appended to the release's
	// instructions and returns the new body. An empty oneTimeContext is legal (regenerate
	// with no extra context). A non-nil error means the regeneration failed; the
	// engine surfaces it and aborts (fail-loud) rather than looping on a broken AI.
	Regenerate(ctx context.Context, oneTimeContext string) (string, error)
}

// RegenRun is what a run's `r` regenerates from and reports to: Merged, the reader of
// the merge inputs of the chunked run that wrote the notes now at the gate, returning
// nil for none (a nil reader reads as none); and the sinks of the chunked run an `r`
// becomes. Build one through chunkNotesKeeper.regenRun, which binds both to the keeper,
// so an `r` re-merges from the same inputs a failed `r` saves from.
type RegenRun struct {
	Merged func() *notes.MergeInputs
	NotesSinks
}

// reviewRegen is the review gate's `r`: the Regenerator it consults; shown, run once
// each regenerated body is shown; and keep, handed the cause of an `r` that failed
// before any ✗ line shows it, returning the cause naming whatever it kept. A nil shown
// or keep does nothing.
type reviewRegen struct {
	regenerator Regenerator
	shown       func()
	keep        func(error) error
}

func (r reviewRegen) keepFailed(cause error) error {
	if r.keep == nil {
		return cause
	}
	return r.keep(cause)
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
	// Editor is the OPTIONAL edit seam consulted ONLY on an `e` choice, at the review
	// gate or the notes floor. Runs that never reach `e` (every non-interactive and
	// accept/abort path) may leave it nil; production wires the real $EDITOR
	// resolution. If `e` is chosen with a nil Editor (a misconfiguration), the run
	// surfaces a clear error and aborts rather than panicking.
	Editor Editor
	// Regenerator is the OPTIONAL regenerate seam consulted ONLY on the `r`
	// review-gate choice, which the gate offers ONLY for notes.KindNormalAI. Every
	// other path (the no-AI Kinds, every non-interactive and accept/abort path) may
	// leave it nil. In production it is left nil: Release builds a PER-RUN regenerator
	// closure over the resolved Generator (see perRunRegenerator). When non-nil it
	// OVERRIDES that closure — the test-injection seam
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
	// NoteCache is the note-cache seam, written and read by dry and real runs alike. A
	// run that generates AI notes writes the body, with a chunked run's merge inputs,
	// keyed by a hash of (the post-diff_exclude diff + the computed version + the
	// resolved prompt/context) with a TTL stamp. A later run recomputes the SAME key and
	// Looks it up: a live (within-TTL) match reuses the exact bytes and SKIPS the notes calls, and
	// `r` on reused chunked notes re-merges the cached part notes; a miss (or an expired
	// entry) regenerates. When nil both the write and the reuse lookup are skipped (the
	// run always generates); production wires a user-cache-dir notescache.Store.
	NoteCache NoteCache
	// ChunkNotes keeps a failed chunked notes run's finished chunk notes, so the
	// failure can name the file they were saved to. When nil none are kept.
	ChunkNotes ChunkNotesSaver
	// Yes is the -y flag and StdinInteractive the startup stdin-is-a-TTY signal. A
	// run with StdinInteractive and without Yes is attended.
	Yes              bool
	StdinInteractive bool
	// MintVersion is mint's own bare version for the run's header, empty for a dev
	// build, and Home the user's home directory the header shortens the repo root
	// under. Both come from the cmd layer, so the engine reads no environment for them.
	MintVersion string
	Home        string
	// Clock is what the run and its stages are timed by, the wall clock when nil.
	Clock func() time.Time
	// run closes the run Release or RegenerateRun opened; nil outside them (a --all
	// backfill's versions, a test driving Unwind alone).
	run *runclose.Run
}

// clock is what the run and its stages are timed by: Clock, or the wall clock.
func (d ReleaseDeps) clock() func() time.Time {
	if d.Clock != nil {
		return d.Clock
	}
	return time.Now
}

// header is the run's header line for command over root.
func (d ReleaseDeps) header(command, root string) presenter.Header {
	return presenter.Header{Command: command, Where: fsutil.HomeRelative(root, d.Home), Version: d.MintVersion}
}

// ChunkNotesSaver saves a failed chunked run's finished chunk notes for repoRoot's
// tag and returns the saved file's path; notescache.ChunkNotes satisfies it.
type ChunkNotesSaver interface {
	Save(repoRoot, tag, notes string) (string, error)
}

// NoteCache is the engine's note-cache seam: the WRITE side that persists generated
// notes and the READ side that reuses them on a later run. It composes the
// segregated writer and reader interfaces, defined HERE (the consumer) per the
// accept-interfaces convention; notescache.Store satisfies it.
type NoteCache interface {
	NoteCacheWriter
	NoteCacheReader
}

// NoteCacheWriter is the engine's note-cache WRITE seam: it persists a generated note
// body, with the merge inputs of the chunked run that wrote it, under a precomputed
// key, scoped to a repo root, with its own TTL stamp. It is defined HERE (the
// consumer) per the accept-interfaces convention; notescache.Store satisfies it.
type NoteCacheWriter interface {
	// Write persists body and merge, nil for notes a single pass wrote, under key for
	// repoRoot. A non-nil error means the cache entry could not be written; the run
	// warns and carries on.
	Write(repoRoot, key, body string, merge *notes.MergeInputs) error
	// Prune deletes every cached note for repoRoot except keepKey's, bounding the
	// per-project cache to the current diff's note so stale entries never accumulate.
	// Best-effort housekeeping — there is no error to handle (a leftover entry is
	// harmless; the cache is only an optimization).
	Prune(repoRoot, keepKey string)
}

// NoteCacheReader is the engine's note-cache REUSE seam: it looks up previously
// written notes by their precomputed key, reporting found ONLY for a
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
	// path with an AI to regenerate), while EVERY other Kind offers the y/n/e variant
	// with no `r`.
	NotesKind notes.Kind
	// NoAI is the --no-ai flag: a DELIBERATE skip of the AI path (after the
	// first-release and degenerate guards). It is threaded into the selector's
	// SelectState so the precedence routes to the commit-subject fallback body and
	// never calls the AI. It is irrelevant when NotesBody overrides the selector.
	NoAI bool
	// NotesFile is the --notes-file path, empty when not given. A relative path
	// resolves against the directory mint was run from. Release reads it once, before
	// any other step, so a later change to the file never reaches the run. Its body
	// wins the notes precedence outright: no AI path step runs and on_notes_failure
	// never applies. A file inside the repo is left out of the clean-tree check, the
	// stash and every commit the run makes, so a successful run leaves it as it
	// started, staged edits still staged.
	NotesFile string
	// NotesFileSet reports that --notes-file was given, so an empty NotesFile is
	// refused as naming no file rather than read as the flag left off.
	NotesFileSet bool
	// WorkDir is the directory a relative NotesFile resolves against. Empty means the
	// process working directory.
	WorkDir string
	// AutoStash is the --autostash escape hatch: when set, mint runs `git stash push
	// --include-untracked` BEFORE the clean-tree gate (so a dirty tree passes the gate)
	// and restores the WIP with `git stash pop` afterward — on success AND on
	// abort/failure. On an abort the restore is layered ON TOP of the surgical unwind:
	// the unwind runs first, THEN the pop applies
	// the WIP, so the WIP is never popped against mint's release commits. A no-WIP run is
	// a no-op (nothing stashed → nothing popped); a pop conflict leaves the stash intact
	// and warns rather than discarding the user's work. It is OPT-IN because the release
	// mutates the tree, so popping unrelated WIP can conflict — opting in is the user
	// asserting it is safe. All stash/pop ops flow through the lock-resilient Mutator.
	//
	// Under DryRun the real stash is SKIPPED ENTIRELY (no push, no pop) — a dry run takes
	// no action, so it never reaches the Mutator. Because the stash that would have
	// cleaned the tree is skipped, the clean-tree preflight gate is bypassed for the
	// DryRun && AutoStash combo so a dirty-tree dry run still completes its preview.
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
	// consumes it); the --autostash stash push/pop is SKIPPED too (with the clean-tree
	// gate bypassed for the DryRun && AutoStash combo so a dirty-tree dry run still
	// previews); and — at the dry-run boundary after the gate (4-7a) — the
	// version-file projection, the changelog write, the bookkeeping commit, the
	// annotated tag, the atomic push, and the provider release are ALL skipped so a dry
	// run NEVER reaches the lock-resilient Mutator and the repo is byte-for-byte
	// unchanged (this guarantee now holds for the --autostash combo too). The -d/--dry-run
	// flag is wired in production (cmd/mint) and sets this field. Outside the repo, a
	// dry run still writes the AI note it previews to the user-level dry-run cache, so
	// the following real run can reuse it, and a failed chunked run's finished chunk
	// notes to their file, as a real run does.
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
// a pre-PONR abort the repo is already unwound by the time the pop applies the WIP on
// top. A no-WIP run is a no-op (nothing stashed → no deferred pop); a pop conflict
// warns and leaves the stash intact (never discarded).
//
// Under --dry-run the autostash stash push/pop is SKIPPED — a dry run takes no action,
// so it never mutates the tree. Because that skipped stash is what would have cleaned
// the tree for the clean-tree gate, the gate is bypassed for the DryRun && AutoStash
// combo (and ONLY that combo: a non-autostash dirty dry run still aborts at the gate),
// so a dirty-tree dry run completes its byte-identical preview without reaching the
// Mutator.
//
// Every run that opened with RunStarted closes with one RunFinished, emitted here
// from whatever ending the run reached: released, with or without a warning, or a
// dry run, or, stopped short, unwound, declined or failed, its end saying what the
// run left behind.
func Release(ctx context.Context, deps ReleaseDeps, opts ReleaseOptions) error {
	deps.run = runclose.Begin(deps.Presenter, deps.clock())
	// Deferred, so the work the run deferred (the autostash pop) runs after the close
	// and still runs when the run panics, which closes nothing.
	defer deps.run.RunDeferred()
	err := cutRelease(ctx, deps, opts)
	deps.run.Close(declined(err))
	return err
}

// declined reports whether err ends the run at a question answered no. Input that
// closed at a question is no answer, so it ends the run as a failure.
func declined(err error) bool {
	return errors.Is(err, errReleaseDeclined) || errors.Is(err, errGateAborted) ||
		errors.Is(err, errFloorAborted)
}

// cutRelease is Release's run, from reading the notes file to the end of its
// post_release hook; Release closes it.
func cutRelease(ctx context.Context, deps ReleaseDeps, opts ReleaseOptions) error {
	p := deps.Presenter

	notesPath := notesFilePath(opts.WorkDir, opts.NotesFile)
	supplied, err := ReadNotesFile(p, notesPath, opts.NotesFileSet)
	if err != nil {
		return err
	}

	// Stage 1 — resolve root, load config, resolve branch, compute the tag.
	root, err := gitrepo.ResolveRoot(ctx, deps.Runner)
	if err != nil {
		return surface(p, "preflight", err)
	}
	keepOut, err := notesExclusion(ctx, deps.Runner, root, notesPath)
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
	// Its start station is where the release starts from: the previous tag, and the
	// branch and commit HEAD stands at, read here in one call.
	head := gitrepo.ResolveHead(ctx, deps.Runner)
	deps.run.Start(presenter.RunInfo{
		Project: projectName(root),
		Version: versionKey,
		Action:  releaseAction,
		Leaf:    cfg.Release.CommitPrefix,
		DryRun:  opts.DryRun,
		Header:  deps.header(releaseCommand, root),
		Start:   releaseStart(current, cfg.Release.TagPrefix, head),
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
	// state. Deferring is load-bearing: the surgical unwind runs inline before any abort
	// return, so by the time this deferred pop fires the repo is already unwound —
	// unwind-then-pop holds for every abort path. A no-WIP tree stashes nothing (no pop
	// is owed and none is deferred); a pop conflict warns and leaves the stash intact.
	//
	// DRY-RUN: a dry run takes no action, so neither the stash push NOR the deferred pop
	// runs — the autostash block is gated on !opts.DryRun. Skipping the real stash keeps
	// the dry-run guarantee that NOTHING reaches the lock-resilient Mutator and the repo
	// is byte-for-byte unchanged, now holding for the --autostash combo too. Because the
	// stash that would have cleaned the tree is skipped, the clean-tree gate is bypassed
	// for this same DryRun && AutoStash combo (passed into runPreflight below) so a
	// dirty-tree dry run still completes its preview rather than aborting at the gate.
	if opts.AutoStash && !opts.DryRun {
		if autostashPush(ctx, deps, keepOut) {
			deps.run.Defer(func() { autostashPop(ctx, deps) })
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
	// The clean-tree gate is bypassed ONLY for the dry-run+autostash combo: a dry run
	// skips the real autostash (so the tree stays dirty), and since autostash's only job
	// is to clean the tree for that gate, the gate must be bypassed too — otherwise a
	// dirty-tree dry run would newly abort here instead of completing its preview. The
	// bypass is conditioned on DryRun && AutoStash, never DryRun alone, so a non-autostash
	// dirty dry run still aborts at the clean-tree gate (correctly flagging the dirty tree).
	if err := runPreflight(ctx, deps.Runner, releaseBranch, tag, opts.AnyBranch, opts.DryRun && opts.AutoStash, keepOut); err != nil {
		return surface(p, "preflight", err)
	}

	// Stage 2 — project preflight hook. After mint's built-in gates pass and BEFORE
	// any mutation (and before startingHEAD is even captured), the project's optional
	// preflight hook runs for project-specific gates/validation. An absent or empty
	// hook (nil, "", []) is a no-op. A non-zero exit (for an array, the first non-zero
	// entry) aborts the whole release cleanly: it is surfaced as a "preflight"
	// StageFailed. Because this precedes all mutation there is nothing to unwind — a
	// plain surface, not the auto-unwind path.
	hookSkipped, err := runPreflightHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun)
	if err != nil {
		return surface(p, "preflight", err)
	}
	// Stage 2 narration: the preflight gates are read-only (no spinner), so emit a
	// completion line only — no StageStarted. Fired once both the built-in gates and
	// the project preflight hook have passed (Stage 2 complete), a dry run's skip of
	// the hook its note.
	checked := preflightDetail(releaseBranch, tag, opts.AnyBranch)
	p.StageSucceeded(presenter.StageSuccess{Name: stagePreflight, Key: stagePreflight, Detail: checked, Sentence: "Preflight passed: " + checked, Note: hookSkipped})

	// With preflight passed every stage the release will go through is known: lay them
	// out now, ahead of the first slow one, in a Quiet plan. The steps plain prints
	// still follow the notes, where its line has always been.
	versionDetail, _ := versionPreview(current, next, cfg.Release.TagPrefix, bumpKind)
	route := releaseNotesRoute(selectState(current, cfg, opts, supplied), opts.NotesBody != "")
	branch := pushedBranch(releaseBranch, head, opts.AnyBranch)
	p.ShowPlan(releasePlan{
		cfg:       cfg,
		tag:       tag,
		branch:    branch,
		version:   versionDetail,
		preflight: checked,
		notes:     route,
	}.stages())

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
	start := StartState{HEAD: startingHEAD, Tag: tag, TagExisted: false, KeepOut: keepOut}

	// Track what mint actually makes this run (NOT inferred by probing git): each
	// commit count is bumped as the step that made it runs, and TagCreated is set when
	// the annotated tag is created. The surgical unwind drives off this state — it
	// resets exactly made.Commits and deletes the tag iff made.TagCreated.
	made := MadeState{hookRan: hookRuns(cfg.Release.Hooks.Preflight, opts.DryRun)}

	// Stage 3 — project prep. The optional pre_tag hook builds/generates artifacts
	// (e.g. a knowledge bundle) and may dirty the tree; mint then commits whatever it
	// left dirty as its OWN `chore(release): pre-tag artifacts for {tag}` commit, kept
	// distinct from the bookkeeping commit. This runs AFTER startingHEAD so the
	// artifact commit is covered by the auto-unwind, and BEFORE notes so they generate
	// at the post-hook HEAD. An absent or empty hook is a no-op (no prep, no artifact
	// commit); a non-zero exit aborts cleanly before any notes/tag/push — routed
	// through the surgical unwind so mint's OWN artifact commit (when one landed before
	// the hook step failed) is reset back to startingHEAD. The committed signal is
	// folded into made.Commits the moment the artifact commit lands, so it is tracked
	// even if a LATER stage fails.
	made.hookRan = made.hookRan || hookRuns(cfg.Release.Hooks.PreTag, opts.DryRun)
	artifactCommitted, err := runPreTagHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun, keepOut)
	if artifactCommitted {
		made.Commits++
	}
	if err != nil {
		var uncommitted *record.UncommittedError
		made.preTagStaged = errors.As(err, &uncommitted)
		return surfaceAndUnwind(ctx, deps, "pre_tag", start, made, err)
	}

	// Stage 4 — notes body. resolveBody runs the notes-path PRECEDENCE (SelectBody)
	// to pick the body + Kind for this run, building the selector from deps.Runner now
	// that root is resolved. opts.NotesBody is a test-injection override that bypasses
	// the selector. An on_notes_failure=abort failure aborts here, with the next moves,
	// unless an attended real run takes a body at the notes floor's choice instead.
	// The exclusion check's answers are recorded inside this stage, before the first
	// notes attempt, and folded into made whether or not the stage then fails.
	// Nothing is tagged yet, but a pre_tag artifact commit may already be in made, so an
	// abort routes through the surgical unwind (not a plain surface)
	// — exactly like every other pre-push failure — so that artifact commit is reset
	// back to the clean start. Whatever
	// resolves flows WHOLE to every active sink below — no parsing, no per-sink reassembly.
	// (versionKey was computed at Stage 1 so RunStarted could lead the run; resolveBody
	// reuses it to recompute the dry-run cache key for the real-run reuse lookup.)
	//
	// Stage 4 is BLOCKING: generating notes can call the AI. Narrate it with a
	// blocking StageStarted (spinner), a chunked run's progress while it runs, and a
	// StageSucceeded carrying the engine-measured Elapsed once the body resolves.
	stage := startNotesStage(p, deps.clock())
	chunkNotes := newChunkNotesKeeper(deps.ChunkNotes, root, tag)
	answers := &answersCommit{root: root, tag: tag}
	body, kind, generator, err := resolveBody(ctx, deps, root, cfg, selectState(current, cfg, opts, supplied), chunkNotes, versionKey, opts, stage, answers)
	if answers.landed {
		made.Commits++
		answers.position = made.Commits
		made.keptAnswers = answers
	}
	made.answersEdit = answers.edit
	made.answersLeft = answers.leftInPlace
	made.wrote = answers.wrote
	var recordingErr *answersRecordingError
	if errors.As(err, &recordingErr) {
		return surfaceAndUnwind(ctx, deps, "notes", start, made, err)
	}
	if err != nil {
		atFloor := reachesNotesFloor(err)
		offersChoice := atFloor && deps.asksAtTerminal(opts)
		failure := stage.failure(err)
		failure.Recoverable = offersChoice
		p.StageFailed(failure)
		chunkNotes.warnUnsaved(p)
		if !offersChoice {
			if atFloor {
				p.NextMoves(deps.notesFloorMoves(err, cfg, releaseRerun(opts), failure.SavedNotes))
				stage.showSuggestions(cfg.Release.DiffExclude)
			}
			return Unwind(ctx, deps, start, made, err)
		}
		body, kind, err = releaseNotesFloor(ctx, deps, current.String(cfg.Release.TagPrefix), chunkNotes.saved)
		if err != nil {
			return Unwind(ctx, deps, start, made, err)
		}
	} else {
		stage.settleUncheckedCheck(route.checks)
		stage.doneFrom(kind, opts.NotesFile)
		chunkNotes.warnUnsaved(p)
		if opts.DryRun {
			stage.warnDryRunFallback()
		} else {
			stage.warnFallback(versionKey)
		}
	}

	p.ShowPlan(buildPlan(tag, cfg.Release.Publish))
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})
	hintLowerCap(p, stage.lowerCap())
	stage.showSuggestions(cfg.Release.DiffExclude)

	// DRY-RUN BOUNDARY: a dry run takes no action, so it asks for nothing — it stops
	// HERE, before the review gate, having shown the full preview (version, plan, notes),
	// previews the review where a real run asks it (Quiet: plain has no line for it),
	// and prints the "no changes made" close-out instead of prompting, recording,
	// tagging, pushing, or publishing. The working tree is byte-for-byte unchanged
	// (hooks were skipped-and-reported above; the notes were generated read-only).
	if opts.DryRun {
		review := gateForKind(kind)
		review.Quiet = true
		p.PreviewGate(review)
		return finishDryRun(ctx, deps, cfg, root, versionKey, current, next, tag, bumpKind)
	}

	// The review gate is the FINAL gate of a real release: accepting it proceeds straight
	// through Record → tag → push → publish (there is no further confirmation), which is
	// why ReleaseReviewGate's wording says "release", not "accept".
	regens := stage.regens(func(lowerCap int) { hintLowerCap(p, lowerCap) })
	regen := regens.review(perRunRegenerator(deps, generator, chunkNotes, regens, current.String(cfg.Release.TagPrefix), cfg), chunkNotes)
	body, err = chunkNotes.reviewGate(ctx, deps, regen, gateForKind(kind), body, versionKey, releaseRerun(opts))
	if err != nil {
		// Unwind, not surfaceAndUnwind: reviewGate has already surfaced whatever
		// needed a StageFailed.
		return Unwind(ctx, deps, start, made, err)
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
	made.wrote = true
	projection, err := record.ProjectVersionFile(root, cfg.Release.VersionFile, cfg.Release.VersionPattern, versionKey)
	if err != nil {
		// Post-mutation, pre-PONR: route through the surgical unwind so any pre_tag
		// artifact commit already in made is reset back to the clean start. The
		// version-file projection is a filesystem write that made no commit, so made is
		// unchanged here; the unwind's reset (when made.Commits>0) discards the projection
		// along with the artifact commit. No tag exists yet (made.TagCreated false).
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}
	versionChanged := projection.Changed
	made.recordWrite(cfg.Release.VersionFile, versionChanged, projection.Created)

	// When changelog=false WriteChangelog no-ops (Changed:false). When the version file
	// also nets no change, CommitBookkeeping makes NO commit — the tag then points at
	// HEAD as it stands and STILL carries the full body via TagAndPush. Nothing durable
	// is lost.
	writeResult, err := record.WriteChangelog(root, versionKey, opts.Now, body, cfg.Release.Changelog)
	if err != nil {
		// Post-mutation, pre-PONR: route through the surgical unwind so any artifact commit
		// (and a half-written changelog discarded by the reset) is rolled back to the clean
		// start — the abort path is identical to the pre-push failure path. No tag yet.
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}
	made.recordWrite(record.ChangelogFileName, writeResult.Changed, writeResult.Created)

	// ONE folded bookkeeping commit stages the changelog and the version file together —
	// the version file is never given its own separate commit. Kept DISTINCT from the
	// pre_tag artifact commit (3-3), which precedes it. It commits IFF something net-
	// changed (the changelog and/or the version file); the SHARED record predicate —
	// the same rule CommitBookkeeping's no-op branch consumes — tells the spine whether
	// HEAD just moved, so made.Commits is bumped exactly when a commit landed —
	// tracked, not inferred by probing git.
	bookkeepingCommitted := record.BookkeepingWillCommit(cfg.Release.VersionFile, writeResult.Changed, versionChanged)
	if err := record.CommitBookkeeping(ctx, deps.Mutator, root, cfg.Release.CommitPrefix, tag, cfg.Release.VersionFile, writeResult.Changed, versionChanged, keepOut); err != nil {
		var uncommitted *record.UncommittedError
		if errors.As(err, &uncommitted) {
			// Staged, the files the record created are tracked, so a reset removes them.
			made.created = nil
		}
		return surfaceAndUnwind(ctx, deps, "record", start, made, err)
	}
	if bookkeepingCommitted {
		made.Commits++
		made.written, made.created = nil, nil
	}
	// Stage 5 narration: the ✓ line names WHAT was just recorded (the artifacts the
	// bookkeeping commit carried) rather than a bare stage codeword, so the user can
	// follow the run without knowing mint's stage taxonomy.
	recorded := presenter.StageSuccess{
		Name:     stageRecord,
		Key:      stageRecord,
		Detail:   recordDetail(writeResult.Changed, versionChanged, bookkeepingCommitted),
		Sentence: recordSentence(writeResult.Changed, versionChanged, bookkeepingCommitted),
	}
	if bookkeepingCommitted {
		recorded.Commit = record.BookkeepingSubject(cfg.Release.CommitPrefix, tag)
	}
	p.StageSucceeded(recorded)

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
	// (this whole block is skipped), not a warned downgrade. Any OTHER resolution error,
	// like a failed gh gate, is a pre-PONR abort labelled "preflight" on the publish node
	// it gates, routed through the surgical unwind.
	var publisher publish.Publisher
	if cfg.Release.Publish {
		resolved, err := resolvePublisher(ctx, deps, cfg)
		switch {
		case errors.Is(err, publish.ErrProviderUnresolved):
			warnPublishDowngraded(p, err, "downgrading to tag + push only")
		case err != nil:
			return failAndUnwind(ctx, deps, publishGateFailure(err), start, made, err)
		default:
			publisher = resolved
		}

		// The gh gate gates ONLY an actually-selected driver: on a downgrade publisher is
		// nil, so it is never reached — keeping the pushed tag from being stranded.
		if publisher != nil {
			if err := preflight.CheckGhAuth(ctx, deps.Runner); err != nil {
				// The bookkeeping commit may have moved HEAD; the surgical unwind resets exactly
				// the commits in made. No tag yet (made.TagCreated false).
				return failAndUnwind(ctx, deps, publishGateFailure(err), start, made, err)
			}
		}
	}

	// Stage 6 — tag + atomic push.
	//
	// COMMIT-GRAPH INVARIANT (assembled across stages 3, 4 and 5): this run produces UP
	// TO THREE commits before the tag, in this order — an OPTIONAL pre_tag
	// hook-artifact commit (`chore(release): pre-tag artifacts for {tag}`, Stage 3, made
	// only when the hook dirtied the tree), an OPTIONAL answers commit of .mint.toml
	// alone (`chore(release): release-notes path answers for {tag}`, Stage 4, made only
	// when an exclusion pause was answered), and an OPTIONAL release-bookkeeping commit
	// (`{commit_prefix} Release {tag}`, Stage 5, made only when the changelog and/or
	// version file netted a change). The annotated tag conceptually points at the
	// release-bookkeeping commit; because TagAndPush creates it with `git tag -a {tag}`
	// (which tags the CURRENT HEAD) and HEAD EQUALS the bookkeeping commit whenever one
	// was made, the tag is correct by construction — no explicit target is passed.
	// Without a bookkeeping commit the tag sits on the latest commit mint did make, or
	// on the pre-existing HEAD when it made none. NO-OP SAFETY: any of the three commits
	// may be absent and no empty commit is ever created (CommitDirtyTree and
	// CommitBookkeeping each commit only when there is a real change, the answers commit
	// only once answers were written). The single atomic
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
		return failAndUnwind(ctx, deps, pushFailure(err, 0), start, made, err)
	}

	// Stage 6 is BLOCKING: the tag + atomic push round-trips the network. Narrate it
	// with a blocking StageStarted (spinner) and a StageSucceeded carrying the
	// engine-measured Elapsed once the push crosses the PONR. On failure a timed
	// StageFailed narrates the stage instead, so no StageSucceeded fires.
	push := emitBlockingStageStarted(p, deps.clock(), stagePush, "pushing branch and tag to origin…", "Pushed branch + "+tag+" atomically")
	if _, err := deps.Releaser.TagAndPush(ctx, tag, cfg.Release.CommitPrefix, body); err != nil {
		// Pre-PONR failure: route through the surgical unwind. A push REJECTION means the
		// local tag WAS created (TagAndPush wraps it in release.ErrPushRejected), so the
		// unwind must delete it; a tag-CREATION failure leaves no tag, so only the tracked
		// commits are reset. Either way the reset is driven by made.Commits — the exact
		// count mint made this run — not a HEAD probe.
		made.TagCreated = errors.Is(err, release.ErrPushRejected)
		return failAndUnwind(ctx, deps, pushFailure(err, push.elapsed()), start, made, err)
	}
	push.succeeded(presenter.StageSuccess{
		Detail:   "branch + " + tag + " pushed atomically",
		Row:      branch + " + " + tag + " pushed atomically",
		Sentence: push.sentence,
	})

	// Stage 7 — publish. Post-PONR: a publish failure is WARN-ONLY (the tag is
	// already public); the run does not unwind and exits successfully. The release is
	// created through the resolved Publisher INTERFACE (whichever driver detection
	// picked), never a concrete type. It is gated on a publisher actually being
	// SELECTED, not merely on publish=true: a safe downgrade (provider unresolved)
	// leaves publisher nil and already warned above, so publishing is skipped here —
	// the run is tag + push only.
	releaseURL := ""
	outcome := presenter.OutcomeReleased
	if publisher != nil {
		// CreateRelease returns the published release URL (parsed from the driver's
		// stdout); thread it into RunResult.URL so the success footer renders the real
		// URL. A publish FAILURE is warn-only and leaves releaseURL empty (no bogus URL
		// in the footer); a downgrade leaves publisher nil so releaseURL stays empty too.
		url, err := publisher.CreateRelease(ctx, tag, tag, body)
		if err != nil {
			warnPublishFailed(p, err, versionKey)
			outcome = presenter.OutcomeReleasedWithWarning
		} else {
			releaseURL = url
			// Post-PONR success narration: the release exists on the provider now.
			emitGateSucceeded(p, stagePublish, "release published", "Published the release")
		}
	}

	// Stage 7 — post_release hook. Post-PONR follow-ups (notifications, tap
	// repository_dispatch, a signed build's notarization and upload), narrated as a
	// blocking stage like pre_tag. It runs UNCONDITIONALLY: reaching here means the
	// push already crossed the PONR, so the tag is public whether publish=true or
	// publish=false, and post-release follow-ups apply either way. Like the publish
	// failure above, a non-zero exit is WARN-ONLY — it does NOT abort or unwind; the
	// run still reaches RunFinished and returns nil. This is the ONLY hook point whose
	// failure is non-fatal (preflight and pre_tag abort); the array stop-on-first-
	// failure SEQUENCING is identical across points — only the CONSEQUENCE differs.
	if err := runPostReleaseHook(ctx, deps, cfg, root, current, next, tag, bumpKind, opts.DryRun); err != nil {
		warnPostReleaseFailed(p, err)
		outcome = presenter.OutcomeReleasedWithWarning
	}

	deps.run.Land(presenter.RunResult{
		Project: projectName(root),
		Version: versionKey,
		URL:     releaseURL,
		Leaf:    cfg.Release.CommitPrefix,
		Outcome: outcome,
		End:     presenter.Station{Name: tag, Detail: "released"},
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
// defaults to yes (a bare Enter proceeds). No stage event settles the version stage
// on a real run, so the gate carries the detail it settles to: the one a dry run's
// version line shows.
func versionConfirmGate(current, next version.SemVer, prefix string, bumpKind version.Bump) presenter.Gate {
	nextDisplay := next.String(prefix)
	question := fmt.Sprintf("Release %s → %s (%s)?", current.String(prefix), nextDisplay, hookBump(bumpKind))
	if current.String("") == "0.0.0" {
		question = fmt.Sprintf("Release %s (first release)?", nextDisplay)
	}
	settled, _ := versionPreview(current, next, prefix, bumpKind)
	return presenter.Gate{
		Stage:      stageVersion,
		Detail:     settled,
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

// selectState builds the notes precedence inputs for this run. A tagless repo
// resolves current to the zero SemVer, so an actual v0.0.0 tag also reads as a first
// release.
func selectState(current version.SemVer, cfg config.Config, opts ReleaseOptions, supplied *string) notes.SelectState {
	return notes.SelectState{
		Supplied:     supplied,
		FirstRelease: current == version.SemVer{},
		LastTag:      current.String(cfg.Release.TagPrefix),
		NoAI:         opts.NoAI,
	}
}

func notesFilePath(workDir, notesFile string) string {
	if workDir == "" || notesFile == "" || filepath.IsAbs(notesFile) {
		return notesFile
	}
	return filepath.Join(workDir, notesFile)
}

// notesExclusion is the zero Exclusion when no file was given, the file lies outside
// the repo, or git ignores it. An ignored file needs none, since git already leaves it
// out, and an exclusion naming it makes `add -A` and `stash push` fail.
func notesExclusion(ctx context.Context, r runner.CommandRunner, root, path string) (git.Exclusion, error) {
	rel := repoRelativePath(root, path)
	if rel == "" {
		return "", nil
	}
	res, err := r.Run(ctx, "git", "-C", root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--", git.LiteralPathspec(rel))
	if err != nil {
		return "", fmt.Errorf("checking whether git ignores --notes-file %s: %w", path, err)
	}
	if res.Stdout != "" {
		return "", nil
	}
	return git.Exclusion(rel), nil
}

// repoRelativePath is path relative to root, slash-separated, or "" when path is
// empty or lies outside the repo. The file's directory and root are both resolved
// through symlinks first, since git reports root by its real path; the file itself is
// not, as git tracks a symlink by its own path.
func repoRelativePath(root, path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	file := filepath.Join(realPath(filepath.Dir(abs)), filepath.Base(abs))
	rel, err := filepath.Rel(realPath(root), file)
	if err != nil || !filepath.IsLocal(rel) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func realPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// ReadNotesFile reads the --notes-file body, or returns nil when the flag was not
// given. An unusable file is shown on p as the notes stage's failure and returned as
// an exit-1 *AbortError.
func ReadNotesFile(p presenter.Presenter, path string, set bool) (*string, error) {
	supplied, err := readNotesFile(path, set)
	if err != nil {
		return nil, surface(p, "notes", err)
	}
	return supplied, nil
}

// readNotesFile reads the --notes-file body, or returns nil when the flag was not
// given. A given empty path names no file and fails as a missing file does. A blank
// body is refused, since git accepts an empty tag message and the run would ship it.
func readNotesFile(path string, set bool) (*string, error) {
	if path == "" && !set {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --notes-file %s: %w", path, err)
	}
	body := string(data)
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("--notes-file %s is empty", path)
	}
	return &body, nil
}

// resolveBody resolves this run's notes body + Kind via the notes-path precedence
// (SelectBody), returning the generator it built so the caller can bind a per-run
// regenerator for the gate's `r` choice. The selector's generation reports a chunked
// run to stage, saves a failed one's chunk notes through chunkNotes and keeps a
// written one's merge inputs there, and the selector hands stage the failure a
// fallback body ships in place of; the returned generator does none of these. It is the single Stage-4 body decision:
//
//   - opts.NotesBody is a TEST-INJECTION OVERRIDE: when non-empty it bypasses the
//     selector and returns (opts.NotesBody, opts.NotesKind, nil generator, nil). The
//     gate's `r` is never reached for the no-AI Kinds these overrides use, so a nil
//     generator is fine; a normal-AI override test supplies its own deps.Regenerator.
//   - otherwise the selector is built from deps.Runner (now that root is resolved) and
//     SelectBody runs the precedence over the run's SelectState.
func resolveBody(ctx context.Context, deps ReleaseDeps, root string, cfg config.Config, state notes.SelectState, chunkNotes *chunkNotesKeeper, versionKey string, opts ReleaseOptions, stage *notesStage, answers *answersCommit) (string, notes.Kind, *notes.Generator, error) {
	if opts.NotesBody != "" {
		return opts.NotesBody, opts.NotesKind, nil, nil
	}

	// One Assembler (the single git seam) is shared by the Generator and the Selector
	// so the degenerate-check diff and the AI path range over the same git, exactly as
	// NewSelector documents. On the forward path the version_file rule is inert (notes
	// generate at Stage 4 precedes the version write at Stage 5, so the file is unchanged
	// at notes time).
	assembler := notes.NewAssembler(deps.Runner, root, notesExcludeConfig(cfg))
	transport := aiTransport(deps, cfg)
	generator := notes.NewGenerator(assembler, transport, root)
	selecting := chunkNotes.sinks(stage).wire(generator)
	selector := notes.NewSelector(selecting, assembler, deps.Runner, root).
		WithFallbackNotice(stage.fellBack).
		WithExclusionCheck(exclusionCheck(deps, cfg, opts, transport, stage, answers))

	// SelectBodyWithReuse runs the precedence and surfaces, for the normal-AI path, the
	// diff the exclusions and the check's Exclude answers leave + resolved prompt/context
	// the cache key hashes. After the exclusion check, it consults the reuse hook BEFORE the notes
	// calls — a live cache match skips them and reuses the cached body. The hook differs
	// by mode: a real run PROMPTS (use/regenerate) on a match, a dry run reuses SILENTLY
	// (a dry run asks for nothing).
	// reuse records that the body came from the cache, so the write below skips a no-op
	// re-write, and keeps a chunked entry's merge inputs for the gate's `r`.
	reused := false
	reuse := func(entry notescache.Entry) {
		reused = true
		chunkNotes.merged = entry.Merge
	}
	body, kind, cacheInputs, err := selector.SelectBodyWithReuse(ctx, state, cfg, cacheReuse(deps, root, versionKey, opts.DryRun, reuse))
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
			if werr := deps.NoteCache.Write(root, key, body, chunkNotes.merged); werr != nil {
				warnNoteCacheFailed(deps.Presenter, werr)
			}
		}
		// Housekeeping: keep only the current diff's note, pruning entries left by earlier
		// runs (older diffs/versions) so the per-project cache never accumulates.
		deps.NoteCache.Prune(root, key)
	}
	return body, kind, generator, nil
}

// exclusionCheck is the release's exclusion check, pausing on its suggestions on an
// attended real run and recording the answers through answers, handing them to stage
// to print on an unattended one, and on an attended dry run showing each where its
// pause would be, unanswered.
func exclusionCheck(deps ReleaseDeps, cfg config.Config, opts ReleaseOptions, transport notes.Transport, stage *notesStage, answers *answersCommit) *notes.ExclusionCheck {
	check := notes.NewExclusionCheck(transport, cfg.Release.DiffKeep, stage)
	switch {
	case deps.asksAtTerminal(opts):
		return check.WithPauses(exclusionPauses{p: deps.Presenter}).WithRecord(answers.record(deps))
	case !deps.attended():
		return check.WithReport(stage.suggested)
	default:
		return check.WithReport(exclusionPauses{p: deps.Presenter}.preview)
	}
}

// answersCommit records the exclusion check's answers in root's .mint.toml and
// commits that file alone for tag's release, unless git ignores it; wrote reports
// that the write began, landed whether the commit was made, leftInPlace that git ignores the file so the write
// stays whatever the run does, position is how many of mint's commits it brought the
// run to, and edit is the write until a commit holds it or git is found to ignore it.
type answersCommit struct {
	root, tag   string
	globs       config.ReleaseGlobs
	wrote       bool
	landed      bool
	leftInPlace bool
	position    int
	edit        *fileEdit
}

// releaseTable is the .mint.toml table the answers are written to.
const releaseTable = "[release]"

func (a *answersCommit) record(deps ReleaseDeps) func(context.Context, notes.Answers) error {
	return func(ctx context.Context, answers notes.Answers) error {
		const writing = "recording the exclusion answers"
		edit, err := beforeEdit(a.root, config.FileName)
		if err != nil {
			return &answersRecordingError{step: writing, err: err, output: err.Error()}
		}
		a.globs = config.ReleaseGlobs{DiffExclude: answers.Exclude, DiffKeep: answers.Keep}
		a.wrote = true
		if err := a.write(); err != nil {
			return &answersRecordingError{step: writing, err: err, output: err.Error()}
		}
		a.edit = edit
		const committing = "committing the exclusion answers"
		ignored, err := record.IsIgnored(ctx, deps.Mutator, a.root, config.FileName)
		if err != nil {
			return &answersRecordingError{step: committing, err: err, output: gitOutput(err)}
		}
		// An ignored file is the operator's local config: staging it would take
		// `add -f`, and the unwind keeps the write as it keeps an answers commit.
		if !ignored {
			if err := a.commit(ctx, deps.Mutator); err != nil {
				return &answersRecordingError{step: committing, err: err, output: gitOutput(err)}
			}
			a.landed = true
		}
		a.leftInPlace = ignored
		a.edit = nil
		narrateAnswers(deps.Presenter, answers, ignored)
		return nil
	}
}

func narrateAnswers(p presenter.Presenter, answers notes.Answers, ignored bool) {
	p.SuspendSpinner()
	p.ExclusionAnswersRecorded(presenter.ExclusionAnswers{
		Stage:   "notes",
		Key:     stageCheck,
		File:    config.FileName,
		Table:   releaseTable,
		Ignored: ignored,
		Exclude: answers.Exclude,
		Keep:    answers.Keep,
	})
	p.ResumeSpinner()
}

func (a *answersCommit) write() error {
	return config.AppendReleaseGlobs(a.root, a.globs)
}

func (a *answersCommit) commit(ctx context.Context, m *git.Mutator) error {
	return record.CommitFile(ctx, m, a.root, config.FileName, record.AnswersSubject(a.tag))
}

// recommit records the answers again on whatever HEAD is now. On failure it returns
// the manual steps that finish the re-commit, given restore as the steps that bring
// back the answers commit itself, and the failing tool's own message.
func (a *answersCommit) recommit(ctx context.Context, m *git.Mutator, restore []manualStep) (manual []manualStep, output string, err error) {
	if err := a.write(); err != nil {
		return restore, err.Error(), err
	}
	if err := a.commit(ctx, m); err != nil {
		root := cmdline.Quote(a.root)
		return []manualStep{{
			does: "commit the exclusion answers mint wrote to " + config.FileName,
			commands: []string{
				"git -C " + root + " add " + config.FileName,
				"git -C " + root + " commit -m " + cmdline.Quote(record.AnswersSubject(a.tag)),
			},
		}}, gitOutput(err), err
	}
	return nil, "", nil
}

// answersRecordingError is the exclusion answers failing to be written or committed;
// output is the failing tool's own message.
type answersRecordingError struct {
	step   string
	err    error
	output string
}

func (e *answersRecordingError) Error() string { return e.step + ": " + e.err.Error() }

func (e *answersRecordingError) Unwrap() error { return e.err }

// gitOutput is the stderr git wrote for err, carried by a *record.GitError, or err's
// text when it wrote none.
func gitOutput(err error) string {
	var gitErr *record.GitError
	if errors.As(err, &gitErr) {
		return failedGitOutput(gitErr.Stderr, err)
	}
	return err.Error()
}

// failedGitOutput is what a failed git step shows beneath its warning or failure: the
// stderr git wrote, verbatim, or err's text when it wrote none.
func failedGitOutput(stderr string, err error) string {
	if stderr != "" {
		return stderr
	}
	return err.Error()
}

// exclusionPauses asks the operator about each suggestion at a gate, the spinner
// suspended while it waits.
type exclusionPauses struct {
	p presenter.Presenter
}

func (e exclusionPauses) Pause(_ context.Context, s notes.Suggestion, m notes.Matches) (notes.Answer, error) {
	e.p.SuspendSpinner()
	choice, err := e.p.Prompt(exclusionPauseGate(s, m))
	e.p.ResumeSpinner()
	if err != nil {
		return notes.AnswerKeep, err
	}
	switch choice {
	case presenter.ChoiceExclude:
		return notes.AnswerExclude, nil
	case presenter.ChoiceKeep:
		return notes.AnswerKeep, nil
	}
	return notes.AnswerKeep, errUnexpectedChoice(choice)
}

func (e exclusionPauses) preview(s notes.Suggestion, m notes.Matches) {
	e.p.SuspendSpinner()
	e.p.PreviewGate(exclusionPauseGate(s, m))
	e.p.ResumeSpinner()
}

func exclusionPauseGate(s notes.Suggestion, m notes.Matches) presenter.Gate {
	matches := describeMatches(m)
	return presenter.Gate{
		Stage:      stageCheck,
		Suggestion: &presenter.ExclusionSuggestion{Glob: s.Glob, Reason: s.Reason, Matches: matches},
		Question:   fmt.Sprintf("Exclude %s from the notes? (%s; %s)", s.Glob, s.Reason, matches),
		Subject:    s.Glob,
		AcceptEcho: "kept",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceExclude, Action: "exclude"},
			{Key: presenter.ChoiceKeep, Action: "keep"},
		},
		Default: presenter.ChoiceKeep,
	}
}

// describeMatches writes what an Exclude answer would take out of the notes.
func describeMatches(m notes.Matches) string {
	files := "1 file"
	if len(m.Paths) != 1 {
		files = fmt.Sprintf("%d files", len(m.Paths))
	}
	description := files + ", " + bytesize.Format(m.DiffBytes)
	if len(m.Kept) > 0 {
		description += "; on the keep list: " + strings.Join(m.Kept, ", ")
	}
	return description
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
//   - a live (within-TTL) match → hand the entry to reuse and return its body, so the
//     selector SKIPS the notes calls. The two modes differ ONLY here: a real run PROMPTS first
//     (use/regenerate — the spinner is suspended around reuseConfirmGate, so "regenerate"
//     re-runs the AI under a live spinner; under -y the gate auto-accepts "use"), while a
//     dry run reuses SILENTLY (a dry run asks for nothing — it is a no-action preview).
func cacheReuse(deps ReleaseDeps, root, versionKey string, dryRun bool, reuse func(notescache.Entry)) notes.ReuseFunc {
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
			reuse(entry)
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
		reuse(entry)
		return entry.Body, true, nil
	}
}

// reuseConfirmGate is the real-run prompt shown when a dry-run preview matches the
// current diff: use the cached notes, or regenerate fresh. Default is "use" (a bare
// Enter, and the -y auto-accept, reuse the preview). It declares y/r only — there is
// no edit here (the notes-review gate downstream owns editing).
func reuseConfirmGate() presenter.Gate {
	return presenter.Gate{
		Stage:      stageNotes,
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
// generator was built (the normal-AI selector path), it returns the gate's `r` rule
// (RegenRun.regenerator) over chunkNotes' regenRun, its size-check generation
// GenerateWithContext over lastTag + cfg, reporting to progress. When neither is
// present (an override body, or a no-AI path with no generator), it returns nil; `r`
// is only offered for KindNormalAI, so the nil is never consulted on the no-AI paths.
func perRunRegenerator(deps ReleaseDeps, generator *notes.Generator, chunkNotes *chunkNotesKeeper, progress notes.Progress, lastTag string, cfg config.Config) Regenerator {
	if deps.Regenerator != nil {
		return deps.Regenerator
	}
	if generator == nil {
		return nil
	}
	return chunkNotes.regenRun(progress).regenerator(generator, cfg, func(ctx context.Context, regenerating *notes.Generator, oneTimeContext string) (string, error) {
		return regenerating.GenerateWithContext(ctx, lastTag, cfg, oneTimeContext)
	})
}

// resolveHEAD reads the current commit SHA via `git rev-parse HEAD` through the
// runner seam (every git op goes through the seam). It captures the clean starting
// state before the gate — the exact sha the surgical unwind resets back to. The
// surgical unwind never probes HEAD to decide what to undo; it drives off the tracked
// MadeState.
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
		warnPublishDowngraded(deps.Presenter, err, "no provider release can be written")
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
	return failAndUnwind(ctx, deps, failedStage(stage, cause), start, made, cause)
}

// failAndUnwind is surfaceAndUnwind for a failure whose payload is already built.
func failAndUnwind(ctx context.Context, deps ReleaseDeps, failure presenter.StageFailure, start StartState, made MadeState, cause error) error {
	deps.Presenter.StageFailed(failure)
	return Unwind(ctx, deps, start, made, cause)
}

// pushFailure is the StageFailed payload for the tag and atomic push failing on cause
// after running for elapsed: labelled "tag", as plain has always shown it, and keyed
// to the push stage it fills.
func pushFailure(cause error, elapsed time.Duration) presenter.StageFailure {
	failure := stageFailure("tag", cause)
	failure.Key, failure.Elapsed = stagePush, elapsed
	return failure
}

// publishGateFailure is the StageFailed payload for the provider's resolution or its
// auth gate failing on cause: labelled "preflight", as plain shows it, and keyed to
// the publish stage it gates, since preflight has settled by then.
func publishGateFailure(cause error) presenter.StageFailure {
	failure := stageFailure(stagePreflight, cause)
	failure.Key = stagePublish
	return failure
}

// surfaceStageFailure renders stage's ✗ line for cause.
func surfaceStageFailure(p presenter.Presenter, stage string, cause error) {
	p.StageFailed(failedStage(stage, cause))
}

// stageFailure is the StageFailed payload for stage's cause, stage being its plan
// key too.
func stageFailure(stage string, cause error) presenter.StageFailure {
	return presenter.StageFailure{
		Name:    stage,
		Key:     stage,
		Message: failureMessage(cause),
		// failureOutput is called for EVERY stage — not special-cased to "notes" — because
		// it returns "" for any cause it reads no tool output from, leaving the ✗ line
		// standing alone.
		Output:     failureOutput(cause),
		SavedNotes: notes.SavedChunkNotesPath(cause),
	}
}

// runPreflight runs the Stage 2 gate chain in the spec's order: fetch (read-only,
// refreshes tags + upstream refs), then the cheap local gates, then the network
// gates. The first failure short-circuits and is returned for the caller to
// surface and abort on. The conditional gh gate is run separately by Release
// (only when publishing, after the bookkeeping commit and before the tag).
//
// anyBranch is the --any-branch escape hatch: it is passed to RunLocalGates, which
// SKIPS the on-release-branch gate when it is true (the gate is not evaluated). It
// affects ONLY the branch gate — fetch, tag-free, and remote-sync all run unchanged.
//
// skipCleanTree is the dry-run+autostash bypass: it is passed to RunLocalGates, which
// SKIPS the clean-tree gate when it is true (no `git status --porcelain` probe). The
// caller sets it ONLY for DryRun && AutoStash — a dry run skips the real autostash, so
// the clean-tree gate it would have satisfied is bypassed too, letting a dirty-tree dry
// run finish its preview. It affects ONLY the clean-tree gate.
//
// keepOut is the path the clean-tree gate does not count.
func runPreflight(ctx context.Context, r runner.CommandRunner, releaseBranch, tag string, anyBranch, skipCleanTree bool, keepOut git.Exclusion) error {
	if err := preflight.Fetch(ctx, r); err != nil {
		return err
	}
	if err := preflight.RunLocalGates(ctx, r, releaseBranch, tag, anyBranch, skipCleanTree, keepOut); err != nil {
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
// sites so the convention is consistent. It rides the Warn seam (see dryRunLabel),
// Skipped and keyed to the hook's own node, which the rail leaves to come. It returns
// the message, for a skip whose hook has no node of its own to carry it another way.
func reportHookSkipped(p presenter.Presenter, point string) string {
	message := "skipping " + point + " hook"
	p.Warn(presenter.Warning{Label: dryRunLabel, Key: point, Skipped: true, Message: message})
	return message
}

// hookRuns reports whether a hook point configured with value runs on this run: it
// has entries and the run is not a dry run, which skips every hook.
func hookRuns(value config.HookValue, dryRun bool) bool {
	return hooks.HasEntries(value) && !dryRun
}

// runPreflightHook runs the project's optional preflight hook through the shared
// hooks runner. It assembles the MINT_* env from the run's computed versions
// (NewVersion = the bare version being released, PreviousVersion = the bare prior
// latest, VersionTag = the full prefixed tag), the bump kind, and the dryRun mode,
// then runs the configured [release.hooks].preflight value from the repo root. An
// absent or empty value (nil, "", []) is a silent no-op: no run, no report. A
// non-zero exit (the first non-zero entry for an array) returns a non-nil error the
// caller surfaces and aborts on.
//
// DRY-RUN: when dryRun is active AND the hook is configured, the hook is NOT
// invoked — no `sh -c …` reaches the runner; instead the skip is REPORTED via the
// presenter and its message returned, for the preflight row to hang: the hook has no
// node of its own. Once the hook runs the run's close says only that nothing was
// pushed: what the hook did is unknown to mint.
func runPreflightHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool) (skipped string, err error) {
	if !hooks.HasEntries(cfg.Release.Hooks.Preflight) {
		return "", nil
	}
	if dryRun {
		return reportHookSkipped(deps.Presenter, "preflight"), nil
	}
	env := buildHookEnv(current, next, tag, bump, dryRun)
	deps.run.Left(nothingPushed)
	return "", hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.Preflight, root, env)
}

// runPreTagHook runs the project's optional Stage-3 pre_tag hook and then applies
// the artifact-commit interplay rule. The hook (build/generate artifacts) runs
// through the shared hooks runner with the same MINT_* env as the preflight hook. An
// absent or empty value (nil, "", []) is a no-op — nothing runs, no artifact commit
// is considered, and committed is false. A non-zero exit (the first non-zero entry
// for an array) returns a non-nil error the caller routes through the surgical
// unwind.
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
// applies ONLY after a hook actually ran: an absent or empty hook skips the
// artifact-commit probe entirely, so the existing no-hook spine is untouched.
//
// DRY-RUN: when dryRun is active AND the hook is configured, the hook is NOT invoked
// and — crucially — the artifact-commit step is SKIPPED TOO: because the hook never
// ran, the tree was not dirtied by mint, so NO porcelain probe and NO
// `chore(release): pre-tag artifacts for {tag}` commit must be produced (committed is
// false). The skip is reported via the presenter and the function returns immediately.
// An absent or empty hook stays a silent no-op (no run, no report, no probe).
//
// keepOut is left out of the artifact commit: its changes neither trigger nor ride in it.
//
// A failed artifact commit after its stage is a *record.UncommittedError in the chain:
// the hook's changes are left staged.
func runPreTagHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool, keepOut git.Exclusion) (bool, error) {
	if !hooks.HasEntries(cfg.Release.Hooks.PreTag) {
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
	stage := emitBlockingStageStarted(deps.Presenter, deps.clock(), stagePreTag, "running pre_tag hook…", "Ran the pre_tag hook")
	env := buildHookEnv(current, next, tag, bump, dryRun)
	if err := hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PreTag, root, env); err != nil {
		return false, &stageError{stage: stage, err: err}
	}
	subject := pretagArtifactSubject(tag)
	committed, err := record.CommitDirtyTree(ctx, deps.Mutator, root, subject, keepOut)
	if err != nil {
		return committed, &stageError{stage: stage, err: err}
	}
	success := presenter.StageSuccess{Detail: "hook completed", Sentence: stage.sentence}
	if committed {
		success.Commit = subject
	}
	stage.succeeded(success)
	return committed, nil
}

// stageError is a blocking stage's failure, carrying the stage so its ✗ line shows
// how long it ran.
type stageError struct {
	stage *blockingStage
	err   error
}

func (e *stageError) Error() string { return e.err.Error() }

func (e *stageError) Unwrap() error { return e.err }

// failedStage is the StageFailed payload for the stage name failing on cause, timed
// when cause carries the blocking stage that failed.
func failedStage(name string, cause error) presenter.StageFailure {
	var timed *stageError
	if errors.As(cause, &timed) {
		return timed.stage.failure(cause)
	}
	return stageFailure(name, cause)
}

// runPostReleaseHook runs the project's optional Stage-7 post_release hook through
// the shared hooks runner with the same MINT_* env as the other points (reusing
// buildHookEnv). An absent or empty value (nil, "", []) is a silent no-op: no run,
// no report, no stage narration. A non-zero exit (the first non-zero entry for an
// array — the stop-on-first-failure SEQUENCING is identical to the other points)
// returns a non-nil error. UNLIKE preflight/pre_tag, the CONSEQUENCE here is
// warn-only: the caller does NOT abort or unwind, because by Stage 7 the push has
// crossed the PONR and the tag is public.
//
// DRY-RUN: when dryRun is active AND the hook is configured, the hook is NOT invoked;
// the skip is REPORTED via the presenter and nil is returned (the caller's warn-only
// branch is irrelevant — there is no failure).
func runPostReleaseHook(ctx context.Context, deps ReleaseDeps, cfg config.Config, root string, current, next version.SemVer, tag string, bump version.Bump, dryRun bool) error {
	if !hooks.HasEntries(cfg.Release.Hooks.PostRelease) {
		return nil
	}
	if dryRun {
		reportHookSkipped(deps.Presenter, "post_release")
		return nil
	}

	// Stage 7 is BLOCKING, as pre_tag is: a post-release build, notarization or
	// upload can run for minutes, so the spinner must show it is alive. A failure
	// skips the success line; the caller's Warn stops the spinner and prints the ⚠
	// line with the hook's output in its place.
	stage := emitBlockingStageStarted(deps.Presenter, deps.clock(), stagePostRelease, "running post_release hook…", "Ran the post_release hook")
	env := buildHookEnv(current, next, tag, bump, dryRun)
	if err := hooks.NewRunner(deps.Runner).Run(ctx, cfg.Release.Hooks.PostRelease, root, env); err != nil {
		return err
	}
	stage.done("hook completed")
	return nil
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

// pushedBranch is the branch the atomic push sends: HEAD's, which preflight holds to
// the release branch unless --any-branch bypassed that gate.
func pushedBranch(releaseBranch string, head gitrepo.Head, anyBranch bool) string {
	if anyBranch && head.Branch != "" {
		return head.Branch
	}
	return releaseBranch
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
// AFTER this boundary) and lands the dry run, which Release closes with the "no
// changes made" close-out, returning nil
// WITHOUT any mutation. No commit, tag, push, or provider release is reached, so the
// working tree is byte-for-byte unchanged (apart from the user-level note cache). The
// preflight and pre_tag hook skips already fired above this point.
func finishDryRun(ctx context.Context, deps ReleaseDeps, cfg config.Config, root, versionKey string, current, next version.SemVer, tag string, bumpKind version.Bump) error {
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

	deps.run.Land(presenter.RunResult{
		Project: projectName(root),
		Version: versionKey,
		Leaf:    cfg.Release.CommitPrefix,
		Outcome: presenter.OutcomeDryRun,
		End:     presenter.Station{Name: tag, Detail: "dry run · nothing changed"},
	})
	return nil
}

var errEditorUnavailable = errors.New("edit chosen but no editor is configured")

var errRegeneratorUnavailable = errors.New("regenerate chosen but no regenerator is configured")

// reviewGate runs the notes review gate as the engine-owned re-entry LOOP and
// returns the (possibly edited or regenerated) FINAL body the caller threads to
// every sink. Rendering stays in the presenter; this owns only the semantics.
//
// The gate is supplied by the caller. The loop is gate-agnostic — it acts on the
// choice keys the supplied gate declares, and the SAME gate is re-rendered on every
// iteration.
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
//     Unwound.
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
//     regenerateBody. It reads a one-time context line via AskLine, hands it to the
//     Regenerator,
//     REPLACES the body with the regenerated text, re-shows the notes, and LOOPS so
//     the user may regenerate again or settle on y/n/e.
//
// The gate never unwinds or emits an Unwound; any reset is the caller's.
func reviewGate(ctx context.Context, deps ReleaseDeps, regen reviewRegen, gate presenter.Gate, body, versionKey string) (string, error) {
	p := deps.Presenter
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
			editDone := emitBlockingStageStarted(p, deps.clock(), "edit", "applying edited notes…", "Applied edited notes")
			edited, eerr := editBody(ctx, deps.Editor, body)
			switch {
			case errors.Is(eerr, ErrEditorReturnToGate):
				// No editor could be launched: the launcher already reported the problem
				// via the presenter. Close the edit stage (stopping the spinner) and
				// RE-PRESENT the gate with the body UNCHANGED — this is not a failure, so do
				// not surface or abort, and do not re-render.
				editDone.done("notes updated")
				continue
			case eerr != nil:
				// A genuine edit failure (a launched-but-failed editor, an IO error, or
				// the nil-editor misconfiguration): surface and abort.
				return "", surface(p, "edit", eerr)
			}
			editDone.done("notes updated")
			// Use the edited text VERBATIM — no re-parse, no re-validation — then
			// re-render the notes and loop back to re-prompt.
			body = edited
			p.ShowNotes(presenter.Notes{Version: versionKey, Body: body})
		case presenter.ChoiceRegen:
			// Regenerate (only reachable on the four-choice gate): regenerate with a
			// one-time context line, replace the body, re-show, and loop. Any failure
			// (closed input, nil seam, regenerate error) is surfaced and aborts.
			regenerated, rerr := regenerateBody(ctx, deps, regen, versionKey)
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
// path with an AI to regenerate), and EVERY other Kind gets the y/n/e gate with no
// `r` — offering `r` there would be meaningless (no AI to nudge) and, under --no-ai,
// would contradict the flag.
func gateForKind(kind notes.Kind) presenter.Gate {
	gate := FirstReleaseReviewGate()
	if kind == notes.KindNormalAI {
		gate = presenter.ReleaseReviewGate()
	}
	gate.Stage = stageNotes
	return gate
}

// regenerateBody runs the `r` regenerate step and returns the regenerated body.
// It reads a one-time context line via the presenter's AskLine input seam (the
// engine NEVER reads stdin directly), an EMPTY answer being legal (regenerate with
// no extra context), then hands that line to the Regenerator. Each failure mode is
// fail-loud:
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
// On success the regenerated body is re-shown (so the user reviews it), regen.shown
// runs, and the body is returned; the caller replaces the body and loops the gate.
func regenerateBody(ctx context.Context, deps ReleaseDeps, regen reviewRegen, versionKey string) (string, error) {
	p := deps.Presenter
	line, err := p.AskLine(regenContextPrompt)
	if err != nil {
		// A closed/non-interactive input on the one-time-context read is fail-loud: the
		// presenter leaves ErrInputClosed unrendered, so the abort (and any closing
		// narration) is the engine's to surface — done via abort, mirroring ReviewDecision.
		return "", abort(regen.keepFailed(err))
	}

	if regen.regenerator == nil {
		// `r` was offered (KindNormalAI) but no Regenerator was wired — a
		// misconfiguration. Surface a clean failure and abort rather than panicking.
		return "", regenFailed(p, errRegeneratorUnavailable, 0)
	}

	now := deps.clock()
	began := now()
	regenerated, rerr := regen.regenerator.Regenerate(ctx, line)
	if rerr != nil {
		// A regenerate failure is fail-loud: surface and abort.
		return "", regenFailed(p, regen.keepFailed(rerr), now().Sub(began))
	}

	// Re-show the regenerated notes so the user reviews them before the gate re-prompts.
	p.ShowNotes(presenter.Notes{Version: versionKey, Body: regenerated})
	if regen.shown != nil {
		regen.shown()
	}
	return regenerated, nil
}

// regenFailed surfaces an `r` that failed after elapsed and aborts. Its line keeps
// the "regenerate" label, but it is the notes being written again, so it carries the
// notes stage's key.
func regenFailed(p presenter.Presenter, cause error, elapsed time.Duration) error {
	failure := stageFailure(regenerateLabel, cause)
	failure.Key, failure.Elapsed = stageNotes, elapsed
	p.StageFailed(failure)
	return abort(cause)
}

// regenerateLabel names the ✗ line of a failed `r`.
const regenerateLabel = "regenerate"

// errUnexpectedChoice builds the cause for a choice outside the gate's declared set —
// a presenter-contract violation the engine refuses to ignore.
func errUnexpectedChoice(choice presenter.Choice) error {
	return fmt.Errorf("unexpected gate choice %q", choice)
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

// warnPublishDowngraded emits the LOUD warning when the publishing provider could not
// be resolved. It NAMES the reason (an unsupported provider value, an unrecognised
// host, no remote, or an unparseable remote) so the user can see WHY publishing was
// skipped, and consequence says what the run does instead: a release downgrades to
// tag + push only, while a regenerate, which cuts no tag, writes no provider release.
// It rides the Warn seam (which does not set failure state), so the run proceeds and
// finishes successfully — distinct from publish=false, which is a SILENT opt-out with
// no warning at all.
func warnPublishDowngraded(p presenter.Presenter, cause error, consequence string) {
	p.Warn(presenter.Warning{
		Key:     stagePublish,
		Label:   "publish skipped",
		Message: "provider could not be resolved (" + downgradeReason(cause) + "); " + consequence,
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
// must NOT unwind. It warns and points at the heal path, the regenerate that publishes
// version's release from its tag; the run still finishes successfully. Plain's line
// names the heal; the rail's row says what happened, and the heal's full command
// reaches it as a Quiet next move.
func warnPublishFailed(p presenter.Presenter, cause error, version string) {
	p.Warn(presenter.Warning{
		Key:     stagePublish,
		Label:   "publish failed",
		Message: "tag is already published; heal with regenerate --source tag",
		Row:     "provider release failed · the tag is already pushed",
		Output:  cause.Error(),
	})
	p.NextMoves(presenter.NextMoves{Stage: stagePublish, Quiet: true, Moves: []presenter.NextMove{{
		Lead:    "Publish the release from the tag's notes:",
		Command: "mint release regenerate " + cmdline.Quote(version) + " --source tag --target release",
	}}})
}

// warnPostReleaseFailed emits the post-PONR warn-only event for a failed
// post_release hook: by Stage 7 the push has crossed the point of no return, so the
// tag is public and mint must NOT unwind. It warns (the spec-fixed message) and the
// run still finishes successfully. When the failure carries a *hooks.HookError its
// captured output (the failing entry's stderr) is rendered beneath the warn line, as
// warnPublishFailed does for the provider error; otherwise the Output is empty.
func warnPostReleaseFailed(p presenter.Presenter, cause error) {
	p.Warn(presenter.Warning{
		Key:     stagePostRelease,
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

// failureOutput is the failing tool's own message for cause, shown beneath its ✗ line:
// a failed answers recording's write error or git stderr, a failed preflight or
// pre_tag hook entry's stderr, or claude's output.
func failureOutput(cause error) string {
	var recordingErr *answersRecordingError
	if errors.As(cause, &recordingErr) {
		return recordingErr.output
	}
	if output := hookFailureOutput(cause); output != "" {
		return output
	}
	return notesFailureOutput(cause)
}

func notesFailureOutput(cause error) string {
	var genErr *ai.GenerationError
	if !errors.As(cause, &genErr) {
		return ""
	}
	return genErr.Output()
}

// surface renders a stage failure through the presenter and returns the engine's
// typed abort carrying a non-zero exit code. It is the single pre-PONR failure
// path: every stage-1–8 error flows through here so the failure is both shown and
// terminal. A *GateError surfaces its actionable Message; any other error
// surfaces its Error() text.
func surface(p presenter.Presenter, stage string, cause error) error {
	surfaceStageFailure(p, stage, cause)
	return abort(cause)
}

// failureMessage extracts the display message for a stage failure — the SINGLE
// display-derivation seam every StageFailed site (surface, surfaceAndUnwind,
// resetAndAbort) funnels its Message through:
//
//   - a preflight *GateError carries an actionable, display-ready Message;
//   - a failed answers recording names its step, its tool's message going beneath;
//   - a notes/AI failure collapses to the CONCISE cause phrase notes.CauseText derives
//     through the %w chain, so it resolves the same phrase whether the cause carries
//     the forward abortError chain ("notes generation failed (%s): %w") or
//     regenerate's shorter "generating notes: %w" chain, instead of rendering the
//     verbose nested %w concatenation. The %w chain is left intact for errors.Is/logs;
//     only this DISPLAY string changes. Ordered after the gate branch because a gate
//     error never wraps a notes-failure sentinel.
//   - everything else falls back to the wrapped error text (the defensive path for an
//     unmapped cause, e.g. resetAndAbort's git record/push failure).
func failureMessage(cause error) string {
	var gate *preflight.GateError
	if errors.As(cause, &gate) {
		return gate.Message()
	}
	var recordingErr *answersRecordingError
	if errors.As(cause, &recordingErr) {
		return recordingErr.step
	}
	if phrase, known := notes.CauseText(cause); known {
		return phrase
	}
	return cause.Error()
}

// projectName derives the project label from the repo root's final path segment.
// Phase 1 has no configured project name, so the directory basename is the natural
// stand-in for the brand/header lines.
func projectName(root string) string {
	return filepath.Base(root)
}
