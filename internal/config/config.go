// Package config loads mint's optional .mint.toml from the repo root, and appends globs
// to its [release] lists (AppendReleaseGlobs). Config is
// fully optional: zero config yields sensible defaults everywhere, so Load never
// requires a file to exist.
//
// Config is the single CANONICAL schema for the verb-namespaced .mint.toml: the
// shared engine keys at the top level (ai_command, diff_exclude, max_prompt_bytes,
// timeout), the [release] table, and the nested [release.hooks]
// sub-table. Every documented key has its Go type here and a default applied uniformly — on zero config (file
// absent, empty, or comment-only) every key comes back at its documented default,
// and a file that sets only part of a table leaves the unset keys at their
// defaults individually.
//
// Load decodes the file STRICTLY: any key matching no field in the canonical schema
// — at the top level, inside [release], or inside [release.hooks] — is rejected with
// an actionable message naming the key and its table (a top-level [hooks] table gets
// targeted guidance to nest under [release.hooks], and the removed max_diff_lines and
// [release].fallback name what replaced them). It also fails loud on bad TYPES: a
// value that cannot decode into its
// schema Go type (e.g. a string max_prompt_bytes, a string publish/changelog, a scalar diff_exclude) is re-wrapped into a mint message
// naming the key and its expected type; a [release.hooks] entry that is neither a
// command string nor an array of command strings is rejected naming the hook key;
// max_prompt_bytes must be positive; and on_notes_failure is constrained to the closed
// set abort | fallback.
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

	"mint/internal/cmdline"
)

// FileName is the fixed name mint looks for at the repo root.
const FileName = ".mint.toml"

// Default values for the Phase 1 [release] keys. ReleaseBranch's default is the
// empty string, a sentinel meaning "auto-derive from origin/HEAD" (resolved by the
// release spine); an explicit empty value in the file is indistinguishable from this
// and means the same thing.
const (
	defaultTagPrefix    = "v"
	defaultCommitPrefix = "🌿"
	defaultPublish      = true
	defaultChangelog    = true
)

// defaultOnNotesFailure is the notes-failure policy an absent on_notes_failure key
// resolves to.
const defaultOnNotesFailure = "abort"

// onNotesFailureValues is the closed set of accepted on_notes_failure values. abort
// (the default) fails loud; fallback opts into the one non-AI body, the commit-subject
// list. Any other non-empty value fails loud at load.
var onNotesFailureValues = []string{"abort", "fallback"}

// DefaultMaxPromptBytes is the shipped per-call byte cap on the composed prompt of any
// single AI call (instructions, Change Map, diff and closing reminder together). It is
// deliberately high: too high for the model fails loud, too low fails silent. Exported so
// the SoT default column derives the value from here.
const DefaultMaxPromptBytes = 1_500_000

// DefaultAICommand is the out-of-the-box AI transport command every verb falls through
// to when no ai_command is set. It is claude in lean form: the flags keep Claude Code's
// own system prompt, tool definitions, CLAUDE.md files, skills, plugins, hooks and MCP
// servers out of the call, and stop each one-shot call being saved as a session, so a
// call carries mint's prompt and nothing else. The system prompt names no task because
// the same command serves every AI call mint makes. --bare is not used: it forces
// API-key auth and breaks the subscription login.
//
// The model is PINNED, by the `sonnet` alias, so zero-config behaviour does not inherit
// whatever model the operator's Claude CLI defaults to, and the alias tracks the current
// Sonnet without a rebuild.
//
// The empty --tools argument and the multi-word system prompt survive only because the
// transport splits the command by POSIX shell quoting rules.
//
// This is the single canonical source of the default value: every other site derives it
// from here.
const DefaultAICommand = `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."`

// The shipped per-attempt AI deadlines TimeoutFor falls to when no timeout is set at any
// level. Release-notes prompts run far larger than commit prompts, so they get the longer
// deadline; a hung commit call still fails fast.
const (
	DefaultReleaseTimeout = 180 * time.Second
	DefaultCommitTimeout  = 60 * time.Second
)

