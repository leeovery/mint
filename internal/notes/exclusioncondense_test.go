package notes_test

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/bytesize"
	"mint/internal/config"
	"mint/internal/notes"
)

const (
	vendorFiles     = 4_312
	vendorFileBytes = 8_813
)

// vendorRun is a release adding 4,312 files under vendor/, each diff 8,813 bytes, so
// 38,001,656 bytes in all.
var vendorRun = sync.OnceValue(func() string {
	var b strings.Builder
	for i := range vendorFiles {
		path := fmt.Sprintf("vendor/pkg/file%04d.go", i)
		header := "diff --git a/" + path + " b/" + path + "\nnew file mode 100644\n@@ -0,0 +1 @@\n+"
		b.WriteString(header + strings.Repeat("x", vendorFileBytes-len(header)-1) + "\n")
	}
	return b.String()
})

const vendorSummary = "- vendor/: 4,312 files, 38 MB"

// checkListOf is the list lines of a check prompt.
func checkListOf(prompt string) []string {
	var lines []string
	for line := range strings.Lines(prompt) {
		if strings.HasPrefix(line, "- ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	return lines
}

// listedFiles is sizedFileDiff over n paths named by pattern, each size bytes, and the
// list lines the check gives them.
func listedFiles(t *testing.T, pattern string, n, size int) (diff string, lines []string) {
	t.Helper()
	for i := range n {
		path := fmt.Sprintf(pattern, i)
		section := sizedFileDiff(t, path, size)
		diff += section
		lines = append(lines, fmt.Sprintf("- %s (modified, %s)", path, bytesize.Format(len(section))))
	}
	return diff, lines
}

const condensedClause = "or one folder's changed paths summed up as how many and the total size of their diffs"

// sentList runs the check over diff under maxPromptBytes, the AI answering "none", and
// returns its one call's list lines.
func sentList(t *testing.T, diff string, maxPromptBytes int) []string {
	t.Helper()
	return checkListOf(sentPrompt(t, diff, maxPromptBytes))
}

// sentPrompt runs the check over diff under maxPromptBytes, the AI answering "none",
// and returns its one call's prompt.
func sentPrompt(t *testing.T, diff string, maxPromptBytes int) string {
	t.Helper()
	tr := &recordingTransport{body: "none"}
	if _, err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), diff, maxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	if tr.calls() != 1 {
		t.Fatalf("transport calls = %d, want one", tr.calls())
	}
	if got := len(tr.lastPrompt(t)); got > maxPromptBytes {
		t.Errorf("check prompt = %d bytes, over the %d-byte cap", got, maxPromptBytes)
	}
	return tr.lastPrompt(t)
}

func TestExclusionCheck_AListWithinATenthOfTheCap_ListsEveryPathOnItsOwnLine(t *testing.T) {
	t.Parallel()

	diff, want := listedFiles(t, "vendor/pkg/file%02d.go", 40, 200)

	prompt := sentPrompt(t, diff, config.DefaultMaxPromptBytes)
	if got := checkListOf(prompt); !slices.Equal(got, want) {
		t.Errorf("check list =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if strings.Contains(prompt, condensedClause) {
		t.Errorf("an uncondensed check prompt tells of folder summaries:\n%s", prompt)
	}
}

func TestExclusionCheck_AListOverATenthOfTheCap_CondensesAFolderRunIntoOneSummaryLine(t *testing.T) {
	t.Parallel()

	source := sizedFileDiff(t, "src/main.go", 640)
	diff := source + vendorRun()

	want := []string{"- src/main.go (modified, 640 B)", vendorSummary}
	prompt := sentPrompt(t, diff, config.DefaultMaxPromptBytes)
	if got := checkListOf(prompt); !slices.Equal(got, want) {
		t.Errorf("check list = %q, want %q", got, want)
	}
	wantIntro := "Each line below is one changed path, with its change and the size of its diff, " + condensedClause + ":\n"
	if !strings.Contains(prompt, wantIntro) {
		t.Errorf("condensed check prompt does not introduce its list as %q:\n%s", wantIntro, prompt[:min(len(prompt), 2_000)])
	}
}

func TestExclusionCheck_CondensesTheLargestRunFirst_LeavingASmallerRunListed(t *testing.T) {
	t.Parallel()

	docs, docLines := listedFiles(t, "docs/guide%02d.md", 12, 300)

	want := append(slices.Clone(docLines), vendorSummary)
	if got := sentList(t, docs+vendorRun(), config.DefaultMaxPromptBytes); !slices.Equal(got, want) {
		t.Errorf("check list =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestExclusionCheck_TheSameReleaseCondensesByteIdenticallyOnEveryRun(t *testing.T) {
	t.Parallel()

	// Equal runs under two folders, of which collapsing one is enough: which one
	// collapses must never vary.
	alpha, _ := listedFiles(t, "alpha/file%02d.go", 30, 200)
	beta, _ := listedFiles(t, "beta/file%02d.go", 30, 200)
	diff := alpha + beta
	const capBytes = 20_000

	first := strings.Join(sentList(t, diff, capBytes), "\n")
	for range 20 {
		if got := strings.Join(sentList(t, diff, capBytes), "\n"); got != first {
			t.Fatalf("check list differs between runs:\n%s\nthen\n%s", first, got)
		}
	}
	if !strings.Contains(first, "- alpha/: 30 files, ") && !strings.Contains(first, "- beta/: 30 files, ") {
		t.Errorf("check list did not condense:\n%s", first)
	}
}

func TestExclusionCheck_TheShareIsATenthOfTheConfiguredCap(t *testing.T) {
	t.Parallel()

	diff, lines := listedFiles(t, "vendor/pkg/file%02d.go", 40, 200)
	if got := sentList(t, diff, config.DefaultMaxPromptBytes); !slices.Equal(got, lines) {
		t.Fatalf("under the default cap, check list = %q, want every path", got)
	}

	// The 40-line list takes more than a tenth of a 10,000-byte cap.
	want := []string{"- vendor/: 40 files, " + bytesize.Format(40*200)}
	if got := sentList(t, diff, 10_000); !slices.Equal(got, want) {
		t.Errorf("under a 10,000-byte cap, check list = %q, want %q", got, want)
	}
}

func TestExclusionCheck_AFolderSuggestedFromASummaryLine_PausesOnEveryFileUnderIt(t *testing.T) {
	t.Parallel()

	pauses := &recordedPauses{}
	check := notes.NewExclusionCheck(&recordingTransport{body: "vendor/: vendored dependencies"}, nil, nil).WithPauses(pauses)

	if _, err := check.Check(t.Context(), sizedFileDiff(t, "src/main.go", 640)+vendorRun(), config.DefaultMaxPromptBytes); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if want := []notes.Suggestion{{Glob: "vendor/", Reason: "vendored dependencies"}}; !slices.Equal(pauses.paused, want) {
		t.Fatalf("paused on %+v, want %+v", pauses.paused, want)
	}
	m := pauses.matches[0]
	if len(m.Paths) != vendorFiles {
		t.Errorf("pause counts %d files, want %d", len(m.Paths), vendorFiles)
	}
	if got := bytesize.Format(m.DiffBytes); got != "38 MB" {
		t.Errorf("pause sizes the files at %s (%d bytes), want 38 MB", got, m.DiffBytes)
	}
}

func TestExclusionCheck_AListStillOverItsShareCondensed_IsSentCondensedWhenTheCallFits(t *testing.T) {
	t.Parallel()

	// Top-level paths have no folder to condense under.
	topLevel, topLines := listedFiles(t, "file%03d.txt", 100, 200)
	vendor, vendorLines := listedFiles(t, "vendor/pkg/file%03d.go", 500, 200)
	const capBytes = 20_000
	if full := len(strings.Join(append(slices.Clone(topLines), vendorLines...), "\n")); full <= capBytes {
		t.Fatalf("uncondensed list = %d bytes, want it over the %d-byte cap", full, capBytes)
	}

	want := append(slices.Clone(topLines), "- vendor/: 500 files, "+bytesize.Format(500*200))
	got := sentList(t, topLevel+vendor, capBytes)
	if !slices.Equal(got, want) {
		t.Errorf("check list =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if size := len(strings.Join(got, "\n")); size <= capBytes/10 {
		t.Fatalf("condensed list = %d bytes, want it still over the %d-byte share", size, capBytes/10)
	}
}

func TestExclusionCheck_ACallThatCannotFitUnderTheCapEvenCondensed_IsNotSentAndFailsTheCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		capBytes int
		diff     func(t *testing.T) string
	}{
		{
			name:     "instructions alone over the cap",
			capBytes: 900,
			diff:     func(t *testing.T) string { return sizedFileDiff(t, "src/main.go", 640) },
		},
		{
			// Top-level paths have no folder to condense under.
			name:     "a list condensed as far as it goes",
			capBytes: 3_000,
			diff: func(t *testing.T) string {
				topLevel, _ := listedFiles(t, "file%03d.txt", 100, 200)
				return topLevel
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := &recordingTransport{body: "vendor/: vendored dependencies"}
			narration := &checkNarration{}
			pauses := &recordedPauses{}
			check := notes.NewExclusionCheck(tr, nil, narration).WithPauses(pauses)

			excluded, err := check.Check(t.Context(), tt.diff(t), tt.capBytes)
			if err != nil {
				t.Fatalf("Check returned unexpected error: %v", err)
			}

			if tr.calls() != 0 {
				t.Errorf("transport calls = %d, want none", tr.calls())
			}
			if want := []string{"failed: "}; !slices.Equal(narration.events, want) {
				t.Errorf("narration = %q, want %q", narration.events, want)
			}
			if len(pauses.paused) != 0 || excluded != nil {
				t.Errorf("paused on %+v and excluded %q, want no suggestions", pauses.paused, excluded)
			}
		})
	}
}
