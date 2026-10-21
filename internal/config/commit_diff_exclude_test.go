package config_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
)

func TestLoad_CommitDiffExclude_DecodesBesideTheOtherLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      string
		wantShared  []string
		wantRelease []string
		wantCommit  []string
	}{
		{name: "absent", config: ""},
		{name: "commit list only", config: "[commit]\ndiff_exclude = [\"tmp/\", \"*.snap\"]\n", wantCommit: []string{"tmp/", "*.snap"}},
		{
			name:        "all three levels",
			config:      "diff_exclude = [\"*.lock\"]\n\n[release]\ndiff_exclude = [\"specs/\"]\n\n[commit]\ndiff_exclude = [\"tmp/\"]\n",
			wantShared:  []string{"*.lock"},
			wantRelease: []string{"specs/"},
			wantCommit:  []string{"tmp/"},
		},
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
			if !slices.Equal(cfg.DiffExclude, tt.wantShared) {
				t.Errorf("DiffExclude = %#v, want %#v", cfg.DiffExclude, tt.wantShared)
			}
			if !slices.Equal(cfg.Release.DiffExclude, tt.wantRelease) {
				t.Errorf("Release.DiffExclude = %#v, want %#v", cfg.Release.DiffExclude, tt.wantRelease)
			}
			if !slices.Equal(cfg.Commit.DiffExclude, tt.wantCommit) {
				t.Errorf("Commit.DiffExclude = %#v, want %#v", cfg.Commit.DiffExclude, tt.wantCommit)
			}
		})
	}
}

func TestLoad_CommitDiffExcludeScalar_RejectedNamingTheCommitKeyAndArrayType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "[commit]\ndiff_exclude = \"tmp/\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for a string [commit].diff_exclude, want non-nil")
	}
	const want = "invalid .mint.toml: commit.diff_exclude must be an array of strings"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

const commitDiffExcludeDescription = "pathspec globs kept out of the commit-message diff only, added to the shared diff_exclude list"

func TestMetadataRows_CommitDiffExcludeRow(t *testing.T) {
	t.Parallel()

	set := configtest.MustByLevelKey(t)
	row, ok := set[configtest.RowKey{Level: config.LevelCommit, Key: "diff_exclude"}]
	if !ok {
		t.Fatal("missing SoT row for ([commit], diff_exclude)")
	}
	if row.Default != "[]" {
		t.Errorf("([commit], diff_exclude) Default = %q, want %q", row.Default, "[]")
	}
	if row.Description != commitDiffExcludeDescription {
		t.Errorf("([commit], diff_exclude) Description = %q, want %q", row.Description, commitDiffExcludeDescription)
	}
}

func TestREADME_CommitTableListsDiffExclude(t *testing.T) {
	t.Parallel()

	table := readmeSection(t, readREADME(t, readmePath(t)), "### `[commit]`")
	want := "| `diff_exclude` | `[]` | pathspec globs kept out of the commit-message diff only, added to the shared `diff_exclude` list"
	if !strings.Contains(table, want) {
		t.Errorf("README [commit] table has no row opening %q", want)
	}
}
