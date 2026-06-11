package main

import (
	"flag"
	"fmt"
	"io"
)

// regenerateSource selects where regenerate sources its notes. The zero value is
// sourceFresh so a run with neither --reuse nor --fresh defaults to the
// re-diff + AI path.
type regenerateSource int

const (
	// sourceFresh re-diffs vX-1..vX and re-runs the AI for genuinely new notes.
	// It is the zero value and the default when no source flag is given.
	sourceFresh regenerateSource = iota
	// sourceReuse reads the tag annotation body verbatim — deterministic,
	// parse-free, config-independent. Selected by --reuse.
	sourceReuse
)

// regenerateTarget selects which surface(s) regenerate writes. The zero value is
// targetUnset: --target is optional at parse time (it is asked interactively, or
// in task 5-2 enforced for the fresh -y path), so "no --target given" is its own
// distinct state rather than a defaulted surface.
type regenerateTarget int

const (
	// targetUnset means no --target flag was supplied. It is the zero value so an
	// omitted flag is distinguishable from an explicit choice.
	targetUnset regenerateTarget = iota
	// targetRelease writes the provider release body only.
	targetRelease
	// targetChangelog writes CHANGELOG.md only.
	targetChangelog
	// targetBoth writes both the provider release and CHANGELOG.md.
	targetBoth
)

// regenerateRequest is the parsed `mint release regenerate` CLI surface: the
// optional <version> positional, the two-axis source/target selection, and the
// --all / -y booleans. It is a plain value carrying only the parsed surface — the
// SEMANTIC axis-contract validation (reuse⇒release-only, changelog-disabled,
// fresh -y needs target) and execution live in later tasks.
type regenerateRequest struct {
	// Version is the positional <version> argument (with or without tag_prefix).
	// Empty when --all is used; the engine owns prefix normalisation and the
	// tag-exists check.
	Version string
	// Source is the resolved notes source (default sourceFresh). --reuse and
	// --fresh are mutually exclusive.
	Source regenerateSource
	// SourceSet reports whether a source flag (--reuse or --fresh) was SUPPLIED, as
	// distinct from the defaulted sourceFresh. The interactive default flow (task
	// 5-10) skips the source question only when a flag was supplied, so "no flag" must
	// be distinguishable from an explicit --fresh — which Source alone cannot express
	// (both resolve to sourceFresh).
	SourceSet bool
	// Target is the resolved write surface, or targetUnset when --target is
	// omitted.
	Target regenerateTarget
	// All is the --all batch flag: regenerate every version, oldest → newest.
	All bool
	// Yes skips the confirmation / per-version review gate (-y/--yes).
	Yes bool
	// Plain forces the plain (un-styled) presenter regardless of TTY. It is the
	// global --plain render flag — identical name, default, and meaning as the
	// forward `mint release` route — so it composes with every regenerate flag.
	Plain bool
}

