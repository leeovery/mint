package engine

// This file is the regenerate FRESH SOURCE production (task 5-6): re-diffing the
// resolved `{PreviousTag}..{Tag}` range and running the AI to produce a fresh notes
// body, REUSING the forward notes engine end to end.
//
// The fresh path is the forward Stage-4 AI path with ONE substitution: the diff base.
// The forward path ranges `last_tag..HEAD`; the fresh path ranges 5-3's resolved
// DiffRange (`vX-1..vX`). Every other layer is reused unchanged — the consolidated
// exclusion tiers (built-in CHANGELOG.md, configured diff_exclude globs, the
// strategy-aware version_file decision), the Change Map computed AFTER exclusion and
// prepended to the AI input, the max_diff_lines guard, and the AI validation/retry.
//
// Exclusion is PATH-based, not commit-based: the `vX-1..vX` range already contains
// mint's release-bookkeeping commit (`{commit_prefix} Release {tag}`), and a range
// diff cannot subtract a commit. The :(exclude) pathspecs (CHANGELOG.md +
// plain-mode version_file) drop exactly the bookkeeping PATHS, which is what
// reproduces the forward path's source view — so the fresh re-diff matches what the
// forward run originally fed the AI. No commit surgery is attempted.
//
// The oldest release (5-3's FirstRelease) mirrors the forward first-release rule:
// the fixed `record.FirstReleaseBody` ("Initial release.") is returned with NO AI and
// NO diff.
//
// FAILURE ROUTING: an AI failure is SURFACED (wrapped, errors.Is still matches), not
// swallowed — single-mode fresh follows the on_notes_failure default abort at a higher
// layer, and 5-12's `--all` intercepts the surfaced failure for skip-and-continue. This
// function owns the fresh-source body PRODUCTION only; the provider/changelog write and
// push are later tasks (5-7/5-8).

import (
	"context"
	"fmt"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/record"
	"mint/internal/runner"
	"mint/internal/version"
)

// RegenerateFreshBody produces the fresh-source release-notes body for the resolved
// regenerate target. For the oldest release (res.FirstRelease) it returns the fixed
// first-release body with no AI and no diff; otherwise it re-diffs the resolved
// `{PreviousTag}..{Tag}` range and runs the forward AI path over it.
//
// It REUSES the forward notes engine: the Assembler is built with the SAME
// consolidated ExcludeConfig the forward path uses (diff_exclude globs + the
// strategy-aware version_file inputs), so the regenerate diff and Change Map apply the
// identical exclusion tiers — only the range differs. The transport is the notes.Transport
// seam (production wires the real ai.Transport; tests inject a recording fake).
//
// The body is returned WHOLE (no parsing/splitting) for the downstream
// provider/changelog write in later tasks. An AI/transport failure is surfaced.
func RegenerateFreshBody(ctx context.Context, r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config, res version.Resolution) (string, error) {
	// Oldest release: mirror the forward first-release rule — fixed body, no AI, no
	// diff. DiffRange would return "" here, so this guard MUST precede any assembly.
	if res.FirstRelease {
		return record.FirstReleaseBody, nil
	}

	body, err := freshGenerator(r, transport, root, cfg).GenerateFromRange(ctx, res.DiffRange(), cfg)
	if err != nil {
		return "", fmt.Errorf("regenerating fresh notes for %s: %w", res.Tag, err)
	}
	return body, nil
}

// RegenerateFreshRegenerator builds the PER-RUN Regenerator the fresh notes-review
// gate's `r` choice consults — the regenerate analogue of the forward path's
// perRunRegenerator. It binds the SAME fresh generator RegenerateFreshBody builds to
// the version's resolved `{PreviousTag}..{Tag}` range, so an `r` re-runs the IDENTICAL
// fresh AI path the body producer took, with the user's one-time context appended for
// that one generation. The one-time context is TRANSIENT (appended to the prompt only,
// never persisted), exactly as the forward path's `r`.
//
// For the oldest release (res.FirstRelease) there is no range to re-diff, so it mirrors
// RegenerateFreshBody's first-release rule: the fixed first-release body is returned
// with NO AI and NO diff, regardless of the one-time context — so `r` on a first-release
// fresh gate is well-defined and never breaks.
func RegenerateFreshRegenerator(r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config, res version.Resolution) Regenerator {
	generator := freshGenerator(r, transport, root, cfg)
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		if res.FirstRelease {
			return record.FirstReleaseBody, nil
		}
		body, err := generator.GenerateFromRangeWithContext(ctx, res.DiffRange(), cfg, oneTimeContext)
		if err != nil {
			return "", fmt.Errorf("regenerating fresh notes for %s: %w", res.Tag, err)
		}
		return body, nil
	})
}

// freshGenerator builds the notes Generator the fresh regenerate path drives: the
// Assembler with the consolidated fresh ExcludeConfig (the SAME tiers the forward path
// uses) over the resolved transport. Both RegenerateFreshBody and
// RegenerateFreshRegenerator build it identically, so an `r` re-run ranges over exactly
// what the initial body production did — only the range range/context differ.
func freshGenerator(r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config) *notes.Generator {
	assembler := notes.NewAssembler(r, freshExcludeConfig(cfg))
	return notes.NewGenerator(assembler, resolveFreshTransport(r, transport, cfg), root)
}

// freshExcludeConfig builds the consolidated ExcludeConfig the fresh Assembler layers
// on top of the built-in CHANGELOG.md always-exclude — the SAME tiers the forward
// path's resolveBody threads: the configured diff_exclude globs AND the strategy-aware
// version_file inputs (plain mode excludes the whole-file version; embedded mode keeps
// the real source). It is the exact forward decision, consumed over the regenerate
// range — the tier logic is reused, never reimplemented.
func freshExcludeConfig(cfg config.Config) notes.ExcludeConfig {
	return notes.ExcludeConfig{
		Globs:          cfg.DiffExclude,
		VersionFile:    cfg.Release.VersionFile,
		VersionPattern: cfg.Release.VersionPattern,
	}
}

// resolveFreshTransport mirrors the forward aiTransport seam: the injected transport
// when set (the test fake), else the production ai.Transport over the run's runner —
// so production passes nil and gets the real transport. The validated cfg.AICommand
// drives the invocation (the same documented top-level ai_command key the forward path
// reads); NewTransport re-defaults an empty value to `claude -p`, so a zero-config run
// uses the documented default exactly.
func resolveFreshTransport(r runner.CommandRunner, transport notes.Transport, cfg config.Config) notes.Transport {
	if transport != nil {
		return transport
	}
	// TODO(2-3/2-4/2-5): thread config.TimeoutFor(VerbRelease) — regenerate rides on
	// [release], so the resolved timeout must come from the release verb, not its own
	// table. Temporary compile-bridge for task 2-2 (which changed ai.Config.Timeout to
	// *time.Duration with a strict nil-is-wiring-bug guard). A non-nil pointer to the 60s
	// floor keeps this off the nil (panic) path until the resolved timeout is wired here.
	timeout := config.DefaultTimeout
	return ai.NewTransport(r, ai.Config{AICommand: cfg.AICommand, Timeout: &timeout})
}
