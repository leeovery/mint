package presenter_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// accent is the mint accent as a 24-bit terminal draws it.
const accent = frametest.Colour("#5FD7AF")

// ruleMark is the point of no return's ┿ as the terminal shows it.
func ruleMark(t *testing.T, r *railTerm) frametest.Cell {
	t.Helper()
	g, err := frametest.Parse(r.frame(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range g {
		if len(row) > 1 && row[1].Glyph == "┿" {
			return row[1]
		}
	}
	t.Fatalf("no point of no return on the terminal:\n%s", strings.Join(g.Text(), "\n"))
	return frametest.Cell{}
}

func TestRail_ThePointOfNoReturnIsCrossedOnceItsStageIsDoneNotAsItStarts(t *testing.T) {
	t.Parallel()
	r := newRailTermIn(t, termenv.TrueColor, 80, 24, answers())
	commitRun(r)

	r.StageStarted(presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing…"})
	if mark := ruleMark(t, r); mark.FG == accent {
		t.Errorf("┿ = %v with the push running, want it short of the accent: the run hasn't crossed", mark)
	}

	r.StageSucceeded(presenter.StageSuccess{Name: "push", Key: "push", Blocking: true, Detail: "main → origin"})
	if mark := ruleMark(t, r); mark.FG != accent {
		t.Errorf("┿ = %v with the push done, want the accent", mark)
	}
}

func TestRail_APushThatFailsLeavesThePointOfNoReturnReachedNotCrossed(t *testing.T) {
	t.Parallel()
	r := newRailTermIn(t, termenv.TrueColor, 80, 24, answers())
	commitRun(r)
	r.StageSucceeded(presenter.StageSuccess{Name: "commit", Key: "commit", Detail: "a3f9c21"})

	r.StageStarted(presenter.StageStart{Name: "push", Key: "push", Blocking: true, Text: "pushing…"})
	r.StageFailed(presenter.StageFailure{Name: "tag", Key: "push", Message: "push rejected"})
	r.Unwound(presenter.Unwind{Undone: "reset 1 commit", Key: "unwind"})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "main", Detail: "back where it started · nothing pushed"}, Elapsed: secs(4.9)})

	if mark := ruleMark(t, r); mark.FG == accent {
		t.Errorf("┿ = %v above a push that failed, want it short of the accent", mark)
	}
	wantScreen(t, r, append(commitHead,
		" ●  commit        a3f9c21",
		" │",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ✗  push          push rejected",
		" ↩  unwind        reset 1 commit",
		" │",
		row(" ◇  main          back where it started · nothing pushed", "4.9s"),
	)...)
}

func TestRail_AWarningAheadOfTheRunLeavesItsEdgeAboveThePlanStillToCome(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.Warn(presenter.Warning{Key: "commit", Label: "commit skipped", Message: "nothing staged"})
	r.Warn(presenter.Warning{Label: "editor", Message: "could not launch"})
	r.StageSucceeded(presenter.StageSuccess{Name: "message", Key: "message", Detail: "written"})

	wantScreen(t, r, append(commitHead,
		" !  editor        could not launch",
		" ●  message       written",
		" !  commit        nothing staged",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ○  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)
}

func TestRail_AWarningAheadOfAFailedStageStaysUnreachedWithTheUnwindAboveIt(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.Warn(presenter.Warning{Key: "commit", Label: "commit skipped", Message: "nothing staged"})
	r.StageFailed(presenter.StageFailure{Name: "message", Key: "message", Message: "the AI failed"})
	r.Unwound(presenter.Unwind{Undone: "reset 1 commit", Key: "unwind"})

	wantScreen(t, r, append(commitHead,
		" ✗  message       the AI failed",
		" ↩  unwind        reset 1 commit",
		" !  commit        nothing staged",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ○  push          to its upstream",
		" ┆",
		" ◇  main",
	)...)
}

func TestRail_AWarningAfterTheCloseDrawsUnderTheEndStationWithNextLast(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.NextMoves(presenter.NextMoves{Stage: "message", Moves: []presenter.NextMove{{Lead: "write the message yourself:", Command: "mint commit --no-ai"}}})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}, Elapsed: secs(1.2)})
	r.Warn(presenter.Warning{Label: "autostash", Message: "could not restore stashed changes cleanly"})
	r.Close()

	wantScreen(t, r, append(commitHead[:3:3],
		" │",
		row(" ◇  main          nothing changed", "1.2s"),
		" !  autostash     could not restore stashed changes cleanly",
		"",
		" next",
		"   write the message yourself:",
		"     mint commit --no-ai",
	)...)
}

