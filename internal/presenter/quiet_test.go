package presenter_test

import (
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// quietPlan is a run's stages as the engine sends them ahead of the rail: Quiet,
// though it carries the steps plain would otherwise list.
var quietPlan = presenter.Plan{
	Steps:           []presenter.PlanStep{{Verb: "tag", Target: "v1.4.0"}},
	Stages:          []presenter.PlanStage{{Key: "push", Name: "push", Detail: "main + v1.4.0 → origin, atomic"}},
	PointOfNoReturn: "push",
	Quiet:           true,
}

// quietStart is the notes stage running again for an `r` that writes them in one
// call, which plain has never had a line for.
var quietStart = presenter.StageStart{Name: "notes", Key: "notes", Blocking: true, Text: "generating release notes…", Quiet: true}

// quietMoves are a failed unwind's manual step, which plain's warning line names.
var quietMoves = presenter.NextMoves{
	Stage: "unwind",
	Quiet: true,
	Moves: []presenter.NextMove{{Lead: "Reset HEAD back to 4f9c2e1:", Command: "git reset --hard 4f9c2e1"}},
}

// quietReview is the review a dry run previews only for the rail.
func quietReview() presenter.Gate {
	g := presenter.ReleaseReviewGate()
	g.Stage, g.Quiet = "notes", true
	return g
}

// TestQuietEvents_WriteNothingInPlainOrTodaysPretty pins Quiet: an event that only
// fills in the run's structure has no line in plain's log, pinned byte for byte
// from before the rail, nor in today's pretty.
func TestQuietEvents_WriteNothingInPlainOrTodaysPretty(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		plain  func(*presenter.PlainPresenter)
		pretty func(*presenter.PrettyPresenter)
	}{
		{
			name:   "a quiet plan",
			plain:  func(p *presenter.PlainPresenter) { p.ShowPlan(quietPlan) },
			pretty: func(p *presenter.PrettyPresenter) { p.ShowPlan(quietPlan) },
		},
		{
			name:   "a settled stage",
			plain:  func(p *presenter.PlainPresenter) { p.StageSucceeded(presenter.Settled("diff", "3 files staged")) },
			pretty: func(p *presenter.PrettyPresenter) { p.StageSucceeded(presenter.Settled("diff", "3 files staged")) },
		},
		{
			name:   "a quiet blocking start",
			plain:  func(p *presenter.PlainPresenter) { p.StageStarted(quietStart) },
			pretty: func(p *presenter.PrettyPresenter) { p.StageStarted(quietStart) },
		},
		{
			name:   "a quiet gate's preview",
			plain:  func(p *presenter.PlainPresenter) { p.PreviewGate(quietReview()) },
			pretty: func(p *presenter.PrettyPresenter) { p.PreviewGate(quietReview()) },
		},
		{
			name:   "quiet next moves",
			plain:  func(p *presenter.PlainPresenter) { p.NextMoves(quietMoves) },
			pretty: func(p *presenter.PrettyPresenter) { p.NextMoves(quietMoves) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if out, errBuf := drive(tc.plain); out.Len() != 0 || errBuf.Len() != 0 {
				t.Errorf("plain wrote %q / %q, want nothing", out.String(), errBuf.String())
			}
			if out := drivePretty(termenv.Ascii, tc.pretty); out.Len() != 0 {
				t.Errorf("pretty wrote %q, want nothing", out.String())
			}
		})
	}
}

// TestPlainPresenter_AQuietPlanLeavesTheLoudPlanAlone proves Quiet silences only the
// event that carries it: the plan a dry run prints still renders its steps.
func TestPlainPresenter_AQuietPlanLeavesTheLoudPlanAlone(t *testing.T) {
	t.Parallel()

	loud := quietPlan
	loud.Quiet = false
	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.ShowPlan(quietPlan)
		p.ShowPlan(loud)
	})

	if want := "plan: tag v1.4.0\n"; out.String() != want {
		t.Errorf("plain plans = %q, want only the loud plan's %q", out.String(), want)
	}
}

func TestSettled_IsAQuietSuccessKeyedAndNamedForItsStage(t *testing.T) {
	t.Parallel()

	want := presenter.StageSuccess{Name: "commit", Key: "commit", Detail: "a3f9c21", Quiet: true}
	if got := presenter.Settled("commit", "a3f9c21"); got != want {
		t.Errorf("Settled = %+v, want %+v", got, want)
	}
}

// TestNextMoveText_FlattensLeadAndCommandToTheSentencePlainPrints pins the one join
// plain and today's pretty print a move by: the lead, then the command after a
// space, or the lead alone when the move names no command.
func TestNextMoveText_FlattensLeadAndCommandToTheSentencePlainPrints(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		move presenter.NextMove
		want string
	}{
		{
			name: "a lead and its command",
			move: presenter.NextMove{Lead: "write the notes to a file and release with", Command: "mint release --notes-file <file>"},
			want: "write the notes to a file and release with mint release --notes-file <file>",
		},
		{
			name: "a lead alone",
			move: presenter.NextMove{Lead: "re-run with --no-ai to release with the commit-subject list"},
			want: "re-run with --no-ai to release with the commit-subject list",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.move.Text(); got != tc.want {
				t.Errorf("Text() = %q, want %q", got, tc.want)
			}
		})
	}
}
