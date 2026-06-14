package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/config"
)

// writeConfig writes body to {dir}/.mint.toml, failing the test on error.
func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

func TestLoad_AbsentFile_ReturnsAllDefaults(t *testing.T) {
	t.Parallel()

	// An empty temp dir has no .mint.toml — the loader must fall back to defaults
	// rather than erroring, because config is fully optional.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.TagPrefix != "v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Release.TagPrefix, "v")
	}
	if cfg.Release.CommitPrefix != "🌿" {
		t.Errorf("CommitPrefix = %q, want %q", cfg.Release.CommitPrefix, "🌿")
	}
	if cfg.Release.ReleaseBranch != "" {
		t.Errorf("ReleaseBranch = %q, want empty (auto-derive sentinel)", cfg.Release.ReleaseBranch)
	}
	if !cfg.Release.Publish {
		t.Errorf("Publish = %v, want true", cfg.Release.Publish)
	}
	if cfg.Release.OnNotesFailure != "abort" {
		t.Errorf("OnNotesFailure = %q, want default %q", cfg.Release.OnNotesFailure, "abort")
	}
}

func TestLoad_AbsentMaxDiffLines_DefaultsTo50000(t *testing.T) {
	t.Parallel()

	// max_diff_lines is a shared TOP-LEVEL engine key (not under [release]). When
	// absent it must default to 50000, the cost+quality guard ceiling.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.MaxDiffLines != 50000 {
		t.Errorf("MaxDiffLines = %d, want default 50000", cfg.MaxDiffLines)
	}
}

func TestLoad_ExplicitMaxDiffLines_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit top-level max_diff_lines must override the 50000 default. It sits
	// above the [release] table, so it is set with no table header.
	writeConfig(t, dir, "max_diff_lines = 1200\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.MaxDiffLines != 1200 {
		t.Errorf("MaxDiffLines = %d, want explicit 1200", cfg.MaxDiffLines)
	}
}

func TestLoad_SubsetOfKeys_OverridesPresentRestDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Only tag_prefix is present; the other three must remain at their defaults.
	writeConfig(t, dir, "[release]\ntag_prefix = \"pkg-name/v\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.TagPrefix != "pkg-name/v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Release.TagPrefix, "pkg-name/v")
	}
	if cfg.Release.CommitPrefix != "🌿" {
		t.Errorf("CommitPrefix = %q, want default %q", cfg.Release.CommitPrefix, "🌿")
	}
	if cfg.Release.ReleaseBranch != "" {
		t.Errorf("ReleaseBranch = %q, want default empty", cfg.Release.ReleaseBranch)
	}
	if !cfg.Release.Publish {
		t.Errorf("Publish = %v, want default true", cfg.Release.Publish)
	}
}

func TestLoad_PublishFalse_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// publish=false is the bool trap: its zero value is also false, so an explicit
	// false must be distinguished from absent (which defaults to true).
	writeConfig(t, dir, "[release]\npublish = false\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Publish {
		t.Errorf("Publish = %v, want false (explicit false must be honoured)", cfg.Release.Publish)
	}
}

func TestLoad_AbsentProvider_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	// [release].provider is the optional publishing-driver override. Absent from the
	// file it defaults to "" — the sentinel for "auto-detect from the remote host".
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Provider != "" {
		t.Errorf("Provider = %q, want empty default (auto-detect)", cfg.Release.Provider)
	}
}

func TestLoad_ExplicitProvider_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit provider value is carried through verbatim; the publish resolver
	// interprets it (a recognised value forces that driver over detection).
	writeConfig(t, dir, "[release]\nprovider = \"github\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Provider != "github" {
		t.Errorf("Provider = %q, want %q", cfg.Release.Provider, "github")
	}
}

func TestLoad_UnsupportedProviderValue_LoadsCleanNotAConfigError(t *testing.T) {
	t.Parallel()

	// provider is a normal string key: ANY string is a valid TYPE, so a well-typed but
	// unsupported value (e.g. "gitlab", which mint has no driver for) MUST load cleanly
	// — it is NOT a config error. The provider-VALUE carve-out lives in the Phase 4
	// publish resolver (an unsupported value warns + downgrades to tag + push), NOT in
	// config validation. config carries the raw value verbatim for the resolver.
	dir := t.TempDir()
	writeConfig(t, dir, "[release]\nprovider = \"gitlab\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned %v for an unsupported provider value; it must load cleanly (the value carve-out is the publish resolver's, not config's)", err)
	}

	if cfg.Release.Provider != "gitlab" {
		t.Errorf("Provider = %q, want %q carried verbatim for the publish resolver", cfg.Release.Provider, "gitlab")
	}
}

func TestLoad_AbsentChangelog_DefaultsToTrue(t *testing.T) {
	t.Parallel()

	// [release].changelog gates the CHANGELOG projection. Absent from the file it
	// defaults to true — the changelog is written out of the box, mirroring publish.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if !cfg.Release.Changelog {
		t.Errorf("Changelog = %v, want true (absent key defaults to true)", cfg.Release.Changelog)
	}
}

func TestLoad_ChangelogFalse_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// changelog=false is the same bool trap as publish: its zero value is also false,
	// so an explicit false must be distinguished from absent (which defaults to true).
	writeConfig(t, dir, "[release]\nchangelog = false\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Changelog {
		t.Errorf("Changelog = %v, want false (explicit false must be honoured)", cfg.Release.Changelog)
	}
}

func TestLoad_ChangelogTrue_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit changelog=true is the default value stated explicitly; it must be
	// carried through as true (not coerced or dropped).
	writeConfig(t, dir, "[release]\nchangelog = true\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if !cfg.Release.Changelog {
		t.Errorf("Changelog = %v, want true (explicit true must be honoured)", cfg.Release.Changelog)
	}
}

func TestLoad_ExplicitEmptyTagPrefix_Preserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit empty tag_prefix means prefix-less tags; it is a valid value and
	// must NOT be coerced back to the "v" default.
	writeConfig(t, dir, "[release]\ntag_prefix = \"\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.TagPrefix != "" {
		t.Errorf("TagPrefix = %q, want empty (explicit empty must be preserved)", cfg.Release.TagPrefix)
	}
}

func TestLoad_BlankOrCommentsOnlyFile_ReturnsDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A file containing only blank lines and comments carries no [release] keys, so
	// it must behave exactly like an absent file: all defaults.
	writeConfig(t, dir, "\n# just a comment\n\n   \n# another comment\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.TagPrefix != "v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Release.TagPrefix, "v")
	}
	if cfg.Release.CommitPrefix != "🌿" {
		t.Errorf("CommitPrefix = %q, want %q", cfg.Release.CommitPrefix, "🌿")
	}
	if cfg.Release.ReleaseBranch != "" {
		t.Errorf("ReleaseBranch = %q, want empty", cfg.Release.ReleaseBranch)
	}
	if !cfg.Release.Publish {
		t.Errorf("Publish = %v, want true", cfg.Release.Publish)
	}
}

func TestLoad_ConfiguredCommitPrefixAndReleaseBranch_Returned(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "[release]\ncommit_prefix = \"🚀\"\nrelease_branch = \"trunk\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.CommitPrefix != "🚀" {
		t.Errorf("CommitPrefix = %q, want %q", cfg.Release.CommitPrefix, "🚀")
	}
	if cfg.Release.ReleaseBranch != "trunk" {
		t.Errorf("ReleaseBranch = %q, want %q", cfg.Release.ReleaseBranch, "trunk")
	}
}

