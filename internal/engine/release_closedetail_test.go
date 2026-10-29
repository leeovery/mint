package engine_test

// This file pins what a release that stopped short says it left: the close claims only
// what mint did. It says "nothing changed" only while nothing that could write has
// run; once a hook has run, or mint has written or staged anything, it names what
// mint left or makes no claim about the tree, and it always pushed nothing.

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// assertCloses fails unless the run's one close stopped with outcome, its end
// station's detail detail.
func assertCloses(t *testing.T, rec *presentertest.RecordingPresenter, outcome presenter.Outcome, detail string) {
	t.Helper()
	if fin := closingEvent(t, rec); fin.Outcome != outcome || fin.End.Detail != detail {
		t.Errorf("close = %v %q, want %v %q", fin.Outcome, fin.End.Detail, outcome, detail)
	}
}

const (
	preflightHookConfig = "[release.hooks]\npreflight = \"check.sh\"\n"
	plainVersionFile    = "[release]\nversion_file = \"VERSION\"\n"
	// noClaim is the close of a release stopped once something that could write ran,
	// having left nothing mint can name.
	noClaim = "nothing committed · nothing pushed"
)

var (
	hookFails     = runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}
	dirtyAfterTag = ScriptedOut(" M bundle.js\n")
)

func TestRelease_AStoppedReleaseClosesClaimingOnlyWhatMintDid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		hook    runner.ScriptedCall
		git     []runner.ScriptedCall
		outcome presenter.Outcome
		detail  string
	}{
		{
			name:    "a preflight hook that fails, its writes unknown",
			config:  preflightHookConfig,
			hook:    hookFails,
			outcome: presenter.OutcomeFailed,
			detail:  noClaim,
		},
		{
			name:    "a pre_tag hook that fails, its writes unknown",
			config:  preTagHookConfig,
			hook:    hookFails,
			outcome: presenter.OutcomeFailed,
			detail:  noClaim,
		},
		{
			name:    "pre_tag's changes that fail to stage",
			config:  preTagHookConfig,
			git:     []runner.ScriptedCall{dirtyAfterTag, gitFailing(stageStderr)}, // status, add -A
			outcome: presenter.OutcomeFailed,
			detail:  noClaim,
		},
		{
			name:    "pre_tag's changes staged, then their commit failing",
			config:  preTagHookConfig,
			git:     []runner.ScriptedCall{dirtyAfterTag, ScriptedOut(""), gitFailing(commitStderr)}, // status, add -A, commit
			outcome: presenter.OutcomeFailed,
			detail:  "pre_tag's changes staged, not committed · nothing pushed",
		},
		{
			name:    "the record's writes with no commit to reset",
			git:     []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)}, // add CHANGELOG.md, commit
			outcome: presenter.OutcomeFailed,
			detail:  "CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name:    "the record's writes, a version file among them, with no commit to reset",
			config:  plainVersionFile,
			git:     []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)}, // add VERSION CHANGELOG.md, commit
			outcome: presenter.OutcomeFailed,
			detail:  "VERSION + CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name:   "files the record created, never staged, that the reset leaves untracked",
			config: preTagHookConfig + plainVersionFile,
			git: []runner.ScriptedCall{
				dirtyAfterTag, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				gitFailing(stageStderr), // add VERSION CHANGELOG.md
				ScriptedOut(""),         // unwind: reset --hard
			},
			outcome: presenter.OutcomeUnwound,
			detail:  "VERSION + CHANGELOG.md left untracked · nothing pushed",
		},
		{
			name:   "files the record created and staged, which the reset removes",
			config: preTagHookConfig + plainVersionFile,
			git: []runner.ScriptedCall{
				dirtyAfterTag, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				ScriptedOut(""), gitFailing(commitStderr), // add VERSION CHANGELOG.md, commit
				ScriptedOut(""), // unwind: reset --hard
			},
			outcome: presenter.OutcomeUnwound,
			detail:  "back where it started · repo clean · nothing pushed",
		},
		{
			name:   "an unwind whose reset fails",
			config: preTagHookConfig,
			git: []runner.ScriptedCall{
				dirtyAfterTag, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				ScriptedOut(""), gitFailing(commitStderr), // add CHANGELOG.md, commit
				scriptedMutateFailure(), // unwind: reset --hard
			},
			outcome: presenter.OutcomeUnwound,
			detail:  "manual cleanup required · nothing pushed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			seedHappyGitThroughGate(f, root, "main", "v0.0.1")
			f.SeedSequence("git", tt.git...)
			f.SeedSequence("sh", tt.hook)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

			assertAbortNonZero(t, err)
			assertCloses(t, rec, tt.outcome, tt.detail)
		})
	}
}

func TestRelease_ARunStoppedBeforeAnythingCouldWriteClosesHavingChangedNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, preTagHookConfig)
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	for _, inv := range f.Invocations() {
		if inv.Name == "sh" {
			t.Errorf("the hook ran though the version was declined")
		}
	}
	assertCloses(t, rec, presenter.OutcomeDeclined, "nothing changed")
}
