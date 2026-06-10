package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
)

// TestSuspendStopsSpinnerThenResumeRestartsItOnSameLine is the core
// suspend/resume lifecycle around the engine-driven $EDITOR hand-off: a blocking
// StageStarted starts a spinner; SuspendSpinner stops it (releasing the terminal
// so $EDITOR can take over) and clears the active handle; ResumeSpinner restarts a
// spinner on the SAME stage line — same dim start text. The spy proves the first
// spinner ends stopped and a fresh spinner is started with the identical text.
func TestSuspendStopsSpinnerThenResumeRestartsItOnSameLine(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.SuspendSpinner()
		p.ResumeSpinner()
	})

	if len(tr.created) != 2 {
		t.Fatalf("expected two spinners (the original then the resumed one), got %d", len(tr.created))
	}
	first := tr.created[0]
	if !first.started || !first.stopped {
		t.Errorf("the suspended spinner must be Started then Stopped: started=%v stopped=%v", first.started, first.stopped)
	}
	resumed := tr.created[1]
	if !resumed.started {
		t.Errorf("ResumeSpinner must Start a spinner again")
	}
	if resumed.text != first.text {
		t.Errorf("resumed spinner text = %q, want the same stage line text %q", resumed.text, first.text)
	}
}

// TestNoFramesBetweenSuspendAndResume proves the editor session is animation-free:
// while suspended (between SuspendSpinner and ResumeSpinner) there is NO active
// spinner. The tracker is driven directly here (not via drivePrettySpy) so the spy
// state can be snapshotted DURING the suspended window — at that point the sole
// spinner created so far must be stopped and the active count must be 0.
func TestNoFramesBetweenSuspendAndResume(t *testing.T) {
	out := &bytes.Buffer{}
	tr := &spyTracker{}
	p := presenter.NewPrettyPresenter(out, presenter.WithProfile(termenv.Ascii)).WithSpinnerFactory(tr.factory())

	p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
	p.SuspendSpinner()

	// Snapshot DURING the suspended window — the $EDITOR session. No spinner may be
	// animating, and no new spinner may have been created yet.
	if tr.active != 0 {
		t.Errorf("a spinner was animating during the suspended window: active = %d, want 0", tr.active)
	}
	if len(tr.created) != 1 {
		t.Errorf("a new spinner was created during the suspended window: %d created, want only the original", len(tr.created))
	}

	p.ResumeSpinner()

	// After resume the original is stopped and exactly one resumed spinner runs;
	// peak active never exceeded 1.
	if tr.maxActive > 1 {
		t.Errorf("more than one spinner active at once across suspend/resume: peak = %d", tr.maxActive)
	}
}

// TestSuspendResumeNoActiveSpinnerIsSafeNoOp proves both hooks are safe no-ops when
// no spinner is active: SuspendSpinner then ResumeSpinner with NO prior blocking
// stage creates no spinner, produces no output, and does not panic.
func TestSuspendResumeNoActiveSpinnerIsSafeNoOp(t *testing.T) {
	out, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.SuspendSpinner()
		p.ResumeSpinner()
	})

	if len(tr.created) != 0 {
		t.Errorf("no spinner must be created when none was active, got %d", len(tr.created))
	}
	if got := out.String(); got != "" {
		t.Errorf("suspend/resume with no active spinner must produce no output, got %q", got)
	}
}

// TestPlainSuspendResumeAreNoOps proves the plain presenter's hooks produce no
// output and do not error — plain never animates, so suspend/resume are pure
// no-ops.
func TestPlainSuspendResumeAreNoOps(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	p := presenter.NewPlainPresenter(out, errBuf)

	p.SuspendSpinner()
	p.ResumeSpinner()

	if got := out.String(); got != "" {
		t.Errorf("plain SuspendSpinner/ResumeSpinner wrote to out: %q", got)
	}
	if got := errBuf.String(); got != "" {
		t.Errorf("plain SuspendSpinner/ResumeSpinner wrote to err: %q", got)
	}
}

