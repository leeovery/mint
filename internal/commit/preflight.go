package commit

// This file is commit's EMPTY-STAGING PREFLIGHT cluster — the staging-mode-aware
// "something to commit?" check that runs BEFORE generate so the AI is never invoked on an
// empty diff. It is colocated here (out of run.go's orchestration spine) and DERIVES its
// per-mode probes from the SAME shared source selection (source.go) the L1 diff sources
// use, so the emptiness verdict and the AI's L1 source cannot drift.
//
// The three sentinel messages, checkSomethingToCommit, wouldStageNothing, the per-mode
// probe builders, emptyStagingError, and gitOutputEmpty all live here; run.go keeps only
// the orchestration spine.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/runner"
)

// The empty-staging preflight failures. Each message is rendered VERBATIM by commit's
// surface helper (it renders cause.Error()), so the user sees the exact git-style line
// with no mint wrapping. All three are returned UNWRAPPED so that verbatim text survives
// to the presenter, and all carry lowercase, punctuation-free messages mirroring git's
// own diagnostics (per the spec's Empty-staging handling). Which one fires is keyed on
// the ACTUAL post-mode tree state, NOT the flag passed:
//
//   - errNothingToCommit — git's own clean-tree line VERBATIM: the tree is genuinely
//     clean (nothing anywhere), so the chosen mode had nothing it could ever stage.
//   - errNoChangesStaged — bare `mint commit` with unstaged changes but nothing staged:
//     guide the user to the staging modes (mint's flavour of git's "no changes added to
//     commit"). The em dash is U+2014.
//   - errNoTrackedChanges — `mint commit -a` when the only changes are untracked, so the
//     tracked-only -a staged nothing: point specifically at -A/--add-all (the mode that
//     would include them). The em dash is U+2014.
var (
	errNothingToCommit  = errors.New("nothing to commit, working tree clean")
	errNoChangesStaged  = errors.New("no changes staged — use -a/--all, -A/--add-all, or git add")
	errNoTrackedChanges = errors.New("no tracked changes to stage — use -A/--add-all to include untracked files")
)

// checkSomethingToCommit is commit's staging-mode-aware "something to commit" preflight.
// It computes the would-be-staged emptiness for the resolved StagingMode READ-ONLY (no
// `git add`, no AI) and fails loud when that set is empty, short-circuiting generation so
// the AI is never invoked on an empty diff. All probes go through the consumed
// CommandRunner seam (the same read-only idiom as generate's source helpers), so they are
// fully scriptable via the FakeRunner.
//
// diffExclude (cfg.DiffExclude) is mapped onto the probes via the SAME excludePathspecs
// helper generate's L1 source uses, so the emptiness verdict and the AI's L1 source diff
// derive from ONE exclusion-filtered source and cannot drift: an all-excluded
// staged/changed set is recognised as "nothing to commit" HERE — failing loud before any
// generate/AI call — rather than passing preflight and reaching the transport with a blank
// post-exclusion diff.
//
// A NON-empty would-be-staged set returns nil → the run proceeds to generate (as before).
// An EMPTY set selects the failure by the ACTUAL post-mode tree state (probed once with a
// read-only `git status --porcelain`), NOT the flag passed — so `mint commit -A` on a
// pristine tree yields the clean-tree message, because an empty -A set means a clean tree.
//
// Every probe runs with the repo ROOT as its working directory (root, via RunInDir) —
// the same anchoring the L1 diff sources use — because the shared `-- .` selector is
// cwd-relative: from a subdirectory an unanchored probe would miss staging outside the
// subtree and wrongly fail loud while the whole-index `git commit` had plenty to commit.
func checkSomethingToCommit(ctx context.Context, r runner.CommandRunner, root string, mode StagingMode, diffExclude []string) error {
	empty, err := wouldStageNothing(ctx, r, root, mode, diffExclude)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	return emptyStagingError(ctx, r, root, mode)
}

// wouldStageNothing reports whether the resolved StagingMode would stage nothing,
// computed READ-ONLY from name-only probes (sufficient for emptiness — no diff body is
// needed). The per-mode sources come from the SHARED sourcesForMode descriptor (the SAME
// one generate's sourceDiff consumes), so the dispatch is defined once; the would-be-staged
// set is EMPTY iff EVERY source spec is empty, which encodes the AddAll "tracked first,
// short-circuit on the first non-empty, else untracked" composition as a single
// all-specs-empty fold. EVERY probe carries the SAME diffExclude :(exclude) pathspecs (via
// probeArgs) the L1 source applies — so the probe measures the POST-exclusion would-be-staged
// set and an all-excluded set is reported empty here, matching the AI's L1 diff exactly:
//
//   - StagedOnly: empty iff `git diff --cached --name-only -- . :(exclude)…` is empty
//     (the staged index, post-exclusion — mirrors the staged L1 source).
//   - All (-a): empty iff `git diff HEAD --name-only -- . :(exclude)…` is empty (tracked
//     mods + deletions, post-exclusion — mirrors the tracked L1 source).
//   - AddAll (-A): empty iff BOTH `git diff HEAD --name-only -- . :(exclude)…` AND
//     `git ls-files --others --exclude-standard -- . :(exclude)…` are empty (tracked
//     changes AND untracked files, both post-exclusion — mirrors the AddAll L1 source).
//
// A genuine git failure is wrapped and surfaced so it is never mistaken for an empty set.
func wouldStageNothing(ctx context.Context, r runner.CommandRunner, root string, mode StagingMode, diffExclude []string) (bool, error) {
	for _, spec := range sourcesForMode(mode) {
		empty, err := gitOutputEmpty(ctx, r, root, probeArgs(spec, diffExclude)...)
		if err != nil {
			return false, err
		}
		if !empty {
			// First non-empty source short-circuits: the set is non-empty (the AddAll
			// "tracked first, else untracked" rule).
			return false, nil
		}
	}
	return true, nil
}

