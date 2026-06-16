package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"mint/internal/setupguide"
)

// parseSetupFlags parses the `mint setup` arguments. setup takes NO flags beyond the
// implicit -h/--help: the flag set is registered (with its default usage dump
// discarded) solely so a help request surfaces as flag.ErrHelp — which the cmd layer
// routes to the curated setupUsage — and any other flag is a usage error the parser
// surfaces, so a typo fails loudly. setup takes no positional arguments either.
func parseSetupFlags(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump
	return fs.Parse(args)
}

// runSetup emits the embedded AI setup guide to stdout and returns the process exit
// code. It mirrors runVersion's IO-descriptor / no-ctx shape: setup spawns nothing to
// interrupt and resolves NO repo. It deliberately diverges from runInit — it performs
// NO gitrepo.ResolveRoot, no `git rev-parse`, no config load, no presenter/engine
// wiring — so it runs UNCONDITIONALLY from any directory (setup instructions are
// commonly read before `cd`-ing into the target repo); the cwd-confirm safety net
// lives in the emitted guide. The only IO is writing the guide to stdout (or the parse
// diagnostic to stderr).
//
// The guide is written via a cmd-layer fmt.Fprint to stdout — the documented seam-3
// exception for curated help/usage text — rather than through a presenter, so a pure
// text emit pulls in no presenter/engine dependency.
func runSetup(args []string, stdout, stderr io.Writer) int {
	if err := parseSetupFlags(args); err != nil {
		// -h/--help is a requested action, not a usage error: stdout, exit 0.
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(stdout, setupUsage)
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "mint: %v\n", err)
		return usageExitCode
	}

	_, _ = fmt.Fprint(stdout, setupguide.Guide())
	return 0
}
