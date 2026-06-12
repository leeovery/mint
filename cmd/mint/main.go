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
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"time"

	"mint/internal/commit"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/gitrepo"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/release"
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

	// Build the ONE signal-cancellable context for the whole run: a Ctrl-C
	// (os.Interrupt) or SIGTERM cancels it, which threads down through every command's
	// external-command seam so a long pre-PONR step (the AI call, a hook, the gap before
	// the atomic push) is interrupted. The engine treats a pre-PONR cancellation as a
	// failure routed through its surgical unwind (resets, tag delete, autostash pop), so
	// the repo is left clean; a post-PONR cancellation keeps the existing warn-only
	// contract. stop() restores the default signal handling on return.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	default:
		fmt.Fprintln(os.Stderr, "mint: unknown command (only `mint release`, `mint release regenerate`, `mint init`, `mint version`, and `mint commit` are wired)")
		return usageExitCode
	}
}

// runRegenerate parses and validates the `mint release regenerate` flag surface,
// runs the applicable preflight subset, then dispatches to the single-version
// interactive run or the --all batch backfill. After the structural parse it loads
// config (to read the changelog toggle) and runs the semantic source × target
// axis-contract validation: --reuse is release-only and implies --target release, a
// changelog/both target is rejected when the changelog is disabled, and a fresh -y
// run needs an explicit --target. The only mutation/network beyond reading the repo
// root + config happens inside the dispatched run.
func runRegenerate(ctx context.Context, rest []string) int {
	req, err := parseRegenerateFlags(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// The changelog-disabled axis check needs the loaded config; resolve the repo
	// root and load it the same way the forward release pipeline does (read-only —
	// no mutation, no network).
	r := runner.NewExecRunner()
	root, err := gitrepo.ResolveRoot(ctx, r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	validated, err := validateRegenerateRequest(req, cfg.Release.Changelog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Resolve the release branch (read-only, the same way the forward path does) for the
	// preflight on-branch / remote-sync gates. Both dispatch paths consume it: the batch
	// path preflights here up front (its axes are resolved inside runRegenerateAll); the
	// single-version path threads it into RegenerateRun, which preflights the RESOLVED
	// target AFTER the interactive source/target prompts — the only point at which a bare
	// `regenerate <ver>` (no --target) knows which surface(s) it writes.
	p := presenter.NewForStartup(validated.Plain, validated.Yes, os.Stdout, os.Stderr, os.Stdin)
	releaseBranch, err := gitrepo.ResolveReleaseBranch(ctx, r, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}
	deps := engine.ReleaseDeps{
		Presenter: p,
		Runner:    r,
		Mutator:   git.NewMutator(r),
		// The `e` review-gate choice on the fresh notes-review gate hands the notes to
		// the real $EDITOR resolution, launched through the same presenter + runner.
		Editor: engine.NewEditorLauncher(p, r),
	}

	// The --all batch backfill (every version, oldest → newest) and the single-version
	// interactive run share the resolved deps/runner/config/root; both thread the
	// resolved release branch so the engine can preflight the RESOLVED target subset
	// AFTER the interactive axis prompts (a bare invocation does not know its surface(s)
	// until the source/target prompts resolve — the cmd layer cannot run that gate set).
	if validated.All {
		return runRegenerateAll(ctx, deps, r, cfg, root, releaseBranch, validated)
	}

	return runRegenerateSingle(ctx, deps, r, cfg, root, releaseBranch, validated)
}

// runRegenerateSingle executes one single-version interactive regenerate run: it
// resolves the <version> argument to its canonical tag + fresh diff base (task 5-3),
// resolves the publishing driver, then runs the interactive default flow (task 5-10) —
// asking source/target as needed, preflighting the RESOLVED target's gate subset,
// showing the plan, and confirming before the write. releaseBranch threads to the
// engine for the preflight on-branch / remote-sync gates, which run AFTER the
// interactive target resolves (a bare `regenerate <ver>` does not know its surface(s)
// until then).
func runRegenerateSingle(ctx context.Context, deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root, releaseBranch string, req regenerateRequest) int {
	res, err := version.ResolveRegenerateTarget(ctx, r, cfg.Release.TagPrefix, req.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// The bare x.y.z key (no tag prefix) used in the changelog header and the
	// notes/plan, recovered by re-parsing the canonical tag.
	versionKey, err := version.ParseSemVer(res.Tag, cfg.Release.TagPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Resolve the publishing driver through the SHARED engine helper, which handles an
	// unresolvable provider exactly as the forward engine.Release path does — warn-and-
	// downgrade (a nil publisher proceeds, the provider write is skipped downstream) or a
	// clean abort. This REPLACES the former `publisher, _ := …` discard that passed a nil
	// interface down and crashed DispatchRelease on a non-github / no-remote origin.
	publisher, code, proceed := resolveRegeneratePublisher(ctx, deps, cfg)
	if !proceed {
		return code
	}

	source, target := regenerateRunAxes(req)
	runReq := engine.RegenerateRunRequest{
		Source:           source,
		Target:           target,
		Tag:              res.Tag,
		VersionKey:       versionKey.String(""),
		Project:          filepath.Base(root),
		ReleaseBranch:    releaseBranch,
		ChangelogEnabled: cfg.Release.Changelog,
		Yes:              req.Yes,
		ProduceBody:      newRegenerateBodyProducer(r, cfg, root, res),
		// The fresh notes-review gate's `r` choice consults this per-run regenerator,
		// bound to the resolved range — the regenerate analogue of the forward path's
		// per-run regenerator. Without it the rendered `r` would abort.
		ProduceRegenerator: newRegenerateRegeneratorProducer(r, cfg, root, res),
	}

	if err := engine.RegenerateRun(ctx, deps, publisher, root, runReq); err != nil {
		return exitCode(err)
	}
	return 0
}

// resolveRegeneratePublisher resolves the publishing driver for a regenerate run via the
// SHARED engine.ResolvePublisher, which mirrors the forward engine.Release Stage-6
// handling: a RESOLVED provider yields the driver; an UNRESOLVED provider (a non-github /
// no-remote origin) WARNS and downgrades to a nil publisher; any other resolution error
// is a surfaced abort. It collapses that into the cmd layer's three needs: the publisher
// to thread (nil on a downgrade — the engine write nil-guards it), the exit code, and
// whether to proceed. It returns proceed=true with a nil publisher on a clean downgrade,
// and proceed=false with the abort's exit code on a real failure. Both regenerate dispatch
// paths (single-version and --all) share it so neither can drift back to discarding the
// resolver error.
func resolveRegeneratePublisher(ctx context.Context, deps engine.ReleaseDeps, cfg config.Config) (publish.Publisher, int, bool) {
	publisher, err := engine.ResolvePublisher(ctx, deps, cfg)
	if err != nil {
		return nil, exitCode(err), false
	}
	return publisher, 0, true
}

// runRelease wires the production seams and runs the forward release pipeline for
// a parsed `mint release` invocation, returning the process exit code.
func runRelease(ctx context.Context, rest []string) int {
	opts, err := parseReleaseFlags(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Construct the presenter ONCE at startup — the single production construction
	// site. TTY/mode detection never happens downstream.
	p := presenter.NewForStartup(opts.Plain, opts.Yes, os.Stdout, os.Stderr, os.Stdin)

	// One runner backs every external-command seam. The lock-resilient git Mutator is
	// constructed ONCE over that runner and shared by the engine's mutation calls and
	// the Releaser, so every git mutation (commit, tag, push, reset, tag-delete) retries
	// a contended .git lock and clears a provably-stale one. Read-only git stays on r.
	r := runner.NewExecRunner()
	mut := git.NewMutator(r)
	deps := engine.ReleaseDeps{
		Presenter: p,
		Runner:    r,
		Mutator:   mut,
		Releaser:  release.NewReleaser(mut),
		// The `e` review-gate choice hands the notes to the real $EDITOR resolution,
		// launched interactively through the same presenter + runner. The launcher
		// reports a missing editor and returns to the gate rather than aborting.
		Editor: engine.NewEditorLauncher(p, r),
		// The dry-run note cache lives UNDER the repo at {root}/.mint/cache (gitignored,
		// never committed), repo-scoped and stamped with the wall clock for the ~1h TTL.
		// A --dry-run writes the generated note here; the subsequent real run recomputes
		// the key, looks it up, and on a live (within-TTL) match reuses the previewed
		// bytes — skipping the AI. A miss or an expired entry regenerates.
		NoteCache: notescache.NewRepoStore(time.Now),
	}

	if err := engine.Release(ctx, deps, opts.ReleaseOptions()); err != nil {
		return exitCode(err)
	}
	return 0
}

// runCommit wires the production seams and runs the bare `mint commit` generate-and-
// commit thread for a parsed `mint commit` invocation, returning the process exit
// code. It mirrors runRelease's seam-wiring idiom: construct the presenter ONCE via
// NewForStartup, one ExecRunner backing the read-side staged-diff seam, and the
// lock-resilient git Mutator (git_safe) as the commit sink over that same runner. All
// orchestration lives in the testable commit.Run; main stays thin.
func runCommit(ctx context.Context, rest []string) int {
	opts, err := parseCommitFlags(rest)
	if err != nil {
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
	// site. TTY/mode detection never happens downstream.
	p := presenter.NewForStartup(opts.Plain, opts.Yes, os.Stdout, os.Stderr, os.Stdin)

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
		Push: opts.Push,
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
	if args[0] != "release" {
		return commandUnknown, nil
	}
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "regenerate" {
		return commandRegenerate, rest[1:]
	}
	return commandRelease, rest
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
