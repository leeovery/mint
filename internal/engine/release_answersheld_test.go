package engine_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const minorTag = "v1.3.0"

// answeredConfig is a .mint.toml already holding answers of its own, so a write of
// any new one would show in its bytes.
const answeredConfig = "# answers so far\n[release]\ndiff_exclude = [\"specs/\"] # old specs\ndiff_keep = [\"docs/api/\"]\n"

const threeSuggestions = "docs/design/: design documents\ntest/fixtures/: test data\nREADME.md: readme wording\n"

var threeSuggestionGlobs = []string{"docs/design/", "test/fixtures/", "README.md"}

// seedThreeSuggestionNotes scripts the notes diff the three suggestions are asked about.
func seedThreeSuggestionNotes(f *runner.FakeRunner) {
	f.SeedSequence("git", ScriptedOut(exportGoDiff+designDocDiff+fixturesDiff+readmeDiff))
}

// pausesThen answers the exclusion pauses with answers in turn and the pause after
// them through last; every other gate takes its default.
func pausesThen(answers []presenter.Choice, last func() (presenter.Choice, error)) *presentertest.RecordingPresenter {
	return &presentertest.RecordingPresenter{
		PromptResult: func(g presenter.Gate) (presenter.Choice, error) {
			if !isPauseGate(g) {
				return g.Default, nil
			}
			if len(answers) == 0 {
				return last()
			}
			choice := answers[0]
			answers = answers[1:]
			return choice, nil
		},
	}
}

func readConfigBytes(t *testing.T, root string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("reading .mint.toml: %v", err)
	}
	return string(got)
}

// assertNothingRecorded fails unless .mint.toml reads as before, nothing staged it, no
// answers commit for tag was made, and no answers line was narrated.
func assertNothingRecorded(t *testing.T, rec *presentertest.RecordingPresenter, f *runner.FakeRunner, root, before, tag string) {
	t.Helper()
	if got := readConfigBytes(t, root); got != before {
		t.Errorf(".mint.toml =\n%s\nwant it byte-identical to\n%s", got, before)
	}
	for _, inv := range invocationsNamed(f, "git") {
		if slices.Contains(inv.Args, "add") && slices.Contains(inv.Args, config.FileName) {
			t.Errorf("the run staged .mint.toml: %s", commandLine(inv))
		}
	}
	if got := countCommitsWithSubject(f, root, answersSubject(tag)); got != 0 {
		t.Errorf("answers commits = %d, want none", got)
	}
	if got := countKind(rec, presentertest.KindExclusionAnswersRecorded); got != 0 {
		t.Errorf("answers lines = %d, want none", got)
	}
}

func TestRelease_CtrlCAtTheThirdPauseAfterAnsweringTwo_RecordsNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, answeredConfig)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedThreeSuggestionNotes(f)
	f.SeedSequence("claude", ScriptedOut(threeSuggestions), ScriptedOut(aiBody))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// A Ctrl-C reaches the run as a signal cancelling it or, at the raw-mode key read, as closed input; this pause delivers both.
	rec := pausesThen([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, func() (presenter.Choice, error) {
		cancel()
		return "", presenter.ErrInputClosed
	})

	err := engine.Release(ctx, attendedDeps(rec, f), minorOptions())

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want a cancel", err)
	}
	if got := len(pauseGates(rec)); got != 3 {
		t.Fatalf("pauses = %d, want the Ctrl-C at the third", got)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made after the Ctrl-C")
	}
	assertNoMutation(t, f)
	assertNothingRecorded(t, rec, f, root, answeredConfig, minorTag)
}

