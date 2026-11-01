package engine_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	// noStderrOutput is the error text scriptedMutateFailure carries, which a failed step
	// that wrote no stderr shows instead.
	noStderrOutput = "fatal: recovery git command failed"
	recoveryStderr = "fatal: could not write the index\n"
	lockStderr     = "fatal: Unable to create '/repo/.git/index.lock': File exists.\n"
)

// unwindIncomplete is the manual-cleanup warning naming action, with output beneath it.
func unwindIncomplete(action, output string) presenter.Warning {
	return presenter.Warning{Label: "unwind incomplete", Message: "automatic recovery failed; manually " + action, Output: output, Key: "unwind", Failed: true, Row: "automatic recovery failed"}
}

// onlyWarning fails the test unless rec holds exactly the one warning want.
func onlyWarning(t *testing.T, rec *presentertest.RecordingPresenter, want presenter.Warning) {
	t.Helper()
	var warns []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	if len(warns) != 1 || warns[0] != want {
		t.Errorf("warnings = %#v, want only %#v", warns, want)
	}
}

// assertManualSteps fails unless rec's only next moves are moves, the unwind's manual
// steps, Quiet: plain's warning line already names them.
func assertManualSteps(t *testing.T, rec *presentertest.RecordingPresenter, moves ...presenter.NextMove) {
	t.Helper()
	want := []presenter.NextMoves{{Stage: "unwind", Quiet: true, Moves: moves}}
	if got := nextMovesEvents(rec); !reflect.DeepEqual(got, want) {
		t.Errorf("next moves = %+v, want %+v", got, want)
	}
}

func TestUnwind_ResetLockedPastTheRetries_ShowsGitsLockStderrBeneathTheWarning(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	locked := gitFailing(lockStderr)
	f.SeedSequence("git", ScriptedOut(""), nothingStaged, locked, locked, locked) // tag -d, the staged read, then every reset attempt
	rec := &presentertest.RecordingPresenter{}
	mut := git.NewMutator(f, git.WithRetryBudget(3), git.WithBackoff(func(int) {}))

	err := engine.Unwind(t.Context(), newDepsWithMutator(rec, f, mut), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

	assertAbortNonZero(t, err)
	onlyWarning(t, rec, unwindIncomplete(keepSteps, lockStderr))
	if got := countCmd(f, "git", "reset", "--keep", startingSHA); got != 3 {
		t.Errorf("reset attempts = %d, want 3 (the Mutator's whole budget)", got)
	}
	if got, want := unwoundText(t, rec), "deleted tag v0.0.1; manual cleanup required"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
}

// git can't stash a file marked intent-to-add, so a reset refused with one in the index
// leaves steps that unmark it before the stash and mark it again after the pop.
func TestUnwind_ResetRefusedWithAFileMarkedIntentToAdd_UnmarksItBeforeTheStashAndMarksItAfter(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git", ScriptedOut(""), indexHolds(".A", "next.go"), gitFailing(keepRefusalStderr)) // tag -d, the index read, the reset
	rec := &presentertest.RecordingPresenter{}

	err := engine.Unwind(t.Context(), newDeps(rec, f), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

	assertAbortNonZero(t, err)
	const unmark = "git rm --cached -q -- ':(top,literal)next.go'"
	onlyWarning(t, rec, unwindIncomplete(
		"unmark next.go, which git can't stash (`"+unmark+"`), then "+keepSteps+
			", then mark next.go as intended to add again (`"+remarkIntent+"`)",
		keepRefusalStderr))
	assertManualSteps(t, rec, slices.Concat(
		[]presenter.NextMove{{Lead: "Unmark next.go, which git can't stash:", Command: unmark}},
		keepMoves,
		[]presenter.NextMove{{Lead: "Mark next.go as intended to add again:", Command: remarkIntent}},
	)...)
}

// restageStep is the manual step a failed restage of NEXT_RELEASE.md leaves.
const restageStep = "git restore --source=" + stagedTree + " --staged -- ':(top,literal)NEXT_RELEASE.md'"

// restageDeletion and remarkIntent are the manual steps a failed restage of a staged
// deletion of old.go, and of next.go's intent-to-add mark, leave.
const (
	restageDeletion = "git rm --cached -q --ignore-unmatch -- ':(top,literal)old.go'"
	remarkIntent    = "git add --intent-to-add -- ':(top,literal)next.go'"
)

func TestUnwind_FailedGitStep_ShowsItsStderrOrElseTheErrorText(t *testing.T) {
	t.Parallel()

	const tag = "delete the leftover tag v0.0.1 (`git tag -d v0.0.1`)"
	stagedNotes := indexHolds("M.", "NEXT_RELEASE.md")
	snapshot := ScriptedOut(stagedTree + "\n")
	steps := []struct {
		name   string
		calls  func(failed runner.ScriptedCall) []runner.ScriptedCall
		action string
		moves  []presenter.NextMove
		// summary is the Unwound line, naming only the steps that succeeded: the tag
		// delete when the reset fails after it, the reset when the delete fails, and
		// both when the reset moved HEAD back and only the restage after it failed.
		summary string
	}{
		{
			name: "the reset",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), nothingStaged, failed}
			},
			action:  keepSteps,
			moves:   keepMoves,
			summary: "deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the tag delete",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{failed, nothingStaged, ScriptedOut("")}
			},
			action:  tag,
			moves:   []presenter.NextMove{{Lead: "Delete the leftover tag v0.0.1:", Command: "git tag -d v0.0.1"}},
			summary: "reset 2 commits; manual cleanup required",
		},
		{
			name: "the read of what the index holds",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), failed}
			},
			action:  keepSteps,
			moves:   keepMoves,
			summary: "deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the index written as a tree",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), stagedNotes, failed}
			},
			action:  keepSteps,
			moves:   keepMoves,
			summary: "deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the restage",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), stagedNotes, snapshot, ScriptedOut(""), failed}
			},
			action:  "stage your changes to NEXT_RELEASE.md again (`" + restageStep + "`)",
			moves:   []presenter.NextMove{{Lead: "Stage your changes to NEXT_RELEASE.md again:", Command: restageStep}},
			summary: "reset 2 commits and deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the staged deletion",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), indexHolds("D.", "old.go"), ScriptedOut(""), failed}
			},
			action:  "stage your deletion of old.go again (`" + restageDeletion + "`)",
			moves:   []presenter.NextMove{{Lead: "Stage your deletion of old.go again:", Command: restageDeletion}},
			summary: "reset 2 commits and deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the intent-to-add mark",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), indexHolds(".A", "next.go"), ScriptedOut(""), failed}
			},
			action:  "mark next.go as intended to add again (`" + remarkIntent + "`)",
			moves:   []presenter.NextMove{{Lead: "Mark next.go as intended to add again:", Command: remarkIntent}},
			summary: "reset 2 commits and deleted tag v0.0.1; manual cleanup required",
		},
	}
	failures := []struct {
		name   string
		failed runner.ScriptedCall
		output string
	}{
		{name: "with stderr", failed: gitFailing(recoveryStderr), output: recoveryStderr},
		{name: "without stderr", failed: scriptedMutateFailure(), output: noStderrOutput},
	}

	for _, step := range steps {
		for _, failure := range failures {
			t.Run(step.name+" "+failure.name, func(t *testing.T) {
				t.Parallel()

				f := runner.NewFakeRunner()
				f.SeedSequence("git", step.calls(failure.failed)...)
				rec := &presentertest.RecordingPresenter{}

				err := engine.Unwind(t.Context(), newDeps(rec, f), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

				assertAbortNonZero(t, err)
				onlyWarning(t, rec, unwindIncomplete(step.action, failure.output))
				assertManualSteps(t, rec, step.moves...)
				if got := unwoundText(t, rec); got != step.summary {
					t.Errorf("Unwound.Text() = %q, want %q", got, step.summary)
				}
			})
		}
	}
}

