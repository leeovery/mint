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

// newTransport builds a Transport over r with an explicit `claude -p` ai_command and
// a generous per-attempt timeout, the common setup for the content tests. The command
// is passed explicitly because the transport no longer self-defaults: config resolves
// the concrete command (its floor is config.DefaultAICommand) and hands it to the
// transport verbatim, so the test mirrors that by supplying a real command.
func newTransport(r runner.CommandRunner) *ai.Transport {
	return ai.NewTransport(r, ai.Config{AICommand: "claude -p", Timeout: generousTimeout})
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

func TestTransport_Generate_RunsPassedAICommandVerbatim(t *testing.T) {
	t.Parallel()

	// The transport runs the ai_command config hands it VERBATIM — it no longer carries
	// its own default. Config's floor (config.DefaultAICommand) guarantees a non-empty,
	// already-resolved command, so there is nothing for the transport to re-default. A
	// custom `mybot gen` must parse into name `mybot` + args `["gen"]` and be invoked
	// exactly — no `claude` substitution.
	r := runner.NewFakeRunner()
	r.Seed("mybot", runner.Result{Stdout: "ok\n"}, nil)

	tr := ai.NewTransport(r, ai.Config{AICommand: "mybot gen", Timeout: generousTimeout})
	if _, err := tr.Generate(t.Context(), "p"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	inv := r.Invocations()[0]
	if inv.Name != "mybot" {
		t.Errorf("command = %q, want mybot (the passed ai_command, not a re-default)", inv.Name)
	}
	if !equalArgs(inv.Args, []string{"gen"}) {
		t.Errorf("args = %v, want [gen]", inv.Args)
	}
	for _, got := range r.Invocations() {
		if got.Name == "claude" {
			t.Errorf("transport invoked claude — it must not re-default the passed command")
		}
	}
}

func TestTransport_Generate_PassesBlankAICommandThroughUnchanged(t *testing.T) {
	t.Parallel()

	// The transport no longer re-defaults a blank/whitespace ai_command to `claude -p`:
	// config's floor guarantees a non-empty command, so the old blank-re-default path is
	// dead code and has been removed. A blank command is carried through VERBATIM — it
	// whitespace-splits to an empty name (the defensive parseCommand no-op), and crucially
	// the transport must NOT substitute `claude`. Production never reaches this because the
	// config floor is asserted in the config-layer tests.
	r := runner.NewFakeRunner()
	// Seed the empty-name binary that a blank command parses to, so the call resolves
	// without an unseeded-command error.
	r.Seed("", runner.Result{Stdout: "ok\n"}, nil)

	tr := ai.NewTransport(r, ai.Config{AICommand: "  ", Timeout: generousTimeout})
	if _, err := tr.Generate(t.Context(), "p"); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	for _, got := range r.Invocations() {
		if got.Name == "claude" {
			t.Errorf("transport substituted claude for a blank command — the blank-re-default must be gone")
		}
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
	// the bad-content notes failure (ErrGenerationFailed) and the command was invoked
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
			if !errors.Is(err, ai.ErrGenerationFailed) {
				t.Fatalf("error = %v, want it to match ErrGenerationFailed", err)
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
	if errors.Is(err, ai.ErrGenerationFailed) {
		t.Errorf("timeout failure must be distinguishable from the bad-content ErrGenerationFailed")
	}
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("invocations = %d, want 1 (a timeout is not retried)", n)
	}
}

func TestTransport_Generate_DoesNotRetryCancel(t *testing.T) {
	t.Parallel()

	// A CALLER cancellation (Ctrl-C threading down from main's signal.NotifyContext)
	// is NOT retried — a retry against a dead context can never succeed — and it is
	// NOT an AI failure: it must propagate as context.Canceled itself, never as one of
	// the three transport sentinels, so sentinel-routing callers (release's
	// on_notes_failure, commit's editor fallback) treat it as a plain abort rather
	// than opening an editor for a user who just pressed Ctrl-C. Simulated by seeding
	// the error shape the runner produces on a cancel kill (it wraps ctx.Err()).
	r := runner.NewFakeRunner()
	r.Seed("claude", runner.Result{}, fmt.Errorf("running claude: %w", context.Canceled))

	_, err := newTransport(r).Generate(t.Context(), "p")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want it to match context.Canceled", err)
	}
	if errors.Is(err, ai.ErrGenerationFailed) || errors.Is(err, ai.ErrTimeout) || errors.Is(err, ai.ErrCommandMissing) {
		t.Errorf("a cancellation must not match any transport sentinel, got %v", err)
	}
	if n := len(r.Invocations()); n != 1 {
		t.Errorf("invocations = %d, want 1 (a cancellation is not retried)", n)
	}
}

func TestTransport_Generate_RealDeadlineKillIsNonRetriedTimeout(t *testing.T) {
	t.Parallel()

	// END-TO-END production path: a REAL exec.CommandContext deadline kill — not an
	// injected DeadlineExceeded wrapper — must classify as a timeout. A genuine deadline
	// makes exec.CommandContext SIGKILL the child, surfacing an *exec.ExitError; only
	// because the runner now wraps ctx.Err() does classifyFatal see
	// context.DeadlineExceeded and return ErrTimeout rather than misreading the kill as
	// bad content. This asserts ONLY the timing-robust classification — that a real
	// deadline kill maps to ErrTimeout and is distinguishable from ErrGenerationFailed.
	//
	// The "exactly one invocation / no retry on timeout" behaviour is covered
	// deterministically by TestTransport_Generate_DoesNotRetryTimeout (FakeRunner), so
	// this test deliberately asserts no invocation count: doing so would require a
	// subprocess side-effect (a marker write) that races process startup against the
	// deadline and flakes under CPU contention.
	//
	// The transport applies its own per-attempt timeout via context.WithTimeout, so a
	// tiny Config.Timeout against a command that sleeps far longer guarantees the
	// deadline fires regardless of load. The ai_command is whitespace-split (no shell
	// quoting), so the per-attempt body is a standalone executable script that simply
	// sleeps well past the deadline.
	dir := t.TempDir()
	script := filepath.Join(dir, "ai-command")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("writing fake ai_command script: %v", err)
	}

	// The 5s sleep guarantees the 300ms deadline fires mid-sleep — independent of how
	// quickly the subprocess starts — so the real timeout path is exercised on every run.
	tr := ai.NewTransport(runner.NewExecRunner(), ai.Config{
		AICommand: script,
		Timeout:   300 * time.Millisecond,
	})

	_, err := tr.Generate(t.Context(), "p")
	if !errors.Is(err, ai.ErrTimeout) {
		t.Fatalf("error = %v, want it to match ErrTimeout", err)
	}
	if errors.Is(err, ai.ErrGenerationFailed) {
		t.Errorf("a real timeout must be distinguishable from the bad-content ErrGenerationFailed")
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
	if errors.Is(err, ai.ErrGenerationFailed) {
		t.Errorf("missing-tool failure must be distinguishable from the bad-content ErrGenerationFailed")
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
