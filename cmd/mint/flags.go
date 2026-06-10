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
// -y skip, the global --plain render flag, the --no-ai deliberate AI-skip flag, the
// --autostash WIP stash/restore escape hatch, and the --any-branch branch-gate
// bypass. It is a plain value (no engine types beyond the bump) so flag parsing stays
// decoupled from the orchestrator; ReleaseOptions converts it into the engine's option struct.
type releaseFlags struct {
	// Bump is the resolved version bump (default BumpPatch). It is ignored when
	// SetVersion is set — the two are mutually exclusive (parseReleaseFlags rejects
	// combining --set-version with a bump flag rather than applying silent precedence).
	Bump version.Bump
	// SetVersion is the raw --set-version value (e.g. "2.0.0" or "v2.0.0"): the
	// explicit-version escape hatch. It is threaded UNPARSED to the engine, which owns
	// the strict-SemVer parse and the strictly-greater-than-latest gate (those need the
	// configured tag_prefix and the resolved latest tag, neither known at flag time).
	// Empty (the default) selects the bump-flag path.
	SetVersion string
	// Yes skips the interactive confirmation/notes-review gate (the presenter
	// performs the skip inside Prompt).
	Yes bool
	// Plain forces the plain (un-styled) presenter regardless of TTY.
	Plain bool
	// NoAI is the --no-ai deliberate-skip flag: it bypasses the AI notes path (after
	// the first-release and degenerate guards) in favour of the commit-subject
	// fallback body, never aborting.
	NoAI bool
	// AutoStash is the --autostash escape hatch: it stashes unrelated WIP
	// (`git stash push --include-untracked`) before the clean-tree gate and restores it
	// after the run — including on abort/failure (the surgical unwind runs first, then
	// the pop). Opt-in, because the release mutates the tree and popping WIP on top can
	// conflict; opting in is the user asserting it is safe.
	AutoStash bool
	// AnyBranch is the --any-branch escape hatch: it bypasses the on-release-branch
	// preflight gate (the gate is skipped, not evaluated) so a deliberate off-branch
	// release proceeds. Every other gate still runs; the bypass is reported via the
	// presenter. Opt-in, because releasing off the release branch is normally a mistake.
	AnyBranch bool
}

// ReleaseOptions converts the parsed flags into the engine's run options, binding
// the production clock (time.Now) so the changelog date is the real release date.
func (f releaseFlags) ReleaseOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: f.Bump, SetVersion: f.SetVersion, Now: time.Now(), NoAI: f.NoAI, AutoStash: f.AutoStash, AnyBranch: f.AnyBranch}
}

// parseReleaseFlags parses the `mint release [bump] [options]` arguments into a
// releaseFlags. The bump selectors (-p/-m/-M and their long forms) are MUTUALLY
// EXCLUSIVE — combining more than one is a usage error, not silent precedence —
// and default to patch when none is given. -y/--yes and --plain are independent
// booleans.
func parseReleaseFlags(args []string) (releaseFlags, error) {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var patch, minor, major, yes, plain, noAI, autoStash, anyBranch bool
	var setVersion string
	fs.BoolVar(&patch, "p", false, "patch bump (default)")
	fs.BoolVar(&patch, "patch", false, "patch bump (default)")
	fs.BoolVar(&minor, "m", false, "minor bump")
	fs.BoolVar(&minor, "minor", false, "minor bump")
	fs.BoolVar(&major, "M", false, "major bump")
	fs.BoolVar(&major, "major", false, "major bump")
	fs.StringVar(&setVersion, "set-version", "", "explicit version X.Y.Z (mutually exclusive with bump flags)")
	fs.BoolVar(&yes, "y", false, "skip the confirmation/notes-review gate")
	fs.BoolVar(&yes, "yes", false, "skip the confirmation/notes-review gate")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")
	fs.BoolVar(&noAI, "no-ai", false, "skip the AI notes path; use the commit-subject fallback body")
	fs.BoolVar(&autoStash, "autostash", false, "stash/restore unrelated WIP around the run")
	fs.BoolVar(&anyBranch, "any-branch", false, "bypass the release-branch gate")

	if err := fs.Parse(args); err != nil {
		return releaseFlags{}, err
	}

	bump, err := resolveVersionSelection(setVersion, patch, minor, major)
	if err != nil {
		return releaseFlags{}, err
	}
	return releaseFlags{Bump: bump, SetVersion: setVersion, Yes: yes, Plain: plain, NoAI: noAI, AutoStash: autoStash, AnyBranch: anyBranch}, nil
}

// resolveVersionSelection enforces the version-selection rules and returns the
// resolved bump. --set-version and the bump flags are MUTUALLY EXCLUSIVE: combining
// --set-version with ANY of -p/-m/-M is a usage error (the exact spec message), not
// silent precedence. With --set-version set and no bump flag, the returned bump is
// irrelevant (the engine pins the version and ignores it) and left at the default.
// Without --set-version, selection falls through to resolveBump (the bump flags'
// own mutual exclusivity).
func resolveVersionSelection(setVersion string, patch, minor, major bool) (version.Bump, error) {
	if setVersion != "" && boolsSet(patch, minor, major) > 0 {
		return 0, fmt.Errorf("can't combine --set-version with a bump flag")
	}
	return resolveBump(patch, minor, major)
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
