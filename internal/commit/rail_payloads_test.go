package commit_test

// This file pins the data a commit puts in its presenter payloads for the rail to
// draw: the header and the start station, the run's stages, the settles that fill
// them, and the end station every run closes at.

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// railCommit runs a commit over r from a repo directly under the home directory, by
// a stamped mint, answering its question with choice.
func railCommit(t *testing.T, r *runner.FakeRunner, choice presenter.Choice, configure func(*commit.Deps)) (*presentertest.RecordingPresenter, string, error) {
	t.Helper()
	root := t.TempDir()
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{choice}}
	deps := newCommitDeps(rec, r, scriptedTransport("feat: rail payloads"), root)
	deps.Home = filepath.Dir(root)
	deps.MintVersion = "0.1.2"
	configure(&deps)
	return rec, root, commit.Run(context.Background(), deps)
}

func noFlags(*commit.Deps) {}

// railEvent returns the one event of kind rec holds.
func railEvent(t *testing.T, rec *presentertest.RecordingPresenter, kind presentertest.EventKind) presentertest.Event {
	t.Helper()
	var found []presentertest.Event
	for _, ev := range rec.Events {
		if ev.Kind == kind {
			found = append(found, ev)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%v events = %d, want one; kinds = %v", kind, len(found), rec.Kinds())
	}
	return found[0]
}

// closingResult returns the run's RunFinished, failing unless it was the last event.
func closingResult(t *testing.T, rec *presentertest.RecordingPresenter) presenter.RunResult {
	t.Helper()
	fin := railEvent(t, rec, presentertest.KindRunFinished)
	if last := rec.Events[len(rec.Events)-1]; last.Kind != presentertest.KindRunFinished {
		t.Errorf("last event = %v, want the closing RunFinished; kinds = %v", last.Kind, rec.Kinds())
	}
	return fin.RunFinished
}

// settles returns each success's key and detail, in order.
func settles(rec *presentertest.RecordingPresenter) []presenter.StageSuccess {
	var out []presenter.StageSuccess
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded {
			success := ev.StageSucceeded
			success.Elapsed = 0
			out = append(out, success)
		}
	}
	return out
}

func TestRun_Rail_ACommitStartsAtTheBranchAndLandsAtTheNewCommit(t *testing.T) {
	t.Parallel()

	rec, root, err := railCommit(t, seedDiffThenCommit("diff --git a/x b/x\n+work"), presenter.ChoiceYes, noFlags)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	info := railEvent(t, rec, presentertest.KindRunStarted).RunStarted
	if want := (presenter.Header{Command: "commit", Where: "~/" + filepath.Base(root), Version: "0.1.2"}); info.Header != want {
		t.Errorf("Header = %+v, want %+v", info.Header, want)
	}
	if want := (presenter.Station{Name: "main", Detail: "4f9c2e1"}); info.Start != want {
		t.Errorf("Start = %+v, want %+v (the branch and HEAD's short sha)", info.Start, want)
	}
	fin := closingResult(t, rec)
	if want := (presenter.Station{Name: "main", Detail: endHeadSHA}); fin.Outcome != presenter.OutcomeCommitted || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeCommitted %+v", fin.Outcome, fin.End, want)
	}
	if fin.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want the run's time", fin.Elapsed)
	}
}

func TestRun_Rail_TheQuietPlanAndItsSettlesFillEveryNode(t *testing.T) {
	t.Parallel()

	rec, _, err := railCommit(t, seedDiffThenCommit("diff --git a/x b/x\n+work"), presenter.ChoiceYes, noFlags)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	plan := railEvent(t, rec, presentertest.KindShowPlan).ShowPlan
	want := []presenter.PlanStage{
		{Key: "diff", Name: "diff", Detail: "staged changes"},
		{Key: "message", Name: "message", Detail: "written by the AI"},
		{Key: "commit", Name: "commit", Detail: "on main"},
	}
	if !plan.Quiet || plan.PointOfNoReturn != "" || !slices.Equal(plan.Stages, want) {
		t.Errorf("plan = %+v, want a quiet plan of %+v with no point of no return", plan, want)
	}
	var keys []string
	for _, s := range settles(rec) {
		keys = append(keys, s.Key)
	}
	if wantKeys := []string{"diff", "message", "commit"}; !slices.Equal(keys, wantKeys) {
		t.Errorf("settled keys = %q, want %q", keys, wantKeys)
	}
	if got := settles(rec)[2]; got != presenter.Settled("commit", endHeadSHA) {
		t.Errorf("commit settle = %+v, want the new commit's short sha", got)
	}
	if gate := railEvent(t, rec, presentertest.KindPrompt).Prompt; gate.Stage != "message" {
		t.Errorf("question Stage = %q, want the message node", gate.Stage)
	}
}

