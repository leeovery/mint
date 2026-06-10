// Package config loads mint's optional .mint.toml from the repo root. Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// This loads the keys the release pipeline needs so far: the Phase 1 [release]
// keys (tag_prefix, commit_prefix, release_branch, publish), the changelog
// toggle ([release].changelog), the top-level max_diff_lines guard, and the
// Phase 2 notes-engine prompt-control keys ([release].context, [release].prompt).
// The full schema (shared engine
// keys, the rest of [release], [release.hooks]) and typed fail-loud validation
// are consolidated in Phase 6; until then unknown keys are tolerated and ignored
// rather than rejected.
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

// Config is the loaded mint configuration. The [release] table plus the
// shared top-level engine keys read so far are populated; the remaining
// engine-level keys and other verb tables arrive in later phases.
type Config struct {
	Release Release

	// MaxDiffLines is the shared engine-level max_diff_lines guard ceiling (default
	// 50000). It is top-level — NOT under [release] — because it serves every verb's
	// notes engine, not just release. The notes size guard compares the
	// post-exclusion diff's line count against it.
	MaxDiffLines int
}

// Release holds the [release] table values needed so far: TagPrefix and
// CommitPrefix feed tag/commit subjects, ReleaseBranch gates the on-branch check
// (empty = auto-derive), Publish decides whether to publish a GitHub release
// or stop at tag + push, and Changelog decides whether the CHANGELOG.md
// projection is written (default true) or skipped — the annotated tag still
// carries the full body either way.
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
	Context        string
	Prompt         string
	OnNotesFailure string
	Fallback       string
	VersionFile    string
	VersionPattern string
	Hooks          Hooks
}

// Hooks carries the RAW parsed [release.hooks] values keyed by lifecycle point.
// Each is typed `any` because a TOML decoder surfaces a hook entry as a string
// (one command) or a slice (an ordered list); the hooks package normalises that
// shape when it runs the hook. A nil field means the key was absent — no hook,
// a no-op. config does NOT interpret the values; it carries them verbatim for
// the hook runner.
type Hooks struct {
	Preflight   any
	PreTag      any
	PostRelease any
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
			Context:        "",
			Prompt:         "",
			OnNotesFailure: defaultOnNotesFailure,
			Fallback:       "",
			VersionFile:    "",
			VersionPattern: "",
		},
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
	MaxDiffLines *int         `toml:"max_diff_lines"`
}

type releaseShape struct {
	TagPrefix      string     `toml:"tag_prefix"`
	CommitPrefix   string     `toml:"commit_prefix"`
	ReleaseBranch  string     `toml:"release_branch"`
	Publish        *bool      `toml:"publish"`
	Changelog      *bool      `toml:"changelog"`
	Context        string     `toml:"context"`
	Prompt         string     `toml:"prompt"`
	OnNotesFailure string     `toml:"on_notes_failure"`
	Fallback       string     `toml:"fallback"`
	VersionFile    string     `toml:"version_file"`
	VersionPattern string     `toml:"version_pattern"`
	Hooks          hooksShape `toml:"hooks"`
}

// hooksShape mirrors the on-disk [release.hooks] sub-table. Each key is typed
// `any` so the decoder surfaces whatever TOML shape the value has (a string or
// an array) verbatim; an absent key leaves the field nil. resolveRelease copies
// these straight onto Release.Hooks.
type hooksShape struct {
	Preflight   any `toml:"preflight"`
	PreTag      any `toml:"pre_tag"`
	PostRelease any `toml:"post_release"`
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
		MaxDiffLines: resolveMaxDiffLines(shape.MaxDiffLines),
	}, nil
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
