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
