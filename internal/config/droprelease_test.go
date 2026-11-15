package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"mint/internal/config"
)

// edited writes file to a fresh root, edits its [release] lists with add and drop,
// and returns the root and the file as it then reads.
func edited(t *testing.T, file string, add, drop config.ReleaseGlobs) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, config.FileName)
	if err := os.WriteFile(path, []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.EditReleaseGlobs(root, add, drop); err != nil {
		t.Fatalf("EditReleaseGlobs returned unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return root, string(got)
}

func TestEditReleaseGlobs_DropsEachNamedEntryLeavingEveryOtherByte(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		file string
		drop config.ReleaseGlobs
		want string
	}{
		{
			name: "the first of a line's entries",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\", \"c/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "[release]\ndiff_exclude = [\"b/\", \"c/\"]\n",
		},
		{
			name: "a middle one",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\", \"c/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "[release]\ndiff_exclude = [\"a/\", \"c/\"]\n",
		},
		{
			name: "the last one",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\", \"c/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"c/"}},
			want: "[release]\ndiff_exclude = [\"a/\", \"b/\"]\n",
		},
		{
			name: "two side by side",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\", \"c/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/", "c/"}},
			want: "[release]\ndiff_exclude = [\"a/\"]\n",
		},
		{
			name: "every one",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/", "b/"}},
			want: "[release]\ndiff_exclude = []\n",
		},
		{
			name: "a line to itself, with its comment",
			file: "[release]\ndiff_exclude = [\n    \"a/\",\n    \"b/\", # retired\n    \"c/\",\n]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "[release]\ndiff_exclude = [\n    \"a/\",\n    \"c/\",\n]\n",
		},
		{
			name: "the last line, with no trailing comma",
			file: "[release]\ndiff_exclude = [\n    \"a/\",\n    \"b/\"\n]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "[release]\ndiff_exclude = [\n    \"a/\",\n]\n",
		},
		{
			name: "one sharing the opening bracket's line",
			file: "[release]\ndiff_exclude = [\"a/\",\n    \"b/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "[release]\ndiff_exclude = [\n    \"b/\"]\n",
		},
		{
			name: "one sharing the closing bracket's line",
			file: "[release]\ndiff_exclude = [\"a/\",\n    \"b/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "[release]\ndiff_exclude = [\"a/\",\n    ]\n",
		},
		{
			name: "the second of a line's two, before more lines",
			file: "[release]\ndiff_exclude = [\n    \"a/\", \"b/\",\n    \"c/\",\n]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "[release]\ndiff_exclude = [\n    \"a/\",\n    \"c/\",\n]\n",
		},
		{
			name: "a duplicate, each time it appears",
			file: "[release]\ndiff_exclude = [\"a/\", \"b/\", \"a/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "[release]\ndiff_exclude = [\"b/\"]\n",
		},
		{
			name: "from the keep list alone",
			file: "[release]\ndiff_exclude = [\"a/\"]\ndiff_keep = [\"a/\", \"k/\"]\n",
			drop: config.ReleaseGlobs{DiffKeep: []string{"a/"}},
			want: "[release]\ndiff_exclude = [\"a/\"]\ndiff_keep = [\"k/\"]\n",
		},
		{
			name: "never from the shared or [commit] lists",
			file: "diff_exclude = [\"a/\"]\n\n[release]\ndiff_exclude = [\"a/\", \"b/\"]\n\n[commit]\ndiff_exclude = [\"a/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "diff_exclude = [\"a/\"]\n\n[release]\ndiff_exclude = [\"b/\"]\n\n[commit]\ndiff_exclude = [\"a/\"]\n",
		},
		{
			name: "an inline [release] table",
			file: "release = { diff_exclude = [\"a/\", \"b/\"] }\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "release = { diff_exclude = [\"b/\"] }\n",
		},
		{
			name: "a dotted key",
			file: "release.diff_exclude = [\"a/\", \"b/\"]\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"b/"}},
			want: "release.diff_exclude = [\"a/\"]\n",
		},
		{
			name: "an entry written with an escape, by its value",
			file: "[release]\ndiff_exclude = [\"a\\\\b\", 'c/']\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{`a\b`, "c/"}},
			want: "[release]\ndiff_exclude = []\n",
		},
		{
			name: "CRLF lines",
			file: "[release]\r\ndiff_exclude = [\r\n    \"a/\",\r\n    \"b/\",\r\n]\r\n",
			drop: config.ReleaseGlobs{DiffExclude: []string{"a/"}},
			want: "[release]\r\ndiff_exclude = [\r\n    \"b/\",\r\n]\r\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, got := edited(t, tt.file, config.ReleaseGlobs{}, tt.drop)
			if got != tt.want {
				t.Errorf("file =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestEditReleaseGlobs_DropsThenAppendsToTheSameList(t *testing.T) {
	t.Parallel()

	root, got := edited(t, "[release]\ndiff_exclude = [\"old/\", \"live/\"]\n",
		config.ReleaseGlobs{DiffExclude: []string{"new/"}},
		config.ReleaseGlobs{DiffExclude: []string{"old/"}})
	if want := "[release]\ndiff_exclude = [\"live/\", \"new/\"]\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if want := []string{"live/", "new/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("loaded diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
}

func TestEditReleaseGlobs_ADropNamingNothingTheListsHold_LeavesTheFileUnwritten(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, config.FileName)
	const file = "[release]\ndiff_exclude = [\"a/\"]\n"
	if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Unix(1_000_000_000, 0)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := config.EditReleaseGlobs(root, config.ReleaseGlobs{}, config.ReleaseGlobs{DiffExclude: []string{"gone/"}, DiffKeep: []string{"gone/"}}); err != nil {
		t.Fatalf("EditReleaseGlobs returned unexpected error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("the file was written (modified %v), want it left as it was", info.ModTime())
	}
}

func TestEditReleaseGlobs_WithNoFileAndNothingToAppend_CreatesNone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := config.EditReleaseGlobs(root, config.ReleaseGlobs{}, config.ReleaseGlobs{DiffExclude: []string{"a/"}}); err != nil {
		t.Fatalf("EditReleaseGlobs returned unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, config.FileName)); !os.IsNotExist(err) {
		t.Errorf("stat error = %v, want no file", err)
	}
}
