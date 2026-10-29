package engine_test

// This file pins the data a release and a regenerate put in their presenter payloads
// for the rail to draw: the header and the start station, the run's stages and its
// point of no return, the stage keys and times, the questions' nodes, the unwind's
// parts, and the end station every run closes at.

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

// railDeps are newDeps run from a repo directly under the home directory, by a
// stamped mint.
func railDeps(rec *presentertest.RecordingPresenter, f runner.CommandRunner, root string) engine.ReleaseDeps {
	mut := git.NewMutator(f)
	return engine.ReleaseDeps{
		Presenter:   rec,
		Runner:      f,
		Mutator:     mut,
		Releaser:    release.NewReleaser(mut),
		Home:        filepath.Dir(root),
		MintVersion: "0.1.2",
	}
}

// runInfo returns the run's one RunStarted payload.
func runInfo(t *testing.T, rec *presentertest.RecordingPresenter) presenter.RunInfo {
	t.Helper()
	if n := countKind(rec, presentertest.KindRunStarted); n != 1 {
		t.Fatalf("RunStarted events = %d, want one; kinds = %v", n, rec.Kinds())
	}
	return rec.Events[indexOfKind(rec, presentertest.KindRunStarted)].RunStarted
}

// quietPlan returns the run's one quiet plan: its stages for the rail.
func quietPlan(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Plan {
	t.Helper()
	var plans []presenter.Plan
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowPlan && ev.ShowPlan.Quiet {
			plans = append(plans, ev.ShowPlan)
		}
	}
	if len(plans) != 1 {
		t.Fatalf("quiet plans = %d, want one; kinds = %v", len(plans), rec.Kinds())
	}
	return plans[0]
}

// stageKeys returns the key of every stage event and warning the run emitted, in order.
func stageKeys(rec *presentertest.RecordingPresenter) []string {
	var keys []string
	for _, ev := range rec.Events {
		switch ev.Kind {
		case presentertest.KindStageStarted:
			keys = append(keys, ev.StageStarted.Key)
		case presentertest.KindStageSucceeded:
			keys = append(keys, ev.StageSucceeded.Key)
		case presentertest.KindStageFailed:
			keys = append(keys, ev.StageFailed.Key)
		case presentertest.KindWarn:
			keys = append(keys, ev.Warn.Key)
		}
	}
	return keys
}

// stageSuccess returns the success the run settled key with, failing the test unless
// there is exactly one.
func stageSuccess(t *testing.T, rec *presentertest.RecordingPresenter, key string) presenter.StageSuccess {
	t.Helper()
	var found []presenter.StageSuccess
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Key == key {
			found = append(found, ev.StageSucceeded)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s successes = %+v, want one; kinds = %v", key, found, rec.Kinds())
	}
	return found[0]
}

// assertKeysInPlan fails unless every stage event and warning names a stage of plan.
func assertKeysInPlan(t *testing.T, rec *presentertest.RecordingPresenter, plan presenter.Plan) {
	t.Helper()
	planned := map[string]bool{}
	for _, s := range plan.Stages {
		planned[s.Key] = true
	}
	for _, key := range stageKeys(rec) {
		if !planned[key] {
			t.Errorf("a stage event keyed %q names no stage of the plan %+v", key, plan.Stages)
		}
	}
}

// assertEveryPlannedStageSettled fails unless every stage of the run's plan, the one
// carrying its stage keys, ended filled: settled, checked, failed or warned on, or
// answered at a question carrying the detail it settles to.
func assertEveryPlannedStageSettled(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	filled := map[string]bool{}
	var plan presenter.Plan
	for _, ev := range rec.Events {
		switch ev.Kind {
		case presentertest.KindShowPlan:
			if len(ev.ShowPlan.Stages) > 0 {
				plan = ev.ShowPlan
			}
		case presentertest.KindPrompt:
			filled[ev.Prompt.Stage] = filled[ev.Prompt.Stage] || ev.Prompt.Detail != ""
		case presentertest.KindNotesChecked:
			filled[ev.NotesCheck.Key] = true
		case presentertest.KindStageSucceeded:
			filled[ev.StageSucceeded.Key] = true
		case presentertest.KindStageFailed:
			filled[ev.StageFailed.Key] = true
		case presentertest.KindWarn:
			filled[ev.Warn.Key] = true
		}
	}
	if len(plan.Stages) == 0 {
		t.Fatalf("no plan laid out stages; kinds = %v", rec.Kinds())
	}
	for _, s := range plan.Stages {
		if !filled[s.Key] {
			t.Errorf("planned stage %q never ended filled; kinds = %v", s.Key, rec.Kinds())
		}
	}
}

