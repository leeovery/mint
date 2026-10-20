package config_test

import (
	"strconv"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
)

func TestLoad_MaxPromptBytes_ResolvesCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string // empty writes no .mint.toml at all
		want   int
	}{
		{name: "no config file", config: "", want: 1_500_000},
		{name: "config without the key", config: "[release]\ntag_prefix = \"v\"\n", want: 1_500_000},
		{name: "explicit top-level value", config: "max_prompt_bytes = 800000\n", want: 800_000},
		{name: "explicit one byte", config: "max_prompt_bytes = 1\n", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.config != "" {
				writeConfig(t, dir, tt.config)
			}

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load returned unexpected error: %v", err)
			}
			if cfg.MaxPromptBytes != tt.want {
				t.Errorf("MaxPromptBytes = %d, want %d", cfg.MaxPromptBytes, tt.want)
			}
		})
	}
}

func TestLoad_MaxPromptBytes_NonPositiveRejected(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, "max_prompt_bytes = "+value+"\n")

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for max_prompt_bytes = %s, want non-nil", value)
			}
			const want = "invalid .mint.toml: max_prompt_bytes must be a positive number of bytes"
			if err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestLoad_MaxPromptBytes_NonIntegerRejectedNamingIntegerType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "string", value: `"1500000"`},
		{name: "float", value: "1.5"},
		{name: "boolean", value: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, "max_prompt_bytes = "+tt.value+"\n")

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for max_prompt_bytes = %s, want non-nil", tt.value)
			}
			const want = "invalid .mint.toml: max_prompt_bytes must be an integer (bytes)"
			if err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestLoad_MaxPromptBytes_PerVerbRejectedAsUnknownKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		table string
		want  string
	}{
		{table: "release", want: `invalid .mint.toml: unknown key "max_prompt_bytes" in [release]`},
		{table: "commit", want: `invalid .mint.toml: unknown key "max_prompt_bytes" in [commit]`},
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, "["+tt.table+"]\nmax_prompt_bytes = 800000\n")

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for max_prompt_bytes under [%s], want non-nil", tt.table)
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

// TestMetadataRows_SharedMaxPromptBytesRow pins the shared-only SoT row to the exported
// default constant, so the setup guide's config reference cannot drift from Load.
func TestMetadataRows_SharedMaxPromptBytesRow(t *testing.T) {
	t.Parallel()

	if config.DefaultMaxPromptBytes != 1_500_000 {
		t.Errorf("DefaultMaxPromptBytes = %d, want 1500000", config.DefaultMaxPromptBytes)
	}

	set := configtest.MustByLevelKey(t)
	row, ok := set[configtest.RowKey{Level: config.LevelShared, Key: "max_prompt_bytes"}]
	if !ok {
		t.Fatal("missing SoT row for (shared, max_prompt_bytes)")
	}
	if want := strconv.Itoa(config.DefaultMaxPromptBytes); row.Default != want {
		t.Errorf("(shared, max_prompt_bytes) Default = %q, want %q", row.Default, want)
	}
	const wantDescription = "cap, in bytes, on the composed prompt of any single AI call"
	if row.Description != wantDescription {
		t.Errorf("(shared, max_prompt_bytes) Description = %q, want %q", row.Description, wantDescription)
	}

	for _, level := range []config.MetadataLevel{config.LevelRelease, config.LevelCommit} {
		if _, ok := set[configtest.RowKey{Level: level, Key: "max_prompt_bytes"}]; ok {
			t.Errorf("SoT carries a %q max_prompt_bytes row; the key is shared-only", level)
		}
	}
}

// TestREADME_SharedMaxPromptBytesRowShowsDefault proves the README's shared engine keys
// table lists the per-call byte cap with its shipped default and meaning.
func TestREADME_SharedMaxPromptBytesRowShowsDefault(t *testing.T) {
	t.Parallel()

	table := readmeSection(t, readREADME(t, readmePath(t)), "### Shared engine keys")
	want := "| `max_prompt_bytes` | `" + strconv.Itoa(config.DefaultMaxPromptBytes) + "` | cap, in bytes, on the composed prompt of any single AI call"
	if !strings.Contains(table, want) {
		t.Errorf("README shared engine keys table has no row opening %q", want)
	}
}
