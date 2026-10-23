package notes

import (
	"context"
	"fmt"

	"mint/internal/config"
)

// Transport is the content-agnostic AI seam the generator depends on: a finished
// prompt in, a validated body (or a typed failure) out. It is defined HERE, where
// it is consumed, so the notes engine stays decoupled from the ai package's
// concretions — ai.Transport satisfies it in production, while tests inject a
// recording fake that captures the prompt and returns a scripted body without
// scripting the real `claude` command through the runner. Production wires the
// real ai.Transport, which itself goes through the CommandRunner seam, so "every
// external call via the seam" still holds end to end.
//
// Generate is called concurrently, once per chunk call in flight, so an
// implementation must be safe for concurrent use.
type Transport interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Progress is told how a generation advances. Its calls never overlap, but they
// may arrive from the chunk calls' goroutines.
type Progress interface {
	// PartsDone reports done of parts finished, done 0 as the run splits.
	// promptBytes is the size of the single-pass prompt the run split from.
	PartsDone(promptBytes, parts, done int)
	// Halved reports a call classified SIZE re-cut into parts smaller calls, in place
	// of the split when the call is the single-pass call. part is the re-cut part's
	// number within the run's total before the re-cut, 0 for the single-pass call.
	Halved(part, parts int)
	// Merging reports the merge call starting over parts' notes.
	Merging(parts int)
	// Merged reports the merge call having returned the body.
	Merged(parts int)
	// CallSucceeded reports a call carrying diff, the single-pass call or a chunk or
	// piece call, having returned its notes. promptBytes is the call's whole prompt.
	CallSucceeded(promptBytes int)
}

type noProgress struct{}

func (noProgress) PartsDone(int, int, int) {}
func (noProgress) Halved(int, int)         {}
func (noProgress) Merging(int)             {}
func (noProgress) Merged(int)              {}
func (noProgress) CallSucceeded(int)       {}

// Generator is the NORMAL AI release-notes path: it orchestrates the assembly
// pieces (diff, Change Map, prompt compose, size check) and hands the finished
// prompt — or, over the cap, the chunk and merge prompts — to the transport. It
// holds the git-aware Assembler, the AI Transport, and the repo root (for
// ResolveInstructions, which reads context/prompt files); max_prompt_bytes and the
// [release] knobs arrive per-call via config.Config.
//
// This is the NORMAL PATH ONLY: it assumes a NON-DEGENERATE (non-empty) diff. The
// notes-path precedence that selects this path over the first-release, --no-ai,
// and degenerate/empty-diff stubs sits IN FRONT of it (later tasks) and is not
// this type's concern. It also does NOT decide abort-vs-fallback on a failure —
// it surfaces the typed cause and leaves on_notes_failure routing to its caller.
type Generator struct {
	assembler *Assembler
	transport Transport
	root      string
	progress  Progress
	// saveChunkNotes keeps a failed chunked run's finished chunk notes; nil keeps none.
	saveChunkNotes SaveChunkNotes
	// keepMergeInputs is handed a written chunked run's merge inputs; nil is handed none.
	keepMergeInputs func(MergeInputs)
}

// NewGenerator builds a Generator over the assembler (the git seam), the AI
// transport, and the repo root. Dependencies are injected so production wires the
// real Assembler + ai.Transport while tests pass a FakeRunner-backed Assembler
// and a recording fake transport.
func NewGenerator(assembler *Assembler, transport Transport, root string) *Generator {
	return &Generator{assembler: assembler, transport: transport, root: root, progress: noProgress{}}
}

// WithProgress returns a copy of g that reports its generations to progress; g
// itself is unchanged. A nil progress reports nowhere.
func (g *Generator) WithProgress(progress Progress) *Generator {
	c := *g
	c.progress = progress
	if progress == nil {
		c.progress = noProgress{}
	}
	return &c
}