func TestRail_MovesAndSuggestionsArrivingAfterTheCloseJoinNext(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	r.NextMoves(presenter.NextMoves{Stage: "message", Moves: []presenter.NextMove{{Lead: "write the message yourself:", Command: "mint commit --no-ai"}}})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}, Elapsed: secs(1.2)})
	r.NextMoves(presenter.NextMoves{Stage: "autostash", Moves: []presenter.NextMove{{Lead: "Put your work back:", Command: "git stash pop"}}})
	s := specsSuggestion()
	s.Line = `diff_exclude = ["docs/specs/**"]`
	r.ExclusionSuggestions(presenter.ExclusionSuggestions{Stage: "message", Table: "[release]", File: ".mint.toml", Suggestions: []presenter.ExclusionSuggestion{s}})
	r.Close()

	want := []string{
		" next",
		"   write the message yourself:",
		"     mint commit --no-ai",
		"   Put your work back:",
		"     git stash pop",
		"   Leave docs/specs/** out of the notes, under [release] in .mint.toml:",
		`     diff_exclude = ["docs/specs/**"]`,
	}
	screen := r.screen(t)
	if got := screen[max(len(screen)-len(want), 0):]; !slices.Equal(got, want) {
		t.Errorf("the run ends\n%s\nwant next to hold every move, in the order it came\n%s", strings.Join(screen, "\n"), strings.Join(want, "\n"))
	}
}

// unfinishedUnwind is a commit run whose push is rejected and whose unwind's reset
// fails, having undone the commit.
func unfinishedUnwind(r *railTerm) {
	commitRun(r)
	r.StageSucceeded(presenter.StageSuccess{Name: "commit", Key: "commit", Detail: "a3f9c21"})
	r.StageFailed(presenter.StageFailure{Name: "tag", Key: "push", Message: "push rejected"})
	r.Warn(presenter.Warning{
		Label: "unwind incomplete", Key: "unwind", Failed: true,
		Message: "automatic recovery failed; manually reset HEAD back to 7c1e0b4",
		Output:  "fatal: could not write the index\n",
	})
	r.Unwound(presenter.Unwind{
		Undone: "reset 1 commit", Kept: "kept your exclusion answers in .mint.toml",
		KeptRow: "kept your exclusion answers in .mint.toml", Repo: "manual cleanup required", Key: "unwind",
	})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeUnwound, End: presenter.Station{Name: "main", Detail: "manual cleanup required · nothing pushed"}, Elapsed: secs(4.9)})
}

