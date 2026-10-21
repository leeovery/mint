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

// ignoredAnswers is an attended prior-tag release whose .mint.toml git ignores, the
// operator excluding docs/design/ at the one pause.
type ignoredAnswers struct {
	root string
	f    *runner.FakeRunner
	rec  *presentertest.RecordingPresenter
}

// newIgnoredAnswers readies the release with .mint.toml holding config, or absent when
// config is nil, through the notes; preTag adds the pre_tag hook's artifact commit. The
// caller scripts git from the bookkeeping commit on.
func newIgnoredAnswers(t *testing.T, rec *presentertest.RecordingPresenter, config *string, preTag bool) *ignoredAnswers {
	t.Helper()
	root := t.TempDir()
	if config != nil {
		writeConfig(t, root, *config)
	}
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	if preTag {
		seedPreTagArtifactCommit(f)
	}
	f.SeedSequence("git", append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), configIgnored, ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...)
	f.SeedSequence("claude", ScriptedOut(designDocAnswer), ScriptedOut(aiBody))
	f.Seed("gh", runner.Result{}, nil)
	return &ignoredAnswers{root: root, f: f, rec: rec}
}

func (a *ignoredAnswers) release(t *testing.T) error {
	t.Helper()
	return engine.Release(t.Context(), attendedDeps(a.rec, a.f), priorTagNormalAIOptions())
}

func (a *ignoredAnswers) probeArgs() []string {
	return []string{"-C", a.root, "check-ignore", "-q", "--", config.FileName}
}

// assertNotCommitted fails unless the run probed .mint.toml, then neither staged it nor
// made an answers commit nor unstaged it.
func (a *ignoredAnswers) assertNotCommitted(t *testing.T) {
	t.Helper()
	if !invokedWith(a.f, "git", a.probeArgs()...) {
		t.Errorf("no check-ignore of .mint.toml at the root; got %v", commandLines(a.f.Invocations()))
	}
	for _, inv := range invocationsNamed(a.f, "git") {
		if slices.Contains(inv.Args, config.FileName) && (slices.Contains(inv.Args, "add") || slices.Contains(inv.Args, "reset")) {
			t.Errorf("the run staged or unstaged .mint.toml: %s", commandLine(inv))
		}
	}
	if got := countCommitsWithSubject(a.f, a.root, answersSubject(nextTag)); got != 0 {
		t.Errorf("answers commits = %d, want none", got)
	}
}

func (a *ignoredAnswers) assertAnswersWritten(t *testing.T, want string) {
	t.Helper()
	if got := readConfigBytes(t, a.root); got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
}

const (
	ignoredStartConfig = "# kept local\n[release]\ncommit_prefix = \"🌿\"\n"
	ignoredAnswered    = ignoredStartConfig + "diff_exclude = [\"docs/design/\"]\n"
	createdAnswers     = "[release]\ndiff_exclude = [\"docs/design/\"]\n"
)

func TestRelease_IgnoredConfig_WritesTheAnswersWithoutCommittingThemAndReleases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config *string
		want   string
	}{
		{name: "an existing file", config: ptr(ignoredStartConfig), want: ignoredAnswered},
		{name: "an absent file under a broad rule", want: createdAnswers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newIgnoredAnswers(t, answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil), tc.config, false)
			seedRecordTagPush(a.f, a.root)

			if err := a.release(t); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			a.assertNotCommitted(t)
			a.assertAnswersWritten(t, tc.want)
			if got, want := commitSubjectsIn(a.f, a.root), []string{"🌿 Release " + nextTag}; !slices.Equal(got, want) {
				t.Errorf("commits = %q, want %q", got, want)
			}
			if !invokedWith(a.f, "git", "push", "--atomic", "origin", "HEAD", nextTag) {
				t.Errorf("the release was not pushed; got %v", commandLines(a.f.Invocations()))
			}
			if got := tagAnnotationBody(t, a.f, nextTag); got != terminatedAnnotationBody(aiBody) {
				t.Errorf("tag annotation body = %q, want the AI notes", got)
			}
			assertNoNotesCallCarries(t, a.f, "docs/design/")
		})
	}
}

