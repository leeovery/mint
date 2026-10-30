package engine_test

// This file pins what a release that stopped short says it left. The close states
// only mint's own actions (what it committed, reset, wrote, staged, created or kept,
// and that it pushed nothing) and where HEAD stands, never how clean the tree is or
// what a hook did. It says "nothing changed" only while no hook and no write of
// mint's has run, and once a hook has run it no longer says "nothing committed".

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
	noChangelog         = "[release]\nchangelog = false\n"
	// afterAHook is the close of a release stopped once a hook ran, with nothing of
	// mint's own to name: what the hook did is beyond what mint can see.
	afterAHook = "nothing pushed"
	// noClaim is the close of a release stopped once mint began a write of its own, no
	// hook having run, with nothing of it left to name.
	noClaim = "nothing committed · nothing pushed"
)

var (
	hookFails      = runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}
	dirtyAfterHook = ScriptedOut(" M bundle.js\n")
	cleanAfterHook = ScriptedOut("")
)

// seedReadGates scripts a first release's git reads through preflight, up to the
// starting-HEAD capture.
func seedReadGates(f *runner.FakeRunner, root string) {
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),        // rev-list left-right count (ahead only)
		ScriptedOut(""),            // ls-remote --tags (tag free remote)
	)
}

func TestRelease_AStoppedReleaseClosesClaimingOnlyWhatMintDid(t *testing.T) {
	t.Parallel()

	captured := ScriptedOut(startingSHA)                                           // rev-parse HEAD (capture the clean start)
	recorded := []runner.ScriptedCall{ScriptedOut(""), ScriptedOut("")}            // add CHANGELOG.md, commit
	tagged := []runner.ScriptedCall{ScriptedOut(githubRemoteURL), ScriptedOut("")} // remote get-url, tag -a
	tests := []struct {
		name     string
		config   string
		hook     runner.ScriptedCall
		capture  *runner.ScriptedCall
		git      [][]runner.ScriptedCall
		gh       error
		declines bool
		outcome  presenter.Outcome
		detail   string
	}{
		{
			name:    "a preflight hook that fails",
			config:  preflightHookConfig,
			hook:    hookFails,
			outcome: presenter.OutcomeFailed,
			detail:  afterAHook,
		},
		{
			name:    "the starting HEAD unreadable after a preflight hook ran",
			config:  preflightHookConfig,
			capture: &runner.ScriptedCall{Result: runner.Result{ExitCode: 128}, Err: errors.New("exit status 128")},
			outcome: presenter.OutcomeFailed,
			detail:  afterAHook,
		},
		{
			name:    "a pre_tag hook that fails",
			config:  preTagHookConfig,
			hook:    hookFails,
			outcome: presenter.OutcomeFailed,
			detail:  afterAHook,
		},
		{
			name:    "pre_tag's changes that fail to stage",
			config:  preTagHookConfig,
			git:     [][]runner.ScriptedCall{{dirtyAfterHook, gitFailing(stageStderr)}}, // status, add -A
			outcome: presenter.OutcomeFailed,
			detail:  afterAHook,
		},
		{
			name:    "pre_tag's changes staged, then their commit failing",
			config:  preTagHookConfig,
			git:     [][]runner.ScriptedCall{{dirtyAfterHook, ScriptedOut(""), gitFailing(commitStderr)}}, // status, add -A, commit
			outcome: presenter.OutcomeFailed,
			detail:  "pre_tag's changes staged, not committed · nothing pushed",
		},
		{
			name:     "a pre_tag hook that made its own commit, then a declined review",
			config:   preTagHookConfig,
			git:      [][]runner.ScriptedCall{{cleanAfterHook}},
			declines: true,
			outcome:  presenter.OutcomeDeclined,
			detail:   afterAHook,
		},
		{
			name:   "a preflight hook that left a file, then a rejected push and its reset",
			config: preflightHookConfig,
			git: [][]runner.ScriptedCall{recorded, tagged, {
				pushRejected(),  // push --atomic origin HEAD v0.0.1
				ScriptedOut(""), // unwind: tag -d v0.0.1
				ScriptedOut(""), // unwind: reset --hard
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "back where it started · nothing pushed",
		},
		{
			name:   "a tag-only unwind, HEAD never moved",
			config: noChangelog,
			git: [][]runner.ScriptedCall{tagged, {
				pushRejected(),  // push --atomic origin HEAD v0.0.1
				ScriptedOut(""), // unwind: tag -d v0.0.1
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "back where it started · nothing pushed",
		},
		{
			name:   "a tag-only unwind after a pre_tag hook that could have moved HEAD",
			config: preTagHookConfig + noChangelog,
			git: [][]runner.ScriptedCall{{cleanAfterHook}, tagged, {
				pushRejected(),  // push --atomic origin HEAD v0.0.1
				ScriptedOut(""), // unwind: tag -d v0.0.1
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  afterAHook,
		},
		{
			name:    "the publish gate failing with nothing recorded, no hook having run",
			config:  noChangelog,
			git:     [][]runner.ScriptedCall{{ScriptedOut(githubRemoteURL)}}, // remote get-url
			gh:      errors.New("gh: not authenticated"),
			outcome: presenter.OutcomeFailed,
			detail:  noClaim,
		},
		{
			name:    "the record's writes with no commit to reset",
			git:     [][]runner.ScriptedCall{{ScriptedOut(""), gitFailing(commitStderr)}}, // add CHANGELOG.md, commit
			outcome: presenter.OutcomeFailed,
			detail:  "CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name:    "the record's writes, a version file among them, with no commit to reset",
			config:  plainVersionFile,
			git:     [][]runner.ScriptedCall{{ScriptedOut(""), gitFailing(commitStderr)}}, // add VERSION CHANGELOG.md, commit
			outcome: presenter.OutcomeFailed,
			detail:  "VERSION + CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name:   "files the record created, never staged, that the reset leaves untracked",
			config: preTagHookConfig + plainVersionFile,
			git: [][]runner.ScriptedCall{{
				dirtyAfterHook, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				gitFailing(stageStderr), // add VERSION CHANGELOG.md
				ScriptedOut(""),         // unwind: reset --hard
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "back where it started · VERSION + CHANGELOG.md left untracked · nothing pushed",
		},
		{
			name:   "files the record created and staged, which the reset removes",
			config: preTagHookConfig + plainVersionFile,
			git: [][]runner.ScriptedCall{{
				dirtyAfterHook, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				ScriptedOut(""), gitFailing(commitStderr), // add VERSION CHANGELOG.md, commit
				ScriptedOut(""), // unwind: reset --hard
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "back where it started · nothing pushed",
		},
		{
			// The files were never staged, so no commit holds them, failed reset or not:
			// they are untracked all the same.
			name:   "files the record created, never staged, then a reset that fails",
			config: preTagHookConfig + plainVersionFile,
			git: [][]runner.ScriptedCall{{
				dirtyAfterHook, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				gitFailing(stageStderr),         // add VERSION CHANGELOG.md
				scriptedMutateFailure(),         // unwind: reset --hard
				ScriptedOut("9f3e2a1c4b\nmain"), // where HEAD stands
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "9f3e2a1 · 1 commit not reset · manual cleanup required · VERSION + CHANGELOG.md left untracked · nothing pushed",
		},
		{
			name:   "an unwind whose reset fails",
			config: preTagHookConfig,
			git: [][]runner.ScriptedCall{{
				dirtyAfterHook, ScriptedOut(""), ScriptedOut(""), // status, add -A, artifact commit
				ScriptedOut(""), gitFailing(commitStderr), // add CHANGELOG.md, commit
				scriptedMutateFailure(),         // unwind: reset --hard
				ScriptedOut("9f3e2a1c4b\nmain"), // where HEAD stands
			}},
			outcome: presenter.OutcomeUnwound,
			detail:  "9f3e2a1 · 1 commit not reset · manual cleanup required · nothing pushed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tt.config)
			f := runner.NewFakeRunner()
			seedReadGates(f, root)
			capture := captured
			if tt.capture != nil {
				capture = *tt.capture
			}
			f.SeedSequence("git", capture)
			for _, calls := range tt.git {
				f.SeedSequence("git", calls...)
			}
			f.SeedSequence("sh", tt.hook)
			f.Seed("gh", runner.Result{}, tt.gh)
			rec := &presentertest.RecordingPresenter{}
			if tt.declines {
				rec.PromptResult = stopAtTheNotes
			}

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

func TestRelease_AnswersFailingToRecordCloseClaimingNothingCommitted(t *testing.T) {
	t.Parallel()

	for _, failure := range []answersFailure{writeFails, stageFails, commitFails} {
		t.Run(failure.name, func(t *testing.T) {
			t.Parallel()

			run := recordAnswersFailing(t, ptr(answeredConfig), false, failure)

			assertAbortedBeforeTheNotes(t, run)
			assertCloses(t, run.rec, presenter.OutcomeFailed, noClaim)
		})
	}
}
