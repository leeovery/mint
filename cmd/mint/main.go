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
	"path/filepath"
	"strings"

	"time"

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
	// `regenerate` is a subcommand of `release` (`mint release regenerate …`), not a
	// top-level verb; classifyCommand resolves the route. The init/version verbs are
	// reserved for later phases; an unknown command is a usage error.
	kind, rest := classifyCommand(args)
	switch kind {
	case commandRelease:
		return runRelease(rest)
	case commandRegenerate:
		return runRegenerate(rest)
	default:
		fmt.Fprintln(os.Stderr, "mint: unknown command (only `mint release` and `mint release regenerate` are wired)")
		return usageExitCode
	}
}

// runRegenerate parses and validates the `mint release regenerate` flag surface.
// After the structural parse it loads config (to read the changelog toggle) and
// runs the semantic source × target axis-contract validation: --reuse is
// release-only and implies --target release, a changelog/both target is rejected
// when the changelog is disabled, and a fresh -y run needs an explicit --target.
// It performs NO mutation or network call beyond reading the repo root and
// config — the heal/backfill execution is wired in a later phase, so for now a
// successful parse + validation reports that the command is not yet executable
// rather than silently doing nothing.
func runRegenerate(rest []string) int {
	req, err := parseRegenerateFlags(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// The changelog-disabled axis check needs the loaded config; resolve the repo
	// root and load it the same way the forward release pipeline does (read-only —
	// no mutation, no network).
	r := runner.NewExecRunner()
	root, err := gitrepo.ResolveRoot(context.Background(), r)
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

	// Preflight subset (task 5-4): run only the gates the resolved target needs —
	// gh-auth when a provider write occurs, clean-tree + on-branch + remote-sync
	// when the changelog is committed + pushed; never tag-free (regenerate cuts no
	// tag) and no version compute. A failing applicable gate aborts cleanly BEFORE
	// any work. The presenter surfaces the abort; resolve the release branch for the
	// on-branch / remote-sync gates the same read-only way the forward path does.
	p := presenter.NewForStartup(false, validated.Yes, os.Stdout, os.Stderr, os.Stdin)
	releaseBranch, err := gitrepo.ResolveReleaseBranch(context.Background(), r, cfg)
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
	if err := engine.RegeneratePreflight(context.Background(), deps, releaseBranch, regenerateGateSet(validated.Target)); err != nil {
		return exitCode(err)
	}

	// The batch --all backfill is a separate task; only the single-version interactive
	// run is wired here.
	if validated.All {
		fmt.Fprintln(os.Stderr, "mint: `release regenerate --all` is not yet implemented")
		return usageExitCode
	}

	return runRegenerateSingle(deps, r, cfg, root, validated)
}

// runRegenerateSingle executes one single-version interactive regenerate run: it
// resolves the <version> argument to its canonical tag + fresh diff base (task 5-3),
// resolves the publishing driver, then runs the interactive default flow (task 5-10) —
// asking source/target as needed, showing the plan, and confirming before the write.
func runRegenerateSingle(deps engine.ReleaseDeps, r runner.CommandRunner, cfg config.Config, root string, req regenerateRequest) int {
	ctx := context.Background()

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

	// A non-github / no-remote host downgrades to an unresolved publisher; the engine's
	// release-write surfaces that, so pass the resolver result through (nil publisher on
	// an unresolved provider).
	publisher, _ := publish.ResolvePublisher(regenerateRemoteURL(ctx, r), cfg.Release.Provider, r)

	source, target := regenerateRunAxes(req)
	runReq := engine.RegenerateRunRequest{
		Source:           source,
		Target:           target,
		Tag:              res.Tag,
		VersionKey:       versionKey.String(""),
		Project:          filepath.Base(root),
		ChangelogEnabled: cfg.Release.Changelog,
		Yes:              req.Yes,
		ProduceBody:      newRegenerateBodyProducer(r, cfg, root, res),
	}

	if err := engine.RegenerateRun(ctx, deps, publisher, root, runReq); err != nil {
		return exitCode(err)
	}
	return 0
}

// regenerateRemoteURL reads the release remote's URL via `git remote get-url origin`
// through the runner seam, returning "" on any failure (no `origin` remote) — the
// publisher resolver treats an empty URL as an unresolved provider, downgrading the
// run rather than failing it, the same way the forward path does.
func regenerateRemoteURL(ctx context.Context, r runner.CommandRunner) string {
	res, err := r.Run(ctx, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

// runRelease wires the production seams and runs the forward release pipeline for
// a parsed `mint release` invocation, returning the process exit code.
func runRelease(rest []string) int {
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

	if err := engine.Release(context.Background(), deps, opts.ReleaseOptions()); err != nil {
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
)

// classifyCommand resolves the route for an invocation's args and returns the
// route plus the remaining args for that route's parser. `release` with no
// subcommand is the cut action; `release regenerate` routes to the regenerate
// subcommand (so regenerate is reachable ONLY under release, never top-level);
// anything else is commandUnknown. It is pure — no execution, no parsing.
func classifyCommand(args []string) (commandKind, []string) {
	if len(args) == 0 || args[0] != "release" {
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