// Generate produces the AI release-notes body for lastTag..HEAD, orchestrating
// the engine pieces in EXACTLY this order:
//
//  1. AssembleDiff — the post-exclusion diff.
//  2. BuildChangeMap — the coverage preamble.
//  3. ResolveInstructions + ComposePrompt — the single-pass AI input (instructions +
//     Change Map + diff + closing reminder, nothing else; no commit messages).
//  4. The size check — a single-pass prompt within max_prompt_bytes goes to
//     transport.Generate as one call; one over it skips that call and is written by
//     chunk calls and one merge call instead (generateChunked). A single-pass or
//     chunk call whose failure is classified SIZE has its slice re-sent as chunk
//     calls each at most half the failed call's size, up to three times per slice; a
//     merge call's is not.
//
// The body is returned WHOLE: no parsing, splitting, label extraction, or
// per-sink reassembly — a valid generation, single-pass or merged, passes through
// byte-identical. Typed transport failures (ai.ErrTimeout / ai.ErrCommandMissing /
// ai.ErrGenerationFailed) are surfaced with the cause PRESERVED (wrapped with %w so
// errors.Is still matches). The decision of abort-vs-fallback is on_notes_failure
// routing, NOT this method's concern.
func (g *Generator) Generate(ctx context.Context, lastTag string, cfg config.Config) (string, error) {
	return g.GenerateWithContext(ctx, lastTag, cfg, "")
}

// GenerateWithContext is Generate with a ONE-TIME context line appended to the
// resolved instructions for THIS generation only: a transient nudge ("lead with the
// auth package") appended to the prompt's instructions before ComposePrompt, flowing
// into every AI call this generation makes.
//
// The one-time context is TRANSIENT — it is appended to the prompt only and is
// NEVER written back to cfg/[release].context. cfg is passed by value and is not
// mutated; the context does not persist beyond this call.
//
// An EMPTY oneTimeContext appends nothing and produces a BYTE-IDENTICAL prompt to
// Generate, so Generate is exactly GenerateWithContext(..., "") and the existing
// no-context behaviour is unchanged.
func (g *Generator) GenerateWithContext(ctx context.Context, lastTag string, cfg config.Config, oneTimeContext string) (string, error) {
	diff, err := g.assembler.AssembleDiff(ctx, lastTag)
	if err != nil {
		return "", fmt.Errorf("assembling diff for notes: %w", err)
	}
	return g.generateFromDiffWithContext(ctx, forwardRange(lastTag), diff, cfg, oneTimeContext)
}

// GenerateFromRange is the REGENERATE FRESH SOURCE path (task 5-6): it produces the AI
// release-notes body for an ARBITRARY git range — 5-3's `{PreviousTag}..{Tag}` —
// instead of the forward `{lastTag}..HEAD`. It REUSES the forward orchestration end to
// end: AssembleRange (the post-exclusion range diff with the SAME exclusion tiers) ->
// BuildChangeMapForRange (the Change Map computed AFTER exclusion, prepended to the AI
// input) -> ComposePrompt -> the size check -> transport.Generate as one call, or
// chunk calls and a merge over the cap (the SAME AI validation/retry either way).
//
// The failure is SURFACED (wrapped, errors.Is still matches) exactly as the forward
// generator, never swallowed: routing it is the caller's. The body is returned WHOLE
// (no parsing/splitting) for the downstream provider/changelog write.
func (g *Generator) GenerateFromRange(ctx context.Context, diffRange string, cfg config.Config) (string, error) {
	return g.GenerateFromRangeWithContext(ctx, diffRange, cfg, "")
}

// GenerateFromRangeWithContext is GenerateFromRange with a ONE-TIME context line
// appended to the resolved instructions for THIS generation only — the range
// counterpart of GenerateWithContext. It is the AI support for the regenerate FRESH
// path's review-gate `r` (regenerate) choice on notes not written by a chunked run
// (chunked notes re-merge through Remerge instead): the user supplies a transient nudge,
// which is appended to the prompt before ComposePrompt and flows into every AI call
// this generation makes over the resolved `{PreviousTag}..{Tag}` range.
//
// The one-time context is TRANSIENT — appended to the prompt only, NEVER written back
// to cfg/[release].context (cfg is passed by value and not mutated). An EMPTY
// oneTimeContext appends nothing, so GenerateFromRange is exactly
// GenerateFromRangeWithContext(..., "") and the existing no-context behaviour is
// byte-identical.
func (g *Generator) GenerateFromRangeWithContext(ctx context.Context, diffRange string, cfg config.Config, oneTimeContext string) (string, error) {
	diff, err := g.assembler.AssembleRange(ctx, diffRange)
	if err != nil {
		return "", fmt.Errorf("assembling range diff for notes: %w", err)
	}
	return g.generateFromDiffWithContext(ctx, diffRange, diff, cfg, oneTimeContext)
}

