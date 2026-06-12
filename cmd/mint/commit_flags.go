package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"mint/internal/commit"
)

// commitFlags is the parsed `mint commit` CLI surface. Phase 1 wired the presentation
// flags the presenter consumes at startup: --plain forces un-styled output, and
// -y/--yes auto-accepts. Phase 2 adds the staging selectors -a/--all and -A/--add-all,
// resolved into a single commit.StagingMode. Phase 3 adds --no-ai, which skips AI
// generation and the Continue? gate and drops to the $EDITOR fallback. Phase 5 adds
// -p/--push, which arms a push after a successful commit. All are plain values so flag
// parsing stays decoupled from the orchestrator.
type commitFlags struct {
	// Yes auto-accepts the review gate; the presenter performs the skip inside Prompt.
	Yes bool
	// Plain forces the plain (un-styled) presenter regardless of TTY.
	Plain bool
	// Staging is the resolved staging mode: StagedOnly (default, neither -a nor -A —
	// the Phase 1 behaviour), All (-a/--all = git commit -a), or AddAll (-A/--add-all
	// = git add -A then commit). -a and -A are mutually exclusive (parseCommitFlags
	// rejects combining them with the spec's exact message).
	Staging commit.StagingMode
	// NoAI selects --no-ai: skip AI generation AND the Continue? gate and drop to the
	// $EDITOR fallback, where the editor save IS the accept event. False is the normal
	// AI-generate path.
	NoAI bool
	// Push selects -p/--push: arm a push after a successful commit. Push is FLAG-ONLY —
	// "we never push without the -p flag" — so there is deliberately NO push config
	// default; this flag is the sole source of the armed value. Default false = disarmed
	// (no push). The push itself fires only after a successful commit (gating is Phase 5's
	// later tasks); this flag merely carries the armed value.
	Push bool
}

// parseCommitFlags parses the `mint commit [options]` arguments into a commitFlags.
// -y/--yes, --plain, and the two staging selectors are independent booleans; an
// unrecognised flag is a parse error the cmd layer surfaces as a usage error.
//
// Bundled single-letter flags (e.g. -aA, -ay) are PRE-EXPANDED into separate tokens
// (-a -A, -a -y) BEFORE fs.Parse, because stdlib flag.NewFlagSet has no POSIX
// short-flag bundling — without the expansion `-aA` would be one unknown flag (a
// generic usage error) and the -a/-A conflict would never reach resolveStagingMode.
// Long flags (`--all`, `--add-all`, `--no-ai`, `--plain`, …) start with `--` and pass
// through the expansion UNTOUCHED.
func parseCommitFlags(args []string) (commitFlags, error) {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var yes, plain, all, addAll, noAI, push bool
	fs.BoolVar(&yes, "y", false, "skip the review gate (auto-accept)")
	fs.BoolVar(&yes, "yes", false, "skip the review gate (auto-accept)")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")
	fs.BoolVar(&all, "a", false, "stage tracked changes first (git commit -a semantics)")
	fs.BoolVar(&all, "all", false, "stage tracked changes first (git commit -a semantics)")
	fs.BoolVar(&addAll, "A", false, "stage everything incl. untracked first (git add -A)")
	fs.BoolVar(&addAll, "add-all", false, "stage everything incl. untracked first (git add -A)")
	fs.BoolVar(&noAI, "no-ai", false, "skip AI generation; write the message in $EDITOR")
	// -p/--push set the same push var (the paired short/long pattern -a/--all and -y/--yes
	// use). Registering -p as a defined single-letter flag also lets the short-flag
	// pre-expansion fold it into bundles, so -Ap → -A -p and -Apy → -A -p -y. Push is
	// flag-only — no config default is read or defaulted; this flag is the sole source.
	fs.BoolVar(&push, "p", false, "push after committing (no push without this; no config default)")
	fs.BoolVar(&push, "push", false, "push after committing (no push without this; no config default)")

	if err := fs.Parse(expandShortFlagBundles(args, fs)); err != nil {
		return commitFlags{}, err
	}

	staging, err := resolveStagingMode(all, addAll)
	if err != nil {
		return commitFlags{}, err
	}
	return commitFlags{Yes: yes, Plain: plain, Staging: staging, NoAI: noAI, Push: push}, nil
}

// resolveStagingMode maps the two mutually-exclusive staging booleans to a
// commit.StagingMode. -a and -A together is a conflicting-flags error with the spec's
// EXACT message (fail loud — never silently picking a winner, since -A already
// includes -a's changes). Neither set is StagedOnly (the Phase 1 default), mirroring
// resolveBump's idiom in flags.go.
func resolveStagingMode(all, addAll bool) (commit.StagingMode, error) {
	switch {
	case all && addAll:
		return commit.StagedOnly, fmt.Errorf("-a and -A cannot be combined; -A already includes -a's changes")
	case addAll:
		return commit.AddAll, nil
	case all:
		return commit.All, nil
	default:
		return commit.StagedOnly, nil
	}
}

// expandShortFlagBundles pre-expands POSIX-style bundled single-letter flags into
// separate tokens so stdlib flag (which has no bundling) sees one flag per token —
// e.g. `-aA` → `-a -A`, `-ay` → `-a -y`. A token is expanded ONLY when:
//   - it starts with a single `-` (NOT `--`), and
//   - it is at least two chars after the `-`, and
//   - EVERY char after the `-` is a DEFINED single-letter flag in fs.
//
// Long flags (`--all`, `--no-ai`, …) and anything that fails the all-defined check
// (e.g. `-xz`, a value, the lone `-` stdin marker) pass through UNCHANGED, left for
// fs.Parse to accept or reject as usual. Expansion stops at the `--` terminator: every
// token after it is a positional argument and passes through verbatim.
func expandShortFlagBundles(args []string, fs *flag.FlagSet) []string {
	expanded := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			// The `--` terminator and everything after it are positional — pass through
			// verbatim with no further expansion.
			expanded = append(expanded, args[i:]...)
			break
		}
		if bundle, ok := shortFlagBundle(arg, fs); ok {
			expanded = append(expanded, bundle...)
			continue
		}
		expanded = append(expanded, arg)
	}
	return expanded
}

// shortFlagBundle reports whether arg is a bundle of two-or-more DEFINED single-letter
// flags (a single `-` followed by chars that are each a defined flag) and, if so,
// returns the unbundled `-x -y …` tokens. It returns ok=false for long flags (`--…`),
// the lone `-`, single short flags (already unbundled), and any token containing a
// char that is not a defined single-letter flag — those pass through unexpanded.
func shortFlagBundle(arg string, fs *flag.FlagSet) ([]string, bool) {
	if len(arg) < 3 || !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return nil, false
	}
	chars := arg[1:]
	for _, c := range chars {
		if fs.Lookup(string(c)) == nil {
			return nil, false
		}
	}
	tokens := make([]string, 0, len(chars))
	for _, c := range chars {
		tokens = append(tokens, "-"+string(c))
	}
	return tokens, true
}