func TestLoad_AbsentContextAndPrompt_DefaultToEmpty(t *testing.T) {
	t.Parallel()

	// [release].context and [release].prompt are the notes-engine prompt-control
	// knobs. Absent from the file, both default to the empty string — the "no
	// inject, default prompt" baseline.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Context != "" {
		t.Errorf("Context = %q, want empty (absent key defaults to empty)", cfg.Release.Context)
	}
	if cfg.Release.Prompt != "" {
		t.Errorf("Prompt = %q, want empty (absent key defaults to empty)", cfg.Release.Prompt)
	}
}

func TestLoad_ExplicitContext_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [release].context value (a literal string here) must be carried
	// through verbatim; the notes engine later treats it as string-or-file.
	writeConfig(t, dir, "[release]\ncontext = \"dev-workflow toolkit; emphasise user-facing changes\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Context != "dev-workflow toolkit; emphasise user-facing changes" {
		t.Errorf("Context = %q, want the explicit value", cfg.Release.Context)
	}
}

func TestLoad_ExplicitPrompt_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [release].prompt value is a file path carried through verbatim;
	// the notes engine later reads that file as the full prompt override.
	writeConfig(t, dir, "[release]\nprompt = \".mint/release-prompt.md\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Prompt != ".mint/release-prompt.md" {
		t.Errorf("Prompt = %q, want the explicit value", cfg.Release.Prompt)
	}
}

func TestLoad_AbsentOnNotesFailure_DefaultsToAbort(t *testing.T) {
	t.Parallel()

	// [release].on_notes_failure governs the normal-path notes-failure routing. Absent
	// from the file it defaults to "abort" — fail loud, tag nothing.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.OnNotesFailure != "abort" {
		t.Errorf("OnNotesFailure = %q, want default %q", cfg.Release.OnNotesFailure, "abort")
	}
}

func TestLoad_ExplicitOnNotesFailureFallback_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit on_notes_failure value must be carried through verbatim; the resolver
	// in the notes engine interprets the value (abort / fallback / fixed string).
	writeConfig(t, dir, "[release]\non_notes_failure = \"fallback\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.OnNotesFailure != "fallback" {
		t.Errorf("OnNotesFailure = %q, want %q", cfg.Release.OnNotesFailure, "fallback")
	}
}

func TestLoad_ExplicitOnNotesFailureUnknown_Rejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// on_notes_failure is a closed-set enum (abort | fallback). Phase 6's typed
	// validation rejects any other non-empty value rather than carrying it verbatim —
	// it is fail-loud, not interpreted later.
	writeConfig(t, dir, "[release]\non_notes_failure = \"something-unknown\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for unknown on_notes_failure, want non-nil")
	}
	if !strings.Contains(err.Error(), "on_notes_failure") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "on_notes_failure")
	}
}

func TestLoad_AbsentFallback_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	// [release].fallback is the dedicated fixed-fallback-body key, shared by both the
	// on_notes_failure=fallback path and --no-ai. Absent from the file it defaults to
	// the empty string — meaning "no fixed string, use the commit-subject list".
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Fallback != "" {
		t.Errorf("Fallback = %q, want empty (absent key defaults to empty)", cfg.Release.Fallback)
	}
}

func TestLoad_ExplicitFallback_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [release].fallback is the fixed fallback body string, carried through
	// verbatim; the notes engine uses it as the literal body for both fallback paths.
	writeConfig(t, dir, "[release]\nfallback = \"Notes unavailable — see commit history.\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Fallback != "Notes unavailable — see commit history." {
		t.Errorf("Fallback = %q, want the explicit fixed string", cfg.Release.Fallback)
	}
}

func TestLoad_AbsentVersionFileAndPattern_DefaultToEmpty(t *testing.T) {
	t.Parallel()

	// [release].version_file and [release].version_pattern drive the optional
	// version-file projection. Absent from the file, both default to the empty
	// string — meaning "no projection, tag-only" (the out-of-the-box behaviour).
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.VersionFile != "" {
		t.Errorf("VersionFile = %q, want empty (absent key defaults to empty)", cfg.Release.VersionFile)
	}
	if cfg.Release.VersionPattern != "" {
		t.Errorf("VersionPattern = %q, want empty (absent key defaults to empty)", cfg.Release.VersionPattern)
	}
}

func TestLoad_ExplicitVersionFile_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [release].version_file is the path mint mirrors the version into,
	// carried through verbatim; Record writes the new version there (plain mode when
	// no version_pattern is set).
	writeConfig(t, dir, "[release]\nversion_file = \"release.txt\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.VersionFile != "release.txt" {
		t.Errorf("VersionFile = %q, want the explicit path", cfg.Release.VersionFile)
	}
	if cfg.Release.VersionPattern != "" {
		t.Errorf("VersionPattern = %q, want empty (no pattern → plain mode)", cfg.Release.VersionPattern)
	}
}

func TestLoad_ExplicitVersionPattern_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [release].version_pattern selects embedded mode (surgical line
	// replacement); it is carried through verbatim for Record to apply.
	writeConfig(t, dir, "[release]\nversion_file = \"main.go\"\nversion_pattern = \"RELEASE_VERSION = \\\"{version}\\\"\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.VersionFile != "main.go" {
		t.Errorf("VersionFile = %q, want %q", cfg.Release.VersionFile, "main.go")
	}
	if cfg.Release.VersionPattern != "RELEASE_VERSION = \"{version}\"" {
		t.Errorf("VersionPattern = %q, want the explicit pattern", cfg.Release.VersionPattern)
	}
}

func TestLoad_AbsentDiffExclude_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	// diff_exclude is a shared TOP-LEVEL engine key (not under [release]) listing
	// extra globs to exclude from the diff ON TOP OF the built-in CHANGELOG.md.
	// Absent from the file it must default to empty — only CHANGELOG.md is excluded.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if len(cfg.DiffExclude) != 0 {
		t.Errorf("DiffExclude = %#v, want empty (absent key → no extra excludes)", cfg.DiffExclude)
	}
}

func TestLoad_DiffExcludeArray_DecodesAllGlobsInOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit top-level diff_exclude array of glob strings decodes to a []string
	// in file order; each entry later becomes a :(exclude)<glob> pathspec. It sits
	// above the [release] table, so it is set with no table header.
	writeConfig(t, dir, "diff_exclude = [\"skills/**/knowledge.cjs\", \"*.min.js\"]\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	want := []string{"skills/**/knowledge.cjs", "*.min.js"}
	if len(cfg.DiffExclude) != len(want) {
		t.Fatalf("DiffExclude = %#v, want %#v", cfg.DiffExclude, want)
	}
	for i := range want {
		if cfg.DiffExclude[i] != want[i] {
			t.Errorf("DiffExclude[%d] = %q, want %q (order must be preserved)", i, cfg.DiffExclude[i], want[i])
		}
	}
}

func TestLoad_DiffExcludeSingleGlob_Decodes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A single-element diff_exclude array decodes to a one-element []string — the
	// minimal configured-exclude case.
	writeConfig(t, dir, "diff_exclude = [\"dist/bundle.js\"]\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if len(cfg.DiffExclude) != 1 || cfg.DiffExclude[0] != "dist/bundle.js" {
		t.Errorf("DiffExclude = %#v, want [\"dist/bundle.js\"]", cfg.DiffExclude)
	}
}

