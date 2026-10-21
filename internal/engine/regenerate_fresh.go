package engine

// This file is the regenerate FRESH SOURCE production (task 5-6): re-diffing the
// resolved `{PreviousTag}..{Tag}` range and running the AI to produce a fresh notes
// body, REUSING the forward notes engine end to end.
//
// The fresh path is the forward Stage-4 AI path less the exclusion check, over its own diff base.
// The forward path ranges `last_tag..HEAD`; the fresh path ranges 5-3's resolved
// DiffRange (`vX-1..vX`). Every other layer is reused unchanged — the consolidated
// exclusion tiers (built-in CHANGELOG.md and .mint.toml, the shared and [release] diff_exclude globs,
// the strategy-aware version_file decision), the Change Map computed AFTER exclusion and
// prepended to the AI input, the max_prompt_bytes size check with chunking over it,
// and the AI validation/retry.
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
// swallowed — a single-version run answers it at the notes floor's choice when attended
// and aborts otherwise, and `--all` skips the version and carries on. This function
// owns the fresh-source body PRODUCTION only, not the provider/changelog write and push.

import (
	"context"
	"fmt"
	"slices"

	"mint/internal/aitransport"
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
// ExcludeConfig the forward path uses (notesExcludeConfig), so the regenerate diff and
// Change Map apply the identical exclusion tiers — only the range differs. The transport
// is the notes.Transport seam (production wires the real ai.Transport; tests inject a
// recording fake).
//
// The body is returned WHOLE (no parsing/splitting) for the downstream
// provider/changelog write. An AI/transport failure is surfaced. A chunked run reports
// to sinks and, when it fails, saves its finished chunk notes there; a written one
// hands sinks the merge inputs its body was merged from.
func RegenerateFreshBody(ctx context.Context, r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config, res version.Resolution, sinks NotesSinks) (string, error) {
	// Oldest release: mirror the forward first-release rule — fixed body, no AI, no
	// diff. DiffRange would return "" here, so this guard MUST precede any assembly.
	if res.FirstRelease {
		return record.FirstReleaseBody, nil
	}

	body, err := sinks.wire(freshGenerator(r, transport, root, cfg)).GenerateFromRange(ctx, res.DiffRange(), cfg)
	if err != nil {
		return "", fmt.Errorf("regenerating fresh notes for %s: %w", res.Tag, err)
	}
	return body, nil
}

// NotesSinks is where a fresh generation reports a chunked run's advance, saves a
// failed one's finished chunk notes, and hands a written one's merge inputs. A nil
// field reports, saves or is handed nothing.
type NotesSinks struct {
	Progress    notes.Progress
	ChunkNotes  notes.SaveChunkNotes
	MergeInputs func(notes.MergeInputs)
}

// wire returns a copy of generator reporting to, saving to and handing to s.
func (s NotesSinks) wire(generator *notes.Generator) *notes.Generator {
	return generator.WithProgress(s.Progress).WithChunkNotes(s.ChunkNotes).WithMergeInputs(s.MergeInputs)
}

// RegenerateFreshRegenerator builds the PER-RUN Regenerator the fresh notes-review
// gate's `r` choice consults — the regenerate analogue of the forward path's
// perRunRegenerator, sharing its `r` rule (RegenRun.regenerator) over run. Its size-check
// generation re-runs the fresh AI path, over a generator built as RegenerateFreshBody
// builds its own, over the version's resolved `{PreviousTag}..{Tag}` range. The
// one-time context is TRANSIENT (never persisted), exactly as the forward path's `r`.
//
// For the oldest release (res.FirstRelease) there is no range to re-diff, so it mirrors
// RegenerateFreshBody's first-release rule: the fixed first-release body is returned
// with NO AI and NO diff, regardless of the one-time context — so `r` on a first-release
// fresh gate is well-defined and never breaks.
func RegenerateFreshRegenerator(r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config, res version.Resolution, run RegenRun) Regenerator {
	regenerate := run.regenerator(freshGenerator(r, transport, root, cfg), cfg, func(ctx context.Context, generator *notes.Generator, oneTimeContext string) (string, error) {
		return generator.GenerateFromRangeWithContext(ctx, res.DiffRange(), cfg, oneTimeContext)
	})
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		if res.FirstRelease {
			return record.FirstReleaseBody, nil
		}
		body, err := regenerate(ctx, oneTimeContext)
		if err != nil {
			return "", fmt.Errorf("regenerating fresh notes for %s: %w", res.Tag, err)
		}
		return body, nil
	})
}

// freshGenerator builds the notes Generator the fresh regenerate path drives: the
// Assembler with the consolidated fresh ExcludeConfig (the SAME tiers the forward path
// uses) over the resolved transport. RegenerateFreshBody and RegenerateFreshRegenerator
// each build their own through it, so an `r` that re-runs the AI path ranges over
// exactly what the initial body production did and a re-merge goes through the same
// release transport.
func freshGenerator(r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config) *notes.Generator {
	assembler := notes.NewAssembler(r, notesExcludeConfig(cfg))
	return notes.NewGenerator(assembler, resolveFreshTransport(r, transport, cfg), root)
}

// notesExcludeConfig is the exclusion set release notes are built over: a forward
// release and a regenerate of the same range must read the same paths.
func notesExcludeConfig(cfg config.Config) notes.ExcludeConfig {
	return notes.ExcludeConfig{
		Globs:          slices.Concat(cfg.DiffExclude, cfg.Release.DiffExclude),
		VersionFile:    cfg.Release.VersionFile,
		VersionPattern: cfg.Release.VersionPattern,
	}
}

// resolveFreshTransport mirrors the forward aiTransport seam: the injected transport
// when set (the test fake), else the production ai.Transport over the run's runner — so
// production passes nil and gets the real transport. This is the EASY-MISS third
// construction site, and it deliberately resolves through the RELEASE verb: it calls the
// shared aitransport.New helper with config.VerbRelease. Regenerate re-runs the
// release-notes task and shares release's salience needs and timeout exposure, and there
// is NO [regenerate] table — so the fresh-regenerate AI call reads [release], NOT [commit]
// and NOT the bare shared/default. The closed Verb enum has no `regenerate` value, so
// VerbRelease is the ONLY verb this site can pass. The `[release] → shared → floor`
// resolution and the never-zero-by-omission timeout contract live once in that helper; the
// injected-transport short-circuit is this site's own (its transport-arg seam differs from
// the other sites' deps wrappers) and stays here — only the construction expression is
// shared.
func resolveFreshTransport(r runner.CommandRunner, transport notes.Transport, cfg config.Config) notes.Transport {
	if transport != nil {
		return transport
	}
	return aitransport.New(r, cfg, config.VerbRelease)
}
