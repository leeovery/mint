package rail_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The rail is a leaf: the presenter draws with it, so it imports no package of
// mint's, the presenter's least of all.
func TestTheRailImportsNoPackageOfMints(t *testing.T) {
	t.Parallel()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, imp := range file.Imports {
			if p := strings.Trim(imp.Path.Value, `"`); p == "mint" || strings.HasPrefix(p, "mint/") {
				t.Errorf("%s imports %s", path, p)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no sources")
	}
}
