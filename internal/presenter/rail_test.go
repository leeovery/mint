package presenter_test

import (
	"strings"
	"testing"

	"github.com/muesli/termenv"
	"github.com/rivo/uniseg"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// row is a row of an 80-column layout: left, then right ending on its last column.
func row(left, right string) string {
	return left + strings.Repeat(" ", 80-uniseg.StringWidth(left)-uniseg.StringWidth(right)) + right
}

// commitRun opens a commit on main at 7c1e0b4 with its plan laid out.
func commitRun(p presenter.Presenter) {
	p.RunStarted(presenter.RunInfo{
		Header: presenter.Header{Command: "commit", Where: pigeonWhere, Version: mintVersion},
		Start:  presenter.Station{Name: "main", Detail: "7c1e0b4"},
	})
	p.ShowPlan(presenter.Plan{Quiet: true, PointOfNoReturn: "push", Stages: []presenter.PlanStage{
		{Key: "message", Name: "message", Detail: "written by the AI"},
		{Key: "commit", Name: "commit", Detail: "on main"},
		{Key: "push", Name: "push", Detail: "to its upstream"},
	}})
}

// commitHead is the commit's header and start station, the rail under it solid once
// the run has reached a node.
var commitHead = []string{
	row(" mint ─ commit", "~/Code/pigeon · v0.1.2"),
	"",
	" ◇  main          7c1e0b4",
	" │",
}

// commitWaiting is commitHead with nothing reached yet, the rail under it dotted.
var commitWaiting = append(commitHead[:3:3], " ┆")

func TestRail_ThePlanWaitsBelowTheRunHollowWithThePointOfNoReturnAndTheEndStation(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	wantScreen(t, r, append(commitWaiting,
		" ○  message       written by the AI",
		" ○  commit        on main",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ○  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)
}

func TestRail_APlanOfStepsAloneDrawsNothing(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	r.RunStarted(releaseStarted(false))
	before := r.writes.count()
	r.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{{Verb: "tag", Target: "v1.4.0"}}})
	if r.writes.count() != before {
		t.Error("a plan with no stages wrote to the terminal, want nothing")
	}
}

func TestRail_StagesFillTheirNodesByKey(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)

	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing the commit message…"})
	r.clock.Advance(secs(2.5))
	r.ticks.tick(t)
	wantScreen(t, r, append(commitHead,
		row(" ⠙  message       writing the commit message…", "2s"),
		" ┆",
		" ○  commit        on main",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ○  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)

	r.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Blocking: true, Elapsed: secs(2.54), Detail: "written"})
	r.StageSucceeded(presenter.StageSuccess{Name: "commit", Key: "commit", Detail: "a3f9c21", Commit: "feat: retry deliveries"})
	r.StageStarted(presenter.StageStart{Name: "push", Key: "push"})
	wantScreen(t, r, append(commitHead,
		row(" ●  message       written", "2.5s"),
		" ●  commit        a3f9c21",
		" │                committed as feat: retry deliveries",
		" │",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ⠋  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)
}

func TestRail_AQuietSettleSettlesItsNode(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageSucceeded(presenter.Settled("message", "written in your editor"))
	if got := r.screen(t)[4]; got != " ●  message       written in your editor" {
		t.Errorf("the message node reads %q, want it settled by the quiet success", got)
	}
}