func TestLoad_AbsentHooksTable_AllNil(t *testing.T) {
	t.Parallel()

	// With no [release.hooks] table, all three raw hook values are nil — absent
	// meaning "no hook". The hook runner treats nil as a no-op.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.Hooks.Preflight != nil {
		t.Errorf("Hooks.Preflight = %v, want nil (absent table)", cfg.Release.Hooks.Preflight)
	}
	if cfg.Release.Hooks.PreTag != nil {
		t.Errorf("Hooks.PreTag = %v, want nil (absent table)", cfg.Release.Hooks.PreTag)
	}
	if cfg.Release.Hooks.PostRelease != nil {
		t.Errorf("Hooks.PostRelease = %v, want nil (absent table)", cfg.Release.Hooks.PostRelease)
	}
}

func TestLoad_StringPreflightHook_DecodesToString(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A single-string preflight value decodes to a string, carried verbatim for the
	// hook runner to normalise to one command entry.
	writeConfig(t, dir, "[release.hooks]\npreflight = \"scripts/check.sh\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	got, ok := cfg.Release.Hooks.Preflight.(string)
	if !ok {
		t.Fatalf("Hooks.Preflight = %#v (%T), want a string", cfg.Release.Hooks.Preflight, cfg.Release.Hooks.Preflight)
	}
	if got != "scripts/check.sh" {
		t.Errorf("Hooks.Preflight = %q, want %q", got, "scripts/check.sh")
	}
}

func TestLoad_ArrayPreflightHook_DecodesToSlice(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An array preflight value decodes to a slice of the elements in order, carried
	// verbatim for the hook runner to normalise to ordered command entries.
	writeConfig(t, dir, "[release.hooks]\npreflight = [\"a\", \"b\"]\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	got, ok := cfg.Release.Hooks.Preflight.([]any)
	if !ok {
		t.Fatalf("Hooks.Preflight = %#v (%T), want a []any slice", cfg.Release.Hooks.Preflight, cfg.Release.Hooks.Preflight)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Hooks.Preflight = %#v, want [\"a\", \"b\"]", got)
	}
}

func TestLoad_PreTagAndPostReleaseHooks_Decode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// pre_tag and post_release decode the same way (string here) so the 3-3/3-4
	// wiring tasks have them available with no further config change.
	writeConfig(t, dir, "[release.hooks]\npre_tag = \"scripts/pre-tag.sh\"\npost_release = \"scripts/post.sh\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	preTag, ok := cfg.Release.Hooks.PreTag.(string)
	if !ok || preTag != "scripts/pre-tag.sh" {
		t.Errorf("Hooks.PreTag = %#v, want %q", cfg.Release.Hooks.PreTag, "scripts/pre-tag.sh")
	}
	postRelease, ok := cfg.Release.Hooks.PostRelease.(string)
	if !ok || postRelease != "scripts/post.sh" {
		t.Errorf("Hooks.PostRelease = %#v, want %q", cfg.Release.Hooks.PostRelease, "scripts/post.sh")
	}
}

func TestLoad_UnknownTopLevelKey_Rejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Fail-loud validation (Phase 6): an unknown TOP-LEVEL key (one matching no
	// shared-engine field) must be rejected with a message naming the key, not
	// silently ignored.
	writeConfig(t, dir, "bar = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for unknown top-level key, want non-nil")
	}
	if !strings.Contains(err.Error(), "bar") {
		t.Errorf("error = %q, want it to name the unknown key %q", err.Error(), "bar")
	}
}

func TestLoad_UnknownReleaseKey_RejectedNamingReleaseTable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An unknown key inside [release] must be rejected with a message naming both the
	// key and the [release] table so the offender is unambiguous.
	writeConfig(t, dir, "[release]\ntag_prefix = \"v\"\nunknown_key = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for unknown [release] key, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown_key") {
		t.Errorf("error = %q, want it to name the unknown key %q", err.Error(), "unknown_key")
	}
	if !strings.Contains(err.Error(), "[release]") {
		t.Errorf("error = %q, want it to name the [release] table", err.Error())
	}
}

func TestLoad_UnknownCommitKey_RejectedNamingCommitTable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [commit] deliberately carries ONLY context/prompt — no push/scope keys (push is
	// FLAG-ONLY by spec). An unknown key — the explicitly-excluded `push` above all —
	// must be rejected naming both the key and the [commit] table, pinning the
	// deliberately-excluded-keys guarantee against regression.
	writeConfig(t, dir, "[commit]\npush = true\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for unknown [commit] key, want non-nil")
	}
	if !strings.Contains(err.Error(), "push") {
		t.Errorf("error = %q, want it to name the unknown key %q", err.Error(), "push")
	}
	if !strings.Contains(err.Error(), "[commit]") {
		t.Errorf("error = %q, want it to name the [commit] table", err.Error())
	}
}

func TestLoad_UnknownReleaseHooksKey_RejectedNamingHooksTable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An unknown key inside [release.hooks] must be rejected with a message naming both
	// the key and the [release.hooks] table.
	writeConfig(t, dir, "[release.hooks]\npreflight = \"scripts/check.sh\"\nbaz = \"oops\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for unknown [release.hooks] key, want non-nil")
	}
	if !strings.Contains(err.Error(), "baz") {
		t.Errorf("error = %q, want it to name the unknown key %q", err.Error(), "baz")
	}
	if !strings.Contains(err.Error(), "[release.hooks]") {
		t.Errorf("error = %q, want it to name the [release.hooks] table", err.Error())
	}
}

func TestLoad_TopLevelHooksTable_RejectedWithNestGuidance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A TOP-LEVEL [hooks] table is the documented contradiction (hooks must nest under
	// [release.hooks]). It gets a TARGETED message pointing to [release.hooks], NOT the
	// generic unknown-key message.
	writeConfig(t, dir, "[hooks]\npreflight = \"scripts/check.sh\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for top-level [hooks] table, want non-nil")
	}
	if !strings.Contains(err.Error(), "not valid at the top level") {
		t.Errorf("error = %q, want the targeted top-level [hooks] message, not the generic unknown-key one", err.Error())
	}
	if !strings.Contains(err.Error(), "[release.hooks]") {
		t.Errorf("error = %q, want it to guide the user to nest under [release.hooks]", err.Error())
	}
}

func TestLoad_TypeMismatch_MappedFriendlyMessages(t *testing.T) {
	t.Parallel()

	// Every constrained key's type mismatch must surface its mapped, actionable
	// message. This guards against go-toml/v2 changing its DecodeError field-path
	// text: if a DecodeError matches NONE of the mapped field paths, translation
	// silently degrades to the raw library description and this test fails loudly.
	cases := []struct {
		name string
		toml string
		want string
	}{
		{"max_diff_lines not an integer", "max_diff_lines = \"lots\"\n", "max_diff_lines must be an integer"},
		{"timeout not an integer", "timeout = \"fast\"\n", "timeout must be an integer (seconds)"},
		{"diff_exclude not an array", "diff_exclude = \"CHANGELOG.md\"\n", "diff_exclude must be an array of strings"},
		{"publish not a boolean", "[release]\npublish = \"yes\"\n", "publish must be a boolean"},
		{"changelog not a boolean", "[release]\nchangelog = \"no\"\n", "changelog must be a boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for %s, want non-nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want the mapped message %q (the decoder's field-path text no longer matches the type-message map)", err.Error(), tc.want)
			}
		})
	}
}

func TestLoad_TypodKey_SurfacedClearly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A typo'd key (tag_prefx instead of tag_prefix) must be surfaced by name, not
	// silently ignored — fail-loud is the whole point of catching typos.
	writeConfig(t, dir, "[release]\ntag_prefx = \"v\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for typo'd key, want non-nil")
	}
	if !strings.Contains(err.Error(), "tag_prefx") {
		t.Errorf("error = %q, want it to name the typo'd key %q", err.Error(), "tag_prefx")
	}
}

