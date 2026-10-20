package config_test

import (
	"strings"
	"testing"

	"mint/internal/config"
)

// removedFallbackMessage is the load error a .mint.toml still setting [release].fallback
// fails with: it names what replaced the key, so the operator learns at load.
const removedFallbackMessage = "invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file"

func TestLoad_ReleaseFallback_RejectedNamingNotesFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "string value", config: "[release]\nfallback = \"Notes unavailable — see commit history.\"\n"},
		{name: "empty string value", config: "[release]\nfallback = \"\"\n"},
		{name: "non-string value", config: "[release]\nfallback = 3\n"},
		{name: "beside on_notes_failure", config: "[release]\non_notes_failure = \"fallback\"\nfallback = \"fixed\"\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tt.config)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatal("Load returned nil error for a [release].fallback key, want non-nil")
			}
			if err.Error() != removedFallbackMessage {
				t.Errorf("error = %q, want %q", err.Error(), removedFallbackMessage)
			}
		})
	}
}

func TestLoad_FallbackOutsideRelease_StaysAGenericUnknownKey(t *testing.T) {
	t.Parallel()

	// Only [release].fallback was a key; a fallback anywhere else never was, so it keeps
	// the generic unknown-key message rather than claiming a removal.
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "top level", config: "fallback = \"x\"\n", want: "invalid .mint.toml: unknown top-level key \"fallback\""},
		{name: "commit table", config: "[commit]\nfallback = \"x\"\n", want: "invalid .mint.toml: unknown key \"fallback\" in [commit]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tt.config)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatal("Load returned nil error for an unknown fallback key, want non-nil")
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestLoad_OnNotesFailure_KeepsBothValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"abort", "fallback"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, "[release]\non_notes_failure = \""+value+"\"\n")

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}
			if cfg.Release.OnNotesFailure != value {
				t.Errorf("OnNotesFailure = %q, want %q", cfg.Release.OnNotesFailure, value)
			}
		})
	}
}

func TestMetadataRows_CarryNoReleaseFallbackRow(t *testing.T) {
	t.Parallel()

	for _, row := range config.MetadataRows() {
		if row.Key == "fallback" {
			t.Errorf("config metadata still carries a fallback row: %+v", row)
		}
	}
}

func TestMetadataRows_OnNotesFailureNamesTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	for _, row := range config.MetadataRows() {
		if row.Key != "on_notes_failure" {
			continue
		}
		if !strings.Contains(row.Description, "fallback ships the commit-subject list") {
			t.Errorf("on_notes_failure description = %q, want it to say fallback ships the commit-subject list", row.Description)
		}
		if strings.Contains(row.Description, "fixed") {
			t.Errorf("on_notes_failure description = %q still names a fixed string", row.Description)
		}
		return
	}
	t.Fatal("config metadata carries no on_notes_failure row")
}

func TestREADME_ReleaseTableCarriesNoFallbackRow(t *testing.T) {
	t.Parallel()

	for _, line := range strings.Split(readREADME(t, readmePath(t)), "\n") {
		if strings.HasPrefix(line, "| `fallback`") {
			t.Errorf("README still carries a fallback row: %q", line)
		}
	}
}

func TestREADME_OnNotesFailureRowNamesTheCommitSubjectList(t *testing.T) {
	t.Parallel()

	for _, line := range strings.Split(readREADME(t, readmePath(t)), "\n") {
		if !strings.HasPrefix(line, "| `on_notes_failure`") {
			continue
		}
		if !strings.Contains(line, "`fallback` ships the commit-subject list") {
			t.Errorf("README on_notes_failure row = %q, want it to say `fallback` ships the commit-subject list", line)
		}
		if strings.Contains(line, "string") || strings.Contains(line, "fixed") {
			t.Errorf("README on_notes_failure row = %q still names a fixed string", line)
		}
		return
	}
	t.Fatal("README carries no on_notes_failure row")
}
