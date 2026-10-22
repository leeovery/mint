package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	exportGoDiff = "diff --git a/src/export.go b/src/export.go\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/src/export.go\n" +
		"+++ b/src/export.go\n" +
		"@@ -1 +1 @@\n" +
		"-func Export() {}\n" +
		"+func Export() error { return nil }\n"
	designDocDiff = "diff --git a/docs/design/export.md b/docs/design/export.md\n" +
		"new file mode 100644\n" +
		"index 0000000..3333333\n" +
		"--- /dev/null\n" +
		"+++ b/docs/design/export.md\n" +
		"@@ -0,0 +1 @@\n" +
		"+# Export design\n"
)

// checkedRelease runs a prior-tag release under config whose notes diff is diff, with
// the Change Map's git answers, claude answering the check with checkAnswer and the
// notes with aiBody.
func checkedRelease(t *testing.T, config, diff, nameStatus, numstat, checkAnswer string) (*runner.FakeRunner, *presentertest.RecordingPresenter) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, config)
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(diff), ScriptedOut(nameStatus), ScriptedOut(numstat))
	f.SeedSequence("claude", ScriptedOut(checkAnswer), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}
	return f, rec
}

// checkList is the list lines of a check prompt.
func checkList(prompt string) []string {
	var lines []string
	for line := range strings.Lines(prompt) {
		if strings.HasPrefix(line, "- ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	return lines
}

func TestRelease_ExclusionCheck_OneCallBeforeTheNotesListingEachChangedPathAndNoDiff(t *testing.T) {
	t.Parallel()

	f, _ := checkedRelease(t, "", exportGoDiff+designDocDiff,
		"M\tsrc/export.go\nA\tdocs/design/export.md\n", "1\t1\tsrc/export.go\n1\t0\tdocs/design/export.md\n", "none")

	claude := invocationsNamed(f, "claude")
	if len(claude) != 2 {
		t.Fatalf("claude calls = %d, want the check then the notes", len(claude))
	}
	check, notesCall := claude[0].Stdin, claude[1].Stdin
	if !isCheckPrompt(check) {
		t.Fatalf("first claude call is not the check:\n%s", check)
	}
	want := []string{
		fmt.Sprintf("- docs/design/export.md (added, %d B)", len(designDocDiff)),
		fmt.Sprintf("- src/export.go (modified, %d B)", len(exportGoDiff)),
	}
	if got := checkList(check); !slices.Equal(got, want) {
		t.Errorf("check list = %q, want %q", got, want)
	}
	for _, content := range []string{"func Export", "# Export design", "@@", "diff --git"} {
		if strings.Contains(check, content) {
			t.Errorf("check prompt carries diff content %q", content)
		}
	}
	for _, ask := range []string{"Suggest git pathspec globs", "then a short reason", "answer with exactly: none"} {
		if !strings.Contains(check, ask) {
			t.Errorf("check prompt does not ask %q:\n%s", ask, check)
		}
	}
	if !strings.Contains(notesCall, exportGoDiff+designDocDiff) {
		t.Errorf("the second claude call is not the notes call over the diff:\n%s", notesCall)
	}
}

func TestRelease_ExclusionCheck_ListsNoPathAnExclusionOrTheKeepListCovers(t *testing.T) {
	t.Parallel()

	const config = "diff_exclude = [\"*.lock\"]\n\n" +
		"[release]\n" +
		"diff_exclude = [\"specs/\"]\n" +
		"diff_keep = [\"docs/api/\"]\n" +
		"version_file = \"VERSION\"\n"
	keptDiff := "diff --git a/docs/api/export.md b/docs/api/export.md\n@@ -0,0 +1 @@\n+# Export API\n"
	// git leaves the excluded paths out of the notes diff, so the diff holds the kept
	// path and the source file alone.
	f, _ := checkedRelease(t, config, keptDiff+exportGoDiff,
		"A\tdocs/api/export.md\nM\tsrc/export.go\n", "1\t0\tdocs/api/export.md\n1\t1\tsrc/export.go\n", "none")

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD",
		":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)*.lock", ":(exclude)specs/", ":(exclude)VERSION")
	checks := checkInvocations(f, "claude")
	if len(checks) != 1 {
		t.Fatalf("check calls = %d, want one", len(checks))
	}
	want := []string{fmt.Sprintf("- src/export.go (modified, %d B)", len(exportGoDiff))}
	if got := checkList(checks[0].Stdin); !slices.Equal(got, want) {
		t.Errorf("check list = %q, want only %q", got, want)
	}
	for _, path := range []string{"app.lock", "specs/export.md", "docs/api/export.md", ".mint.toml", "CHANGELOG.md", "VERSION"} {
		if strings.Contains(checks[0].Stdin, path) {
			t.Errorf("check prompt lists %q", path)
		}
	}
	if notes := notesStdinOf(t, f, "claude", leanClaudeArgs...); !strings.Contains(notes, keptDiff) {
		t.Errorf("the notes call does not carry the kept path's diff:\n%s", notes)
	}
}

// fileDiffOfSize is a modified file's diff section for path, exactly size bytes long.
func fileDiffOfSize(t *testing.T, path string, size int) string {
	t.Helper()
	header := "diff --git a/" + path + " b/" + path + "\n@@ -1 +1 @@\n+"
	if size < len(header)+1 {
		t.Fatalf("a %d-byte section is smaller than its header", size)
	}
	return header + strings.Repeat("x", size-len(header)-1) + "\n"
}

func TestRelease_ExclusionCheck_ListsBigFilesAsOrdinaryEntriesInDecimalSizes(t *testing.T) {
	t.Parallel()

	diff := fileDiffOfSize(t, "logs/transcript.txt", 3_100_000) + fileDiffOfSize(t, "src/small.go", 640)
	// A cap the whole diff fits under, so the notes go in one call.
	f, _ := checkedRelease(t, "max_prompt_bytes = 5000000\n", diff,
		"M\tlogs/transcript.txt\nM\tsrc/small.go\n", "1\t1\tlogs/transcript.txt\n1\t1\tsrc/small.go\n", "none")

	checks := checkInvocations(f, "claude")
	if len(checks) != 1 {
		t.Fatalf("check calls = %d, want one", len(checks))
	}
	want := []string{"- logs/transcript.txt (modified, 3.1 MB)", "- src/small.go (modified, 640 B)"}
	if got := checkList(checks[0].Stdin); !slices.Equal(got, want) {
		t.Errorf("check list = %q, want %q", got, want)
	}
}

func TestRelease_ExclusionCheck_RunsTheReleaseCommandUnderTheReleaseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   string
		wantArgs []string
		timeout  time.Duration
	}{
		{
			name:     "[release] command and timeout",
			config:   "[release]\nai_command = \"claude -p --model opus\"\ntimeout = 300\n",
			wantArgs: []string{"-p", "--model", "opus"},
			timeout:  300 * time.Second,
		},
		{name: "neither key set", config: "", wantArgs: leanClaudeArgs, timeout: 180 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tt.config != "" {
				writeConfig(t, root, tt.config)
			}
			f := runner.NewFakeRunner()
			seedPriorTagReadGates(f, root, "main")
			seedNormalAINotes(f)
			seedClaudeAfterCheck(f, ScriptedOut(aiBody))
			seedRecordTagPush(f, root)
			f.Seed("gh", runner.Result{}, nil)
			d := &deadlineRunner{FakeRunner: f}
			deps := newDeps(&presentertest.RecordingPresenter{}, f)
			deps.Runner = d

			if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			checks := checkInvocations(f, "claude")
			if len(checks) != 1 || len(d.checks) != 1 {
				t.Fatalf("check calls = %d, want one", len(checks))
			}
			if !slices.Equal(checks[0].Args, tt.wantArgs) {
				t.Errorf("check args = %q, want %q", checks[0].Args, tt.wantArgs)
			}
			if got := d.checks[0].remaining; got > tt.timeout || got < tt.timeout-5*time.Second {
				t.Errorf("check deadline remaining = %v, want about %v", got, tt.timeout)
			}
		})
	}
}

