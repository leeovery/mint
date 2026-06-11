package ai_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/runner"
)

// generousTimeout is a per-attempt deadline large enough never to expire during a
// fast, in-memory FakeRunner call. The transport's timeout is a configurable
// field precisely so tests never wait the production ~60s.
const generousTimeout = time.Minute

// newTransport builds a Transport over r with the default ai_command and a
// generous per-attempt timeout, the common setup for the content tests.
func newTransport(r runner.CommandRunner) *ai.Transport {
	return ai.NewTransport(r, ai.Config{Timeout: generousTimeout})
}

func TestTransport_Generate_ReturnsValidBodyUnchanged(t *testing.T) {
	t.Parallel()

	// A good body — non-empty, non-whitespace, zero-exit — is returned WHOLE and
	// UNCHANGED: no trimming or normalisation. mint uses the body verbatim for every
	// sink, so the transport must not touch a valid generation (leading/trailing
	// newlines and internal whitespace are part of the presentation body).
	const body = "## TL;DR\n\nShipped the thing.\n\n✨ Added\n- A feature\n"
	r := runner.NewFakeRunner()
	r.Seed("claude", runner.Result{Stdout: body}, nil)

	got, err := newTransport(r).Generate(t.Context(), "the prompt")
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want it returned unchanged %q", got, body)
	}

	// Exactly one attempt: a good first body must never trigger the retry.
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("invocations = %d, want 1 (no retry on a good body)", n)
	}
}

func TestTransport_Generate_PipesPromptToStdinReadsStdout(t *testing.T) {
	t.Parallel()

	// The prompt is delivered on STDIN and the body is read from STDOUT — the engine
	// contract for `claude -p`. This pins the wiring: the recorded Stdin must be the
	// exact prompt, and the returned body must be exactly what the command wrote to
	// stdout (Stderr is ignored on success).
	const prompt = "describe these changes"
	const body = "described\n"
	r := runner.NewFakeRunner()
	r.Seed("claude", runner.Result{Stdout: body, Stderr: "warning: noise on stderr\n"}, nil)

	got, err := newTransport(r).Generate(t.Context(), prompt)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want stdout %q", got, body)
	}

	inv := r.Invocations()[0]
	if inv.Stdin != prompt {
		t.Errorf("stdin = %q, want the prompt %q", inv.Stdin, prompt)
	}
}

