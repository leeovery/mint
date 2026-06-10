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
// ai.ErrCommandMissing / ai.ErrNotesFailure from the transport) are surfaced with
// the cause PRESERVED (wrapped with %w so errors.Is still matches). The decision
// of abort-vs-fallback is on_notes_failure routing, NOT this method's concern.
func (g *Generator) Generate(ctx context.Context, lastTag string, cfg config.Config) (string, error) {
	diff, err := g.assembler.AssembleDiff(ctx, lastTag)
	if err != nil {
		return "", fmt.Errorf("assembling diff for notes: %w", err)
	}
	return g.GenerateFromDiff(ctx, lastTag, diff, cfg)
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
	if err := CheckDiffSize(diff, cfg.MaxDiffLines); err != nil {
		return "", fmt.Errorf("notes size guard: %w", err)
	}

	changeMap, err := g.assembler.BuildChangeMap(ctx, lastTag)
	if err != nil {
		return "", fmt.Errorf("building change map for notes: %w", err)
	}

	instructions, err := ResolveInstructions(g.root, cfg.Release)
	if err != nil {
		return "", fmt.Errorf("resolving notes instructions: %w", err)
	}

	prompt := ComposePrompt(instructions, changeMap, diff)

	body, err := g.transport.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generating notes: %w", err)
	}
	return body, nil
}
