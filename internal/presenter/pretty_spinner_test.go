package presenter_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// spySpinner is the deterministic stand-in for the real briandowns spinner. It
// records its Start/Stop calls and reports to a shared spyTracker so the tests can
// assert the LIFECYCLE (started on a blocking StageStarted, stopped on completion)
// and the "one spinner at a time" invariant WITHOUT the real library's timed
// goroutine and frame output. It writes nothing — the seam under test is the call
// sequence, not rendered frames.
type spySpinner struct {
	tracker *spyTracker
	started bool
	stopped bool
	text    string
}

func (s *spySpinner) Start() {
	s.started = true
	s.tracker.active++
	if s.tracker.active > s.tracker.maxActive {
		s.tracker.maxActive = s.tracker.active
	}
}

func (s *spySpinner) Stop() {
	// Stopping a spinner that was never started, or stopping twice, must not drive
	// the active count negative — the presenter is allowed a defensive Stop.
	if s.started && !s.stopped {
		s.tracker.active--
	}
	s.stopped = true
}

// spyTracker is shared across every spinner a single presenter creates, so the
// peak concurrent count (maxActive) can be asserted across a multi-stage run: the
// "no two concurrent spinners" rule means maxActive must never exceed 1.
type spyTracker struct {
	created   []*spySpinner
	active    int
	maxActive int
}

// factory returns a spinnerFactory closure that records each spinner it builds on
// the tracker, so a test can inspect the per-stage spinners and the peak active
// count after driving a sequence.
func (tr *spyTracker) factory() func(out io.Writer, text string) presenter.StageSpinner {
	return func(_ io.Writer, text string) presenter.StageSpinner {
		sp := &spySpinner{tracker: tr, text: text}
		tr.created = append(tr.created, sp)
		return sp
	}
}

// drivePrettySpy runs fn against a pretty presenter whose spinner factory is the
// spy, capturing narration into out and exposing the tracker for lifecycle
// assertions. The colour profile is forced to Ascii so the stage lines are asserted
// on layout/glyphs rather than ANSI bytes.
func drivePrettySpy(t *testing.T, fn func(p *presenter.PrettyPresenter)) (out *bytes.Buffer, tr *spyTracker) {
	t.Helper()
	out = &bytes.Buffer{}
	tr = &spyTracker{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithSpinnerFactory(tr.factory())
	fn(p)
	return out, tr
}

// TestPrettyPresenterSpinnerReplacedByCheckOnSuccess is the core success
// lifecycle: a blocking StageStarted starts a spinner; the subsequent
// StageSucceeded stops it (the spinner clears its line in place) and renders the ✓
// completion line. The spy proves Start→Stop; the buffer proves the ✓ line lands.
func TestPrettyPresenterSpinnerReplacedByCheckOnSuccess(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Elapsed: 1100 * 1000 * 1000, Blocking: true})
	})

	if len(tr.created) != 1 {
		t.Fatalf("expected exactly one spinner created, got %d", len(tr.created))
	}
	sp := tr.created[0]
	if !sp.started {
		t.Errorf("spinner was not Started on a blocking StageStarted")
	}
	if !sp.stopped {
		t.Errorf("spinner was not Stopped on StageSucceeded")
	}
	if !strings.Contains(out.String(), "✓ notes") {
		t.Errorf("✓ completion line missing after the spinner stopped:\n%q", out.String())
	}
}

// TestPrettyPresenterSpinnerReplacedByCrossOnFailure is the failure lifecycle: a
// blocking StageStarted starts a spinner; the subsequent StageFailed stops it and
// renders the ✗ line in the cleared place. The spy proves Start→Stop; the buffer
// proves the ✗ line lands.
func TestPrettyPresenterSpinnerReplacedByCrossOnFailure(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "tag/push", Blocking: true})
		p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected: remote moved"})
	})

	if len(tr.created) != 1 {
		t.Fatalf("expected exactly one spinner created, got %d", len(tr.created))
	}
	sp := tr.created[0]
	if !sp.started || !sp.stopped {
		t.Errorf("spinner lifecycle on failure: started=%v stopped=%v, want both true", sp.started, sp.stopped)
	}
	if !strings.Contains(out.String(), "✗ tag/push") {
		t.Errorf("✗ failure line missing after the spinner stopped:\n%q", out.String())
	}
}