// Config is the loaded mint configuration: the [release] and [commit] verb tables
// plus the shared top-level engine keys (ai_command, max_prompt_bytes, timeout,
// diff_exclude).
type Config struct {
	Release Release

	// Commit holds the [commit] table values: the commit verb's two prompt-control
	// knobs (Context and Prompt), both optional and defaulting to empty, plus the
	// per-verb engine overrides ai_command (AICommand) and timeout (Timeout).
	// Context/Prompt are the commit-specific counterparts of [release]'s
	// Context/Prompt. The ai_command / timeout overrides resolve through the layered
	// chain [commit] → shared top-level → shipped default (mirroring [release]);
	// max_prompt_bytes serves commit from the top level, and commit's AI diff excludes the
	// shared diff_exclude list plus Commit.DiffExclude.
	Commit Commit

	// AICommand is the shared engine-level ai_command notes-transport command (its
	// default is config.DefaultAICommand). It is top-level — NOT under [release] — because
	// every verb's notes engine uses the same AI transport. config carries it verbatim:
	// an ABSENT key is seeded with the pinned default, but an explicit empty/blank value
	// is carried through unchanged (NOT re-defaulted at Load) so AICommandFor's trim-and-
	// skip is what falls it through to the floor. Blank-skipping and the verb → shared →
	// floor chain are AICommandFor's job; the
	// transport splits the resolved command into name + args by POSIX shell quoting
	// rules.
	AICommand string

	// MaxPromptBytes is the shared per-call byte cap on the composed prompt of any single
	// AI call (default DefaultMaxPromptBytes), one cap for both verbs. Load guarantees it
	// is positive: zero never means "no cap", unlike timeout's 0.
	MaxPromptBytes int

	// Timeout is the shared engine-level timeout per-attempt AI deadline, carried as a
	// *time.Duration so the absent-vs-explicit-zero distinction survives to the
	// TimeoutFor accessor: NIL means absent (the accessor applies the verb's shipped floor),
	// while a NON-NIL value is the operator's explicit choice carried verbatim — including
	// an explicit 0 ("no deadline", honoured) and any negative (TimeoutFor drops it to the
	// floor). It is top-level — NOT under [release] — because it is the shared baseline
	// both verbs resolve through. The TOML key is integer SECONDS (e.g. `timeout = 90`),
	// converted to a duration at the config boundary. defaults() seeds nothing here: the
	// shipped deadline differs per verb, so only TimeoutFor can apply it.
	Timeout *time.Duration

	// DiffExclude is the shared list of glob pathspecs every verb's AI diff excludes —
	// what neither release notes nor commit messages should read. A per-verb list
	// (Release.DiffExclude, Commit.DiffExclude) adds to it and never replaces it. Each entry is a git
	// pathspec glob (e.g. "skills/**/knowledge.cjs") turned into a :(exclude)<glob>
	// argument; git performs the matching, mint does no Go-side glob matching.
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
// MODE-ONLY ("abort" → abort; "fallback" → the commit-subject list, the one non-AI
// body --no-ai also ships).
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
// AICommandFor needs that distinction for its blank-skip fall-through. config
// carries the pointer's value verbatim (nil when absent; the literal string, blank or
// not, when present); the override chain (verb → shared top-level → shipped default) and
// blank-skipping are AICommandFor's job, NOT Load's.
//
// Timeout is the OPTIONAL per-verb timeout override (raw [release].timeout, integer
// SECONDS in TOML), carried as a *time.Duration so the per-verb absent-vs-explicit-zero
// distinction survives to the TimeoutFor accessor: NIL means absent (no override —
// fall through to the shared top-level / 180s release floor), while a NON-NIL value is the
// operator's explicit per-verb choice carried verbatim — including an explicit 0 ("no
// deadline", honoured by the accessor, stops fall-through) and any NEGATIVE (carried RAW
// here; TimeoutFor drops the negative to the floor, NOT Load). config seeds NO per-verb
// timeout default — the absent baseline is "no override" (nil). The seconds → duration
// conversion happens at the config boundary (resolveTimeout).
//
// DiffExclude is the [release] list of glob pathspecs the release-notes diff excludes on
// top of the shared top-level list — never in place of it. Commit's AI diff never reads it.
//
// DiffKeep is the [release] keep list: glob pathspecs, in diff_exclude's form, whose paths
// the exclusion check never raises. It does nothing else — it adds nothing to the diff and
// never overrides an exclusion, so a path both kept and excluded stays out of the notes.
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
	VersionFile    string
	VersionPattern string
	AICommand      *string
	Timeout        *time.Duration
	DiffExclude    []string
	DiffKeep       []string
	Hooks          Hooks
}