// priorTagRelease runs a prior-tag release of v1.2.4 through the AI notes, the
// review and a publish, under railDeps.
func priorTagRelease(t *testing.T, rec *presentertest.RecordingPresenter) (string, error) {
	t.Helper()
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	seedClaudeAfterCheck(f, ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	return root, engine.Release(t.Context(), railDeps(rec, f, root), priorTagNormalAIOptions())
}

func TestRelease_Rail_TheHeaderAndStartStationNameWhereTheRunBegins(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	root, err := priorTagRelease(t, rec)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	info := runInfo(t, rec)
	if want := (presenter.Header{Command: "release", Where: "~/" + filepath.Base(root), Version: "0.1.2"}); info.Header != want {
		t.Errorf("Header = %+v, want %+v", info.Header, want)
	}
	if want := (presenter.Station{Name: priorTag, Detail: "main @ 4f9c2e1"}); info.Start != want {
		t.Errorf("Start = %+v, want %+v (the previous tag, then the branch @ HEAD's short sha)", info.Start, want)
	}
}

func TestRelease_Rail_ABranchWithNoCommitsYetStartsSayingSo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stdout: "HEAD\n"}, Err: errors.New("exit status 128")}, // rev-parse HEAD --abbrev-ref HEAD
		ScriptedOut("main\n"),    // symbolic-ref --short -q HEAD
		gitFailing("fatal: bad"), // fetch --tags
	)
	rec := &presentertest.RecordingPresenter{}

	_ = engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	if want := (presenter.Station{Name: "first release", Detail: "main · no commits yet"}); runInfo(t, rec).Start != want {
		t.Errorf("Start = %+v, want %+v", runInfo(t, rec).Start, want)
	}
}

func TestRelease_Rail_AFirstReleaseStartsFromTheFirstRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if want := (presenter.Station{Name: "first release", Detail: "main @ 4f9c2e1"}); runInfo(t, rec).Start != want {
		t.Errorf("Start = %+v, want %+v", runInfo(t, rec).Start, want)
	}
	info := runInfo(t, rec)
	if want := (presenter.Header{Command: "release", Where: root}); info.Header != want {
		t.Errorf("Header = %+v, want %+v (no home to shorten against, an unstamped mint)", info.Header, want)
	}
}

func TestRelease_Rail_ThePlanLaysOutEveryStageWithThePushAsThePointOfNoReturn(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := priorTagRelease(t, rec); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	plan := quietPlan(t, rec)
	want := []presenter.PlanStage{
		{Key: "version", Name: "version", Detail: "v1.2.4 (patch bump)"},
		{Key: "preflight", Name: "preflight", Detail: "tree clean, on main, v1.2.4 free, origin in sync"},
		{Key: "check", Name: "check", Detail: "files that don't belong in the notes", Inside: true},
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
		{Key: "record", Name: "record", Detail: "CHANGELOG.md"},
		{Key: "push", Name: "push", Detail: "main + v1.2.4 → origin, atomic"},
		{Key: "publish", Name: "publish", Detail: "provider release"},
	}
	if !slices.Equal(plan.Stages, want) {
		t.Errorf("plan stages =\n%+v\nwant\n%+v", plan.Stages, want)
	}
	if plan.PointOfNoReturn != "push" {
		t.Errorf("PointOfNoReturn = %q, want the push", plan.PointOfNoReturn)
	}
	if len(plan.Steps) != 0 {
		t.Errorf("Steps = %+v, want none on the rail's plan (plain prints no plan for a real release)", plan.Steps)
	}
	if at, settled := indexOfKind(rec, presentertest.KindShowPlan), indexOfStage(rec, presentertest.KindStageSucceeded, "preflight"); at < settled {
		t.Errorf("the plan (at %d) came before preflight settled (at %d); it is known once preflight passes", at, settled)
	}
	assertKeysInPlan(t, rec, plan)
	assertEveryPlannedStageSettled(t, rec)
}

// stopAtTheNotes answers the version question yes and every later question no.
func stopAtTheNotes(g presenter.Gate) (presenter.Choice, error) {
	if g.Stage == "version" {
		return presenter.ChoiceYes, nil
	}
	return presenter.ChoiceNo, nil
}

