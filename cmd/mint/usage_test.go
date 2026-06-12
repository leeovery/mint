package main

// The --help surface: -h/--help on any verb surfaces flag.ErrHelp from the parser,
// the verb runner prints the curated usage text to stdout and exits 0 (a help
// request is a requested action, not a usage error — usage errors stay exit 2), and
// `mint help`/`mint --help` print the top-level command list. The usage texts are
// hand-written (the paired short/long registration would make PrintDefaults list
// every boolean twice), so content tests pin each text to its flag set to catch a
// flag added without its help line.

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

// TestParseFlags_HelpSurfacesErrHelp proves every verb parser surfaces -h/--help as
// flag.ErrHelp (with the flag set's own usage dump discarded), so the cmd layer can
// route it to the curated usage text.
func TestParseFlags_HelpSurfacesErrHelp(t *testing.T) {
	t.Parallel()

	parsers := map[string]func([]string) error{
		"release":    func(a []string) error { _, err := parseReleaseFlags(a); return err },
		"regenerate": func(a []string) error { _, err := parseRegenerateFlags(a); return err },
		"commit":     func(a []string) error { _, err := parseCommitFlags(a); return err },
		"init":       func(a []string) error { _, err := parseInitFlags(a); return err },
	}
	for name, parse := range parsers {
		for _, helpArg := range []string{"-h", "--help"} {
			if err := parse([]string{helpArg}); !errors.Is(err, flag.ErrHelp) {
				t.Errorf("%s %s: err = %v, want flag.ErrHelp", name, helpArg, err)
			}
		}
	}
}

// TestRunVerb_Help_ExitsZero proves a per-verb --help exits 0 — distinct from the
// usage-error exit 2 — and runs nothing (the help branch returns before any seam is
// constructed, so no git/config is touched).
func TestRunVerb_Help_ExitsZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runs := map[string]func() int{
		"release":    func() int { return runRelease(ctx, []string{"--help"}) },
		"regenerate": func() int { return runRegenerate(ctx, []string{"--help"}) },
		"commit":     func() int { return runCommit(ctx, []string{"--help"}) },
		"init":       func() int { return runInit(ctx, []string{"--help"}) },
	}
	for name, run := range runs {
		if code := run(); code != 0 {
			t.Errorf("mint %s --help exited %d, want 0", name, code)
		}
	}
}

// TestRun_TopLevelHelp_ExitsZero proves `mint help`, `mint -h`, and `mint --help`
// route to the top-level usage with exit 0 (an unknown command stays a usage error).
func TestRun_TopLevelHelp_ExitsZero(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		if code := run(args); code != 0 {
			t.Errorf("mint %s exited %d, want 0", strings.Join(args, " "), code)
		}
	}
	if code := run([]string{"frobnicate"}); code != usageExitCode {
		t.Errorf("mint frobnicate exited %d, want the usage error %d", code, usageExitCode)
	}
}

// TestUsageTexts_CoverTheirFlagSets pins every registered long-form flag to a
// mention in its verb's usage text, so a flag added to a parser without a help line
// fails here rather than shipping undocumented.
func TestUsageTexts_CoverTheirFlagSets(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		usage string
		flags []string
	}{
		{"release", releaseUsage, []string{"--patch", "--minor", "--major", "--set-version", "--dry-run", "--yes", "--no-ai", "--autostash", "--any-branch", "--plain"}},
		{"regenerate", regenerateUsage, []string{"--reuse", "--fresh", "--target", "--all", "--yes", "--plain"}},
		{"commit", commitUsage, []string{"--all", "--add-all", "--push", "--yes", "--no-ai", "--plain"}},
		{"init", initUsage, []string{"--force", "--plain"}},
	} {
		for _, f := range tc.flags {
			if !strings.Contains(tc.usage, f) {
				t.Errorf("%s usage text is missing %s", tc.name, f)
			}
		}
		if !strings.HasPrefix(tc.usage, "usage: mint ") {
			t.Errorf("%s usage text must open with the synopsis line, got %q…", tc.name, tc.usage[:20])
		}
	}
	for _, verb := range []string{"release", "regenerate", "commit", "init", "version"} {
		if !strings.Contains(rootUsage, verb) {
			t.Errorf("root usage is missing the %s command", verb)
		}
	}
}
