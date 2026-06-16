package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"mint/internal/setupguide"
)

// TestRunSetupEmitsGuideAndExitsZero proves the happy path: mint setup writes the
// embedded setup guide (setupguide.Guide()) byte-for-byte to stdout, nothing to
// stderr, and exits 0. It is the pure-text-emit contract — no gate, no footer.
func TestRunSetupEmitsGuideAndExitsZero(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	if code := runSetup(nil, &out, &errBuf); code != 0 {
		t.Fatalf("runSetup exit code = %d, want 0", code)
	}
	if out.String() != setupguide.Guide() {
		t.Errorf("runSetup stdout = %q, want setupguide.Guide()", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("runSetup wrote to stderr: %q", errBuf.String())
	}
}

// TestRunSetupRunsAnywhere_NoRepoResolution proves mint setup runs unconditionally:
// driven from a fresh temp dir that is NOT a git repository, it still emits the guide
// and exits 0. runSetup constructs no runner and resolves no repo root, so the absence
// of a git work tree cannot fail it — the cwd-confirm safety lives in the emitted
// instructions, not in a cmd-layer guard (the explicit divergence from runInit).
func TestRunSetupRunsAnywhere_NoRepoResolution(t *testing.T) {
	dir := t.TempDir() // a fresh dir, deliberately not `git init`-ed
	t.Chdir(dir)

	var out, errBuf bytes.Buffer
	if code := runSetup(nil, &out, &errBuf); code != 0 {
		t.Fatalf("runSetup exit code = %d outside a git repo, want 0 (must run anywhere)", code)
	}
	if out.String() != setupguide.Guide() {
		t.Errorf("runSetup stdout outside a repo = %q, want setupguide.Guide()", out.String())
	}
}

// TestRunSetupHelpExitsZero proves mint setup --help is a requested action: it prints
// the curated setupUsage to stdout via the flag.ErrHelp path and exits 0 — NOT the
// usage-error exit 2 — for both -h and --help spellings.
func TestRunSetupHelpExitsZero(t *testing.T) {
	t.Parallel()

	for _, helpArg := range []string{"-h", "--help"} {
		var out, errBuf bytes.Buffer
		if code := runSetup([]string{helpArg}, &out, &errBuf); code != 0 {
			t.Errorf("mint setup %s exited %d, want 0", helpArg, code)
		}
		if out.String() != setupUsage {
			t.Errorf("mint setup %s stdout = %q, want setupUsage", helpArg, out.String())
		}
		if errBuf.Len() != 0 {
			t.Errorf("mint setup %s wrote to stderr: %q", helpArg, errBuf.String())
		}
	}
}

// TestRunSetupUnknownFlagIsUsageError proves an unrecognised flag is a usage error:
// exit usageExitCode with the diagnostic on stderr (not stdout), so a typo fails
// loudly rather than being silently ignored or treated as a help request.
func TestRunSetupUnknownFlagIsUsageError(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	if code := runSetup([]string{"--nope"}, &out, &errBuf); code != usageExitCode {
		t.Errorf("mint setup --nope exited %d, want the usage error %d", code, usageExitCode)
	}
	if errBuf.Len() == 0 {
		t.Error("mint setup --nope wrote nothing to stderr; want a usage diagnostic")
	}
	if out.Len() != 0 {
		t.Errorf("mint setup --nope wrote to stdout: %q (the diagnostic belongs on stderr)", out.String())
	}
}

// TestParseSetupFlagsHelpSurfacesErrHelp proves the setup parser surfaces -h/--help as
// flag.ErrHelp (with the flag set's own dump discarded), so the cmd layer can route it
// to the curated setupUsage. mint setup takes no flags beyond the implicit --help/-h.
func TestParseSetupFlagsHelpSurfacesErrHelp(t *testing.T) {
	t.Parallel()

	for _, helpArg := range []string{"-h", "--help"} {
		if err := parseSetupFlags([]string{helpArg}); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("parseSetupFlags(%s) err = %v, want flag.ErrHelp", helpArg, err)
		}
	}
}

// TestRunListsSetupInUnknownCommandMessage proves the unknown-command diagnostic lists
// `mint setup` among the wired commands, so a typo's diagnostic enumerates every wired
// command (including the newly wired setup route).
func TestRunListsSetupInUnknownCommandMessage(t *testing.T) {
	t.Parallel()

	if !strings.Contains(unknownCommandMessage, "mint setup") {
		t.Errorf("unknown-command message = %q, want it to list `mint setup`", unknownCommandMessage)
	}
}
