// Package config loads mint's optional .mint.toml from the repo root. Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// This is the Phase 1 slice — only the four [release] keys the walking-skeleton
// release pipeline needs (tag_prefix, commit_prefix, release_branch, publish).
// The full schema (shared engine keys, the rest of [release], [release.hooks])
// and typed fail-loud validation are consolidated in Phase 6; until then unknown
// keys are tolerated and ignored rather than rejected.
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

// Config is the loaded mint configuration. Only the [release] table is populated
// in Phase 1; the engine-level keys and other verb tables arrive in later phases.
type Config struct {
	Release Release
}

// Release holds the [release] table values needed by the Phase 1 pipeline:
// TagPrefix and CommitPrefix feed tag/commit subjects, ReleaseBranch gates the
// on-branch check (empty = auto-derive), and Publish decides whether to publish
// a GitHub release or stop at tag + push.
type Release struct {
	TagPrefix     string
	CommitPrefix  string
	ReleaseBranch string
	Publish       bool
}

// defaults returns a Config seeded with the Phase 1 default values.
func defaults() Config {
	return Config{
		Release: Release{
			TagPrefix:     defaultTagPrefix,
			CommitPrefix:  defaultCommitPrefix,
			ReleaseBranch: "",
			Publish:       defaultPublish,
		},
	}
}

// fileShape mirrors the on-disk TOML so absent keys can be told apart from
// present-but-zero ones. Publish is a *bool because its zero value (false) is a
// meaningful, explicit choice: nil means "key absent, apply default true" while
// a non-nil false means "publish disabled". The string fields are decoded onto a
// struct pre-seeded with defaults, so the decoder only overwrites keys actually
// present in the file — an explicit empty tag_prefix overwrites "v" with "" (a
// valid prefix-less choice) while an absent key leaves the default intact.
type fileShape struct {
	Release releaseShape `toml:"release"`
}

type releaseShape struct {
	TagPrefix     string `toml:"tag_prefix"`
	CommitPrefix  string `toml:"commit_prefix"`
	ReleaseBranch string `toml:"release_branch"`
	Publish       *bool  `toml:"publish"`
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
			TagPrefix:     defaultTagPrefix,
			CommitPrefix:  defaultCommitPrefix,
			ReleaseBranch: "",
		},
	}
	if err := toml.Unmarshal(data, &shape); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", configFileName, err)
	}

	return Config{Release: resolveRelease(shape.Release)}, nil
}

// resolveRelease applies the publish default when the key was absent (nil) and
// copies the already-defaulted string fields through.
func resolveRelease(shape releaseShape) Release {
	publish := defaultPublish
	if shape.Publish != nil {
		publish = *shape.Publish
	}

	return Release{
		TagPrefix:     shape.TagPrefix,
		CommitPrefix:  shape.CommitPrefix,
		ReleaseBranch: shape.ReleaseBranch,
		Publish:       publish,
	}
}
