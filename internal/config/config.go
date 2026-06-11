// Package config loads mint's optional .mint.toml from the repo root. Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// Config is the single CANONICAL schema for the verb-namespaced .mint.toml: the
// shared engine keys at the top level (ai_command, diff_exclude, max_diff_lines),
// the [release] table, and the nested [release.hooks] sub-table. Every documented
// key has its Go type here and a default applied uniformly — on zero config (file
// absent, empty, or comment-only) every key comes back at its documented default,
// and a file that sets only part of a table leaves the unset keys at their
// defaults individually.
//
// Typed fail-loud validation (rejecting unknown keys / bad types) and rewiring the
// earlier per-key reads through this schema are SEPARATE Phase 6 tasks; this file
// establishes the consolidated shape and complete default application only. Until
// validation lands, unknown keys are tolerated and ignored rather than rejected.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// configFileName is the fixed name mint looks for at the repo root.
const configFileName = ".mint.toml"

// Default values for the Phase 1 [release] keys. ReleaseBranch's default is the
// empty string, a sentinel meaning "auto-derive from origin/HEAD" (resolved in
// task 1-4); an explicit empty value in the file is indistinguishable from this
// and means the same thing.
const (
	defaultTagPrefix    = "v"
	defaultCommitPrefix = "🌿"
	defaultPublish      = true
	defaultChangelog    = true
)

// defaultOnNotesFailure is the out-of-the-box notes-failure policy: "abort" — when
// the normal AI path fails, mint fails loud and tags nothing. The opt-in alternative
// is "fallback"; the notes engine's resolver interprets the value as MODE-ONLY
// (abort | fallback). config carries the raw string verbatim (Phase 6 adds typed
// validation that rejects unknown values).
const defaultOnNotesFailure = "abort"

// defaultMaxDiffLines is the out-of-the-box ceiling for the notes-engine
// max_diff_lines guard: a post-exclusion diff larger than this is too costly to
// summarise well, so the AI is skipped. It is a shared engine key, not release
// specific, so it lives at the top level of Config (see Config.MaxDiffLines).
const defaultMaxDiffLines = 50000

// defaultAICommand is the out-of-the-box notes transport command: `claude -p`,
// the AI invocation mint pipes the composed prompt into. It is a shared engine
// key (every verb's notes engine uses it), so it lives at the top level of Config
// (see Config.AICommand). An explicit empty value is re-defaulted by the transport
// itself, not here — config carries whatever the file holds verbatim, applying this
// default only when the key is absent.
const defaultAICommand = "claude -p"

// Config is the loaded mint configuration. The [release] table plus the
// shared top-level engine keys read so far are populated; the remaining
// engine-level keys and other verb tables arrive in later phases.
type Config struct {
	Release Release

	// AICommand is the shared engine-level ai_command notes-transport command (default
	// "claude -p"). It is top-level — NOT under [release] — because every verb's notes
	// engine uses the same AI transport. config carries it verbatim; the transport
	// re-defaults an explicit empty value and whitespace-splits the command into name +
	// args (it is operator-controlled config, not arbitrary input).
	AICommand string

	// MaxDiffLines is the shared engine-level max_diff_lines guard ceiling (default
	// 50000). It is top-level — NOT under [release] — because it serves every verb's
	// notes engine, not just release. The notes size guard compares the
	// post-exclusion diff's line count against it.
	MaxDiffLines int

	// DiffExclude is the shared engine-level list of extra glob pathspecs to exclude
	// from the diff, ON TOP OF the built-in CHANGELOG.md exclusion. It is top-level —
	// NOT under [release] — because it serves every verb's notes engine. Each entry is
	// a git pathspec glob (e.g. "skills/**/knowledge.cjs") the notes engine turns into
	// a :(exclude)<glob> argument; git performs the matching, mint does no Go-side glob
	// matching. Absent → nil/empty, so only CHANGELOG.md is excluded.
	DiffExclude []string
}