func TestRelease_IgnoredConfig_TheProbeRunsAtTheRootAfterTheLastPause(t *testing.T) {
	t.Parallel()

	a := newIgnoredAnswers(t, answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil), ptr(ignoredStartConfig), false)
	seedRecordTagPush(a.f, a.root)
	if err := a.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	probeAt := indexOfCmd(a.f, "git", a.probeArgs()...)
	if probeAt == -1 || countCmd(a.f, "git", a.probeArgs()...) != 1 {
		t.Fatalf("check-ignore calls = %d, want one; got %v", countCmd(a.f, "git", a.probeArgs()...), commandLines(a.f.Invocations()))
	}
	if got := notesDiffCalls(a.f); len(got) != 2 {
		t.Fatalf("notes diff calls = %d, want the checked diff and the excluded one", len(got))
	}
	checkedDiffAt := indexOfCmd(a.f, "git", notesDiffCalls(a.f)[0]...)
	notesDiffAt := indexOfCmd(a.f, "git", notesDiffCalls(a.f)[1]...)
	if probeAt < checkedDiffAt || probeAt > notesDiffAt {
		t.Errorf("the probe (at %d) does not sit between the checked diff (at %d) and the notes diff (at %d)", probeAt, checkedDiffAt, notesDiffAt)
	}
}

func TestRelease_IgnoredConfig_NarratesTheAnswersWrittenAndNotCommitted(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newIgnoredAnswers(t, rec, ptr(ignoredStartConfig), false)
	seedRecordTagPush(a.f, a.root)
	if err := a.release(t); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if n := countKind(rec, presentertest.KindExclusionAnswersRecorded); n != 1 {
		t.Fatalf("answers lines = %d, want one; events = %v", n, rec.Kinds())
	}
	recorded := indexOfKind(rec, presentertest.KindExclusionAnswersRecorded)
	want := presenter.ExclusionAnswers{Stage: "notes", File: ".mint.toml", Table: "[release]", Ignored: true}
	if got := rec.Events[recorded].ExclusionAnswers; got != want {
		t.Errorf("answers payload = %+v, want %+v", got, want)
	}
	kinds := rec.Kinds()
	if len(pauseGates(rec)) != 1 || indexOfPauseFrom(rec, recorded) != -1 {
		t.Errorf("the answers line does not follow the pause; events = %v", kinds)
	}
	if kinds[recorded-1] != presentertest.KindSuspendSpinner || kinds[recorded+1] != presentertest.KindResumeSpinner {
		t.Errorf("the answers line is not shown with the notes line suspended; events = %v", kinds[recorded-1:recorded+2])
	}
	notesDone := indexOfKindFrom(rec, presentertest.KindStageSucceeded, recorded)
	if notesDone == -1 || rec.Events[notesDone].StageSucceeded.Name != "notes" {
		t.Errorf("the notes stage does not complete after the answers line; events = %v", kinds)
	}
}

func TestRelease_IgnoredConfig_AbortOrFailureAfterAnswering_KeepsTheWrite(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		config  *string
		preTag  bool
		reviews []presenter.Choice
		tail    []runner.ScriptedCall
		want    string
		summary string
	}{
		{
			name:    "a decline after the artifact commit",
			config:  ptr(preTagHookConfig + ignoredStartConfig),
			preTag:  true,
			reviews: []presenter.Choice{presenter.ChoiceNo},
			want:    preTagHookConfig + ignoredAnswered,
			summary: "reset 1 commit; repo clean",
		},
		{
			name:    "the bookkeeping commit failing",
			config:  ptr(preTagHookConfig + ignoredStartConfig),
			preTag:  true,
			tail:    []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)},
			want:    preTagHookConfig + ignoredAnswered,
			summary: "reset 1 commit; repo clean",
		},
		{
			name:    "a decline with the file the write created",
			reviews: []presenter.Choice{presenter.ChoiceNo},
			want:    createdAnswers,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, tc.reviews)
			a := newIgnoredAnswers(t, rec, tc.config, tc.preTag)
			a.f.SeedSequence("git", tc.tail...)
			a.f.SeedSequence("git", ScriptedOut("")) // the unwind's reset

			err := a.release(t)

			var abort *engine.AbortError
			if !errors.As(err, &abort) || abort.ExitCode != 1 {
				t.Fatalf("err = %v, want an abort exiting 1", err)
			}
			a.assertNotCommitted(t)
			a.assertAnswersWritten(t, tc.want)
			if tc.summary == "" {
				if n := countKind(rec, presentertest.KindUnwound); n != 0 {
					t.Errorf("Unwound events = %d, want none for a run that made no commit", n)
				}
				return
			}
			if !invokedWith(a.f, "git", "reset", "--hard", startingSHA) {
				t.Errorf("the run was not reset to the starting commit; got %v", commandLines(a.f.Invocations()))
			}
			if got := unwoundSummary(t, rec); got != tc.summary {
				t.Errorf("Unwound.Summary = %q, want %q", got, tc.summary)
			}
		})
	}
}

