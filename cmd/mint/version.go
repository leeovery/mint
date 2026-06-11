package main

import (
	"os"

	"mint/internal/buildinfo"
	"mint/internal/presenter"
)

// versionFlag is the standard global spelling of the version surface, handled
// before command classification so `mint --version` works independently of any
// subcommand.
const versionFlag = "--version"

// hasVersionFlag reports whether the global --version flag appears anywhere in the
// args. It is the standard "print version and exit" convention, matched as a whole
// token so the `version` VERB (no leading dashes) is NOT mistaken for the flag.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == versionFlag {
			return true
		}
	}
	return false
}

// runVersion wires the production presenter and prints mint's own tool version,
// returning the process exit code (always 0 — printing the version cannot fail).
// It deliberately constructs NO runner and resolves NO repo root: the version is
// the build identity of the binary, not anything derived from git, so it works
// outside a git repository. Both the `version` verb and the `--version` flag call
// this, so their output is byte-identical.
//
// The presenter is constructed non-interactively (yes=true): version has no Prompt
// to skip, but the flag keeps the construction on the non-interactive axis since
// version never reads stdin. The first arg (false) is plainFlag, not a force-plain —
// TTY detection still governs pretty vs plain, so `mint --version` on a terminal still
// renders the dressed form.
func runVersion(stdout, stderr, stdin *os.File) int {
	p := presenter.NewForStartup(false, true, stdout, stderr, stdin)
	emitVersion(p)
	return 0
}

// emitVersion drives the SINGLE presenter call for the version surface:
// ShowVersion carrying mint's own tool version from the single buildinfo source.
// It is the shared core both version entry points route through, so output can
// never diverge. The Leaf is left empty — the pretty presenter defaults it to the
// brand 🌿, so version needs no git/repo to derive a leaf. version drives no gate
// (no Prompt) and no footer (no RunFinished): the value line is the terminal
// output.
func emitVersion(p presenter.Presenter) {
	p.ShowVersion(presenter.Version{Value: buildinfo.Version})
}