func TestRail_AnUnwindThatCouldNotFinishIsAFailedUnwindNodeWithWhatItUndid(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	unfinishedUnwind(r)

	screen := r.screen(t)
	want := []string{
		" ✗  unwind        automatic recovery failed; manually reset HEAD back to 7c1e0b4",
		" │                reset 1 commit",
		" │                │ fatal: could not write the index",
		" │                kept your exclusion answers in .mint.toml",
		" │",
		row(" ◇  main          manual cleanup required · nothing pushed", "4.9s"),
	}
	if got := screen[len(screen)-len(want):]; !slices.Equal(got, want) {
		t.Errorf("the run ends\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if slices.ContainsFunc(screen, func(line string) bool { return strings.Contains(line, "↩") }) {
		t.Errorf("an unwind that couldn't finish drew a ↩ node of its own:\n%s", strings.Join(screen, "\n"))
	}
	if got, want := r.err.String(), "✗ tag  push rejected\n⚠ unwind incomplete  automatic recovery failed; manually reset HEAD back to 7c1e0b4\n"; got != want {
		t.Errorf("err = %q, want the failure's and the warning's summaries %q", got, want)
	}
}

// TestRail_WithNoRegionAnUnwindThatCouldNotFinishStillHangsWhatItUndidUnderItsFailedNode
// pins that the failed unwind waits for the Unwound however the run is drawn: on a
// terminal too narrow or too short for a region, printing as it happens, what the
// unwind undid still hangs under the failed node, above the step's output, and no ↩
// node of its own is drawn.
func TestRail_WithNoRegionAnUnwindThatCouldNotFinishStillHangsWhatItUndidUnderItsFailedNode(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ cols, rows int }{{50, 24}, {80, 8}} {
		t.Run(fmt.Sprintf("%dx%d", size.cols, size.rows), func(t *testing.T) {
			t.Parallel()
			r := newRailTerm(t, size.cols, size.rows, answers())
			unfinishedUnwind(r)

			screen := r.screen(t)
			at := func(text string) int {
				return slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, text) })
			}
			failed, undone, output, kept := at("✗  unwind"), at("reset 1 commit"), at("fatal: could not write the index"), at("kept your exclusion answers")
			if failed == -1 || failed >= undone || undone >= output || output >= kept {
				t.Errorf("the terminal shows\n%s\nwant the failed unwind, then what it undid, the step's output and what it kept, in that order", strings.Join(screen, "\n"))
			}
			if slices.ContainsFunc(screen, func(line string) bool { return strings.Contains(line, "↩") }) {
				t.Errorf("an unwind that couldn't finish drew a ↩ node of its own:\n%s", strings.Join(screen, "\n"))
			}
		})
	}
}

