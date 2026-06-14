// Package config loads mint's optional .mint.toml from the repo root. Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// Config is the single CANONICAL schema for the verb-namespaced .mint.toml: the
// shared engine keys at the top level (ai_command, diff_exclude, max_diff_lines,
// timeout), the [release] table, and the nested [release.hooks] sub-table. Every documented
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
	"time"

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

// DefaultAICommand is the out-of-the-box notes transport command:
// `claude -p --model sonnet`, the AI invocation mint pipes the composed prompt into.
// The model is PINNED so zero-config behaviour is predictable — a bare `claude -p`
// would inherit whatever default model the operator's Claude CLI is set to, an
// external mutable setting mint does not control, so quality, cost, and latency would
// vary silently. The pin uses the `sonnet` ALIAS, not a full versioned model ID: the
// alias tracks the current Sonnet version automatically, whereas a baked-in versioned
// ID goes stale every model release and would force a rebuild just to track versions.
//
// This is the single CANONICAL source of the pinned default value — it is EXPORTED so
// every other site derives the value from here rather than re-typing the literal (the
// transport's duplicate self-default and initgen's scaffold literal are removed/sourced
// from this constant in later phases).
//
// It is a shared engine key (every verb's notes engine uses it), so it lives at the
// top level of Config (see Config.AICommand). config carries whatever the file holds
// verbatim, applying this default only when the key is absent.
const DefaultAICommand = "claude -p --model sonnet"

// DefaultTimeout is the shipped per-attempt AI deadline (60s) — the value the transport
// retries every attempt against. It is the CANONICAL source of that default, mirroring
// the transport's defaultTimeout literal (which Phase 2 deletes in favour of this), and
// is EXPORTED so the init template / README and any direct reader derive the value from
// here rather than re-typing it.
//
// timeout is a shared engine key (every verb's notes transport uses it), so it lives at
// the top level of Config (see Config.Timeout). config seeds this default in defaults()
// for the zero-config path; an absent key in a PRESENT file leaves Config.Timeout nil so
// Task 1-7's accessor applies the floor, while an explicit value (including 0) is carried
// verbatim. The TOML key is integer SECONDS, converted to a time.Duration at the boundary.
const DefaultTimeout = 60 * time.Second

// Config is the loaded mint configuration. The [release] table plus the
// shared top-level engine keys read so far are populated; the remaining
// engine-level keys and other verb tables arrive in later phases.
type Config struct {
	Release Release

	// Commit holds the [commit] table values: the commit verb's two prompt-control
	// knobs (Context and Prompt), both optional and defaulting to empty, plus the
	// per-verb engine overrides ai_command (AICommand) and timeout (Timeout).
	// Context/Prompt are the commit-specific counterparts of [release]'s
	// Context/Prompt. The ai_command / timeout overrides resolve through the layered
	// chain [commit] → shared top-level → shipped default (mirroring [release]); the
	// other shared engine keys diff_exclude / max_diff_lines stay shared-only and
	// serve commit from the top level (these two keys have no [commit] override).
	Commit Commit

	// AICommand is the shared engine-level ai_command notes-transport command (the
	// pinned default "claude -p --model sonnet", the single canonical source of which
	// is config.DefaultAICommand). It is top-level — NOT under [release] — because
	// every verb's notes engine uses the same AI transport. config carries it verbatim:
	// an ABSENT key is seeded with the pinned default, but an explicit empty/blank value
	// is carried through unchanged (NOT re-defaulted at Load) so AICommandFor's trim-and-
	// skip is what falls it through to the floor. Blank-skipping and the verb → shared →
	// floor chain are AICommandFor's job (the single place blank-detection lives); the
	// transport just whitespace-splits the resolved command into name + args (it is
	// operator-controlled config, not arbitrary input).
	AICommand string

	// MaxDiffLines is the shared engine-level max_diff_lines guard ceiling (default
	// 50000). It is top-level — NOT under [release] — because it serves every verb's
	// notes engine, not just release. The notes size guard compares the
	// post-exclusion diff's line count against it.
	MaxDiffLines int

	// Timeout is the shared engine-level timeout per-attempt AI deadline, carried as a
	// *time.Duration so the absent-vs-explicit-zero distinction survives to Task 1-7's
	// TimeoutFor accessor: NIL means absent (the accessor applies the shipped 60s floor),
	// while a NON-NIL value is the operator's explicit choice carried verbatim — including
	// an explicit 0 ("no deadline", honoured) and any negative (dropped to the floor in
	// 1-7). It is top-level — NOT under [release] — because every verb's notes transport
	// uses the same deadline. The TOML key is integer SECONDS (e.g. `timeout = 90`),
	// converted to a duration at the config boundary. defaults() seeds DefaultTimeout
	// directly for the zero-config path so a direct reader sees 60s, not a bare nil.
	Timeout *time.Duration

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
//
// AICommand is the OPTIONAL per-verb ai_command override (raw [release].ai_command). It
// is a *string so absent (nil) is distinguishable from an explicit empty/blank value —
// the resolver (1-4) needs that distinction for its blank-skip fall-through. config
// carries the pointer's value verbatim (nil when absent; the literal string, blank or
// not, when present); the override chain (verb → shared top-level → shipped default) and
// blank-skipping are the resolver's job, NOT Load's.
//
// Timeout is the OPTIONAL per-verb timeout override (raw [release].timeout, integer
// SECONDS in TOML), carried as a *time.Duration so the per-verb absent-vs-explicit-zero
// distinction survives to Task 1-7's TimeoutFor accessor: NIL means absent (no override —
// fall through to the shared top-level / 60s floor), while a NON-NIL value is the
// operator's explicit per-verb choice carried verbatim — including an explicit 0 ("no
// deadline", honoured by the accessor, stops fall-through) and any NEGATIVE (carried RAW
// here; the negative-drop to the floor is 1-7's job, NOT Load's). config seeds NO per-verb
// timeout default — the absent baseline is "no override" (nil). The seconds → duration
// conversion happens at the config boundary (resolveTimeout).
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
	AICommand      *string
	Timeout        *time.Duration
	Hooks          Hooks
}