// TestRepeatedSuspendResumeCyclesNeverOrphanASpinner proves the one-at-a-time
// invariant holds across N suspend/resume cycles (repeated edit passes): after
// each stop/resume there is at most one spinner, the peak active count never
// exceeds 1, and after the final resume exactly one spinner is active (no orphan,
// no duplicate). A trailing StageSucceeded then drains it to zero.
func TestRepeatedSuspendResumeCyclesNeverOrphanASpinner(t *testing.T) {
	const cycles = 3
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		for i := 0; i < cycles; i++ {
			p.SuspendSpinner()
			p.ResumeSpinner()
		}
		// Drain the final resumed spinner with the stage's completion.
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})
	})

	if tr.maxActive > 1 {
		t.Errorf("more than one spinner active at once across %d cycles: peak = %d, want <= 1", cycles, tr.maxActive)
	}
	// One original + one per resume cycle = 1 + cycles spinners created.
	if want := 1 + cycles; len(tr.created) != want {
		t.Errorf("expected %d spinners across %d cycles, got %d", want, cycles, len(tr.created))
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active after the final completion: active = %d, want 0", tr.active)
	}
	// Every created spinner must have been stopped (none orphaned).
	for i, sp := range tr.created {
		if !sp.stopped {
			t.Errorf("spinner %d was orphaned (never stopped)", i)
		}
	}
}

// TestStageCompletingWhileSuspendedClearsSuspendedState proves a stage that
// completes while suspended clears the suspended state: StageStarted(blocking) →
// SuspendSpinner → StageSucceeded → ResumeSpinner must NOT resurrect a spinner for
// the already-completed stage. Only the original spinner is ever created.
func TestStageCompletingWhileSuspendedClearsSuspendedState(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.SuspendSpinner()
		p.StageSucceeded(presenter.StageSuccess{Name: "notes", Detail: "generated", Blocking: true})
		p.ResumeSpinner()
	})

	if len(tr.created) != 1 {
		t.Fatalf("ResumeSpinner resurrected a spinner for a completed stage: %d spinners created, want 1", len(tr.created))
	}
	if !tr.created[0].stopped {
		t.Errorf("the suspended spinner must be stopped, not left active")
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active after the stage completed while suspended: active = %d, want 0", tr.active)
	}
}

// TestStageFailedWhileSuspendedClearsSuspendedState mirrors the success case for
// the failure path: a stage that FAILS while suspended also clears the suspended
// state, so a later ResumeSpinner does not resurrect a spinner for the failed
// stage.
func TestStageFailedWhileSuspendedClearsSuspendedState(t *testing.T) {
	_, tr := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.SuspendSpinner()
		p.StageFailed(presenter.StageFailure{Name: "notes", Message: "boom"})
		p.ResumeSpinner()
	})

	if len(tr.created) != 1 {
		t.Fatalf("ResumeSpinner resurrected a spinner for a failed stage: %d spinners created, want 1", len(tr.created))
	}
	if tr.active != 0 {
		t.Errorf("a spinner was left active after the stage failed while suspended: active = %d, want 0", tr.active)
	}
}

// TestSuspendResumeRendersNoOutputItself proves the hooks are control-only: they
// emit no narration of their own — the only thing on out is the stage spinner's
// own (spy-silent) lifecycle, never an extra line. With the Ascii profile and the
// silent spy, a suspend/resume around a blocking stage produces no spurious bytes.
func TestSuspendResumeRendersNoOutputItself(t *testing.T) {
	out, _ := drivePrettySpy(t, func(p *presenter.PrettyPresenter) {
		p.StageStarted(presenter.StageStart{Name: "notes", Blocking: true})
		p.SuspendSpinner()
		p.ResumeSpinner()
	})

	// The spy spinner writes nothing, so the only possible output would be an
	// erroneous line synthesised by the hooks themselves.
	if got := out.String(); strings.TrimSpace(got) != "" {
		t.Errorf("suspend/resume hooks must emit no narration of their own, got %q", got)
	}
}

// TestPresenterInterfaceHasSuspendResume proves the interface gained the two hooks
// by driving them through Presenter-typed values (plain and pretty): the methods
// exist on the interface and are callable on both implementers.
func TestPresenterInterfaceHasSuspendResume(t *testing.T) {
	var p presenter.Presenter = presenter.NewPlainPresenter(&bytes.Buffer{}, &bytes.Buffer{})
	p.SuspendSpinner()
	p.ResumeSpinner()

	var pp presenter.Presenter = presenter.NewPrettyPresenter(&bytes.Buffer{}, presenter.WithProfile(termenv.Ascii))
	pp.SuspendSpinner()
	pp.ResumeSpinner()
}