func TestRail_AFailedStageShowsItsOutputBehindAGutterOfItsOwnAndTheSavedNotes(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageFailed(presenter.StageFailure{
		Name: "generate", Key: "message", Message: "AI call failed", Elapsed: secs(3),
		Output: "Not logged in\n\tPlease run /login", SavedNotes: "/tmp/v1.4.0.md",
	})
	wantScreen(t, r, append(commitHead,
		row(" ✗  message       AI call failed", "3.0s"),
		" │                │ Not logged in",
		" │                │         Please run /login",
		" │                part notes saved to",
		" │                /tmp/v1.4.0.md",
		" ┆",
		" ○  commit        on main",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ○  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)
	if got, want := r.err.String(), "✗ generate  AI call failed\n"; got != want {
		t.Errorf("err = %q, want the one-line summary %q", got, want)
	}
}

func TestRail_AWarningEndsTheStageItNamesOrStandsAsANodeOfItsOwn(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.Warn(presenter.Warning{Label: "editor", Message: "could not launch editor"})
	r.StageSucceeded(presenter.Settled("message", "written"))
	r.StageSucceeded(presenter.Settled("commit", "a3f9c21"))
	r.Warn(presenter.Warning{Key: "push", Label: "push", Message: "commit is in place; re-run the push to retry", Output: " ! [rejected] main -> main"})
	wantScreen(t, r, append(commitHead,
		" !  editor        could not launch editor",
		" ●  message       written",
		" ●  commit        a3f9c21",
		" │",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" !  push          commit is in place; re-run the push to retry",
		" │                │  ! [rejected] main -> main",
		" ┆",
		" ◇  main",
	)...)
	if got, want := r.err.String(), "⚠ editor  could not launch editor\n⚠ push  commit is in place; re-run the push to retry\n"; got != want {
		t.Errorf("err = %q, want each warning's summary %q", got, want)
	}
}

func TestRail_AStageThatGoesOnAfterAWarningDrawsANodeUnderIt(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true})
	r.Warn(presenter.Warning{Key: "message", Label: "diff too large", Message: "opening the editor"})
	r.StageSucceeded(presenter.Settled("message", "written in your editor"))
	got := r.screen(t)[4:6]
	want := []string{" !  message       opening the editor", " ●  message       written in your editor"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the message rows read\n%s\nwant the warning kept and the outcome under it\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRail_TheUnwindNamesWhatItUndidThenWhatItKept(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.Unwound(presenter.Unwind{Undone: "reset 2 commits", Kept: "kept the answers commit on the starting commit", Repo: "repo clean"})
	got := r.screen(t)[4:7]
	want := []string{
		" ↩  unwind        reset 2 commits",
		" │                kept the answers commit on the starting commit",
		" │                repo clean",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the unwind reads\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRail_TheExclusionCheckRunsAsANodeOfItsOwnAheadOfTheNotes(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	r.RunStarted(releaseStarted(false))
	r.ShowPlan(presenter.Plan{Stages: []presenter.PlanStage{
		{Key: "check", Name: "check", Detail: "files that don't belong in the notes"},
		{Key: "notes", Name: "notes", Detail: "written by the AI"},
	}})
	r.StageStarted(notesStarted())
	r.NotesChecking(presenter.NotesCheck{Key: "check"})
	if got := r.screen(t)[4:7]; !strings.HasPrefix(got[0], " ⠋  check         files that don't belong in the notes") || got[2] != " ○  notes         written by the AI" {
		t.Errorf("while the check runs the rail reads\n%s\nwant the check running and the notes to come", strings.Join(got, "\n"))
	}
	r.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2)})
	got := r.screen(t)[4:6]
	want := []string{row(" ●  check         files that don't belong in the notes", "6.2s"), row(" ⠋  notes         writing the notes…", "0s")}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("once the check is done the rail reads\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRail_NotesInPartsDrawABarTheReCutAndTheMergeItsLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		say  func(p presenter.Presenter)
		want string
	}{
		{
			name: "parts done",
			say:  func(p presenter.Presenter) { p.NotesProgress(presenter.NotesProgress{Parts: 4, Done: 1}) },
			want: " ⠋  notes         1 of 4 parts  ━━━━━━──────────────────",
		},
		{
			name: "a part re-cut",
			say: func(p presenter.Presenter) {
				p.NotesProgress(presenter.NotesProgress{Parts: 4, Done: 1})
				p.NotesHalving(presenter.NotesHalving{Part: 2, Parts: 2})
			},
			want: " ⠋  notes         part 2 was too big for the model, retrying it as 2 smaller parts  ━━━━────────────",
		},
		{
			name: "the merge",
			say:  func(p presenter.Presenter) { p.NotesMerging(presenter.NotesMerge{Parts: 4}) },
			want: " ⠋  notes         merging 4 parts  ━━━━━━━━━━━━━━━━━━━━━━━━",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 100, 24, answers())
			r.RunStarted(releaseStarted(false))
			r.StageStarted(presenter.StageStart{Name: "notes", Key: "notes"})
			tt.say(r)
			if got := r.screen(t)[4]; got != tt.want {
				t.Errorf("the notes row reads\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestRail_NotesProgressWithNoStageRunningDrawsNothing(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	before := r.writes.count()
	r.NotesProgress(presenter.NotesProgress{Parts: 2, Done: 1})
	if r.writes.count() != before {
		t.Error("progress with no stage running wrote to the terminal, want nothing")
	}
}

func TestRail_ARunThatDoesNotLandLeavesTheStagesItNeverRanOutOfTheDrawing(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageFailed(presenter.StageFailure{Name: "generate", Key: "message", Message: "AI call failed"})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}, Elapsed: secs(1.2)})
	wantScreen(t, r, append(commitHead,
		" ✗  message       AI call failed",
		" │",
		row(" ◇  main          nothing changed", "1.2s"),
	)...)
}