// Commit holds the [commit] table values: the commit verb's two prompt-control
// knobs (Context and Prompt) plus optional per-verb ai_command and timeout overrides
// (AICommand, Timeout). Context and Prompt are carried as raw TOML strings (both default
// empty): Context injects project guidance into the default commit prompt
// (empty = no injection); Prompt is a file path that fully overrides the default
// commit prompt (empty = use the default). config carries both verbatim — the
// configured Prompt file is read by ResolveCommitPrompt at the point of use
// (assembly, 1-2), NOT here, and a configured-but-unreadable/missing override
// fails loud there rather than silently falling through to the default. AICommand
// is a *string and Timeout a *time.Duration (both absent → nil), NOT raw values
// defaulting to a zero — see their per-field comments below.
type Commit struct {
	Context string
	Prompt  string

	// AICommand is the OPTIONAL per-verb ai_command override (raw [commit].ai_command),
	// a *string so absent (nil) is distinguishable from an explicit empty/blank value
	// (the resolver in 1-4 needs that distinction for blank-skip fall-through). config
	// carries the value verbatim; the override chain and blank-skipping are the
	// resolver's job. The external commit-command spec-document revision is handled by
	// a separate commit-spec pass.
	AICommand *string

	// Timeout is the OPTIONAL per-verb timeout override (raw [commit].timeout, integer
	// SECONDS in TOML), the commit-verb mirror of Release.Timeout. It is a *time.Duration
	// so absent (nil) is distinguishable from an explicit 0: NIL means no override (fall
	// through to the shared / 60s floor), while a NON-NIL value is the operator's explicit
	// per-verb choice carried verbatim — an explicit 0 ("no deadline") or a NEGATIVE
	// carried RAW (the negative-drop is Task 1-7's accessor job, not Load's). config seeds
	// NO per-verb timeout default; the seconds → duration conversion happens at the config
	// boundary (resolveTimeout). This per-verb override is the commit-verb counterpart
	// of AICommand.
	Timeout *time.Duration
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
	// Timeout is seeded with a pointer to the shipped 60s default so the ZERO-CONFIG path
	// (no .mint.toml) yields the documented deadline to any direct reader, mirroring the
	// other top-level keys. A present-file decode does NOT go through here — Load builds a
	// fresh Config literal where an absent key leaves Timeout nil (floor-applied in 1-7)
	// and an explicit value wins — so this seed cannot mask a present-file absent-vs-zero.
	timeout := DefaultTimeout
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
		AICommand:    DefaultAICommand,
		MaxDiffLines: defaultMaxDiffLines,
		Timeout:      &timeout,
	}
}

