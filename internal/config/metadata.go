package config

import (
	"fmt"
	"strconv"
	"time"
)

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
// The Default column carries each key's compiled default in the DECIDED representation
// convention — the same convention the README per-key tables use, so the two surfaces
// (agent-facing SoT, human-facing README) stay mutually consistent. The convention
// tokens: a blank cell for empty-string defaults (genuinely no value), the word "auto"
// for the sentinel-empty auto-derive/auto-detect keys (release_branch, provider — whose
// "" MEANS auto, not absence), "[]" for the empty collection (diff_exclude), the word
// "shared" for the per-verb ai_command/timeout overrides (which inherit the shared value
// unless overridden), and the concrete scalar default verbatim otherwise. The
// [release.hooks] keys are activate-only (no compiled default) and carry a blank cell.
// The two shared-level literal-default cells (ai_command, timeout) are sourced from the
// EXPORTED config constants (DefaultAICommand / DefaultTimeout) so they cannot drift; a
// dedicated pinning test in metadata_test.go ties each cell to its constant. This pin
// SUBSUMES initgen's scaffold value-drift pins — the SoT Default column is now the
// drift-pinned carrier of those values (the scaffold's defaults are stripped in a later
// phase). timeout's TOML key is integer seconds, so DefaultTimeout (a duration) renders
// as int(DefaultTimeout / time.Second), the same derivation initgen used.

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
//
// The default branch ENFORCES the closed-enum claim. LevelShared legitimately renders ""
// (its real TOML form — the top-level shared scope has no table header), so the fallback
// must NOT also return "": an undeclared/out-of-range level (a corrupted or uninitialised
// MetadataLevel) sharing LevelShared's output would let it silently masquerade as the
// shared/top-level cell in the agent-facing config table (setupguide.LevelCell maps the
// empty string — and only the empty string — to "top-level"). We return the conventional
// Stringer sentinel ("MetadataLevel(N)", what golang.org/x/tools/cmd/stringer generates)
// rather than panicking: a Stringer is expected to be safe to call, and a panic in the
// render path would crash `mint setup` outright — strictly worse than an unmistakable,
// self-describing cell that names the offending value. The sentinel is non-empty and
// cannot collide with any legitimate scope output, so render and any future caller can
// tell an invalid level apart from the genuine shared scope.
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
		return fmt.Sprintf("MetadataLevel(%d)", int(l))
	}
}

