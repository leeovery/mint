package notes_test

import (
	"slices"
	"testing"

	"mint/internal/notes"
)

func TestUnmatched_IsTheSpecsMatchingNoPath(t *testing.T) {
	t.Parallel()

	paths := []string{"cmd/mint/main.go", "internal/notes/size_test.go", "docs/guide.md", "go.sum"}
	for _, tt := range []struct {
		name  string
		specs []string
		want  []string
	}{
		{name: "a wildcard crosses slashes", specs: []string{"*_test.go"}},
		{name: "a directory with its slash", specs: []string{"docs/"}},
		{name: "a directory without it", specs: []string{"docs"}},
		{name: "a file", specs: []string{"go.sum"}},
		{name: "a deleted directory", specs: []string{".github/workflows/"}, want: []string{".github/workflows/"}},
		{name: "a prefix that is no directory", specs: []string{"go.s"}, want: []string{"go.s"}},
		{name: "in specs' order, duplicates kept", specs: []string{".goreleaser.yaml", "docs/", ".github/", ".goreleaser.yaml"}, want: []string{".goreleaser.yaml", ".github/", ".goreleaser.yaml"}},
		{name: "no specs", specs: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := notes.Unmatched(tt.specs, paths); !slices.Equal(got, tt.want) {
				t.Errorf("Unmatched(%q) = %q, want %q", tt.specs, got, tt.want)
			}
		})
	}
}