func TestTransport_Generate_DefaultCommandIsClaudeDashP(t *testing.T) {
	t.Parallel()

	// With ai_command left unset, the default is `claude -p`: name `claude`, args
	// `["-p"]`. The split is a simple whitespace split (operator-controlled config),
	// so the default must parse into exactly that name+args.
	r := runner.NewFakeRunner()
	r.Seed("claude", runner.Result{Stdout: "ok\n"}, nil)

	// No AICommand set -> default applies.
	tr := ai.NewTransport(r, ai.Config{Timeout: generousTimeout})
	if _, err := tr.Generate(t.Context(), "p"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := r.Invocations()[0]
	if inv.Name != "claude" {
		t.Errorf("command = %q, want claude", inv.Name)
	}
	if !equalArgs(inv.Args, []string{"-p"}) {
		t.Errorf("args = %v, want [-p]", inv.Args)
	}
}

func TestTransport_Generate_HonoursOverriddenAICommand(t *testing.T) {
	t.Parallel()

	// ai_command is overridable: a custom command is whitespace-split into name +
	// args and invoked exactly. This guards the swap-the-binary/model future-proofing
	// — mint owns the prompt, the command is just transport.
	r := runner.NewFakeRunner()
	r.Seed("llm", runner.Result{Stdout: "ok\n"}, nil)

	tr := ai.NewTransport(r, ai.Config{
		AICommand: "llm --model gpt-4 chat",
		Timeout:   generousTimeout,
	})
	if _, err := tr.Generate(t.Context(), "p"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := r.Invocations()[0]
	if inv.Name != "llm" {
		t.Errorf("command = %q, want llm", inv.Name)
	}
	if !equalArgs(inv.Args, []string{"--model", "gpt-4", "chat"}) {
		t.Errorf("args = %v, want [--model gpt-4 chat]", inv.Args)
	}
}

func TestTransport_Generate_RetriesOnceThenFailsOnBadContent(t *testing.T) {
	t.Parallel()

	// Bad CONTENT — empty, whitespace-only, or a non-zero exit (error/refusal) —
	// triggers EXACTLY ONE retry. If the second attempt is still bad, Generate returns
	// the bad-content notes failure (ErrNotesFailure) and the command was invoked
	// exactly twice (the original + one retry, no more).
	tests := []struct {
		name   string
		result runner.Result
		err    error
	}{
		{name: "empty body", result: runner.Result{Stdout: ""}, err: nil},
		{name: "whitespace-only body", result: runner.Result{Stdout: "   \n\t\n"}, err: nil},
		{name: "non-zero exit (error/refusal)", result: runner.Result{Stdout: "I cannot help", ExitCode: 1}, err: errors.New("exit status 1")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("claude", tt.result, tt.err)

			_, err := newTransport(r).Generate(t.Context(), "p")
			if !errors.Is(err, ai.ErrNotesFailure) {
				t.Fatalf("error = %v, want it to match ErrNotesFailure", err)
			}
			// Timeout and missing-tool are NOT this failure — keep them distinguishable.
			if errors.Is(err, ai.ErrTimeout) {
				t.Errorf("bad-content failure must not match ErrTimeout")
			}
			if errors.Is(err, ai.ErrCommandMissing) {
				t.Errorf("bad-content failure must not match ErrCommandMissing")
			}

			if n := len(r.Invocations()); n != 2 {
				t.Errorf("invocations = %d, want 2 (original + exactly one retry)", n)
			}
		})
	}
}

func TestTransport_Generate_SucceedsOnSecondAttemptWhenFirstBad(t *testing.T) {
	t.Parallel()

	// The single retry recovers a transient bad generation: a bad first attempt
	// followed by a good second attempt returns that good body. Both attempts hit the
	// SAME command, modelled with SeedSequence so the first call is bad and the second
	// is good.
	const good = "## TL;DR\n\nRecovered on retry.\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("claude",
		runner.ScriptedCall{Result: runner.Result{Stdout: "   "}},
		runner.ScriptedCall{Result: runner.Result{Stdout: good}},
	)

	got, err := newTransport(r).Generate(t.Context(), "p")
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}
	if got != good {
		t.Errorf("body = %q, want the good second-attempt body %q", got, good)
	}
	if n := len(r.Invocations()); n != 2 {
		t.Errorf("invocations = %d, want 2 (bad first, good second)", n)
	}
}

func TestTransport_Generate_RetryRepipesPromptFreshOnStdin(t *testing.T) {
	t.Parallel()

	// CRITICAL: an io.Reader is consumed once, so the retry MUST create a fresh
	// reader over the prompt. If the transport reused the first attempt's reader, the
	// retry would send an EMPTY stdin. Assert BOTH recorded invocations carry the full
	// prompt — the regression this guards is a silently-empty retry prompt.
	const prompt = "the full prompt that must survive the retry"
	r := runner.NewFakeRunner()
	r.SeedSequence("claude",
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "good\n"}},
	)

	if _, err := newTransport(r).Generate(t.Context(), prompt); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2", len(invs))
	}
	for i, inv := range invs {
		if inv.Stdin != prompt {
			t.Errorf("attempt %d stdin = %q, want the full prompt %q", i+1, inv.Stdin, prompt)
		}
	}
}

func TestTransport_Generate_DoesNotRetryTimeout(t *testing.T) {
	t.Parallel()

	// A timeout is NOT retried — retrying a hung call only risks a second full
	// timeout. It goes straight to a DISTINGUISHABLE timeout failure (ErrTimeout) so
	// task 2-7's on_notes_failure routing can branch on it, and the command is invoked
	// exactly ONCE. Simulated by seeding an error that wraps context.DeadlineExceeded.
	r := runner.NewFakeRunner()
	r.Seed("claude", runner.Result{}, fmt.Errorf("running claude: %w", context.DeadlineExceeded))

	_, err := newTransport(r).Generate(t.Context(), "p")
	if !errors.Is(err, ai.ErrTimeout) {
		t.Fatalf("error = %v, want it to match ErrTimeout", err)
	}
	if errors.Is(err, ai.ErrNotesFailure) {
		t.Errorf("timeout failure must be distinguishable from the bad-content ErrNotesFailure")
	}
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("invocations = %d, want 1 (a timeout is not retried)", n)
	}
}

func TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout(t *testing.T) {
	t.Parallel()

	// END-TO-END production path: a REAL exec.CommandContext deadline kill — not an
	// injected DeadlineExceeded wrapper — must classify as a non-retried timeout. A
	// genuine deadline makes exec.CommandContext SIGKILL the child, surfacing an
	// *exec.ExitError; only because the runner now wraps ctx.Err() does classifyFatal
	// see context.DeadlineExceeded and return ErrTimeout WITHOUT a second attempt.
	// Without the fix the kill is misread as bad content and retried, doubling the
	// worst-case latency to two full timeouts.
	//
	// The transport applies its own per-attempt timeout via context.WithTimeout, so a
	// tiny Config.Timeout against a command that sleeps far longer guarantees the
	// deadline fires. The ai_command is whitespace-split (no shell quoting), so the
	// per-attempt body is a standalone executable script — appending a byte on every
	// invocation, then sleeping past the deadline. Counting the bytes proves the real
	// subprocess ran exactly ONCE (no retry) — the equivalent of the FakeRunner
	// invocation count for the real ExecRunner.
	dir := t.TempDir()
	marker := filepath.Join(dir, "invocations")
	script := filepath.Join(dir, "ai-command")
	body := fmt.Sprintf("#!/bin/sh\nprintf x >> %q\nsleep 5\n", marker)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("writing fake ai_command script: %v", err)
	}

	// 300ms is generous enough for the subprocess to reliably fork, open the marker,
	// and write its byte before any deadline kill (a tighter budget races the SIGKILL
	// against process startup and flakes), yet the 5s sleep guarantees the deadline
	// still fires mid-sleep so the timeout path is genuinely exercised.
	tr := ai.NewTransport(runner.NewExecRunner(), ai.Config{
		AICommand: script,
		Timeout:   300 * time.Millisecond,
	})

	_, err := tr.Generate(t.Context(), "p")
	if !errors.Is(err, ai.ErrTimeout) {
		t.Fatalf("error = %v, want it to match ErrTimeout", err)
	}
	if errors.Is(err, ai.ErrNotesFailure) {
		t.Errorf("a real timeout must be distinguishable from the bad-content ErrNotesFailure")
	}

	got, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("reading invocation marker: %v", readErr)
	}
	if n := len(got); n != 1 {
		t.Errorf("subprocess ran %d time(s), want 1 (a timeout is not retried)", n)
	}
}

func TestTransport_Generate_DoesNotRetryMissingTool(t *testing.T) {
	t.Parallel()

	// A missing AI tool (command-not-found) is NOT retried — re-invoking an absent
	// binary cannot help. It surfaces a DISTINGUISHABLE missing-tool failure
	// (ErrCommandMissing) so on_notes_failure can branch on it, and the command is
	// invoked exactly ONCE. Simulated with SeedNotFound (matches ErrCommandNotFound).
	r := runner.NewFakeRunner()
	r.SeedNotFound("claude")

	_, err := newTransport(r).Generate(t.Context(), "p")
	if !errors.Is(err, ai.ErrCommandMissing) {
		t.Fatalf("error = %v, want it to match ErrCommandMissing", err)
	}
	if errors.Is(err, ai.ErrNotesFailure) {
		t.Errorf("missing-tool failure must be distinguishable from the bad-content ErrNotesFailure")
	}
	if errors.Is(err, ai.ErrTimeout) {
		t.Errorf("missing-tool failure must be distinguishable from ErrTimeout")
	}
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("invocations = %d, want 1 (a missing tool is not retried)", n)
	}
}

// equalArgs reports whether two argument slices are element-for-element equal, so
// command-line assertions check the exact argv rather than a substring.
func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
