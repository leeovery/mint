package config_test

import (
	"strings"
	"testing"

	"mint/internal/config"
)

func TestLoad_MaxDiffLines_RejectedNamingItsReplacement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "integer value", config: "max_diff_lines = 50000\n"},
		{name: "non-integer value", config: "max_diff_lines = \"lots\"\n"},
		{name: "beside valid keys", config: "ai_command = \"claude -p\"\nmax_diff_lines = 1200\n\n[release]\ntag_prefix = \"v\"\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tt.config)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatal("Load returned nil error for a max_diff_lines key, want non-nil")
			}
			const want = "invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes"
			if err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestREADME_PerKeyTablesCarryNoMaxDiffLinesRow(t *testing.T) {
	t.Parallel()

	for _, line := range strings.Split(readREADME(t, readmePath(t)), "\n") {
		if strings.HasPrefix(line, "| `max_diff_lines`") {
			t.Errorf("README still carries a max_diff_lines row: %q", line)
		}
	}
}