// Commit holds the [commit] table values: the commit verb's two prompt-control
// knobs (Context and Prompt), optional per-verb ai_command and timeout overrides
// (AICommand, Timeout), and the [commit] diff_exclude list (DiffExclude). Context and Prompt are carried as raw TOML strings (both default
// empty): Context injects project guidance into the default commit prompt
// (empty = no injection); Prompt is a file path that fully overrides the default
// commit prompt (empty = use the default). config carries both verbatim — the
// configured Prompt file is read by ResolveCommitPrompt at the point of use,
// NOT here, and a configured-but-unreadable/missing override
// fails loud there rather than silently falling through to the default. AICommand
// is a *string and Timeout a *time.Duration (both absent → nil), NOT raw values
// defaulting to a zero — see their per-field comments below.
type Commit struct {
	Context string
	Prompt  string

	// AICommand is the OPTIONAL per-verb ai_command override (raw [commit].ai_command),
	// a *string so absent (nil) is distinguishable from an explicit empty/blank value
	// (AICommandFor needs that distinction for blank-skip fall-through). config
	// carries the value verbatim; the override chain and blank-skipping are
	// AICommandFor's job. The external commit-command spec-document revision is handled by
	// a separate commit-spec pass.
	AICommand *string

	// Timeout is the OPTIONAL per-verb timeout override (raw [commit].timeout, integer
	// SECONDS in TOML), the commit-verb mirror of Release.Timeout. It is a *time.Duration
	// so absent (nil) is distinguishable from an explicit 0: NIL means no override (fall
	// through to the shared / 60s commit floor), while a NON-NIL value is the operator's explicit
	// per-verb choice carried verbatim — an explicit 0 ("no deadline") or a NEGATIVE
	// carried RAW (TimeoutFor drops the negative, NOT Load). config seeds
	// NO per-verb timeout default; the seconds → duration conversion happens at the config
	// boundary (resolveTimeout). This per-verb override is the commit-verb counterpart
	// of AICommand.
	Timeout *time.Duration

	// DiffExclude is the [commit] list of glob pathspecs commit's AI diff excludes on top
	// of the shared top-level list — never in place of it. Release notes never read it, and
	// commit's preflight ignores it like every other level.
	DiffExclude []string
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
			VersionFile:    "",
			VersionPattern: "",
		},
		AICommand:      DefaultAICommand,
		MaxPromptBytes: DefaultMaxPromptBytes,
	}
}

// fileShape mirrors the on-disk TOML so absent keys can be told apart from
// present-but-zero ones. Publish and Changelog are *bool because their zero value
// (false) is a meaningful, explicit choice: nil means "key absent, apply default
// true" while a non-nil false means the surface is disabled. MaxPromptBytes is a *int for
// the same reason, so an explicit 0 reaches Load's positive-value check rather than
// reading as absent. Timeout is a *int (integer SECONDS) too — nil means "key absent"
// (TimeoutFor applies the verb's shipped floor) while a non-nil value (even 0) is an
// explicit operator choice; integer seconds makes a non-integer TOML value a strict
// decode error at Load, exactly like max_prompt_bytes. The
// string fields are decoded onto a struct pre-seeded with defaults, so the decoder only
// overwrites keys actually present in the file — an explicit empty tag_prefix overwrites
// "v" with "" (a valid prefix-less choice) while an absent key leaves the default intact.
type fileShape struct {
	Release        releaseShape `toml:"release"`
	Commit         commitShape  `toml:"commit"`
	AICommand      *string      `toml:"ai_command"`
	MaxPromptBytes *int         `toml:"max_prompt_bytes"`
	Timeout        *int         `toml:"timeout"`
	DiffExclude    []string     `toml:"diff_exclude"`
}