func TestRelease_ExclusionCheckAnswersNone_TheNotesAreWrittenFromTheSameDiff(t *testing.T) {
	t.Parallel()

	diff := exportGoDiff + designDocDiff
	f, _ := checkedRelease(t, "", diff,
		"M\tsrc/export.go\nA\tdocs/design/export.md\n", "1\t1\tsrc/export.go\n1\t0\tdocs/design/export.md\n", "none")

	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 1 {
		t.Fatalf("notes calls = %d, want one", len(notesCalls))
	}
	if !strings.Contains(notesCalls[0].Stdin, "\n\n"+diff+"\n\n") {
		t.Errorf("the notes call does not carry the whole diff unchanged:\n%s", notesCalls[0].Stdin)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the notes %q", got, aiBody)
	}
}

func TestRelease_RealRunAfterADryRunCachedItsNotes_ChecksBeforeOfferingTheCachedNotes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)
	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(checkInvocations(f, "claude")); got != 1 {
		t.Errorf("check calls = %d, want one", got)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made; the cached notes should be reused")
	}
	checked := indexOfKind(rec, presentertest.KindNotesChecked)
	offered := slices.IndexFunc(rec.Events, func(ev presentertest.Event) bool {
		return ev.Kind == presentertest.KindPrompt && ev.Prompt.Question == "Cached notes found for this diff — use them?"
	})
	if checked < 0 || offered < 0 || checked > offered {
		t.Errorf("check ended at event %d, cached notes offered at %d; want the check first: %v", checked, offered, rec.Kinds())
	}
}