func TestRegenerateWrite_FailedChangelogReset_ShowsItsStderrOrElseTheErrorText(t *testing.T) {
	t.Parallel()

	reset := keepStepsTo("startHEAD", "to drop the leftover changelog commit")
	pushRejected := runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}
	tests := []struct {
		name string
		// recovery are the reset's git steps, the last of them failing.
		recovery []runner.ScriptedCall
		action   string
		output   string
		// undone is what the rail's unwind names undone: the reset, once HEAD moved.
		undone string
	}{
		{
			name:     "the reset with stderr",
			recovery: []runner.ScriptedCall{nothingStaged, gitFailing(recoveryStderr)},
			action:   reset,
			output:   recoveryStderr,
		},
		{
			name:     "the reset without stderr",
			recovery: []runner.ScriptedCall{nothingStaged, scriptedMutateFailure()},
			action:   reset,
			output:   noStderrOutput,
		},
		{
			name:     "a reset git refuses over a change of the user's",
			recovery: []runner.ScriptedCall{nothingStaged, gitFailing(keepRefusalStderr)},
			action:   reset,
			output:   keepRefusalStderr,
		},
		{
			name:     "the read of what the index holds",
			recovery: []runner.ScriptedCall{gitFailing(recoveryStderr)},
			action:   reset,
			output:   recoveryStderr,
		},
		{
			// HEAD has left the commit, so only the notes file's staged state is left to
			// put back.
			name:     "the restage after the reset went through",
			recovery: []runner.ScriptedCall{indexHolds("M.", "notes.md"), ScriptedOut(stagedTree + "\n"), ScriptedOut(""), gitFailing(recoveryStderr)},
			action:   "stage your changes to notes.md again (`git restore --source=" + stagedTree + " --staged -- ':(top,literal)notes.md'`)",
			output:   recoveryStderr,
			undone:   "reset the changelog commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			seedChangelog(t, dir, regenNotesStaleChangelog)
			f := runner.NewFakeRunner()
			f.SeedSequence("git", regenChangelogWrite()...)
			f.SeedSequence("git", ScriptedOut(""), committedPaths) // -C root commit of CHANGELOG.md, the read of the paths it changed
			f.SeedSequence("git", append([]runner.ScriptedCall{pushRejected}, tt.recovery...)...)
			rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

			err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

			assertAbortNonZero(t, err)
			onlyWarning(t, rec, unwindIncomplete(tt.action, tt.output))
			if got := unwoundText(t, rec); got != tt.undone {
				t.Errorf("Unwound = %q, want %q", got, tt.undone)
			}
		})
	}
}
