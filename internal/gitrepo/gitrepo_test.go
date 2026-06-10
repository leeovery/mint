package gitrepo_test

import (
	"errors"
	"testing"

	"mint/internal/config"
	"mint/internal/gitrepo"
	"mint/internal/runner"
)

func TestResolveRoot_ReturnsTrimmedToplevel(t *testing.T) {
	t.Parallel()

	// `git rev-parse --show-toplevel` prints the absolute repo root with a
	// trailing newline; ResolveRoot must return the trimmed path.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "/Users/dev/project\n"}, nil)

	root, err := gitrepo.ResolveRoot(t.Context(), r)
	if err != nil {
		t.Fatalf("ResolveRoot returned unexpected error: %v", err)
	}

	if root != "/Users/dev/project" {
		t.Errorf("root = %q, want %q", root, "/Users/dev/project")
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if got := invs[0]; got.Name != "git" ||
		len(got.Args) != 2 || got.Args[0] != "rev-parse" || got.Args[1] != "--show-toplevel" {
		t.Errorf("invocation = %+v, want git rev-parse --show-toplevel", got)
	}
}

func TestResolveRoot_NotAGitRepo_AbortsCleanly(t *testing.T) {
	t.Parallel()

	// Outside a git repo, `git rev-parse --show-toplevel` exits non-zero with a
	// populated Result. ResolveRoot must surface a clean error (no panic) and an
	// empty root.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{
		Stderr:   "fatal: not a git repository (or any of the parent directories): .git\n",
		ExitCode: 128,
	}, errors.New("exit status 128"))

	root, err := gitrepo.ResolveRoot(t.Context(), r)
	if err == nil {
		t.Fatalf("ResolveRoot returned nil error, want a not-a-repo abort")
	}
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("error = %v, want it to match ErrNotARepository", err)
	}
	if root != "" {
		t.Errorf("root = %q, want empty on failure", root)
	}
}

func TestResolveReleaseBranch_ConfigOverride_UsedVerbatim(t *testing.T) {
	t.Parallel()

	// A non-empty release_branch override is used as-is and origin/HEAD derivation
	// is skipped entirely — the runner is never called.
	r := runner.NewFakeRunner()
	cfg := config.Config{Release: config.Release{ReleaseBranch: "release/next"}}

	branch, err := gitrepo.ResolveReleaseBranch(t.Context(), r, cfg)
	if err != nil {
		t.Fatalf("ResolveReleaseBranch returned unexpected error: %v", err)
	}

	if branch != "release/next" {
		t.Errorf("branch = %q, want %q", branch, "release/next")
	}
	if got := len(r.Invocations()); got != 0 {
		t.Errorf("invocations = %d, want 0 (derivation skipped on override)", got)
	}
}

func TestResolveReleaseBranch_NoOverride_DerivesFromOriginHEAD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stdout   string
		expected string
	}{
		{name: "main", stdout: "origin/main\n", expected: "main"},
		{name: "master", stdout: "origin/master\n", expected: "master"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// `git symbolic-ref --short refs/remotes/origin/HEAD` yields
			// `origin/<branch>`; the derived release branch is the short name with the
			// `origin/` prefix stripped.
			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{Stdout: tt.stdout}, nil)
			cfg := config.Config{} // empty release_branch -> derive

			branch, err := gitrepo.ResolveReleaseBranch(t.Context(), r, cfg)
			if err != nil {
				t.Fatalf("ResolveReleaseBranch returned unexpected error: %v", err)
			}

			if branch != tt.expected {
				t.Errorf("branch = %q, want %q", branch, tt.expected)
			}

			invs := r.Invocations()
			if len(invs) != 1 {
				t.Fatalf("invocations = %d, want 1", len(invs))
			}
			if got := invs[0]; got.Name != "git" ||
				len(got.Args) != 3 ||
				got.Args[0] != "symbolic-ref" ||
				got.Args[1] != "--short" ||
				got.Args[2] != "refs/remotes/origin/HEAD" {
				t.Errorf("invocation = %+v, want git symbolic-ref --short refs/remotes/origin/HEAD", got)
			}
		})
	}
}

func TestResolveReleaseBranch_OriginHEADUnset_SurfacesDistinguishableError(t *testing.T) {
	t.Parallel()

	// With no override and origin/HEAD unset, `git symbolic-ref` exits non-zero.
	// ResolveReleaseBranch must surface a distinguishable condition rather than
	// silently defaulting to main/master.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{
		Stderr:   "fatal: ref refs/remotes/origin/HEAD is not a symbolic ref\n",
		ExitCode: 128,
	}, errors.New("exit status 128"))
	cfg := config.Config{}

	branch, err := gitrepo.ResolveReleaseBranch(t.Context(), r, cfg)
	if err == nil {
		t.Fatalf("ResolveReleaseBranch returned nil error, want origin/HEAD-unset condition")
	}
	if !errors.Is(err, gitrepo.ErrOriginHeadUnset) {
		t.Errorf("error = %v, want it to match ErrOriginHeadUnset", err)
	}
	if branch != "" {
		t.Errorf("branch = %q, want empty on failure (no silent default)", branch)
	}
}
