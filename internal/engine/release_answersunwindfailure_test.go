package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/release"
	"mint/internal/runner"
)

const resetStderr = "fatal: Could not reset index file to revision '" + startingSHA + "'.\n"

// unwindWarnings returns every warning the run emitted, in order.
func unwindWarnings(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	return warns
}

// assertUnwoundWithOneWarning fails unless the run exited 1 having warned want alone
// and narrated summary once.
func assertUnwoundWithOneWarning(t *testing.T, rec *presentertest.RecordingPresenter, err error, want presenter.Warning, summary string) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", err)
	}
	if warns := unwindWarnings(rec); len(warns) != 1 || warns[0] != want {
		t.Errorf("warnings = %+v, want only %+v", warns, want)
	}
	if got := unwoundSummary(t, rec); got != summary {
		t.Errorf("Unwound.Summary = %q, want %q", got, summary)
	}
	if n := countKind(rec, presentertest.KindUnwound); n != 1 {
		t.Errorf("Unwound events = %d, want one", n)
	}
}

// pushRejectedAfterTheTag is git's answers from the bookkeeping commit to a rejected
// push, leaving the answers commit one below the tip.
func pushRejectedAfterTheTag() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin
		ScriptedOut(""),              // tag -a v1.3.0 -F -
		pushRejected(),               // push --atomic origin HEAD v1.3.0
	}
}

// answersCommitRead is git's answer to the unwind's read of the answers commit's sha.
var answersCommitRead = ScriptedOut(answersSHA + "\n")

// assertAnswersCommitReadFirst fails unless the unwind reads the answers commit's sha
// as rev immediately before its first mutation.
func assertAnswersCommitReadFirst(t *testing.T, f *runner.FakeRunner, rev string) {
	t.Helper()
	want := "git rev-parse " + rev
	read := -1
	for i, inv := range f.Invocations() {
		if commandLine(inv) == want {
			read = i
		}
	}
	if read == -1 {
		t.Fatalf("no read of %s; got %v", rev, commandLines(f.Invocations()))
	}
	first := indexOfCmd(f, "git", "reset", "--hard", startingSHA)
	if del := indexOfCmd(f, "git", "tag", "-d", minorTagV130); del != -1 && del < first {
		first = del
	}
	if first != read+1 {
		t.Errorf("unwind calls = %q, want the read immediately before the first mutation", commandLines(f.Invocations()[read:]))
	}
}

func TestRelease_UnwindAfterAnswering_WhenTheReCommitFails_WarnsTheStepThatCommitsTheAnswers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		git    []runner.ScriptedCall
		stderr string
	}{
		{name: "staging fails", git: []runner.ScriptedCall{gitFailing(stageStderr)}, stderr: stageStderr},
		{name: "committing fails", git: []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)}, stderr: commitStderr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceNo})
			a := newAnsweredAbort(t, rec, preTagHookConfig)
			a.f.SeedSequence("git", append([]runner.ScriptedCall{answersCommitRead, ScriptedOut("")}, tc.git...)...)

			err := a.release(t.Context())

			want := presenter.Warning{
				Label: "unwind incomplete",
				Message: "automatic recovery failed; manually commit the exclusion answers mint wrote to .mint.toml (`git -C " + a.root +
					" add .mint.toml` then `git -C " + a.root + " commit -m '" + answersSubject(minorTagV130) + "'`)",
				Output: tc.stderr,
			}
			assertUnwoundWithOneWarning(t, rec, err, want, "reset 2 commits; manual cleanup required")
			got := readConfigBytes(t, a.root)
			if !strings.HasPrefix(got, a.atStart) {
				t.Errorf(".mint.toml =\n%s\nwant the starting config kept", got)
			}
			cfg, lerr := config.Load(a.root)
			if lerr != nil {
				t.Fatalf("loading .mint.toml: %v", lerr)
			}
			if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
				t.Errorf("[release].diff_exclude = %q, want the answers left for the step to commit %q", cfg.Release.DiffExclude, want)
			}
		})
	}
}

// unreadableConfigRunner plays git's part in the unwind's reset, leaving .mint.toml
// under root a directory the re-commit cannot read.
type unreadableConfigRunner struct {
	*runner.FakeRunner
	root string
}

func (r *unreadableConfigRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	if name == "git" && slices.Equal(args, []string{"reset", "--hard", startingSHA}) {
		path := filepath.Join(r.root, config.FileName)
		if err := os.Remove(path); err != nil {
			return runner.Result{ExitCode: 1}, err
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			return runner.Result{ExitCode: 1}, err
		}
	}
	return r.FakeRunner.Run(ctx, name, args...)
}