// Release holds the [release] table values needed so far: TagPrefix and
// CommitPrefix feed tag/commit subjects, ReleaseBranch gates the on-branch check
// (empty = auto-derive), Publish decides whether to publish a GitHub release
// or stop at tag + push, and Changelog decides whether the CHANGELOG.md
// projection is written (default true) or skipped — the annotated tag still
// carries the full body either way.
//
// Provider is the OPTIONAL publishing-driver override (raw [release].provider,
// default ""). Empty means "auto-detect from the release remote's host"; a
// non-empty value forces that provider's driver regardless of the host (e.g.
// "github"). config carries the raw value verbatim — the publish resolver
// interprets it (a recognised value selects its driver; a recognised-but-
// unsupported value, e.g. "gitlab", is NOT a config error but routes to the
// safe-downgrade path). Phase 6's typed validation rejects unknown KEYS / bad
// TYPES, not unsupported provider VALUES.
//
// Context and Prompt are the Phase 2 notes-engine prompt-control knobs, carried
// here as raw TOML strings (both default empty). Context (string-or-file) injects
// project guidance into the default prompt; Prompt is a file path that fully
// overrides the default prompt. The string-or-file detection and file reading live
// in the notes engine, NOT here — config carries the raw values verbatim.
//
// OnNotesFailure is the normal-path notes-failure policy (default "abort"). config
// carries the raw value verbatim; the notes engine's ResolveFailure interprets it as
// MODE-ONLY ("" / "abort" → abort; "fallback" → commit-subject fallback; any other
// value → abort for now, rejected by Phase 6's typed validation).
//
// Fallback is the dedicated fixed-fallback-body string (raw [release].fallback,
// default ""). It is SHARED by both fallback paths — on_notes_failure=fallback and
// --no-ai: when non-empty it is used verbatim as the body in place of the
// commit-subject list. Empty means "no fixed string, use the commit-subject list".
// Unlike OnNotesFailure (a mode), this carries the body string itself.
//
// VersionFile and VersionPattern are the optional version-file projection knobs
// (raw [release].version_file / [release].version_pattern, both default ""). They
// are carried verbatim for the Record stage. VersionFile empty means "tag-only, no
// projection"; non-empty is the repo-relative path mint mirrors the new version
// into (a write-only mirror, never a version source). VersionPattern empty selects
// PLAIN mode (the whole file is the version); non-empty selects EMBEDDED mode
// (surgical version-line replacement inside a real source file).
type Release struct {
	TagPrefix      string
	CommitPrefix   string
	ReleaseBranch  string
	Publish        bool
	Changelog      bool
	Provider       string
	Context        string
	Prompt         string
	OnNotesFailure string
	Fallback       string
	VersionFile    string
	VersionPattern string
	Hooks          Hooks
}

// HookValue is the dedicated string-or-array type for a [release.hooks] entry: a
// TOML hook value is either a single command string or an ordered array of command
// strings, and HookValue accepts BOTH at the schema level. Its underlying type is
// the empty interface, so the decoder surfaces a TOML string as a HookValue carrying
// a string and a TOML array as a HookValue carrying a slice, both verbatim, while a
// nil HookValue means the key was absent (no hook, a no-op). config does NOT
// interpret or normalise the value — the hooks package coerces the carried shape to
// ordered command strings when it runs the hook, and strict string-vs-array
// validation is a separate Phase 6 task.
type HookValue any

// Hooks carries the RAW parsed [release.hooks] values keyed by lifecycle point.
// Each is a HookValue (string-or-array); a nil field means the key was absent.
type Hooks struct {
	Preflight   HookValue
	PreTag      HookValue
	PostRelease HookValue
}

// defaults returns a Config seeded with the Phase 1 default values.
func defaults() Config {
	return Config{
		Release: Release{
			TagPrefix:      defaultTagPrefix,
			CommitPrefix:   defaultCommitPrefix,
			ReleaseBranch:  "",
			Publish:        defaultPublish,
			Changelog:      defaultChangelog,
			Provider:       "",
			Context:        "",
			Prompt:         "",
			OnNotesFailure: defaultOnNotesFailure,
			Fallback:       "",
			VersionFile:    "",
			VersionPattern: "",
		},
		AICommand:    defaultAICommand,
		MaxDiffLines: defaultMaxDiffLines,
	}
}

// fileShape mirrors the on-disk TOML so absent keys can be told apart from
// present-but-zero ones. Publish and Changelog are *bool because their zero value
// (false) is a meaningful, explicit choice: nil means "key absent, apply default
// true" while a non-nil false means the surface is disabled. MaxDiffLines is a *int for the same
// reason — nil means "key absent, apply default 50000" while a non-nil value
// (even 0) is an explicit operator choice. The string fields are decoded onto a
// struct pre-seeded with defaults, so the decoder only overwrites keys actually
// present in the file — an explicit empty tag_prefix overwrites "v" with "" (a
// valid prefix-less choice) while an absent key leaves the default intact.
type fileShape struct {
	Release      releaseShape `toml:"release"`
	AICommand    *string      `toml:"ai_command"`
	MaxDiffLines *int         `toml:"max_diff_lines"`
	DiffExclude  []string     `toml:"diff_exclude"`
}