// fileShape mirrors the on-disk TOML so absent keys can be told apart from
// present-but-zero ones. Publish and Changelog are *bool because their zero value
// (false) is a meaningful, explicit choice: nil means "key absent, apply default
// true" while a non-nil false means the surface is disabled. MaxDiffLines is a *int for the same
// reason — nil means "key absent, apply default 50000" while a non-nil value
// (even 0) is an explicit operator choice. Timeout is a *int (integer SECONDS) for the
// SAME reason — nil means "key absent" (the accessor in 1-7 applies the 60s floor) while
// a non-nil value (even 0) is an explicit operator choice; integer seconds makes a
// non-integer TOML value a strict decode error at Load, exactly like max_diff_lines. The
// string fields are decoded onto a struct pre-seeded with defaults, so the decoder only
// overwrites keys actually present in the file — an explicit empty tag_prefix overwrites
// "v" with "" (a valid prefix-less choice) while an absent key leaves the default intact.
type fileShape struct {
	Release      releaseShape `toml:"release"`
	Commit       commitShape  `toml:"commit"`
	AICommand    *string      `toml:"ai_command"`
	MaxDiffLines *int         `toml:"max_diff_lines"`
	Timeout      *int         `toml:"timeout"`
	DiffExclude  []string     `toml:"diff_exclude"`
}