func TestRelease_Rail_ThePlanLeavesOutWhatTheRunWillNotDo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config string
		seed   func(*runner.FakeRunner)
		want   []presenter.PlanStage
	}{
		{
			name:   "hooks, a version file and a named provider; no check on a notes file",
			config: "[release]\nprovider = \"github\"\nversion_file = \"VERSION\"\n\n[release.hooks]\npre_tag = [\"make build\", \"make dist\"]\npost_release = \"notify.sh\"\n",
			seed: func(f *runner.FakeRunner) {
				f.Seed("sh", runner.Result{ExitCode: 1, Stdout: "build failed\n"}, errors.New("exit status 1"))
			},
			want: []presenter.PlanStage{
				{Key: "version", Name: "version", Detail: "v1.2.4 (patch bump)"},
				{Key: "preflight", Name: "preflight", Detail: "tree clean, on main, v1.2.4 free, origin in sync"},
				{Key: "pre_tag", Name: "pre_tag", Detail: "make build && make dist"},
				{Key: "notes", Name: "notes", Detail: "from the notes file"},
				{Key: "record", Name: "record", Detail: "CHANGELOG.md + VERSION"},
				{Key: "push", Name: "push", Detail: "main + v1.2.4 → origin, atomic"},
				{Key: "publish", Name: "publish", Detail: "GitHub release"},
				{Key: "post_release", Name: "post_release", Detail: "notify.sh"},
			},
		},
		{
			name:   "no hooks, nothing to record and no publish",
			config: "[release]\npublish = false\nchangelog = false\n",
			seed:   func(*runner.FakeRunner) {},
			want: []presenter.PlanStage{
				{Key: "version", Name: "version", Detail: "v1.2.4 (patch bump)"},
				{Key: "preflight", Name: "preflight", Detail: "tree clean, on main, v1.2.4 free, origin in sync"},
				{Key: "notes", Name: "notes", Detail: "from the notes file"},
				{Key: "record", Name: "record", Detail: "nothing to record"},
				{Key: "push", Name: "push", Detail: "main + v1.2.4 → origin, atomic"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, tc.config)
			notesDir := t.TempDir()
			writeFile(t, notesDir, "notes.md", "Hand-written notes.\n")
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			tc.seed(f)
			rec := &presentertest.RecordingPresenter{PromptResult: stopAtTheNotes}
			opts := priorTagNormalAIOptions()
			opts.NotesFile, opts.NotesFileSet = filepath.Join(notesDir, "notes.md"), true

			_ = engine.Release(t.Context(), railDeps(rec, f, root), opts)

			if got := quietPlan(t, rec).Stages; !slices.Equal(got, tc.want) {
				t.Errorf("plan stages =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func TestRelease_Rail_TheQuestionsNameTheNodeTheyAskAbout(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := priorTagRelease(t, rec); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	gates := promptGates(rec)
	if len(gates) != 2 {
		t.Fatalf("questions = %d, want the version and the review", len(gates))
	}
	if gates[0].Stage != "version" || gates[0].Detail != "v1.2.4 (patch bump)" {
		t.Errorf("version question Stage/Detail = %q/%q, want version/%q", gates[0].Stage, gates[0].Detail, "v1.2.4 (patch bump)")
	}
	if gates[1].Stage != "notes" {
		t.Errorf("review question Stage = %q, want notes", gates[1].Stage)
	}
}

func TestRelease_Rail_TheStagesThatCommitCarryTheCommitsSubject(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := priorTagRelease(t, rec); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	if got := stageSuccess(t, rec, "record").Commit; got != "🌿 Release v1.2.4" {
		t.Errorf("record Commit = %q, want the bookkeeping commit's subject", got)
	}

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	f := runner.NewFakeRunner()
	seedPreTagDirty(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec = &presentertest.RecordingPresenter{}
	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	if got := stageSuccess(t, rec, "pre_tag").Commit; got != pretagArtifactSubject("v0.0.1") {
		t.Errorf("pre_tag Commit = %q, want the artifacts commit's subject", got)
	}
}

func TestRelease_Rail_AReleaseLandsAtItsTag(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	if _, err := priorTagRelease(t, rec); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	fin := closingEvent(t, rec)
	if fin.Outcome != presenter.OutcomeReleased {
		t.Errorf("Outcome = %v, want OutcomeReleased", fin.Outcome)
	}
	if want := (presenter.Station{Name: nextTag, Detail: "released"}); fin.End != want {
		t.Errorf("End = %+v, want %+v", fin.End, want)
	}
	if fin.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want the run's time", fin.Elapsed)
	}
}

func TestRelease_Rail_ADryRunLandsAtItsTagHavingChangedNothing(t *testing.T) {
	t.Parallel()

	opts := priorTagNormalAIOptions()
	opts.DryRun = true
	rec, _ := runDryRunNormalAI(t, t.TempDir(), t.TempDir(), priorTagDiff, aiBody, opts)

	fin := closingEvent(t, rec)
	if fin.Outcome != presenter.OutcomeDryRun {
		t.Errorf("Outcome = %v, want OutcomeDryRun", fin.Outcome)
	}
	if want := (presenter.Station{Name: nextTag, Detail: "dry run · nothing changed"}); fin.End != want {
		t.Errorf("End = %+v, want %+v", fin.End, want)
	}
}

func TestRelease_Rail_AFailureBeforeAnythingIsMadeClosesAtTheStartHavingChangedNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"), // tag --list
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(" M dirty.go"), // status --porcelain (dirty)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	fin := closingEvent(t, rec)
	if fin.Outcome != presenter.OutcomeFailed {
		t.Errorf("Outcome = %v, want OutcomeFailed", fin.Outcome)
	}
	if want := (presenter.Station{Name: priorTag, Detail: "nothing changed"}); fin.End != want {
		t.Errorf("End = %+v, want %+v", fin.End, want)
	}
	if got := onlyStageFailureEvent(t, rec).Key; got != "preflight" {
		t.Errorf("failure Key = %q, want preflight", got)
	}
}

// slowRunner holds every call slow picks for stageDelay before the FakeRunner
// answers it, so the stage making it has a time to measure.
type slowRunner struct {
	*runner.FakeRunner
	slow func(name string, args []string) bool
}

const stageDelay = 20 * time.Millisecond

func (r slowRunner) wait(name string, args []string) {
	if r.slow(name, args) {
		time.Sleep(stageDelay)
	}
}

func (r slowRunner) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	r.wait(name, args)
	return r.FakeRunner.RunWith(ctx, stdin, name, args...)
}

func (r slowRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return r.RunWith(ctx, nil, name, args...)
}

func (r slowRunner) RunInDir(ctx context.Context, dir string, env []string, name string, args ...string) (runner.Result, error) {
	r.wait(name, args)
	return r.FakeRunner.RunInDir(ctx, dir, env, name, args...)
}

func isPush(name string, args []string) bool { return name == "git" && slices.Contains(args, "push") }

func isHook(name string, _ []string) bool { return name == "sh" }

func TestRelease_Rail_AFailedPreTagHookIsItsNodeTimed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, preTagHookConfig)
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{ExitCode: 1, Stdout: "build failed\n"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), railDeps(rec, slowRunner{f, isHook}, root), patchOptions())

	assertAbortNonZero(t, err)
	failure := onlyStageFailureEvent(t, rec)
	if failure.Name != "pre_tag" || failure.Key != "pre_tag" || failure.Elapsed < stageDelay {
		t.Errorf("failure = %+v, want the pre_tag node timed at least %v", failure, stageDelay)
	}
}

