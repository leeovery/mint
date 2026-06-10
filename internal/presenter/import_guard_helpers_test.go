package presenter_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// assertImportsExclude is the shared parse-and-scan engine behind the package's
// dependency guards (the plain UI-library guard and the render-only subprocess
// guard). It parses each source with go/parser in ImportsOnly mode and fails if
// any import path matches one of markers. The exact flag selects the match mode:
// exact-equality (exact=true) when a marker names a precise package
// (e.g. "os/exec"), or substring (exact=false) when a marker is a path fragment
// that may appear anywhere in the import path (e.g. "lipgloss"). The scanned==0
// defence lives here so BOTH guards inherit it: an empty source glob is a false
// positive, not a pass.
//
// Failures stay diagnostic: each t.Errorf reports the file, the offending import
// path, the matched marker, and the match mode, so a caller-specific message is
// unnecessary to tell the two guards apart.
func assertImportsExclude(t *testing.T, sources []string, markers []string, exact bool) {
	t.Helper()

	mode := "substring"
	if exact {
		mode = "exact"
	}

	fset := token.NewFileSet()
	scanned := 0
	for _, path := range sources {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		scanned++
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, marker := range markers {
				matched := strings.Contains(p, marker)
				if exact {
					matched = p == marker
				}
				if matched {
					t.Errorf("%s imports %q which matches banned marker %q (%s match)", filepath.Base(path), p, marker, mode)
				}
			}
		}
	}
	// Defend against a glob/parse regression silently scanning nothing — an empty
	// source set must fail, not pass as a false positive.
	if scanned == 0 {
		t.Fatal("scanned no sources — the import guard never ran")
	}
}