func TestRun_Rail_StagingAndAPushAddTheirNodesAndAPushLandsPushed(t *testing.T) {
	t.Parallel()

	r := seedAddAllModeThenStageAndCommit("diff --git a/x b/x\n+work")
	r.SeedSequence("git",
		runner.ScriptedCall{}, // git push
		runner.ScriptedCall{Result: runner.Result{Stdout: "upstream/main\n"}}, // git rev-parse --abbrev-ref @{push}
	)
	rec, _, err := railCommit(t, r, presenter.ChoiceYes, func(d *commit.Deps) {
		d.Staging = commit.AddAll
		d.Push = true
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	plan := railEvent(t, rec, presentertest.KindShowPlan).ShowPlan
	want := []presenter.PlanStage{
		{Key: "diff", Name: "diff", Detail: "everything, staged when you accept"},
		{Key: "message", Name: "message", Detail: "written by the AI"},
		{Key: "stage", Name: "stage", Detail: "git add -A"},
		{Key: "commit", Name: "commit", Detail: "on main"},
		{Key: "push", Name: "push", Detail: "to its upstream"},
	}
	if !slices.Equal(plan.Stages, want) {
		t.Errorf("plan stages = %+v, want %+v", plan.Stages, want)
	}
	got := settles(rec)
	wantTail := []presenter.StageSuccess{
		presenter.Settled("stage", "staged"),
		presenter.Settled("commit", endHeadSHA),
		{Name: "push", Key: "push", Detail: "pushed to upstream/main", Blocking: true, Quiet: true},
	}
	if len(got) < len(wantTail) || !slices.Equal(got[len(got)-len(wantTail):], wantTail) {
		t.Errorf("settles = %+v, want them to end %+v", got, wantTail)
	}
	if fin := closingResult(t, rec); fin.End != (presenter.Station{Name: "main", Detail: endHeadSHA + " · pushed"}) {
		t.Errorf("End = %+v, want the new commit, pushed", fin.End)
	}
}

func TestRun_Rail_AFailedPushStillLandsAtTheCommitWithAWarning(t *testing.T) {
	t.Parallel()

	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", "fatal: no upstream\n")
	rec, _, err := railCommit(t, r, presenter.ChoiceYes, func(d *commit.Deps) { d.Push = true })
	if err == nil {
		t.Fatal("Run returned nil, want the push failure's exit")
	}

	fin := closingResult(t, rec)
	if want := (presenter.Station{Name: "main", Detail: endHeadSHA}); fin.Outcome != presenter.OutcomeCommittedWithWarning || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeCommittedWithWarning %+v", fin.Outcome, fin.End, want)
	}
	if warn := railEvent(t, rec, presentertest.KindWarn).Warn; warn.Key != "push" {
		t.Errorf("warning Key = %q, want the push node", warn.Key)
	}
}

func TestRun_Rail_ADeclinedCommitClosesAtTheStartHavingChangedNothing(t *testing.T) {
	t.Parallel()

	rec, _, err := railCommit(t, seedDiffThenCommit("diff --git a/x b/x\n+work"), presenter.ChoiceNo, noFlags)
	if err == nil {
		t.Fatal("Run returned nil, want the decline's exit")
	}

	fin := closingResult(t, rec)
	if want := (presenter.Station{Name: "main", Detail: "nothing changed"}); fin.Outcome != presenter.OutcomeDeclined || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeDeclined %+v", fin.Outcome, fin.End, want)
	}
}