// probeArgs builds the name-only emptiness probe argv for ONE source spec, derived from
// the SAME shared base prefix the L1 source uses (via sourceArgs) so the verb, refspec,
// and `-- .` selector are never re-spelled. A diffSource carries `--name-only` (spliced
// after the verb/refspec, before the `-- .` selector — the body is not needed for
// emptiness); an untrackedListSource reuses its ls-files prefix VERBATIM (no `--name-only`),
// exactly the L1 enumeration argv. So a diff probe is provably the L1 diff argv plus
// `--name-only`, and the untracked probe is provably the L1 untracked argv.
func probeArgs(spec sourceSpec, diffExclude []string) []string {
	if spec.kind == untrackedListSource {
		return sourceArgs(spec.base, diffExclude)
	}
	return sourceArgs(nameOnly(spec.base), diffExclude)
}

// nameOnly splices `--name-only` into a `git diff …` base prefix, after the verb +
// refspec and BEFORE the `-- .` selector tail (the last two base elements: `--`, `.`).
// Keeping the selector tail in place means the probe is the base with one extra flag, not
// a re-spelled argv.
func nameOnly(base []string) []string {
	head := base[:len(base)-2]
	tail := base[len(base)-2:]
	withFlag := append(append([]string{}, head...), "--name-only")
	return append(withFlag, tail...)
}

// stagedProbeArgs / trackedProbeArgs / untrackedProbeArgs are the per-mode name-only
// emptiness probes, each derived from the matching shared source spec via probeArgs — so
// the probe argv is the L1 source argv plus `--name-only` (the two diff cases) / the
// shared ls-files prefix verbatim (the untracked case). They take the pre-mapped excludes
// for symmetry with their historical callers; probeArgs re-derives the same tail via
// excludePathspecs, so passing already-mapped excludes here is equivalent (the tests use
// these as the single checkable builders).
func stagedProbeArgs(excludes []string) []string {
	return append(nameOnly(stagedBaseArgs()), excludes...)
}

func trackedProbeArgs(excludes []string) []string {
	return append(nameOnly(trackedBaseArgs()), excludes...)
}

func untrackedProbeArgs(excludes []string) []string {
	return append(append([]string{}, untrackedBaseArgs()...), excludes...)
}

// emptyStagingError selects the fail-loud cause for an empty would-be-staged set, keyed on
// the ACTUAL tree state (a read-only `git status --porcelain`), NOT the flag passed:
//
//   - Genuinely clean tree (status empty → nothing anywhere) → errNothingToCommit. An
//     empty -A set ALWAYS lands here (if -A staged nothing, the tree is clean).
//   - Changes exist but the chosen mode staged none (status non-empty):
//   - StagedOnly (bare) → errNoChangesStaged.
//   - All (-a) → errNoTrackedChanges (only untracked remain — tracked changes would have
//     been captured by -a, so an empty -a set with changes present means they are
//     untracked; point at -A/--add-all).
//   - AddAll (-A) → unreachable (an empty -A set ⇒ a clean tree); defensively return the
//     clean-tree message.
func emptyStagingError(ctx context.Context, r runner.CommandRunner, root string, mode StagingMode) error {
	clean, err := gitOutputEmpty(ctx, r, root, "status", "--porcelain")
	if err != nil {
		return err
	}
	if clean {
		return errNothingToCommit
	}

	switch mode {
	case All:
		return errNoTrackedChanges
	case AddAll:
		// Unreachable: an empty -A would-be-staged set implies a clean tree, handled above.
		// Defensive fall-back to the clean-tree message keeps the function total.
		return errNothingToCommit
	default:
		return errNoChangesStaged
	}
}

// gitOutputEmpty runs a READ-ONLY git command from the repo root and reports whether
// its trimmed stdout is empty. It is the shared probe of the emptiness checks: a genuine
// git failure is wrapped and surfaced (never mistaken for an empty result). Anchoring at
// root keeps the cwd-relative `-- .` probes whole-tree from any invocation directory.
func gitOutputEmpty(ctx context.Context, r runner.CommandRunner, root string, args ...string) (bool, error) {
	res, err := r.RunInDir(ctx, root, nil, "git", args...)
	if err != nil {
		return false, fmt.Errorf("checking %v: %w", args, err)
	}
	return strings.TrimSpace(res.Stdout) == "", nil
}
