package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/config"
)

// appended writes file (none when absent) to a fresh root, appends globs, and returns
// the root and the file as it then reads.
func appended(t *testing.T, file *string, globs config.ReleaseGlobs) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, config.FileName)
	if file != nil {
		if err := os.WriteFile(path, []byte(*file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := config.AppendReleaseGlobs(root, globs); err != nil {
		t.Fatalf("AppendReleaseGlobs returned unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return root, string(got)
}

func written(s string) *string { return &s }

func loaded(t *testing.T, root string) config.Config {
	t.Helper()
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("the written .mint.toml does not load: %v", err)
	}
	return cfg
}

func TestAppendReleaseGlobs_AppendsToKeysTheReleaseTableAlreadySets(t *testing.T) {
	t.Parallel()

	root, got := appended(t, written("[release]\ndiff_exclude = [\"specs/\"]\ndiff_keep = [\"docs/api/\"]\n"),
		config.ReleaseGlobs{DiffExclude: []string{"docs/design/"}, DiffKeep: []string{"test/fixtures/"}})

	want := "[release]\ndiff_exclude = [\"specs/\", \"docs/design/\"]\ndiff_keep = [\"docs/api/\", \"test/fixtures/\"]\n"
	if got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	cfg := loaded(t, root)
	if want := []string{"specs/", "docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if want := []string{"docs/api/", "test/fixtures/"}; !slices.Equal(cfg.Release.DiffKeep, want) {
		t.Errorf("[release].diff_keep = %q, want %q", cfg.Release.DiffKeep, want)
	}
}

func TestAppendReleaseGlobs_AddsAMissingKeyToTheExistingReleaseTable(t *testing.T) {
	t.Parallel()

	root, got := appended(t, written("[release]\n"+
		"version_file = \"VERSION\"\n"+
		"version_pattern = '^(\\d+\\.\\d+\\.\\d+)$'\n"+
		"\n"+
		"# The commit verb.\n"+
		"[commit]\n"+
		"context = \"Go CLI\"\n"),
		config.ReleaseGlobs{DiffExclude: []string{"docs/design/"}})

	want := "[release]\n" +
		"version_file = \"VERSION\"\n" +
		"version_pattern = '^(\\d+\\.\\d+\\.\\d+)$'\n" +
		"diff_exclude = [\"docs/design/\"]\n" +
		"\n" +
		"# The commit verb.\n" +
		"[commit]\n" +
		"context = \"Go CLI\"\n"
	if got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	cfg := loaded(t, root)
	if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if cfg.Release.DiffKeep != nil {
		t.Errorf("[release].diff_keep = %q, want it unset", cfg.Release.DiffKeep)
	}
}

func TestAppendReleaseGlobs_AddsAReleaseTableToAFileWithNone(t *testing.T) {
	t.Parallel()

	file := "# Paths no AI diff should read.\n" +
		"diff_exclude = [\n" +
		"  \"go.sum\",\n" +
		"  \"dist/\",\n" +
		"]\n" +
		"\n" +
		"[release.hooks]\n" +
		"pre_tag = \"make build\"\n"
	root, got := appended(t, written(file), config.ReleaseGlobs{DiffExclude: []string{"docs/design/"}})

	if want := file + "\n[release]\ndiff_exclude = [\"docs/design/\"]\n"; got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	cfg := loaded(t, root)
	if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if want := []string{"go.sum", "dist/"}; !slices.Equal(cfg.DiffExclude, want) {
		t.Errorf("shared diff_exclude = %q, want %q", cfg.DiffExclude, want)
	}
	if cfg.Release.Hooks.PreTag != "make build" {
		t.Errorf("[release.hooks].pre_tag = %v, want make build", cfg.Release.Hooks.PreTag)
	}
}

func TestAppendReleaseGlobs_WithNoFile_CreatesItHoldingOnlyTheReleaseTable(t *testing.T) {
	t.Parallel()

	root, got := appended(t, nil, config.ReleaseGlobs{DiffKeep: []string{"test/fixtures/"}})

	if want := "[release]\ndiff_keep = [\"test/fixtures/\"]\n"; got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	if want := []string{"test/fixtures/"}; !slices.Equal(loaded(t, root).Release.DiffKeep, want) {
		t.Errorf("[release].diff_keep = %q, want %q", loaded(t, root).Release.DiffKeep, want)
	}
}

func TestAppendReleaseGlobs_LeavesEveryCommentBlankLineAndAlignmentInPlace(t *testing.T) {
	t.Parallel()

	head := "# mint config for this repo\n" +
		"# kept by hand\n" +
		"\n" +
		"ai_command   = \"claude -p\"   # the shared command\n" +
		"timeout      = 90\n" +
		"\n" +
		"# Shared: lockfiles and build output.\n" +
		"diff_exclude = [\"go.sum\"]\n" +
		"\n" +
		"[release]\n" +
		"tag_prefix   = \"v\"\n" +
		"# Not for the public notes:\n" +
		"diff_exclude = [\n" +
		"    # planning docs\n" +
		"    \"specs/\",   # specs\n" +
		"    \"plans/\",\n"
	tail := "    # agent transcripts\n" +
		"]\n" +
		"changelog    = true\n" +
		"\n" +
		"[release.hooks]\n" +
		"pre_tag = \"make\"   # build first\n"
	root, got := appended(t, written(head+tail), config.ReleaseGlobs{DiffExclude: []string{"docs/design/", "tmp/"}})

	if want := head + "    \"docs/design/\",\n    \"tmp/\",\n" + tail; got != want {
		t.Errorf(".mint.toml =\n%s\nwant\n%s", got, want)
	}
	cfg := loaded(t, root)
	if want := []string{"specs/", "plans/", "docs/design/", "tmp/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if want := []string{"go.sum"}; !slices.Equal(cfg.DiffExclude, want) {
		t.Errorf("shared diff_exclude = %q, want %q", cfg.DiffExclude, want)
	}
}

func TestAppendReleaseGlobs_TakesTheReleaseTableInTheFormItIsWritten(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, file, want string
	}{
		{
			name: "inline",
			file: "# config\nrelease = { tag_prefix = \"v\" }\n\n[commit]\ncontext = \"Go CLI\"\n",
			want: "# config\nrelease = { tag_prefix = \"v\", diff_exclude = [\"docs/design/\"] }\n\n[commit]\ncontext = \"Go CLI\"\n",
		},
		{
			name: "inline and empty",
			file: "release = {}\n",
			want: "release = { diff_exclude = [\"docs/design/\"] }\n",
		},
		{
			name: "inline with the key set",
			file: "release = { diff_exclude = [\"specs/\"], tag_prefix = \"v\" }\n",
			want: "release = { diff_exclude = [\"specs/\", \"docs/design/\"], tag_prefix = \"v\" }\n",
		},
		{
			name: "dotted",
			file: "release.version_file = \"VERSION\"\nrelease.tag_prefix = \"v\"   # tags\n\n[commit]\ncontext = \"Go CLI\"\n",
			want: "release.version_file = \"VERSION\"\nrelease.tag_prefix = \"v\"   # tags\nrelease.diff_exclude = [\"docs/design/\"]\n\n[commit]\ncontext = \"Go CLI\"\n",
		},
		{
			name: "dotted with the key set",
			file: "release.diff_exclude = [\"specs/\"]\nrelease.version_file = \"VERSION\"\n",
			want: "release.diff_exclude = [\"specs/\", \"docs/design/\"]\nrelease.version_file = \"VERSION\"\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root, got := appended(t, written(tc.file), config.ReleaseGlobs{DiffExclude: []string{"docs/design/"}})

			if got != tc.want {
				t.Errorf(".mint.toml =\n%s\nwant\n%s", got, tc.want)
			}
			if diffExclude := loaded(t, root).Release.DiffExclude; diffExclude[len(diffExclude)-1] != "docs/design/" {
				t.Errorf("[release].diff_exclude = %q, want it to end with docs/design/", diffExclude)
			}
		})
	}
}

func TestAppendReleaseGlobs_ShapesOfAnExistingList(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, file, want string
	}{
		{
			name: "empty",
			file: "[release]\ndiff_exclude = []\n",
			want: "[release]\ndiff_exclude = [\"docs/design/\", \"tmp/\"]\n",
		},
		{
			name: "one line with a trailing comma",
			file: "[release]\ndiff_exclude = [\"specs/\",]\n",
			want: "[release]\ndiff_exclude = [\"specs/\", \"docs/design/\", \"tmp/\",]\n",
		},
		{
			name: "several lines, the last entry with no comma and a comment",
			file: "[release]\ndiff_exclude = [\n  \"specs/\",\n  \"plans/\"  # plans\n]\n",
			want: "[release]\ndiff_exclude = [\n  \"specs/\",\n  \"plans/\",  # plans\n  \"docs/design/\",\n  \"tmp/\"\n]\n",
		},
		{
			name: "several lines, closed on the last entry's line",
			file: "[release]\ndiff_exclude = [\n  \"specs/\",\n  \"plans/\"]\n",
			want: "[release]\ndiff_exclude = [\n  \"specs/\",\n  \"plans/\", \"docs/design/\", \"tmp/\"]\n",
		},
		{
			name: "CRLF line ends",
			file: "[release]\r\ndiff_exclude = [\r\n  \"specs/\",\r\n]\r\n",
			want: "[release]\r\ndiff_exclude = [\r\n  \"specs/\",\r\n  \"docs/design/\",\r\n  \"tmp/\",\r\n]\r\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root, got := appended(t, written(tc.file), config.ReleaseGlobs{DiffExclude: []string{"docs/design/", "tmp/"}})

			if got != tc.want {
				t.Errorf(".mint.toml =\n%q\nwant\n%q", got, tc.want)
			}
			loaded(t, root)
		})
	}
}