// MetadataRow is one config-metadata SoT entry — one (Level, Key) pair plus its Default
// and Description columns. Row identity is the (Level, Key) PAIR, not Key alone: the
// dual-level ai_command/timeout keys appear as one row per level (shared + [release] +
// [commit]), never collapsed.
//
// Default is the compiled default's rendered representation in the decided convention
// (see the file header for the full token set): blank, "auto", "[]", "shared", or the
// concrete scalar default verbatim. The shared-level ai_command/timeout cells are sourced
// from the exported config constants so they cannot drift (pinned in metadata_test.go).
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
		// All three shared scalar defaults (ai_command, timeout, max_diff_lines) carry the
		// REAL compiled defaults, sourced from the exported config constants so they cannot
		// drift (each pinned in metadata_test.go against DefaultAICommand / DefaultTimeout /
		// DefaultMaxDiffLines). timeout's TOML key is integer seconds, so DefaultTimeout
		// (a duration) renders as int(DefaultTimeout / time.Second).
		{Key: "ai_command", Level: LevelShared, Default: DefaultAICommand, Description: "the AI invocation: composed prompt on stdin, generated body on stdout; resolved [verb] → shared → default"},
		{Key: "max_diff_lines", Level: LevelShared, Default: strconv.Itoa(DefaultMaxDiffLines), Description: "diffs larger than this (post-exclusion line count) skip the AI"},
		{Key: "timeout", Level: LevelShared, Default: strconv.Itoa(int(DefaultTimeout / time.Second)), Description: "per-attempt AI deadline in seconds; 0 means no deadline; resolved [verb] → shared → default"},
		{Key: "diff_exclude", Level: LevelShared, Default: "[]", Description: "extra pathspec globs kept out of every AI diff, on top of the built-in CHANGELOG.md exclusion"},

		// [release] table leaf keys (releaseShape leaf fields), in field order.
		// release_branch/provider default to "" in the struct, but their "" is a SENTINEL
		// meaning auto-derive/auto-detect — so they render "auto", distinct from the genuinely
		// blank empty-string keys (context/prompt/fallback/version_file/version_pattern).
		// The per-verb ai_command/timeout overrides render "shared" (inherit the shared
		// top-level value unless overridden) — the inverse of the shared-level rows.
		{Key: "tag_prefix", Level: LevelRelease, Default: "v", Description: "prefix on the release tag name (e.g. v1.4.0)"},
		{Key: "commit_prefix", Level: LevelRelease, Default: "🌿", Description: "brand prefix on mint's bookkeeping commit subject"},
		{Key: "release_branch", Level: LevelRelease, Default: "auto", Description: "branch releases must run on (empty auto-derives from origin/HEAD)"},
		{Key: "publish", Level: LevelRelease, Default: "true", Description: "publish a provider (GitHub) release, or stop at tag + push when false"},
		{Key: "changelog", Level: LevelRelease, Default: "true", Description: "write the CHANGELOG.md projection, or skip it when false (the tag still carries the full body)"},
		{Key: "provider", Level: LevelRelease, Default: "auto", Description: "publishing-driver override (empty auto-detects from the release remote's host)"},
		{Key: "context", Level: LevelRelease, Description: "project guidance injected into the default release-notes prompt"},
		{Key: "prompt", Level: LevelRelease, Description: "path to a file whose contents fully override the default release-notes prompt"},
		{Key: "on_notes_failure", Level: LevelRelease, Default: "abort", Description: "notes-failure policy: abort fails loud, fallback uses the commit-subject / fixed-string body"},
		{Key: "fallback", Level: LevelRelease, Description: "fixed fallback body used verbatim by on_notes_failure = fallback and --no-ai (empty uses the commit-subject list)"},
		{Key: "version_file", Level: LevelRelease, Description: "repo-relative file the new version is mirrored into (empty means tag-only, no projection)"},
		{Key: "version_pattern", Level: LevelRelease, Description: "version line replaced inside version_file (empty treats the whole file as the version)"},
		{Key: "ai_command", Level: LevelRelease, Default: "shared", Description: "optional per-verb override of the AI command for release; resolved [release] → shared → default"},
		{Key: "timeout", Level: LevelRelease, Default: "shared", Description: "optional per-verb override of the per-attempt AI deadline (seconds) for release; resolved [release] → shared → default"},

		// [release.hooks] sub-table keys (hooksShape fields), in field order. These are
		// activate-only — no compiled default — so their Default cell is left BLANK (the
		// chosen no-default token; the description carries when each hook runs).
		{Key: "preflight", Level: LevelReleaseHooks, Description: "runs before any release work; failure aborts the release"},
		{Key: "pre_tag", Level: LevelReleaseHooks, Description: "runs after notes, before the tag; accepts a single command or an ordered array of commands"},
		{Key: "post_release", Level: LevelReleaseHooks, Description: "runs after the release is published"},

		// [commit] table keys (commitShape leaf fields), in field order. context/prompt
		// mirror [release]'s empty-string defaults (blank cell); ai_command/timeout render
		// "shared" (inherit), mirroring the [release] override rows.
		{Key: "context", Level: LevelCommit, Description: "project guidance injected into the default commit-message prompt"},
		{Key: "prompt", Level: LevelCommit, Description: "path to a file whose contents fully override the default commit-message prompt"},
		{Key: "ai_command", Level: LevelCommit, Default: "shared", Description: "optional per-verb override of the AI command for commit; resolved [commit] → shared → default"},
		{Key: "timeout", Level: LevelCommit, Default: "shared", Description: "optional per-verb override of the per-attempt AI deadline (seconds) for commit; resolved [commit] → shared → default"},
	}
}