func TestRelease_StdinClosedAtTheThirdPauseAfterAnsweringTwo_UnwindsTheArtifactCommitAndRecordsNothing(t *testing.T) {
	t.Parallel()

	const before = preTagHookConfig + "[release]\ndiff_exclude = [\"specs/\"]\n"
	root, f := gateAbortRelease(t, before, seedThreeSuggestionNotes)
	f.SeedSequence("claude", ScriptedOut(threeSuggestions), ScriptedOut(aiBody))
	rec := pausesThen([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, func() (presenter.Choice, error) {
		return "", presenter.ErrInputClosed
	})

	err := engine.Release(t.Context(), attendedDeps(rec, f), minorOptions())

	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err = %v, want the closed input", err)
	}
	if got := len(pauseGates(rec)); got != 3 {
		t.Fatalf("pauses = %d, want stdin closing at the third", got)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made; the closed input was taken as keep")
	}
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("the pre_tag artifact commit was not unwound; got %v", commandLines(f.Invocations()))
	}
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Text() = %q, want %q", got, want)
	}
	assertNoMutation(t, f)
	assertNothingRecorded(t, rec, f, root, before, minorTag)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("loading .mint.toml: %v", err)
	}
	for _, glob := range threeSuggestionGlobs {
		if slices.Contains(cfg.Release.DiffExclude, glob) || slices.Contains(cfg.Release.DiffKeep, glob) {
			t.Errorf("%s was recorded: diff_exclude = %q, diff_keep = %q", glob, cfg.Release.DiffExclude, cfg.Release.DiffKeep)
		}
	}
}

const twoUnattendedSuggestions = "docs/design/: design documents\ntmp/: scratch files\n"

func TestRelease_UnderYes_WithSurvivingSuggestions_RecordsNothing(t *testing.T) {
	t.Parallel()

	for _, interactive := range []bool{true, false} {
		t.Run(fmt.Sprintf("stdin a terminal: %v", interactive), func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, answeredConfig, twoUnattendedSuggestions, notesBody(aiBody, nil)).shipping()
			run.deps.StdinInteractive = interactive

			if err := run.release(t); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if _, got := onlySuggestions(t, run.rec); len(got.Suggestions) != 2 {
				t.Fatalf("suggestions printed = %d, want both surviving", len(got.Suggestions))
			}
			assertNothingRecorded(t, run.rec, run.f, run.root, answeredConfig, nextTag)
		})
	}
}

func TestRelease_DryRun_WithSurvivingSuggestions_WritesNothingAndCommitsNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		yes  bool
	}{
		{name: "at a terminal without -y"},
		{name: "-y", yes: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, answeredConfig, twoUnattendedSuggestions, notesBody(aiBody, nil))
			run.deps.Yes = tc.yes
			run.deps.StdinInteractive = true

			if err := run.releaseWith(t, dryRunOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := pauseGates(run.rec); len(got) != 0 {
				t.Errorf("the dry run paused: %v", pauseQuestions(got))
			}
			assertNothingRecorded(t, run.rec, run.f, run.root, answeredConfig, nextTag)
			for _, inv := range invocationsNamed(run.f, "git") {
				if slices.Contains(inv.Args, "commit") {
					t.Errorf("the dry run committed: %s", commandLine(inv))
				}
			}
		})
	}
}

func TestRelease_Attended_CheckAnsweringNoneOrFailing_RecordsNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		check    string
		checkErr error
	}{
		{name: "the AI answers none", check: "none"},
		{name: "an answer mint cannot read", check: "nothing here looks out of place\n"},
		{name: "a generation failure", checkErr: &ai.GenerationError{Stdout: notLoggedIn, ExitCode: 1}},
		{name: "a timeout", checkErr: fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout)},
		{name: "a missing binary", checkErr: fmt.Errorf("%w: claude", ai.ErrCommandMissing)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, answeredConfig, tc.check, notesBody(aiBody, nil)).shipping()
			run.deps.Yes = false
			run.deps.StdinInteractive = true
			if tc.checkErr != nil {
				run.deps.Transport = &checkFailingTransport{checkErr: tc.checkErr}
			}

			if err := run.release(t); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			if got := pauseGates(run.rec); len(got) != 0 {
				t.Errorf("pauses = %v, want none", pauseQuestions(got))
			}
			assertNothingRecorded(t, run.rec, run.f, run.root, answeredConfig, nextTag)
		})
	}
}
