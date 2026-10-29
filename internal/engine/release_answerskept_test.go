package engine_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const minorTagV130 = "v1.3.0"

// startResettingRunner plays git's part in `git reset --hard {startingSHA}` that the
// fake cannot: it returns root's .mint.toml to its content at the starting commit.
type startResettingRunner struct {
	*runner.FakeRunner
	root    string
	atStart string
}

func (r *startResettingRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	if name == "git" && slices.Equal(args, []string{"reset", "--hard", startingSHA}) {
		if err := os.WriteFile(filepath.Join(r.root, config.FileName), []byte(r.atStart), 0o644); err != nil {
			return runner.Result{ExitCode: 1}, err
		}
	}
	return r.FakeRunner.Run(ctx, name, args...)
}

// answeredTransport answers the exclusion check with docs/design/, a classify call
// with OTHER, a call carrying nudge with regen, and any other notes call with notes.
type answeredTransport struct {
	nudge string
	regen func(context.Context) (string, error)
	notes func(context.Context) (string, error)
}

func (tr *answeredTransport) Generate(ctx context.Context, prompt string) (string, error) {
	switch {
	case isCheckPrompt(prompt):
		return designDocAnswer, nil
	case isClassifyPrompt(prompt):
		return "OTHER", nil
	case tr.nudge != "" && strings.HasPrefix(prompt, nudgedInstructions(tr.nudge)):
		return tr.regen(ctx)
	case tr.notes != nil:
		return tr.notes(ctx)
	default:
		return aiBody, nil
	}
}

func notesFailing(context.Context) (string, error) {
	return "", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}
}

// answeredAbort is an attended release of v1.3.0 whose operator excluded docs/design/
// at the one pause.
type answeredAbort struct {
	root    string
	atStart string
	f       *runner.FakeRunner
	deps    engine.ReleaseDeps
}

// newAnsweredAbort readies the release under config, the pre_tag hook making the
// artifact commit when config sets one, the notes diff, the answers commit and the
// notes calls scripted. The caller scripts what follows the notes, then unwound.
func newAnsweredAbort(t *testing.T, rec *presentertest.RecordingPresenter, config string) *answeredAbort {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	if strings.Contains(config, "pre_tag") {
		seedPreTagArtifactCommit(f)
	}
	f.SeedSequence("git", withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...)...)
	deps := newDepsWithMutator(rec, f, git.NewMutator(&startResettingRunner{FakeRunner: f, root: root, atStart: config}))
	deps.StdinInteractive = true
	deps.Transport = &answeredTransport{}
	return &answeredAbort{root: root, atStart: config, f: f, deps: deps}
}

// answersSHA is the answers commit's sha, as the unwind reads it.
const answersSHA = "answerssha"

// unwound scripts the unwind's read of the answers commit's sha, beforeReset, then the
// reset and its re-commit of the answers.
func (a *answeredAbort) unwound(beforeReset ...runner.ScriptedCall) *answeredAbort {
	a.f.SeedSequence("git", append(append([]runner.ScriptedCall{ScriptedOut(answersSHA + "\n")}, beforeReset...), ScriptedOut(""), ScriptedOut(""), ScriptedOut(""))...)
	return a
}

func (a *answeredAbort) release(ctx context.Context) error {
	return engine.Release(ctx, a.deps, minorOptions())
}

const keptSummary = "reset 2 commits; kept the answers commit on the starting commit; repo clean"