func TestRail_TheCloseSettlesTheRun_AWarningAfterItDrawsUnderIt(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageSucceeded(presenter.Settled("message", "written"))
	r.StageSucceeded(presenter.Settled("commit", "a3f9c21"))
	r.StageSucceeded(presenter.Settled("push", "main → origin"))
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeCommitted, End: presenter.Station{Name: "main", Detail: "a3f9c21 · pushed"}, Elapsed: secs(4.8)})
	r.Warn(presenter.Warning{Label: "autostash", Message: "could not restore stashed changes cleanly"})
	wantScreen(t, r, append(commitHead,
		" ●  message       written",
		" ●  commit        a3f9c21",
		" │",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ●  push          main → origin",
		" │",
		row(" ◆  main          a3f9c21 · pushed", "4.8s"),
		" !  autostash     could not restore stashed changes cleanly",
	)...)
}

func TestRail_RegenerateAllDrawsABlockPerVersionAndClosesTheBatchOnce(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		r.RunStarted(presenter.RunInfo{Version: v, Header: presenter.Header{Command: "release regenerate --all", Where: pigeonWhere}})
		r.ShowPlan(presenter.Plan{Stages: []presenter.PlanStage{
			{Key: "notes", Name: "notes", Detail: "written by the AI"},
			{Key: "publish", Name: "release", Detail: "provider release"},
		}})
		r.StageStarted(notesStarted())
		if v == "1.1.0" {
			r.Warn(presenter.Warning{Label: "skipped v1.1.0", Message: "AI returned empty notes"})
			continue
		}
		r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Detail: "generated"})
	}
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeAllRegenerated, Summary: "2 regenerated, 1 skipped"})
	header := row(" mint ─ release regenerate --all", "~/Code/pigeon")
	wantScreen(t, r,
		header, "", " ●  notes         generated",
		header, "", " !  skipped v1.1.0", "                  AI returned empty notes",
		header, "", " ●  notes         generated",
	)
}

func TestRail_TheWidthIsReadAgainAtEachRedraw(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 100, 24, answers())
	commitRun(r)
	if rule := r.screen(t)[7]; uniseg.StringWidth(rule) != 100 {
		t.Fatalf("the rule is %d wide at 100 columns, want 100: %q", uniseg.StringWidth(rule), rule)
	}
	r.size.set(80, 24)
	r.StageStarted(presenter.StageStart{Name: "message", Key: "message"})
	if rule := r.screen(t)[8]; uniseg.StringWidth(rule) != 80 {
		t.Errorf("the rule is %d wide once the terminal is 80 columns, want 80: %q", uniseg.StringWidth(rule), rule)
	}
}

func TestRail_UnderSixtyColumnsThereIsNoRegion_LinesOnlySettle(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 50, 24, in)
	commitRun(r)
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true})
	r.StageSucceeded(presenter.Settled("message", "written"))
	r.ShowMessage(presenter.Message{Body: "feat: retry deliveries"})
	in.asked = func(int) {
		if got := r.screen(t); got[len(got)-1] != " ?  Commit this message?   y accept   n abort" {
			t.Errorf("while asked the terminal ends %q, want the question settled where it can be seen", got[len(got)-1])
		}
	}
	g := presenter.ReuseConfirmGate()
	g.Question, g.Stage = "Commit this message?", "message"
	if _, err := r.Prompt(g); err != nil {
		t.Fatal(err)
	}
	if all := r.writes.written(); strings.ContainsAny(all, "\r") || strings.Contains(all, "\x1b[") {
		t.Errorf("wrote %q, want no move back up and no erase: nothing redrawn", all)
	}
	got := r.screen(t)
	if answer := got[len(got)-2:]; strings.Join(answer, "\n") != " ●  review\n │  accepted" {
		t.Errorf("the answer reads %q, want it recorded on a row of its own, its detail under its name below 70 columns", answer)
	}
}

func TestRail_TheColourIsTheProfiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		profile termenv.Profile
		want    frametest.Colour
	}{
		{name: "truecolor", profile: termenv.TrueColor, want: "#5FD7AF"},
		{name: "256 colours", profile: termenv.ANSI256, want: "256:79"},
		{name: "the theme's sixteen", profile: termenv.ANSI, want: "ansi:6"},
		{name: "none, as NO_COLOR gives", profile: termenv.Ascii, want: frametest.Default},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := frametest.NewTerminal(80, 24)
			p := presenter.NewRailPresenter(term, presenter.WithRailProfile(tt.profile), presenter.WithRailSize(func() (int, int) { return 80, 24 }))
			p.ShowVersion(presenter.Version{Value: "0.1.2"})
			if err := term.Err(); err != nil {
				t.Fatal(err)
			}
			if got := term.Grid()[0][1]; got.Glyph != "m" || got.FG != tt.want {
				t.Errorf("the wordmark's m is %v, want it in %s", got, tt.want)
			}
		})
	}
}