// TestPrettyPresenterCapturedOutputBelowCrossOnlyOnFailure proves the captured
// underlying-command output is printed below the ✗ on failure — and is NOT printed
// on a successful stage. The body is buffered (engine-supplied), never streamed
// through the spinner.
func TestPrettyPresenterCapturedOutputBelowCrossOnlyOnFailure(t *testing.T) {
	const chatter = "fatal: failed to push some refs to 'origin'\nhint: remote moved"

	t.Run("failure prints the captured body below the cross", func(t *testing.T) {
		out, _ := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
			p.StageStarted(presenter.StageStart{Name: "tag/push", Blocking: true})
			p.StageFailed(presenter.StageFailure{Name: "tag/push", Message: "push rejected", Output: chatter})
		})

		got := out.String()
		crossIdx := strings.Index(got, "✗ tag/push")
		bodyIdx := strings.Index(got, chatter)
		if crossIdx < 0 || bodyIdx < 0 {
			t.Fatalf("expected the ✗ line and the captured body present:\n%q", got)
		}
		if bodyIdx <= crossIdx {
			t.Errorf("captured body must follow the ✗ line: cross=%d body=%d\n%q", crossIdx, bodyIdx, got)
		}
	})

	t.Run("success does not print the captured body", func(t *testing.T) {
		out, _ := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
			p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
			p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})
		})

		if strings.Contains(out.String(), chatter) {
			t.Errorf("a successful stage must NOT print captured output:\n%q", out.String())
		}
	})
}

// TestPrettyPresenterOneSpinnerAtATimeAcrossStages is the "one spinner at a time"
// invariant across sequential stages: A starts→A succeeds→B starts→B succeeds. The
// spy tracker's peak active count must never exceed 1 — A's spinner is stopped
// before B's is started.
func TestPrettyPresenterOneSpinnerAtATimeAcrossStages(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "prep", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "prep", Detail: "built", Blocking: true})
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})
	})

	if tr.maxActive > 1 {
		t.Errorf("more than one spinner was active at once: peak active = %d, want <= 1", tr.maxActive)
	}
	if len(tr.created) != 2 {
		t.Errorf("expected two spinners across two blocking stages, got %d", len(tr.created))
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active at the end of the run: active = %d, want 0", tr.active)
	}
}

// TestPrettyPresenterDefensiveStopOnDoubleStart proves the defensive guard: if a
// blocking StageStarted fires while a spinner is somehow already active (no
// intervening completion), the presenter stops the previous spinner before
// starting the new one — never two concurrent. The first spinner must end stopped
// and the peak active count must stay at 1.
func TestPrettyPresenterDefensiveStopOnDoubleStart(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "a", Blocking: true})
		p.StageStarted(presenter.StageStart{Name: "b", Blocking: true})
		p.StageSucceeded(presenter.StageSuccess{Name: "b", Blocking: true})
	})

	if len(tr.created) != 2 {
		t.Fatalf("expected two spinners created, got %d", len(tr.created))
	}
	if !tr.created[0].stopped {
		t.Errorf("the first spinner must be defensively stopped before the second starts")
	}
	if tr.maxActive > 1 {
		t.Errorf("defensive stop failed: peak active = %d, want <= 1", tr.maxActive)
	}
}

// TestPrettyPresenterShortStageStartsNoSpinner proves a short (non-blocking) stage
// is unaffected by the spinner work: a non-blocking StageStarted creates NO
// spinner and renders nothing, and the subsequent StageSucceeded renders only the
// static ✓ line.
func TestPrettyPresenterShortStageStartsNoSpinner(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "preflight", Blocking: false})
		p.StageSucceeded(presenter.StageSuccess{Name: "preflight", Detail: "clean · on main"})
	})

	if len(tr.created) != 0 {
		t.Errorf("a short (non-blocking) stage must start NO spinner, got %d created", len(tr.created))
	}
	want := "  ✓ preflight  clean · on main\n"
	if got := out.String(); got != want {
		t.Errorf("short stage = %q, want only the static ✓ line %q", got, want)
	}
}

// TestPrettyPresenterNonBlockingStageStartedRendersNothing locks the behaviour
// change this task makes: a non-blocking StageStarted renders NOTHING (no spinner,
// no static line) — replacing the Phase-1 placeholder static-dim-line behaviour,
// consistent with plain and the spec worked example where short stages show only
// the ✓ line.
func TestPrettyPresenterNonBlockingStageStartedRendersNothing(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "preflight", Blocking: false})
	})

	if got := out.String(); got != "" {
		t.Errorf("non-blocking StageStarted must render nothing, got %q", got)
	}
	if len(tr.created) != 0 {
		t.Errorf("non-blocking StageStarted must start no spinner, got %d", len(tr.created))
	}
}

