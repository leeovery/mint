package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"mint/internal/engine"
	"mint/internal/version"
)

// releaseFlags is the parsed `mint release` CLI surface: the bump selection, the
// -y skip, the global --plain render flag, and the --no-ai deliberate AI-skip flag.
// It is a plain value (no engine types beyond the bump) so flag parsing stays
// decoupled from the orchestrator; ReleaseOptions converts it into the engine's
// option struct.
type releaseFlags struct {
	// Bump is the resolved version bump (default BumpPatch).
	Bump version.Bump
	// Yes skips the interactive confirmation/notes-review gate (the presenter
	// performs the skip inside Prompt).
	Yes bool
	// Plain forces the plain (un-styled) presenter regardless of TTY.
	Plain bool
	// NoAI is the --no-ai deliberate-skip flag: it bypasses the AI notes path (after
	// the first-release and degenerate guards) in favour of the commit-subject
	// fallback body, never aborting.
	NoAI bool
}

// ReleaseOptions converts the parsed flags into the engine's run options, binding
// the production clock (time.Now) so the changelog date is the real release date.
func (f releaseFlags) ReleaseOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: f.Bump, Now: time.Now(), NoAI: f.NoAI}
}

// parseReleaseFlags parses the `mint release [bump] [options]` arguments into a
// releaseFlags. The bump selectors (-p/-m/-M and their long forms) are MUTUALLY
// EXCLUSIVE — combining more than one is a usage error, not silent precedence —
// and default to patch when none is given. -y/--yes and --plain are independent
// booleans.
func parseReleaseFlags(args []string) (releaseFlags, error) {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var patch, minor, major, yes, plain, noAI bool
	fs.BoolVar(&patch, "p", false, "patch bump (default)")
	fs.BoolVar(&patch, "patch", false, "patch bump (default)")
	fs.BoolVar(&minor, "m", false, "minor bump")
	fs.BoolVar(&minor, "minor", false, "minor bump")
	fs.BoolVar(&major, "M", false, "major bump")
	fs.BoolVar(&major, "major", false, "major bump")
	fs.BoolVar(&yes, "y", false, "skip the confirmation/notes-review gate")
	fs.BoolVar(&yes, "yes", false, "skip the confirmation/notes-review gate")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")
	fs.BoolVar(&noAI, "no-ai", false, "skip the AI notes path; use the commit-subject fallback body")

	if err := fs.Parse(args); err != nil {
		return releaseFlags{}, err
	}

	bump, err := resolveBump(patch, minor, major)
	if err != nil {
		return releaseFlags{}, err
	}
	return releaseFlags{Bump: bump, Yes: yes, Plain: plain, NoAI: noAI}, nil
}

// resolveBump maps the three mutually-exclusive bump booleans to a version.Bump.
// More than one set is a usage error; none set defaults to patch (also the
// version.Bump zero value).
func resolveBump(patch, minor, major bool) (version.Bump, error) {
	count := boolsSet(patch, minor, major)
	if count > 1 {
		return 0, fmt.Errorf("the bump flags -p/-m/-M are mutually exclusive; pass at most one")
	}

	switch {
	case major:
		return version.BumpMajor, nil
	case minor:
		return version.BumpMinor, nil
	default: // patch explicitly, or no bump flag (default patch)
		return version.BumpPatch, nil
	}
}

// boolsSet counts how many of the given booleans are true.
func boolsSet(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}