// commitShape mirrors the on-disk [commit] table. Context and Prompt are plain strings
// whose zero value (empty) IS the documented absent default — empty context means no
// injection, empty prompt means the default prompt — so no *string absent-vs-zero
// distinction is needed (mirroring releaseShape.Context / releaseShape.Prompt).
// AICommand is the optional per-verb ai_command override, a *string so absent (nil) is
// distinguishable from an explicit empty/blank value — AICommandFor needs that
// distinction for its blank-skip fall-through. config carries it verbatim; blank-skipping
// is the accessor's job, not Load's. Timeout is the optional per-verb timeout override,
// a *int (integer SECONDS) so absent (nil) is distinguishable from an explicit 0 —
// TimeoutFor needs that distinction to honour 0 ("no deadline") vs falling through;
// integer seconds makes a non-integer TOML value a strict decode error at Load.
type commitShape struct {
	Context     string   `toml:"context"`
	Prompt      string   `toml:"prompt"`
	AICommand   *string  `toml:"ai_command"`
	Timeout     *int     `toml:"timeout"`
	DiffExclude []string `toml:"diff_exclude"`
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
	VersionFile    string     `toml:"version_file"`
	VersionPattern string     `toml:"version_pattern"`
	AICommand      *string    `toml:"ai_command"`
	Timeout        *int       `toml:"timeout"`
	DiffExclude    []string   `toml:"diff_exclude"`
	DiffKeep       []string   `toml:"diff_keep"`
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
	path := filepath.Join(root, FileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return defaults(), nil
		}
		return Config{}, fmt.Errorf("reading %s: %w", FileName, err)
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
			return Config{}, translateTypeError(decErr, data)
		}
		return Config{}, fmt.Errorf("parsing %s: %w", FileName, err)
	}

	if err := validateHooks(shape.Release.Hooks); err != nil {
		return Config{}, err
	}
	if err := validateOnNotesFailure(shape.Release.OnNotesFailure); err != nil {
		return Config{}, err
	}
	if err := validateAICommands(shape); err != nil {
		return Config{}, err
	}
	if err := validateMaxPromptBytes(shape.MaxPromptBytes); err != nil {
		return Config{}, err
	}

	return Config{
		Release:        resolveRelease(shape.Release),
		Commit:         resolveCommit(shape.Commit),
		AICommand:      aiCommandOrDefault(shape.AICommand),
		MaxPromptBytes: resolveMaxPromptBytes(shape.MaxPromptBytes),
		Timeout:        resolveTimeout(shape.Timeout),
		DiffExclude:    shape.DiffExclude,
	}, nil
}

// translateStrict turns the decoder's *toml.StrictMissingError (one entry per
// unknown key, each carrying its full dotted key path) into mint's actionable
// message. It reports the first offending key — naming both the key and its owning
// table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks], and the removed top-level max_diff_lines with its replacement.
func translateStrict(strict *toml.StrictMissingError) error {
	for _, de := range strict.Errors {
		key := de.Key()
		if len(key) == 0 {
			continue
		}
		return fmt.Errorf("invalid %s: %s", FileName, unknownKeyMessage(key))
	}
	// Defensive: a StrictMissingError with no usable key path. Fall back to the
	// library's own multi-line description so nothing is silently swallowed.
	return fmt.Errorf("invalid %s: %s", FileName, strict.String())
}

// typeErrorMessages maps a schema struct-field path (as it appears in the decoder's
// *toml.DecodeError text, e.g. "fileShape.MaxPromptBytes") to the actionable mint
// message naming the TOML key and its expected type. The decoder reports a type
// mismatch by Go field path with a nil Key(), so this lookup re-attaches the
// user-facing key + type. Each entry covers one schema field whose TOML type is
// constrained enough to misuse (the bool toggles, the integer keys, the string
// array).
var typeErrorMessages = map[string]string{
	"fileShape.MaxPromptBytes": "max_prompt_bytes must be an integer (bytes)",
	"fileShape.Timeout":        "timeout must be an integer (seconds)",
	"fileShape.DiffExclude":    "diff_exclude must be an array of strings",
	"releaseShape.Publish":     "publish must be a boolean",
	"releaseShape.Changelog":   "changelog must be a boolean",
	"releaseShape.AICommand":   "release.ai_command must be a string",
	"releaseShape.Timeout":     "release.timeout must be an integer (seconds)",
	"releaseShape.DiffExclude": "release.diff_exclude must be an array of strings",
	"releaseShape.DiffKeep":    "release.diff_keep must be an array of strings",
	"commitShape.Context":      "commit.context must be a string",
	"commitShape.Prompt":       "commit.prompt must be a string",
	"commitShape.AICommand":    "commit.ai_command must be a string",
	"commitShape.Timeout":      "commit.timeout must be an integer (seconds)",
	"commitShape.DiffExclude":  "commit.diff_exclude must be an array of strings",
}

// topLevelIntegerKeys maps a top-level integer key to its typeErrorMessages field
// path, for the mismatches go-toml reports WITHOUT a struct-field path: a float or
// boolean assigned to an int ("float cannot be assigned to int") names no field, so the
// field-path lookup cannot see it. translateTypeError finds such a key by its value's
// type in a generic decode instead.
var topLevelIntegerKeys = map[string]string{
	"max_prompt_bytes": "fileShape.MaxPromptBytes",
}