// TestPrettyPresenterWarnStopsActiveSpinnerBatchSkipPath covers case (a): a
// notes-production-failure SKIP fires a Warn while a blocking notes stage's spinner
// is still live (the engine emits StageStarted{Blocking} then, on the failure,
// reportSkip→Warn WITHOUT a StageSucceeded). The spinner must be stopped BEFORE the
// ⚠ line so nothing animates over the skip notice or the end summary that follows;
// the stage ENDS here (skip-and-continue), so the spinner must not be resurrected by
// a later RunFinished.
func TestPrettyPresenterWarnStopsActiveSpinnerBatchSkipPath(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		// The skip path: a Warn with NO StageSucceeded, then the run's end summary.
		p.Warn(presenter.Warning{Label: "skipped v1.1.0", Message: "diff too large"})
		p.RunFinished(presenter.RunResult{Verb: presenter.VerbRegenerate, Project: "mint", Summary: "0 regenerated, 1 skipped: v1.1.0 (diff too large)"})
	})

	if len(tr.created) != 1 {
		t.Fatalf("expected exactly one spinner created, got %d", len(tr.created))
	}
	if !tr.created[0].stopped {
		t.Errorf("the active spinner must be stopped before the skip Warn line, but it was left running")
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active across the skip Warn / end summary: active = %d, want 0", tr.active)
	}
}

// TestPrettyPresenterWarnStopsActiveSpinnerCacheReusePath covers case (b): a real-run
// cache-reuse / miss / unreadable notice rides the Warn seam INSIDE the live blocking
// notes stage (the engine emits StageStarted{Blocking}, the reuse callback fires a
// Warn mid-stage, then the stage continues and ends with StageSucceeded). The spinner
// must be stopped BEFORE the ⚠ line so nothing animates over the notice; the stage
// still terminates correctly on the following StageSucceeded (which prints the ✓
// line), and never leaves a spinner active.
func TestPrettyPresenterWarnStopsActiveSpinnerCacheReusePath(t *testing.T) {
	out := &bytes.Buffer{}
	tr := &spyTracker{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithSpinnerFactory(tr.factory())

	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	p.Warn(presenter.Warning{Label: "notes", Message: "reusing the previewed notes from the dry-run cache"})
	// Capture the spinner state at the MOMENT the Warn fired — it must already be
	// stopped here, before the stage continues and later StageSucceeds.
	stoppedAtWarn := tr.created[0].stopped
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})

	if len(tr.created) != 1 {
		t.Fatalf("expected exactly one spinner created, got %d", len(tr.created))
	}
	if !stoppedAtWarn {
		t.Errorf("the active spinner must be stopped by the cache-reuse Warn itself, but it was still running when the Warn returned")
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active after the stage completed: active = %d, want 0", tr.active)
	}
	got := out.String()
	warnIdx := strings.Index(got, "⚠ notes")
	checkIdx := strings.Index(got, "✓ notes")
	if warnIdx < 0 || checkIdx < 0 {
		t.Fatalf("expected both the ⚠ notice and the ✓ completion line:\n%q", got)
	}
	if checkIdx <= warnIdx {
		t.Errorf("the ✓ completion line must follow the ⚠ notice: warn=%d check=%d\n%q", warnIdx, checkIdx, got)
	}
}

// TestPrettyPresenterWarnWithNoActiveSpinnerCreatesNone proves the general fix is a
// safe no-op when no spinner is live: a standalone Warn (the common post_release /
// push case, fired outside any blocking stage) neither creates nor touches a spinner.
func TestPrettyPresenterWarnWithNoActiveSpinnerCreatesNone(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.Warn(presenter.Warning{Label: "post_release", Message: "hook exited 1"})
	})

	if len(tr.created) != 0 {
		t.Errorf("a Warn with no active spinner must create no spinner, got %d", len(tr.created))
	}
}

// TestPrettyPresenterSpinnerFramesGoToStdoutNotStderr proves the stream contract
// for the spinner: spinner frames are narration → stdout, NEVER stderr. The real
// spinner factory is used (driven start→stop deterministically) and stderr is
// asserted to carry no braille frame glyph.
func TestPrettyPresenterSpinnerFramesGoToStdoutNotStderr(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.Ascii))
	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})

	for _, frame := range []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'} {
		if strings.ContainsRune(errBuf.String(), frame) {
			t.Errorf("braille spinner frame %q leaked to stderr:\n%q", frame, errBuf.String())
		}
	}
}