// commitShape mirrors the on-disk [commit] table. Context and Prompt are plain strings
// whose zero value (empty) IS the documented absent default — empty context means no
// injection, empty prompt means the default prompt — so no *string absent-vs-zero
// distinction is needed (mirroring releaseShape.Context / releaseShape.Prompt).
// AICommand is the optional per-verb ai_command override, a *string so absent (nil) is
// distinguishable from an explicit empty/blank value — the resolver (1-4) needs that
// distinction for its blank-skip fall-through. config carries it verbatim; blank-skipping
// is the accessor's job, not Load's. Timeout is the optional per-verb timeout override,
// a *int (integer SECONDS) so absent (nil) is distinguishable from an explicit 0 — the
// accessor (1-7) needs that distinction to honour 0 ("no deadline") vs falling through;
// integer seconds makes a non-integer TOML value a strict decode error at Load.
type commitShape struct {
	Context   string  `toml:"context"`
	Prompt    string  `toml:"prompt"`
	AICommand *string `toml:"ai_command"`
	Timeout   *int    `toml:"timeout"`
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
	AICommand      *string    `toml:"ai_command"`
	Timeout        *int       `toml:"timeout"`
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
		Commit:       resolveCommit(shape.Commit),
		AICommand:    aiCommandOrDefault(shape.AICommand),
		MaxDiffLines: resolveMaxDiffLines(shape.MaxDiffLines),
		Timeout:      resolveTimeout(shape.Timeout),
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
	"fileShape.Timeout":      "timeout must be an integer (seconds)",
	"fileShape.DiffExclude":  "diff_exclude must be an array of strings",
	"releaseShape.Publish":   "publish must be a boolean",
	"releaseShape.Changelog": "changelog must be a boolean",
	"releaseShape.AICommand": "release.ai_command must be a string",
	"releaseShape.Timeout":   "release.timeout must be an integer (seconds)",
	"commitShape.Context":    "commit.context must be a string",
	"commitShape.Prompt":     "commit.prompt must be a string",
	"commitShape.AICommand":  "commit.ai_command must be a string",
	"commitShape.Timeout":    "commit.timeout must be an integer (seconds)",
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

// AICommandFor resolves the AI notes-transport command for verb through the layered
// chain `[verb].ai_command → top-level shared ai_command → shipped DefaultAICommand`.
// It is the SINGLE place blank-skipping lives: each candidate is trimmed and SKIPPED
// when its trimmed form is empty, so a blank/whitespace per-verb override falls to the
// shared value, a blank/whitespace shared value falls to the floor, and the floor
// (always non-empty) guarantees the result is never empty. The transport's old
// single-layer empty→re-default is therefore unreachable — config supplies a valid
// command at every layer.
//
// The trim is used ONLY for the empty-check: the RAW (untrimmed) candidate is returned
// so the operator's exact command survives verbatim (the transport whitespace-splits
// it into name + args; collapsing internal spacing here would mutate the intended argv).
//
// Resolution is per-key independent: this reads only the ai_command candidates and
// never consults timeout. verb is the closed two-value enum, so the per-verb candidate
// is selected exhaustively — VerbCommit reads [commit]'s override, every other (i.e.
// VerbRelease, the zero value) reads [release]'s.
func (c Config) AICommandFor(verb Verb) string {
	override := c.Release.AICommand
	if verb == VerbCommit {
		override = c.Commit.AICommand
	}

	// The override → shared chain, each tried in order; the floor is appended last so a
	// single trim-and-skip loop covers every layer and the non-empty floor makes the
	// result total (it always satisfies the empty-check).
	candidates := []string{c.AICommand, DefaultAICommand}
	if override != nil {
		candidates = append([]string{*override}, candidates...)
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	// Unreachable in practice — DefaultAICommand is a non-empty constant and is always
	// the last candidate — but a total return keeps the method honest without relying on
	// the loop's structure.
	return DefaultAICommand
}

// TimeoutFor resolves the per-attempt AI deadline for verb through the layered chain
// `[verb].timeout → top-level shared timeout → shipped DefaultTimeout (60s)`. It is the
// SINGLE place the timeout value semantics live, mirroring AICommandFor's structure but
// applying timeout's OWN per-candidate rules (which differ from ai_command's blank-skip):
//
//   - ABSENT (nil candidate) → SKIP to the next candidate;
//   - explicit ZERO → HONOUR it as "no deadline": return a pointer to 0 and STOP (it is
//     NOT treated as missing — a present shared/floor value below is NOT consulted);
//   - NEGATIVE (value-invalid) → DROP through to the next candidate (NOT honoured, NOT
//     collapsed into the zero/no-deadline branch);
//   - POSITIVE → use it as-is.
//
// The 60s floor is the last candidate and is always present, so the result is TOTAL —
// this accessor never returns nil. A negative per-verb override with no shared value
// therefore resolves to the 60s floor (negative dropped, floor applied), never to a
// negative and never to "no deadline".
//
// Boundary contract for Phase 2: the return type is *time.Duration precisely so the
// transport (internal/ai) can distinguish the operator's explicit 0 ("no deadline") from
// a positive/floor deadline — a pointer to 0 means "no deadline", a pointer to a positive
// value means a real per-attempt deadline. Phase 2's ai.Config.Timeout is also
// *time.Duration and takes this accessor's return by DIRECT ASSIGNMENT (no conversion), so
// "no deadline" stays reachable ONLY via an operator's explicit 0 — never by a wiring site
// omitting the field. The transport must therefore SKIP context.WithTimeout when the
// resolved value is the explicit-0/no-deadline case (a zero duration passed to
// WithTimeout fires immediately) — but that conditional lives in Phase 2, NOT here.
//
// Resolution is per-key independent: this reads only the timeout candidates and never
// consults ai_command. verb is the closed two-value enum, so the per-verb candidate is
// selected exhaustively — VerbCommit reads [commit]'s override, every other (i.e.
// VerbRelease, the zero value) reads [release]'s.
func (c Config) TimeoutFor(verb Verb) *time.Duration {
	override := c.Release.Timeout
	if verb == VerbCommit {
		override = c.Commit.Timeout
	}

	// The floor is appended last so a single value-semantics loop covers every layer and
	// the always-present floor makes the result total. The floor is a local copy because
	// DefaultTimeout is a const (its address cannot be taken).
	floor := DefaultTimeout
	candidates := []*time.Duration{override, c.Timeout, &floor}

	for _, candidate := range candidates {
		if candidate == nil {
			continue // absent → next layer
		}
		switch {
		case *candidate == 0:
			return candidate // explicit zero honoured ("no deadline") — STOP the chain
		case *candidate < 0:
			continue // negative is value-invalid → drop to the next layer
		default:
			return candidate // positive used as-is
		}
	}
	// Unreachable in practice — the floor is a non-nil, positive last candidate — but a
	// total return keeps the method honest without relying on the loop's structure.
	return &floor
}

// aiCommandOrDefault applies the pinned DefaultAICommand when the top-level key was
// absent (nil) and otherwise carries the explicit value verbatim — including an
// explicit empty, which Load must NOT re-default so AICommandFor's trim-and-skip is
// what falls it through to the floor. Blank/whitespace detection does NOT live here:
// it is the AICommandFor accessor's sole job (this is absent→default only, mirroring
// the max_diff_lines *int handling).
func aiCommandOrDefault(v *string) string {
	if v != nil {
		return *v
	}
	return DefaultAICommand
}

// resolveMaxDiffLines applies the 50000 default when the key was absent (nil) and
// otherwise honours the explicit value (mirroring the publish *bool handling).
func resolveMaxDiffLines(v *int) int {
	if v != nil {
		return *v
	}
	return defaultMaxDiffLines
}

// resolveTimeout converts the decoded *int integer seconds into the *time.Duration
// carried on Config, preserving the absent-vs-explicit distinction the accessor needs.
// An ABSENT key (nil) stays nil — the present-file no-timeout case Task 1-7's TimeoutFor
// resolves to the 60s floor; this is DELIBERATELY not pre-defaulted here (the zero-config
// 60s comes from defaults(), so the seed never masks a present-file absent). A PRESENT
// value (including an explicit 0, "no deadline", or a negative the 1-7 accessor drops) is
// carried verbatim as seconds → duration, so absent (nil) is distinguishable from an
// explicit 0 (a non-nil pointer to 0). Mirrors the max_diff_lines *int nil-vs-present idiom,
// adding only the seconds→duration boundary conversion.
func resolveTimeout(seconds *int) *time.Duration {
	if seconds == nil {
		return nil
	}
	d := time.Duration(*seconds) * time.Second
	return &d
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
		AICommand:      shape.AICommand,
		Timeout:        resolveTimeout(shape.Timeout),
		Hooks: Hooks{
			Preflight:   shape.Hooks.Preflight,
			PreTag:      shape.Hooks.PreTag,
			PostRelease: shape.Hooks.PostRelease,
		},
	}
}

// resolveCommit copies the [commit] table's raw knobs through. Context and Prompt default
// to empty (the absent baseline: no context injection / default prompt), and an explicit
// empty value carries the same meaning, so there is nothing to re-default — config carries
// the raw values and ResolveCommitPrompt reads the Prompt file at the point of use.
// AICommand is the optional per-verb override, carried as the raw *string verbatim (nil
// when absent; the literal value, blank or not, when present) for the resolver in 1-4 —
// blank-skipping is the accessor's job, not here. Timeout is the optional per-verb timeout
// override: commitShape carries it as a *int (integer seconds) but Commit carries a
// *time.Duration, so resolveTimeout performs the seconds → duration boundary conversion
// (nil stays nil; an explicit value, including 0 or a negative, is carried verbatim for
// 1-7 to interpret). Because those field TYPES now differ, this is an explicit
// field-by-field copy — the old direct Commit(shape) conversion no longer compiles (it
// relied on commitShape and Commit being field-identical, which the timeout type boundary
// breaks), mirroring resolveRelease.
func resolveCommit(shape commitShape) Commit {
	return Commit{
		Context:   shape.Context,
		Prompt:    shape.Prompt,
		AICommand: shape.AICommand,
		Timeout:   resolveTimeout(shape.Timeout),
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

// ResolveCommitPrompt resolves cfg.Commit.Prompt into the full prompt-override text
// commit assembly (1-2) consumes, reading the configured file relative to root. It is
// the point-of-use read deferred from Load: config carries the raw path verbatim, and
// the file is only read when a prompt is actually needed.
//
//   - Prompt unset (empty) → no override: returns "" with no error, so assembly uses
//     the default commit prompt.
//   - Prompt set → it is a FILE PATH whose contents fully OVERRIDE the default commit
//     prompt. A missing or unreadable file is an error naming the path, NEVER a silent
//     fall-back to the default — a configured prompt is an explicit operator choice.
func ResolveCommitPrompt(cfg Config, root string) (string, error) {
	if cfg.Commit.Prompt == "" {
		return "", nil
	}

	path := cfg.Commit.Prompt
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("reading commit prompt override %q: %w", path, err)
	}
	return string(data), nil
}
