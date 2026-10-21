package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/config"
)

func TestDiffExcludeLine_LoadsBackAsTheEntriesWithTheGlobAppended(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		existing []string
		glob     string
	}{
		{name: "key unset", glob: "docs/design/"},
		{name: "key set", existing: []string{"specs/", "vendor/**"}, glob: "docs/design/"},
		{name: "quotes and backslashes", existing: []string{`a\b`}, glob: `odd "name"/`},
		{name: "control and non-ASCII bytes", glob: "tab\there/é\x7f/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			toml := "[release]\n" + diffExcludeLine(tc.existing, tc.glob) + "\n"
			if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte(toml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatalf("loading %q: %v", toml, err)
			}
			if want := append(slices.Clone(tc.existing), tc.glob); !slices.Equal(cfg.Release.DiffExclude, want) {
				t.Errorf("[release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
			}
		})
	}
}

func TestDiffExcludeLine_LeavesTheEntriesItWasGivenUntouched(t *testing.T) {
	t.Parallel()

	existing := make([]string, 1, 4)
	existing[0] = "specs/"
	first := diffExcludeLine(existing, "docs/design/")
	second := diffExcludeLine(existing, "tmp/")

	if first != `diff_exclude = ["specs/", "docs/design/"]` || second != `diff_exclude = ["specs/", "tmp/"]` {
		t.Errorf("lines = %q, %q", first, second)
	}
}