// translateTypeError turns the decoder's *toml.DecodeError (a value that cannot
// unmarshal into its schema Go type) into mint's actionable message naming the key
// and expected type. The DecodeError carries no usable Key() for type mismatches, so
// it identifies the offending field by the struct-field path embedded in the error
// text and maps it via typeErrorMessages. When the text names no field (a float or
// boolean into an int), the keys in topLevelIntegerKeys are checked against a generic
// decode of data. Anything still unmatched falls back to the library's positioned
// description so nothing is silently swallowed. The map iteration order is
// non-deterministic but safe: a single DecodeError names exactly one offending field,
// so at most one entry can match.
func translateTypeError(decErr *toml.DecodeError, data []byte) error {
	text := decErr.Error()
	for field, msg := range typeErrorMessages {
		if strings.Contains(text, field+" ") {
			return fmt.Errorf("invalid %s: %s", FileName, msg)
		}
	}
	if field, ok := nonIntegerKey(data); ok {
		return fmt.Errorf("invalid %s: %s", FileName, typeErrorMessages[field])
	}
	return fmt.Errorf("invalid %s: %s", FileName, decErr.String())
}

// nonIntegerKey reports the typeErrorMessages field of a topLevelIntegerKeys key that
// is present in data with a non-integer value. A generic decode carries a TOML integer
// as int64, so any other type is the mismatch.
func nonIntegerKey(data []byte) (string, bool) {
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	for key, field := range topLevelIntegerKeys {
		value, ok := doc[key]
		if !ok {
			continue
		}
		if _, isInt := value.(int64); !isInt {
			return field, true
		}
	}
	return "", false
}