func TestRun_Rail_AFailedPreflightIsTheDiffNode(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		scriptedHead(),        // git rev-parse HEAD --abbrev-ref HEAD (the start station)
		runner.ScriptedCall{}, // git diff --cached --numstat (empty index)
		runner.ScriptedCall{}, // git status --porcelain (clean tree)
	)
	rec, _, err := railCommit(t, r, presenter.ChoiceYes, noFlags)
	if err == nil {
		t.Fatal("Run returned nil, want the empty index's exit")
	}

	if failure := railEvent(t, rec, presentertest.KindStageFailed).StageFailed; failure.Name != "preflight" || failure.Key != "diff" {
		t.Errorf("failure Name/Key = %q/%q, want the preflight label on the diff node", failure.Name, failure.Key)
	}
	if fin := closingResult(t, rec); fin.Outcome != presenter.OutcomeFailed || fin.End != (presenter.Station{Name: "main", Detail: "nothing changed"}) {
		t.Errorf("close = %v %+v, want OutcomeFailed back at main with nothing changed", fin.Outcome, fin.End)
	}
}

func TestRun_Rail_AFailedAcceptClosesClaimingOnlyWhatItStaged(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/x b/x\n+work"
	failed := runner.ScriptedCall{Result: runner.Result{Stderr: "error: gpg failed to sign the data\n"}, Err: errExitOne}
	tests := []struct {
		name    string
		staging commit.StagingMode
		git     []runner.ScriptedCall
		detail  string
	}{
		{
			name:    "a staging that fails",
			staging: commit.AddAll,
			git: []runner.ScriptedCall{
				{Result: runner.Result{Stdout: "x\n"}}, // git diff HEAD --numstat (non-empty)
				{},                                     // git ls-files --others (preflight: no untracked)
				{Result: runner.Result{Stdout: diff}},  // git diff HEAD -- .
				{},                                     // git ls-files --others (no untracked)
				failed,                                 // git add -A
			},
			detail: "nothing committed",
		},
		{
			name:    "a commit failing after its staging",
			staging: commit.AddAll,
			git: []runner.ScriptedCall{
				{Result: runner.Result{Stdout: "x\n"}}, // git diff HEAD --numstat (non-empty)
				{},                                     // git ls-files --others (preflight: no untracked)
				{Result: runner.Result{Stdout: diff}},  // git diff HEAD -- .
				{},                                     // git ls-files --others (no untracked)
				{},                                     // git add -A
				failed,                                 // git commit -F -
			},
			detail: "changes staged, not committed",
		},
		{
			name:    "a commit of the staged changes failing, its hooks run",
			staging: commit.StagedOnly,
			git: []runner.ScriptedCall{
				{Result: runner.Result{Stdout: "x\n"}}, // git diff --cached --numstat (non-empty index)
				{Result: runner.Result{Stdout: diff}},  // git diff --cached
				failed,                                 // git commit -F -
			},
			detail: "nothing committed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.SeedSequence("git", scriptedHead()) // git rev-parse HEAD --abbrev-ref HEAD (the start station)
			r.SeedSequence("git", tt.git...)
			rec, _, err := railCommit(t, r, presenter.ChoiceYes, func(d *commit.Deps) { d.Staging = tt.staging })
			if err == nil {
				t.Fatal("Run returned nil, want the failure's exit")
			}

			fin := closingResult(t, rec)
			if want := (presenter.Station{Name: "main", Detail: tt.detail}); fin.Outcome != presenter.OutcomeFailed || fin.End != want {
				t.Errorf("close = %v %+v, want OutcomeFailed %+v", fin.Outcome, fin.End, want)
			}
		})
	}
}

func TestRun_Rail_InputClosingAtTheQuestionClosesAsAFailureNotADecline(t *testing.T) {
	t.Parallel()

	rec, _, err := railCommit(t, seedDiffThenCommit("diff --git a/x b/x\n+work"), presenter.ChoiceYes, func(d *commit.Deps) {
		d.Presenter.(*presentertest.RecordingPresenter).PromptResult = func(presenter.Gate) (presenter.Choice, error) {
			return "", presenter.ErrInputClosed
		}
	})
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Fatalf("Run returned %v, want ErrInputClosed in the chain", err)
	}

	fin := closingResult(t, rec)
	if want := (presenter.Station{Name: "main", Detail: "nothing changed"}); fin.Outcome != presenter.OutcomeFailed || fin.End != want {
		t.Errorf("close = %v %+v, want OutcomeFailed %+v", fin.Outcome, fin.End, want)
	}
}

