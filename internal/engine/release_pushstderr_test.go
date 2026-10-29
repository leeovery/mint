package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// TestRelease_ARejectedPushShowsGitsStderrVerbatimBeneathItsFailure pins that the
// remote's reason for rejecting the push travels verbatim in StageFailure.Output, as
// every git step's stderr does, beneath a message that names the push.
func TestRelease_ARejectedPushShowsGitsStderrVerbatimBeneathItsFailure(t *testing.T) {
	t.Parallel()

	const rejected = " ! [rejected]        main -> main (fetch first)\nerror: failed to push some refs to 'https://github.com/acme/pigeon'\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		gitFailing(rejected), // push --atomic origin HEAD v0.0.1
		ScriptedOut(""),      // unwind: tag -d v0.0.1
		ScriptedOut(""),      // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	failure := onlyStageFailureEvent(t, rec)
	if want := `pushing tag "v0.0.1" atomically: push rejected: exit status 1`; failure.Message != want || failure.Output != rejected {
		t.Errorf("failure = %q with output %q, want %q with git's stderr %q verbatim", failure.Message, failure.Output, want, rejected)
	}
}
