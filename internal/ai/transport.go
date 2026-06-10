// Package ai is mint's content-agnostic AI transport — the "message out" half of
// the notes engine's two-part layering (context assembly is git-aware and lives
// elsewhere). A Transport takes a FINISHED prompt string and an ai_command, runs
// the command through the runner.CommandRunner seam (prompt on stdin, body on
// stdout), applies sanity validation, retries once on bad content, and returns the
// body or a typed failure. It knows nothing about git, diffs, tags, or the Change
// Map — pure "content in, message out" — which is exactly what keeps it trivially
// testable (a string + a fake ai_command) and lets the assembly side evolve
// without touching transport. This is the shared engine the sibling `mint commit`
// verb also consumes, with a different prompt.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mint/internal/runner"
)

// The transport returns TYPED, DISTINGUISHABLE causes so the on_notes_failure
// routing (a later task) can branch on what went wrong. All three are exposed as
// sentinels matched with errors.Is.
var (
	// ErrNotesFailure is a bad-content failure that SURVIVED the single retry:
	// empty, whitespace-only, or a non-zero command exit on BOTH attempts. It is the
	// "the AI could not produce a usable body" condition.
	ErrNotesFailure = errors.New("ai notes generation failed")

	// ErrTimeout is the per-attempt deadline expiring (a hung call). It is reported
	// immediately and NOT retried — retrying a hung call only risks a second full
	// timeout — so it is kept distinct from the bad-content ErrNotesFailure.
	ErrTimeout = errors.New("ai command timed out")

	// ErrCommandMissing is the ai_command binary not being found on PATH. Re-invoking
	// an absent binary cannot help, so it is reported immediately and NOT retried, and
	// kept distinct from the other causes.
	ErrCommandMissing = errors.New("ai command not found")
)

// defaultAICommand is the out-of-the-box transport command: `claude -p`, piping the
// composed prompt to stdin and reading the body off stdout.
const defaultAICommand = "claude -p"

// Config holds the operator-tunable transport settings.
//
// AICommand is the command invoked to generate notes (default `claude -p` when
// empty). It is whitespace-split into name + args; see Generate for why a simple
// split is sufficient.
//
// Timeout is the PER-ATTEMPT deadline applied to each invocation (including the
// retry). It is a configurable field — rather than a hard-coded ~60s — so tests can
// inject a tiny value and never actually wait a minute. A zero or negative Timeout
// falls back to the production default.
type Config struct {
	AICommand string
	Timeout   time.Duration
}

// defaultTimeout is the production per-attempt deadline (~60s) so a hung AI call
// cannot stall a release. Tests override it via Config.Timeout.
const defaultTimeout = 60 * time.Second

// Transport runs the AI command through the CommandRunner seam and validates the
// body. It holds the runner plus the resolved command and per-attempt timeout.
type Transport struct {
	runner  runner.CommandRunner
	command string
	timeout time.Duration
}

// NewTransport builds a Transport over r with cfg. An empty AICommand resolves to
// `claude -p` and a non-positive Timeout resolves to the ~60s production default,
// so the zero Config yields a fully working production transport. The runner is
// injected so production wiring passes the os/exec-backed runner and tests pass a
// FakeRunner.
func NewTransport(r runner.CommandRunner, cfg Config) *Transport {
	command := cfg.AICommand
	if strings.TrimSpace(command) == "" {
		command = defaultAICommand
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Transport{runner: r, command: command, timeout: timeout}
}

// Generate runs the AI command with prompt on stdin and returns the body from
// stdout, or a typed failure.
//
// The ai_command is parsed into name + args by a whitespace split (strings.Fields):
// it is operator-controlled config, not arbitrary user input, so it never carries
// quoting or shell metacharacters that a real parser would be needed for — a simple
// split is sufficient and documented as such.
//
// Each attempt gets its own deadline via context.WithTimeout(ctx, t.timeout), so the
// original call and the retry each get a fresh per-attempt budget. The prompt is
// piped fresh on every attempt (a new strings.NewReader) because an io.Reader is
// consumed once — reusing it on the retry would send an empty prompt.
//
// Failure routing is by cause:
//   - A timeout (the per-attempt context expiring, errors.Is DeadlineExceeded) is
//     reported immediately as ErrTimeout and NOT retried.
//   - A missing binary (errors.Is runner.ErrCommandNotFound) is reported immediately
//     as ErrCommandMissing and NOT retried.
//   - Bad CONTENT (empty, whitespace-only, or a non-zero exit) is retried EXACTLY
//     ONCE; if the second attempt is still bad it becomes ErrNotesFailure. A good
//     attempt (first or second) returns its body UNCHANGED.
func (t *Transport) Generate(ctx context.Context, prompt string) (string, error) {
	name, args := parseCommand(t.command)

	// First attempt. Timeout and missing-tool short-circuit without a retry; bad
	// content falls through to the single retry below.
	body, err := t.attempt(ctx, name, args, prompt)
	if err != nil {
		if cause := classifyFatal(err); cause != nil {
			return "", cause
		}
		// Bad content: fall through to the retry.
	} else if isValid(body) {
		return body, nil
	}

	// Single retry — covers empty/whitespace/error content only.
	body, err = t.attempt(ctx, name, args, prompt)
	if err != nil {
		if cause := classifyFatal(err); cause != nil {
			return "", cause
		}
		return "", ErrNotesFailure
	}
	if !isValid(body) {
		return "", ErrNotesFailure
	}
	return body, nil
}

// attempt runs a single AI invocation under its own per-attempt deadline, piping a
// FRESH reader over prompt to stdin (an io.Reader is consumed once, so the retry
// must re-create it) and returning the captured stdout.
func (t *Transport) attempt(ctx context.Context, name string, args []string, prompt string) (string, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	res, err := t.runner.RunWith(attemptCtx, strings.NewReader(prompt), name, args...)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

// classifyFatal maps a non-retryable runner error to its distinguishable sentinel,
// or returns nil for a retryable bad-content error (a plain non-zero exit). A
// timeout and a missing binary are the two fatal, non-retried causes.
func classifyFatal(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	case errors.Is(err, runner.ErrCommandNotFound):
		return fmt.Errorf("%w: %v", ErrCommandMissing, err)
	default:
		// A non-zero exit with no special cause is bad content — retryable.
		return nil
	}
}

// isValid reports whether body passes the sanity check: non-empty and not
// whitespace-only. Validation is sanity, NOT structure — there is no machine
// wrapper to validate, and refusal detection is deliberately MINIMAL: an empty or
// whitespace-only body, plus the non-zero-exit signalled separately via the command
// error, are the whole of "bad content". No refusal-sentinel heuristic is layered
// on; a non-empty body that ran zero-exit is trusted, and the human review gate is
// the backstop for style.
func isValid(body string) bool {
	return strings.TrimSpace(body) != ""
}

// parseCommand splits an ai_command into its binary name and argument list by
// whitespace. The command is operator-controlled config (e.g. `claude -p`), so a
// strings.Fields split is sufficient — it carries no quoting or shell syntax a
// fuller parser would be needed for. An all-whitespace command (already guarded
// against in NewTransport) yields an empty name.
func parseCommand(command string) (name string, args []string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