func TestRelease_UnwindAfterAnswering_WhenTheReCommitCannotWriteTheAnswers_WarnsTheStepThatRestoresTheAnswersCommit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		tail        []runner.ScriptedCall
		beforeReset []runner.ScriptedCall
		rev         string
		summary     string
	}{
		{
			name:    "the answers commit at the tip",
			rev:     "HEAD",
			summary: "reset 2 commits; manual cleanup required",
		},
		{
			name:        "the answers commit below the bookkeeping commit",
			tail:        pushRejectedAfterTheTag(),
			beforeReset: []runner.ScriptedCall{ScriptedOut("")}, // tag -d v1.3.0
			rev:         "HEAD~1",
			summary:     "reset 3 commits and deleted tag " + minorTagV130 + "; manual cleanup required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reviews := []presenter.Choice{presenter.ChoiceNo}
			if tc.tail != nil {
				reviews = nil
			}
			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, reviews)
			a := newAnsweredAbort(t, rec, preTagHookConfig)
			mut := git.NewMutator(&unreadableConfigRunner{FakeRunner: a.f, root: a.root})
			a.deps.Mutator, a.deps.Releaser = mut, release.NewReleaser(mut)
			a.f.SeedSequence("git", tc.tail...)
			a.f.Seed("gh", runner.Result{}, nil)
			a.f.SeedSequence("git", slices.Concat([]runner.ScriptedCall{answersCommitRead}, tc.beforeReset, []runner.ScriptedCall{ScriptedOut("")})...)

			err := a.release(t.Context())

			path := filepath.Join(a.root, config.FileName)
			want := presenter.Warning{
				Label:   "unwind incomplete",
				Message: "automatic recovery failed; manually restore the exclusion answers commit on top (`git cherry-pick " + answersSHA + "`)",
				Output:  "reading " + config.FileName + ": read " + path + ": is a directory",
			}
			assertUnwoundWithOneWarning(t, rec, err, want, tc.summary)
			assertAnswersCommitReadFirst(t, a.f, tc.rev)
			reset := indexOfCmd(a.f, "git", "reset", "--hard", startingSHA)
			if after := a.f.Invocations()[reset+1:]; len(after) != 0 {
				t.Errorf("after the reset = %q, want no git call", commandLines(after))
			}
		})
	}
}

func TestRelease_UnwindAfterAnswering_WhenTheResetFails_DoesNotReCommitAndNamesTheRestoreStepAfterTheReset(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		tail        []runner.ScriptedCall
		beforeReset []runner.ScriptedCall
		rev         string
		summary     string
	}{
		{
			name:    "the answers commit at the tip",
			rev:     "HEAD",
			summary: "reset 2 commits; manual cleanup required",
		},
		{
			name:        "the answers commit below the bookkeeping commit",
			tail:        pushRejectedAfterTheTag(),
			beforeReset: []runner.ScriptedCall{ScriptedOut("")}, // tag -d v1.3.0
			rev:         "HEAD~1",
			summary:     "reset 3 commits and deleted tag " + minorTagV130 + "; manual cleanup required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reviews := []presenter.Choice{presenter.ChoiceNo}
			if tc.tail != nil {
				reviews = nil
			}
			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, reviews)
			a := newAnsweredAbort(t, rec, preTagHookConfig)
			a.f.SeedSequence("git", tc.tail...)
			a.f.Seed("gh", runner.Result{}, nil)
			a.f.SeedSequence("git", slices.Concat([]runner.ScriptedCall{answersCommitRead}, tc.beforeReset, []runner.ScriptedCall{gitFailing(resetStderr)})...)

			err := a.release(t.Context())

			want := presenter.Warning{
				Label: "unwind incomplete",
				Message: "automatic recovery failed; manually reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA +
					"`), then restore the exclusion answers commit on top (`git cherry-pick " + answersSHA + "`)",
				Output: "exit status 1",
			}
			assertUnwoundWithOneWarning(t, rec, err, want, tc.summary)
			assertAnswersCommitReadFirst(t, a.f, tc.rev)
			reset := indexOfCmd(a.f, "git", "reset", "--hard", startingSHA)
			if reset == -1 {
				t.Fatalf("no reset to the starting commit; got %v", commandLines(a.f.Invocations()))
			}
			if after := a.f.Invocations()[reset+1:]; len(after) != 0 {
				t.Errorf("after the failed reset = %q, want no re-commit", commandLines(after))
			}
		})
	}
}

func TestRelease_UnwindAfterAnswering_WhenTheShaCannotBeRead_StillResetsAndNamesTheAnswersCommitBelowOrigHead(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAnsweredAbort(t, rec, preTagHookConfig)
	a.f.SeedSequence("git", pushRejectedAfterTheTag()...)
	a.f.Seed("gh", runner.Result{}, nil)
	a.f.SeedSequence("git", gitFailing("fatal: bad revision 'HEAD~1'\n"), ScriptedOut(""), gitFailing(resetStderr))

	err := a.release(t.Context())

	want := presenter.Warning{
		Label: "unwind incomplete",
		Message: "automatic recovery failed; manually reset HEAD back to " + startingSHA + " (`git reset --hard " + startingSHA +
			"`), then restore the exclusion answers commit on top (`git cherry-pick ORIG_HEAD~1`)",
		Output: "exit status 1",
	}
	assertUnwoundWithOneWarning(t, rec, err, want, "reset 3 commits and deleted tag "+minorTagV130+"; manual cleanup required")
	assertAnswersCommitReadFirst(t, a.f, "HEAD~1")
}
