package record_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

// caseInsensitive reports whether dir's filesystem takes two spellings of a name in
// different case as one file.
func caseInsensitive(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "case-probe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(filepath.Join(dir, "case-probe")) }()
	_, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	return err == nil
}

func spellingArgs(root, path string) []string {
	return []string{"-C", root, "ls-files", "-z", "--", ":(top,icase,literal)" + path}
}

// The version file is named as git spells it where the two differ in case alone and
// the disk takes them as one file: there a literal pathspec of the config's spelling
// matches nothing. Where the disk keeps them apart, the config's spelling names its own
// file and stands.
func TestTrackedSpelling_NamesTheFileAsGitSpellsIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		listed string
		disk   []string
		// want is the spelling on a disk that ignores case; sensitive on one that
		// doesn't.
		want, sensitive string
	}{
		{name: "tracked as spelled", path: "VERSION", listed: "VERSION\x00", disk: []string{"VERSION"}, want: "VERSION", sensitive: "VERSION"},
		{name: "tracked in another case", path: "version.txt", listed: "VERSION.txt\x00", disk: []string{"VERSION.txt"}, want: "VERSION.txt", sensitive: "version.txt"},
		{name: "in a directory, tracked in another case", path: "pkg/version.go", listed: "PKG/Version.go\x00", disk: []string{"PKG/Version.go"}, want: "PKG/Version.go", sensitive: "pkg/version.go"},
		{name: "not tracked", path: "VERSION", listed: "", want: "VERSION", sensitive: "VERSION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			for _, name := range tt.disk {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte("0.0.0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			r := runner.NewFakeRunner()
			r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{Stdout: tt.listed}})

			got, err := record.TrackedSpelling(t.Context(), mutator(r), root, tt.path)
			if err != nil {
				t.Fatalf("TrackedSpelling returned unexpected error: %v", err)
			}
			want := tt.sensitive
			if caseInsensitive(t, root) {
				want = tt.want
			}
			if got != want {
				t.Errorf("TrackedSpelling = %q, want %q", got, want)
			}
			if invs := r.Invocations(); len(invs) != 1 || !slices.Equal(invs[0].Args, spellingArgs(root, tt.path)) {
				t.Errorf("git calls = %v, want the one icase read %q", invs, spellingArgs(root, tt.path))
			}
		})
	}
}

// Where git tracks the file under two spellings the disk takes as one, no one spelling
// is git's, so the run stops before writing, naming both.
func TestTrackedSpelling_TwoSpellingsOfOneFile_IsAnErrorNamingBoth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if !caseInsensitive(t, root) {
		t.Skip("the disk keeps the spellings apart")
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("0.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runner.NewFakeRunner()
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{Stdout: "VERSION\x00Version\x00"}})

	_, err := record.TrackedSpelling(t.Context(), mutator(r), root, "version")
	if err == nil || !strings.Contains(err.Error(), "VERSION and Version") {
		t.Fatalf("TrackedSpelling error = %v, want one naming VERSION and Version", err)
	}
}

func TestTrackedSpelling_NoVersionFile_ReadsNothing(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	got, err := record.TrackedSpelling(t.Context(), mutator(r), t.TempDir(), "")
	if err != nil || got != "" {
		t.Fatalf("TrackedSpelling = %q, %v, want no file and no error", got, err)
	}
	if n := len(r.Invocations()); n != 0 {
		t.Errorf("git calls = %d, want none", n)
	}
}

func TestTrackedSpelling_AFailedRead_CarriesGitsStderr(t *testing.T) {
	t.Parallel()

	const stderr = "fatal: not a git repository\n"
	r := runner.NewFakeRunner()
	r.SeedSequence("git", runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: stderr}, Err: errors.New("exit status 128")})

	_, err := record.TrackedSpelling(t.Context(), mutator(r), t.TempDir(), "VERSION")
	var gitErr *record.GitError
	if !errors.As(err, &gitErr) || gitErr.Stderr != stderr {
		t.Fatalf("TrackedSpelling error = %v, want a *record.GitError carrying %q", err, stderr)
	}
}
