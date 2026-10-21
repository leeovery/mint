package config_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
)

func TestLoad_ReleaseDiffKeep_DecodesAsTheReleaseKeepList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      string
		wantKeep    []string
		wantExclude []string
	}{
		{name: "absent", config: "", wantKeep: nil, wantExclude: nil},
		{name: "keep list only", config: "[release]\ndiff_keep = [\"docs/api/\", \"*.md\"]\n", wantKeep: []string{"docs/api/", "*.md"}, wantExclude: nil},
		{name: "beside an exclusion", config: "[release]\ndiff_exclude = [\"docs/\"]\ndiff_keep = [\"docs/api/\"]\n", wantKeep: []string{"docs/api/"}, wantExclude: []string{"docs/"}},
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
			if !slices.Equal(cfg.Release.DiffKeep, tt.wantKeep) {
				t.Errorf("Release.DiffKeep = %#v, want %#v", cfg.Release.DiffKeep, tt.wantKeep)
			}
			if !slices.Equal(cfg.Release.DiffExclude, tt.wantExclude) {
				t.Errorf("Release.DiffExclude = %#v, want %#v (diff_keep must not touch the exclusions)", cfg.Release.DiffExclude, tt.wantExclude)
			}
		})
	}
}

func TestLoad_ReleaseDiffKeepScalar_RejectedNamingTheReleaseKeyAndArrayType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "[release]\ndiff_keep = \"docs/api/\"\n")

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load returned nil error for a string [release].diff_keep, want non-nil")
	}
	const want = "invalid .mint.toml: release.diff_keep must be an array of strings"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoad_DiffKeepOutsideRelease_RejectedAsAnUnknownKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "top level", config: "diff_keep = [\"docs/api/\"]\n", want: `invalid .mint.toml: unknown top-level key "diff_keep"`},
		{name: "commit table", config: "[commit]\ndiff_keep = [\"docs/api/\"]\n", want: `invalid .mint.toml: unknown key "diff_keep" in [commit]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeConfig(t, dir, tt.config)

			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("Load returned nil error for %s diff_keep, want non-nil", tt.name)
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

const releaseDiffKeepDescription = "pathspec globs whose paths the exclusion check never raises; they add nothing to the release-notes diff and never override an exclusion"

func TestMetadataRows_ReleaseDiffKeepRow(t *testing.T) {
	t.Parallel()

	set := configtest.MustByLevelKey(t)
	row, ok := set[configtest.RowKey{Level: config.LevelRelease, Key: "diff_keep"}]
	if !ok {
		t.Fatal("missing SoT row for ([release], diff_keep)")
	}
	if row.Default != "[]" {
		t.Errorf("([release], diff_keep) Default = %q, want %q", row.Default, "[]")
	}
	if row.Description != releaseDiffKeepDescription {
		t.Errorf("([release], diff_keep) Description = %q, want %q", row.Description, releaseDiffKeepDescription)
	}
}

func TestREADME_ReleaseTableListsDiffKeep(t *testing.T) {
	t.Parallel()

	table := readmeSection(t, readREADME(t, readmePath(t)), "### `[release]`")
	want := "| `diff_keep` | `[]` | pathspec globs whose paths the exclusion check never raises; they add nothing to the release-notes diff and never override an exclusion"
	if !strings.Contains(table, want) {
		t.Errorf("README [release] table has no row opening %q", want)
	}
}
