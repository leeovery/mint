package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// unsignedStderr is git refusing a commit for want of a signing key.
const unsignedStderr = "error: gpg failed to sign the data\nfatal: failed to write commit object\n"

// TestRegenerateWrite_AFailedChangelogCommitShowsGitsStderrVerbatimBeneathItsFailure
// pins that regenerate's changelog commit, refused by git, shows git's stderr verbatim
// in StageFailure.Output, as release's commits do.
func TestRegenerateWrite_AFailedChangelogCommitShowsGitsStderrVerbatimBeneathItsFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, regenNotesStaleChangelog)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", regenChangelogWrite()...)
	f.SeedSequence("git", gitFailing(unsignedStderr)) // -C root commit -m
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	if failure := onlyStageFailureEvent(t, rec); failure.Output != unsignedStderr {
		t.Errorf("failure = %q with output %q, want git's stderr %q verbatim", failure.Message, failure.Output, unsignedStderr)
	}
}

// TestRegenerateAllValidated_AFailedRebuildCommitShowsGitsStderrVerbatimBeneathItsFailure
// pins the same for --all's one commit of the rebuilt changelog.
func TestRegenerateAllValidated_AFailedRebuildCommitShowsGitsStderrVerbatimBeneathItsFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2024-01-01\n\nstale v1\n")
	f := runner.NewFakeRunner()
	f.SeedSequence("git", append(preflightCommitPushBucket(),
		ScriptedOut("startHEAD"),   // rev-parse HEAD (captured start)
		ScriptedOut(batchV1Date),   // for-each-ref creatordate:short v1
		ScriptedOut(batchV2Date),   // for-each-ref creatordate:short v2
		ScriptedOut(batchV3Date),   // for-each-ref creatordate:short v3
		ScriptedOut(""),            // add CHANGELOG.md
		gitFailing(unsignedStderr), // commit
	)...)
	rec := &presentertest.RecordingPresenter{}

	err := engine.RegenerateAllValidated(t.Context(), batchDeps(rec, f), newFakePublisher(), dir,
		freshChangelogBatchReq(threeVersions(), engine.RegenerateTargetChangelog), true)

	assertAbortNonZero(t, err)
	if failure := onlyStageFailureEvent(t, rec); failure.Output != unsignedStderr {
		t.Errorf("failure = %q with output %q, want git's stderr %q verbatim", failure.Message, failure.Output, unsignedStderr)
	}
}
