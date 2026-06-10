// Package config loads mint's optional .mint.toml from the repo root. Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// This loads the keys the release pipeline needs so far: the four Phase 1
// [release] keys (tag_prefix, commit_prefix, release_branch, publish), the
// top-level max_diff_lines guard, and the Phase 2 notes-engine prompt-control
// keys ([release].context, [release].prompt). The full schema (shared engine
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
// (empty = auto-derive), and Publish decides whether to publish a GitHub release
// or stop at tag + push.
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
type Release struct {
	TagPrefix      string
	CommitPrefix   string
	ReleaseBranch  string
	Publish        bool
	Context        string
	Prompt         string
	OnNotesFailure string
	Fallback       string
}

// defaults returns a Config seeded with the Phase 1 default values.
func defaults() Config {
	return Config{
		Release: Release{
			TagPrefix:      defaultTagPrefix,
			CommitPrefix:   defaultCommitPrefix,
			ReleaseBranch:  "",
			Publish:        defaultPublish,
			Context:        "",
			Prompt:         "",
			OnNotesFailure: defaultOnNotesFailure,
			Fallback:       "",
		},
		MaxDiffLines: defaultMaxDiffLines,
	}
}

// fileShape mirrors the on-disk TOML so absent keys can be told apart from
// present-but-zero ones. Publish is a *bool because its zero value (false) is a
// meaningful, explicit choice: nil means "key absent, apply default true" while
// a non-nil false means "publish disabled". MaxDiffLines is a *int for the same
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
	TagPrefix      string `toml:"tag_prefix"`
	CommitPrefix   string `toml:"commit_prefix"`
	ReleaseBranch  string `toml:"release_branch"`
	Publish        *bool  `toml:"publish"`
	Context        string `toml:"context"`
	Prompt         string `toml:"prompt"`
	OnNotesFailure string `toml:"on_notes_failure"`
	Fallback       string `toml:"fallback"`
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

// resolveRelease applies the publish default when the key was absent (nil) and
// copies the already-defaulted string fields through.
func resolveRelease(shape releaseShape) Release {
	publish := defaultPublish
	if shape.Publish != nil {
		publish = *shape.Publish
	}

	return Release{
		TagPrefix:      shape.TagPrefix,
		CommitPrefix:   shape.CommitPrefix,
		ReleaseBranch:  shape.ReleaseBranch,
		Publish:        publish,
		Context:        shape.Context,
		Prompt:         shape.Prompt,
		OnNotesFailure: shape.OnNotesFailure,
		Fallback:       shape.Fallback,
	}
}
