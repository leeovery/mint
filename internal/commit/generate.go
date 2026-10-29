package commit

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// errDiffFullyExcluded is the typed SKIP returned when the would-be-committed diff is
// non-empty BEFORE diff_exclude but empty AFTER it: every would-be-committed change
// matches a diff_exclude glob, so the AI has nothing to summarise. It is DISTINCT from a
// genuinely empty tree (caught earlier by the preflight, which is diff_exclude-blind) —
// reaching here means the tree IS dirty, so the commit must still happen. Like the
// oversized skip (notes.ErrDiffTooLarge), it is NOT an AI-transport failure: the
// transport is never called. Run routes it to the SAME $EDITOR fallback as
// --no-ai/oversized, so the excluded files get committed with a human-written message
// rather than the model being handed a blank diff. It is a SKIP signal, never surfaced
// to the user verbatim, so its text is internal.
var errDiffFullyExcluded = errors.New("commit: every change is excluded from the AI context")

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
// committed diff — staged-only, -a, or -A per the StagingMode — with the shared and
// [commit] diff_exclude lists mapped to :(exclude) pathspecs) to the prompt composer, the max_prompt_bytes guard on
// the composed prompt, and the consumed Layer-2 transport.
//
// It holds the CommandRunner (commit's L1 git seam — production passes the
// os/exec-backed runner, tests a FakeRunner), the L2 Transport (the consumed
// ai.Transport in production, a recording fake in tests), the repo root (for
// ResolveInstructions, which reads the optional [commit].prompt override file, AND as
// the working directory every L1 git read runs from — see sourceDiff), and the
// resolved StagingMode (which selects the L1 SOURCE — only the source command differs
// per mode; the :(exclude) exclusion, the compose, the size guard, and the L2 transport
// are shared across all three).
//
// Commit owns its OWN L1 rather than reusing internal/notes because notes is
// release-specific: range-based (AssembleRange only), release-tier excludes are
// hardwired (the built-in always-excludes + version_file), and its exclude-pathspec assembly is
// unexported. Commit therefore builds the would-be-staged source and its own pathspecs
// (the shared plus [commit] diff_exclude lists, never [release]'s) here, while CONSUMING the genuinely shared pieces: notes.ErrDiffTooLarge
// (the oversized sentinel Run routes on) and the ai.Transport (validate + one retry)
// behind the local Transport seam.
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
//     StagingMode, with the shared diff_exclude list plus cfg.Commit.DiffExclude
//     mapped to :(exclude) pathspecs (no built-in
//     / version_file tiers — those are release-specific). The SOURCE command differs per
//     mode (StagedOnly: `git diff --cached`; All: the read-only tracked-vs-HEAD diff;
//     AddAll: that diff plus untracked files rendered read-only as additions), but git
//     performs every exclusion, so excluded files are absent from the post-exclusion
//     diff and never reach the prompt. The -a/-A source is computed READ-ONLY — no
//     `git add`, the index byte-for-byte unchanged.
//  2. ResolveInstructions + ComposePrompt — the finished AI input (commit's default
//     Conventional Commits prompt / [commit] knobs, then the would-be-committed diff,
//     then the closing reminder).
//  3. checkPromptSize — the max_prompt_bytes guard on the WHOLE composed prompt,
//     applied BEFORE any L2 call. An over-cap prompt returns the consumed
//     notes.ErrDiffTooLarge (wrapped) and the transport is NEVER called.
//  4. transport.Generate — the consumed L2: validation + one retry, returning the
//     validated body or a typed transport failure.
//
// The body is returned WHOLE: no parsing, splitting, or trimming — a valid generation
// passes through byte-identical, suitable for the commit sink. Typed failures are
// surfaced with the cause PRESERVED (wrapped with %w so errors.Is still matches) and
// remain distinguishable: notes.ErrDiffTooLarge (the guard) vs ai.ErrGenerationFailed
// / ai.ErrTimeout / ai.ErrCommandMissing (the transport), so Run can route oversized
// vs generation-failure. This method only surfaces the outcome.
func (g *Generator) Generate(ctx context.Context, cfg config.Config) (string, error) {
	return g.GenerateWithContext(ctx, cfg, "")
}

