package commit

// This file is commit's EMPTY-STAGING PREFLIGHT cluster — the staging-mode-aware
// "something to commit?" check that runs BEFORE generate so the run fails loud on a
// genuinely empty tree before any AI call. The same read-only probes count what the
// commit would carry, the files and the lines added and removed, for the diff's row:
// git counts the tracked changes, and the untracked files -A lists are read from the
// disk (stat.go).
//
// The preflight answers ONE question: would `git commit` (for the resolved
// StagingMode) create a commit? That is a property of the working tree and index
// ALONE — it is deliberately INDEPENDENT of diff_exclude. diff_exclude is an
// AI-CONTEXT filter (it shapes what the model reads — see generate.go); it must NEVER
// decide whether the tree is dirty or whether a commit can be made. So every probe
// here carries NO :(exclude) pathspecs: a changeset whose every file matches a
// diff_exclude glob is STILL something to commit, and the preflight lets it through.
// The "the AI's post-exclusion diff is empty" case (an all-excluded but non-empty
// tree) is handled DOWNSTREAM by the empty-AI-diff editor fallback (generate.go's
// errDiffFullyExcluded → run.go's runEditorFallback), NOT by failing the preflight —
// so the excluded files still get committed, with a human-written message. For the
// same reason the counts are the whole commit's, not the AI's post-exclusion view, so
// they are read here rather than from the diff the AI reads.
//
// The probes DERIVE their per-mode source COMMANDS from the SAME shared sourcesForMode
// descriptor (source.go) the L1 diff sources use, so the preflight and the AI source
// dispatch the identical git command per mode — they differ ONLY in the tail: the
// preflight applies no exclusion (and adds `--numstat` to the diff probes), while
// the L1 source applies diff_exclude. The shared descriptor keeps the command
// selection and the AddAll composition single-sourced; the exclusion is L1-only.
//
// The three sentinel messages, checkSomethingToCommit, readDiffStat, the per-mode
// probe builders, emptyStagingError, and gitOutputEmpty all live here; run.go keeps only
// the orchestration spine.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

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
// It counts what the commit would carry for the resolved StagingMode READ-ONLY (no
// `git add`, no AI) and fails loud when that is nothing, short-circuiting generation so
// the run never proceeds on a genuinely clean tree; otherwise it returns the count. All
// probes go through the consumed CommandRunner seam (the same read-only idiom as
// generate's source helpers), so they are fully scriptable via the FakeRunner.
//
// It does NOT consult diff_exclude: the emptiness verdict is "would `git commit` create a
// commit?", a property of the tree/index alone. diff_exclude only shapes the AI's L1 diff
// (generate.go) — an all-excluded but non-empty changeset is NOT "nothing to commit", so
// it passes preflight here and is handled by the empty-AI-diff editor fallback downstream
// (it still gets committed, with a human-written message), rather than being wrongly
// rejected as empty.
//
// An EMPTY set selects the failure by the ACTUAL post-mode tree state (probed once with a
// read-only `git status --porcelain`), NOT the flag passed — so `mint commit -A` on a
// pristine tree yields the clean-tree message, because an empty -A set means a clean tree.
//
// Every probe runs with the repo ROOT as its working directory (root, via RunInDir) —
// the same anchoring the L1 diff sources use — because the shared `-- .` selector is
// cwd-relative: from a subdirectory an unanchored probe would miss staging outside the
// subtree and wrongly fail loud while the whole-index `git commit` had plenty to commit.
func checkSomethingToCommit(ctx context.Context, r runner.CommandRunner, root string, mode StagingMode) (diffStat, error) {
	stat, found, err := readDiffStat(ctx, r, root, mode)
	if err != nil {
		return diffStat{}, err
	}
	if found {
		return stat, nil
	}
	return diffStat{}, emptyStagingError(ctx, r, root, mode)
}