func TestLoad_FullyValidFile_LoadsWithoutError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A file containing ONLY valid keys at every level (top-level engine, [release],
	// [release.hooks]) must still load with no error once strict validation is on.
	writeConfig(t, dir, `ai_command     = "claude -p"
diff_exclude   = ["*.min.js"]
max_diff_lines = 50000

[release]
tag_prefix       = "v"
commit_prefix    = "🌿"
release_branch   = "main"
version_file     = "bin/tool"
version_pattern  = 'RELEASE_VERSION="{version}"'
changelog        = true
publish          = true
provider         = "github"
on_notes_failure = "abort"
context          = "dev toolkit"
prompt           = ".mint/notes-prompt.md"
fallback         = "see history"

[release.hooks]
preflight    = "scripts/check.sh"
pre_tag      = ["npm ci", "npm run build"]
post_release = "scripts/notify.sh"
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error for fully-valid file: %v", err)
	}
	if cfg.Release.TagPrefix != "v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Release.TagPrefix, "v")
	}
	if cfg.Release.Provider != "github" {
		t.Errorf("Provider = %q, want %q", cfg.Release.Provider, "github")
	}
}

func TestLoad_AbsentAICommand_DefaultsToClaudeSonnet(t *testing.T) {
	t.Parallel()

	// ai_command is a shared TOP-LEVEL engine key (not under [release]). Absent from
	// the file it defaults to the pinned "claude -p --model sonnet" — the model is
	// pinned so zero-config behaviour is predictable rather than inheriting the
	// operator's mutable Claude CLI default.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.AICommand != "claude -p --model sonnet" {
		t.Errorf("AICommand = %q, want default %q", cfg.AICommand, "claude -p --model sonnet")
	}
}

func TestDefaultAICommand_ExportedCanonicalValue(t *testing.T) {
	t.Parallel()

	// DefaultAICommand is the single CANONICAL source of the pinned default command:
	// it is exported so later sites (the transport, initgen's template) derive the
	// value from here rather than re-typing the literal. This test pins both that the
	// constant is referenceable from another package and that its value is the pinned
	// "claude -p --model sonnet".
	if config.DefaultAICommand != "claude -p --model sonnet" {
		t.Errorf("DefaultAICommand = %q, want %q", config.DefaultAICommand, "claude -p --model sonnet")
	}
}

func TestLoad_ExplicitAICommand_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit top-level ai_command must override the "claude -p" default. It sits
	// above the [release] table, so it is set with no table header.
	writeConfig(t, dir, "ai_command = \"llm --model gpt-4 chat\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.AICommand != "llm --model gpt-4 chat" {
		t.Errorf("AICommand = %q, want explicit %q", cfg.AICommand, "llm --model gpt-4 chat")
	}
}

func TestLoad_OnlyTopLevelKeys_ReleaseAndHooksFullyDefaulted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A file that sets ONLY shared top-level engine keys carries no [release] table at
	// all: the entire [release] table and the [release.hooks] sub-table must come back
	// fully defaulted, key by key, exactly as if absent.
	writeConfig(t, dir, "ai_command = \"custom\"\nmax_diff_lines = 99\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.AICommand != "custom" {
		t.Errorf("AICommand = %q, want explicit %q", cfg.AICommand, "custom")
	}
	if cfg.MaxDiffLines != 99 {
		t.Errorf("MaxDiffLines = %d, want explicit 99", cfg.MaxDiffLines)
	}
	if cfg.Release.TagPrefix != "v" {
		t.Errorf("Release.TagPrefix = %q, want default %q", cfg.Release.TagPrefix, "v")
	}
	if cfg.Release.CommitPrefix != "🌿" {
		t.Errorf("Release.CommitPrefix = %q, want default %q", cfg.Release.CommitPrefix, "🌿")
	}
	if !cfg.Release.Publish {
		t.Errorf("Release.Publish = %v, want default true", cfg.Release.Publish)
	}
	if !cfg.Release.Changelog {
		t.Errorf("Release.Changelog = %v, want default true", cfg.Release.Changelog)
	}
	if cfg.Release.OnNotesFailure != "abort" {
		t.Errorf("Release.OnNotesFailure = %q, want default %q", cfg.Release.OnNotesFailure, "abort")
	}
	if cfg.Release.Hooks.Preflight != nil || cfg.Release.Hooks.PreTag != nil || cfg.Release.Hooks.PostRelease != nil {
		t.Errorf("Release.Hooks = %#v, want all nil (table fully defaulted/absent)", cfg.Release.Hooks)
	}
}

func TestHookValue_DecodesFromStringAndArray(t *testing.T) {
	t.Parallel()

	// The canonical [release.hooks] fields are typed config.HookValue, a dedicated
	// string-or-array type: a TOML string decodes to a HookValue carrying a string and
	// a TOML array decodes to a HookValue carrying a slice. Both shapes are supported at
	// the schema level now (strict string-vs-array validation is a later task).
	tests := []struct {
		name    string
		body    string
		field   func(config.Config) config.HookValue
		wantStr string
		wantArr []string
	}{
		{
			name:    "string preflight",
			body:    "[release.hooks]\npreflight = \"scripts/check.sh\"\n",
			field:   func(c config.Config) config.HookValue { return c.Release.Hooks.Preflight },
			wantStr: "scripts/check.sh",
		},
		{
			name:    "array pre_tag",
			body:    "[release.hooks]\npre_tag = [\"npm ci\", \"npm run build\"]\n",
			field:   func(c config.Config) config.HookValue { return c.Release.Hooks.PreTag },
			wantArr: []string{"npm ci", "npm run build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tt.body)

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}

			got := tt.field(cfg)
			switch {
			case tt.wantStr != "":
				s, ok := got.(string)
				if !ok || s != tt.wantStr {
					t.Errorf("HookValue = %#v (%T), want string %q", got, got, tt.wantStr)
				}
			case tt.wantArr != nil:
				arr, ok := got.([]any)
				if !ok {
					t.Fatalf("HookValue = %#v (%T), want a []any slice", got, got)
				}
				if len(arr) != len(tt.wantArr) {
					t.Fatalf("HookValue len = %d, want %d", len(arr), len(tt.wantArr))
				}
				for i := range tt.wantArr {
					if arr[i] != tt.wantArr[i] {
						t.Errorf("HookValue[%d] = %v, want %q", i, arr[i], tt.wantArr[i])
					}
				}
			}
		})
	}
}

func TestLoad_MalformedTOML_ReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Syntactically broken TOML may surface as an error (it is not a tolerated
	// unknown key — the document cannot be parsed at all).
	writeConfig(t, dir, "[release\ntag_prefix = ")

	if _, err := config.Load(dir); err == nil {
		t.Fatal("Load returned nil error for malformed TOML, want non-nil")
	}
}

func TestLoad_MaxDiffLinesString_RejectedNamingIntegerType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Fail-loud bad-type validation (Phase 6): max_diff_lines is an integer key. A
	// string value must be rejected with a message naming both the key and the
	// expected integer type — not opaque decoder field-path output.
	writeConfig(t, dir, "max_diff_lines = \"big\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for string max_diff_lines, want non-nil")
	}
	if !strings.Contains(err.Error(), "max_diff_lines") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "max_diff_lines")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error = %q, want it to name the expected integer type", err.Error())
	}
}

func TestLoad_PublishString_RejectedNamingBooleanType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// publish is a boolean toggle. A string value must be rejected naming both the
	// key and the expected boolean type.
	writeConfig(t, dir, "[release]\npublish = \"yes\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for string publish, want non-nil")
	}
	if !strings.Contains(err.Error(), "publish") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "publish")
	}
	if !strings.Contains(err.Error(), "boolean") {
		t.Errorf("error = %q, want it to name the expected boolean type", err.Error())
	}
}

func TestLoad_ChangelogString_RejectedNamingBooleanType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// changelog is the other boolean toggle and must be rejected the same way as
	// publish when given a string.
	writeConfig(t, dir, "[release]\nchangelog = \"no\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for string changelog, want non-nil")
	}
	if !strings.Contains(err.Error(), "changelog") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "changelog")
	}
	if !strings.Contains(err.Error(), "boolean") {
		t.Errorf("error = %q, want it to name the expected boolean type", err.Error())
	}
}

func TestLoad_DiffExcludeScalar_RejectedNamingArrayOfStrings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// diff_exclude is an array of glob strings. A bare scalar (not an array) must be
	// rejected naming both the key and the expected array-of-strings shape.
	writeConfig(t, dir, "diff_exclude = '*.min.js'\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for scalar diff_exclude, want non-nil")
	}
	if !strings.Contains(err.Error(), "diff_exclude") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "diff_exclude")
	}
	if !strings.Contains(err.Error(), "array of strings") {
		t.Errorf("error = %q, want it to name the expected array-of-strings type", err.Error())
	}
}

func TestLoad_HookValueString_LoadsAndIsConsumable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A hook value given as a single command string must load successfully and stay
	// in the string shape the hooks runner's normalise consumes.
	writeConfig(t, dir, "[release.hooks]\npre_tag = \"scripts/check.sh\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error for string hook value: %v", err)
	}
	got, ok := cfg.Release.Hooks.PreTag.(string)
	if !ok || got != "scripts/check.sh" {
		t.Fatalf("Hooks.PreTag = %#v (%T), want string %q", cfg.Release.Hooks.PreTag, cfg.Release.Hooks.PreTag, "scripts/check.sh")
	}
}

func TestLoad_HookValueArrayOfStrings_LoadsAndIsConsumable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A hook value given as an array of command strings must load successfully and
	// stay in the []any shape the hooks runner's normalise consumes.
	writeConfig(t, dir, "[release.hooks]\npre_tag = [\"npm ci\", \"npm run build\"]\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error for array hook value: %v", err)
	}
	got, ok := cfg.Release.Hooks.PreTag.([]any)
	if !ok {
		t.Fatalf("Hooks.PreTag = %#v (%T), want a []any slice", cfg.Release.Hooks.PreTag, cfg.Release.Hooks.PreTag)
	}
	want := []string{"npm ci", "npm run build"}
	if len(got) != len(want) {
		t.Fatalf("Hooks.PreTag len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Hooks.PreTag[%d] = %v, want %q", i, got[i], want[i])
		}
	}
}

func TestLoad_HookValueInteger_RejectedNamingHookKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A hook value of a non-string/non-array type (here an integer) must be rejected
	// with a message naming the offending hook key.
	writeConfig(t, dir, "[release.hooks]\npre_tag = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for integer hook value, want non-nil")
	}
	if !strings.Contains(err.Error(), "pre_tag") {
		t.Errorf("error = %q, want it to name the hook key %q", err.Error(), "pre_tag")
	}
	if !strings.Contains(err.Error(), "string or an array of strings") {
		t.Errorf("error = %q, want it to state the valid hook shapes", err.Error())
	}
}

func TestLoad_HookValueTable_RejectedNamingHookKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A hook value given as a table is also an invalid shape and must be rejected
	// naming the offending hook key.
	writeConfig(t, dir, "[release.hooks.preflight]\nfoo = \"bar\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for table hook value, want non-nil")
	}
	if !strings.Contains(err.Error(), "preflight") {
		t.Errorf("error = %q, want it to name the hook key %q", err.Error(), "preflight")
	}
}

func TestLoad_HookValueArrayOfNonStrings_RejectedNamingHookKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An array hook value must contain only strings; an array carrying a non-string
	// element is an invalid shape and must be rejected naming the offending hook key.
	writeConfig(t, dir, "[release.hooks]\npost_release = [\"ok\", 7]\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for array-of-non-strings hook value, want non-nil")
	}
	if !strings.Contains(err.Error(), "post_release") {
		t.Errorf("error = %q, want it to name the hook key %q", err.Error(), "post_release")
	}
}

func TestLoad_OnNotesFailureInvalid_RejectedListingValidValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// on_notes_failure is a closed-set enum (abort | fallback). A correctly-typed but
	// out-of-set value must fail loud, listing the valid values.
	writeConfig(t, dir, "[release]\non_notes_failure = \"retry\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for invalid on_notes_failure, want non-nil")
	}
	if !strings.Contains(err.Error(), "on_notes_failure") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "on_notes_failure")
	}
	if !strings.Contains(err.Error(), "abort") || !strings.Contains(err.Error(), "fallback") {
		t.Errorf("error = %q, want it to list the valid values abort and fallback", err.Error())
	}
}

func TestLoad_OnNotesFailureValidValues_Load(t *testing.T) {
	t.Parallel()

	for _, want := range []string{"abort", "fallback"} {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			// Both members of the closed set must load without error and be carried
			// through verbatim.
			writeConfig(t, dir, "[release]\non_notes_failure = \""+want+"\"\n")

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error for on_notes_failure=%q: %v", want, err)
			}
			if cfg.Release.OnNotesFailure != want {
				t.Errorf("OnNotesFailure = %q, want %q", cfg.Release.OnNotesFailure, want)
			}
		})
	}
}

func TestLoad_AbsentCommitTable_ContextAndPromptEmpty(t *testing.T) {
	t.Parallel()

	// [commit].context and [commit].prompt are commit's prompt-control knobs. With no
	// [commit] table at all (here, an absent file), both must default to the empty
	// string — the "no inject, default prompt" baseline. Empty is a valid non-error
	// state, not a failure.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Commit.Context != "" {
		t.Errorf("Commit.Context = %q, want empty (absent [commit] table defaults to empty)", cfg.Commit.Context)
	}
	if cfg.Commit.Prompt != "" {
		t.Errorf("Commit.Prompt = %q, want empty (absent [commit] table defaults to empty)", cfg.Commit.Prompt)
	}
}

func TestLoad_ExplicitCommitContext_Honoured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [commit].context string must be carried through verbatim; assembly
	// (1-2) injects it into the commit prompt.
	writeConfig(t, dir, "[commit]\ncontext = \"Conventional Commits; dev-workflow toolkit.\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Commit.Context != "Conventional Commits; dev-workflow toolkit." {
		t.Errorf("Commit.Context = %q, want the explicit value", cfg.Commit.Context)
	}
}

func TestLoad_ExplicitCommitPrompt_HonouredAsPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit [commit].prompt value is a file path carried through verbatim;
	// assembly (1-2) reads that file via ResolveCommitPrompt as the full prompt override.
	writeConfig(t, dir, "[commit]\nprompt = \".mint/commit-prompt.md\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Commit.Prompt != ".mint/commit-prompt.md" {
		t.Errorf("Commit.Prompt = %q, want the explicit path", cfg.Commit.Prompt)
	}
}

func TestLoad_EmptyCommitContextAndPrompt_AreValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A [commit] table present with explicit empty values is a valid non-error state:
	// empty context means no injection, empty prompt means the default prompt.
	writeConfig(t, dir, "[commit]\ncontext = \"\"\nprompt = \"\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error for empty commit context/prompt: %v", err)
	}

	if cfg.Commit.Context != "" {
		t.Errorf("Commit.Context = %q, want empty (no injection)", cfg.Commit.Context)
	}
	if cfg.Commit.Prompt != "" {
		t.Errorf("Commit.Prompt = %q, want empty (default prompt)", cfg.Commit.Prompt)
	}
}

func TestLoad_CommitContextNonString_RejectedNamingKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [commit].context is a string key. A non-string value must fail loud with a
	// message naming the key (mint's message style, not the go-toml fallback text).
	writeConfig(t, dir, "[commit]\ncontext = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for non-string commit.context, want non-nil")
	}
	if !strings.Contains(err.Error(), "commit.context") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "commit.context")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error = %q, want it to name the expected string type", err.Error())
	}
}

func TestLoad_CommitPromptNonString_RejectedNamingKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [commit].prompt is a string key (a path). A non-string value must fail loud with
	// the mapped mint-style message naming the key and its expected string type. An
	// integer is used here because go-toml/v2 embeds the offending struct-field path in
	// its DecodeError for an integer-into-string mismatch — the signal translateTypeError
	// maps via typeErrorMessages. (A boolean-into-string mismatch is a separate go-toml
	// quirk that emits no field path for ANY string key; see
	// TestLoad_CommitBooleanValue_StillFailsLoud.)
	writeConfig(t, dir, "[commit]\nprompt = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for non-string commit.prompt, want non-nil")
	}
	if !strings.Contains(err.Error(), "commit.prompt") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "commit.prompt")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error = %q, want it to name the expected string type", err.Error())
	}
}

func TestLoad_CommitBooleanValue_StillFailsLoud(t *testing.T) {
	t.Parallel()

	// A boolean assigned to a [commit] string key still FAILS LOUD (never silently
	// accepted) — but go-toml/v2 emits no struct-field path for a boolean-into-string
	// mismatch (a library quirk shared by EVERY string key, e.g. [release].context /
	// provider), so translateTypeError cannot map it to the mint-style message and the
	// decoder's own positioned description is surfaced instead. This test pins that the
	// fail-loud guarantee holds for the boolean case and that the offending line (which
	// names the key) is still visible to the user.
	cases := []struct {
		name string
		toml string
		key  string
	}{
		{"boolean context", "[commit]\ncontext = true\n", "context"},
		{"boolean prompt", "[commit]\nprompt = false\n", "prompt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for %s, want non-nil (must fail loud)", tc.name)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error = %q, want the offending key %q to remain visible", err.Error(), tc.key)
			}
		})
	}
}

func TestLoad_ReleaseAICommand_CarriedRawOntoConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [release].ai_command is the optional per-verb override. config carries the raw
	// value verbatim onto the Release struct (a non-nil pointer to the literal string);
	// the override-chain resolution against the shared/floor is Task 1-4, not here.
	writeConfig(t, dir, "[release]\nai_command = \"claude -p --model opus\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.AICommand == nil {
		t.Fatalf("Release.AICommand = nil, want a non-nil pointer carrying the explicit value")
	}
	if *cfg.Release.AICommand != "claude -p --model opus" {
		t.Errorf("Release.AICommand = %q, want the explicit value carried verbatim", *cfg.Release.AICommand)
	}
}

func TestLoad_CommitAICommand_CarriedRawOntoConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [commit].ai_command is the commit-verb mirror of [release].ai_command. config
	// carries the raw value verbatim onto the Commit struct; resolution is Task 1-4.
	writeConfig(t, dir, "[commit]\nai_command = \"llm --model gpt-4 chat\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Commit.AICommand == nil {
		t.Fatalf("Commit.AICommand = nil, want a non-nil pointer carrying the explicit value")
	}
	if *cfg.Commit.AICommand != "llm --model gpt-4 chat" {
		t.Errorf("Commit.AICommand = %q, want the explicit value carried verbatim", *cfg.Commit.AICommand)
	}
}

func TestLoad_AbsentPerVerbAICommand_DistinctFromExplicitBlank(t *testing.T) {
	t.Parallel()

	t.Run("absent is nil", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// With no per-verb ai_command at all, the carried pointer is nil — the absent
		// sentinel that Task 1-4's resolver treats as "no override, fall through". A
		// *string (not plain string) is what makes absent distinguishable from blank.
		writeConfig(t, dir, "[release]\ntag_prefix = \"v\"\n")

		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load returned unexpected error: %v", err)
		}

		if cfg.Release.AICommand != nil {
			t.Errorf("Release.AICommand = %v, want nil (absent key → nil sentinel)", cfg.Release.AICommand)
		}
		if cfg.Commit.AICommand != nil {
			t.Errorf("Commit.AICommand = %v, want nil (absent key → nil sentinel)", cfg.Commit.AICommand)
		}
	})

	t.Run("explicit blank is a non-nil empty string", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// An explicit empty ai_command is a non-nil pointer to "" — DISTINCT from absent
		// (nil). config carries it verbatim, blank or not; blank-SKIPPING is the Task 1-4
		// accessor's job, NOT this task's.
		writeConfig(t, dir, "[release]\nai_command = \"\"\n")

		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load returned unexpected error: %v", err)
		}

		if cfg.Release.AICommand == nil {
			t.Fatalf("Release.AICommand = nil, want a non-nil pointer to the explicit empty string")
		}
		if *cfg.Release.AICommand != "" {
			t.Errorf("Release.AICommand = %q, want the explicit empty string carried verbatim", *cfg.Release.AICommand)
		}
	})
}

func TestAICommandFor_PresentNonBlankReleaseOverride_Wins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A present, non-blank [release].ai_command is the first layer of the chain
	// (verb → shared → floor), so it wins outright — neither the shared top-level
	// nor the floor is consulted.
	writeConfig(t, dir, "ai_command = \"shared-cmd\"\n[release]\nai_command = \"claude -p --model opus\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbRelease); got != "claude -p --model opus" {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the per-verb override %q", got, "claude -p --model opus")
	}
}

func TestAICommandFor_PresentNonBlankCommitOverride_Wins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// The [commit].ai_command override is the commit-verb mirror: present and
	// non-blank, it wins over the shared top-level value for VerbCommit.
	writeConfig(t, dir, "ai_command = \"shared-cmd\"\n[commit]\nai_command = \"llm --model gpt-4 chat\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbCommit); got != "llm --model gpt-4 chat" {
		t.Errorf("AICommandFor(VerbCommit) = %q, want the per-verb override %q", got, "llm --model gpt-4 chat")
	}
}

func TestAICommandFor_BlankPerVerbOverride_FallsToShared(t *testing.T) {
	t.Parallel()

	// A blank/whitespace [verb].ai_command is a present-but-empty override: the
	// accessor trims it, finds it empty, and SKIPS it, falling through to the
	// shared top-level ai_command (which here is an explicit non-default value).
	cases := []struct {
		name string
		toml string
		verb config.Verb
	}{
		{
			name: "empty release override falls to shared",
			toml: "ai_command = \"shared-cmd\"\n[release]\nai_command = \"\"\n",
			verb: config.VerbRelease,
		},
		{
			name: "whitespace release override falls to shared",
			toml: "ai_command = \"shared-cmd\"\n[release]\nai_command = \"   \"\n",
			verb: config.VerbRelease,
		},
		{
			name: "empty commit override falls to shared",
			toml: "ai_command = \"shared-cmd\"\n[commit]\nai_command = \"\"\n",
			verb: config.VerbCommit,
		},
		{
			name: "whitespace commit override falls to shared",
			toml: "ai_command = \"shared-cmd\"\n[commit]\nai_command = \"\\t \"\n",
			verb: config.VerbCommit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}

			if got := cfg.AICommandFor(tc.verb); got != "shared-cmd" {
				t.Errorf("AICommandFor = %q, want the shared top-level %q (blank override skipped)", got, "shared-cmd")
			}
		})
	}
}

func TestAICommandFor_WhitespaceSharedCommand_FallsToFloor(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A whitespace-only shared top-level ai_command is itself a blank candidate:
	// the accessor trims and skips it too, so both verbs fall through to the
	// shipped DefaultAICommand floor.
	writeConfig(t, dir, "ai_command = \"   \"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbRelease); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the shipped floor %q", got, config.DefaultAICommand)
	}
	if got := cfg.AICommandFor(config.VerbCommit); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbCommit) = %q, want the shipped floor %q", got, config.DefaultAICommand)
	}
}

func TestAICommandFor_TopLevelEmptyCommand_ResolvesToShippedDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit top-level `ai_command = ""` (empty, no per-verb override) must
	// reach the accessor AS empty — Load must NOT re-default it — so the accessor's
	// trim-and-skip is what falls it through to DefaultAICommand.
	writeConfig(t, dir, "ai_command = \"\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbRelease); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the shipped default %q", got, config.DefaultAICommand)
	}
	if got := cfg.AICommandFor(config.VerbCommit); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbCommit) = %q, want the shipped default %q", got, config.DefaultAICommand)
	}
}

func TestAICommandFor_NeverReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Across every blank-at-every-layer input the floor guarantees a non-empty
	// command: the accessor must never yield "" for either verb. This pins the
	// invariant the transport's old empty→re-default path relied on becoming
	// unreachable.
	cases := []struct {
		name string
		toml string
	}{
		{"all layers blank for release", "ai_command = \"\"\n[release]\nai_command = \"  \"\n"},
		{"all layers blank for commit", "ai_command = \"  \"\n[commit]\nai_command = \"\"\n"},
		{"shared blank, no per-verb override", "ai_command = \"\\t\\t\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}

			for _, verb := range []config.Verb{config.VerbRelease, config.VerbCommit} {
				if got := cfg.AICommandFor(verb); got == "" {
					t.Errorf("AICommandFor(%v) = empty, want a non-empty command (floor guarantees one)", verb)
				}
			}
		})
	}
}

func TestAICommandFor_NoConfigFile_ResolvesBothVerbsToPinnedDefault(t *testing.T) {
	t.Parallel()

	// Zero-config: no .mint.toml at all. Both verbs resolve straight to the pinned
	// DefaultAICommand floor — the shared top-level is itself the default (seeded by
	// defaults()), and no per-verb override exists.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbRelease); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the pinned default %q", got, config.DefaultAICommand)
	}
	if got := cfg.AICommandFor(config.VerbCommit); got != config.DefaultAICommand {
		t.Errorf("AICommandFor(VerbCommit) = %q, want the pinned default %q", got, config.DefaultAICommand)
	}
}

func TestAICommandFor_PreservesRawCommandString(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// The accessor trims ONLY for the empty-check — the returned value is the
	// operator's RAW string. Internal multi-space runs must survive verbatim (the
	// transport whitespace-splits it; collapsing here would mutate the operator's
	// intended argv).
	const raw = "claude   -p    --model   opus"
	writeConfig(t, dir, "[release]\nai_command = \"claude   -p    --model   opus\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got := cfg.AICommandFor(config.VerbRelease); got != raw {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the raw command %q (no internal-spacing collapse)", got, raw)
	}
}

func TestAICommandFor_ReadsOnlyAICommandCandidates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Per-key independence by construction: the accessor reads only ai_command
	// candidates. A leading/trailing-padded shared command is preserved verbatim,
	// proving the trim is empty-check-only and the value is never re-derived from an
	// unrelated key.
	const raw = "  padded-cmd  "
	writeConfig(t, dir, "ai_command = \"  padded-cmd  \"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	// Non-blank after trim, so it wins — but the RAW padded string is returned.
	if got := cfg.AICommandFor(config.VerbRelease); got != raw {
		t.Errorf("AICommandFor(VerbRelease) = %q, want the raw padded shared command %q", got, raw)
	}
}

func TestLoad_IntegerReleaseAICommand_RejectedNamingKeyAndStringType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [release].ai_command is a string key. An integer value must fail loud with the
	// mapped mint-style message naming both the key and its expected string type — an
	// integer-into-string mismatch embeds the struct-field path translateTypeError maps.
	writeConfig(t, dir, "[release]\nai_command = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for integer release.ai_command, want non-nil")
	}
	if !strings.Contains(err.Error(), "release.ai_command") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "release.ai_command")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error = %q, want it to name the expected string type", err.Error())
	}
}

func TestLoad_IntegerCommitAICommand_RejectedNamingKeyAndStringType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// [commit].ai_command is a string key. An integer value must fail loud with the
	// mapped mint-style message naming both the key and its expected string type.
	writeConfig(t, dir, "[commit]\nai_command = 42\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for integer commit.ai_command, want non-nil")
	}
	if !strings.Contains(err.Error(), "commit.ai_command") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "commit.ai_command")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error = %q, want it to name the expected string type", err.Error())
	}
}

func TestLoad_BooleanPerVerbAICommand_StillFailsLoud(t *testing.T) {
	t.Parallel()

	// A boolean assigned to a per-verb ai_command string key still FAILS LOUD (never
	// silently accepted) — but go-toml/v2 emits no struct-field path for a
	// boolean-into-string mismatch (the library quirk pinned by
	// TestLoad_CommitBooleanValue_StillFailsLoud), so translateTypeError falls back to
	// the decoder's own positioned description. This pins fail-loud holds and the
	// offending key stays visible — it deliberately does NOT over-assert the mapped
	// message for the boolean case.
	cases := []struct {
		name string
		toml string
		key  string
	}{
		{"boolean release.ai_command", "[release]\nai_command = true\n", "ai_command"},
		{"boolean commit.ai_command", "[commit]\nai_command = false\n", "ai_command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for %s, want non-nil (must fail loud)", tc.name)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error = %q, want the offending key %q to remain visible", err.Error(), tc.key)
			}
		})
	}
}

func TestLoad_UnknownSiblingKey_StillRejectedAfterAddingAICommand(t *testing.T) {
	t.Parallel()

	// Adding the recognised per-verb ai_command key must NOT loosen DisallowUnknownFields
	// for the rest of the table: an unknown sibling alongside a valid ai_command in
	// [release] / [commit] is still rejected naming the key and its table.
	cases := []struct {
		name  string
		toml  string
		key   string
		table string
	}{
		{
			name:  "release sibling",
			toml:  "[release]\nai_command = \"claude -p\"\nbogus = 1\n",
			key:   "bogus",
			table: "[release]",
		},
		{
			name:  "commit sibling",
			toml:  "[commit]\nai_command = \"claude -p\"\nbogus = 1\n",
			key:   "bogus",
			table: "[commit]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tc.toml)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for unknown sibling in %s, want non-nil", tc.table)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error = %q, want it to name the unknown key %q", err.Error(), tc.key)
			}
			if !strings.Contains(err.Error(), tc.table) {
				t.Errorf("error = %q, want it to name the %s table", err.Error(), tc.table)
			}
		})
	}
}

func TestResolveCommitPrompt_AbsentPrompt_ReturnsEmptyNoError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No [commit].prompt configured → no override. ResolveCommitPrompt returns the
	// empty string with no error: assembly (1-2) reads empty as "use the default
	// commit prompt", never an override.
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	override, err := config.ResolveCommitPrompt(cfg, dir)
	if err != nil {
		t.Fatalf("ResolveCommitPrompt returned unexpected error: %v", err)
	}
	if override != "" {
		t.Errorf("override = %q, want empty (no [commit].prompt configured)", override)
	}
}

func TestResolveCommitPrompt_ConfiguredPrompt_ReturnsFileContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A configured [commit].prompt path that exists is read and its contents returned
	// verbatim as the full prompt override for assembly.
	const body = "You are writing a Conventional Commits message.\n"
	if err := os.WriteFile(filepath.Join(dir, "commit-prompt.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing prompt override file: %v", err)
	}
	writeConfig(t, dir, "[commit]\nprompt = \"commit-prompt.md\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	override, err := config.ResolveCommitPrompt(cfg, dir)
	if err != nil {
		t.Fatalf("ResolveCommitPrompt returned unexpected error: %v", err)
	}
	if override != body {
		t.Errorf("override = %q, want the file contents %q", override, body)
	}
}

func TestResolveCommitPrompt_MissingPromptFile_FailsLoudNamingPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A configured [commit].prompt path that does not exist must fail loud naming the
	// path — never silently fall through to the default prompt.
	writeConfig(t, dir, "[commit]\nprompt = \".mint/missing-commit-prompt.md\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	_, err = config.ResolveCommitPrompt(cfg, dir)
	if err == nil {
		t.Fatal("ResolveCommitPrompt returned nil error for a missing prompt file, want non-nil")
	}
	if !strings.Contains(err.Error(), ".mint/missing-commit-prompt.md") {
		t.Errorf("error = %q, want it to name the configured path", err.Error())
	}
}

func TestResolveCommitPrompt_UnreadablePromptFile_FailsLoudNamingPath(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		// root ignores 0o000 permissions, so the unreadable-file premise does not hold
		// and the assertion would silently pass; skip rather than fake-verify.
		t.Skip("running as root: stripped permissions do not make the file unreadable")
	}

	dir := t.TempDir()
	// A configured [commit].prompt path that exists but cannot be read (here, perms
	// stripped) must fail loud naming the path — never silently fall through to the
	// default prompt.
	const name = "unreadable-commit-prompt.md"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("secret prompt"), 0o000); err != nil {
		t.Fatalf("writing unreadable prompt file: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	writeConfig(t, dir, "[commit]\nprompt = \""+name+"\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	_, err = config.ResolveCommitPrompt(cfg, dir)
	if err == nil {
		t.Fatal("ResolveCommitPrompt returned nil error for an unreadable prompt file, want non-nil")
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("error = %q, want it to name the configured path %q", err.Error(), name)
	}
}

func TestDefaultTimeout_ExportedCanonicalValue(t *testing.T) {
	t.Parallel()

	// DefaultTimeout is the single CANONICAL source of the shipped per-attempt
	// deadline (60s) — exported so later phases derive the value from here rather than
	// re-typing the transport's defaultTimeout literal (which Phase 2 deletes). This
	// pins both that the constant is referenceable from another package and its value.
	if config.DefaultTimeout != 60*time.Second {
		t.Errorf("DefaultTimeout = %v, want %v", config.DefaultTimeout, 60*time.Second)
	}
}

func TestLoad_TopLevelTimeout_CarriedAsSeconds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// timeout is a shared TOP-LEVEL engine key (not under [release]). It is integer
	// seconds in TOML; config converts it to a time.Duration at the boundary, so a
	// top-level `timeout = 90` is carried onto Config as 90 seconds.
	writeConfig(t, dir, "timeout = 90\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Timeout == nil {
		t.Fatalf("Timeout = nil, want a non-nil pointer to 90s")
	}
	if *cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s (integer seconds converted to a duration)", *cfg.Timeout)
	}
}

func TestLoad_AbsentTimeout_DistinctFromExplicitZero(t *testing.T) {
	t.Parallel()

	t.Run("absent in a present file is nil", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// With no top-level timeout key at all (a present file setting only an
		// unrelated key), the carried pointer is nil — the absent sentinel Task 1-7's
		// accessor treats as "fall through to the 60s floor". A *time.Duration (not a
		// plain duration) is what makes absent distinguishable from an explicit zero.
		writeConfig(t, dir, "max_diff_lines = 1200\n")

		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load returned unexpected error: %v", err)
		}

		if cfg.Timeout != nil {
			t.Errorf("Timeout = %v, want nil (absent key → nil sentinel)", cfg.Timeout)
		}
	})

	t.Run("explicit zero is a non-nil pointer to 0", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// An explicit `timeout = 0` is a non-nil pointer to 0 — DISTINCT from absent
		// (nil). Zero means "no deadline", a conscious operator choice; config carries
		// it verbatim and Task 1-7's accessor honours it (stops the fall-through). The
		// distinction is the whole point of the *time.Duration field.
		writeConfig(t, dir, "timeout = 0\n")

		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load returned unexpected error: %v", err)
		}

		if cfg.Timeout == nil {
			t.Fatalf("Timeout = nil, want a non-nil pointer to the explicit zero")
		}
		if *cfg.Timeout != 0 {
			t.Errorf("Timeout = %v, want explicit 0 (carried verbatim, not coerced to 60s)", *cfg.Timeout)
		}
	})
}

func TestLoad_AbsentFile_ResolvesShipped60sDefault(t *testing.T) {
	t.Parallel()

	// With no .mint.toml at all, defaults() seeds the canonical 60s onto Config.Timeout
	// directly — the zero-config per-attempt deadline. This mirrors how defaults() seeds
	// the other top-level shared keys (ai_command, max_diff_lines) so a direct reader of
	// the zero-config Config sees the shipped value, not a bare nil.
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Timeout == nil {
		t.Fatalf("Timeout = nil, want the shipped 60s default for zero config")
	}
	if *cfg.Timeout != config.DefaultTimeout {
		t.Errorf("Timeout = %v, want the shipped default %v", *cfg.Timeout, config.DefaultTimeout)
	}
}

func TestLoad_ExplicitTimeoutZero_CarriedAsDistinguishableExplicitZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// An explicit top-level `timeout = 0` must NOT be coerced to the 60s default — it is
	// a conscious "no deadline" choice that Task 1-7 honours. The decode-onto-shape must
	// let an explicit 0 win as a non-nil pointer to 0, never collapsing into the absent
	// (nil → 60s floor) case.
	writeConfig(t, dir, "timeout = 0\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Timeout == nil {
		t.Fatalf("Timeout = nil, want a non-nil pointer to the explicit zero (not the default)")
	}
	if *cfg.Timeout != 0 {
		t.Errorf("Timeout = %v, want explicit 0 honoured (not coerced to the 60s default)", *cfg.Timeout)
	}
}

func TestLoad_NonIntegerTimeout_RejectedNamingKeyAndIntegerType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// timeout is an integer-seconds key, so a non-integer TOML value (a string here) is
	// a strict decode (type) error at Load — exactly like max_diff_lines. The friendly
	// message must name both the key and the expected integer/seconds type, not opaque
	// decoder field-path output.
	writeConfig(t, dir, "timeout = \"fast\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for string timeout, want non-nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %q, want it to name the key %q", err.Error(), "timeout")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error = %q, want it to name the expected integer type", err.Error())
	}
}
