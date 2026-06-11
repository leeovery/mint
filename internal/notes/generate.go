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
type Transport interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Generator is the NORMAL AI release-notes path: it orchestrates the assembly
// pieces (diff, size guard, Change Map, prompt compose) and hands the finished
// prompt to the transport. It holds the git-aware Assembler, the AI Transport,
// and the repo root (for ResolveInstructions, which reads context/prompt files);
// max_diff_lines and the [release] knobs arrive per-call via config.Config.
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
}

// NewGenerator builds a Generator over the assembler (the git seam), the AI
// transport, and the repo root. Dependencies are injected so production wires the
// real Assembler + ai.Transport while tests pass a FakeRunner-backed Assembler
// and a recording fake transport.
func NewGenerator(assembler *Assembler, transport Transport, root string) *Generator {
	return &Generator{assembler: assembler, transport: transport, root: root}
}

// Generate produces the AI release-notes body for lastTag..HEAD, orchestrating
// the engine pieces in EXACTLY this order:
//
//  1. AssembleDiff — the post-exclusion diff.
//  2. CheckDiffSize — the max_diff_lines guard. An over-ceiling diff returns
//     ErrDiffTooLarge (wrapped) and the AI is NEVER called.
//  3. BuildChangeMap — the salience preamble.
//  4. ResolveInstructions + ComposePrompt — the full AI input (instructions +
//     Change Map + capped diff, nothing else; no commit messages).
//  5. transport.Generate — the validated body, or a typed transport failure.
//
// The body is returned WHOLE: no parsing, splitting, label extraction, or
// per-sink reassembly — a valid generation passes through byte-identical. Typed
// notes failures (ErrDiffTooLarge from the guard; ai.ErrTimeout /
// ai.ErrCommandMissing / ai.ErrGenerationFailed from the transport) are surfaced with
// the cause PRESERVED (wrapped with %w so errors.Is still matches). The decision
// of abort-vs-fallback is on_notes_failure routing, NOT this method's concern.
func (g *Generator) Generate(ctx context.Context, lastTag string, cfg config.Config) (string, error) {
	return g.GenerateWithContext(ctx, lastTag, cfg, "")
}

// GenerateWithContext is Generate with a ONE-TIME context line appended to the
// resolved instructions for THIS generation only. It is the AI support for the
// review gate's `r` (regenerate) choice: the user supplies a transient nudge
// ("lead with the auth package"), which is appended to the prompt's instructions
// before ComposePrompt and flows into this one AI call.
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
// CheckDiffSize (the max_diff_lines guard) -> BuildChangeMapForRange (the Change Map
// computed AFTER exclusion, prepended to the AI input) -> ComposePrompt ->
// transport.Generate (the SAME AI validation/retry).
//
// The failure is SURFACED (wrapped, errors.Is still matches) exactly as the forward
// generator — single-mode fresh routes on_notes_failure at a higher layer and 5-12's
// --all intercepts the surfaced failure for skip-and-continue, so this method must NOT
// swallow it. The body is returned WHOLE (no parsing/splitting) for the downstream
// provider/changelog write in later tasks.
func (g *Generator) GenerateFromRange(ctx context.Context, diffRange string, cfg config.Config) (string, error) {
	return g.GenerateFromRangeWithContext(ctx, diffRange, cfg, "")
}

// GenerateFromRangeWithContext is GenerateFromRange with a ONE-TIME context line
// appended to the resolved instructions for THIS generation only — the range
// counterpart of GenerateWithContext. It is the AI support for the regenerate FRESH
// path's review-gate `r` (regenerate) choice: the user supplies a transient nudge,
// which is appended to the prompt before ComposePrompt and flows into this one AI call
// over the resolved `{PreviousTag}..{Tag}` range.
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
// diff: it runs the size guard, change map, prompt compose, and transport on the
// supplied diff instead of assembling it itself. It exists so the notes-path
// precedence (SelectBody), which already assembles the diff for the degenerate
// check, can pass that same diff straight into the AI path — assembling ONCE
// rather than twice. Generate is the thin wrapper that assembles then calls this,
// so its behaviour is byte-identical to before; callers needing the assemble +
// generate in one step keep using Generate.
//
// Steps 2-5 of Generate's documented order run here unchanged: CheckDiffSize ->
// BuildChangeMap -> ResolveInstructions + ComposePrompt -> transport.Generate.
// The diff IS NOT re-assembled, so the caller is responsible for passing the same
// post-exclusion diff AssembleDiff would have produced for lastTag.
func (g *Generator) GenerateFromDiff(ctx context.Context, lastTag, diff string, cfg config.Config) (string, error) {
	return g.generateFromDiffWithContext(ctx, forwardRange(lastTag), diff, cfg, "")
}

// generateFromDiffWithContext is the shared body of GenerateFromDiff,
// GenerateWithContext, and GenerateFromRange: it runs steps 2-5 of Generate's
// documented order over the supplied diff (CheckDiffSize ->
// BuildChangeMapForRange -> ResolveInstructions + appendOneTimeContext +
// ComposePrompt -> transport.Generate). diffRange is the git range the Change Map is
// computed over — `{lastTag}..HEAD` on the forward path, `{PreviousTag}..{Tag}` on the
// regenerate fresh path — so the map matches the diff exactly. The one-time context is
// appended to the resolved instructions ONLY for this prompt; an empty context appends
// nothing, so GenerateFromDiff (which passes "") is byte-identical to before.
func (g *Generator) generateFromDiffWithContext(ctx context.Context, diffRange, diff string, cfg config.Config, oneTimeContext string) (string, error) {
	// Degenerate guard, shared by EVERY range producer that reaches this core: an
	// empty/whitespace-only post-exclusion diff returns StubBody with NO AI call —
	// the same short-circuit the forward path enforces in SelectBody. An empty diff
	// is the one input the AI reliably hallucinates on, so the rule is path-agnostic.
	// The fresh `vX-1..vX` range always carries mint's release-bookkeeping commit, so
	// a version whose only non-excluded change was that bookkeeping lands here with an
	// empty diff; this guard catches it before the size check, change map, and
	// transport. GenerateFromDiff (the forward AI path) reaches this core ONLY after
	// SelectBody has already excluded degenerate diffs, so re-asserting it here is a
	// harmless, never-true check on that path — the rule lives in ONE place.
	if IsDegenerate(diff) {
		return StubBody(), nil
	}

	if err := CheckDiffSize(diff, cfg.MaxDiffLines); err != nil {
		return "", fmt.Errorf("notes size guard: %w", err)
	}

	changeMap, err := g.assembler.BuildChangeMapForRange(ctx, diffRange)
	if err != nil {
		return "", fmt.Errorf("building change map for notes: %w", err)
	}

	instructions, err := ResolveInstructions(g.root, cfg.Release)
	if err != nil {
		return "", fmt.Errorf("resolving notes instructions: %w", err)
	}

	prompt := ComposePrompt(appendOneTimeContext(instructions, oneTimeContext), changeMap, diff)

	body, err := g.transport.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generating notes: %w", err)
	}
	return body, nil
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