// parseRegenerateFlags parses the `mint release regenerate [<version>] [flags]`
// arguments into a regenerateRequest. It enforces three structural rules — the
// --reuse/--fresh mutual exclusivity, presence rule A (neither <version> nor
// --all), and presence rule B (both <version> and --all) — and rejects an
// unrecognised --target value. It performs NO mutation, network call, or
// semantic axis-contract validation (that is task 5-2).
func parseRegenerateFlags(args []string) (regenerateRequest, error) {
	fs := flag.NewFlagSet("regenerate", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var reuse, fresh, all, yes, plain bool
	var target string
	fs.BoolVar(&reuse, "reuse", false, "source = tag annotation body (no AI); implies --target release")
	fs.BoolVar(&fresh, "fresh", false, "source = re-diff + AI (default)")
	fs.StringVar(&target, "target", "", "surface(s) to write: release, changelog, or both")
	fs.BoolVar(&all, "all", false, "regenerate every version, oldest → newest")
	fs.BoolVar(&yes, "y", false, "skip the confirmation / per-version review gate")
	fs.BoolVar(&yes, "yes", false, "skip the confirmation / per-version review gate")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")

	// The flag package stops at the first non-flag token, so a <version> positional
	// before any flag would shadow them. Lift the lone positional out first, then
	// parse the flag-only remainder, so `<version> --reuse` and `--reuse <version>`
	// are equivalent.
	version, flagArgs, err := splitRegeneratePositional(args)
	if err != nil {
		return regenerateRequest{}, err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return regenerateRequest{}, err
	}

	source, err := resolveRegenerateSource(reuse, fresh)
	if err != nil {
		return regenerateRequest{}, err
	}

	parsedTarget, err := resolveRegenerateTarget(target)
	if err != nil {
		return regenerateRequest{}, err
	}

	if err := checkVersionPresence(version, all); err != nil {
		return regenerateRequest{}, err
	}

	return regenerateRequest{
		Version:   version,
		Source:    source,
		SourceSet: reuse || fresh,
		Target:    parsedTarget,
		All:       all,
		Yes:       yes,
		Plain:     plain,
	}, nil
}

// splitRegeneratePositional lifts the single optional <version> positional out of
// args, returning it plus the flag-only remainder for flag.Parse. It is needed
// because Go's flag package stops at the first non-flag token, so a positional
// before a flag (`<version> --reuse`) would otherwise swallow the flags.
//
// It walks the tokens, passing flag tokens (and the VALUE of the one value-taking
// flag, --target, when written as a separate token) straight through, and treats
// any bare non-flag token as the positional. More than one bare positional is a
// usage error (regenerate takes at most one <version>).
func splitRegeneratePositional(args []string) (version string, flagArgs []string, err error) {
	found := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isFlagToken(arg) {
			flagArgs = append(flagArgs, arg)
			// --target / -target written as two tokens (`--target release`) carries its
			// value in the next token; pass it through so it is not mistaken for the
			// positional. The `--target=release` single-token form needs no special case.
			if isTargetFlag(arg) && i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		if found {
			return "", nil, fmt.Errorf("unexpected argument %s (regenerate takes at most one version)", arg)
		}
		version = arg
		found = true
	}
	return version, flagArgs, nil
}

// isFlagToken reports whether a token is a flag (begins with "-") rather than a
// bare positional. The lone "-" is treated as a positional, matching the flag
// package's own convention.
func isFlagToken(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isTargetFlag reports whether a flag token is the value-taking --target flag in
// its separate-token form (i.e. without an inline "=value"). Both the "--target"
// and "-target" spellings are recognised, mirroring the flag package's leniency.
func isTargetFlag(arg string) bool {
	return arg == "--target" || arg == "-target"
}

// resolveRegenerateSource maps the mutually-exclusive --reuse/--fresh booleans to
// a regenerateSource. Both set is a usage error; neither set defaults to
// sourceFresh (also the zero value).
func resolveRegenerateSource(reuse, fresh bool) (regenerateSource, error) {
	if reuse && fresh {
		return sourceFresh, fmt.Errorf("the source flags --reuse and --fresh are mutually exclusive; pass at most one")
	}
	if reuse {
		return sourceReuse, nil
	}
	return sourceFresh, nil
}

// resolveRegenerateTarget maps the --target value to a regenerateTarget. An empty
// value (flag omitted) is targetUnset; any value other than release/changelog/both
// is a usage error.
func resolveRegenerateTarget(value string) (regenerateTarget, error) {
	switch value {
	case "":
		return targetUnset, nil
	case "release":
		return targetRelease, nil
	case "changelog":
		return targetChangelog, nil
	case "both":
		return targetBoth, nil
	default:
		return targetUnset, fmt.Errorf("invalid --target value %s (expected release, changelog, or both)", value)
	}
}

// checkVersionPresence enforces the two scope-presence rules: neither a <version>
// nor --all is an error (rule A), and supplying both is an error (rule B).
func checkVersionPresence(version string, all bool) error {
	switch {
	case version == "" && !all:
		return fmt.Errorf("specify a version or --all")
	case version != "" && all:
		return fmt.Errorf("cannot combine a version with --all")
	default:
		return nil
	}
}
