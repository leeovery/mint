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
// consolidated ExcludeConfig the forward path uses (diff_exclude globs + the
// strategy-aware version_file inputs), so the regenerate diff and Change Map apply the
// identical exclusion tiers — only the range differs. The transport is the notes.Transport
// seam (production wires the real ai.Transport; tests inject a recording fake).
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

	body, err := freshGenerator(r, transport, root, cfg).WithProgress(sinks.Progress).WithChunkNotes(sinks.ChunkNotes).WithMergeInputs(sinks.MergeInputs).GenerateFromRange(ctx, res.DiffRange(), cfg)
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

// RegenerateFreshRegenerator builds the PER-RUN Regenerator the fresh notes-review
// gate's `r` choice consults — the regenerate analogue of the forward path's
// perRunRegenerator. While it holds merged, at first run.Merged, the merge inputs of
// the chunked run that wrote the notes now at the gate, each `r` makes one merge call
// over those part notes with only its own one-time context in the release's
// instructions. Otherwise it binds a fresh generator, built as RegenerateFreshBody
// builds its own, to the version's resolved `{PreviousTag}..{Tag}` range, so an `r`
// re-runs the fresh AI path from the size check with the one-time context appended for
// that one generation, reporting to run's sinks; of a chunked run that `r` becomes, they
// save a failed one's finished chunk notes, and it holds a written one's merge inputs,
// handing them on to run.MergeInputs too. Either way the one-time context is
// TRANSIENT (never persisted), exactly as the forward path's `r`.
//
// For the oldest release (res.FirstRelease) there is no range to re-diff, so it mirrors
// RegenerateFreshBody's first-release rule: the fixed first-release body is returned
// with NO AI and NO diff, regardless of the one-time context — so `r` on a first-release
// fresh gate is well-defined and never breaks.
func RegenerateFreshRegenerator(r runner.CommandRunner, transport notes.Transport, root string, cfg config.Config, res version.Resolution, run RegenRun) Regenerator {
	merged := run.Merged
	generator := freshGenerator(r, transport, root, cfg).WithProgress(run.Progress).WithChunkNotes(run.ChunkNotes).WithMergeInputs(func(in notes.MergeInputs) {
		merged = &in
		if run.MergeInputs != nil {
			run.MergeInputs(in)
		}
	})
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		if res.FirstRelease {
			return record.FirstReleaseBody, nil
		}
		var body string
		var err error
		if merged != nil {
			body, err = generator.Remerge(ctx, *merged, cfg, oneTimeContext)
		} else {
			body, err = generator.GenerateFromRangeWithContext(ctx, res.DiffRange(), cfg, oneTimeContext)
		}
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
