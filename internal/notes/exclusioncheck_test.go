package notes_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/notes"
)

// modifiedFileDiff is a modified file's diff section for path, carrying body as its one hunk.
func modifiedFileDiff(path, body string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/" + path + "\n" +
		"+++ b/" + path + "\n" +
		"@@ -1 +1 @@\n" + body
}

// addedFileDiff is a new file's diff section for path, carrying body as its one hunk.
func addedFileDiff(path, body string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"new file mode 100644\n" +
		"index 0000000..2222222\n" +
		"--- /dev/null\n" +
		"+++ b/" + path + "\n" +
		"@@ -0,0 +1 @@\n" + body
}

func deletedFileDiff(path, body string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"deleted file mode 100644\n" +
		"index 2222222..0000000\n" +
		"--- a/" + path + "\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" + body
}

func renamedFileDiff(from, to string) string {
	return "diff --git a/" + from + " b/" + to + "\n" +
		"similarity index 100%\n" +
		"rename from " + from + "\n" +
		"rename to " + to + "\n"
}

// sized pads a hunk line so section's whole diff is exactly bytes long.
func sized(t *testing.T, section func(body string) string, bytes int) string {
	t.Helper()
	pad := bytes - len(section("+\n"))
	if pad < 0 {
		t.Fatalf("a %d-byte section is smaller than its headers", bytes)
	}
	return section("+" + strings.Repeat("x", pad) + "\n")
}

type checkNarration struct{ events []string }

func (n *checkNarration) Checking() { n.events = append(n.events, "checking") }
func (n *checkNarration) Checked()  { n.events = append(n.events, "checked") }

const checkPromptOpening = "A software release's notes are about to be written from its diff."

func checkPrompt(lines ...string) string {
	return checkPromptOpening + " Release notes are a public list of what the release holds, for the people who use it. " +
		"Before they are written, check the release's changed paths for any that do not belong in its notes: " +
		"files about how the software is made rather than what it does, such as design documents, specs and plans, " +
		"tests and their fixtures, agent and tooling configuration, transcripts, and generated or vendored files.\n" +
		"\n" +
		"Each line below is one changed path, with its change and the size of its diff:\n" +
		"\n" +
		strings.Join(lines, "\n") + "\n" +
		"\n" +
		"Suggest git pathspec globs that cover the paths that do not belong. A glob ending in / covers a whole folder, " +
		"and * matches across folders. Prefer one folder glob to a glob per file where a folder holds them.\n" +
		"\n" +
		"Answer with one suggestion per line, written as the glob, a colon and a space, then a short reason, and nothing else. " +
		"If every path belongs in the notes, answer with exactly: none"
}

func TestExclusionCheck_ListsEachChangedPathWithItsChangeAndDiffSize_AndNoDiffContent(t *testing.T) {
	t.Parallel()

	exportGo := modifiedFileDiff("src/export.go", "-func Export() {}\n+func Export() error { return nil }\n")
	designDoc := addedFileDiff("docs/design/export.md", "+# Export design\n")
	tr := &recordingTransport{body: "none"}
	check := notes.NewExclusionCheck(tr, nil, nil)

	if err := check.Check(t.Context(), exportGo+designDoc); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if tr.calls() != 1 {
		t.Fatalf("transport calls = %d, want one", tr.calls())
	}
	want := checkPrompt(
		fmt.Sprintf("- docs/design/export.md (added, %d B)", len(designDoc)),
		fmt.Sprintf("- src/export.go (modified, %d B)", len(exportGo)),
	)
	if got := tr.lastPrompt(t); got != want {
		t.Errorf("check prompt =\n%s\nwant\n%s", got, want)
	}
	for _, content := range []string{"func Export", "# Export design", "@@"} {
		if strings.Contains(tr.lastPrompt(t), content) {
			t.Errorf("check prompt carries diff content %q", content)
		}
	}
}