// assertAnswersKept fails unless the run exited 1 having made no tag left standing,
// the unwind reset to the starting commit and then committed .mint.toml alone, holding
// the starting config and the Exclude answer, under the answers subject, and narrated
// summary.
func assertAnswersKept(t *testing.T, rec *presentertest.RecordingPresenter, a *answeredAbort, err error, summary string) {
	t.Helper()
	var abort *engine.AbortError
	if !errors.As(err, &abort) || abort.ExitCode != 1 {
		t.Fatalf("err = %v, want an abort exiting 1", err)
	}
	reset := indexOfCmd(a.f, "git", "reset", "--hard", startingSHA)
	if reset == -1 {
		t.Fatalf("no reset to the starting commit; got %v", commandLines(a.f.Invocations()))
	}
	after := a.f.Invocations()[reset+1:]
	want := []string{
		"git -C " + a.root + " add " + config.FileName,
		"git -C " + a.root + " commit -m " + answersSubject(minorTagV130),
	}
	if got := commandLines(after); !slices.Equal(got, want) {
		t.Errorf("after the reset = %q, want the answers re-committed alone %q", got, want)
	}
	got, rerr := os.ReadFile(filepath.Join(a.root, config.FileName))
	if rerr != nil {
		t.Fatalf("reading .mint.toml: %v", rerr)
	}
	if !strings.HasPrefix(string(got), a.atStart) {
		t.Errorf(".mint.toml =\n%s\nwant the starting config kept", got)
	}
	cfg, lerr := config.Load(a.root)
	if lerr != nil {
		t.Fatalf("loading .mint.toml: %v", lerr)
	}
	if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	tags := countCmd(a.f, "git", "tag", "-a", "--cleanup=verbatim", minorTagV130, "-F", "-")
	if deleted := countCmd(a.f, "git", "tag", "-d", minorTagV130); tags != deleted {
		t.Errorf("tags made = %d, deleted = %d, want no %s left", tags, deleted, minorTagV130)
	}
	if got := unwoundSummary(t, rec); got != summary {
		t.Errorf("Unwound.Text() = %q, want %q", got, summary)
	}
	if n := countKind(rec, presentertest.KindUnwound); n != 1 {
		t.Errorf("Unwound events = %d, want one", n)
	}
}

func TestRelease_DeclineAfterAnswering_KeepsTheAnswersCommitOnTheStartingCommit(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceNo})
	a := newAnsweredAbort(t, rec, preTagHookConfig).unwound()

	err := a.release(t.Context())

	assertAnswersKept(t, rec, a, err, keptSummary)
	if got, want := commitSubjectsIn(a.f, a.root), []string{pretagArtifactSubject(minorTagV130), answersSubject(minorTagV130), answersSubject(minorTagV130)}; !slices.Equal(got, want) {
		t.Errorf("commits = %q, want %q", got, want)
	}
}

func TestRelease_ReviewGateAbortsAfterAnswering_KeepTheAnswersCommit(t *testing.T) {
	t.Parallel()

	editorFailure := errors.New(`launching editor "vi": exit status 1`)
	for _, tc := range []struct {
		name  string
		setup func(*testing.T) (*presentertest.RecordingPresenter, *answeredAbort)
	}{
		{
			name: "a failed r",
			setup: func(t *testing.T) (*presentertest.RecordingPresenter, *answeredAbort) {
				rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceRegen})
				rec.NextLines = []string{exportNudge}
				a := newAnsweredAbort(t, rec, preTagHookConfig)
				a.f.SeedSequence("git", append([]runner.ScriptedCall{ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...)
				a.deps.Transport = &answeredTransport{nudge: exportNudge, regen: func(context.Context) (string, error) {
					return "", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}
				}}
				return rec, a.unwound()
			},
		},
		{
			name: "a launched editor failing at e",
			setup: func(t *testing.T) (*presentertest.RecordingPresenter, *answeredAbort) {
				rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceEdit})
				a := newAnsweredAbort(t, rec, preTagHookConfig).unwound()
				a.deps.Editor = &fakeEditor{err: editorFailure}
				return rec, a
			},
		},
		{
			name: "stdin closing at the keypress",
			setup: func(t *testing.T) (*presentertest.RecordingPresenter, *answeredAbort) {
				review := presenter.ReleaseReviewGate()
				rec := &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
					switch {
					case isPauseGate(g):
						return presenter.ChoiceExclude, nil
					case g.Question == review.Question:
						return "", presenter.ErrInputClosed
					default:
						return g.Default, nil
					}
				}}
				return rec, newAnsweredAbort(t, rec, preTagHookConfig).unwound()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec, a := tc.setup(t)

			err := a.release(t.Context())

			assertAnswersKept(t, rec, a, err, keptSummary)
		})
	}
}

