package record_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

// The commit names the path alone, so a change the user staged elsewhere stays staged
// and out of it.
func TestCommitFile_StagesThePathByNameThenCommitsItAloneWithSubject(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitFile(t.Context(), mutator(r), dir, ".mint.toml", "chore(release): release-notes path answers for v1.3.0"); err != nil {
		t.Fatalf("CommitFile returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 3 || !slices.Equal(invs[0].Args, []string{"-C", dir, "add", ".mint.toml"}) || invs[0].Stdin != "" {
		t.Fatalf("git calls = %v, want the stage of .mint.toml, its commit, then the read of what it changed", invs)
	}
	assertPathsCommit(t, invs[1], dir, "chore(release): release-notes path answers for v1.3.0", ".mint.toml")
}

func TestCommitFile_StageFails_MakesNoCommit(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	stageErr := errors.New("exit status 128")
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{Stderr: "fatal: pathspec"}, Err: stageErr})

	err := record.CommitFile(t.Context(), mutator(r), "/repo/root", ".mint.toml", "subject")
	if !errors.Is(err, stageErr) {
		t.Fatalf("CommitFile error = %v, want it wrapping the stage failure", err)
	}
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("git calls = %d, want the stage alone", n)
	}
}

func TestCommitFile_AFailedStep_CarriesGitsStderr(t *testing.T) {
	t.Parallel()

	const stderr = "error: gpg failed to sign the data\nfatal: failed to write commit object\n"
	for _, tc := range []struct {
		name string
		git  []runner.ScriptedCall
	}{
		{name: "staging", git: []runner.ScriptedCall{{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: errors.New("exit status 128")}}},
		{name: "committing", git: []runner.ScriptedCall{{}, {Result: runner.Result{ExitCode: 1, Stderr: stderr}, Err: errors.New("exit status 1")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.SeedSequence("git", tc.git...)

			err := record.CommitFile(t.Context(), mutator(r), "/repo/root", ".mint.toml", "subject")
			var gitErr *record.GitError
			if !errors.As(err, &gitErr) {
				t.Fatalf("CommitFile error = %v, want a *record.GitError", err)
			}
			if gitErr.Stderr != stderr {
				t.Errorf("Stderr = %q, want %q", gitErr.Stderr, stderr)
			}
		})
	}
}
