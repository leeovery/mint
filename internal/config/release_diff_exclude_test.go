package config_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
)

func TestLoad_ReleaseDiffExclude_DecodesBesideTheSharedList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      string
		wantShared  []string
		wantRelease []string
	}{
		{name: "absent", config: "", wantShared: nil, wantRelease: nil},
		{name: "release list only", config: "[release]\ndiff_exclude = [\"specs/\", \".workflows/\"]\n", wantShared: nil, wantRelease: []string{"specs/", ".workflows/"}},
		{name: "both lists", config: "diff_exclude = [\"*.lock\"]\n\n[release]\ndiff_exclude = [\"specs/\"]\n", wantShared: []string{"*.lock"}, wantRelease: []string{"specs/"}},
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
		})
	}
}

func TestLoad_ReleaseDiffExcludeScalar_RejectedNamingTheReleaseKeyAndArrayType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "[release]\ndiff_exclude = \"specs/\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for a string [release].diff_exclude, want non-nil")
	}
	const want = "invalid .mint.toml: release.diff_exclude must be an array of strings"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

const releaseDiffExcludeDescription = "pathspec globs kept out of the release-notes diff only, added to the shared diff_exclude list"

func TestMetadataRows_ReleaseDiffExcludeRow(t *testing.T) {
	t.Parallel()

	set := configtest.MustByLevelKey(t)
	row, ok := set[configtest.RowKey{Level: config.LevelRelease, Key: "diff_exclude"}]
	if !ok {
		t.Fatal("missing SoT row for ([release], diff_exclude)")
	}
	if row.Default != "[]" {
		t.Errorf("([release], diff_exclude) Default = %q, want %q", row.Default, "[]")
	}
	if row.Description != releaseDiffExcludeDescription {
		t.Errorf("([release], diff_exclude) Description = %q, want %q", row.Description, releaseDiffExcludeDescription)
	}
}

func TestREADME_ReleaseTableListsDiffExclude(t *testing.T) {
	t.Parallel()

	table := readmeSection(t, readREADME(t, readmePath(t)), "### `[release]`")
	want := "| `diff_exclude` | `[]` | pathspec globs kept out of the release-notes diff only, added to the shared `diff_exclude` list"
	if !strings.Contains(table, want) {
		t.Errorf("README [release] table has no row opening %q", want)
	}
}

func TestREADME_SharedEngineKeysDoNotCallDiffExcludeSharedOnly(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "### Shared engine keys")
	flat := strings.Join(strings.Fields(section), " ")
	if strings.Contains(flat, "the rest are shared-only") {
		t.Error("README shared engine keys section still calls every other key shared-only, diff_exclude included")
	}
	want := "`diff_exclude` has a shared list plus a `[release]` and a `[commit]` list, and each verb's list adds to the shared one rather than replacing it"
	if !strings.Contains(flat, want) {
		t.Errorf("README shared engine keys section does not say %q", want)
	}
}