// unknownKeyMessage builds the human-readable description for one unknown key,
// given its full dotted path (e.g. ["release", "foo"]).
func unknownKeyMessage(key []string) string {
	// A top-level [hooks] table contradicts the verb-namespaced shape — hooks must
	// nest under [release.hooks]. Give that specific guidance instead of the generic
	// unknown-key message.
	if key[0] == "hooks" {
		return "[hooks] is not valid at the top level — nest hooks under [release.hooks]"
	}
	if len(key) == 1 && key[0] == "max_diff_lines" {
		return "max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes"
	}
	if len(key) == 2 && key[0] == "release" && key[1] == "fallback" {
		return "release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file"
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
// Each candidate is trimmed and SKIPPED
// when its trimmed form is empty, so a blank/whitespace per-verb override falls to the
// shared value, a blank/whitespace shared value falls to the floor, and the floor
// (always non-empty) guarantees the result is never empty. The transport's old
// single-layer empty→re-default is therefore unreachable — config supplies a valid
// command at every layer.
//
// The trim is used ONLY for the empty-check: the RAW (untrimmed) candidate is returned
// so the operator's exact command survives verbatim (the transport splits it into name +
// args by POSIX shell quoting rules; altering spacing here could change a quoted argument).
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
// `[verb].timeout → top-level shared timeout → the verb's shipped floor`
// (DefaultReleaseTimeout or DefaultCommitTimeout). It is the
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
// The floor is the last candidate and is always present, so the result is TOTAL —
// this accessor never returns nil. A negative per-verb override with no shared value
// therefore resolves to the floor (negative dropped, floor applied), never to a
// negative and never to "no deadline".
//
// Boundary contract with the transport: the return type is *time.Duration precisely so the
// transport (internal/ai) can distinguish the operator's explicit 0 ("no deadline") from
// a positive/floor deadline — a pointer to 0 means "no deadline", a pointer to a positive
// value means a real per-attempt deadline. ai.Config.Timeout is also
// *time.Duration and takes this accessor's return by DIRECT ASSIGNMENT (no conversion), so
// "no deadline" stays reachable ONLY via an operator's explicit 0 — never by a wiring site
// omitting the field. The transport therefore SKIPS context.WithTimeout when the
// resolved value is the explicit-0/no-deadline case (a zero duration passed to
// WithTimeout fires immediately) — that conditional lives in the transport, NOT in this
// accessor, because honouring "no deadline" is a transport-level call wiring concern, not
// part of resolving the value.
//
// Resolution is per-key independent: this reads only the timeout candidates and never
// consults ai_command. verb is the closed two-value enum, so the per-verb candidate is
// selected exhaustively — VerbCommit reads [commit]'s override, every other (i.e.
// VerbRelease, the zero value) reads [release]'s.
func (c Config) TimeoutFor(verb Verb) *time.Duration {
	override, floor := c.Release.Timeout, DefaultReleaseTimeout
	if verb == VerbCommit {
		override, floor = c.Commit.Timeout, DefaultCommitTimeout
	}

	// The floor is appended last so a single value-semantics loop covers every layer and
	// the always-present floor makes the result total.
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
// it is AICommandFor's (this is absent→default only).
func aiCommandOrDefault(v *string) string {
	if v != nil {
		return *v
	}
	return DefaultAICommand
}

// resolveMaxPromptBytes applies DefaultMaxPromptBytes when the key was absent (nil) and
// otherwise carries the explicit value, which validateMaxPromptBytes has already proven
// positive.
func resolveMaxPromptBytes(v *int) int {
	if v != nil {
		return *v
	}
	return DefaultMaxPromptBytes
}

// resolveTimeout converts the decoded *int integer seconds into the *time.Duration
// carried on Config, preserving the absent-vs-explicit distinction the accessor needs.
// An ABSENT key (nil) stays nil — TimeoutFor resolves it to the verb's shipped floor, so
// it is DELIBERATELY not pre-defaulted here. A PRESENT
// value (including an explicit 0, "no deadline", or a negative TimeoutFor drops) is
// carried verbatim as seconds → duration, so absent (nil) is distinguishable from an
// explicit 0 (a non-nil pointer to 0). Mirrors the max_prompt_bytes *int nil-vs-present
// idiom, adding only the seconds→duration boundary conversion.
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
		VersionFile:    shape.VersionFile,
		VersionPattern: shape.VersionPattern,
		AICommand:      shape.AICommand,
		Timeout:        resolveTimeout(shape.Timeout),
		DiffExclude:    shape.DiffExclude,
		DiffKeep:       shape.DiffKeep,
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
// when absent; the literal value, blank or not, when present) for AICommandFor —
// blank-skipping is the accessor's job, not here. Timeout is the optional per-verb timeout
// override: commitShape carries it as a *int (integer seconds) but Commit carries a
// *time.Duration, so resolveTimeout performs the seconds → duration boundary conversion
// (nil stays nil; an explicit value, including 0 or a negative, is carried verbatim for
// TimeoutFor to interpret). Because those field TYPES now differ, this is an explicit
// field-by-field copy — the old direct Commit(shape) conversion no longer compiles (it
// relied on commitShape and Commit being field-identical, which the timeout type boundary
// breaks), mirroring resolveRelease.
func resolveCommit(shape commitShape) Commit {
	return Commit{
		Context:     shape.Context,
		Prompt:      shape.Prompt,
		AICommand:   shape.AICommand,
		Timeout:     resolveTimeout(shape.Timeout),
		DiffExclude: shape.DiffExclude,
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
				FileName, e.key,
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
		FileName, strings.Join(onNotesFailureValues, ", "),
	)
}

// validateMaxPromptBytes rejects an explicit max_prompt_bytes of zero or less: zero is
// never "no cap". An absent key (nil) passes and resolves to the default.
func validateMaxPromptBytes(v *int) error {
	if v != nil && *v <= 0 {
		return fmt.Errorf("invalid %s: max_prompt_bytes must be a positive number of bytes", FileName)
	}
	return nil
}

// validateAICommands checks every level whatever verb is running, so a bad
// [commit].ai_command also stops `mint release`. A blank value is skipped, not
// rejected: AICommandFor falls it through to the next level.
func validateAICommands(shape fileShape) error {
	levels := []struct {
		key   string
		value *string
	}{
		{"ai_command", shape.AICommand},
		{"release.ai_command", shape.Release.AICommand},
		{"commit.ai_command", shape.Commit.AICommand},
	}
	for _, level := range levels {
		if level.value == nil || strings.TrimSpace(*level.value) == "" {
			continue
		}
		if cause := aiCommandDefect(*level.value); cause != "" {
			return fmt.Errorf("invalid %s: %s %s", FileName, level.key, cause)
		}
	}
	return nil
}

// aiCommandDefect returns why a non-blank command cannot run, or "" when it is well-formed.
func aiCommandDefect(command string) string {
	words, err := cmdline.Split(command)
	switch {
	case errors.Is(err, cmdline.ErrUnterminatedQuote):
		return "has an unterminated quote"
	case errors.Is(err, cmdline.ErrTrailingBackslash):
		return "ends in a lone backslash"
	case err != nil:
		return err.Error()
	case len(words) == 0 || words[0] == "":
		return "names no program"
	default:
		return ""
	}
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
