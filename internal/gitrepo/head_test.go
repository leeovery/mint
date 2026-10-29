package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/gitrepo"
	"mint/internal/runner"
)

func TestResolveHead_ReadsTheShortShaAndBranchInOneCall(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "4f9c2e1a0b7d3c5e6f8091a2b3c4d5e6f7a8b9c0\nmain\n"}, nil)

	got := gitrepo.ResolveHead(t.Context(), r)

	if want := (gitrepo.Head{Branch: "main", ShortSHA: "4f9c2e1"}); got != want {
		t.Errorf("Head = %+v, want %+v", got, want)
	}
	invs := r.Invocations()
	if len(invs) != 1 || invs[0].Name != "git" || !slices.Equal(invs[0].Args, []string{"rev-parse", "HEAD", "--abbrev-ref", "HEAD"}) {
		t.Errorf("invocations = %+v, want the one git rev-parse HEAD --abbrev-ref HEAD", invs)
	}
}

func TestResolveHead_AHeadGitCannotReadIsTheZeroHead(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{ExitCode: 128, Stderr: "fatal: ambiguous argument 'HEAD'\n"}, errors.New("exit status 128"))

	if got := gitrepo.ResolveHead(t.Context(), r); got != (gitrepo.Head{}) {
		t.Errorf("Head = %+v, want the zero Head for an unreadable HEAD", got)
	}
}

func TestResolveShortHEAD_IsTheSameLengthAsResolveHeads(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		stdout string
		want   string
	}{
		{name: "git's usual seven", stdout: "a3f9c21\n", want: "a3f9c21"},
		{name: "a longer abbreviation is cut to seven", stdout: "a3f9c21b0\n", want: "a3f9c21"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{Stdout: tc.stdout}, nil)

			if got := gitrepo.ResolveShortHEAD(t.Context(), r); got != tc.want {
				t.Errorf("ResolveShortHEAD = %q, want %q", got, tc.want)
			}
			invs := r.Invocations()
			if len(invs) != 1 || !slices.Equal(invs[0].Args, []string{"rev-parse", "--short", "HEAD"}) {
				t.Errorf("invocations = %+v, want the one git rev-parse --short HEAD", invs)
			}
		})
	}
}

func TestResolveShortHEAD_AHeadGitCannotReadIsEmpty(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{ExitCode: 128}, errors.New("exit status 128"))

	if got := gitrepo.ResolveShortHEAD(t.Context(), r); got != "" {
		t.Errorf("ResolveShortHEAD = %q, want empty for an unreadable HEAD", got)
	}
}
