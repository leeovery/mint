package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const designDocAnswer = "docs/design/: design documents\n"

// gitFailing is a git call that exits 1 having written stderr.
func gitFailing(stderr string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{ExitCode: 1, Stderr: stderr}, Err: errors.New("exit status 1")}
}

const (
	stageStderr  = "fatal: Unable to write new index file\n"
	commitStderr = "error: gpg failed to sign the data\nfatal: failed to write commit object\n"
)

// answersFailure is one way recording an Exclude answer fails: an unwritable
// .mint.toml, or git's answers to whether it ignores the file, then to staging and
// committing it.
type answersFailure struct {
	name     string
	readOnly bool
	git      []runner.ScriptedCall
}

var (
	writeFails  = answersFailure{name: "writing .mint.toml", readOnly: true}
	stageFails  = answersFailure{name: "staging .mint.toml", git: []runner.ScriptedCall{configNotIgnored, gitFailing(stageStderr)}}
	commitFails = answersFailure{name: "committing .mint.toml", git: []runner.ScriptedCall{configNotIgnored, ScriptedOut(""), gitFailing(commitStderr)}}
)

// failedAnswers is a run whose Exclude answer to docs/design/ could not be recorded.
type failedAnswers struct {
	root string
	f    *runner.FakeRunner
	rec  *presentertest.RecordingPresenter
	err  error
}

// recordAnswersFailing runs an attended prior-tag release with .mint.toml holding
// config, or absent when config is nil, the operator excluding docs/design/ and
// recording that answer failing as failure does; preTag adds the pre_tag hook's
// artifact commit.
func recordAnswersFailing(t *testing.T, config *string, preTag bool, failure answersFailure) failedAnswers {
	t.Helper()
	return recordAnswersFailingIn(t, t.TempDir(), config, preTag, failure)
}

// recordAnswersFailingIn is recordAnswersFailing run in the given repo root.
func recordAnswersFailingIn(t *testing.T, root string, config *string, preTag bool, failure answersFailure) failedAnswers {
	t.Helper()
	if config != nil {
		writeConfig(t, root, *config)
	}
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	if preTag {
		seedPreTagArtifactCommit(f)
	}
	f.SeedSequence("git", ScriptedOut(exportGoDiff+designDocDiff))
	f.SeedSequence("git", failure.git...)
	f.SeedSequence("git", ScriptedOut(""), ScriptedOut("")) // the unwind
	f.SeedSequence("claude", ScriptedOut(designDocAnswer), ScriptedOut(aiBody))
	f.Seed("gh", runner.Result{}, nil)
	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	if failure.readOnly {
		makeReadOnly(t, root)
	}

	err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions())

	if failure.readOnly {
		if cerr := os.Chmod(root, 0o755); cerr != nil {
			t.Fatalf("making %s writable again: %v", root, cerr)
		}
	}
	return failedAnswers{root: root, f: f, rec: rec, err: err}
}

func ptr(s string) *string { return &s }

// notesFailure returns the notes stage's StageFailed payload.
func notesFailure(t *testing.T, rec *presentertest.RecordingPresenter) presenter.StageFailure {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed && ev.StageFailed.Name == "notes" {
			return ev.StageFailed
		}
	}
	t.Fatalf("no notes StageFailed; kinds = %v", rec.Kinds())
	return presenter.StageFailure{}
}

// assertAbortedBeforeTheNotes fails unless run exited 1 having made no notes call, no
// tag and no push.
func assertAbortedBeforeTheNotes(t *testing.T, run failedAnswers) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(run.err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", run.err)
	}
	if notesCallMade(run.f) {
		t.Errorf("a notes call was made after the answers failed to record")
	}
	assertNoMutation(t, run.f)
}

func unstageConfigArgs(root string) []string {
	return []string{"-C", root, "reset", "-q", startingSHA, "--", config.FileName}
}

func TestRelease_AnswersWriteFailing_ShowsTheWriteErrorBeneathItsLineAndStopsBeforeTheNotes(t *testing.T) {
	t.Parallel()

	run := recordAnswersFailing(t, ptr(answeredConfig), false, writeFails)

	assertAbortedBeforeTheNotes(t, run)
	failure := notesFailure(t, run.rec)
	if failure.Message != "recording the exclusion answers" {
		t.Errorf("✗ line = %q, want %q", failure.Message, "recording the exclusion answers")
	}
	writeErr := strings.TrimPrefix(run.err.Error(), "aborting (exit 1): recording the exclusion answers: ")
	if !strings.HasPrefix(writeErr, "writing .mint.toml: ") || !errors.Is(run.err, os.ErrPermission) {
		t.Fatalf("err = %v, want the permission-denied write", run.err)
	}
	if failure.Output != writeErr {
		t.Errorf("beneath the ✗ line = %q, want the write error %q", failure.Output, writeErr)
	}
	if got := readConfigBytes(t, run.root); got != answeredConfig {
		t.Errorf(".mint.toml =\n%s\nwant it unchanged", got)
	}
}

