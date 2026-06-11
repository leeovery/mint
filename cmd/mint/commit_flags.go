package main

import (
	"flag"
	"io"
)

// commitFlags is the parsed bare `mint commit` CLI surface. Phase 1 (the walking
// skeleton) wires only the presentation flags the presenter consumes at startup:
// --plain forces un-styled output, and -y/--yes auto-accepts. The staging (-a/-A),
// push (-p), and AI-skip (--no-ai) flags are LATER phases and are deliberately NOT
// parsed here — adding them before their behaviour exists would advertise a no-op
// flag. It is a plain value so flag parsing stays decoupled from the orchestrator.
type commitFlags struct {
	// Yes auto-accepts the (future) review gate; the presenter performs the skip
	// inside Prompt. The bare path has no gate yet (task 1-5), so it only feeds the
	// presenter's startup mode today.
	Yes bool
	// Plain forces the plain (un-styled) presenter regardless of TTY.
	Plain bool
}

// parseCommitFlags parses the bare `mint commit [options]` arguments into a
// commitFlags. -y/--yes and --plain are independent booleans; an unrecognised flag
// is a parse error the cmd layer surfaces as a usage error.
func parseCommitFlags(args []string) (commitFlags, error) {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // main prints its own error; suppress flag's default usage dump

	var yes, plain bool
	fs.BoolVar(&yes, "y", false, "skip the review gate (auto-accept)")
	fs.BoolVar(&yes, "yes", false, "skip the review gate (auto-accept)")
	fs.BoolVar(&plain, "plain", false, "force plain (un-styled) output")

	if err := fs.Parse(args); err != nil {
		return commitFlags{}, err
	}
	return commitFlags{Yes: yes, Plain: plain}, nil
}
