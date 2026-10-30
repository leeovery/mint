package runner_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mint/internal/runner"
)

func TestFakeRunner_SeedCall_AnswersItsArgvAheadOfTheSequenceAndConsumesNoEntry(t *testing.T) {
	t.Parallel()
	f := runner.NewFakeRunner()
	f.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{Stdout: "first"}}, runner.ScriptedCall{Result: runner.Result{Stdout: "second"}})
	f.SeedCall("git", []string{"remote", "get-url", "origin"}, runner.ScriptedCall{Result: runner.Result{Stdout: "url"}})

	var got []string
	for _, args := range [][]string{{"status"}, {"remote", "get-url", "origin"}, {"status"}, {"remote", "get-url", "origin"}} {
		res, err := f.Run(context.Background(), "git", args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		got = append(got, res.Stdout)
	}
	if want := []string{"first", "url", "second", "url"}; !slices.Equal(got, want) {
		t.Errorf("outcomes = %q, want %q", got, want)
	}
}

func TestFakeRunner_SeedFallback_AnswersOnlyWhatNothingElseScripts(t *testing.T) {
	t.Parallel()
	f := runner.NewFakeRunner()
	f.SeedSequence("gh", runner.ScriptedCall{Result: runner.Result{Stdout: "scripted"}})
	f.SeedFallback("gh", runner.Result{Stdout: "fallback"}, nil)

	first, _ := f.Run(context.Background(), "gh", "auth", "status")
	second, _ := f.Run(context.Background(), "gh", "auth", "status")
	if first.Stdout != "scripted" || second.Stdout != "fallback" {
		t.Errorf("outcomes = %q, %q, want the sequence's, then the fallback's", first.Stdout, second.Stdout)
	}

	missing := runner.NewFakeRunner()
	missing.SeedNotFound("gh")
	missing.SeedFallback("gh", runner.Result{}, nil)
	if _, err := missing.Run(context.Background(), "gh", "auth", "status"); !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("err = %v, want SeedNotFound to win over the fallback seeded after it", err)
	}
}
