package commit

// This file is commit's SINGLE source-of-truth for the per-mode git source selection
// shared by BOTH the preflight emptiness probes (preflight.go) and the AI's L1 diff
// sources (generate.go). The spec makes "the preflight and the AI's L1 diff read ONE
// exclusion-filtered source and cannot drift" a load-bearing invariant; colocating the
// per-mode base argv prefixes, the `-- .` selector, and the StagingMode→sources mapping
// HERE makes that invariant STRUCTURAL rather than two hand-aligned copies.
//
// Each per-mode source is described once by a sourceSpec (its base argv prefix + its
// kind). Both consumers iterate the SAME spec list per mode:
//
//   - generate.go (L1 diff) appends excludePathspecs to each spec's base and renders the
//     diff body (a diff source uses git's stdout verbatim; an untracked-list source
//     enumerates paths then renders each as a read-only addition diff).
//   - preflight.go (emptiness) appends `--name-only` (diff specs only) + excludePathspecs
//     and reports the would-be-staged set EMPTY iff EVERY spec is empty — which encodes
//     the AddAll "tracked first, short-circuit on non-empty, else untracked" composition
//     in ONE place (an all-specs-empty fold).
//
// So a mode's source-command prefix, its `-- .` selector, and the AddAll composition rule
// each live in exactly one place; the preflight probe argv is provably the same
// exclusion-filtered source as the L1 diff, differing only by the `--name-only` the
// emptiness probe adds.

// sourceKind classifies a per-mode source so the two consumers render it correctly: a
// diffSource is a `git diff …` whose stdout IS the diff (preflight adds `--name-only` to
// it), while an untrackedListSource is a `git ls-files --others …` enumeration whose
// paths are rendered as read-only addition diffs on the L1 path and whose emptiness IS
// the probe on the preflight path (NO `--name-only` — the ls-files prefix is shared
// verbatim).
type sourceKind int

const (
	// diffSource is a `git diff …` source: stdout is the diff body (L1), and the
	// emptiness probe carries `--name-only`.
	diffSource sourceKind = iota
	// untrackedListSource is a `git ls-files --others --exclude-standard …` enumeration:
	// each listed path becomes a read-only addition diff (L1), and the probe uses the
	// SAME prefix verbatim (no `--name-only`).
	untrackedListSource
)

// sourceSpec is the single description of ONE per-mode git source: the base argv prefix
// (`[verb, refspec/flags, "--", "."]`, with no excludes — callers append those via the
// shared exclusion tail) and its kind. It is the structural unit both the L1 diff sources
// and the preflight probes derive from.
type sourceSpec struct {
	base []string
	kind sourceKind
}

// stagedBaseArgs is the StagedOnly source prefix: `git diff --cached -- .` — the staged
// index. It is spelled ONCE here; both the L1 diff render (renderSource) and
// stagedProbeArgs (preflight) derive from it, appending the shared exclusion tail (and
// `--name-only` for the probe).
func stagedBaseArgs() []string {
	return []string{"diff", "--cached", "--", "."}
}

// trackedBaseArgs is the All (-a) / AddAll (-A) tracked source prefix: `git diff HEAD
// -- .` — tracked modifications + deletions against HEAD (no untracked). Spelled ONCE;
// the L1 diff render (renderSource) and trackedProbeArgs (preflight) both derive from it.
func trackedBaseArgs() []string {
	return []string{"diff", "HEAD", "--", "."}
}

// untrackedBaseArgs is the AddAll (-A) untracked source prefix: `git ls-files --others
// --exclude-standard -- .` — the untracked, non-ignored enumeration. Spelled ONCE;
// the L1 untracked render (untrackedAdditions) enumerates from it and untrackedProbeArgs
// (preflight) reuses it VERBATIM (no `--name-only`).
func untrackedBaseArgs() []string {
	return []string{"ls-files", "--others", "--exclude-standard", "--", "."}
}

// sourcesForMode is the SINGLE StagingMode→sources mapping both the emptiness path
// (wouldStageNothing) and the diff path (sourceDiff) consume — so the dispatch and the
// AddAll "tracked then untracked" composition are defined exactly once:
//
//   - All (-a): the tracked diff alone.
//   - AddAll (-A): the tracked diff THEN the untracked enumeration (the order the L1
//     combined diff uses; the emptiness fold makes "tracked first, short-circuit on
//     non-empty, else untracked" fall out as all-specs-empty).
//   - StagedOnly (the default): the staged diff alone.
func sourcesForMode(mode StagingMode) []sourceSpec {
	switch mode {
	case All:
		return []sourceSpec{{base: trackedBaseArgs(), kind: diffSource}}
	case AddAll:
		return []sourceSpec{
			{base: trackedBaseArgs(), kind: diffSource},
			{base: untrackedBaseArgs(), kind: untrackedListSource},
		}
	default:
		return []sourceSpec{{base: stagedBaseArgs(), kind: diffSource}}
	}
}

// sourceArgs is the L1 argv for a source: its base prefix plus the shared
// excludePathspecs exclusion tail (the SAME tail the preflight probe appends). It is the
// single place the L1 diff sources turn a base prefix into the executed argv.
func sourceArgs(base, diffExclude []string) []string {
	return append(append([]string{}, base...), excludePathspecs(diffExclude)...)
}
