package presenter_test

import (
	"slices"
	"strings"
	"testing"
	"time"

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

// TestRail_TheUnwindNamesWhatItUndidThenWhatItKept pins that where the unwind left
// the repo is not drawn: a claim about the tree the rail doesn't make.
func TestRail_TheUnwindNamesWhatItUndidThenWhatItKept(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.Unwound(presenter.Unwind{Undone: "reset 2 commits", Kept: "kept the answers commit on the starting commit", Repo: "repo clean"})
	got := r.screen(t)[4:7]
	want := []string{
		" ↩  unwind        reset 2 commits",
		" │                kept the answers commit on the starting commit",
		" ┆",
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
			want: " ⠋  notes         1 of 4 parts done  ━━━━━━──────────────────",
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

// regenerateAll drives a --all backfill of three versions as the engine narrates it:
// one run, its header and station Quiet, a node per version that plain's own start
// and plan for each version leave as they are, v1.1.0 skipped, then plain's close
// Ahead and the run's Quiet close, then the presenter's.
func regenerateAll(r *railTerm) {
	header := presenter.Header{Command: "release regenerate --all", Where: pigeonWhere}
	r.RunStarted(presenter.RunInfo{Project: "pigeon", Header: header, Quiet: true})
	r.RunStarted(presenter.RunInfo{
		Project: "pigeon", Header: header, Quiet: true, Toward: "done",
		Start: presenter.Station{Name: "3 versions", Detail: "fresh notes → GitHub releases"},
	})
	tags := []string{"v1.0.0", "v1.1.0", "v2.0.0"}
	stages := make([]presenter.PlanStage, len(tags))
	for i, tag := range tags {
		stages[i] = presenter.PlanStage{Key: tag, Name: tag, Detail: "GitHub release"}
	}
	r.ShowPlan(presenter.Plan{Quiet: true, Stages: stages})
	for _, tag := range tags {
		r.RunStarted(presenter.RunInfo{Project: "pigeon", Version: tag[1:], Header: header})
		r.ShowPlan(presenter.Plan{Steps: []presenter.PlanStep{{Verb: "source", Target: "fresh"}, {Verb: "release", Target: tag}}})
		r.StageStarted(presenter.StageStart{Name: tag, Key: tag, Blocking: true, Quiet: true, Text: "generating release notes…"})
		r.StageStarted(presenter.StageStart{Name: "notes", Key: tag, Blocking: true, Text: "generating release notes…"})
		r.clock.Advance(1400 * time.Millisecond)
		if tag == "v1.1.0" {
			r.Warn(presenter.Warning{Key: tag, Label: "skipped " + tag, Message: "AI returned empty notes", Row: "skipped · AI returned empty notes", Elapsed: 1400 * time.Millisecond})
			r.NextMoves(presenter.NextMoves{Stage: tag, Quiet: true, Moves: []presenter.NextMove{{Lead: tag + " kept its old notes. To try it again:", Command: "mint release regenerate " + tag + " --target release"}}})
			continue
		}
		r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: tag, Detail: "generated", Row: "notes generated", Blocking: true, Ongoing: true, Elapsed: 1400 * time.Millisecond})
		r.StageStarted(presenter.StageStart{Name: "publish", Key: tag, Blocking: true, Quiet: true, Text: "writing the GitHub release…"})
		r.clock.Advance(700 * time.Millisecond)
		r.StageSucceeded(presenter.StageSuccess{Name: tag, Key: tag, Detail: "GitHub release", Blocking: true, Quiet: true, Elapsed: 2100 * time.Millisecond})
	}
	r.RunFinished(presenter.RunResult{Project: "pigeon", Outcome: presenter.OutcomeAllRegenerated, Summary: "2 regenerated, 1 skipped: v1.1.0 (AI returned empty notes)", Ahead: true})
	r.RunFinished(presenter.RunResult{Project: "pigeon", Outcome: presenter.OutcomeAllRegenerated, Quiet: true, End: presenter.Station{Name: "done", Detail: "2 of 3 versions regenerated · 1 skipped"}, Elapsed: 5600 * time.Millisecond})
	r.Close()
}

func TestRail_RegenerateAllIsOneRunWithANodePerVersion(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	regenerateAll(r)
	wantScreen(t, r,
		row(" mint ─ release regenerate --all", "~/Code/pigeon"),
		"",
		" ◇  3 versions    fresh notes → GitHub releases",
		" │",
		" ●  v1.0.0        GitHub release                                            2.1s",
		" !  v1.1.0        skipped · AI returned empty notes                         1.4s",
		" ●  v2.0.0        GitHub release                                            2.1s",
		" │",
		" ◆  done          2 of 3 versions regenerated · 1 skipped                   5.6s",
		"",
		" next",
		"   v1.1.0 kept its old notes. To try it again:",
		"     mint release regenerate v1.1.0 --target release",
	)
}

// TestRail_UnderSixtyColumnsAVersionSettlesAsItStandsAndGoesOnBelow pins that an
// ongoing node is finished for every settling rule: with no region to hold it, a
// --all version prints as it stands once its notes are done, and its release draws
// as a node of its own below.
func TestRail_UnderSixtyColumnsAVersionSettlesAsItStandsAndGoesOnBelow(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 50, 24, answers())
	header := presenter.Header{Command: "release regenerate --all", Where: pigeonWhere}
	r.RunStarted(presenter.RunInfo{Header: header, Quiet: true, Toward: "done", Start: presenter.Station{Name: "1 version", Detail: "fresh notes"}})
	r.ShowPlan(presenter.Plan{Quiet: true, Stages: []presenter.PlanStage{{Key: "v1.0.0", Name: "v1.0.0", Detail: "GitHub release"}}})
	r.StageStarted(presenter.StageStart{Name: "v1.0.0", Key: "v1.0.0", Blocking: true, Quiet: true, Text: "generating…"})
	r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "v1.0.0", Detail: "generated", Row: "notes generated", Blocking: true, Ongoing: true})
	r.StageStarted(presenter.StageStart{Name: "publish", Key: "v1.0.0", Blocking: true, Quiet: true, Text: "writing…"})
	r.StageSucceeded(presenter.StageSuccess{Name: "v1.0.0", Key: "v1.0.0", Detail: "GitHub release", Blocking: true, Quiet: true, Elapsed: 2 * time.Second})
	screen := r.screen(t)
	settled := slices.Index(screen, " │  notes generated")
	released := slices.Index(screen, " │  GitHub release")
	if settled < 0 || released <= settled || screen[released-1] != " ●  v1.0.0" {
		t.Errorf("the terminal shows\n%s\nwant the version as its notes left it, then its release on a node below", strings.Join(screen, "\n"))
	}
}