// readDiffStat counts what the resolved StagingMode would commit, READ-ONLY, from one
// probe per source the SHARED sourcesForMode descriptor lists for the mode (the SAME one
// generate's sourceDiff consumes), so the dispatch is defined once; found is whether any
// probe named something, the would-be-staged set being EMPTY iff none did. NO probe
// carries diff_exclude — the preflight measures the FULL would-be-staged set (not the
// AI's post-exclusion view), so a changeset whose only files are diff_exclude'd is still
// found here and proceeds:
//
//   - StagedOnly: `git diff --cached --numstat -- .` (the staged index — the staged
//     source command, no exclusion).
//   - All (-a): `git diff HEAD --numstat -- .` (tracked mods + deletions — the tracked
//     source command, no exclusion).
//   - AddAll (-A): that, plus `git ls-files --others --exclude-standard -z -- .`, whose
//     files are counted from the disk (countUntracked), not by a git call each (tracked
//     changes AND untracked files — both source commands, no exclusion).
//
// The verdict rests on what git listed alone: the count is display only, so a file it
// can't read leaves the count, never the verdict. A genuine git failure is wrapped and
// surfaced so it is never mistaken for an empty set.
func readDiffStat(ctx context.Context, r runner.CommandRunner, root string, mode StagingMode) (diffStat, bool, error) {
	var stat diffStat
	found := false
	for _, spec := range sourcesForMode(mode) {
		args := probeArgs(spec)
		res, err := r.RunInDir(ctx, root, nil, "git", args...)
		if err != nil {
			return diffStat{}, false, fmt.Errorf("checking %v: %w", args, err)
		}
		found = found || !blank(res.Stdout)
		if spec.kind == untrackedListSource {
			stat = stat.plus(countUntracked(root, res.Stdout))
		} else {
			stat = stat.plus(parseNumstat(res.Stdout))
		}
	}
	return stat, found, nil
}

// probeArgs builds the counting probe argv for ONE source spec, derived from the SAME
// shared base prefix the L1 source uses so the verb, refspec, and `-- .` selector are
// never re-spelled. It carries NO exclusion tail (the preflight is diff_exclude-blind): a
// diffSource gets `--numstat` spliced after the verb/refspec, before the `-- .` selector;
// an untrackedListSource reuses its ls-files prefix VERBATIM, exactly the L1 enumeration
// command minus the excludes. So a diff probe is the L1 diff command (sans excludes) plus
// `--numstat`, and the untracked probe is the L1 untracked command (sans excludes).
func probeArgs(spec sourceSpec) []string {
	if spec.kind == untrackedListSource {
		return append([]string{}, spec.base...)
	}
	return withNumstat(spec.base)
}

// withNumstat splices `--numstat` into a `git diff …` base prefix, after the verb +
// refspec and BEFORE the `-- .` selector tail (the last two base elements: `--`, `.`).
// Keeping the selector tail in place means the probe is the base with one extra flag, not
// a re-spelled argv.
func withNumstat(base []string) []string {
	head := base[:len(base)-2]
	tail := base[len(base)-2:]
	withFlag := append(append([]string{}, head...), "--numstat")
	return append(withFlag, tail...)
}

// stagedProbeArgs / trackedProbeArgs / untrackedProbeArgs are the per-mode counting
// probes, each derived from the matching shared source command via the same base
// builders probeArgs uses — so the probe argv is the L1 source command (WITHOUT the
// diff_exclude tail) plus `--numstat` (the two diff cases) / the shared ls-files prefix
// verbatim (the untracked case). They are the single checkable builders for the tests.
//
// NOTE: these are test-facing builders. Production preflight routes through probeArgs (see
// readDiffStat → probeArgs); nothing on the live preflight path calls
// stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs.
func stagedProbeArgs() []string {
	return withNumstat(stagedBaseArgs())
}

func trackedProbeArgs() []string {
	return withNumstat(trackedBaseArgs())
}

func untrackedProbeArgs() []string {
	return append([]string{}, untrackedBaseArgs()...)
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

// gitOutputEmpty runs a READ-ONLY git command from the repo root — the clean-tree
// check's `git status --porcelain` — and reports whether its stdout is blank: a genuine
// git failure is wrapped and surfaced (never mistaken for an empty result). Anchoring at
// root keeps it whole-tree from any invocation directory.
func gitOutputEmpty(ctx context.Context, r runner.CommandRunner, root string, args ...string) (bool, error) {
	res, err := r.RunInDir(ctx, root, nil, "git", args...)
	if err != nil {
		return false, fmt.Errorf("checking %v: %w", args, err)
	}
	return blank(res.Stdout), nil
}

// blank is whether a probe's stdout holds nothing but separators: whitespace, and the
// NUL that terminates each entry of the untracked probe's `-z` enumeration.
func blank(stdout string) bool {
	return strings.TrimFunc(stdout, func(c rune) bool {
		return c == 0 || unicode.IsSpace(c)
	}) == ""
}
