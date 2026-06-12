package commit

import (
	"context"
	"fmt"
	"strings"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// diffNoIndexFilesDiffer is the exit code `git diff --no-index` returns when its two
// inputs differ — the NORMAL outcome when rendering an untracked file against
// /dev/null. It is treated as success (not a failure) so the addition diff on stdout is
// used; any other non-zero exit is a genuine error.
const diffNoIndexFilesDiffer = 1

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

// Generator is commit's Layer-3 glue: it wires commit's OWN Layer-1 (the would-be-
// committed diff — staged-only, -a, or -A per the StagingMode — with cfg.DiffExclude
// mapped to :(exclude) pathspecs and the consumed max_diff_lines guard) to the prompt
// composer (1-2) and the consumed Layer-2 transport.
//
// It holds the CommandRunner (commit's L1 git seam — production passes the
// os/exec-backed runner, tests a FakeRunner), the L2 Transport (the consumed
// ai.Transport in production, a recording fake in tests), the repo root (for
// ResolveInstructions, which reads the optional [commit].prompt override file), and the
// resolved StagingMode (which selects the L1 SOURCE — only the source command differs
// per mode; the :(exclude) exclusion, the size guard, the compose, and the L2 transport
// are shared across all three).
//
// Commit owns its OWN L1 rather than reusing internal/notes because notes is
// release-specific: range-based (AssembleRange only), release-tier excludes are
// hardwired (CHANGELOG.md + version_file), and its exclude-pathspec assembly is
// unexported. Commit therefore builds the would-be-staged source and cfg.DiffExclude-only
// pathspecs here, while CONSUMING the genuinely shared pieces: notes.CheckDiffSize /
// notes.ErrDiffTooLarge (the pure size guard) and the ai.Transport (validate + one
// retry) behind the local Transport seam.
type Generator struct {
	runner    runner.CommandRunner
	transport Transport
	root      string
	mode      StagingMode
}

// NewGenerator builds a Generator over the L1 git runner, the L2 transport, the repo
// root, and the resolved StagingMode. Dependencies are injected so production wires the
// real os/exec runner + ai.Transport while tests pass a FakeRunner and a recording fake
// transport. The mode selects the L1 source: StagedOnly (the Phase 1 default) reads the
// index; All/AddAll compute the would-be-staged worktree diff READ-ONLY.
func NewGenerator(r runner.CommandRunner, transport Transport, root string, mode StagingMode) *Generator {
	return &Generator{runner: r, transport: transport, root: root, mode: mode}
}

// Generate produces the AI commit-message body for the would-be-committed diff,
// orchestrating the L3 pieces in EXACTLY this order:
//
//  1. sourceDiff — commit's L1: the would-be-committed diff for the resolved
//     StagingMode, with cfg.DiffExclude mapped to :(exclude) pathspecs (no CHANGELOG.md
//     / version_file tiers — those are release-specific). The SOURCE command differs per
//     mode (StagedOnly: `git diff --cached`; All: the read-only tracked-vs-HEAD diff;
//     AddAll: that diff plus untracked files rendered read-only as additions), but git
//     performs every exclusion, so excluded files are absent from the post-exclusion
//     diff and never reach the prompt. The -a/-A source is computed READ-ONLY — no
//     `git add`, the index byte-for-byte unchanged.
//  2. notes.CheckDiffSize — the consumed max_diff_lines guard, applied AFTER
//     diff_exclude and BEFORE any L2 call. An over-ceiling diff returns the consumed
//     notes.ErrDiffTooLarge (wrapped) and the transport is NEVER called.
//  3. ResolveInstructions + ComposePrompt — the finished AI input (commit's default
//     Conventional Commits prompt / [commit] knobs, then the would-be-committed diff).
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
func (g *Generator) Generate(ctx context.Context, cfg config.Config) (string, error) {
	diff, err := g.sourceDiff(ctx, cfg.DiffExclude)
	if err != nil {
		return "", fmt.Errorf("assembling would-be-committed diff for commit: %w", err)
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

// sourceDiff is commit's Layer-1 SOURCE selector: it returns the would-be-committed
// diff for the resolved StagingMode, applying the SAME cfg.DiffExclude :(exclude)
// pathspecs to whichever source the mode chooses. Only the source command differs per
// mode; the post-exclusion result flows into the shared size-guard + compose + L2
// pipeline unchanged.
//
//   - StagedOnly (the Phase 1 default): the staged diff — `git diff --cached`.
//   - All (-a): the read-only tracked-vs-HEAD diff (tracked mods + deletions, no
//     untracked) — computed WITHOUT mutating the index.
//   - AddAll (-A): the tracked-vs-HEAD diff plus each untracked file rendered as an
//     added-file diff — also read-only (no `git add`).
func (g *Generator) sourceDiff(ctx context.Context, diffExclude []string) (string, error) {
	switch g.mode {
	case All:
		return g.trackedWorktreeDiff(ctx, diffExclude)
	case AddAll:
		return g.addAllWorktreeDiff(ctx, diffExclude)
	default:
		return g.stagedDiff(ctx, diffExclude)
	}
}

// stagedDiff is the StagedOnly source: it runs `git diff --cached -- .` plus one
// :(exclude)<glob> pathspec per configured diff_exclude glob, returning git's raw
// post-exclusion stdout. The --cached selects the STAGED diff (the index as it
// stands) — the Phase 1 default, byte-identical to before the modes were added.
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

// trackedWorktreeDiff is the All (-a) source: it runs `git diff HEAD -- .` plus the
// cfg.DiffExclude :(exclude) pathspecs, returning git's raw post-exclusion stdout.
// Diffing the WORKING TREE against HEAD captures every tracked change — modifications
// AND deletions, whether staged or unstaged — while excluding untracked files, exactly
// matching `git commit -a` semantics. It is READ-ONLY: a plain `git diff` reads the
// object store and the worktree and mutates NOTHING (no `git add`, the index unchanged).
// An empty post-exclusion diff is not an error; a non-zero git exit is surfaced as a
// wrapped error so it is never mistaken for an empty diff.
func (g *Generator) trackedWorktreeDiff(ctx context.Context, diffExclude []string) (string, error) {
	args := append([]string{"diff", "HEAD", "--", "."}, excludePathspecs(diffExclude)...)
	res, err := g.runner.Run(ctx, "git", args...)
	if err != nil {
		return "", fmt.Errorf("running git diff HEAD: %w", err)
	}
	return res.Stdout, nil
}

// addAllWorktreeDiff is the AddAll (-A) source: the tracked-vs-HEAD diff (mods +
// deletions) CONCATENATED with each untracked file rendered as an added-file diff,
// matching `git add -A` semantics (everything, including untracked) — all computed
// READ-ONLY, leaving the index byte-for-byte unchanged.
//
// The technique avoids `git add` (and `git add --intent-to-add`, which MUTATES the
// index) entirely:
//  1. trackedWorktreeDiff — the same read-only tracked diff -a uses.
//  2. untrackedFiles — enumerate untracked paths via `git ls-files --others
//     --exclude-standard`, scoped by the SAME :(exclude) pathspecs so an excluded
//     untracked file is never enumerated.
//  3. untrackedAdditionDiff — render each as `git diff --no-index -- /dev/null <file>`,
//     a pure read-only comparison of the file against an empty input.
//
// The tracked diff comes first, then the untracked additions in enumeration order, so
// the combined text is a single coherent would-be-staged diff fed to the shared
// size-guard + compose pipeline.
func (g *Generator) addAllWorktreeDiff(ctx context.Context, diffExclude []string) (string, error) {
	tracked, err := g.trackedWorktreeDiff(ctx, diffExclude)
	if err != nil {
		return "", err
	}

	files, err := g.untrackedFiles(ctx, diffExclude)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(tracked)
	for _, file := range files {
		addition, err := g.untrackedAdditionDiff(ctx, file)
		if err != nil {
			return "", err
		}
		b.WriteString(addition)
	}
	return b.String(), nil
}

// untrackedFiles lists the repo's untracked, non-ignored files via `git ls-files
// --others --exclude-standard -- .` plus the cfg.DiffExclude :(exclude) pathspecs, so
// excluded untracked files (bundles, lockfiles) are never enumerated and so never reach
// the prompt. It is read-only. git prints one NUL-free path per line; blank lines (an
// empty enumeration) yield no files. A non-zero git exit is surfaced as a wrapped error.
func (g *Generator) untrackedFiles(ctx context.Context, diffExclude []string) ([]string, error) {
	args := append([]string{"ls-files", "--others", "--exclude-standard", "--", "."}, excludePathspecs(diffExclude)...)
	res, err := g.runner.Run(ctx, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("running git ls-files --others: %w", err)
	}

	var files []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// untrackedAdditionDiff renders a single untracked file as an added-file diff via
// `git diff --no-index -- /dev/null <file>` — a pure read-only comparison against an
// empty input that NEVER touches the index.
//
// `git diff --no-index` exits 1 (diffNoIndexFilesDiffer) in TWO cases: (a) the inputs
// DIFFER — the NORMAL outcome here, with the addition diff on STDOUT; and (b) a GENUINE
// error like "could not access '<file>'", which ALSO exits 1 but with EMPTY stdout and a
// message on STDERR. The exit code alone cannot tell them apart, so the discriminator is
// STDOUT-NON-EMPTY: a real addition diff always has content. An exit-1 with non-empty
// stdout is the success/differ case and the diff is returned; an exit-1 with empty stdout
// (and populated stderr) is surfaced as a wrapped error so the untracked file is never
// silently dropped from the would-be-staged diff. Any OTHER non-zero exit is likewise a
// genuine failure and is surfaced wrapped.
func (g *Generator) untrackedAdditionDiff(ctx context.Context, file string) (string, error) {
	res, err := g.runner.Run(ctx, "git", "diff", "--no-index", "--", "/dev/null", file)
	if err != nil {
		// The differ family (exit 1) is success ONLY when stdout carries a real diff. An
		// exit-1 with empty stdout is a genuine --no-index error (e.g. could not access
		// the file); any other non-zero exit is a genuine failure too.
		if res.ExitCode == diffNoIndexFilesDiffer && res.Stdout != "" {
			return res.Stdout, nil
		}
		return "", fmt.Errorf("running git diff --no-index for %q: %s: %w", file, strings.TrimSpace(res.Stderr), err)
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