func TestRun_Rail_ABranchWithNoCommitsYetStartsAndLandsOnTheBranch(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stdout: "HEAD\n"}, Err: errors.New("exit status 128")}, // git rev-parse HEAD --abbrev-ref HEAD
		runner.ScriptedCall{Result: runner.Result{Stdout: "main\n"}},                                                    // git symbolic-ref --short -q HEAD
		runner.ScriptedCall{Result: runner.Result{Stdout: "x\n"}},                                                       // git diff --cached --numstat (non-empty index)
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}},                                 // git diff --cached
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),   // git rev-parse --short HEAD (the end station)
	)
	rec, _, err := railCommit(t, r, presenter.ChoiceYes, noFlags)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if start, want := railEvent(t, rec, presentertest.KindRunStarted).RunStarted.Start, (presenter.Station{Name: "main", Detail: "no commits yet"}); start != want {
		t.Errorf("Start = %+v, want %+v", start, want)
	}
	plan := railEvent(t, rec, presentertest.KindShowPlan).ShowPlan
	if got := plan.Stages[len(plan.Stages)-1]; got != (presenter.PlanStage{Key: "commit", Name: "commit", Detail: "on main"}) {
		t.Errorf("commit stage = %+v, want it on main", got)
	}
	if end, want := closingResult(t, rec).End, (presenter.Station{Name: "main", Detail: endHeadSHA}); end != want {
		t.Errorf("End = %+v, want %+v", end, want)
	}
}

func TestRun_Rail_TheDiffNodeCountsWhatTheCommitWillCarry(t *testing.T) {
	t.Parallel()

	const diff = "diff --git a/x b/x\n+work"
	tests := []struct {
		name    string
		staging commit.StagingMode
		reads   []runner.ScriptedCall
		want    string
	}{
		{
			name:    "the staged changes, every file and line counted",
			staging: commit.StagedOnly,
			reads: []runner.ScriptedCall{
				scriptedStdout("35\t9\tsrc/deliver.go\n21\t3\tsrc/queue.go\n"), // diff --cached --numstat
				scriptedStdout(diff), // diff --cached
			},
			want: "2 files · +56 −12 · staged changes",
		},
		{
			name:    "a rename one file, a binary file none of the lines",
			staging: commit.All,
			reads: []runner.ScriptedCall{
				scriptedStdout("4\t1\tsrc/{old.go => new.go}\n-\t-\tlogo.png\n"), // diff HEAD --numstat
				scriptedStdout(diff), // diff HEAD
			},
			want: "2 files · +4 −1 · tracked changes, staged when you accept",
		},
		{
			name:    "everything, an untracked file counted as -A stages it, excluded from the AI or not",
			staging: commit.AddAll,
			reads: []runner.ScriptedCall{
				scriptedStdout("2\t0\tREADME.md\n"), // diff HEAD --numstat
				scriptedStdout("new.go\x00"),        // ls-files --others
				scriptedStdout("7\t0\tnew.go\n"),    // diff --no-index --numstat -- /dev/null new.go
				scriptedStdout(diff),                // diff HEAD
				scriptedStdout(""),                  // ls-files --others, the AI's view
			},
			want: "2 files · +9 −0 · everything, staged when you accept",
		},
		{
			name:    "a single file",
			staging: commit.StagedOnly,
			reads: []runner.ScriptedCall{
				scriptedStdout("1\t0\tREADME.md\n"), // diff --cached --numstat
				scriptedStdout(diff),                // diff --cached
			},
			want: "1 file · +1 −0 · staged changes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.SeedSequence("git", scriptedHead())
			r.SeedSequence("git", tt.reads...)
			rec, _, err := railCommit(t, r, presenter.ChoiceNo, func(d *commit.Deps) { d.Staging = tt.staging })
			if err == nil {
				t.Fatal("Run returned nil, want the decline's exit")
			}

			if got := settles(rec)[0]; got != presenter.Settled("diff", tt.want) {
				t.Errorf("diff settle = %+v, want the count %q", got, tt.want)
			}
		})
	}
}

