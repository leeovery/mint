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

// TestRelease_AFailedBookkeepingCommitShowsGitsStderrVerbatimBeneathItsFailure pins
// that a commit git refuses, here for want of a signing key, shows git's stderr
// verbatim in StageFailure.Output, as the push and the tag do.
func TestRelease_AFailedBookkeepingCommitShowsGitsStderrVerbatimBeneathItsFailure(t *testing.T) {
	t.Parallel()

	const unsigned = "error: gpg failed to sign the data\nfatal: failed to write commit object\n"
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		ScriptedOut(""),      // -C root add CHANGELOG.md
		gitFailing(unsigned), // -C root commit -m
		ScriptedOut(""),      // unwind: reset --hard startingSHA
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if failure := onlyStageFailureEvent(t, rec); failure.Output != unsigned {
		t.Errorf("failure = %q with output %q, want git's stderr %q verbatim", failure.Message, failure.Output, unsigned)
	}
}
