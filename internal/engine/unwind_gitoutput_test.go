package engine_test

import (
	"errors"
	"reflect"
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
	f.SeedSequence("git", ScriptedOut(""), locked, locked, locked) // tag -d, then every reset attempt
	rec := &presentertest.RecordingPresenter{}
	mut := git.NewMutator(f, git.WithRetryBudget(3), git.WithBackoff(func(int) {}))

	err := engine.Unwind(t.Context(), newDepsWithMutator(rec, f, mut), startState(startingSHA, "v0.0.1"), engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

	assertAbortNonZero(t, err)
	onlyWarning(t, rec, unwindIncomplete("reset HEAD back to "+startingSHA+" (`git reset --hard "+startingSHA+"`)", lockStderr))
	if got := countCmd(f, "git", "reset", "--hard", startingSHA); got != 3 {
		t.Errorf("reset attempts = %d, want 3 (the Mutator's whole budget)", got)
	}
	if got, want := unwoundText(t, rec), "deleted tag v0.0.1; manual cleanup required"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
}

func TestUnwind_FailedGitStep_ShowsItsStderrOrElseTheErrorText(t *testing.T) {
	t.Parallel()

	const (
		hard    = "reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA + "`)"
		tag     = "delete the leftover tag v0.0.1 (`git tag -d v0.0.1`)"
		soft    = "`git reset --soft " + startingSHA + "`"
		restore = "`git restore --source=" + startingSHA + " --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'`"
		keeping = ", leaving NEXT_RELEASE.md as it is ("
		// restoreCmd is restore as the command alone on next's line.
		restoreCmd = "git restore --source=" + startingSHA + " --staged --worktree -- ':(top,literal,exclude)NEXT_RELEASE.md'"
	)
	steps := []struct {
		name    string
		keepOut git.Exclusion
		calls   func(failed runner.ScriptedCall) []runner.ScriptedCall
		action  string
		move    presenter.NextMove
		// summary is the Unwound line, naming only the steps that succeeded: the tag
		// delete when the reset fails after it, the reset when the delete fails, and
		// both when the reset moved HEAD back and only the restore after it failed.
		summary string
	}{
		{
			name: "the hard reset",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), failed}
			},
			action:  hard,
			move:    presenter.NextMove{Lead: "Reset HEAD back to " + startingSHA + ":", Command: "git reset --hard " + startingSHA},
			summary: "deleted tag v0.0.1; manual cleanup required",
		},
		{
			name: "the tag delete",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{failed, ScriptedOut("")}
			},
			action:  tag,
			move:    presenter.NextMove{Lead: "Delete the leftover tag v0.0.1:", Command: "git tag -d v0.0.1"},
			summary: "reset 2 commits; manual cleanup required",
		},
		{
			name:    "the keep-out soft reset",
			keepOut: "NEXT_RELEASE.md",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), failed}
			},
			action: "reset HEAD back to " + startingSHA + keeping + soft + " then " + restore + ")",
			move: presenter.NextMove{
				Lead:    "Reset HEAD back to " + startingSHA + ", leaving NEXT_RELEASE.md as it is:",
				Command: "git reset --soft " + startingSHA + " && " + restoreCmd,
			},
			summary: "deleted tag v0.0.1; manual cleanup required",
		},
		{
			name:    "the keep-out restore",
			keepOut: "NEXT_RELEASE.md",
			calls: func(failed runner.ScriptedCall) []runner.ScriptedCall {
				return []runner.ScriptedCall{ScriptedOut(""), ScriptedOut(""), failed}
			},
			action: "restore the index and work tree to " + startingSHA + keeping + restore + ")",
			move: presenter.NextMove{
				Lead:    "Restore the index and work tree to " + startingSHA + ", leaving NEXT_RELEASE.md as it is:",
				Command: restoreCmd,
			},
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
				start := startState(startingSHA, "v0.0.1")
				start.KeepOut = step.keepOut

				err := engine.Unwind(t.Context(), newDeps(rec, f), start, engine.MadeState{Commits: 2, TagCreated: true}, errUnwindReason)

				assertAbortNonZero(t, err)
				onlyWarning(t, rec, unwindIncomplete(step.action, failure.output))
				assertManualSteps(t, rec, step.move)
				if got := unwoundText(t, rec); got != step.summary {
					t.Errorf("Unwound.Text() = %q, want %q", got, step.summary)
				}
			})
		}
	}
}

func TestRegenerateWrite_FailedChangelogReset_ShowsItsStderrOrElseTheErrorText(t *testing.T) {
	t.Parallel()

	const leftover = " to drop the leftover changelog commit"
	pushRejected := runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}
	tests := []struct {
		name    string
		keepOut git.Exclusion
		failed  runner.ScriptedCall
		action  string
		output  string
	}{
		{
			name:   "the hard reset with stderr",
			failed: gitFailing(recoveryStderr),
			action: "reset HEAD back to startHEAD (`git reset --hard startHEAD`)" + leftover,
			output: recoveryStderr,
		},
		{
			name:   "the hard reset without stderr",
			failed: scriptedMutateFailure(),
			action: "reset HEAD back to startHEAD (`git reset --hard startHEAD`)" + leftover,
			output: noStderrOutput,
		},
		{
			name:    "the keep-out soft reset with stderr",
			keepOut: "notes.md",
			failed:  gitFailing(recoveryStderr),
			action: "reset HEAD back to startHEAD, leaving notes.md as it is " +
				"(`git reset --soft startHEAD` then `git restore --source=startHEAD --staged --worktree -- ':(top,literal,exclude)notes.md'`)" + leftover,
			output: recoveryStderr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			seedChangelog(t, dir, regenNotesStaleChangelog)
			f := runner.NewFakeRunner()
			f.SeedSequence("git", regenChangelogWrite()...)
			if tt.keepOut == "" {
				f.SeedSequence("git", ScriptedOut("")) // -C root commit -m
			} else {
				f.SeedSequence("git", keepOutCommit("", stagedChangelog)...)
			}
			f.SeedSequence("git", pushRejected, tt.failed)
			rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

			req := freshWriteReq(engine.RegenerateTargetChangelog)
			req.KeepOut = tt.keepOut
			err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, req)

			assertAbortNonZero(t, err)
			onlyWarning(t, rec, unwindIncomplete(tt.action, tt.output))
		})
	}
}