// GenerateFromDiff is Generate's body over an ALREADY-ASSEMBLED post-exclusion
// diff: it runs the change map, prompt compose, size check, and transport on the
// supplied diff instead of assembling it itself. It exists so the notes-path
// precedence (SelectBody), which already assembles the diff for the degenerate
// check, can pass that same diff straight into the AI path — assembling ONCE
// rather than twice. Generate is the thin wrapper that assembles then calls this,
// so its behaviour is byte-identical to before; callers needing the assemble +
// generate in one step keep using Generate.
//
// Steps 2-4 of Generate's documented order run here unchanged. The diff IS NOT
// re-assembled, so the caller is responsible for passing the same
// post-exclusion diff AssembleDiff would have produced for lastTag.
func (g *Generator) GenerateFromDiff(ctx context.Context, lastTag, diff string, cfg config.Config) (string, error) {
	return g.generateFromDiffWithContext(ctx, forwardRange(lastTag), diff, cfg, "")
}

// generateFromDiffWithContext is the shared body of GenerateFromDiff,
// GenerateWithContext, and GenerateFromRange: it runs steps 2-4 of Generate's
// documented order over the supplied diff (BuildChangeMapForRange ->
// ResolveInstructions + appendOneTimeContext + ComposePrompt -> the size check ->
// transport.Generate or generateChunked). diffRange is the git range the Change Map is
// computed over — `{lastTag}..HEAD` on the forward path, `{PreviousTag}..{Tag}` on the
// regenerate fresh path — so the map matches the diff exactly. The one-time context is
// appended to the resolved instructions ONLY for this generation, so it rides in every
// call a chunked run makes; an empty context appends nothing.
func (g *Generator) generateFromDiffWithContext(ctx context.Context, diffRange, diff string, cfg config.Config, oneTimeContext string) (string, error) {
	// Degenerate guard, shared by EVERY range producer that reaches this core: an
	// empty/whitespace-only post-exclusion diff returns StubBody with NO AI call —
	// the same short-circuit the forward path enforces in SelectBody. An empty diff
	// is the one input the AI reliably hallucinates on, so the rule is path-agnostic.
	// The fresh `vX-1..vX` range always carries mint's release-bookkeeping commit, so
	// a version whose only non-excluded change was that bookkeeping lands here with an
	// empty diff; this guard catches it before the change map, size check, and
	// transport. GenerateFromDiff (the forward AI path) reaches this core ONLY after
	// SelectBody has already excluded degenerate diffs, so re-asserting it here is a
	// harmless, never-true check on that path — the rule lives in ONE place.
	if IsDegenerate(diff) {
		return StubBody(), nil
	}

	changeMap, err := g.assembler.BuildChangeMapForRange(ctx, diffRange, cfg.MaxPromptBytes)
	if err != nil {
		return "", fmt.Errorf("building change map for notes: %w", err)
	}

	releaseInstructions, err := ResolveInstructions(g.root, cfg.Release)
	if err != nil {
		return "", fmt.Errorf("resolving notes instructions: %w", err)
	}

	instructions := appendOneTimeContext(releaseInstructions, oneTimeContext)
	prompt := ComposePrompt(instructions, changeMap, diff)
	if len(prompt) > cfg.MaxPromptBytes {
		body, partNotes, err := g.generateChunked(ctx, instructions, changeMap, diff, len(prompt), cfg.MaxPromptBytes)
		return g.chunkedBody(releaseInstructions, changeMap, body, partNotes, err)
	}

	body, code, err := g.generate(ctx, prompt, cfg.MaxPromptBytes)
	if err == nil {
		g.progress.CallSucceeded(len(prompt))
		return body, nil
	}
	if code == codeSize {
		body, partNotes, err := g.generateHalved(ctx, instructions, changeMap, diff, err, len(prompt), cfg.MaxPromptBytes)
		return g.chunkedBody(releaseInstructions, changeMap, body, partNotes, err)
	}
	return "", fmt.Errorf("generating notes: %w", err)
}

// appendOneTimeContext appends the transient regenerate nudge to the resolved
// instructions, separated by a blank line so the AI reads it as a distinct block.
// An EMPTY context returns the instructions UNCHANGED (byte-identical), so the
// no-context path adds no separator and Generate stays identical to before.
func appendOneTimeContext(instructions, oneTimeContext string) string {
	if oneTimeContext == "" {
		return instructions
	}
	return instructions + "\n\n" + oneTimeContext
}