// GenerateWithContext is Generate with the gate's `r` (regenerate-with-context)
// one-time context augmentation: it runs the IDENTICAL L1 → resolve → compose →
// size-guard → L2 path, but layers oneTimeContext onto the resolved instructions (ON
// TOP of any persisted [commit].context) before composing, so the size guard measures
// the prompt with the one-time context included. An EMPTY oneTimeContext is a
// plain re-roll — no block is injected, so the composed prompt is byte-identical to a
// normal Generate. The one-time context is a local string only; it is NEVER persisted
// to cfg/[commit].context. The diff source and the :(exclude) exclusion are unchanged;
// the transport's one retry is consumed here exactly as Generate consumes it. Generate
// delegates here with an empty context, so the two share one path.
func (g *Generator) GenerateWithContext(ctx context.Context, cfg config.Config, oneTimeContext string) (string, error) {
	diff, err := g.sourceDiff(ctx, slices.Concat(cfg.DiffExclude, cfg.Commit.DiffExclude))
	if err != nil {
		return "", fmt.Errorf("assembling would-be-committed diff for commit: %w", err)
	}

	// The preflight (blind to every diff_exclude level) already proved the tree is dirty
	// for this mode, so an empty POST-exclusion diff means diff_exclude removed EVERY would-be-committed
	// change: the model has nothing to read. Don't compose a blank prompt or call the
	// transport — return the typed skip so Run routes to the SAME $EDITOR fallback as
	// --no-ai/oversized. The excluded changes still get committed (the editor save IS the
	// accept), just with a human-written message. TrimSpace guards a whitespace-only diff
	// the same way the editor-emptiness rule does.
	if strings.TrimSpace(diff) == "" {
		return "", errDiffFullyExcluded
	}

	instructions, err := ResolveInstructions(cfg, g.root)
	if err != nil {
		return "", fmt.Errorf("resolving commit instructions: %w", err)
	}
	instructions = injectOneTimeContext(instructions, oneTimeContext)

	prompt := ComposePrompt(instructions, diff)

	if err := checkPromptSize(prompt, cfg.MaxPromptBytes); err != nil {
		return "", fmt.Errorf("commit size guard: %w", err)
	}

	body, err := g.transport.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}
	return body, nil
}

// checkPromptSize is commit's max_prompt_bytes guard on the whole composed prompt. It
// returns the consumed notes.ErrDiffTooLarge so Run routes an over-cap prompt to the
// oversized editor fallback; the cap is inclusive.
func checkPromptSize(prompt string, maxBytes int) error {
	if len(prompt) > maxBytes {
		return fmt.Errorf("%w: composed prompt exceeds max_prompt_bytes (%d > %d)", notes.ErrDiffTooLarge, len(prompt), maxBytes)
	}
	return nil
}

// sourceDiff is commit's Layer-1 SOURCE selector: it returns the would-be-committed
// diff for the resolved StagingMode by rendering and concatenating each source the SHARED
// sourcesForMode descriptor lists for the mode — the SAME descriptor the preflight
// counting path (readDiffStat) consumes, so the dispatch (and the AddAll
// tracked-then-untracked composition) is defined exactly once. Each source applies the
// SAME diffExclude :(exclude) pathspecs (the shared plus [commit] lists); the post-exclusion result flows into the
// shared compose + size-guard + L2 pipeline unchanged.
//
//   - StagedOnly (the Phase 1 default): the staged diff — `git diff --cached`.
//   - All (-a): the read-only tracked-vs-HEAD diff (tracked mods + deletions, no
//     untracked) — computed WITHOUT mutating the index.
//   - AddAll (-A): the tracked-vs-HEAD diff plus each untracked file rendered as an
//     added-file diff — also read-only (no `git add`).
//
// EVERY L1 git read runs with the repo ROOT as its working directory (RunInDir(g.root)),
// because the shared `-- .` selector and the ls-files enumeration are cwd-relative:
// from a subdirectory a plain Run would scope the PREVIEW to the subtree while the
// accept-time mutations (`git add -u`/`-A` are whole-tree since git 2.0; `git commit`
// commits the whole index) stay repo-wide — a reviewed message describing only part of
// what gets committed. Pinning cwd to the root makes `.` always mean the whole tree, so
// the preview, the emptiness preflight (which anchors the same way), and the mutations
// agree no matter where mint was invoked.
func (g *Generator) sourceDiff(ctx context.Context, diffExclude []string) (string, error) {
	var b strings.Builder
	for _, spec := range sourcesForMode(g.mode) {
		rendered, err := g.renderSource(ctx, spec, diffExclude)
		if err != nil {
			return "", err
		}
		b.WriteString(rendered)
	}
	return b.String(), nil
}

