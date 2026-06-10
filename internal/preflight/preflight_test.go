package preflight_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"mint/internal/preflight"
	"mint/internal/runner"
)

// TestCheckCleanTree_Empty_Passes verifies the strict clean-tree gate: an empty
// `git status --porcelain` (no tracked changes, no non-ignored untracked files)
// passes. Gitignored files are absent from porcelain output, so the gate never
// passes --ignored and they cannot trip it.
func TestCheckCleanTree_Empty_Passes(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	if err := preflight.CheckCleanTree(t.Context(), r); err != nil {
		t.Fatalf("CheckCleanTree returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 2 || got.Args[0] != "status" || got.Args[1] != "--porcelain" {
		t.Errorf("invocation = %+v, want git status --porcelain", got)
	}
}

func TestCheckCleanTree_DirtyOrUntracked_Fails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
	}{
		{
			name:   "modified tracked file",
			stdout: " M internal/version/version.go\n",
		},
		{
			name:   "staged tracked file",
			stdout: "M  internal/config/config.go\n",
		},
		{
			name:   "non-ignored untracked file",
			stdout: "?? scratch.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Any non-empty porcelain output — tracked changes OR non-ignored
			// untracked files — means a dirty tree and must abort.
			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{Stdout: tt.stdout}, nil)

			err := preflight.CheckCleanTree(t.Context(), r)
			if err == nil {
				t.Fatalf("CheckCleanTree returned nil error, want a dirty-tree abort")
			}

			var gateErr *preflight.GateError
			if !errors.As(err, &gateErr) {
				t.Fatalf("error = %v, want a *GateError", err)
			}
			if gateErr.Message() == "" {
				t.Error("GateError.Message() is empty, want an actionable abort message")
			}
		})
	}
}

