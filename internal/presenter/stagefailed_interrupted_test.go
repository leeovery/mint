package presenter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter"
)

// interruptedHook is the pre_tag hook failing as Ctrl-C cancelled the run, its
// command's output captured.
func interruptedHook() presenter.StageFailure {
	return presenter.StageFailure{
		Name:        "pre_tag",
		Key:         "pre_tag",
		Message:     `hook entry "go test -race ./..." failed (exit -1): running "sh": context canceled`,
		Output:      "ok  \tpigeon/webhooks\t0.412s",
		Interrupted: true,
	}
}

func TestStageFailed_Interrupted_TheRailSaysSoAndKeepsTheOutput(t *testing.T) {
	t.Parallel()

	r := newRailTerm(t, 100, 24, answers())
	r.StageFailed(interruptedHook())
	screen := r.screen(t)
	if !slices.ContainsFunc(screen, func(ln string) bool { return strings.HasPrefix(ln, " ✗  pre_tag       interrupted") }) {
		t.Errorf("the rail drew\n%s\nwant the node failed as interrupted", strings.Join(screen, "\n"))
	}
	joined := strings.Join(screen, "\n")
	if strings.Contains(joined, "context canceled") {
		t.Errorf("the rail shows the runner's words about the cancel:\n%s", joined)
	}
	if !strings.Contains(joined, "pigeon/webhooks") {
		t.Errorf("the rail dropped the hook's output:\n%s", joined)
	}
}

func TestPlainPresenterStageFailed_Interrupted_KeepsItsLine(t *testing.T) {
	t.Parallel()

	failure := interruptedHook()
	out, _ := drive(func(p *presenter.PlainPresenter) { p.StageFailed(failure) })
	failure.Interrupted = false
	want, _ := drive(func(p *presenter.PlainPresenter) { p.StageFailed(failure) })
	if out.String() != want.String() {
		t.Errorf("plain StageFailed interrupted = %q, want its line as ever %q", out.String(), want.String())
	}
}

// forcedHook is the pre_tag hook failing as the second press forced it to stop.
func forcedHook() presenter.StageFailure {
	failure := interruptedHook()
	failure.Message = `hook entry "go test -race ./..." failed (exit -1): running "sh": forced to stop: context canceled`
	failure.Forced, failure.HookEntry = true, "go test -race ./..."
	return failure
}

func TestStageFailed_Forced_TheRailSaysTheHookWasForcedAndKeepsTheOutput(t *testing.T) {
	t.Parallel()

	r := newRailTerm(t, 100, 24, answers())
	r.StageFailed(forcedHook())
	screen := r.screen(t)
	if !slices.ContainsFunc(screen, func(ln string) bool {
		return strings.HasPrefix(ln, " ✗  pre_tag       interrupted, hook forced to stop")
	}) {
		t.Errorf("the rail drew\n%s\nwant the node failed as forced", strings.Join(screen, "\n"))
	}
	if joined := strings.Join(screen, "\n"); !strings.Contains(joined, "pigeon/webhooks") {
		t.Errorf("the rail dropped the hook's output:\n%s", joined)
	}
}

func TestPlainPresenterStageFailed_Forced_SaysTheHookWasForced(t *testing.T) {
	t.Parallel()

	out, errBuf := drive(func(p *presenter.PlainPresenter) { p.StageFailed(forcedHook()) })

	wantOut := "pre_tag: FAILED - interrupted, hook forced to stop (hook entry \"go test -race ./...\")\n" +
		"--- output ---\n" +
		"ok  \tpigeon/webhooks\t0.412s\n" +
		"--- end output ---\n"
	if got := out.String(); got != wantOut {
		t.Errorf("out =\n%q\nwant\n%q", got, wantOut)
	}
	if got, want := errBuf.String(), "pre_tag: FAILED - interrupted, hook forced to stop (hook entry \"go test -race ./...\")\n"; got != want {
		t.Errorf("err = %q, want %q", got, want)
	}
}

func TestInterrupted_IsTheRunsCancelInTheChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cause error
		want  bool
	}{
		{"the cancel itself", context.Canceled, true},
		{"the cancel wrapped by the runner", fmt.Errorf("running %q: %w", "sh", context.Canceled), true},
		{"a timeout", fmt.Errorf("running %q: %w", "claude", context.DeadlineExceeded), false},
		{"any other failure", errors.New("exit status 1"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := presenter.Interrupted(tt.cause); got != tt.want {
				t.Errorf("Interrupted(%v) = %v, want %v", tt.cause, got, tt.want)
			}
		})
	}
}