func TestAppendReleaseGlobs_ShapesOfAReleaseTableWithoutTheKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, file, want string
	}{
		{
			name: "no keys, another table after",
			file: "[release]  # the release verb\n\n[commit]\ncontext = \"Go CLI\"\n",
			want: "[release]  # the release verb\ndiff_exclude = [\"docs/design/\"]\ndiff_keep = [\"test/fixtures/\"]\n\n[commit]\ncontext = \"Go CLI\"\n",
		},
		{
			name: "indented keys, no final newline",
			file: "[release]\n  tag_prefix = \"v\"",
			want: "[release]\n  tag_prefix = \"v\"\n  diff_exclude = [\"docs/design/\"]\n  diff_keep = [\"test/fixtures/\"]",
		},
		{
			name: "after its own sub-table",
			file: "[release.hooks]\npre_tag = \"make\"\n\n[release]\ntag_prefix = \"v\"\n",
			want: "[release.hooks]\npre_tag = \"make\"\n\n[release]\ntag_prefix = \"v\"\ndiff_exclude = [\"docs/design/\"]\ndiff_keep = [\"test/fixtures/\"]\n",
		},
		{
			name: "a file with no final newline and no table",
			file: "timeout = 90",
			want: "timeout = 90\n\n[release]\ndiff_exclude = [\"docs/design/\"]\ndiff_keep = [\"test/fixtures/\"]\n",
		},
		{
			name: "an empty file",
			file: "",
			want: "[release]\ndiff_exclude = [\"docs/design/\"]\ndiff_keep = [\"test/fixtures/\"]\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root, got := appended(t, written(tc.file),
				config.ReleaseGlobs{DiffExclude: []string{"docs/design/"}, DiffKeep: []string{"test/fixtures/"}})

			if got != tc.want {
				t.Errorf(".mint.toml =\n%q\nwant\n%q", got, tc.want)
			}
			cfg := loaded(t, root)
			if !slices.Equal(cfg.Release.DiffExclude, []string{"docs/design/"}) || !slices.Equal(cfg.Release.DiffKeep, []string{"test/fixtures/"}) {
				t.Errorf("[release] lists = %q, %q", cfg.Release.DiffExclude, cfg.Release.DiffKeep)
			}
		})
	}
}