func TestRelease_Rail_ARejectedPushIsThePushNodeTimedAndTheRunGoesBackToItsStart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		pushRejected(),  // push --atomic origin HEAD v0.0.1 (rejected)
		ScriptedOut(""), // unwind: tag -d v0.0.1
		ScriptedOut(""), // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), railDeps(rec, slowRunner{f, isPush}, root), patchOptions())

	assertAbortNonZero(t, err)
	failure := onlyStageFailureEvent(t, rec)
	if failure.Name != "tag" || failure.Key != "push" {
		t.Errorf("failure Name/Key = %q/%q, want the tag label on the push node", failure.Name, failure.Key)
	}
	if failure.Elapsed < stageDelay {
		t.Errorf("failure Elapsed = %v, want at least the push's %v", failure.Elapsed, stageDelay)
	}
	unwound := rec.Events[indexOfKind(rec, presentertest.KindUnwound)].Unwound
	if want := (presenter.Unwind{Undone: "reset 1 commit and deleted tag v0.0.1", Repo: "repo clean", Key: "unwind"}); unwound != want {
		t.Errorf("Unwound = %+v, want %+v", unwound, want)
	}
	fin := closingEvent(t, rec)
	if want := (presenter.Station{Name: "first release", Detail: "back where it started · nothing pushed"}); fin.Outcome != presenter.OutcomeUnwound || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeUnwound %+v", fin.Outcome, fin.End, want)
	}
}

func TestRelease_Rail_AnUnwindKeepingTheAnswersNamesThemApart(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAnsweredAbort(t, rec, preTagHookConfig)
	a.f.SeedSequence("git", ScriptedOut(""), gitFailing(commitStderr))
	a.f.Seed("gh", runner.Result{}, nil)
	a.unwound()

	_ = a.release(t.Context())

	unwound := rec.Events[indexOfKind(rec, presentertest.KindUnwound)].Unwound
	want := presenter.Unwind{Undone: "reset 2 commits", Kept: "kept the answers commit on the starting commit", Repo: "repo clean", Key: "unwind"}
	if unwound != want {
		t.Errorf("Unwound = %+v, want %+v", unwound, want)
	}
	if unwound.Text() != keptSummary {
		t.Errorf("Text() = %q, want plain's sentence %q", unwound.Text(), keptSummary)
	}
	fin := closingEvent(t, rec)
	if want := "kept the answers commit on the starting commit · nothing pushed"; fin.Outcome != presenter.OutcomeUnwound || fin.End.Detail != want {
		t.Errorf("close = %v %q, want OutcomeUnwound %q: the kept answers named, not back where it started", fin.Outcome, fin.End.Detail, want)
	}
}

func TestRelease_Rail_AWarningPastThePointOfNoReturnStillLandsAtTheTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npost_release = \"notify.sh\"\n")
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("sh", runner.Result{ExitCode: 1, Stdout: "notify boom\n"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	_ = engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	fin := closingEvent(t, rec)
	if fin.Outcome != presenter.OutcomeReleasedWithWarning {
		t.Errorf("Outcome = %v, want OutcomeReleasedWithWarning", fin.Outcome)
	}
	if want := (presenter.Station{Name: "v0.0.1", Detail: "released"}); fin.End != want {
		t.Errorf("End = %+v, want %+v", fin.End, want)
	}
	if keys := stageKeys(rec); !slices.Contains(keys, "post_release") {
		t.Errorf("stage keys = %q, want the post_release warning keyed to its node", keys)
	}
}

// slowTransport answers the exclusion check with none and fails every notes call,
// each after transportDelay, so the check and the notes stage have times to measure.
type slowTransport struct{}

const transportDelay = 20 * time.Millisecond

func (slowTransport) Generate(_ context.Context, prompt string) (string, error) {
	time.Sleep(transportDelay)
	if isCheckPrompt(prompt) {
		return "none", nil
	}
	if isClassifyPrompt(prompt) {
		return "OTHER", nil
	}
	return "", ai.ErrGenerationFailed
}

func TestRelease_Rail_TheCheckAndAFailedNotesStageAreTimedOnTheirOwnNodes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	rec := &presentertest.RecordingPresenter{}
	deps := railDeps(rec, f, root)
	deps.Transport = slowTransport{}
	deps.Yes = true

	_ = engine.Release(t.Context(), deps, priorTagNormalAIOptions())

	checked := rec.Events[indexOfKind(rec, presentertest.KindNotesChecked)].NotesCheck
	if checked.Key != "check" || checked.Elapsed < transportDelay {
		t.Errorf("NotesChecked = %+v, want the check node timed at least %v", checked, transportDelay)
	}
	if checking := rec.Events[indexOfKind(rec, presentertest.KindNotesChecking)].NotesCheck; checking.Key != "check" {
		t.Errorf("NotesChecking Key = %q, want check", checking.Key)
	}
	failure := onlyStageFailureEvent(t, rec)
	if failure.Key != "notes" || failure.Elapsed < 2*transportDelay {
		t.Errorf("notes failure = %+v, want the notes node timed from the stage's start", failure)
	}
	moves := nextMovesEvents(rec)
	if len(moves) != 1 {
		t.Fatalf("NextMoves = %+v, want one", moves)
	}
	assertFloorMovesLeadTheRail(t, moves[0].Moves, "mint release")
}

