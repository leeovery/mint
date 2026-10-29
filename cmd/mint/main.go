// Command mint is the entry point for the mint release tool. main stays thin: it
// parses the CLI surface, constructs the production seams ONCE (the presenter, the
// os/exec runner, the releaser), runs the engine
// orchestrator, and resolves the engine's typed abort into a process exit code.
// All orchestration lives in the testable internal/engine.Release — main itself
// performs no release logic, so the spine is driven in tests with a recording
// presenter and a fake runner, never through os.Exit or real git.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"time"

	"mint/internal/buildinfo"
	"mint/internal/commit"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/gitrepo"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/publish"
	releasing "mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

// usageExitCode is the Unix usage-error code returned for an invalid invocation
// (bad flags, unknown command), distinct from a runtime failure's exit code which
// the engine's *AbortError carries.
const usageExitCode = 2

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the thin testable shell of main: it dispatches the parsed command, wires
// the production seams, runs the engine, and returns the process exit code WITHOUT
// calling os.Exit (so a future test could drive it). It returns 0 on success, the
// engine abort's non-zero ExitCode on a pre-PONR abort, and usageExitCode on a
// CLI parse error.
func run(args []string) int {
	// `mint --version` is the standard global-flag spelling of the version surface:
	// it is handled BEFORE command classification (independently of any subcommand)
	// and routes through the SAME runVersion as the `version` verb, so the two are
	// byte-identical. It deliberately needs no git repo — and no signal context, since
	// it spawns nothing to interrupt.
	if hasVersionFlag(args) {
		return runVersion(os.Stdout, os.Stderr, os.Stdin)
	}

	// `mint help` / `mint --help` is the top-level help surface — a requested action,
	// not a usage error, so it prints to stdout and exits 0. Per-verb help
	// (`mint <command> --help`) is handled inside each verb's runner via flag.ErrHelp.
	if isHelpCommand(args) {
		_, _ = fmt.Fprint(os.Stdout, rootUsage)
		return 0
	}

	// Build the ONE signal-cancellable context for the whole run: a Ctrl-C
	// (os.Interrupt) or SIGTERM cancels it, which threads down through every command's
	// external-command seam so a long pre-PONR step (the AI call, a hook, the gap before
	// the atomic push) is interrupted. The engine treats a pre-PONR cancellation as a
	// failure routed through its surgical unwind (resets, tag delete, autostash pop), so
	// the repo is left clean; a post-PONR cancellation keeps the existing warn-only
	// contract. stop() restores the default signal handling on return.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// FIRST Ctrl-C: graceful — the cancellation threads through every subprocess
	// seam. SECOND Ctrl-C: fatal — once the context is cancelled, restore default
	// signal delivery so another interrupt terminates the process immediately. This
	// is the escape hatch for any read the cancellation cannot reach (the
	// cooked-mode gate/AskLine stdin reads are not ctx-aware): without it,
	// NotifyContext keeps intercepting and every subsequent Ctrl-C is swallowed —
	// observed live as an unkillable gate. stop() is idempotent, so the deferred
	// call above stays correct on the normal-exit path.
	go func() {
		<-ctx.Done()
		stop()
	}()

	// `regenerate` is a subcommand of `release` (`mint release regenerate …`), not a
	// top-level verb; `init` and `version` are top-level verbs; classifyCommand
	// resolves the route. An unknown command is a usage error.
	kind, rest := classifyCommand(args)
	switch kind {
	case commandRelease:
		return runRelease(ctx, rest)
	case commandRegenerate:
		return runRegenerate(ctx, rest)
	case commandInit:
		return runInit(ctx, rest)
	case commandVersion:
		return runVersion(os.Stdout, os.Stderr, os.Stdin)
	case commandCommit:
		return runCommit(ctx, rest)
	case commandSetup:
		// setup spawns nothing to interrupt and resolves no repo, so — like version —
		// it takes only IO descriptors and no ctx.
		return runSetup(rest, os.Stdout, os.Stderr)
	default:
		fmt.Fprintln(os.Stderr, unknownCommandMessage)
		return usageExitCode
	}
}