func TestRun_Rail_APushIsTimedAndNamesWhereItWent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		target runner.ScriptedCall
		want   string
	}{
		{name: "the remote-tracking branch git names", target: scriptedStdout("upstream/main\n"), want: "pushed to upstream/main"},
		{name: "its upstream where git names none", target: runner.ScriptedCall{Result: runner.Result{ExitCode: 128}, Err: errors.New("exit status 128")}, want: "pushed to its upstream"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := seedDiffThenCommitThenPush("diff --git a/x b/x\n+work")
			r.SeedSequence("git", tt.target) // rev-parse --abbrev-ref @{push}
			rec, _, err := railCommit(t, r, presenter.ChoiceYes, func(d *commit.Deps) {
				d.Push = true
				d.Clock = (&steppingClock{now: runEpoch, step: 1200 * time.Millisecond}).Now
			})
			if err != nil {
				t.Fatalf("Run returned unexpected error: %v", err)
			}

			if want := (presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing to its upstream…", Quiet: true}); keyed(rec, presentertest.KindStageStarted, "push").StageStarted != want {
				t.Errorf("push start = %+v, want %+v, Quiet as plain has no line for it", keyed(rec, presentertest.KindStageStarted, "push").StageStarted, want)
			}
			if want := (presenter.StageSuccess{Name: "push", Key: "push", Detail: tt.want, Elapsed: 1200 * time.Millisecond, Blocking: true, Quiet: true}); keyed(rec, presentertest.KindStageSucceeded, "push").StageSucceeded != want {
				t.Errorf("push success = %+v, want %+v", keyed(rec, presentertest.KindStageSucceeded, "push").StageSucceeded, want)
			}
		})
	}
}

func TestRun_Rail_AFailedPushSaysWhatHappenedAndSendsItsRemedyToNext(t *testing.T) {
	t.Parallel()

	r := seedDiffThenCommitThenFailedPush("diff --git a/x b/x\n+work", "fatal: no upstream\n")
	rec, _, err := railCommit(t, r, presenter.ChoiceYes, func(d *commit.Deps) {
		d.Push = true
		d.Clock = (&steppingClock{now: runEpoch, step: 1200 * time.Millisecond}).Now
	})
	if err == nil {
		t.Fatal("Run returned nil, want the push failure's exit")
	}

	want := presenter.Warning{
		Label:   "push",
		Message: "commit is in place; re-run the push to retry",
		Key:     "push",
		Row:     "not pushed; the commit is in place",
		Output:  "fatal: no upstream\n",
		Elapsed: 1200 * time.Millisecond,
	}
	if warn := railEvent(t, rec, presentertest.KindWarn).Warn; warn != want {
		t.Errorf("warning = %+v, want %+v", warn, want)
	}
	moves := railEvent(t, rec, presentertest.KindNextMoves).NextMoves
	if wantMoves := []presenter.NextMove{{Lead: "Push the commit:", Command: "git push"}}; !moves.Quiet || moves.Stage != "push" || !slices.Equal(moves.Moves, wantMoves) {
		t.Errorf("next moves = %+v, want the remedy plain's sentence names, Quiet, %+v", moves, wantMoves)
	}
}

func TestRun_Rail_AnAIFailureEndsTheMessageNodeInTheTimeItRan(t *testing.T) {
	t.Parallel()

	er := &editorRunner{fake: seedAIFailFallback("myedit"), saved: "feat: written by hand\n"}
	rec := &presentertest.RecordingPresenter{}
	deps := aiFailDeps(rec, er, &failTransport{err: &ai.GenerationError{Stdout: "usage limit reached", ExitCode: 1}}, commit.StagedOnly, t.TempDir())
	deps.Clock = (&steppingClock{now: runEpoch, step: 1200 * time.Millisecond}).Now
	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if warn := railEvent(t, rec, presentertest.KindWarn).Warn; warn.Key != "message" || warn.Elapsed != 1200*time.Millisecond {
		t.Errorf("warning Key/Elapsed = %q/%v, want the message node's, timed at 1.2s", warn.Key, warn.Elapsed)
	}
}

// keyed is the one event of kind rec holds for the stage keyed key.
func keyed(rec *presentertest.RecordingPresenter, kind presentertest.EventKind, key string) presentertest.Event {
	var found presentertest.Event
	for _, ev := range rec.Events {
		if ev.Kind == kind && (ev.StageStarted.Key == key || ev.StageSucceeded.Key == key) {
			found = ev
		}
	}
	return found
}

// scriptedStdout is a call writing stdout.
func scriptedStdout(stdout string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{Stdout: stdout}}
}