// assertFloorMovesLeadTheRail fails unless moves open on the floor's three ways out,
// each a capitalised lead ending in a colon and the command or config line apart from
// it, command the release run again; plain's sentence its own, naming the same command
// where it names one. Every move after them clears the cause: a capitalised lead
// alone, plain's lowercase sentence under it.
func assertFloorMovesLeadTheRail(t *testing.T, moves []presenter.NextMove, command string) {
	t.Helper()
	want := []presenter.NextMove{
		{
			Lead:     "Write the notes to a file, then release with it:",
			Command:  command + " --notes-file <file>",
			Sentence: "write the notes to a file and release with " + command + " --notes-file <file>",
		},
		{
			Lead:     "Release with the commit-subject list:",
			Command:  command + " --no-ai",
			Sentence: "re-run with --no-ai to release with the commit-subject list",
		},
		{
			Lead:     "Ship that list whenever the AI fails, under [release] in .mint.toml:",
			Command:  `on_notes_failure = "fallback"`,
			Sentence: `set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`,
		},
	}
	if len(moves) < len(want) || !slices.Equal(moves[:len(want)], want) {
		t.Fatalf("moves = %+v, want them opening on %+v", moves, want)
	}
	for _, move := range moves[len(want):] {
		if move.Command != "" || move.Lead != strings.ToUpper(move.Sentence[:1])+move.Sentence[1:] {
			t.Errorf("cause move = %+v, want a capitalised lead alone over plain's sentence", move)
		}
	}
}

// readGatesUnder scripts the prior-tag read gates as a run under opts reads them: the
// stash first under a real run's --autostash, no clean-tree check under a dry run's,
// and no on-branch probe under --any-branch.
func readGatesUnder(f *runner.FakeRunner, root string, opts engine.ReleaseOptions) {
	f.SeedSequence("git", ScriptedOut(root), ScriptedOut("origin/main"), ScriptedOut(priorTag+"\n"), ScriptedHead())
	if opts.AutoStash && !opts.DryRun {
		f.SeedSequence("git", stashSaved()) // stash push --include-untracked
	}
	f.SeedSequence("git", ScriptedOut("")) // fetch --tags
	if !opts.AutoStash || !opts.DryRun {
		f.SeedSequence("git", ScriptedOut("")) // status --porcelain
	}
	if !opts.AnyBranch {
		f.SeedSequence("git", ScriptedOut("main")) // rev-parse --abbrev-ref HEAD
	}
	f.SeedSequence("git", ScriptedNonZero(), ScriptedOut("0\t1"), ScriptedOut(""), ScriptedOut(startingSHA))
}

func TestRelease_TheFloorsCommandsCarryTheRunsOwnFlags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		opts    func(*engine.ReleaseOptions)
		command string
	}{
		{name: "a patch bump, the default, carries none", opts: func(*engine.ReleaseOptions) {}, command: "mint release"},
		{name: "a minor bump", opts: func(o *engine.ReleaseOptions) { o.Bump = version.BumpMinor }, command: "mint release -m"},
		{name: "a major bump", opts: func(o *engine.ReleaseOptions) { o.Bump = version.BumpMajor }, command: "mint release -M"},
		{name: "a set version, as typed", opts: func(o *engine.ReleaseOptions) { o.SetVersion = "v2.0.0" }, command: "mint release --set-version v2.0.0"},
		{name: "--any-branch", opts: func(o *engine.ReleaseOptions) { o.AnyBranch = true }, command: "mint release --any-branch"},
		{name: "--autostash", opts: func(o *engine.ReleaseOptions) { o.AutoStash = true }, command: "mint release --autostash"},
		{name: "a dry run", opts: func(o *engine.ReleaseOptions) { o.DryRun = true }, command: "mint release --dry-run"},
		{
			name: "all of them",
			opts: func(o *engine.ReleaseOptions) {
				o.Bump, o.AnyBranch, o.AutoStash, o.DryRun = version.BumpMinor, true, true, true
			},
			command: "mint release -m --any-branch --autostash --dry-run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := priorTagNormalAIOptions()
			tc.opts(&opts)
			root := t.TempDir()
			f := runner.NewFakeRunner()
			readGatesUnder(f, root, opts)
			seedNormalAINotes(f)
			if opts.AutoStash && !opts.DryRun {
				f.SeedSequence("git", ScriptedOut("")) // stash pop
			}
			rec := &presentertest.RecordingPresenter{}
			deps := railDeps(rec, f, root)
			deps.Transport = slowTransport{}
			deps.Yes = true

			_ = engine.Release(t.Context(), deps, opts)

			moves := nextMovesEvents(rec)
			if len(moves) != 1 {
				t.Fatalf("NextMoves = %+v, want one; kinds = %v", moves, rec.Kinds())
			}
			assertFloorMovesLeadTheRail(t, moves[0].Moves, tc.command)
		})
	}
}

