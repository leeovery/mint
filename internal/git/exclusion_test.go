package git_test

import (
	"slices"
	"testing"

	"mint/internal/git"
)

func TestExclusion_Pathspec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		exclusion git.Exclusion
		want      []string
	}{
		{name: "the zero value adds no arguments", exclusion: "", want: nil},
		{name: "a root-level file", exclusion: "notes.md", want: []string{"--", ":(top,literal,exclude)notes.md"}},
		{name: "a nested path with glob characters is matched literally", exclusion: "docs/notes [draft]*.md", want: []string{"--", ":(top,literal,exclude)docs/notes [draft]*.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.exclusion.Pathspec(); !slices.Equal(got, tt.want) {
				t.Errorf("Pathspec() = %q, want %q", got, tt.want)
			}
		})
	}
}
