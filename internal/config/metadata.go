package config

// This file is the config-metadata SOURCE OF TRUTH (SoT): the single in-binary,
// structured record of mint's config metadata — one row per config key carrying
// key · level · default · description. It is schema-adjacent (it lives in the config
// package alongside the canonical decode-shape structs) deliberately so the drift
// test can reflect over the UNEXPORTED fileShape/releaseShape/commitShape/hooksShape
// struct tags from a same-package test (no exported reflection seam needed) and prove
// the SoT and the real schema cannot diverge.
//
// Why this exists: per-key meaning used to live only in the initgen commented template
// (a drift surface this feature strips out) and the README. The SoT centralises that
// metadata so mint setup's config reference renders from a drift-tested table rather
// than from template comments, and so a key added to the schema but not the SoT (or
// vice versa) fails the build.
//
// The Default column is intentionally LEFT EMPTY in this task — the field exists on the
// row type, but the default-representation convention is applied in task 1-2 and the
// literal compiled values are pinned in task 1-5. Do not read meaning into the empty
// Default cells yet.

// MetadataLevel is the typed enum naming WHERE a config key lives in the verb-namespaced
// .mint.toml: the shared top level, the [release] table, the nested [release.hooks]
// sub-table, or the [commit] table. It is a named int (not a raw string) so a SoT row's
// level is a closed, typed identity that the render target and the drift test agree on.
// String() renders the canonical TOML form of each level so both surfaces key on the
// same level identity.
type MetadataLevel int

const (
	// LevelShared is the top-level shared engine scope (no table header): ai_command,
	// max_diff_lines, timeout, diff_exclude. It is iota-0 so the zero value is the
	// shared scope, matching the schema's top-level fileShape.
	LevelShared MetadataLevel = iota
	// LevelRelease is the [release] table scope.
	LevelRelease
	// LevelReleaseHooks is the nested [release.hooks] sub-table scope.
	LevelReleaseHooks
	// LevelCommit is the [commit] table scope.
	LevelCommit
)

// String renders the level's canonical TOML form: the shared scope has no table header
// (empty string), and each table level renders its bracketed header. The render target
// (mint setup's config reference) and the drift test both key on this string, so the
// level identity is single-sourced here.
func (l MetadataLevel) String() string {
	switch l {
	case LevelRelease:
		return "[release]"
	case LevelReleaseHooks:
		return "[release.hooks]"
	case LevelCommit:
		return "[commit]"
	case LevelShared:
		return ""
	default:
		return ""
	}
}

// MetadataRow is one config-metadata SoT entry — one (Level, Key) pair plus its Default
// and Description columns. Row identity is the (Level, Key) PAIR, not Key alone: the
// dual-level ai_command/timeout keys appear as one row per level (shared + [release] +
// [commit]), never collapsed.
//
// Default is the compiled default's rendered representation. It is intentionally EMPTY
// in this task — the representation convention is applied in 1-2 and the literal values
// pinned in 1-5; the field exists now so the row shape is final.
type MetadataRow struct {
	Key         string
	Level       MetadataLevel
	Default     string
	Description string
}

// MetadataRows returns the full ordered slice of config-metadata SoT rows — one row per
// (level, key) pair across the whole schema, in schema field order: the shared top-level
// keys first, then [release], then the nested [release.hooks], then [commit]. The
// dual-level ai_command/timeout keys each appear three times (shared, [release],
// [commit]); the container fields release/commit/hooks emit NO row (their children carry
// their own level explicitly).
//
// The slice is built fresh on each call so callers cannot mutate a shared backing array.
// The count is NOT asserted here — the drift test (task 1-4) against the real schema is
// the authoritative guard against the SoT and the schema diverging.
func MetadataRows() []MetadataRow {
	return []MetadataRow{
		// Shared top-level engine keys (fileShape leaf fields), in field order.
		{Key: "ai_command", Level: LevelShared, Description: "the AI invocation: composed prompt on stdin, generated body on stdout; resolved [verb] → shared → default"},
		{Key: "max_diff_lines", Level: LevelShared, Description: "diffs larger than this (post-exclusion line count) skip the AI"},
		{Key: "timeout", Level: LevelShared, Description: "per-attempt AI deadline in seconds; 0 means no deadline; resolved [verb] → shared → default"},
		{Key: "diff_exclude", Level: LevelShared, Description: "extra pathspec globs kept out of every AI diff, on top of the built-in CHANGELOG.md exclusion"},

		// [release] table leaf keys (releaseShape leaf fields), in field order.
		{Key: "tag_prefix", Level: LevelRelease, Description: "prefix on the release tag name (e.g. v1.4.0)"},
		{Key: "commit_prefix", Level: LevelRelease, Description: "brand prefix on mint's bookkeeping commit subject"},
		{Key: "release_branch", Level: LevelRelease, Description: "branch releases must run on (empty auto-derives from origin/HEAD)"},
		{Key: "publish", Level: LevelRelease, Description: "publish a provider (GitHub) release, or stop at tag + push when false"},
		{Key: "changelog", Level: LevelRelease, Description: "write the CHANGELOG.md projection, or skip it when false (the tag still carries the full body)"},
		{Key: "provider", Level: LevelRelease, Description: "publishing-driver override (empty auto-detects from the release remote's host)"},
		{Key: "context", Level: LevelRelease, Description: "project guidance injected into the default release-notes prompt"},
		{Key: "prompt", Level: LevelRelease, Description: "path to a file whose contents fully override the default release-notes prompt"},
		{Key: "on_notes_failure", Level: LevelRelease, Description: "notes-failure policy: abort fails loud, fallback uses the commit-subject / fixed-string body"},
		{Key: "fallback", Level: LevelRelease, Description: "fixed fallback body used verbatim by on_notes_failure = fallback and --no-ai (empty uses the commit-subject list)"},
		{Key: "version_file", Level: LevelRelease, Description: "repo-relative file the new version is mirrored into (empty means tag-only, no projection)"},
		{Key: "version_pattern", Level: LevelRelease, Description: "version line replaced inside version_file (empty treats the whole file as the version)"},
		{Key: "ai_command", Level: LevelRelease, Description: "optional per-verb override of the AI command for release; resolved [release] → shared → default"},
		{Key: "timeout", Level: LevelRelease, Description: "optional per-verb override of the per-attempt AI deadline (seconds) for release; resolved [release] → shared → default"},

		// [release.hooks] sub-table keys (hooksShape fields), in field order.
		{Key: "preflight", Level: LevelReleaseHooks, Description: "runs before any release work; failure aborts the release"},
		{Key: "pre_tag", Level: LevelReleaseHooks, Description: "runs after notes, before the tag; accepts a single command or an ordered array of commands"},
		{Key: "post_release", Level: LevelReleaseHooks, Description: "runs after the release is published"},

		// [commit] table keys (commitShape leaf fields), in field order.
		{Key: "context", Level: LevelCommit, Description: "project guidance injected into the default commit-message prompt"},
		{Key: "prompt", Level: LevelCommit, Description: "path to a file whose contents fully override the default commit-message prompt"},
		{Key: "ai_command", Level: LevelCommit, Description: "optional per-verb override of the AI command for commit; resolved [commit] → shared → default"},
		{Key: "timeout", Level: LevelCommit, Description: "optional per-verb override of the per-attempt AI deadline (seconds) for commit; resolved [commit] → shared → default"},
	}
}
