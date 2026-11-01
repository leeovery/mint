package record_test

import (
	"testing"

	"mint/internal/record"
)

// A version_file is named as git names it, so a root-anchored literal pathspec matches
// the file the projection writes.
func TestRepoPath_NamesTheFileAsGitDoesFromTheRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{path: "", want: ""},
		{path: "VERSION", want: "VERSION"},
		{path: "./package.json", want: "package.json"},
		{path: "sub/../VERSION", want: "VERSION"},
		{path: "/VERSION", want: "VERSION"},
		{path: "pkg//sub/./VERSION", want: "pkg/sub/VERSION"},
	}
	for _, tt := range tests {
		if got := record.RepoPath("/repo/root", tt.path); got != tt.want {
			t.Errorf("RepoPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