// renderSource renders ONE per-mode source spec (from the shared sourcesForMode
// descriptor) into its L1 diff text, applying the SAME excludePathspecs tail the preflight
// probe applies (via sourceArgs):
//
//   - diffSource: git's stdout IS the diff — `git diff … -- . :(exclude)…`.
//   - untrackedListSource: enumerate untracked paths (`git ls-files --others … -- .
//     :(exclude)…`) then render EACH as a read-only addition diff (`git diff --no-index
//     -- /dev/null <file>`), concatenated in enumeration order.
//
// The diff source and untracked-list rendering are unchanged from the prior
// trackedWorktreeDiff / addAllWorktreeDiff behaviour; only the source SELECTION is now
// driven by the shared descriptor instead of a per-mode switch.
func (g *Generator) renderSource(ctx context.Context, spec sourceSpec, diffExclude []string) (string, error) {
	switch spec.kind {
	case untrackedListSource:
		return g.untrackedAdditions(ctx, spec.base, diffExclude)
	default:
		return g.diffSourceText(ctx, spec.base, diffExclude)
	}
}

// diffSourceText runs a diffSource — a `git diff …` whose stdout IS the post-exclusion
// diff — built from the shared base prefix (stagedBaseArgs / trackedBaseArgs) plus the
// diffExclude :(exclude) pathspecs via the single shared sourceArgs composer. The
// SAME base feeds the matching preflight probe (which adds `--numstat` and NO exclusion
// tail — the preflight is diff_exclude-blind; see preflight.go), so the L1 diff and the
// counting probe share the source COMMAND and differ only in the tail: the L1 diff
// applies diff_exclude, the probe does not.
//
// Selecting the base prefix is the per-mode source SELECTION (sourcesForMode); the verb,
// refspec, and `-- .` selector are spelled exactly once (in the *BaseArgs builders).
// git interprets each glob as a pathspec pattern (mint does no Go-side glob matching), so
// a glob matching nothing is harmless. The returned text is git's stdout verbatim; an
// empty post-exclusion diff is NOT an error, and a non-zero git exit is surfaced as a
// wrapped error so an unexpected failure is never mistaken for an empty diff. Diffing is
// READ-ONLY — a plain `git diff` reads the object store and the worktree and mutates
// nothing (no `git add`, the index unchanged).
func (g *Generator) diffSourceText(ctx context.Context, base, diffExclude []string) (string, error) {
	args := sourceArgs(base, diffExclude)
	res, err := g.runner.RunInDir(ctx, g.root, nil, "git", args...)
	if err != nil {
		return "", fmt.Errorf("running git %v: %w", args, err)
	}
	return res.Stdout, nil
}

// untrackedAdditions renders an untrackedListSource — the AddAll (-A) untracked source —
// into its L1 diff text: enumerate the repo's untracked, non-ignored files from the shared
// untracked base prefix plus the diffExclude :(exclude) pathspecs (so excluded
// untracked files are never enumerated and never reach the prompt), then render EACH as a
// read-only added-file diff in enumeration order, matching `git add -A` semantics
// (everything, including untracked) — all computed READ-ONLY, leaving the index
// byte-for-byte unchanged.
//
// The technique avoids `git add` (and `git add --intent-to-add`, which MUTATES the index)
// entirely: the enumeration is the shared ls-files prefix the preflight probe reuses
// VERBATIM, and untrackedAdditionDiff renders each path as `git diff --no-index --
// /dev/null <file>`, a pure read-only comparison of the file against an empty input.
// The prefix carries `-z`, so git emits one RAW NUL-terminated path per entry — without
// it, core.quotePath C-quotes unusual names (non-ASCII, quotes, backslashes) and the
// quoted literal would reach --no-index as a nonexistent file. Splitting on NUL keeps
// every legal git path intact (NUL is the one byte a path cannot contain); empty
// entries (an empty enumeration, the trailing terminator) yield no additions. A
// non-zero git exit on the enumeration is surfaced as a wrapped error.
func (g *Generator) untrackedAdditions(ctx context.Context, base, diffExclude []string) (string, error) {
	args := sourceArgs(base, diffExclude)
	res, err := g.runner.RunInDir(ctx, g.root, nil, "git", args...)
	if err != nil {
		return "", fmt.Errorf("running git %v: %w", args, err)
	}

	var b strings.Builder
	for _, file := range strings.Split(res.Stdout, "\x00") {
		if file == "" {
			continue
		}
		addition, err := g.untrackedAdditionDiff(ctx, file)
		if err != nil {
			return "", err
		}
		b.WriteString(addition)
	}
	return b.String(), nil
}

// untrackedAdditionDiff renders a single untracked file as an added-file diff via
// `git diff --no-index -- /dev/null <file>` — a pure read-only comparison against an
// empty input that NEVER touches the index. It runs from the repo root (like every L1
// read), so the root-relative paths the root-anchored ls-files enumeration emits
// resolve correctly regardless of mint's invocation directory.
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
	res, err := g.runner.RunInDir(ctx, g.root, nil, "git", "diff", "--no-index", "--", "/dev/null", file)
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