func TestExclusionCheck_NamesDeletionsAndRenames(t *testing.T) {
	t.Parallel()

	removed := deletedFileDiff("old/legacy.go", "-package old\n")
	moved := renamedFileDiff("pkg/a.go", "pkg/b.go")
	tr := &recordingTransport{body: "none"}

	if err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), removed+moved); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	want := checkPrompt(
		fmt.Sprintf("- old/legacy.go (deleted, %d B)", len(removed)),
		fmt.Sprintf("- pkg/b.go (renamed from pkg/a.go, %d B)", len(moved)),
	)
	if got := tr.lastPrompt(t); got != want {
		t.Errorf("check prompt =\n%s\nwant\n%s", got, want)
	}
}

func TestExclusionCheck_BigFilesAreOrdinaryEntriesInDecimalSizes(t *testing.T) {
	t.Parallel()

	transcript := sized(t, func(body string) string { return addedFileDiff("logs/transcript.txt", body) }, 3_100_000)
	small := sized(t, func(body string) string { return modifiedFileDiff("src/small.go", body) }, 640)
	tr := &recordingTransport{body: "none"}

	if err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), small+transcript); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	want := checkPrompt(
		"- logs/transcript.txt (added, 3.1 MB)",
		"- src/small.go (modified, 640 B)",
	)
	if got := tr.lastPrompt(t); got != want {
		t.Errorf("check prompt =\n%s\nwant\n%s", got, want)
	}
}

func TestExclusionCheck_LeavesOutPathsTheKeepListCovers(t *testing.T) {
	t.Parallel()

	kept := addedFileDiff("docs/api/export.md", "+# Export API\n")
	keptByGlob := modifiedFileDiff("docs/guide/intro.md", "+intro\n")
	listed := modifiedFileDiff("src/export.go", "+package src\n")
	tr := &recordingTransport{body: "none"}
	check := notes.NewExclusionCheck(tr, []string{"docs/api/", "docs/*/intro.md"}, nil)

	if err := check.Check(t.Context(), kept+keptByGlob+listed); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	want := checkPrompt(fmt.Sprintf("- src/export.go (modified, %d B)", len(listed)))
	if got := tr.lastPrompt(t); got != want {
		t.Errorf("check prompt =\n%s\nwant\n%s", got, want)
	}
}

func TestExclusionCheck_KeepListCoversEveryPath_MakesNoCallAndNarratesNothing(t *testing.T) {
	t.Parallel()

	narration := &checkNarration{}
	calls := 0
	tr := transportFunc(func(context.Context, string) (string, error) {
		calls++
		return "none", nil
	})
	diff := addedFileDiff("docs/api/export.md", "+# Export API\n") + modifiedFileDiff("docs/api/import.md", "+# Import API\n")

	if err := notes.NewExclusionCheck(tr, []string{"docs/api/"}, narration).Check(t.Context(), diff); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if calls != 0 {
		t.Errorf("check calls = %d, want none", calls)
	}
	if len(narration.events) != 0 {
		t.Errorf("narration = %v, want none", narration.events)
	}
}

func TestExclusionCheck_NarratesTheCallStartingAndEnding(t *testing.T) {
	t.Parallel()

	narration := &checkNarration{}
	tr := transportFunc(func(context.Context, string) (string, error) {
		narration.events = append(narration.events, "call")
		return "none", nil
	})

	if err := notes.NewExclusionCheck(tr, nil, narration).Check(t.Context(), modifiedFileDiff("a.go", "+a\n")); err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if want := []string{"checking", "call", "checked"}; !slices.Equal(narration.events, want) {
		t.Errorf("narration = %v, want %v", narration.events, want)
	}
}

func TestExclusionCheck_AFailedCallLetsTheReleaseCarryOn(t *testing.T) {
	t.Parallel()

	for _, failure := range []error{ai.ErrGenerationFailed, ai.ErrTimeout, ai.ErrCommandMissing} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Parallel()

			narration := &checkNarration{}
			tr := &recordingTransport{err: failure}

			if err := notes.NewExclusionCheck(tr, nil, narration).Check(t.Context(), modifiedFileDiff("a.go", "+a\n")); err != nil {
				t.Errorf("Check returned %v, want nil so the release carries on", err)
			}
			if want := []string{"checking", "checked"}; !slices.Equal(narration.events, want) {
				t.Errorf("narration = %v, want %v", narration.events, want)
			}
		})
	}
}

