package notes

import "testing"

// versionFileExcludePathspec is the pure, strategy-aware decision for whether the
// configured version_file is excluded from the diff, and if so as which pathspec.
// It is unexported (an internal assembly detail), so this white-box test exercises
// it directly — the three branches are the whole contract:
//   - plain mode (version_file set, no version_pattern) → EXCLUDE the file (pure
//     bookkeeping: the whole file is the version);
//   - embedded mode (version_file + version_pattern) → do NOT exclude (a real source
//     file we want in notes; the lone version-line bump is neutralised by the
//     prompt's ignore-version-bumps rule, not by hiding source);
//   - no version_file → nothing for this rule.
func TestVersionFileExcludePathspec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		versionFile    string
		versionPattern string
		wantPathspec   string
		wantExclude    bool
	}{
		{
			name:         "plain mode excludes the whole-file version",
			versionFile:  "release.txt",
			wantPathspec: ":(exclude)release.txt",
			wantExclude:  true,
		},
		{
			name:           "embedded mode does not exclude the source file",
			versionFile:    "main.go",
			versionPattern: `RELEASE_VERSION="{version}"`,
			wantPathspec:   "",
			wantExclude:    false,
		},
		{
			name:         "no version_file adds nothing for this rule",
			versionFile:  "",
			wantPathspec: "",
			wantExclude:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPathspec, gotExclude := versionFileExcludePathspec(tt.versionFile, tt.versionPattern)
			if gotExclude != tt.wantExclude {
				t.Errorf("exclude = %v, want %v", gotExclude, tt.wantExclude)
			}
			if gotPathspec != tt.wantPathspec {
				t.Errorf("pathspec = %q, want %q", gotPathspec, tt.wantPathspec)
			}
		})
	}
}