func TestRelease_Rail_ADryRunPreviewsItsReviewQuietlyOnceTheNotesAreShown(t *testing.T) {
	t.Parallel()

	opts := priorTagNormalAIOptions()
	opts.DryRun = true
	rec, _ := runDryRunNormalAI(t, t.TempDir(), t.TempDir(), priorTagDiff, aiBody, opts)

	at := indexOfKind(rec, presentertest.KindPreviewGate)
	if at == -1 {
		t.Fatalf("no review previewed; kinds = %v", rec.Kinds())
	}
	review := presenter.ReleaseReviewGate()
	review.Stage, review.Quiet = "notes", true
	if got := rec.Events[at].PreviewGate; !reflect.DeepEqual(got, review) {
		t.Errorf("previewed = %+v, want the review a real run asks, Quiet %+v", got, review)
	}
	if shown := indexOfKind(rec, presentertest.KindShowNotes); shown == -1 || shown > at {
		t.Errorf("the review previewed at %d before the notes shown at %d; kinds = %v", at, shown, rec.Kinds())
	}
}

func TestRelease_Rail_TheExclusionPauseCarriesItsSuggestionAndTheAnswersTheirGlobs(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	if _, _, err := answeredRelease(t, rec, "", excludeThenKeepCalls, designAndFixturesAnswer, aiBody); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	pauses := pauseGates(rec)
	want := []presenter.ExclusionSuggestion{
		{Glob: "docs/design/", Reason: "design documents", Matches: "1 file, 176 B"},
		{Glob: "test/fixtures/", Reason: "test data", Matches: "1 file, 175 B"},
	}
	if len(pauses) != len(want) {
		t.Fatalf("pauses = %d, want %d", len(pauses), len(want))
	}
	for i, pause := range pauses {
		if pause.Stage != "check" || pause.Suggestion == nil || *pause.Suggestion != want[i] {
			t.Errorf("pause %d Stage/Suggestion = %q/%+v, want check/%+v", i, pause.Stage, pause.Suggestion, want[i])
		}
	}
	answers := rec.Events[indexOfKind(rec, presentertest.KindExclusionAnswersRecorded)].ExclusionAnswers
	if answers.Key != "check" || !reflect.DeepEqual(answers.Exclude, []string{"docs/design/"}) || !reflect.DeepEqual(answers.Keep, []string{"test/fixtures/"}) {
		t.Errorf("answers = %+v, want the check node's exclude and keep globs", answers)
	}
}

func TestRegenerateRun_Rail_ARegenerateStartsAtItsTagAndLandsRegenerated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## ["+regenRunVersionKey+"] - 2024-02-15\n\nStale body.\n")
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                    // status --porcelain (clean)
		ScriptedOut(regenRunReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                    // fetch --tags
		ScriptedOut("0\t1"),                // rev-list left-right count (ahead only)
		ScriptedOut("startHEAD"),           // rev-parse HEAD (capture clean start)
		ScriptedOut("2024-02-15"),          // for-each-ref creatordate:short (historical date)
		ScriptedOut(""),                    // add CHANGELOG.md
		ScriptedOut(""),                    // commit
		ScriptedOut(""),                    // push origin HEAD
	)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, true, nil)
	rec := &presentertest.RecordingPresenter{}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetBoth))
	req.ReleaseBranch = regenRunReleaseBranch

	if err := engine.RegenerateRun(t.Context(), railDeps(rec, f, dir), pub, dir, req); err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	info := runInfo(t, rec)
	if want := (presenter.Station{Name: regenRunTag, Detail: "fresh → CHANGELOG.md + provider release"}); info.Start != want {
		t.Errorf("Start = %+v, want %+v", info.Start, want)
	}
	if want := (presenter.Header{Command: "release regenerate", Where: "~/" + filepath.Base(dir), Version: "0.1.2"}); info.Header != want {
		t.Errorf("Header = %+v, want %+v", info.Header, want)
	}
	plan := rec.Events[indexOfKind(rec, presentertest.KindShowPlan)].ShowPlan
	wantStages := []presenter.PlanStage{
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
		{Key: "review", Name: "review", Detail: "the notes before they are written"},
		{Key: "record", Name: "CHANGELOG.md", Detail: "rewritten and committed"},
		{Key: "push", Name: "push", Detail: "HEAD → origin"},
		{Key: "publish", Name: "release", Detail: "provider release"},
	}
	if !slices.Equal(plan.Stages, wantStages) || plan.PointOfNoReturn != "push" {
		t.Errorf("plan = %+v / %q, want %+v / push", plan.Stages, plan.PointOfNoReturn, wantStages)
	}
	assertKeysInPlan(t, rec, plan)
	if got := stageSuccess(t, rec, "record"); got.Commit != "docs(changelog): regenerate notes for "+regenRunTag || !got.Quiet {
		t.Errorf("record success = %+v, want a quiet settle carrying the changelog commit's subject", got)
	}
	if got := stageSuccess(t, rec, "publish"); got.Detail != "written" || !got.Quiet {
		t.Errorf("publish success = %+v, want a quiet settle", got)
	}
	if gates := promptGates(rec); len(gates) != 1 || gates[0].Stage != "review" {
		t.Errorf("questions = %+v, want the one review question on the review node", gates)
	}
	fin := closingEvent(t, rec)
	if want := (presenter.Station{Name: regenRunTag, Detail: "regenerated"}); fin.Outcome != presenter.OutcomeRegenerated || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeRegenerated %+v", fin.Outcome, fin.End, want)
	}
}

