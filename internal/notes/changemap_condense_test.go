package notes_test

import (
	"fmt"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// removedRun returns n name-status removal lines spread over subfolders of folder, so
// the run is one leading-directory run regardless of how deep its files sit.
func removedRun(folder string, n int) []string {
	lines := make([]string, 0, n)
	for i := range n {
		lines = append(lines, fmt.Sprintf("D\t%s/github.com/acme/module%02d/internal/pkg/file%05d.go", folder, i%7, i))
	}
	return lines
}

// nameStatusOf joins name-status lines into git's newline-terminated stdout.
func nameStatusOf(lines ...[]string) string {
	var all []string
	for _, l := range lines {
		all = append(all, l...)
	}
	return strings.Join(all, "\n") + "\n"
}

// buildMap runs BuildChangeMap over the scripted name-status (numstat carries one
// modified file) under the given cap.
func buildMap(t *testing.T, nameStatus string, maxPromptBytes int) string {
	t.Helper()
	r := seedChangeMapGit(t, nameStatus, "10\t2\tcore/run.go\n")
	got, err := notes.NewAssembler(r, notes.ExcludeConfig{}).BuildChangeMap(t.Context(), "v1.0.0", maxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	return got
}

// countLinesWithPrefix counts the map lines that start with prefix once indentation
// is trimmed.
func countLinesWithPrefix(changeMap, prefix string) int {
	n := 0
	for _, line := range strings.Split(changeMap, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			n++
		}
	}
	return n
}

func assertWithinShare(t *testing.T, changeMap string, maxPromptBytes int) {
	t.Helper()
	if len(changeMap)*10 > maxPromptBytes {
		t.Errorf("change map is %d bytes, want at most a tenth of the %d-byte cap", len(changeMap), maxPromptBytes)
	}
}

func TestBuildChangeMap_UnderShare_RendersAsBuilt(t *testing.T) {
	t.Parallel()

	nameStatus := nameStatusOf([]string{
		"R096\tdocs/old.md\tdocs/new.md",
		"R100\tdocs/a.md\tdocs/b.md",
		"D\tvendor/x/one.go",
		"D\tvendor/x/two.go",
		"D\tvendor/y/three.go",
		"D\ttop.go",
		"M\tcore/run.go",
	})

	got := buildMap(t, nameStatus, config.DefaultMaxPromptBytes)

	want := "Structural novelty:\n" +
		"  Renamed: docs/old.md -> docs/new.md\n" +
		"  Renamed: docs/a.md -> docs/b.md\n" +
		"  Removed: vendor/x/one.go\n" +
		"  Removed: vendor/x/two.go\n" +
		"  Removed: vendor/y/three.go\n" +
		"  Removed: top.go\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if got != want {
		t.Errorf("change map =\n%s\nwant the as-built rendering\n%s", got, want)
	}
}

func TestBuildChangeMap_OverShare_CollapsesVendorRemovalToOneLine(t *testing.T) {
	t.Parallel()

	nameStatus := nameStatusOf(removedRun("vendor", 4312), []string{"M\tcore/run.go"})
	full := buildMap(t, nameStatus, 1<<40)
	if len(full)*10 <= config.DefaultMaxPromptBytes {
		t.Fatalf("fixture map is %d bytes, want over a tenth of the default cap", len(full))
	}

	got := buildMap(t, nameStatus, config.DefaultMaxPromptBytes)

	if n := countLinesWithPrefix(got, "Removed: vendor/"); n != 0 {
		t.Errorf("condensed map still lists %d per-path vendor/ removals:\n%s", n, got)
	}
	if n := countLinesWithPrefix(got, "Removed: 4,312 files under vendor/"); n != 1 {
		t.Errorf("summary line count = %d, want exactly 1:\n%s", n, got)
	}
	assertWithinShare(t, got, config.DefaultMaxPromptBytes)
}

