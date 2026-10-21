package record_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

func TestCommitFile_StagesThePathByNameThenCommitsWithSubject(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitFile(t.Context(), mutator(r), dir, ".mint.toml", "chore(release): release-notes path answers for v1.3.0"); err != nil {
		t.Fatalf("CommitFile returned unexpected error: %v", err)
	}

	want := [][]string{
		{"-C", dir, "add", ".mint.toml"},
		{"-C", dir, "commit", "-m", "chore(release): release-notes path answers for v1.3.0"},
	}
	var got [][]string
	for _, inv := range r.Invocations() {
		got = append(got, inv.Args)
	}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("git calls = %q, want %q", got, want)
	}
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