func TestRegenerateRun_Rail_AFailedProviderWriteClosesClaimingNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                    // status --porcelain (clean)
		ScriptedOut(regenRunReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                    // fetch --tags
		ScriptedOut("0\t1"),                // rev-list left-right count (ahead only)
	)
	f.Seed("gh", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenRunTag, false, errors.New("gh: HTTP 502"))
	rec := &presentertest.RecordingPresenter{}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetRelease))
	req.ReleaseBranch = regenRunReleaseBranch

	err := engine.RegenerateRun(t.Context(), railDeps(rec, f, dir), pub, dir, req)

	assertAbortNonZero(t, err)
	fin := closingEvent(t, rec)
	if want := (presenter.Station{Name: regenRunTag, Detail: "nothing committed · nothing pushed"}); fin.Outcome != presenter.OutcomeFailed || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeFailed %+v: the provider write ran, so it claims nothing", fin.Outcome, fin.End, want)
	}
}

// regenerateChangelogRun runs a changelog-only regenerate of a section reading stale,
// its git calls from the write's clean start on scripted by tail.
func regenerateChangelogRun(t *testing.T, stale string, tail ...runner.ScriptedCall) (*presentertest.RecordingPresenter, error) {
	t.Helper()
	return regenerateChangelogIn(t, staleChangelogDir(t, stale), tail...)
}

// staleChangelogDir is a repo whose CHANGELOG.md holds the section reading stale.
func staleChangelogDir(t *testing.T, stale string) string {
	t.Helper()
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## ["+regenRunVersionKey+"] - 2024-02-15\n\n"+stale)
	return dir
}

// regenerateChangelogIn is regenerateChangelogRun over the repo at dir.
func regenerateChangelogIn(t *testing.T, dir string, tail ...runner.ScriptedCall) (*presentertest.RecordingPresenter, error) {
	t.Helper()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(""),                    // status --porcelain (clean)
		ScriptedOut(regenRunReleaseBranch), // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedOut(""),                    // fetch --tags
		ScriptedOut("0\t1"),                // rev-list left-right count (ahead only)
		ScriptedOut("startHEAD"),           // rev-parse HEAD (capture clean start)
		ScriptedOut("2024-02-15"),          // for-each-ref creatordate:short (historical date)
	)
	f.SeedSequence("git", tail...)
	rec := &presentertest.RecordingPresenter{}
	req := runReq(engine.SourceOf(engine.RegenerateSourceFresh), engine.TargetOf(engine.RegenerateTargetChangelog))
	req.ReleaseBranch = regenRunReleaseBranch

	err := engine.RegenerateRun(t.Context(), railDeps(rec, f, dir), nil, dir, req)
	return rec, err
}

func TestRegenerateRun_Rail_AFailedRegenerateClosesSayingWhatItLeft(t *testing.T) {
	t.Parallel()

	rejected := runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}
	tests := []struct {
		name string
		tail []runner.ScriptedCall
		end  string
	}{
		{
			name: "a commit that fails leaves the rewrite",
			tail: []runner.ScriptedCall{ScriptedOut(""), gitFailing(commitStderr)}, // add, commit
			end:  "CHANGELOG.md written, not committed · nothing pushed",
		},
		{
			name: "a rejected push resets the commit",
			tail: []runner.ScriptedCall{ScriptedOut(""), ScriptedOut(""), rejected, ScriptedOut("")}, // add, commit, push, reset --hard
			end:  "changelog commit reset · back where it started · nothing pushed",
		},
		{
			name: "a rejected push whose reset fails needs cleaning up",
			tail: []runner.ScriptedCall{ScriptedOut(""), ScriptedOut(""), rejected, scriptedMutateFailure()},
			end:  "manual cleanup required · nothing pushed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec, err := regenerateChangelogRun(t, "Stale body.\n", tt.tail...)

			assertAbortNonZero(t, err)
			fin := closingEvent(t, rec)
			if want := (presenter.Station{Name: regenRunTag, Detail: tt.end}); fin.Outcome != presenter.OutcomeFailed || fin.End != want {
				t.Errorf("close = %v %+v, want OutcomeFailed %+v", fin.Outcome, fin.End, want)
			}
		})
	}
}