func TestRelease_IgnoredConfig_TheNextReleaseReadsTheAnswersAndAsksNothingAboutThem(t *testing.T) {
	t.Parallel()

	a := newIgnoredAnswers(t, answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil), ptr(ignoredStartConfig), false)
	seedRecordTagPush(a.f, a.root)
	if err := a.release(t); err != nil {
		t.Fatalf("the answering release returned unexpected error: %v", err)
	}

	f, next := nextReleaseIn(t, a.root, exportGoDiff, "M\tsrc/export.go\n", "1\t1\tsrc/export.go\n")

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
	checks := checkInvocations(f, "claude")
	if len(checks) != 1 {
		t.Fatalf("check calls = %d, want one", len(checks))
	}
	if got := checkList(checks[0].Stdin); slices.ContainsFunc(got, func(l string) bool { return strings.HasPrefix(l, "- docs/design/") }) {
		t.Errorf("check list = %q, want docs/design/ left out", got)
	}
	if got := len(pauseGates(next)); got != 0 {
		t.Errorf("pauses = %d, want none", got)
	}
}

func TestRelease_IgnoredConfig_AFailedWrite_ShowsTheWriteErrorAndStopsBeforeAnyTag(t *testing.T) {
	t.Parallel()

	run := recordAnswersFailing(t, ptr(answeredConfig), false,
		answersFailure{name: "writing an ignored .mint.toml", readOnly: true, git: []runner.ScriptedCall{configIgnored}})

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
	if got := countKind(run.rec, presentertest.KindExclusionAnswersRecorded); got != 0 {
		t.Errorf("answers lines = %d, want none", got)
	}
}

func TestRelease_TheIgnoreProbeFailing_ShowsGitsStderrAndUndoesTheWrite(t *testing.T) {
	t.Parallel()

	const probeStderr = "fatal: unable to read index\n"
	probeFails := runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: probeStderr}, Err: errors.New("exit status 128")}
	for _, tc := range []struct {
		name   string
		config *string
	}{
		{name: "an existing file", config: ptr(answeredConfig)},
		{name: "an absent file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := recordAnswersFailing(t, tc.config, false, answersFailure{name: "probing", git: []runner.ScriptedCall{probeFails}})

			assertAbortedBeforeTheNotes(t, run)
			failure := notesFailure(t, run.rec)
			if failure.Message != "committing the exclusion answers" || failure.Output != probeStderr {
				t.Errorf("✗ line = %q over %q, want %q over git's stderr %q", failure.Message, failure.Output, "committing the exclusion answers", probeStderr)
			}
			if got := countCommitsWithSubject(run.f, run.root, answersSubject(nextTag)); got != 0 {
				t.Errorf("answers commits = %d, want none", got)
			}
			path := filepath.Join(run.root, config.FileName)
			if tc.config == nil {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("stat .mint.toml = %v, want it absent", err)
				}
				return
			}
			if got := readConfigBytes(t, run.root); got != *tc.config {
				t.Errorf(".mint.toml =\n%s\nwant it as before the answers\n%s", got, *tc.config)
			}
		})
	}
}
