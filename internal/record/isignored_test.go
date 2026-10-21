package record_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

func TestIsIgnored_AsksGitCheckIgnoreAboutThePathInDir(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		git  runner.ScriptedCall
		want bool
	}{
		{name: "exit 0 is ignored", git: runner.ScriptedCall{}, want: true},
		{name: "exit 1 is not ignored", git: runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.SeedSequence("git", tc.git)

			got, err := record.IsIgnored(t.Context(), mutator(r), "/repo/root", ".mint.toml")
			if err != nil {
				t.Fatalf("IsIgnored returned unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsIgnored = %v, want %v", got, tc.want)
			}
			invs := r.Invocations()
			if want := []string{"-C", "/repo/root", "check-ignore", "-q", "--", ".mint.toml"}; len(invs) != 1 || invs[0].Name != "git" || !slices.Equal(invs[0].Args, want) {
				t.Errorf("calls = %+v, want git %q alone", invs, want)
			}
		})
	}
}

func TestIsIgnored_AFailedProbe_CarriesGitsStderr(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: not a git repository (or any of the parent directories): .git\n"
	probeErr := errors.New("exit status 128")
	r := runner.NewFakeRunner()
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: probeErr})

	_, err := record.IsIgnored(t.Context(), mutator(r), "/repo/root", ".mint.toml")

	var gitErr *record.GitError
	if !errors.As(err, &gitErr) || gitErr.Stderr != stderr {
		t.Fatalf("IsIgnored error = %v, want a *GitError carrying %q", err, stderr)
	}
	if !errors.Is(err, probeErr) {
		t.Errorf("IsIgnored error = %v, want it wrapping the runner error", err)
	}
}