func TestRail_ARegenerateAllVersionRunsOnPastItsNotesOnOneNode(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	header := presenter.Header{Command: "release regenerate --all", Where: pigeonWhere}
	r.RunStarted(presenter.RunInfo{Header: header, Quiet: true, Toward: "done", Start: presenter.Station{Name: "1 version", Detail: "fresh notes → GitHub releases"}})
	r.ShowPlan(presenter.Plan{Quiet: true, Stages: []presenter.PlanStage{{Key: "v1.0.0", Name: "v1.0.0", Detail: "GitHub release"}}})
	r.StageStarted(presenter.StageStart{Name: "v1.0.0", Key: "v1.0.0", Blocking: true, Quiet: true, Text: "generating release notes…"})
	r.StageStarted(presenter.StageStart{Name: "notes", Key: "v1.0.0", Blocking: true, Text: "generating release notes…"})
	r.clock.Advance(3 * time.Second)
	r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "v1.0.0", Detail: "generated", Row: "notes generated", Blocking: true, Ongoing: true, Elapsed: 3 * time.Second})
	r.clock.Advance(40 * time.Second) // the review waiting on its answer
	r.StageStarted(presenter.StageStart{Name: "publish", Key: "v1.0.0", Blocking: true, Quiet: true, Text: "writing the GitHub release…"})
	r.clock.Advance(time.Second)
	r.ticks.tick(t)
	if got := r.screen(t)[4]; !strings.Contains(got, "writing the GitHub release…") || !strings.HasSuffix(got, " 4s") {
		t.Errorf("the version's node reads %q, want it running on with the time it ran, 4s, not the review's wait", got)
	}
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