func TestRelease_AnswersStageOrCommitFailing_ShowsGitsOwnStderrBeneathItsLineAndStopsBeforeTheNotes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		failure answersFailure
		stderr  string
	}{
		{failure: stageFails, stderr: stageStderr},
		{failure: commitFails, stderr: commitStderr},
	} {
		t.Run(tc.failure.name, func(t *testing.T) {
			t.Parallel()

			run := recordAnswersFailing(t, ptr(answeredConfig), false, tc.failure)

			assertAbortedBeforeTheNotes(t, run)
			failure := notesFailure(t, run.rec)
			if failure.Message != "committing the exclusion answers" {
				t.Errorf("✗ line = %q, want %q", failure.Message, "committing the exclusion answers")
			}
			if failure.Output != tc.stderr {
				t.Errorf("beneath the ✗ line = %q, want git's stderr %q", failure.Output, tc.stderr)
			}
		})
	}
}

func TestRelease_AnswersCommitFailingAfterTheArtifactCommit_UnwindsToTheStartAndKeepsNoAnswersCommit(t *testing.T) {
	t.Parallel()

	const before = preTagHookConfig + "[release]\ndiff_exclude = [\"specs/\"]\n"
	run := recordAnswersFailing(t, ptr(before), true, commitFails)

	assertAbortedBeforeTheNotes(t, run)
	if !invokedWith(run.f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("the run was not reset to the starting commit; got %v", commandLines(run.f.Invocations()))
	}
	if got := countCommitsWithSubject(run.f, run.root, answersSubject(nextTag)); got != 1 {
		t.Errorf("answers commit attempts = %d, want only the one that failed", got)
	}
	unstageAt := indexOfCmd(run.f, "git", unstageConfigArgs(run.root)...)
	if resetAt := indexOfCmd(run.f, "git", "reset", "--hard", startingSHA); unstageAt == -1 || unstageAt > resetAt {
		t.Errorf("the staged .mint.toml was not unstaged before the reset; got %v", commandLines(run.f.Invocations()))
	}
	if got := readConfigBytes(t, run.root); got != before {
		t.Errorf(".mint.toml =\n%s\nwant it as before the answers\n%s", got, before)
	}
	if got, want := unwoundText(t, run.rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
}

func TestRelease_AnswersStagedButTheirCommitFailing_LeavesMintTomlAsBeforeTheRun(t *testing.T) {
	t.Parallel()

	run := recordAnswersFailing(t, ptr(answeredConfig), false, commitFails)

	if got := readConfigBytes(t, run.root); got != answeredConfig {
		t.Errorf(".mint.toml =\n%s\nwant it byte-identical to\n%s", got, answeredConfig)
	}
	commitAt := indexOfCmd(run.f, "git", "-C", run.root, "commit", "-m", answersSubject(nextTag))
	unstageAt := indexOfCmd(run.f, "git", unstageConfigArgs(run.root)...)
	if unstageAt == -1 || unstageAt < commitAt {
		t.Errorf("the staged .mint.toml was not unstaged after the failed commit; got %v", commandLines(run.f.Invocations()))
	}
	if invokedWith(run.f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("a run that made no commit was reset")
	}
}

func TestRelease_AnswersFailingWithNoConfigFile_LeavesNoMintToml(t *testing.T) {
	t.Parallel()

	for _, failure := range []answersFailure{stageFails, commitFails} {
		t.Run(failure.name, func(t *testing.T) {
			t.Parallel()

			run := recordAnswersFailing(t, nil, false, failure)

			assertAbortedBeforeTheNotes(t, run)
			if _, err := os.Stat(filepath.Join(run.root, config.FileName)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("stat .mint.toml = %v, want it absent", err)
			}
			if !invokedWith(run.f, "git", unstageConfigArgs(run.root)...) {
				t.Errorf("the written .mint.toml was not unstaged; got %v", commandLines(run.f.Invocations()))
			}
		})
	}
}

func TestRelease_AfterTheAnswersFailToRecord_TheNextReleaseListsTheAnsweredPathsAgain(t *testing.T) {
	t.Parallel()

	for _, failure := range []answersFailure{writeFails, stageFails, commitFails} {
		t.Run(failure.name, func(t *testing.T) {
			t.Parallel()

			run := recordAnswersFailing(t, ptr(answeredConfig), false, failure)
			if run.err == nil {
				t.Fatal("Release succeeded with the answers failing to record")
			}
			failedChecks := checkInvocations(run.f, "claude")
			if len(failedChecks) != 1 {
				t.Fatalf("check calls = %d, want one", len(failedChecks))
			}

			next, _ := nextReleaseIn(t, run.root, exportGoDiff+designDocDiff,
				"M\tsrc/export.go\nA\tdocs/design/export.md\n", "1\t1\tsrc/export.go\n1\t0\tdocs/design/export.md\n")

			assertNotesGitCallsExclude(t, next, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)specs/")...)
			checks := checkInvocations(next, "claude")
			if len(checks) != 1 {
				t.Fatalf("next check calls = %d, want one", len(checks))
			}
			want := checkList(failedChecks[0].Stdin)
			if got := checkList(checks[0].Stdin); !slices.Equal(got, want) || !slices.ContainsFunc(got, func(l string) bool { return strings.HasPrefix(l, "- docs/design/export.md ") }) {
				t.Errorf("next check list = %q, want the answered path listed again as %q", got, want)
			}
		})
	}
}

func TestRelease_AnswersCommitFailing_WhenUnstagingTheWriteFails_WarnsTheManualStepAndReportsManualCleanup(t *testing.T) {
	t.Parallel()

	const unstageStderr = "fatal: Unable to create '.git/index.lock': Permission denied\n"
	failure := commitFails
	failure.git = append(slices.Clone(failure.git), gitFailing(unstageStderr))
	run := recordAnswersFailing(t, ptr(preTagHookConfig), true, failure)

	assertAbortedBeforeTheNotes(t, run)
	var warns []presenter.Warning
	for _, ev := range run.rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	want := presenter.Warning{
		Label:   "unwind incomplete",
		Message: "automatic recovery failed; manually unstage .mint.toml (`git -C " + run.root + " reset -q " + startingSHA + " -- .mint.toml`)",
		Output:  unstageStderr,
	}
	if len(warns) != 1 || warns[0] != want {
		t.Errorf("warnings = %+v, want only %+v", warns, want)
	}
	if got, want := unwoundText(t, run.rec), "reset 1 commit; manual cleanup required"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
	assertCloses(t, run.rec, presenter.OutcomeUnwound, "back where it started · manual cleanup required · nothing pushed")
}

func TestRelease_AnswersCommitFailing_WhenUnstagingFailsInARootWithASpace_WarnsTheStepWithTheRootQuoted(t *testing.T) {
	t.Parallel()

	const unstageStderr = "fatal: Unable to create '.git/index.lock': Permission denied\n"
	root := filepath.Join(t.TempDir(), "My App")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("creating %s: %v", root, err)
	}
	failure := commitFails
	failure.git = append(slices.Clone(failure.git), gitFailing(unstageStderr))
	run := recordAnswersFailingIn(t, root, ptr(answeredConfig), false, failure)

	assertAbortedBeforeTheNotes(t, run)
	var warns []presenter.Warning
	for _, ev := range run.rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	want := presenter.Warning{
		Label:   "unwind incomplete",
		Message: "automatic recovery failed; manually unstage .mint.toml (`git -C '" + root + "' reset -q " + startingSHA + " -- .mint.toml`)",
		Output:  unstageStderr,
	}
	if len(warns) != 1 || warns[0] != want {
		t.Errorf("warnings = %+v, want only %+v", warns, want)
	}
}