func TestRelease_ExclusionCheck_ShowsOnTheNotesLineThenTheNotesNarrateAsWithoutIt(t *testing.T) {
	t.Parallel()

	_, rec := checkedRelease(t, "", priorTagDiff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n", "none")

	stages := notesStages(t, rec)
	if len(stages) != 1 {
		t.Fatalf("notes stages = %d, want one", len(stages))
	}
	if got := countKind(rec, presentertest.KindNotesChecking); got != 1 {
		t.Errorf("check narrations = %d, want one", got)
	}
	assertSinglePassNotesStage(t, afterCheck(t, stages[0]))
}

func TestRelease_ExclusionCheck_PlainMode_PrintsItsOwnLineBeforeTheNotesLines(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	seedClaudeAfterCheck(f, ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	out := &bytes.Buffer{}
	deps := newDeps(&presentertest.RecordingPresenter{}, f)
	// Accepts the version gate and the review gate.
	deps.Presenter = presenter.NewPlainPresenterWithInput(out, &bytes.Buffer{}, strings.NewReader("y\ny\n"))

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	var lines []string
	for line := range strings.Lines(out.String()) {
		if strings.HasPrefix(line, "notes: ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	if len(lines) != 3 || lines[0] != "notes: running..." || lines[1] != plainCheckLine || !strings.HasPrefix(lines[2], "notes: generated (") {
		t.Errorf("notes lines =\n%s\nwant the start, the check's line, then the completion", strings.Join(lines, "\n"))
	}
}

func TestRelease_RegenAtTheReviewGate_MakesNoCheckCall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	seedNormalAINotes(f) // the r's generation
	const regenBody = "## regenerated\n"
	seedClaudeAfterCheck(f, ScriptedOut(aiBody), ScriptedOut(regenBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"lead with auth"},
	}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(checkInvocations(f, "claude")); got != 1 {
		t.Errorf("check calls = %d, want only the first attempt's", got)
	}
	notesCalls := notesInvocations(f, "claude")
	if len(notesCalls) != 2 || !strings.Contains(notesCalls[1].Stdin, "lead with auth") {
		t.Fatalf("notes calls = %d, want the first attempt's then the r's", len(notesCalls))
	}
	if got := countKind(rec, presentertest.KindNotesChecking); got != 1 {
		t.Errorf("check narrations = %d, want only the first attempt's", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != regenBody {
		t.Errorf("tag annotation body = %q, want the r's notes %q", got, regenBody)
	}
}

// overflowAboveTransport answers the check "none", fails every call carrying diff over
// limit bytes as an overflow its classify call reads as SIZE, writes the notes of every
// other call carrying diff, and merges them.
type overflowAboveTransport struct {
	limit int

	mu        sync.Mutex
	check     []string
	succeeded []string
}

func (ot *overflowAboveTransport) Generate(_ context.Context, prompt string) (string, error) {
	ot.mu.Lock()
	defer ot.mu.Unlock()
	switch {
	case isCheckPrompt(prompt):
		ot.check = append(ot.check, prompt)
		return "none", nil
	case isClassifyPrompt(prompt):
		return "SIZE", nil
	case !strings.Contains(prompt, "diff --git "):
		return halvedNotesBody, nil
	case len(prompt) > ot.limit:
		return "", &ai.GenerationError{Stdout: "Prompt is too long\n", ExitCode: 1}
	default:
		ot.succeeded = append(ot.succeeded, prompt)
		return "- part notes", nil
	}
}

func TestRelease_HalvedNotes_TheLowerTheCapHintNeverNamesTheCheckCall(t *testing.T) {
	t.Parallel()

	// Many small files under long paths: the check's list is long, while halving
	// leaves every call that carries diff small.
	var diff, nameStatus, numstat strings.Builder
	for i := range 300 {
		path := fmt.Sprintf("pkg/%s_%03d.go", strings.Repeat("long_name_", 9), i)
		diff.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1 @@\n-a\n+b\n")
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("1\t1\t" + path + "\n")
	}
	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(diff.String()), ScriptedOut(nameStatus.String()), ScriptedOut(numstat.String()))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	// The limit sits above twice the default prompt so a halved call still has room
	// for diff, and halving bottoms out under it as the prompt's wording changes.
	tr := &overflowAboveTransport{limit: 2*len(notes.DefaultPrompt) + 8_000}
	deps := newDeps(rec, f)
	deps.Transport = tr

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if len(tr.check) != 1 || len(tr.check[0]) <= largest(tr.succeeded) {
		t.Fatalf("fixture: the check call is not bigger than every call carrying diff")
	}
	assertLowerCapHint(t, rec, largest(tr.succeeded), presenter.ReleaseReviewGate())
}
