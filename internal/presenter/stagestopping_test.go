package presenter_test

import (
	"reflect"
	"strings"
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

var preTagStopping = presenter.StageStop{Name: "pre_tag", Key: "pre_tag", Text: "waiting for the pre_tag hook to stop"}

func TestPlainPresenter_StageStopping_PrintsItsOwnLine(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) {
		p.StageStarted(presenter.StageStart{Name: "pre_tag", Key: "pre_tag", Blocking: true, Text: "running pre_tag hook…"})
		p.StageStopping(preTagStopping)
	})

	want := "pre_tag: running...\n" +
		"pre_tag: waiting for the pre_tag hook to stop...\n"
	if got := out.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("err = %q, want nothing", errBuf.String())
	}
	assertBytePureASCIIStreams(t, "plain stage stopping narration", out, errBuf)
}

// preTagRunning drives a release to its pre_tag hook, running for 2.5s.
func preTagRunning(r *railTerm) {
	r.RunStarted(releaseStarted(false))
	_, _ = r.Prompt(versionGate())
	r.StageSucceeded(presenter.StageSuccess{Name: "preflight", Key: "preflight", Detail: preflightDetail})
	r.ShowPlan(releasePlan())
	r.StageStarted(presenter.StageStart{Name: "pre_tag", Key: "pre_tag", Blocking: true, Text: "running the pre_tag hook…"})
	r.clock.Advance(secs(2.5))
}

// The running node says what mint waits on in place of its activity, its spinner and
// clock going on.
func TestRail_StageStopping_SaysItBesideTheRunningSpinner(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers("y"))
	preTagRunning(r)

	r.StageStopping(preTagStopping)
	r.clock.Advance(secs(1))
	r.ticks.tick(t)

	wantScreen(t, r,
		row(" mint ─ release", "~/Code/pigeon · v0.1.2"),
		"",
		" ◇  v1.3.2        main @ 7c1e0b4",
		" │",
		" ●  version       v1.4.0 (minor bump)",
		" ●  preflight     "+preflightDetail,
		row(" ⠸  pre_tag       waiting for the pre_tag hook to stop…", "3s"),
		" ○  check",
		" ○  notes",
		" ○  record",
		" ┆",
		" ┿━━ point of no return "+strings.Repeat("━", 56),
		" ┆",
		" ○  push",
		" ○  publish",
		" ○  post_release",
		" ┆",
		" ◇  v1.4.0",
	)
	if r.err.Len() != 0 {
		t.Errorf("err = %q, want nothing", r.err.String())
	}
}

// A stage already settled is past waiting on: its node is left as it was.
func TestRail_StageStopping_LeavesASettledNodeAlone(t *testing.T) {
	t.Parallel()
	r := newRailTerm(t, 80, 24, answers("y"))
	preTagRunning(r)
	r.StageSucceeded(presenter.StageSuccess{Name: "pre_tag", Key: "pre_tag", Blocking: true, Elapsed: secs(2.5), Detail: "npm ci && npm run build"})
	before := r.screen(t)

	r.StageStopping(preTagStopping)

	if got := r.screen(t); !reflect.DeepEqual(got, before) {
		t.Errorf("screen =\n%v\nwant it unchanged:\n%v", got, before)
	}
}

func TestRecordingPresenter_StageStopping_RecordsItsPayload(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	rec.StageStopping(preTagStopping)

	if len(rec.Events) != 1 {
		t.Fatalf("events = %d, want one", len(rec.Events))
	}
	if got := rec.Events[0]; got.Kind != presentertest.KindStageStopping || got.StageStopping != preTagStopping {
		t.Errorf("event = %v %+v, want %v %+v", got.Kind, got.StageStopping, presentertest.KindStageStopping, preTagStopping)
	}
	if got := presentertest.KindStageStopping.String(); got != "StageStopping" {
		t.Errorf("kind string = %q, want StageStopping", got)
	}
}
