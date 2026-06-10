package hooks

import (
	"context"
	"fmt"

	"mint/internal/runner"
)

// Runner executes a lifecycle hook's command(s) through the shared CommandRunner
// seam, so the whole mechanism — sh -c, repo root cwd, MINT_* injection, ordered
// execution, first-failure stop — is driven and asserted in tests without
// spawning processes.
type Runner struct {
	runner runner.CommandRunner
}

// NewRunner returns a Runner that executes hook commands via r.
func NewRunner(r runner.CommandRunner) *Runner {
	return &Runner{runner: r}
}

// HookError carries the outcome of a hook entry that exited non-zero, so a caller
// can both branch on the wrapped runner error (errors.Is) and inspect the failing
// entry plus its captured Result (errors.As) when deciding whether to abort or
// warn.
type HookError struct {
	// Entry is the shell command string that failed.
	Entry string
	// Result is the failing entry's captured output and exit code.
	Result runner.Result
	// err is the underlying runner error, exposed via Unwrap for errors.Is.
	err error
}

func (e *HookError) Error() string {
	return fmt.Sprintf("hook entry %q failed (exit %d): %v", e.Entry, e.Result.ExitCode, e.err)
}

// Unwrap exposes the underlying runner error so errors.Is matches it.
func (e *HookError) Unwrap() error { return e.err }

// Run executes the hook described by value from repoRoot with env injected. value
// is the parsed [release.hooks] entry, normalised to an ordered list of command
// strings; an absent/empty value (nil, "", or an empty slice) is a no-op that
// returns nil. Each entry runs as `sh -c "<entry>"` from repoRoot with the MINT_*
// variables layered on the inherited environment. Entries run in declared order
// and the FIRST non-zero exit stops the sequence — later entries do not run — and
// is returned as a *HookError wrapping the runner error. Whether that failure
// aborts the release or merely warns is the caller's decision, not this method's.
func (r *Runner) Run(ctx context.Context, value any, repoRoot string, env HookEnv) error {
	entries := normalise(value)
	if len(entries) == 0 {
		return nil
	}

	renderedEnv := env.Render()
	for _, entry := range entries {
		res, err := r.runner.RunInDir(ctx, repoRoot, renderedEnv, "sh", "-c", entry)
		if err != nil {
			return &HookError{Entry: entry, Result: res, err: err}
		}
	}
	return nil
}

// normalise coerces a parsed hook value into an ordered list of command strings:
//   - a string            -> one entry (empty string -> zero entries)
//   - a []string          -> entries in declared order
//   - a []any             -> elements coerced to strings in order (how some TOML
//     decoders surface a string array)
//   - nil / empty slice   -> zero entries (no-op)
//
// A non-string element in a []any is skipped rather than guessed at; full schema
// validation is a later phase, and this package only needs the shell-command
// strings.
func normalise(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return normaliseString(v)
	case []string:
		return normaliseStringSlice(v)
	case []any:
		return normaliseAnySlice(v)
	default:
		return nil
	}
}

// normaliseString yields one entry for a non-empty string and none for "".
func normaliseString(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// normaliseStringSlice keeps non-empty entries in declared order.
func normaliseStringSlice(values []string) []string {
	entries := make([]string, 0, len(values))
	for _, s := range values {
		if s != "" {
			entries = append(entries, s)
		}
	}
	return entries
}

// normaliseAnySlice coerces string elements in declared order, skipping anything
// that is not a non-empty string.
func normaliseAnySlice(values []any) []string {
	entries := make([]string, 0, len(values))
	for _, item := range values {
		if s, ok := item.(string); ok && s != "" {
			entries = append(entries, s)
		}
	}
	return entries
}