func TestExclusionCheck_ACancelDuringTheCallIsReturned(t *testing.T) {
	t.Parallel()

	tr := &recordingTransport{err: fmt.Errorf("ai call: %w", context.Canceled)}

	err := notes.NewExclusionCheck(tr, nil, nil).Check(t.Context(), modifiedFileDiff("a.go", "+a\n"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Check returned %v, want the cancel", err)
	}
}

// orderTransport answers the check with "none" and every other call with the notes,
// recording which kind of call came in, in order.
type orderTransport struct{ calls []string }

func (ot *orderTransport) Generate(_ context.Context, prompt string) (string, error) {
	if strings.HasPrefix(prompt, checkPromptOpening) {
		ot.calls = append(ot.calls, "check")
		return "none", nil
	}
	ot.calls = append(ot.calls, "notes")
	return "## notes\n", nil
}

func TestSelector_ExclusionCheck_RunsBeforeCacheReuseAndTheNotesCall(t *testing.T) {
	t.Parallel()

	r := seedNormalPathGit(t, modifiedFileDiff("src/a.go", "+a\n"), "M\tsrc/a.go\n", "1\t0\tsrc/a.go\n")
	tr := &orderTransport{}
	root := t.TempDir()
	assembler := notes.NewAssembler(r, notes.ExcludeConfig{})
	sel := notes.NewSelector(notes.NewGenerator(assembler, tr, root), assembler, r, root).
		WithExclusionCheck(notes.NewExclusionCheck(tr, nil, nil))
	reuse := func(string, string) (string, bool, error) {
		tr.calls = append(tr.calls, "reuse")
		return "", false, nil
	}

	_, kind, _, err := sel.SelectBodyWithReuse(t.Context(), notes.SelectState{LastTag: "v1.0.0"}, normalCfg(), reuse)
	if err != nil {
		t.Fatalf("SelectBodyWithReuse returned unexpected error: %v", err)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI", kind)
	}
	if want := []string{"check", "reuse", "notes"}; !slices.Equal(tr.calls, want) {
		t.Errorf("calls = %v, want %v", tr.calls, want)
	}
}

func TestSelector_ExclusionCheckCancelled_ReturnsTheCancelBeforeAnyNotesCall(t *testing.T) {
	t.Parallel()

	r := seedNormalPathGit(t, modifiedFileDiff("src/a.go", "+a\n"), "M\tsrc/a.go\n", "1\t0\tsrc/a.go\n")
	var prompts []string
	tr := transportFunc(func(_ context.Context, prompt string) (string, error) {
		prompts = append(prompts, prompt)
		return "", context.Canceled
	})
	root := t.TempDir()
	assembler := notes.NewAssembler(r, notes.ExcludeConfig{})
	sel := notes.NewSelector(notes.NewGenerator(assembler, tr, root), assembler, r, root).
		WithExclusionCheck(notes.NewExclusionCheck(tr, nil, nil))

	_, _, err := sel.SelectBody(t.Context(), notes.SelectState{LastTag: "v1.0.0"}, normalCfg())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SelectBody returned %v, want the cancel", err)
	}
	if len(prompts) != 1 || !strings.HasPrefix(prompts[0], checkPromptOpening) {
		t.Errorf("calls = %d, want only the check's", len(prompts))
	}
}

func TestSelector_PromptFileMissing_FailsBeforeTheCheckCall(t *testing.T) {
	t.Parallel()

	r := seedNormalPathGit(t, modifiedFileDiff("src/a.go", "+a\n"), "M\tsrc/a.go\n", "1\t0\tsrc/a.go\n")
	tr := &recordingTransport{body: "none"}
	root := t.TempDir()
	assembler := notes.NewAssembler(r, notes.ExcludeConfig{})
	sel := notes.NewSelector(notes.NewGenerator(assembler, tr, root), assembler, r, root).
		WithExclusionCheck(notes.NewExclusionCheck(tr, nil, nil))
	cfg := normalCfg()
	cfg.Release.Prompt = "missing-prompt.md"

	if _, _, err := sel.SelectBody(t.Context(), notes.SelectState{LastTag: "v1.0.0"}, cfg); err == nil {
		t.Fatal("SelectBody returned nil error for a missing [release].prompt file")
	}
	if tr.calls() != 0 {
		t.Errorf("transport calls = %d, want none before the prompt file fails", tr.calls())
	}
}