func TestRelease_AnswersCommitFailingWithNoPreTagHook_WhenUnstagingFails_StillRestoresMintToml(t *testing.T) {
	t.Parallel()

	const unstageStderr = "fatal: Unable to create '.git/index.lock': Permission denied\n"
	failure := commitFails
	failure.git = append(slices.Clone(failure.git), gitFailing(unstageStderr))
	run := recordAnswersFailing(t, ptr(answeredConfig), false, failure)

	assertAbortedBeforeTheNotes(t, run)
	if got := readConfigBytes(t, run.root); got != answeredConfig {
		t.Errorf(".mint.toml =\n%s\nwant it byte-identical to\n%s", got, answeredConfig)
	}
	var warns []presenter.Warning
	for _, ev := range run.rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	want := presenter.Warning{
		Label:   "unwind incomplete",
		Message: "automatic recovery failed; manually unstage .mint.toml (`git -C " + run.root + " reset -q " + startingSHA + " -- .mint.toml`)",
		Output:  unstageStderr,
	}
	if len(warns) != 1 || warns[0] != want {
		t.Errorf("warnings = %+v, want only %+v", warns, want)
	}
	// Nothing was committed to reset, so the run fails, the revert it could not finish
	// named in its close.
	assertCloses(t, run.rec, presenter.OutcomeFailed, "manual cleanup required · nothing pushed")
}
