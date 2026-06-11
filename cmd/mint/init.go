package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/runner"
)

// initFlags is the parsed `mint init` CLI surface: the non-clobber/--force toggle
// and the global --plain presentation flag. It is a plain value (no engine types)
// so flag parsing stays decoupled from the orchestrator; InitOptions converts it
// into the engine's option struct.
type initFlags struct {
	// Force regenerates (overwrites) an existing .mint.toml / release shim instead of
	// skipping it. Default false = idempotent / non-clobbering.
	Force bool
	// Plain forces the plain (un-styled) presenter regardless of TTY — the global
	// presentation flag that applies to every verb.
	Plain bool
}

// InitOptions converts the parsed flags into the engine's init run options.
func (f initFlags) InitOptions() engine.InitOptions {
	return engine.InitOptions{Force: f.Force}
}

// parseInitFlags parses the `mint init [--force] [--plain]` arguments into an
// initFlags. init takes no positional arguments; an unknown flag is a usage error
// (the parser surfaces it) so a typo fails loudly rather than being ignored.
func parseInitFlags(args []string) (initFlags, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var force, plain bool
	fs.BoolVar(&force, "force", false, "regenerate (overwrite) existing files")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")

	if err := fs.Parse(args); err != nil {
		return initFlags{}, err
	}
	return initFlags{Force: force, Plain: plain}, nil
}

// runInit parses the `mint init` flag surface, wires the production seams (the
// presenter and the os/exec runner), and runs the engine init orchestrator —
// dropping the .mint.toml template and release shim at the git-resolved repo root.
// It returns 0 on success, the engine abort's exit code on a resolution/write
// failure, and usageExitCode on a CLI parse error.
//
// init has no interactive gate, so the presenter is constructed non-interactively
// (yes=true): there is no Prompt to skip, but the flag keeps the construction on the
// non-interactive axis since init never reads stdin.
func runInit(rest []string) int {
	opts, err := parseInitFlags(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint: %v\n", err)
		return usageExitCode
	}

	p := presenter.NewForStartup(opts.Plain, true, os.Stdout, os.Stderr, os.Stdin)
	r := runner.NewExecRunner()
	deps := engine.InitDeps{Presenter: p, Runner: r}

	if err := engine.Init(context.Background(), deps, opts.InitOptions()); err != nil {
		return exitCode(err)
	}
	return 0
}
