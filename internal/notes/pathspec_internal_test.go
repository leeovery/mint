package notes

import "testing"

func TestPathspecMatches_MatchesAsGitMatchesAPathspecWithoutMagic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		spec, path string
		want       bool
	}{
		{spec: "docs/api.md", path: "docs/api.md", want: true},
		{spec: "docs/api", path: "docs/api.md", want: false},
		{spec: "docs/api", path: "docs/api/export.md", want: true},
		{spec: "docs/api/", path: "docs/api/export.md", want: true},
		{spec: "docs/api/", path: "docs/api", want: false},
		{spec: "docs", path: "docsite/index.md", want: false},
		{spec: "*.md", path: "README.md", want: true},
		{spec: "*.md", path: "docs/guide/intro.md", want: true},
		{spec: "*.lock", path: "app.lock", want: true},
		{spec: "*.lock", path: "app.lock.bak", want: false},
		{spec: "docs/*.md", path: "docs/design/a.md", want: true},
		{spec: "docs/*.md", path: "src/docs/a.md", want: false},
		{spec: "docs/*/", path: "docs/design/a.md", want: false},
		{spec: "src/?.go", path: "src/a.go", want: true},
		{spec: "src/?.go", path: "src/ab.go", want: false},
		{spec: "src/[ab].go", path: "src/b.go", want: true},
		{spec: "src/[!ab].go", path: "src/b.go", want: false},
		{spec: "src/[!ab].go", path: "src/c.go", want: true},
		{spec: "src/[^ab].go", path: "src/c.go", want: true},
		{spec: "src/[a-c].go", path: "src/b.go", want: true},
		{spec: "src/[[:digit:]].go", path: "src/7.go", want: true},
		{spec: "src/[[:digit:]].go", path: "src/x.go", want: false},
		{spec: "src/[].go", path: "src/].go", want: false},
		{spec: "src/[]].go", path: "src/].go", want: true},
		{spec: "src/[a", path: "src/[a", want: true},
		{spec: "src/[a*", path: "src/[ab", want: false},
		{spec: `src/\*.go`, path: "src/*.go", want: true},
		{spec: `src/\*.go`, path: "src/a.go", want: false},
		{spec: "a.b", path: "aXb", want: false},
		{spec: "**/fixtures/", path: "test/fixtures/a.json", want: false},
		{spec: "**/fixtures/*", path: "test/fixtures/a.json", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.spec+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			if got := pathspecMatches(tt.spec, tt.path); got != tt.want {
				t.Errorf("pathspecMatches(%q, %q) = %v, want %v", tt.spec, tt.path, got, tt.want)
			}
		})
	}
}
