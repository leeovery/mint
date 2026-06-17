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
	"io"
	"os"
	"strings"
	"testing"

	"mint/internal/setupguide"
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
		"setup":      func(a []string) error { return parseSetupFlags(a) },
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
		"setup":      func() int { return runSetup([]string{"--help"}, io.Discard, io.Discard) },
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

// TestRun_Setup_EmitsGuideToStdoutAndExitsZero drives the full dispatch path
// run([]string{"setup"}) end to end — classifyCommand routing, the commandSetup
// switch arm, and runSetup — and proves the composition: the dispatched route
// emits the setup guide to STDOUT (asserted via the stable setupguide.MarkerPipeline
// marker, not fragile prose) and exits 0. The emitter's internal rendering is
// covered by setup_test.go; this test asserts ONLY the seam between the three
// independently-tested parts. Capturing stdout and stderr on separate pipes makes
// the assertion bite on an argument-order swap: if the commandSetup arm wrote the
// guide to stderr, the stdout capture would be empty and the marker assertion fails.
// It is NOT t.Parallel() because it swaps the process-global os.Stdout/os.Stderr.
func TestRun_Setup_EmitsGuideToStdoutAndExitsZero(t *testing.T) {
	stdout, stderr := captureStdStreams(t)

	if code := run([]string{"setup"}); code != 0 {
		t.Fatalf("run([setup]) exit code = %d, want 0", code)
	}

	out, errOut := stdout(), stderr()
	if !strings.Contains(out, setupguide.MarkerPipeline) {
		t.Errorf("run([setup]) stdout missing the guide marker %q; got %q", setupguide.MarkerPipeline, out)
	}
	if strings.Contains(errOut, setupguide.MarkerPipeline) {
		t.Errorf("run([setup]) wrote the guide to stderr (arg-order swap?); stderr = %q", errOut)
	}
}

// TestRun_Setup_UnknownFlag_ExitsUsageError drives the malformed `mint setup --nope`
// through the full dispatch path: the commandSetup flag parse rejects the unknown flag
// (it is not flag.ErrHelp), so run() must return the usage-error exit code and write the
// diagnostic to stderr — never the guide to stdout. setup_test.go proves this at the
// runner level; this closes the symmetry with the happy-path TestRun_Setup_* test by
// proving the error path through the run() switch. NOT t.Parallel(): it swaps the
// process-global os.Stdout/os.Stderr.
func TestRun_Setup_UnknownFlag_ExitsUsageError(t *testing.T) {
	stdout, stderr := captureStdStreams(t)

	if code := run([]string{"setup", "--nope"}); code != usageExitCode {
		t.Fatalf("run([setup --nope]) exit code = %d, want the usage error %d", code, usageExitCode)
	}

	out, errOut := stdout(), stderr()
	if errOut == "" {
		t.Errorf("run([setup --nope]) wrote no diagnostic to stderr")
	}
	if strings.Contains(out, setupguide.MarkerPipeline) {
		t.Errorf("run([setup --nope]) emitted the guide to stdout despite the flag error; stdout = %q", out)
	}
}

// captureStdStreams redirects the process-global os.Stdout and os.Stderr to pipes
// for the duration of the test, restoring the originals on cleanup. It returns two
// readers that drain and return each captured stream — call them AFTER the code
// under test has run. It exists so a run()-level test can prove which stream output
// landed on (run writes the setup guide straight to os.Stdout, not an injected
// writer), distinguishing a stdout emit from a stderr one.
func captureStdStreams(t *testing.T) (stdout, stderr func() string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stdout): %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stderr): %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = outR.Close()
		_ = errR.Close()
	})

	drain := func(w *os.File, r *os.File, name string) func() string {
		return func() string {
			_ = w.Close()
			var b strings.Builder
			if _, err := io.Copy(&b, r); err != nil {
				t.Fatalf("draining %s pipe: %v", name, err)
			}
			return b.String()
		}
	}
	return drain(outW, outR, "stdout"), drain(errW, errR, "stderr")
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
		{"regenerate", regenerateUsage, []string{"--source", "--target", "--all", "--yes", "--plain"}},
		{"commit", commitUsage, []string{"--all", "--add-all", "--push", "--yes", "--no-ai", "--plain"}},
		{"init", initUsage, []string{"--force", "--plain"}},
		{"setup", setupUsage, []string{"--help"}},
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
	for _, verb := range []string{"release", "regenerate", "commit", "init", "version", "setup"} {
		if !strings.Contains(rootUsage, verb) {
			t.Errorf("root usage is missing the %s command", verb)
		}
	}
}

// TestRootUsage_SetupDescriptionPrintsNotWrites pins the spec-decided distinction:
// the rootUsage setup line says the command PRINTS/EMITS the guide, never that it
// writes files — `mint setup` only emits text; `mint init` is the verb that writes.
func TestRootUsage_SetupDescriptionPrintsNotWrites(t *testing.T) {
	t.Parallel()

	var setupLine string
	for _, line := range strings.Split(rootUsage, "\n") {
		if strings.Contains(line, "setup") {
			setupLine = line
			break
		}
	}
	if setupLine == "" {
		t.Fatal("rootUsage has no setup command line")
	}
	if !strings.Contains(setupLine, "print") && !strings.Contains(setupLine, "emit") {
		t.Errorf("rootUsage setup line must state it prints/emits the guide, got %q", setupLine)
	}
	if strings.Contains(setupLine, "write") || strings.Contains(setupLine, "scaffold") {
		t.Errorf("rootUsage setup line must NOT claim it writes files, got %q", setupLine)
	}
}

// TestRootUsage_CarriesNoConfigReference pins that `mint help` stays the frozen
// curated text: it gains only the setup command line and never the config reference.
// Humans get config reference from the GitHub docs / README; the agent gets it from
// `mint setup`. A guard so a future edit cannot retrofit the config table into the
// top-level help surface.
func TestRootUsage_CarriesNoConfigReference(t *testing.T) {
	t.Parallel()

	if strings.Contains(rootUsage, setupguide.MarkerConfigReference) {
		t.Errorf("rootUsage must not carry the config-reference marker %q", setupguide.MarkerConfigReference)
	}
	for _, key := range []string{"diff_exclude", "tag_prefix", "ai_command", "max_diff_lines"} {
		if strings.Contains(rootUsage, key) {
			t.Errorf("rootUsage must not carry the config key %q (mint help stays free of the config reference)", key)
		}
	}
}
