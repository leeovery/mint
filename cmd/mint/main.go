// Command mint is the entry point for the mint release tool. main stays thin: it
// parses the CLI surface, constructs the production seams ONCE (the presenter, the
// os/exec runner, the GitHub publisher, the releaser), runs the engine
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

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/publish"
	"mint/internal/release"
	"mint/internal/runner"
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
	// Phase 1 wires only `mint release`. The subcommand surface (regenerate, init,
	// version) is reserved for later phases; an unknown command is a usage error.
	cmd, rest := splitCommand(args)
	if cmd != "release" {
		fmt.Fprintf(os.Stderr, "mint: unknown command %q (only `mint release` is wired)\n", cmd)
		return usageExitCode
	}

	opts, err := parseReleaseFlags(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	// Construct the presenter ONCE at startup — the single production construction
	// site. TTY/mode detection never happens downstream.
	p := presenter.NewForStartup(opts.Plain, opts.Yes, os.Stdout, os.Stderr, os.Stdin)

	// One runner backs every external-command seam.
	r := runner.NewExecRunner()
	deps := engine.ReleaseDeps{
		Presenter: p,
		Runner:    r,
		Releaser:  release.NewReleaser(r),
		Publisher: publish.NewGitHubPublisher(r),
		// The `e` review-gate choice hands the notes to the real $EDITOR resolution,
		// launched interactively through the same presenter + runner. The launcher
		// reports a missing editor and returns to the gate rather than aborting.
		Editor: engine.NewEditorLauncher(p, r),
	}

	if err := engine.Release(context.Background(), deps, opts.ReleaseOptions()); err != nil {
		return exitCode(err)
	}
	return 0
}

// splitCommand separates the leading subcommand from its arguments. With no args
// the command is empty (the unknown-command branch reports it as a usage error).
func splitCommand(args []string) (cmd string, rest []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
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