func TestRail_UnderSixtyColumnsThereIsNoRegion_NothingIsRedrawn(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 50, 24, in)
	commitRun(r)
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true})
	r.StageSucceeded(presenter.Settled("message", "written"))
	r.ShowMessage(presenter.Message{Body: "feat: retry deliveries"})
	in.asked = func(int) {
		if got := r.screen(t); got[len(got)-1] != " ?  Commit this message?   y accept   n abort ›" {
			t.Errorf("while asked the terminal ends %q, want the question printed where it can be seen", got[len(got)-1])
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

// TestRail_UnderSixtyColumnsAQuestionPrintsTheMomentItIsAsked pins that with no
// region nothing waits to be shown: a question asked under a stage still running,
// or under a stage a dry run passed by, is on the terminal while it waits.
func TestRail_UnderSixtyColumnsAQuestionPrintsTheMomentItIsAsked(t *testing.T) {
	t.Parallel()
	checked := func(r *railTerm) {
		r.RunStarted(releaseStarted(false))
		r.ShowPlan(releasePlan())
		r.StageStarted(notesStarted())
		r.NotesChecking(presenter.NotesCheck{Key: "check"})
		r.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2)})
	}
	tests := []struct {
		name string
		// ask asks the question, calling look while it waits.
		ask  func(r *railTerm, look func())
		want string
	}{
		{name: "the cached notes, asked while the notes run", want: " ?  Cached notes found for this diff — use them?", ask: func(r *railTerm, _ func()) {
			r.RunStarted(releaseStarted(false))
			r.ShowPlan(releasePlan())
			r.StageStarted(notesStarted())
			r.SuspendSpinner()
			_, _ = r.Prompt(cacheGate())
		}},
		{name: "the exclusion pause, under the check", want: " │  ? Exclude docs/specs/** from the notes?", ask: func(r *railTerm, _ func()) {
			checked(r)
			_, _ = r.Prompt(pauseGate())
		}},
		{name: "a dry run's preview, under a stage it passed by", want: " ?  Exclude docs/specs/** from the notes? (design", ask: func(r *railTerm, look func()) {
			r.RunStarted(releaseStarted(true))
			r.ShowPlan(releasePlan())
			r.Warn(presenter.Warning{Label: "dry-run", Message: "skipping pre_tag hook"})
			r.StageStarted(notesStarted())
			r.NotesChecking(presenter.NotesCheck{Key: "check"})
			r.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2)})
			r.PreviewGate(pauseGate())
			look()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := answers("y\n")
			r := newRailTerm(t, 50, 40, in)
			looked := false
			look := func() {
				looked = true
				if screen := r.screen(t); !slices.Contains(screen, tt.want) {
					t.Errorf("while the question waits the terminal shows\n%s\nwant %q on it", strings.Join(screen, "\n"), tt.want)
				}
			}
			in.asked = func(int) { look() }
			tt.ask(r, look)
			if !looked {
				t.Fatal("the question was never up to look at")
			}
		})
	}
}