func TestAppendReleaseGlobs_GlobsLoadBackAsWritten(t *testing.T) {
	t.Parallel()

	globs := []string{`odd "name"/`, `back\slash/`, "tab\there/é/"}
	root, _ := appended(t, written("[release]\ndiff_keep = ['lit\\eral/']\n"), config.ReleaseGlobs{DiffKeep: globs})

	if want := append([]string{`lit\eral/`}, globs...); !slices.Equal(loaded(t, root).Release.DiffKeep, want) {
		t.Errorf("[release].diff_keep = %q, want %q", loaded(t, root).Release.DiffKeep, want)
	}
}

func TestAppendReleaseGlobs_KeepsTheFilesMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, config.FileName)
	if err := os.WriteFile(path, []byte("[release]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.AppendReleaseGlobs(root, config.ReleaseGlobs{DiffKeep: []string{"x/"}}); err != nil {
		t.Fatalf("AppendReleaseGlobs returned unexpected error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAppendReleaseGlobs_AFileThatDoesNotParse_IsAnErrorAndLeftAsItWas(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, config.FileName)
	broken := "[release\ntag_prefix = \"v\"\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := config.AppendReleaseGlobs(root, config.ReleaseGlobs{DiffKeep: []string{"x/"}}); err == nil {
		t.Fatal("AppendReleaseGlobs returned no error")
	}
	if got, _ := os.ReadFile(path); string(got) != broken {
		t.Errorf(".mint.toml =\n%s\nwant it left as it was", got)
	}
}