func TestRelease_FloorAbortsAfterAnswering_KeepTheAnswersCommit(t *testing.T) {
	t.Parallel()

	const abortConfig = preTagHookConfig + "[release]\non_notes_failure = \"abort\"\n"
	for _, tc := range []struct {
		name   string
		floor  func(presenter.Gate) (presenter.Choice, error)
		editor engine.Editor
	}{
		{name: "a", floor: func(presenter.Gate) (presenter.Choice, error) { return presenter.ChoiceAbort, nil }},
		{
			name:   "a launched editor failing at e",
			floor:  func(presenter.Gate) (presenter.Choice, error) { return presenter.ChoiceEdit, nil },
			editor: &fakeEditor{err: errors.New(`launching editor "vi": exit status 1`)},
		},
		{name: "stdin closing at the choice", floor: func(presenter.Gate) (presenter.Choice, error) { return "", presenter.ErrInputClosed }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &presentertest.RecordingPresenter{PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
				switch {
				case isPauseGate(g):
					return presenter.ChoiceExclude, nil
				case g.Has(presenter.ChoiceCommitSubjects):
					return tc.floor(g)
				default:
					return g.Default, nil
				}
			}}
			a := newAnsweredAbort(t, rec, abortConfig).unwound()
			a.deps.Transport = &answeredTransport{notes: notesFailing}
			a.deps.Editor = tc.editor

			err := a.release(t.Context())

			if got := len(floorGates(rec)); got != 1 {
				t.Fatalf("floor gates = %d, want the one choice", got)
			}
			assertAnswersKept(t, rec, a, err, keptSummary)
		})
	}
}

func TestRelease_FailuresBeforeThePushAfterAnswering_KeepTheAnswersCommit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		tail        []runner.ScriptedCall
		beforeReset []runner.ScriptedCall
		summary     string
	}{
		{
			name:    "the bookkeeping commit fails",
			tail:    []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)},
			summary: keptSummary,
		},
		{
			name: "the push is rejected after the tag",
			tail: []runner.ScriptedCall{
				ScriptedOut(""),              // -C root add CHANGELOG.md
				ScriptedOut(""),              // -C root commit -m
				ScriptedOut(githubRemoteURL), // remote get-url origin
				ScriptedOut(""),              // tag -a v1.3.0 -F -
				pushRejected(),               // push --atomic origin HEAD v1.3.0
			},
			beforeReset: []runner.ScriptedCall{ScriptedOut("")}, // tag -d v1.3.0
			summary:     "reset 3 commits and deleted tag " + minorTagV130 + "; kept the answers commit on the starting commit; repo clean",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
			a := newAnsweredAbort(t, rec, preTagHookConfig)
			a.f.SeedSequence("git", tc.tail...)
			a.f.Seed("gh", runner.Result{}, nil)
			a.unwound(tc.beforeReset...)

			err := a.release(t.Context())

			assertAnswersKept(t, rec, a, err, tc.summary)
		})
	}
}

func TestRelease_CtrlCDuringTheNotesCallAfterAnswering_KeepsTheAnswersCommit(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAnsweredAbort(t, rec, preTagHookConfig).unwound()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	a.deps.Transport = &answeredTransport{notes: func(ctx context.Context) (string, error) {
		cancel()
		return "", ctx.Err()
	}}

	err := a.release(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	assertAnswersKept(t, rec, a, err, keptSummary)
}

func TestRelease_DeclineAfterAnsweringWithNoPreTagHook_KeepsTheAnswersCommit(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceNo})
	a := newAnsweredAbort(t, rec, "").unwound()

	err := a.release(t.Context())

	assertAnswersKept(t, rec, a, err, "reset 1 commit; kept the answers commit on the starting commit; repo clean")
}

func TestRelease_AfterAnAbortThatKeptTheAnswers_TheNextReleaseNeitherListsNorPausesOnThem(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceNo})
	a := newAnsweredAbort(t, rec, "").unwound()
	if err := a.release(t.Context()); err == nil {
		t.Fatal("the declined release succeeded")
	}

	f, next := nextReleaseIn(t, a.root, exportGoDiff, "M\tsrc/export.go\n", "1\t1\tsrc/export.go\n")

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
	checks := checkInvocations(f, "claude")
	if len(checks) != 1 {
		t.Fatalf("check calls = %d, want one", len(checks))
	}
	if got, want := checkList(checks[0].Stdin), []string{fmt.Sprintf("- src/export.go (modified, %d B)", len(exportGoDiff))}; !slices.Equal(got, want) {
		t.Errorf("check list = %q, want %q", got, want)
	}
	if got := len(pauseGates(next)); got != 0 {
		t.Errorf("pauses = %d, want none", got)
	}
}
