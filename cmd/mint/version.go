package main

import (
	"os"
	"slices"

	"mint/internal/buildinfo"
	"mint/internal/presenter"
)

// versionFlag is the standard global spelling of the version surface, handled
// before command classification so `mint --version` works independently of any
// subcommand.
const versionFlag = "--version"

// plainArg is --plain, which the version surface honours wherever it appears, as the
// global --version is found.
const plainArg = "--plain"

// hasVersionFlag reports whether the global --version flag appears anywhere in the
// args. It is the standard "print version and exit" convention, matched as a whole
// token so the `version` VERB (no leading dashes) is NOT mistaken for the flag.
func hasVersionFlag(args []string) bool { return slices.Contains(args, versionFlag) }

// hasPlainFlag reports whether --plain appears anywhere in the args, matched as a
// whole token as --version is.
func hasPlainFlag(args []string) bool { return slices.Contains(args, plainArg) }

// runVersion wires the production presenter and prints mint's own tool version,
// returning the process exit code (always 0 — printing the version cannot fail).
// It deliberately constructs NO runner and resolves NO repo root: the version is
// the build identity of the binary, not anything derived from git, so it works
// outside a git repository. Both the `version` verb and the `--version` flag call
// this, so their output is byte-identical.
//
// The presenter is constructed non-interactively (yes=true): version has no Prompt
// to skip, but the flag keeps the construction on the non-interactive axis since
// version never reads stdin. plain is --plain: without it TTY detection governs
// pretty vs plain, so `mint --version` on a terminal renders the dressed form, drawn
// as the Rail draws it, and with it the bare value.
func runVersion(plain bool, stdout, stderr, stdin *os.File) int {
	p := presenter.NewForStartup(plain, true, stdout, stderr, stdin)
	defer presenter.Close(p)
	emitVersion(p)
	return 0
}

// emitVersion drives the SINGLE presenter call for the version surface:
// ShowVersion carrying mint's own tool version from the single buildinfo source.
// It is the shared core both version entry points route through, so output can
// never diverge. version drives no gate (no Prompt) and no footer (no
// RunFinished): the value line is the terminal output.
func emitVersion(p presenter.Presenter) {
	p.ShowVersion(presenter.Version{Value: buildinfo.Version})
}
