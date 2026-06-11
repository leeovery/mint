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
// Load decodes the file STRICTLY: any key matching no field in the canonical schema
// — at the top level, inside [release], or inside [release.hooks] — is rejected with
// an actionable message naming the key and its table (a top-level [hooks] table gets
// targeted guidance to nest under [release.hooks]). It also fails loud on bad TYPES:
// a value that cannot decode into its schema Go type (e.g. a string max_diff_lines, a
// string publish/changelog, a scalar diff_exclude) is re-wrapped into a mint message
// naming the key and its expected type; a [release.hooks] entry that is neither a
// command string nor an array of command strings is rejected naming the hook key; and
// on_notes_failure is constrained to the closed set abort | fallback. The
// provider-VALUE carve-out (unsupported-but-recognised provider names) is a SEPARATE
// Phase 6 task and is NOT done here.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
// is "fallback". These two are the closed valid set: an absent key defaults to abort
// and any other non-empty value is rejected by Load (see validateOnNotesFailure).
const defaultOnNotesFailure = "abort"

// onNotesFailureValues is the closed set of accepted on_notes_failure values. abort
// (the default) fails loud; fallback opts into the commit-subject / fixed-string
// fallback body. Any other non-empty value fails loud at load.
var onNotesFailureValues = []string{"abort", "fallback"}

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
// OnNotesFailure is the normal-path notes-failure policy (default "abort"). Load
// constrains it to the closed set "abort" | "fallback" (an absent key defaults to
// abort); the notes engine's ResolveFailure interprets the validated value as
// MODE-ONLY ("abort" → abort; "fallback" → commit-subject / fixed-string fallback).
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
// a string and a TOML array as a HookValue carrying a slice ([]any of strings), both
// verbatim, while a nil HookValue means the key was absent (no hook, a no-op). config
// does NOT normalise the value — the hooks package coerces the carried shape to
// ordered command strings when it runs the hook — but Load DOES validate the shape:
// anything else (integer, boolean, table, or an array carrying a non-string element)
// fails loud naming the hook key.
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
	// Strict decoding so unknown keys SURFACE rather than being silently ignored,
	// across all three levels of the canonical schema (top level, [release],
	// [release.hooks]). DisallowUnknownFields makes the decoder collect every key
	// matching no struct field into a *toml.StrictMissingError, which translateStrict
	// turns into mint's actionable message.
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&shape); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return Config{}, translateStrict(strict)
		}
		var decErr *toml.DecodeError
		if errors.As(err, &decErr) {
			return Config{}, translateTypeError(decErr)
		}
		return Config{}, fmt.Errorf("parsing %s: %w", configFileName, err)
	}

	if err := validateHooks(shape.Release.Hooks); err != nil {
		return Config{}, err
	}
	if err := validateOnNotesFailure(shape.Release.OnNotesFailure); err != nil {
		return Config{}, err
	}

	return Config{
		Release:      resolveRelease(shape.Release),
		AICommand:    resolveAICommand(shape.AICommand),
		MaxDiffLines: resolveMaxDiffLines(shape.MaxDiffLines),
		DiffExclude:  shape.DiffExclude,
	}, nil
}

// translateStrict turns the decoder's *toml.StrictMissingError (one entry per
// unknown key, each carrying its full dotted key path) into mint's actionable
// message. It reports the first offending key — naming both the key and its owning
// table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks].
func translateStrict(strict *toml.StrictMissingError) error {
	for _, de := range strict.Errors {
		key := de.Key()
		if len(key) == 0 {
			continue
		}
		return fmt.Errorf("invalid %s: %s", configFileName, unknownKeyMessage(key))
	}
	// Defensive: a StrictMissingError with no usable key path. Fall back to the
	// library's own multi-line description so nothing is silently swallowed.
	return fmt.Errorf("invalid %s: %s", configFileName, strict.String())
}

