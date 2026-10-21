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