type releaseShape struct {
	TagPrefix      string     `toml:"tag_prefix"`
	CommitPrefix   string     `toml:"commit_prefix"`
	ReleaseBranch  string     `toml:"release_branch"`
	Publish        *bool      `toml:"publish"`
	Changelog      *bool      `toml:"changelog"`
	Provider       string     `toml:"provider"`
	Context        string     `toml:"context"`
	Prompt         string     `toml:"prompt"`
	OnNotesFailure string     `toml:"on_notes_failure"`
	Fallback       string     `toml:"fallback"`
	VersionFile    string     `toml:"version_file"`
	VersionPattern string     `toml:"version_pattern"`
	Hooks          hooksShape `toml:"hooks"`
}

// hooksShape mirrors the on-disk [release.hooks] sub-table. Each key is a HookValue
// so the decoder surfaces whatever TOML shape the value has (a string or an array)
// verbatim; an absent key leaves the field nil. resolveRelease copies these straight
// onto Release.Hooks.
type hooksShape struct {
	Preflight   HookValue `toml:"preflight"`
	PreTag      HookValue `toml:"pre_tag"`
	PostRelease HookValue `toml:"post_release"`
}

// Load reads {root}/.mint.toml and returns the Phase 1 config. A missing file is
// not an error — config is optional, so an absent file yields all defaults. A
// present file overrides only the keys it specifies; absent keys keep their
// defaults. Malformed TOML surfaces as an error.
func Load(root string) (Config, error) {
	path := filepath.Join(root, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return defaults(), nil
		}
		return Config{}, fmt.Errorf("reading %s: %w", configFileName, err)
	}

	// Pre-seed the decode target with default strings so keys absent from the file
	// retain their defaults; only keys present in the document get overwritten.
	shape := fileShape{
		Release: releaseShape{
			TagPrefix:      defaultTagPrefix,
			CommitPrefix:   defaultCommitPrefix,
			ReleaseBranch:  "",
			OnNotesFailure: defaultOnNotesFailure,
		},
	}
	if err := toml.Unmarshal(data, &shape); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", configFileName, err)
	}

	return Config{
		Release:      resolveRelease(shape.Release),
		AICommand:    resolveAICommand(shape.AICommand),
		MaxDiffLines: resolveMaxDiffLines(shape.MaxDiffLines),
		DiffExclude:  shape.DiffExclude,
	}, nil
}

// resolveAICommand applies the "claude -p" default when the key was absent (nil) and
// otherwise honours the explicit value verbatim (mirroring the max_diff_lines *int
// handling).
func resolveAICommand(v *string) string {
	if v != nil {
		return *v
	}
	return defaultAICommand
}

// resolveMaxDiffLines applies the 50000 default when the key was absent (nil) and
// otherwise honours the explicit value (mirroring the publish *bool handling).
func resolveMaxDiffLines(v *int) int {
	if v != nil {
		return *v
	}
	return defaultMaxDiffLines
}

// resolveRelease applies the publish and changelog defaults when those keys were
// absent (nil) and copies the already-defaulted string fields through.
func resolveRelease(shape releaseShape) Release {
	return Release{
		TagPrefix:      shape.TagPrefix,
		CommitPrefix:   shape.CommitPrefix,
		ReleaseBranch:  shape.ReleaseBranch,
		Publish:        boolOrDefault(shape.Publish, defaultPublish),
		Changelog:      boolOrDefault(shape.Changelog, defaultChangelog),
		Provider:       shape.Provider,
		Context:        shape.Context,
		Prompt:         shape.Prompt,
		OnNotesFailure: shape.OnNotesFailure,
		Fallback:       shape.Fallback,
		VersionFile:    shape.VersionFile,
		VersionPattern: shape.VersionPattern,
		Hooks: Hooks{
			Preflight:   shape.Hooks.Preflight,
			PreTag:      shape.Hooks.PreTag,
			PostRelease: shape.Hooks.PostRelease,
		},
	}
}

// boolOrDefault applies def when the key was absent (nil) and otherwise honours
// the explicit value — the absent-vs-explicit-false idiom shared by the publish
// and changelog toggles.
func boolOrDefault(v *bool, def bool) bool {
	if v != nil {
		return *v
	}
	return def
}