// typeErrorMessages maps a schema struct-field path (as it appears in the decoder's
// *toml.DecodeError text, e.g. "fileShape.MaxDiffLines") to the actionable mint
// message naming the TOML key and its expected type. The decoder reports a type
// mismatch by Go field path with a nil Key(), so this lookup re-attaches the
// user-facing key + type. Each entry covers one schema field whose TOML type is
// constrained enough to misuse (the bool toggles, the integer guard, the string
// array).
var typeErrorMessages = map[string]string{
	"fileShape.MaxDiffLines": "max_diff_lines must be an integer",
	"fileShape.DiffExclude":  "diff_exclude must be an array of strings",
	"releaseShape.Publish":   "publish must be a boolean",
	"releaseShape.Changelog": "changelog must be a boolean",
}

// translateTypeError turns the decoder's *toml.DecodeError (a value that cannot
// unmarshal into its schema Go type) into mint's actionable message naming the key
// and expected type. The DecodeError carries no usable Key() for type mismatches, so
// it identifies the offending field by the struct-field path embedded in the error
// text and maps it via typeErrorMessages; a field not in the map (no constrained
// type to misuse) falls back to the library's positioned description so nothing is
// silently swallowed. The map iteration order is non-deterministic but safe: a single
// DecodeError names exactly one offending field, so at most one entry can match.
func translateTypeError(decErr *toml.DecodeError) error {
	text := decErr.Error()
	for field, msg := range typeErrorMessages {
		if strings.Contains(text, field+" ") {
			return fmt.Errorf("invalid %s: %s", configFileName, msg)
		}
	}
	return fmt.Errorf("invalid %s: %s", configFileName, decErr.String())
}

// unknownKeyMessage builds the human-readable description for one unknown key,
// given its full dotted path (e.g. ["release", "foo"]). A top-level [hooks] table
// gets the targeted nest-guidance variant; every other key names the leaf key and
// its owning table.
func unknownKeyMessage(key []string) string {
	// A top-level [hooks] table contradicts the verb-namespaced shape — hooks must
	// nest under [release.hooks]. Give that specific guidance instead of the generic
	// unknown-key message.
	if key[0] == "hooks" {
		return "[hooks] is not valid at the top level — nest hooks under [release.hooks]"
	}

	leaf := key[len(key)-1]
	if len(key) == 1 {
		return fmt.Sprintf("unknown top-level key %q", leaf)
	}

	table := "[" + strings.Join(key[:len(key)-1], ".") + "]"
	return fmt.Sprintf("unknown key %q in %s", leaf, table)
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

// validateHooks fails loud if any [release.hooks] entry is neither a command string
// nor an array of command strings. The HookValue underlying type is the empty
// interface, so the decoder surfaces any TOML value verbatim — config rejects the
// shapes the hooks runner cannot consume (integer, boolean, table, or an array
// carrying a non-string element) here, naming the offending hook key.
func validateHooks(hooks hooksShape) error {
	entries := []struct {
		key   string
		value HookValue
	}{
		{"preflight", hooks.Preflight},
		{"pre_tag", hooks.PreTag},
		{"post_release", hooks.PostRelease},
	}
	for _, e := range entries {
		if !isHookShape(e.value) {
			return fmt.Errorf(
				"invalid %s: release.hooks.%s must be a string or an array of strings",
				configFileName, e.key,
			)
		}
	}
	return nil
}

// isHookShape reports whether v is a valid hook value: absent (nil), a single command
// string, or an array of command strings (the decoder surfaces a TOML array into an
// interface field as []any). An array is valid only if every element is a string.
func isHookShape(v HookValue) bool {
	switch val := v.(type) {
	case nil, string:
		return true
	case []any:
		for _, item := range val {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// validateOnNotesFailure fails loud if on_notes_failure is a non-empty value outside
// the closed set abort | fallback. An empty value is the absent/default case (resolved
// to abort) and is accepted; this is a closed-set check on an already-correctly-typed
// string, so the message lists the valid values.
func validateOnNotesFailure(value string) error {
	if value == "" {
		return nil
	}
	for _, valid := range onNotesFailureValues {
		if value == valid {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid %s: on_notes_failure must be one of: %s",
		configFileName, strings.Join(onNotesFailureValues, ", "),
	)
}