func TestRail_AnExclusionSuggestionsMoveSaysWhichTableOfWhichFileItGoesUnder(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	s := specsSuggestion()
	s.Line = `diff_exclude = ["docs/specs/**"]`
	r.ExclusionSuggestions(presenter.ExclusionSuggestions{Stage: "message", Table: "[release]", File: ".mint.toml", Suggestions: []presenter.ExclusionSuggestion{s}})
	r.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeFailed, End: presenter.Station{Name: "main", Detail: "nothing changed"}, Elapsed: secs(1.2)})
	r.Close()

	screen := r.screen(t)
	want := []string{
		"   Leave docs/specs/** out of the notes, under [release] in .mint.toml:",
		`     diff_exclude = ["docs/specs/**"]`,
	}
	if got := screen[len(screen)-2:]; !slices.Equal(got, want) {
		t.Errorf("next ends\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRail_AStagesClockStandsStillWhileAQuestionWaits pins that no node's time
// counts a wait on an answer: the notes' clock stands while the cached-notes
// question, asked as they run, waits, and goes on from there.
func TestRail_AStagesClockStandsStillWhileAQuestionWaits(t *testing.T) {
	t.Parallel()
	in := answers("r\n")
	r := newRailTerm(t, 80, 24, in)
	r.RunStarted(releaseStarted(false))
	r.StageStarted(notesStarted())
	r.clock.Advance(secs(2.1))
	in.asked = func(int) { r.clock.Advance(5 * time.Minute) }
	r.SuspendSpinner()
	if _, err := r.Prompt(cacheGate()); err != nil {
		t.Fatal(err)
	}
	r.ResumeSpinner()
	r.clock.Advance(secs(1))
	r.ticks.tick(t)

	if got, want := r.screen(t)[4], row(" ⠇  notes         writing the notes… · regenerate", "3s"); got != want {
		t.Errorf("the running node reads %q, want its time without the wait %q", got, want)
	}
}

func TestRail_AStageStartedAgainWhileItRunsKeepsItsClock(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers())
	commitRun(r)
	start := presenter.StageStart{Name: "commit message", Key: "message", Blocking: true, Text: "writing the commit message…"}

	r.StageStarted(start)
	r.clock.Advance(secs(3.2))
	r.StageStarted(start)
	r.ticks.tick(t)

	if got, want := r.screen(t)[4], row(" ⠋  message       writing the commit message…", "3s"); got != want {
		t.Errorf("the running node reads %q, want its time counted from its first start %q", got, want)
	}
}

// TestRail_ALineSayingWhatMintDidIsDrawnInTheNormalColour pins which lines hanging
// off a node read as mint's own deeds, in the normal colour, and which as metadata,
// dim: a commit made, the answers written, the part notes saved and what an unwind
// that couldn't finish undid and kept, against a step a stage passed over.
func TestRail_ALineSayingWhatMintDidIsDrawnInTheNormalColour(t *testing.T) {
	t.Parallel()
	savedNotes := func(r *railTerm) {
		r.StageFailed(presenter.StageFailure{Name: "notes", Key: "notes", Message: "the AI call failed", SavedNotes: "~/notes/v1.4.0.md"})
	}
	failedUnwind := func(r *railTerm) {
		r.Warn(presenter.Warning{Key: "unwind", Label: "unwind", Message: "automatic recovery failed", Failed: true})
		r.Unwound(presenter.Unwind{Key: "unwind", Undone: "deleted tag v1.4.0", KeptRow: "kept your exclusion answers in .mint.toml"})
	}
	tests := []struct {
		name  string
		event func(r *railTerm)
		text  string
		dim   bool
	}{
		{name: "a commit a stage made", text: "committed as chore(release): pre-tag artifacts", event: func(r *railTerm) {
			r.StageSucceeded(presenter.StageSuccess{Name: "pre_tag", Key: "pre_tag", Detail: "npm run build", Commit: "chore(release): pre-tag artifacts"})
		}},
		{name: "the exclusion answers written", text: "written to .mint.toml's [release] table and committed", event: func(r *railTerm) {
			r.StageSucceeded(presenter.StageSuccess{Name: "check", Key: "check", Detail: "1 flagged"})
			r.ExclusionAnswersRecorded(presenter.ExclusionAnswers{Key: "check", File: ".mint.toml", Table: "[release]"})
		}},
		{name: "the part notes saved", text: "part notes saved to", event: savedNotes},
		{name: "where the part notes were saved", text: "~/notes/v1.4.0.md", event: savedNotes},
		{name: "what an unwind that couldn't finish undid", text: "deleted tag v1.4.0", event: failedUnwind},
		{name: "what an unwind that couldn't finish kept", text: "kept your exclusion answers in .mint.toml", event: failedUnwind},
		{name: "a step a stage passed over", text: "skipping preflight hook", dim: true, event: func(r *railTerm) {
			r.StageSucceeded(presenter.StageSuccess{Name: "preflight", Key: "preflight", Detail: "tree clean", Note: "skipping preflight hook"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRailTermIn(t, termenv.TrueColor, 100, 40, answers())
			commitRun(r)
			tt.event(r)
			r.Close()
			for _, c := range hungCells(t, r, tt.text) {
				if c.Faint != tt.dim {
					t.Errorf("%q draws %v, want faint %v", tt.text, c, tt.dim)
				}
			}
		})
	}
}

// hungCells is the cells of the line hanging off a node that reads text, from the
// detail column to the text's end.
func hungCells(t *testing.T, r *railTerm, text string) []frametest.Cell {
	t.Helper()
	g, err := frametest.Parse(r.frame(t))
	if err != nil {
		t.Fatal(err)
	}
	const detailCol = 18
	for _, row := range g {
		if hung := row[min(detailCol, len(row)):]; (frametest.Grid{hung}).Text()[0] == text {
			return hung[:len([]rune(text))]
		}
	}
	t.Fatalf("no line hangs %q:\n%s", text, strings.Join(g.Text(), "\n"))
	return nil
}