// TestRail_UnderSixtyColumnsAStagePrintsStillAsItStartsThenAgainAsItSettles pins
// that with no region a running stage's line prints the moment it starts, at the
// spinner's first frame with no time and no ticker, and its settled line once it is
// done.
func TestRail_UnderSixtyColumnsAStagePrintsStillAsItStartsThenAgainAsItSettles(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 50, 24, answers())
	commitRun(r)
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing…"})
	started := []string{" ⠋  message", " ┆  writing…"}
	if got := r.screen(t); !slices.Equal(got[len(got)-2:], started) {
		t.Errorf("once started the terminal ends\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(started, "\n"))
	}
	if r.ticks.running() {
		t.Error("a spinner ticks with no region to draw it in")
	}
	r.clock.Advance(secs(2.1))
	r.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Blocking: true, Elapsed: secs(2.1), Detail: "written"})
	want := append(started, " ●  message", " │  written")
	if got := r.screen(t); !slices.Equal(got[len(got)-4:], want) {
		t.Errorf("once done the terminal ends\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRail_AWarningAboutAStageDoneDrawsUnderItAndLeavesItAsItWas pins that a warning
// never changes a node done: it draws as a node of its own under it.
func TestRail_AWarningAboutAStageDoneDrawsUnderItAndLeavesItAsItWas(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true})
	r.StageSucceeded(presenter.StageSuccess{Name: "commit message", Key: "message", Blocking: true, Elapsed: secs(2.1), Detail: "written"})
	r.Warn(presenter.Warning{Key: "message", Label: "editor", Message: "could not launch editor, keeping the message"})
	got := r.screen(t)[4:6]
	want := []string{row(" ●  message       written", "2.1s"), " !  editor        could not launch editor, keeping the message"}
	if !slices.Equal(got, want) {
		t.Errorf("the message rows read\n%s\nwant the node done as it was and the warning under it\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRail_ADryRunSettlesTheStagesItPassedBy pins that a stage a dry run went past
// is as final as one done: hollow at the region's head, it settles as the run moves
// on, so notes taller than the terminal settle under it rather than being cut.
func TestRail_ADryRunSettlesTheStagesItPassedBy(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 100, 14, answers())
	r.RunStarted(releaseStarted(true))
	r.ShowPlan(releasePlan())
	r.Warn(presenter.Warning{Label: "dry-run", Message: "skipping pre_tag hook"})
	r.StageStarted(notesStarted())
	r.NotesChecking(presenter.NotesCheck{Key: "check"})
	r.NotesChecked(presenter.NotesCheck{Key: "check", Elapsed: secs(6.2)})
	r.StageSucceeded(presenter.StageSuccess{Name: "notes", Key: "notes", Blocking: true, Elapsed: secs(52.3), Detail: "written in 4 parts and merged"})
	r.ShowNotes(presenter.Notes{Version: "1.4.0", Body: pigeonNotes})
	screen := r.screen(t)
	for _, want := range []string{" ○  pre_tag       npm ci && npm run build", " │    - Signing secrets with a trailing newline are accepted again."} {
		if !slices.Contains(screen, want) {
			t.Errorf("the terminal shows\n%s\nwant %q on it", strings.Join(screen, "\n"), want)
		}
	}
}

// TestRail_ATerminalTooShortForARegionPrintsAsItHappens pins that a terminal under
// 12 rows, its height known, gets no region, as a narrow one gets none: the cached
// notes question, asked under the notes still running, is on it while it waits,
// and nothing is ever redrawn.
func TestRail_ATerminalTooShortForARegionPrintsAsItHappens(t *testing.T) {
	t.Parallel()
	in := answers("y\n")
	r := newRailTerm(t, 80, 11, in)
	r.RunStarted(releaseStarted(false))
	r.ShowPlan(releasePlan())
	r.StageStarted(notesStarted())
	if r.ticks.running() {
		t.Error("a spinner ticks with no region to draw it in")
	}
	const question = " ?  Cached notes found for this diff — use them?   y use cached   r regenerate"
	in.asked = func(int) {
		if screen := r.screen(t); !slices.Contains(screen, question) {
			t.Errorf("while the question waits the terminal shows\n%s\nwant %q on it", strings.Join(screen, "\n"), question)
		}
	}
	r.SuspendSpinner()
	if _, err := r.Prompt(cacheGate()); err != nil {
		t.Fatal(err)
	}
	if all := r.writes.written(); strings.Contains(all, "\x1b[") {
		t.Errorf("wrote %q, want no move back up and no erase: nothing redrawn", all)
	}
}

// TestRail_UnderSixtyColumnsADryRunPrintsTheStagesItPassedInPlanOrder pins that a
// planned stage starting marks those above it a dry run went past, so they print
// above it, but the one right above it, which runs inside it: the exclusion check,
// inside the notes.
func TestRail_UnderSixtyColumnsADryRunPrintsTheStagesItPassedInPlanOrder(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 50, 40, answers())
	dryRun(r, r.clock)
	screen := r.screen(t)
	order := []string{" ○  pre_tag", " ⠋  notes", " ⠋  check", " ●  check", " ●  notes"}
	at := -1
	for _, want := range order {
		i := slices.Index(screen, want)
		if i <= at {
			t.Fatalf("the terminal shows\n%s\nwant, in this order\n%s", strings.Join(screen, "\n"), strings.Join(order, "\n"))
		}
		at = i
	}
	if slices.Contains(screen, " ○  check") {
		t.Errorf("the check, which ran, is drawn as passed by:\n%s", strings.Join(screen, "\n"))
	}
}

// TestRail_UnderSixtyColumnsADryRunWithNoCheckPrintsItsSkippedHookFirst pins that
// the stage a starting one leaves to come above it is only one planned to run
// inside it: a dry run with no exclusion check, its notes from a file or with no
// AI, prints its skipped pre_tag above the notes running.
func TestRail_UnderSixtyColumnsADryRunWithNoCheckPrintsItsSkippedHookFirst(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 50, 40, answers())
	plan := releasePlan()
	plan.Stages = slices.DeleteFunc(plan.Stages, func(s presenter.PlanStage) bool { return s.Inside })
	r.RunStarted(releaseStarted(true))
	r.StageSucceeded(presenter.StageSuccess{Name: "version", Key: "version", Detail: "v1.4.0 (minor bump)"})
	r.StageSucceeded(presenter.StageSuccess{Name: "preflight", Key: "preflight", Detail: preflightDetail})
	r.ShowPlan(plan)
	r.Warn(presenter.Warning{Label: "dry-run", Message: "skipping pre_tag hook"})
	r.StageStarted(notesStarted())
	screen := r.screen(t)
	if hook, notes := slices.Index(screen, " ○  pre_tag"), slices.Index(screen, " ⠋  notes"); hook < 0 || notes < hook {
		t.Errorf("the terminal shows\n%s\nwant the skipped pre_tag above the notes running", strings.Join(screen, "\n"))
	}
}

// TestRail_AStageStartedOnceTheRunIsOverSettlesStill pins that a stage started once
// the run is over, with nothing live left to hold it, settles at once as it shows:
// its spinner's first frame and no time.
func TestRail_AStageStartedOnceTheRunIsOverSettlesStill(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		end  func(r *railTerm)
	}{
		{name: "the run finished", end: func(r *railTerm) {
			r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}})
		}},
		{name: "the presenter closed", end: func(r *railTerm) { r.Close() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, 80, 24, answers())
			commitRun(r)
			tt.end(r)
			r.clock.Advance(secs(2))
			r.StageStarted(presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing…"})
			want := []string{" ⠋  commit message", " ┆                writing…"}
			if screen := r.screen(t); !slices.Equal(screen[max(len(screen)-len(want), 0):], want) {
				t.Errorf("the terminal shows\n%s\nwant last\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}
