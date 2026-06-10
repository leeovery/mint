// Package hooks is mint's shared lifecycle-hook mechanism. A hook is a config
// value under [release.hooks] keyed by lifecycle point (preflight, pre_tag,
// post_release); the value is a single shell command string or an ordered array
// of them. This package runs those commands as `sh -c "<entry>"` from the repo
// root with mint's MINT_* variables injected on top of the inherited environment,
// stopping at the first non-zero exit.
//
// It owns the mechanism ONLY: which lifecycle point a hook belongs to, and
// whether a failure aborts the release or merely warns, is decided by the caller
// wiring each point — not here. Run therefore takes the already-parsed config
// value (typed `any` because a TOML decoder surfaces it as a string or a slice)
// rather than reading [release.hooks] itself.
package hooks

// Bump is the kind of version change a release performs. It renders to MINT_BUMP
// so a hook can branch on how the version was chosen.
type Bump string

const (
	// BumpPatch/BumpMinor/BumpMajor correspond to the --patch/--minor/--major
	// bump flags.
	BumpPatch Bump = "patch"
	BumpMinor Bump = "minor"
	BumpMajor Bump = "major"
	// BumpExplicit is used when --set-version pinned an exact version rather than
	// deriving it from a bump flag. (The --set-version flag itself lands in a later
	// phase; the value is supported here so the mapping is complete.)
	BumpExplicit Bump = "explicit"
)

// HookEnv carries the release-state variables mint injects into every hook entry.
// It is assembled once per release (via NewHookEnv) and reused across the three
// lifecycle points so they share one render point. The set may grow as later
// stages need more context; Render is the single place it is materialised.
type HookEnv struct {
	NewVersion      string
	PreviousVersion string
	VersionTag      string
	Bump            Bump
	DryRun          bool
}

// NewHookEnv assembles a HookEnv from release state so the lifecycle points share
// one builder. bump maps straight to MINT_BUMP (patch/minor/major for the bump
// flags, explicit for --set-version); dryRun renders to "1"/"0".
func NewHookEnv(newVersion, previousVersion, versionTag string, bump Bump, dryRun bool) HookEnv {
	return HookEnv{
		NewVersion:      newVersion,
		PreviousVersion: previousVersion,
		VersionTag:      versionTag,
		Bump:            bump,
		DryRun:          dryRun,
	}
}

// Render materialises the injected variables as an ordered slice of "KEY=VALUE"
// entries suitable for layering on the inherited environment. It is the SINGLE
// render point for the MINT_* set, so adding a variable is one edit here.
func (e HookEnv) Render() []string {
	return []string{
		"MINT_NEW_VERSION=" + e.NewVersion,
		"MINT_PREVIOUS_VERSION=" + e.PreviousVersion,
		"MINT_VERSION_TAG=" + e.VersionTag,
		"MINT_BUMP=" + string(e.Bump),
		"MINT_DRY_RUN=" + renderDryRun(e.DryRun),
	}
}

// renderDryRun maps the dry-run flag to the "1"/"0" form hooks expect.
func renderDryRun(dryRun bool) string {
	if dryRun {
		return "1"
	}
	return "0"
}