func TestRegenerateRun_Rail_AChangelogWriteThatFailsClosesClaimingNothingCommitted(t *testing.T) {
	t.Parallel()

	dir := staleChangelogDir(t, "Stale body.\n")
	makeReadOnly(t, dir)

	rec, err := regenerateChangelogIn(t, dir)

	assertAbortNonZero(t, err)
	if fin := closingEvent(t, rec); fin.Outcome != presenter.OutcomeFailed || fin.End.Detail != "nothing committed · nothing pushed" {
		t.Errorf("close = %v %+v, want OutcomeFailed with nothing committed and nothing pushed", fin.Outcome, fin.End)
	}
}

func TestRegenerateRun_Rail_AnUnchangedChangelogSettlesThePushAsSkipped(t *testing.T) {
	t.Parallel()

	// The section as the writer lays it out: the body, then a blank line.
	rec, err := regenerateChangelogRun(t, regenRunFreshBody+"\n")
	if err != nil {
		t.Fatalf("RegenerateRun returned unexpected error: %v", err)
	}

	if got := stageSuccess(t, rec, "record"); got.Detail != "unchanged" || !got.Quiet {
		t.Errorf("record success = %+v, want a quiet unchanged settle", got)
	}
	if got := stageSuccess(t, rec, "push"); got.Detail != "skipped: nothing to push" || !got.Quiet {
		t.Errorf("push success = %+v, want a quiet settle saying why it was skipped", got)
	}
	assertKeysInPlan(t, rec, rec.Events[indexOfKind(rec, presentertest.KindShowPlan)].ShowPlan)
	assertEveryPlannedStageSettled(t, rec)
}

func TestRelease_Rail_NotesNothingGeneratedNameTheirSourceOnTheRowAndKeepPlainsDetail(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		seed func(*runner.FakeRunner)
		opts func(*engine.ReleaseOptions)
		row  string
	}{
		{
			name: "an empty diff's stub",
			seed: func(f *runner.FakeRunner) { f.SeedSequence("git", ScriptedOut("")) }, // diff v1.2.3..HEAD
			opts: func(*engine.ReleaseOptions) {},
			row:  "the stub for a release with no source changes",
		},
		{
			name: "--no-ai",
			seed: func(f *runner.FakeRunner) {
				f.SeedSequence("git",
					ScriptedOut(priorTagDiff),   // diff v1.2.3..HEAD
					ScriptedOut(commitSubjects), // log --format=%s v1.2.3..HEAD
				)
			},
			opts: func(o *engine.ReleaseOptions) { o.NoAI = true },
			row:  "the commit-subject list",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			tc.seed(f)
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}
			opts := priorTagNormalAIOptions()
			tc.opts(&opts)

			releaseOrFail(t, newDeps(rec, f), opts)

			if got := stageSuccess(t, rec, "notes"); got.Detail != "generated" || got.Row != tc.row {
				t.Errorf("notes success = %+v, want plain's %q and the rail's row %q", got, "generated", tc.row)
			}
		})
	}
}

func TestRelease_Rail_ABodyTakenAtTheNotesFloorSettlesTheNotesAgainSayingWhereItCameFrom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		choice presenter.Choice
		seed   func(*runner.FakeRunner, *engine.ReleaseDeps)
		detail string
	}{
		{
			name:   "the commit subjects",
			choice: presenter.ChoiceCommitSubjects,
			seed: func(f *runner.FakeRunner, _ *engine.ReleaseDeps) {
				f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s v1.2.3..HEAD
			},
			detail: "the commit-subject list",
		},
		{
			name:   "written in the editor",
			choice: presenter.ChoiceEdit,
			seed: func(_ *runner.FakeRunner, deps *engine.ReleaseDeps) {
				deps.Editor = &fakeEditor{edited: editedBody}
			},
			detail: "written in your editor",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f, rec, deps := singlePassFailingRun(t, root, &ai.GenerationError{Stdout: loginOutput, ExitCode: 1})
			tc.seed(f, &deps)
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			rec.NextChoices = []presenter.Choice{presenter.ChoiceYes, tc.choice, presenter.ChoiceYes}

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			failed, shown := indexOfKind(rec, presentertest.KindStageFailed), indexOfKind(rec, presentertest.KindShowNotes)
			settled := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
				return ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Key == "notes"
			})
			if settled < failed || settled > shown {
				t.Fatalf("notes settled at %d, want between the failure at %d and the notes shown at %d; kinds = %v", settled, failed, shown, rec.Kinds())
			}
			if got, want := rec.Events[settled].StageSucceeded, presenter.Settled("notes", tc.detail); got != want {
				t.Errorf("notes settled = %+v, want %+v", got, want)
			}
		})
	}
}