func TestCheckOnBranch_Matches_Passes(t *testing.T) {
	t.Parallel()

	// `git rev-parse --abbrev-ref HEAD` prints the current branch with a trailing
	// newline; when it matches the release branch the gate passes.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "main\n"}, nil)

	if err := preflight.CheckOnBranch(t.Context(), r, "main"); err != nil {
		t.Fatalf("CheckOnBranch returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 3 ||
		got.Args[0] != "rev-parse" || got.Args[1] != "--abbrev-ref" || got.Args[2] != "HEAD" {
		t.Errorf("invocation = %+v, want git rev-parse --abbrev-ref HEAD", got)
	}
}

func TestCheckOnBranch_Differs_FailsNamingBoth(t *testing.T) {
	t.Parallel()

	// On a different branch than the release branch the gate aborts, naming both
	// the current branch and the expected release branch so the message is
	// actionable.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "feature/widget\n"}, nil)

	err := preflight.CheckOnBranch(t.Context(), r, "main")
	if err == nil {
		t.Fatalf("CheckOnBranch returned nil error, want an off-branch abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	msg := gateErr.Message()
	if !strings.Contains(msg, "feature/widget") || !strings.Contains(msg, "main") {
		t.Errorf("message = %q, want it to name both current branch and release branch", msg)
	}
}

func TestCheckTagFreeLocal_Absent_Passes(t *testing.T) {
	t.Parallel()

	// `git rev-parse -q --verify refs/tags/{tag}` exits non-zero with empty stdout
	// when the tag does NOT exist — the runner returns a populated Result alongside
	// a non-nil error for a clean ran-and-exited-non-zero. That absence is the PASS
	// case, so the gate must NOT treat that error as a hard failure.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "", ExitCode: 1}, errors.New("exit status 1"))

	if err := preflight.CheckTagFreeLocal(t.Context(), r, "v1.2.3"); err != nil {
		t.Fatalf("CheckTagFreeLocal returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 4 ||
		got.Args[0] != "rev-parse" || got.Args[1] != "-q" ||
		got.Args[2] != "--verify" || got.Args[3] != "refs/tags/v1.2.3" {
		t.Errorf("invocation = %+v, want git rev-parse -q --verify refs/tags/v1.2.3", got)
	}
}

func TestCheckTagFreeLocal_Exists_Fails(t *testing.T) {
	t.Parallel()

	// When the tag exists `git rev-parse -q --verify` exits zero and prints the
	// resolved object hash; the gate must abort, naming the conflicting tag.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "9fceb02d0ae598e95dc970b74767f19372d61af8\n"}, nil)

	err := preflight.CheckTagFreeLocal(t.Context(), r, "v1.2.3")
	if err == nil {
		t.Fatalf("CheckTagFreeLocal returned nil error, want a tag-exists abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if !strings.Contains(gateErr.Message(), "v1.2.3") {
		t.Errorf("message = %q, want it to name the existing tag", gateErr.Message())
	}
}

func TestCheckTagFreeLocal_CommandNotFound_IsHardError(t *testing.T) {
	t.Parallel()

	// A missing git binary is a real error, distinct from a clean
	// ran-and-exited-non-zero (the tag-absent PASS). It must surface (not be
	// mistaken for the tag being free) and not be a GateError.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	err := preflight.CheckTagFreeLocal(t.Context(), r, "v1.2.3")
	if err == nil {
		t.Fatalf("CheckTagFreeLocal returned nil error, want the missing-binary error to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}

	var gateErr *preflight.GateError
	if errors.As(err, &gateErr) {
		t.Errorf("error = %v, want a hard error, not a GateError", err)
	}
}

func TestRunLocalGates_AllPass(t *testing.T) {
	t.Parallel()

	// Clean tree + on the release branch + the target tag free locally -> all gates
	// pass and the ordered driver returns nil. The args-dispatching fake lets one
	// runner answer the three distinct git invocations the gates make.
	r := &argRunner{responses: map[string]scripted{
		"status --porcelain":                     {result: runner.Result{Stdout: ""}},
		"rev-parse --abbrev-ref HEAD":            {result: runner.Result{Stdout: "main\n"}},
		"rev-parse -q --verify refs/tags/v1.2.3": {result: runner.Result{ExitCode: 1}, err: errExit},
	}}

	if err := preflight.RunLocalGates(t.Context(), r, "main", "v1.2.3"); err != nil {
		t.Fatalf("RunLocalGates returned unexpected error: %v", err)
	}
}

func TestRunLocalGates_CheapFirstAbort(t *testing.T) {
	t.Parallel()

	// The gates run cheap-first (clean tree, then on-branch, then tag-free) and
	// abort on the first failure. With a dirty tree the driver must stop after the
	// clean-tree gate — the branch and tag gates never run.
	r := &argRunner{responses: map[string]scripted{
		"status --porcelain": {result: runner.Result{Stdout: " M file.go\n"}},
	}}

	err := preflight.RunLocalGates(t.Context(), r, "main", "v1.2.3")
	if err == nil {
		t.Fatalf("RunLocalGates returned nil error, want the clean-tree abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if got := len(r.calls); got != 1 {
		t.Fatalf("git invocations = %d, want 1 (aborted before branch/tag gates)", got)
	}
	if r.calls[0] != "status --porcelain" {
		t.Errorf("first invocation = %q, want the clean-tree check to run first", r.calls[0])
	}
}

func TestFetch_RunsFetchTags(t *testing.T) {
	t.Parallel()

	// Fetch must run `git fetch --tags` so the complete tag set and upstream refs
	// are current before the remote gates (and before "latest" is trusted). It is
	// the only command Fetch issues, and it must never be a pull.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	if err := preflight.Fetch(t.Context(), r); err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 2 || got.Args[0] != "fetch" || got.Args[1] != "--tags" {
		t.Errorf("invocation = %+v, want git fetch --tags", got)
	}
}

func TestFetch_NeverPulls(t *testing.T) {
	t.Parallel()

	// mint must NEVER run `git pull`: auto-integration would silently drag in and
	// release unseen remote commits. Fetch updates refs read-only; assert no pull.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	if err := preflight.Fetch(t.Context(), r); err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}

	for _, inv := range r.Invocations() {
		for _, arg := range inv.Args {
			if arg == "pull" {
				t.Fatalf("Fetch invoked git pull (%+v); mint must never pull", inv)
			}
		}
	}
}

func TestFetch_CommandNotFound_IsHardError(t *testing.T) {
	t.Parallel()

	// A missing git binary is a real infrastructure failure and must surface as-is
	// so it is never mistaken for a successful fetch.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	err := preflight.Fetch(t.Context(), r)
	if err == nil {
		t.Fatalf("Fetch returned nil error, want the missing-binary error to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestCheckRemoteSync_UpToDate_Passes(t *testing.T) {
	t.Parallel()

	// `git rev-list --left-right --count @{u}...HEAD` prints "<behind>\t<ahead>".
	// "0\t0" means up-to-date with upstream — the gate passes and issues exactly the
	// one rev-list call (never a pull).
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "0\t0\n"}, nil)

	if err := preflight.CheckRemoteSync(t.Context(), r, "main"); err != nil {
		t.Fatalf("CheckRemoteSync returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 4 ||
		got.Args[0] != "rev-list" || got.Args[1] != "--left-right" ||
		got.Args[2] != "--count" || got.Args[3] != "@{u}...HEAD" {
		t.Errorf("invocation = %+v, want git rev-list --left-right --count @{u}...HEAD", got)
	}
}

func TestCheckRemoteSync_AheadOnly_Passes(t *testing.T) {
	t.Parallel()

	// Purely ahead (0 behind, >0 ahead) is the expected release state — those local
	// commits are what is being released — so the gate passes.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "0\t3\n"}, nil)

	if err := preflight.CheckRemoteSync(t.Context(), r, "main"); err != nil {
		t.Fatalf("CheckRemoteSync returned unexpected error for ahead-only: %v", err)
	}
}

func TestCheckRemoteSync_Behind_AbortsWithCount(t *testing.T) {
	t.Parallel()

	// Behind upstream (>0 behind, 0 ahead): auto-pulling would silently drag in
	// unseen commits, so the gate aborts. The message must carry the behind count
	// and name the upstream so it is actionable.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "2\t0\n"}, nil)

	err := preflight.CheckRemoteSync(t.Context(), r, "main")
	if err == nil {
		t.Fatalf("CheckRemoteSync returned nil error, want a behind-upstream abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	msg := gateErr.Message()
	if !strings.Contains(msg, "2") {
		t.Errorf("message = %q, want it to include the behind commit count", msg)
	}
	if !strings.Contains(msg, "origin/main") {
		t.Errorf("message = %q, want it to name the upstream (origin/main)", msg)
	}
}

func TestCheckRemoteSync_Diverged_Aborts(t *testing.T) {
	t.Parallel()

	// Diverged (>0 behind AND >0 ahead): local history has commits the upstream
	// lacks and vice versa. Integrating must be a conscious act, so the gate aborts
	// (never auto-pulling), still surfacing the behind count.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "1\t4\n"}, nil)

	err := preflight.CheckRemoteSync(t.Context(), r, "main")
	if err == nil {
		t.Fatalf("CheckRemoteSync returned nil error, want a diverged abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if !strings.Contains(gateErr.Message(), "1") {
		t.Errorf("message = %q, want it to include the behind commit count", gateErr.Message())
	}
}

func TestCheckRemoteSync_NeverPulls(t *testing.T) {
	t.Parallel()

	// The sync gate computes ahead/behind read-only; it must never invoke git pull
	// regardless of the result.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "0\t0\n"}, nil)

	if err := preflight.CheckRemoteSync(t.Context(), r, "main"); err != nil {
		t.Fatalf("CheckRemoteSync returned unexpected error: %v", err)
	}

	for _, inv := range r.Invocations() {
		for _, arg := range inv.Args {
			if arg == "pull" {
				t.Fatalf("CheckRemoteSync invoked git pull (%+v); mint must never pull", inv)
			}
		}
	}
}

func TestCheckRemoteSync_NoUpstream_IsDistinguishable(t *testing.T) {
	t.Parallel()

	// With no tracking branch, `git rev-list @{u}...HEAD` exits non-zero with a
	// "no upstream configured" fatal on stderr. The gate must surface this as a
	// clearly-distinguishable condition (ErrNoUpstream) the caller can report — not
	// crash, and not be confused with a normal behind/diverged GateError abort.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{
		Stderr:   "fatal: no upstream configured for branch 'main'\n",
		ExitCode: 128,
	}, errExit)

	err := preflight.CheckRemoteSync(t.Context(), r, "main")
	if err == nil {
		t.Fatalf("CheckRemoteSync returned nil error, want a no-upstream condition")
	}
	if !errors.Is(err, preflight.ErrNoUpstream) {
		t.Errorf("error = %v, want it to match ErrNoUpstream", err)
	}

	var gateErr *preflight.GateError
	if errors.As(err, &gateErr) {
		t.Errorf("error = %v, want the distinct no-upstream condition, not a GateError", err)
	}
}

func TestCheckRemoteSync_CommandNotFound_IsHardError(t *testing.T) {
	t.Parallel()

	// A missing git binary is a genuine infrastructure error, distinct from a
	// no-upstream condition; it must surface (matching ErrCommandNotFound) and not
	// be reported as a no-upstream or gate abort.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	err := preflight.CheckRemoteSync(t.Context(), r, "main")
	if err == nil {
		t.Fatalf("CheckRemoteSync returned nil error, want the missing-binary error to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
	if errors.Is(err, preflight.ErrNoUpstream) {
		t.Errorf("error = %v, want a hard error, not ErrNoUpstream", err)
	}
}

func TestCheckTagFreeRemote_Absent_Passes(t *testing.T) {
	t.Parallel()

	// `git ls-remote --tags origin refs/tags/{tag}` prints nothing when the tag is
	// absent on the remote — the PASS case. The gate issues exactly that one call.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil)

	if err := preflight.CheckTagFreeRemote(t.Context(), r, "v1.2.3"); err != nil {
		t.Fatalf("CheckTagFreeRemote returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 4 ||
		got.Args[0] != "ls-remote" || got.Args[1] != "--tags" ||
		got.Args[2] != "origin" || got.Args[3] != "refs/tags/v1.2.3" {
		t.Errorf("invocation = %+v, want git ls-remote --tags origin refs/tags/v1.2.3", got)
	}
}

func TestCheckTagFreeRemote_Exists_Fails(t *testing.T) {
	t.Parallel()

	// A non-empty ls-remote result (the resolved hash + ref) means the tag already
	// exists on the remote; the gate must abort, naming the conflicting tag.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{
		Stdout: "9fceb02d0ae598e95dc970b74767f19372d61af8\trefs/tags/v1.2.3\n",
	}, nil)

	err := preflight.CheckTagFreeRemote(t.Context(), r, "v1.2.3")
	if err == nil {
		t.Fatalf("CheckTagFreeRemote returned nil error, want a tag-exists-on-remote abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if !strings.Contains(gateErr.Message(), "v1.2.3") {
		t.Errorf("message = %q, want it to name the existing tag", gateErr.Message())
	}
	if !strings.Contains(gateErr.Message(), "remote") {
		t.Errorf("message = %q, want it to say the tag is on the remote", gateErr.Message())
	}
}

func TestCheckTagFreeRemote_CommandNotFound_IsHardError(t *testing.T) {
	t.Parallel()

	// A missing git binary is a real error, distinct from an empty (tag-absent)
	// result. It must surface (matching ErrCommandNotFound) and not be a GateError —
	// so it is never mistaken for the tag being free.
	r := runner.NewFakeRunner()
	r.SeedNotFound("git")

	err := preflight.CheckTagFreeRemote(t.Context(), r, "v1.2.3")
	if err == nil {
		t.Fatalf("CheckTagFreeRemote returned nil error, want the missing-binary error to surface")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}

	var gateErr *preflight.GateError
	if errors.As(err, &gateErr) {
		t.Errorf("error = %v, want a hard error, not a GateError", err)
	}
}

func TestFetchThenRemoteGates_FetchPrecedesChecks(t *testing.T) {
	t.Parallel()

	// Ordering invariant: `git fetch --tags` must run before the remote checks so the
	// full tag set and upstream refs are visible. The args-dispatching fake answers
	// each distinct git invocation and records call order for the assertion.
	r := &argRunner{responses: map[string]scripted{
		"fetch --tags": {result: runner.Result{Stdout: ""}},
		"rev-list --left-right --count @{u}...HEAD": {result: runner.Result{Stdout: "0\t2\n"}},
		"ls-remote --tags origin refs/tags/v1.2.3":  {result: runner.Result{Stdout: ""}},
	}}

	if err := preflight.Fetch(t.Context(), r); err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if err := preflight.CheckRemoteSync(t.Context(), r, "main"); err != nil {
		t.Fatalf("CheckRemoteSync returned unexpected error: %v", err)
	}
	if err := preflight.CheckTagFreeRemote(t.Context(), r, "v1.2.3"); err != nil {
		t.Fatalf("CheckTagFreeRemote returned unexpected error: %v", err)
	}

	if len(r.calls) != 3 {
		t.Fatalf("calls = %v, want 3", r.calls)
	}
	if r.calls[0] != "fetch --tags" {
		t.Errorf("first call = %q, want the fetch to precede the remote checks", r.calls[0])
	}
}

func TestCheckGhAuth_InstalledAndAuthenticated_Passes(t *testing.T) {
	t.Parallel()

	// gh installed and authenticated: `gh auth status` exits zero. The gate passes
	// and issues exactly that one probe — the conditional pre-tag gate that the
	// orchestrator runs only when publishing.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{Stdout: "github.com\n  ✓ Logged in to github.com\n"}, nil)

	if err := preflight.CheckGhAuth(t.Context(), r); err != nil {
		t.Fatalf("CheckGhAuth returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "gh" ||
		len(got.Args) != 2 || got.Args[0] != "auth" || got.Args[1] != "status" {
		t.Errorf("invocation = %+v, want gh auth status", got)
	}
}

func TestCheckGhAuth_NotInstalled_FailsWithInstallMessage(t *testing.T) {
	t.Parallel()

	// gh missing: the runner reports ErrCommandNotFound. Because this gate runs
	// BEFORE the tag, this aborts the release before any tag/push — a missing gh
	// never strands a pushed tag with no release. The abort is a *GateError naming
	// the "not installed" condition (distinct from "not authenticated"), not a raw
	// infrastructure error.
	r := runner.NewFakeRunner()
	r.SeedNotFound("gh")

	err := preflight.CheckGhAuth(t.Context(), r)
	if err == nil {
		t.Fatalf("CheckGhAuth returned nil error, want a gh-not-installed abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if !strings.Contains(gateErr.Message(), "not installed") {
		t.Errorf("message = %q, want it to say gh is not installed", gateErr.Message())
	}
}

func TestCheckGhAuth_NotAuthenticated_FailsWithAuthMessage(t *testing.T) {
	t.Parallel()

	// gh installed but not authenticated: `gh auth status` exits non-zero with a
	// populated Result alongside a non-nil error (the runner contract). This is an
	// expected condition, NOT an infrastructure crash, so the gate must branch on the
	// non-zero exit and abort with a *GateError naming the "not authenticated"
	// condition — distinct from "not installed" — again before any tag/push.
	r := runner.NewFakeRunner()
	r.Seed("gh", runner.Result{
		Stderr:   "You are not logged into any GitHub hosts. Run gh auth login to authenticate.\n",
		ExitCode: 1,
	}, errExit)

	err := preflight.CheckGhAuth(t.Context(), r)
	if err == nil {
		t.Fatalf("CheckGhAuth returned nil error, want a gh-not-authenticated abort")
	}

	var gateErr *preflight.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("error = %v, want a *GateError", err)
	}
	if !strings.Contains(gateErr.Message(), "not authenticated") {
		t.Errorf("message = %q, want it to say gh is not authenticated", gateErr.Message())
	}
}

// errExit models a clean ran-and-exited-non-zero, mirroring the real runner's
// contract where a non-zero exit returns a populated Result with a non-nil error.
var errExit = errors.New("exit status 1")

// scripted pairs a Result with its error so the args-dispatching fake can model a
// clean ran-and-exited-non-zero (populated Result alongside a non-nil error).
type scripted struct {
	result runner.Result
	err    error
}

// argRunner is a test-local CommandRunner that dispatches on the joined args, so
// a single runner can answer the distinct git invocations the gates make in one
// pass (FakeRunner matches on command name only and cannot). It records each call
// in order for ordering/abort assertions.
type argRunner struct {
	responses map[string]scripted
	calls     []string
}

func (a *argRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return a.RunWith(ctx, nil, name, args...)
}

func (a *argRunner) RunWith(_ context.Context, _ io.Reader, _ string, args ...string) (runner.Result, error) {
	key := strings.Join(args, " ")
	a.calls = append(a.calls, key)

	s, ok := a.responses[key]
	if !ok {
		return runner.Result{}, errors.New("argRunner: no response seeded for " + key)
	}
	return s.result, s.err
}