func TestBuildChangeMap_OverShare_CollapsesRenameRunToOneLine(t *testing.T) {
	t.Parallel()

	renames := make([]string, 0, 50)
	for i := range 50 {
		renames = append(renames, fmt.Sprintf("R100\tlegacy/handlers/h%02d.go\tpkg/handlers/h%02d.go", i, i))
	}
	const maxPromptBytes = 2000

	got := buildMap(t, nameStatusOf(renames, []string{"M\tcore/run.go"}), maxPromptBytes)

	want := "Structural novelty:\n" +
		"  Renamed: 50 files under pkg/\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if got != want {
		t.Errorf("change map =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildChangeMap_OverShare_CollapsesLargestRunFirstAndStopsUnderShare(t *testing.T) {
	t.Parallel()

	docs := make([]string, 0, 12)
	for i := range 12 {
		docs = append(docs, fmt.Sprintf("D\tdocs/page%02d.md", i))
	}
	nameStatus := nameStatusOf(docs, removedRun("vendor", 4312), []string{"M\tcore/run.go"})

	got := buildMap(t, nameStatus, config.DefaultMaxPromptBytes)

	if n := countLinesWithPrefix(got, "Removed: 4,312 files under vendor/"); n != 1 {
		t.Errorf("vendor/ summary line count = %d, want 1:\n%s", n, got)
	}
	for i := range 12 {
		line := fmt.Sprintf("  Removed: docs/page%02d.md\n", i)
		if !strings.Contains(got, line) {
			t.Errorf("docs/ removal %q not listed on its own line:\n%s", line, got)
		}
	}
	if strings.Contains(got, "files under docs/") {
		t.Errorf("docs/ run collapsed although vendor/ alone brought the map under its share:\n%s", got)
	}
}

func TestBuildChangeMap_OverShare_SummaryTakesTheRunsPlace(t *testing.T) {
	t.Parallel()

	nameStatus := nameStatusOf([]string{
		"D\ta/one.go",
		"D\tbig/aaaaaaaaaaaaaaaaaaaa/1.go",
		"D\tbig/aaaaaaaaaaaaaaaaaaaa/2.go",
		"D\tbig/aaaaaaaaaaaaaaaaaaaa/3.go",
		"D\tsmall/x.go",
		"D\tsmall/y.go",
		"D\ttop.go",
		"M\tcore/run.go",
	})
	full := buildMap(t, nameStatus, 1<<40)
	// A cap whose share sits just under the full map: collapsing big/ is enough.
	maxPromptBytes := (len(full) - 1) * 10

	got := buildMap(t, nameStatus, maxPromptBytes)

	want := "Structural novelty:\n" +
		"  Removed: a/one.go\n" +
		"  Removed: 3 files under big/\n" +
		"  Removed: small/x.go\n" +
		"  Removed: small/y.go\n" +
		"  Removed: top.go\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if got != want {
		t.Errorf("change map =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildChangeMap_OverShare_TiedRunsCondenseIdentically(t *testing.T) {
	t.Parallel()

	var lines []string
	for i := range 100 {
		lines = append(lines, fmt.Sprintf("D\taaa/x%03d.go", i))
	}
	for i := range 100 {
		lines = append(lines, fmt.Sprintf("D\tbbb/y%03d.go", i))
	}
	nameStatus := nameStatusOf(lines)
	full := buildMap(t, nameStatus, 1<<40)
	// One collapse is enough, so the tie between the equal-sized runs decides the map.
	maxPromptBytes := (len(full) - 1) * 10

	first := buildMap(t, nameStatus, maxPromptBytes)
	second := buildMap(t, nameStatus, maxPromptBytes)

	bbb := make([]string, 0, 100)
	for i := range 100 {
		bbb = append(bbb, fmt.Sprintf("  Removed: bbb/y%03d.go", i))
	}
	want := "Structural novelty:\n" +
		"  Removed: 100 files under aaa/\n" +
		strings.Join(bbb, "\n") + "\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if first != want {
		t.Errorf("change map =\n%s\nwant\n%s", first, want)
	}
	if second != first {
		t.Errorf("two runs over the same release differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestBuildChangeMap_ConfiguredCap_SetsTheShare(t *testing.T) {
	t.Parallel()

	nameStatus := nameStatusOf(removedRun("vendor", 40), []string{"M\tcore/run.go"})

	atDefault := buildMap(t, nameStatus, config.DefaultMaxPromptBytes)
	if n := countLinesWithPrefix(atDefault, "Removed: vendor/"); n != 40 {
		t.Fatalf("at the default cap the map lists %d removals, want all 40 in full:\n%s", n, atDefault)
	}

	smallCap := len(atDefault) * 5
	got := buildMap(t, nameStatus, smallCap)

	if n := countLinesWithPrefix(got, "Removed: 40 files under vendor/"); n != 1 {
		t.Errorf("under a %d-byte cap the summary line count = %d, want 1:\n%s", smallCap, n, got)
	}
	assertWithinShare(t, got, smallCap)
}

func TestBuildChangeMap_OverShareEvenCondensed_CollapsesEveryRun(t *testing.T) {
	t.Parallel()

	nameStatus := nameStatusOf([]string{
		"D\ta/1.go",
		"D\ta/2.go",
		"D\tb/1.go",
		"D\tb/2.go",
		"D\tb/3.go",
		"D\ttop.go",
		"M\tcore/run.go",
	})

	got := buildMap(t, nameStatus, 10)

	want := "Structural novelty:\n" +
		"  Removed: 2 files under a/\n" +
		"  Removed: 3 files under b/\n" +
		"  Removed: top.go\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if got != want {
		t.Errorf("change map =\n%s\nwant\n%s", got, want)
	}
}

func TestGenerator_Generate_SinglePassCarriesCondensedMap(t *testing.T) {
	t.Parallel()

	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-old\n+new\n"
	nameStatus := nameStatusOf(removedRun("vendor", 4312), []string{"M\tcore/run.go"})
	r := seedNormalPathGit(t, diff, nameStatus, "10\t2\tcore/run.go\n")
	transport := &recordingTransport{body: "notes"}
	cfg := config.Config{MaxDiffLines: 50000, MaxPromptBytes: config.DefaultMaxPromptBytes}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	prompt := transport.lastPrompt(t)
	if len(prompt) > cfg.MaxPromptBytes {
		t.Fatalf("fixture prompt is %d bytes, want it under the cap", len(prompt))
	}
	if !strings.Contains(prompt, "\n  Removed: 4,312 files under vendor/\n") {
		t.Errorf("single-pass prompt does not carry the condensed map summary line")
	}
	if strings.Contains(prompt, "Removed: vendor/") {
		t.Errorf("single-pass prompt still carries per-path vendor/ removals")
	}
}

func TestBuildChangeMap_OverShareEvenCondensed_SingleEntryFolderStaysListed(t *testing.T) {
	t.Parallel()

	const lone = "vendor/github.com/acme/module/internal/pkg/deeply/nested/file.go"
	got := buildMap(t, nameStatusOf([]string{"D\t" + lone, "M\tcore/run.go"}), 10)

	want := "Structural novelty:\n" +
		"  Removed: " + lone + "\n" +
		"Churn by area (largest first):\n" +
		"  core/: 12 lines\n" +
		"Notable files:\n" +
		"  Largest file: core/run.go (12 lines)"
	if got != want {
		t.Errorf("change map =\n%s\nwant\n%s", got, want)
	}
}

func TestGenerator_Generate_ConfiguredCapSetsTheMapsShare(t *testing.T) {
	t.Parallel()

	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-old\n+new\n"
	nameStatus := nameStatusOf(removedRun("vendor", 40), []string{"M\tcore/run.go"})
	r := seedNormalPathGit(t, diff, nameStatus, "10\t2\tcore/run.go\n")
	transport := &recordingTransport{body: "notes"}
	cfg := config.Config{MaxDiffLines: 50000, MaxPromptBytes: 20_000}

	gen := notes.NewGenerator(notes.NewAssembler(r, notes.ExcludeConfig{}), transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	prompt := transport.lastPrompt(t)
	if len(prompt) > cfg.MaxPromptBytes {
		t.Fatalf("fixture prompt is %d bytes, want it under the cap", len(prompt))
	}
	if !strings.Contains(prompt, "\n  Removed: 40 files under vendor/\n") {
		t.Errorf("prompt does not carry the map condensed under the configured cap")
	}
	if strings.Contains(prompt, "Removed: vendor/") {
		t.Errorf("prompt still carries per-path vendor/ removals")
	}
}
