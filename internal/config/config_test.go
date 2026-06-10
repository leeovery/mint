package config_test

import (
	"os"
	"path/filepath"
	"testing"

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

func TestLoad_ExplicitOnNotesFailureAnyString_CarriedVerbatim(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// on_notes_failure is mode-only (abort | fallback); config still carries whatever
	// raw string the file holds verbatim (Phase 6 adds typed validation that rejects
	// unknown values — config does not interpret or coerce the value here).
	writeConfig(t, dir, "[release]\non_notes_failure = \"something-unknown\"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Release.OnNotesFailure != "something-unknown" {
		t.Errorf("OnNotesFailure = %q, want the raw value carried verbatim", cfg.Release.OnNotesFailure)
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

func TestLoad_UnknownKeysTolerated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Phase 6 adds fail-loud validation; Phase 1 must tolerate unknown keys so the
	// skeleton runs against forward-looking config without erroring.
	writeConfig(t, dir, "[release]\ntag_prefix = \"v\"\nversion_file = \"bin/tool\"\nunknown_key = 42\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error for unknown keys: %v", err)
	}
	if cfg.Release.TagPrefix != "v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Release.TagPrefix, "v")
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
