package main

import (
	"flag"
	"fmt"
	"io"
)

// regenerateSource selects where regenerate sources its notes — the source axis,
// supplied via `--source <fresh|tag|release>` (symmetric with --target). The zero
// value is sourceFresh so a run with no --source value defaults to the re-diff + AI
// path.
type regenerateSource int

const (
	// sourceFresh re-diffs vX-1..vX and re-runs the AI for genuinely new notes.
	// It is the zero value and the default when no --source value is given.
	// Selected by --source fresh.
	sourceFresh regenerateSource = iota
	// sourceTag reads the tag annotation body verbatim — deterministic,
	// parse-free, config-independent. Selected by --source tag.
	sourceTag
	// sourceRelease reads the EXISTING provider release body verbatim — deterministic,
	// parse-free. Selected by --source release; requires a resolvable provider.
	sourceRelease
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
// SEMANTIC axis validation (orthogonal axes, changelog-disabled, -y needs target)
// lives in validateRegenerateRequest.
type regenerateRequest struct {
	// Version is the positional <version> argument (with or without tag_prefix).
	// Empty when --all is used; the engine owns prefix normalisation and the
	// tag-exists check.
	Version string
	// Source is the resolved notes source (default sourceFresh), parsed from the
	// single --source <fresh|tag|release> value flag.
	Source regenerateSource
	// SourceSet reports whether a --source value was SUPPLIED, as distinct from the
	// defaulted sourceFresh. The interactive default flow skips the source question only
	// when --source was supplied, so "no flag" must be distinguishable from an explicit
	// --source fresh — which Source alone cannot express (both resolve to sourceFresh).
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
// arguments into a regenerateRequest. It enforces the two scope-presence rules
// (rule A: neither <version> nor --all; rule B: both) and rejects an unrecognised
// --source or --target value. It performs NO mutation, network call, or semantic
// axis validation (that is validateRegenerateRequest).
func parseRegenerateFlags(args []string) (regenerateRequest, error) {
	fs := flag.NewFlagSet("regenerate", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var all, yes, plain bool
	var source, target string
	fs.StringVar(&source, "source", "", "notes source: fresh, tag, or release (default fresh)")
	fs.StringVar(&target, "target", "", "surface(s) to write: release, changelog, or both")
	fs.BoolVar(&all, "all", false, "regenerate every version, oldest → newest")
	fs.BoolVar(&yes, "y", false, "skip the confirmation / per-version review gate")
	fs.BoolVar(&yes, "yes", false, "skip the confirmation / per-version review gate")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")

	// The flag package stops at the first non-flag token, so a <version> positional
	// before any flag would shadow them. Lift the lone positional out first, then
	// parse the flag-only remainder, so `<version> --source tag` and
	// `--source tag <version>` are equivalent.
	version, flagArgs, err := splitRegeneratePositional(args)
	if err != nil {
		return regenerateRequest{}, err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return regenerateRequest{}, err
	}

	parsedSource, err := resolveRegenerateSource(source)
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
		Version: version,
		Source:  parsedSource,
		// An empty --source is the omitted state (ask interactively); any explicit
		// value — including "fresh" — marks the source supplied so the prompt is skipped.
		SourceSet: source != "",
		Target:    parsedTarget,
		All:       all,
		Yes:       yes,
		Plain:     plain,
	}, nil
}

// splitRegeneratePositional lifts the single optional <version> positional out of
// args, returning it plus the flag-only remainder for flag.Parse. It is needed
// because Go's flag package stops at the first non-flag token, so a positional
// before a flag (`<version> --source tag`) would otherwise swallow the flags.
//
// It walks the tokens, passing flag tokens (and the VALUE of a value-taking flag —
// --source or --target — when written as a separate token) straight through, and
// treats any bare non-flag token as the positional. More than one bare positional is
// a usage error (regenerate takes at most one <version>).
func splitRegeneratePositional(args []string) (version string, flagArgs []string, err error) {
	found := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isFlagToken(arg) {
			flagArgs = append(flagArgs, arg)
			// A value-taking flag written as two tokens (`--source tag`, `--target release`)
			// carries its value in the next token; pass it through so it is not mistaken for
			// the positional. The `--source=tag` single-token form needs no special case.
			if isValueFlag(arg) && i+1 < len(args) {
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

// isValueFlag reports whether a flag token is one of the value-taking flags
// (--source, --target) in its separate-token form (i.e. without an inline "=value").
// Both the "--flag" and "-flag" spellings are recognised, mirroring the flag
// package's leniency.
func isValueFlag(arg string) bool {
	switch arg {
	case "--source", "-source", "--target", "-target":
		return true
	default:
		return false
	}
}

// resolveRegenerateSource maps the --source value to a regenerateSource. An empty
// value (flag omitted) defaults to sourceFresh (also the zero value); any value other
// than fresh/tag/release is a usage error.
func resolveRegenerateSource(value string) (regenerateSource, error) {
	switch value {
	case "", "fresh":
		return sourceFresh, nil
	case "tag":
		return sourceTag, nil
	case "release":
		return sourceRelease, nil
	default:
		return sourceFresh, fmt.Errorf("invalid --source value %s (expected fresh, tag, or release)", value)
	}
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