// unknownCommandMessage is the stderr diagnostic for an unrecognised or empty
// command. It enumerates every wired command (so a typo names the full surface),
// and is a named const so a coverage test can pin that every route is listed.
const unknownCommandMessage = "mint: unknown command (only `mint release`, `mint release regenerate`, `mint init`, `mint version`, `mint commit`, and `mint setup` are wired)"

// runRegenerate parses and validates the `mint release regenerate` flag surface,
// runs the applicable preflight subset, then dispatches to the single-version
// interactive run or the --all batch backfill. After the structural parse it loads
// config (to read the changelog toggle) and runs the semantic axis validation: the
// source and target axes are orthogonal (any source can write any target), a
// changelog/both target is rejected when the changelog is disabled, and a -y run needs
// an explicit --target (no source has a safe default surface to guess unattended). The
// only mutation/network beyond reading the repo root + config happens inside the
// dispatched run.
func runRegenerate(ctx context.Context, rest []string) int {
	req, err := parseRegenerateFlags(rest)
	if err != nil {
		// -h/--help is a requested action, not a usage error: print the usage text to
		// stdout and exit 0 (the flag set's own dump is discarded at parse time).
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(os.Stdout, regenerateUsage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Regenerate's pretty output is the Rail, as release's is, whose drawing ends once
	// the run is over, however it ends.
	p := presenter.NewForStartup(req.Plain, req.Yes, os.Stdout, os.Stderr, os.Stdin, presenter.WithRail())
	defer presenter.Close(p)
	req.StdinInteractive = presenter.DetectStartupSignals(req.Plain, os.Stdout, os.Stdin).StdinInteractive
	return regenerate(ctx, p, runner.NewExecRunner(), req, notescache.NewUserChunkNotes(os.UserCacheDir))
}

// regenerate runs a parsed regenerate invocation. Any --notes-file is read first, so
// an unusable one is refused ahead of the repo, config and release-branch resolution.
func regenerate(ctx context.Context, p presenter.Presenter, r runner.CommandRunner, req regenerateRequest, chunkNotes engine.ChunkNotesSaver) int {
	supplied, err := engine.ReadNotesFile(p, req.NotesFile, req.NotesFileSet)
	if err != nil {
		return exitCode(err)
	}

	// The changelog-disabled axis check needs the loaded config; resolve the repo
	// root and load it the same way the forward release pipeline does (read-only —
	// no mutation, no network).
	root, err := gitrepo.ResolveRoot(ctx, r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return exitCode(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return exitCode(err)
	}

	validated, err := validateRegenerateRequest(req, cfg.Release.Changelog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Resolve the release branch (read-only, the same way the forward path does) for the
	// preflight on-branch / remote-sync gates. Both dispatch paths thread it into their
	// run (RegenerateAllRun, RegenerateRunResolving), which preflights the RESOLVED
	// target AFTER the interactive source/target prompts — the only point at which a
	// bare invocation (no --target) knows which surface(s) it writes.
	releaseBranch, err := gitrepo.ResolveReleaseBranch(ctx, r, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return exitCode(err)
	}
	deps := engine.ReleaseDeps{
		Presenter: p,
		Runner:    r,
		Mutator:   git.NewMutator(r),
		// The `e` choice at the notes-review gate and at the notes floor hands the notes
		// to the real $EDITOR resolution, launched through the same presenter + runner.
		Editor:           engine.NewEditorLauncher(p, r),
		ChunkNotes:       chunkNotes,
		Yes:              req.Yes,
		StdinInteractive: req.StdinInteractive,
		MintVersion:      buildinfo.Stamped(),
		Home:             homeDir(),
	}

	// The --all batch backfill (every version, oldest → newest) and the single-version
	// interactive run share the resolved deps/runner/config/root; both thread the
	// resolved release branch so the engine can preflight the RESOLVED target subset
	// AFTER the interactive axis prompts (a bare invocation does not know its surface(s)
	// until the source/target prompts resolve — the cmd layer cannot run that gate set).
	if validated.All {
		return runRegenerateAll(ctx, deps, r, cfg, root, releaseBranch, validated)
	}

	return runRegenerateSingle(ctx, deps, r, cfg, root, releaseBranch, validated, supplied)
}

// runRegenerateSingle executes one single-version interactive regenerate run: it
// resolves the <version> argument to its canonical tag + fresh diff base (task 5-3),
// then runs the interactive default flow (task 5-10) — resolving the publishing driver
// once the run has opened, asking source/target as needed, preflighting the RESOLVED target's gate subset,
// showing the plan, and confirming before the write. releaseBranch threads to the
// engine for the preflight on-branch / remote-sync gates, which run AFTER the
// interactive target resolves (a bare `regenerate <ver>` does not know its surface(s)
// until then). supplied is the --notes-file body read at startup, nil without the flag.
func runRegenerateSingle(ctx context.Context, deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root, releaseBranch string, req regenerateRequest, supplied *string) int {
	res, err := version.ResolveRegenerateTarget(ctx, r, cfg.Release.TagPrefix, req.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return exitCode(err)
	}

	// The bare x.y.z key (no tag prefix) used in the changelog header and the
	// notes/plan, recovered by re-parsing the canonical tag.
	versionKey, err := version.ParseSemVer(res.Tag, cfg.Release.TagPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return exitCode(err)
	}

	runReq := regenerateRunFlags(req)
	runReq.Supplied = supplied
	runReq.Tag = res.Tag
	runReq.VersionKey = versionKey.String("")
	runReq.DiffRange = res.DiffRange()
	runReq.Project = filepath.Base(root)
	runReq.ReleaseBranch = releaseBranch
	runReq.ChangelogEnabled = cfg.Release.Changelog
	runReq.Config = cfg
	// The publishing driver resolves inside the run, through the SHARED engine helper,
	// which handles an unresolvable provider exactly as the forward engine.Release path
	// does — warn-and-downgrade (a nil publisher proceeds, the provider write is skipped
	// downstream) or a clean abort. The body producer reads the driver it resolved.
	var publisher publish.Publisher
	resolve := func(ctx context.Context) (publish.Publisher, error) {
		var err error
		publisher, err = engine.ResolvePublisher(ctx, deps, cfg)
		return publisher, err
	}
	runReq.ProduceBody = func(ctx context.Context, source engine.RegenerateSource, sinks engine.NotesSinks) (string, error) {
		return newRegenerateBodyProducer(r, cfg, root, res, publisher)(ctx, source, sinks)
	}
	// The fresh notes-review gate's `r` choice consults this per-run regenerator,
	// bound to the resolved range — the regenerate analogue of the forward path's
	// per-run regenerator. Without it the rendered `r` would abort.
	runReq.ProduceRegenerator = newRegenerateRegeneratorProducer(r, cfg, root, res)

	if err := engine.RegenerateRunResolving(ctx, deps, root, runReq, resolve); err != nil {
		return exitCode(err)
	}
	return 0
}

// runRelease is the thin shell of `mint release`: it builds the production seams
// from the os globals and hands them to release.
func runRelease(ctx context.Context, rest []string) int {
	opts, err := parseReleaseFlags(rest)
	if err != nil {
		// -h/--help is a requested action, not a usage error: stdout, exit 0.
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(os.Stdout, releaseUsage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// The engine needs the stdin signal to tell an attended run, and the Presenter
	// interface exposes no accessor for it.
	signals := presenter.DetectStartupSignals(opts.Plain, os.Stdout, os.Stdin)

	// Construct the presenter ONCE at startup — the single production construction
	// site. TTY/mode detection never happens downstream. Release's pretty output is
	// the Rail, whose drawing ends once the run is over, however it ends.
	p := presenter.NewForStartup(opts.Plain, opts.Yes, os.Stdout, os.Stderr, os.Stdin, presenter.WithRail())
	defer presenter.Close(p)

	return release(ctx, p, runner.NewExecRunner(), opts, signals.StdinInteractive,
		newNoteCache(), notescache.NewUserChunkNotes(os.UserCacheDir))
}

// release runs a parsed `mint release` invocation over the given seams, returning
// the process exit code.
func release(ctx context.Context, p presenter.Presenter, r runner.CommandRunner, opts releaseFlags, stdinInteractive bool, noteCache engine.NoteCache, chunkNotes engine.ChunkNotesSaver) int {
	// One runner backs every external-command seam. The lock-resilient git Mutator is
	// constructed ONCE over that runner and shared by the engine's mutation calls and
	// the Releaser, so every git mutation (commit, tag, push, reset, tag-delete) retries
	// a contended .git lock and clears a provably-stale one. Read-only git stays on r.
	mut := git.NewMutator(r)
	deps := engine.ReleaseDeps{
		Presenter: p,
		Runner:    r,
		Mutator:   mut,
		Releaser:  releasing.NewReleaser(mut),
		// The `e` review-gate choice hands the notes to the real $EDITOR resolution,
		// launched interactively through the same presenter + runner. The launcher
		// reports a missing editor and returns to the gate rather than aborting.
		Editor:           engine.NewEditorLauncher(p, r),
		NoteCache:        noteCache,
		ChunkNotes:       chunkNotes,
		Yes:              opts.Yes,
		StdinInteractive: stdinInteractive,
		MintVersion:      buildinfo.Stamped(),
		Home:             homeDir(),
	}

	if err := engine.Release(ctx, deps, opts.ReleaseOptions()); err != nil {
		return exitCode(err)
	}
	return 0
}

// runCommit wires the production seams and runs the bare `mint commit` generate-and-
// commit thread for a parsed `mint commit` invocation, returning the process exit
// code. All orchestration lives in the testable commit.Run; main stays thin.
func runCommit(ctx context.Context, rest []string) int {
	opts, err := parseCommitFlags(rest)
	if err != nil {
		// -h/--help is a requested action, not a usage error: stdout, exit 0.
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(os.Stdout, commitUsage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Resolve the run-level startup signals ONCE, each from its own descriptor (stdout
	// for render Mode, stdin for the gating StdinInteractive). NewForStartup does its own
	// internal detection over the same inputs (so the presenter's gate posture is
	// consistent); we additionally surface StdinInteractive as a boolean to the engine's
	// editor-fallback no-message-source guard, which the Presenter interface exposes no
	// accessor for.
	signals := presenter.DetectStartupSignals(opts.Plain, os.Stdout, os.Stdin)

	// Construct the presenter ONCE at startup — the single production construction
	// site. TTY/mode detection never happens downstream. Commit's pretty output is the
	// Rail, whose drawing ends once the run is over, however it ends.
	p := presenter.NewForStartup(opts.Plain, opts.Yes, os.Stdout, os.Stderr, os.Stdin, presenter.WithRail())
	defer presenter.Close(p)

	// One runner backs the read-side staged-diff seam; the lock-resilient git Mutator
	// is constructed ONCE over it and serves as the commit sink so the commit mutation
	// retries a contended .git lock and clears a provably-stale one. Read-only git
	// (the staged diff) stays on the raw runner.
	r := runner.NewExecRunner()
	deps := commit.Deps{
		Presenter: p,
		Runner:    r,
		Mutator:   git.NewMutator(r),
		// Thread the resolved staging mode (StagedOnly by default; All/-a or AddAll/-A
		// when given) into the orchestrator. Phase 2's deferred-staging tasks consume it;
		// StagedOnly leaves the bare path unchanged.
		Staging: opts.Staging,
		// --no-ai routes past AI generation + the Continue? gate to the $EDITOR fallback.
		NoAI: opts.NoAI,
		// Thread the -y flag and the startup-resolved stdin-interactivity signal into the
		// editor-fallback no-message-source guard: under -y or a non-TTY stdin a fallback
		// has no human to write a message, so it fails loud rather than hangs.
		Yes:              opts.Yes,
		StdinInteractive: signals.StdinInteractive,
		// Thread the -p/--push armed value (flag-only; no config default). Phase 5's later
		// tasks consume it to push after a successful commit; disarmed leaves the path
		// push-free.
		Push:        opts.Push,
		MintVersion: buildinfo.Stamped(),
		Home:        homeDir(),
	}

	if err := commit.Run(ctx, deps); err != nil {
		return exitCode(err)
	}
	return 0
}

// commandKind is the resolved top-level route for an invocation. The zero value
// is commandUnknown so an unrecognised or empty command is a usage error by
// default.
type commandKind int

const (
	// commandUnknown is an unrecognised or empty command (a usage error). It is the
	// zero value, so `mint regenerate` (regenerate is NOT a top-level verb) and a
	// bare `mint` both fall here.
	commandUnknown commandKind = iota
	// commandRelease is the forward `mint release [bump]` cut action.
	commandRelease
	// commandRegenerate is the `mint release regenerate …` subcommand — a
	// subcommand of release, never a top-level verb.
	commandRegenerate
	// commandInit is the `mint init` scaffolding verb — a top-level verb that drops
	// the `.mint.toml` template and `release` shim into a project.
	commandInit
	// commandVersion is the `mint version` verb — a top-level verb that prints
	// mint's OWN tool version. It drives no gate, calls no RunFinished, and needs no
	// git repo (it never resolves the repo root).
	commandVersion
	// commandCommit is the `mint commit` verb — a top-level sibling of `release`
	// that mints a conventional-commit message from the staged diff and creates the
	// commit. It does NOT ride the release lifecycle spine.
	commandCommit
	// commandSetup is the `mint setup` verb — a top-level verb that emits the AI
	// setup guide to stdout. It drives no gate, calls no RunFinished, and — unlike
	// init/release/commit — needs NO git repo: it NEVER resolves the repo root and
	// issues no `git rev-parse`, so it runs UNCONDITIONALLY from any directory
	// (setup instructions are commonly read before `cd`-ing into the target repo).
	// The cwd-confirm safety net lives in the emitted guide, not a cmd-layer guard.
	commandSetup
)

// classifyCommand resolves the route for an invocation's args and returns the
// route plus the remaining args for that route's parser. `init` is a top-level
// verb; `release` with no subcommand is the cut action; `release regenerate`
// routes to the regenerate subcommand (so regenerate is reachable ONLY under
// release, never top-level); anything else is commandUnknown. It is pure — no
// execution, no parsing.
func classifyCommand(args []string) (commandKind, []string) {
	if len(args) == 0 {
		return commandUnknown, nil
	}
	if args[0] == "init" {
		return commandInit, args[1:]
	}
	if args[0] == "version" {
		return commandVersion, args[1:]
	}
	if args[0] == "commit" {
		return commandCommit, args[1:]
	}
	if args[0] == "setup" {
		return commandSetup, args[1:]
	}
	if args[0] != "release" {
		return commandUnknown, nil
	}
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "regenerate" {
		return commandRegenerate, rest[1:]
	}
	return commandRelease, rest
}

// newNoteCache builds the user-level note cache, or nil to disable caching. The cache
// lives under os.UserCacheDir() (the XDG-correct home for a cache: ~/.cache/mint on
// Linux, ~/Library/Caches/mint on macOS) so it never pollutes a project with an in-repo
// directory; the Store namespaces per project by repo path. Returning a true nil
// interface (not a typed nil) when the dir can't be resolved keeps the engine's
// `NoteCache == nil` guard correct — caching just turns off, which is fine since it is
// only an optimization.
func newNoteCache() engine.NoteCache {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return notescache.NewStore(filepath.Join(dir, "mint", "cache"), time.Now)
}

// homeDir is the user's home directory a run's header shortens the repo root under,
// or empty, shortening nothing, when it cannot be resolved.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// exitCode resolves the process exit code for a non-nil Release error: an engine
// *AbortError carries its own non-zero ExitCode; any other (unexpected) error
// maps to the generic exit 1.
func exitCode(err error) int {
	var abort *engine.AbortError
	if errors.As(err, &abort) {
		return abort.ExitCode
	}
	return 1
}
