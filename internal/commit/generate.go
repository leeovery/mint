package commit

import (
	"context"
	"fmt"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// Transport is the content-agnostic AI seam commit's L3 glue depends on: a finished
// prompt in, a validated body (or a typed failure) out. It is defined HERE, where it
// is consumed, so commit stays decoupled from the ai package's concretions —
// ai.Transport satisfies it in production, while tests inject a recording fake that
// captures the prompt and returns a scripted body without scripting the real
// `claude` command through the runner. This mirrors the notes engine's consumer-side
// Transport seam (the consumed pattern); production wires the real *ai.Transport,
// which itself goes through the CommandRunner seam, so "every external call via the
// seam" still holds end to end. The transport owns validation and the single retry —
// commit consumes that behaviour, it does not reimplement it.
type Transport interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Generator is commit's Layer-3 glue for the bare-commit path: it wires commit's OWN
// Layer-1 (the staged diff via `git diff --cached`, with cfg.DiffExclude mapped to
// :(exclude) pathspecs and the consumed max_diff_lines guard) to the prompt composer
// (1-2) and the consumed Layer-2 transport.
//
// It holds the CommandRunner (commit's L1 git seam — production passes the
// os/exec-backed runner, tests a FakeRunner), the L2 Transport (the consumed
// ai.Transport in production, a recording fake in tests), and the repo root (for
// ResolveInstructions, which reads the optional [commit].prompt override file).
//
// Commit owns its OWN L1 rather than reusing internal/notes because notes is
// release-specific: range-based (AssembleRange only), release-tier excludes are
// hardwired (CHANGELOG.md + version_file), and its exclude-pathspec assembly is
// unexported. Commit therefore builds the staged-diff source and cfg.DiffExclude-only
// pathspecs here, while CONSUMING the genuinely shared pieces: notes.CheckDiffSize /
// notes.ErrDiffTooLarge (the pure size guard) and the ai.Transport (validate + one
// retry) behind the local Transport seam.
type Generator struct {
	runner    runner.CommandRunner
	transport Transport
	root      string
}

// NewGenerator builds a Generator over the L1 git runner, the L2 transport, and the
// repo root. Dependencies are injected so production wires the real os/exec runner +
// ai.Transport while tests pass a FakeRunner and a recording fake transport.
func NewGenerator(r runner.CommandRunner, transport Transport, root string) *Generator {
	return &Generator{runner: r, transport: transport, root: root}
}

// Generate produces the AI commit-message body for the STAGED diff, orchestrating the
// L3 pieces in EXACTLY this order:
//
//  1. stagedDiff — commit's L1: `git diff --cached -- . {:(exclude)<glob>…}`, with
//     cfg.DiffExclude mapped to :(exclude) pathspecs (no CHANGELOG.md / version_file
//     tiers — those are release-specific). git performs every exclusion, so excluded
//     files are absent from the returned post-exclusion diff and never reach the prompt.
//  2. notes.CheckDiffSize — the consumed max_diff_lines guard, applied AFTER
//     diff_exclude and BEFORE any L2 call. An over-ceiling diff returns the consumed
//     notes.ErrDiffTooLarge (wrapped) and the transport is NEVER called.
//  3. ResolveInstructions + ComposePrompt — the finished AI input (commit's default
//     Conventional Commits prompt / [commit] knobs, then the staged diff).
//  4. transport.Generate — the consumed L2: validation + one retry, returning the
//     validated body or a typed transport failure.
//
// The body is returned WHOLE: no parsing, splitting, or trimming — a valid generation
// passes through byte-identical, suitable for the commit sink. Typed failures are
// surfaced with the cause PRESERVED (wrapped with %w so errors.Is still matches) and
// remain distinguishable: notes.ErrDiffTooLarge (the guard) vs ai.ErrGenerationFailed
// / ai.ErrTimeout / ai.ErrCommandMissing (the transport), so Phase 3 can route
// oversized vs generation-failure. Routing failures to $EDITOR / --no-ai is Phase 3 —
// this method only surfaces the outcome.
//
// Phase 1 source is STAGED-ONLY; the -a/-A would-be-staged source is Phase 2.
func (g *Generator) Generate(ctx context.Context, cfg config.Config) (string, error) {
	diff, err := g.stagedDiff(ctx, cfg.DiffExclude)
	if err != nil {
		return "", fmt.Errorf("assembling staged diff for commit: %w", err)
	}

	if err := notes.CheckDiffSize(diff, cfg.MaxDiffLines); err != nil {
		return "", fmt.Errorf("commit size guard: %w", err)
	}

	instructions, err := ResolveInstructions(cfg, g.root)
	if err != nil {
		return "", fmt.Errorf("resolving commit instructions: %w", err)
	}

	prompt := ComposePrompt(instructions, diff)

	body, err := g.transport.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}
	return body, nil
}

// stagedDiff is commit's Layer-1 source: it runs `git diff --cached -- .` plus one
// :(exclude)<glob> pathspec per configured diff_exclude glob, returning git's raw
// post-exclusion stdout. The --cached selects the STAGED diff (the index as it
// stands), the staged-only Phase 1 source — NOT a -a/-A would-be-staged worktree diff
// (that is Phase 2).
//
// The :(exclude) mapping mirrors the notes assembler's pattern (one entry per glob,
// in config order, after `-- .`) but carries ONLY cfg.DiffExclude — commit does not
// inherit release's hardwired CHANGELOG.md / version_file tiers. git interprets each
// glob as a pathspec pattern (mint does no Go-side glob matching), so a glob matching
// nothing is harmless. The returned text is git's stdout verbatim; an empty
// post-exclusion diff is NOT an error here, and a non-zero git exit is surfaced as a
// wrapped error so an unexpected failure is never mistaken for an empty diff.
func (g *Generator) stagedDiff(ctx context.Context, diffExclude []string) (string, error) {
	args := append([]string{"diff", "--cached", "--", "."}, excludePathspecs(diffExclude)...)
	res, err := g.runner.Run(ctx, "git", args...)
	if err != nil {
		return "", fmt.Errorf("running git diff --cached: %w", err)
	}
	return res.Stdout, nil
}

// excludePathspecs maps each diff_exclude glob to its :(exclude)<glob> pathspec, in
// config order. Unlike the notes assembler's union of exclusion tiers, commit carries
// ONLY the configured globs — there is no built-in CHANGELOG.md or strategy-aware
// version_file exclusion here (both are release-specific). A nil/empty slice yields no
// pathspecs, so the bare argv is exactly `git diff --cached -- .`.
func excludePathspecs(diffExclude []string) []string {
	pathspecs := make([]string, 0, len(diffExclude))
	for _, glob := range diffExclude {
		pathspecs = append(pathspecs, ":(exclude)"+glob)
	}
	return pathspecs
}
