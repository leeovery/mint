package engine_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const caseJSONDiff = "diff --git a/test/fixtures/case.json b/test/fixtures/case.json\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/test/fixtures/case.json\n" +
	"@@ -0,0 +1 @@\n" +
	"+{\"case\": 1}\n"

const designAndFixturesAnswer = "docs/design/: design documents\ntest/fixtures/: test data\n"

func answersSubject(tag string) string {
	return "chore(release): release-notes path answers for " + tag
}

// configNotIgnored and configIgnored are git check-ignore's answers about .mint.toml.
var (
	configNotIgnored = ScriptedNonZero()
	configIgnored    = ScriptedOut("")
)

// answersCommitCalls is git's answers to whether it ignores .mint.toml, then to
// staging and committing it.
var answersCommitCalls = []runner.ScriptedCall{configNotIgnored, ScriptedOut(""), ScriptedOut("")}

// withAnswersCommit is git's answers to a release whose pauses follow the first of
// calls: the notes diff they ask about, the answers commit, then the rest of calls.
func withAnswersCommit(calls ...runner.ScriptedCall) []runner.ScriptedCall {
	return slices.Concat(calls[:1], answersCommitCalls, calls[1:])
}

// commitSubjectsIn returns the subject of every `git -C root commit -m` call, in order.
func commitSubjectsIn(f *runner.FakeRunner, root string) []string {
	var subjects []string
	for _, inv := range invocationsNamed(f, "git") {
		if len(inv.Args) >= 5 && inv.Args[0] == "-C" && inv.Args[1] == root && inv.Args[2] == "commit" && inv.Args[3] == "-m" {
			subjects = append(subjects, inv.Args[4])
		}
	}
	return subjects
}

func minorOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpMinor, Now: fixedClock}
}

func TestRelease_AnswersCommit_SitsBetweenTheArtifactCommitAndTheBookkeepingCommitAndGoesOutWithThePush(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(" M dist/bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
		ScriptedOut(""),                    // -C root add -A
		ScriptedOut(""),                    // -C root commit -m pre-tag artifacts
	)
	f.SeedSequence("git", excludeThenKeepCalls...)
	f.SeedSequence("claude", ScriptedOut(designAndFixturesAnswer), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)

	if err := engine.Release(t.Context(), attendedDeps(rec, f), minorOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	const tag = "v1.3.0"
	want := []string{pretagArtifactSubject(tag), answersSubject(tag), "🌿 Release " + tag}
	if got := commitSubjectsIn(f, root); !slices.Equal(got, want) {
		t.Fatalf("commits = %q, want %q", got, want)
	}
	answersAt := indexOfCmd(f, "git", "-C", root, "commit", "-m", answersSubject(tag))
	if stage := f.Invocations()[answersAt-1]; commandLine(stage) != commandLine(runner.Invocation{Name: "git", Args: []string{"-C", root, "add", config.FileName}}) {
		t.Errorf("the call before the answers commit = %q, want .mint.toml staged by path alone", commandLine(stage))
	}
	pushAt := indexOfCmd(f, "git", "push", "--atomic", "origin", "HEAD", tag)
	bookkeepingAt := indexOfCmd(f, "git", "-C", root, "commit", "-m", "🌿 Release "+tag)
	if pushAt == -1 || pushAt < bookkeepingAt {
		t.Errorf("the atomic push (at %d) does not follow the three commits (bookkeeping at %d)", pushAt, bookkeepingAt)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("loading the committed .mint.toml: %v", err)
	}
	if !slices.Equal(cfg.Release.DiffExclude, []string{"docs/design/"}) || !slices.Equal(cfg.Release.DiffKeep, []string{"test/fixtures/"}) {
		t.Errorf("[release] diff_exclude = %q, diff_keep = %q, want both answers", cfg.Release.DiffExclude, cfg.Release.DiffKeep)
	}
}

func TestRelease_AnswersCommit_KeepsItsFixedSubjectUnderACommitPrefix(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	f, root, err := answeredRelease(t, rec, "[release]\ncommit_prefix = \"🚀\"\n",
		excludeThenKeepCalls, designAndFixturesAnswer, aiBody)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{answersSubject(nextTag), "🚀 Release " + nextTag}
	if got := commitSubjectsIn(f, root); !slices.Equal(got, want) {
		t.Errorf("commits = %q, want %q", got, want)
	}
}

func TestRelease_RecordedAnswers_NarrateOneLineAfterTheLastPauseBeforeTheNotesLineResumes(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	if _, _, err := answeredRelease(t, rec, "", excludeThenKeepCalls, designAndFixturesAnswer, aiBody); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	recorded := indexOfKind(rec, presentertest.KindExclusionAnswersRecorded)
	if recorded == -1 {
		t.Fatalf("no answers line; events = %v", rec.Kinds())
	}
	want := presenter.ExclusionAnswers{Stage: "notes", Key: "check", File: ".mint.toml", Table: "[release]", Exclude: []string{"docs/design/"}, Keep: []string{"test/fixtures/"}}
	if got := rec.Events[recorded].ExclusionAnswers; !reflect.DeepEqual(got, want) {
		t.Errorf("answers payload = %+v, want %+v", got, want)
	}
	if n := countKind(rec, presentertest.KindExclusionAnswersRecorded); n != 1 {
		t.Errorf("answers lines = %d, want one", n)
	}
	kinds := rec.Kinds()
	if len(pauseGates(rec)) != 2 || indexOfPauseFrom(rec, recorded) != -1 {
		t.Errorf("the answers line does not follow the last of the two pauses; events = %v", kinds)
	}
	if kinds[recorded-1] != presentertest.KindSuspendSpinner || kinds[recorded+1] != presentertest.KindResumeSpinner {
		t.Errorf("the answers line is not shown with the notes line suspended; events = %v", kinds[recorded-1:recorded+2])
	}
	notesDone := indexOfKindFrom(rec, presentertest.KindStageSucceeded, recorded)
	if notesDone == -1 || rec.Events[notesDone].StageSucceeded.Name != "notes" {
		t.Errorf("the notes stage does not complete after the answers line; events = %v", kinds)
	}
}

func TestRelease_NoPause_NarratesNoAnswersLineAndMakesNoAnswersCommit(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	f, err := pausedRelease(t, rec, "", "none")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindExclusionAnswersRecorded); got != 0 {
		t.Errorf("answers lines = %d, want none", got)
	}
	for _, inv := range invocationsNamed(f, "git") {
		if slices.Contains(inv.Args, config.FileName) || slices.Contains(inv.Args, answersSubject(nextTag)) {
			t.Errorf("git %q ran with nothing answered", inv.Args)
		}
	}
}

func TestRelease_AFailedAnswersCommit_NarratesNoAnswersLine(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	_, _, err := answeredRelease(t, rec, "",
		[]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), configNotIgnored, ScriptedOut(""), ScriptedNonZero()},
		"docs/design/: design documents\n", aiBody)
	if err == nil {
		t.Fatal("Release succeeded with the answers commit failing")
	}

	if got := countKind(rec, presentertest.KindExclusionAnswersRecorded); got != 0 {
		t.Errorf("answers lines = %d, want none for a commit that never landed", got)
	}
}

func TestRelease_AnswersLeavingNoPath_AreCommittedAndTheReleaseGetsTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, root, err := answeredRelease(t, rec, "",
		withAnswersCommit(ScriptedOut(designDocDiff), ScriptedOut("")),
		"docs/design/: design documents\n")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countCommitsWithSubject(f, root, answersSubject(nextTag)); got != 1 {
		t.Errorf("answers commits = %d, want one", got)
	}
	if notesCallMade(f) {
		t.Errorf("a notes call was made for a diff the answers left empty")
	}
	const maintenance = "Maintenance release — no notable source changes"
	if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(maintenance) {
		t.Errorf("tag annotation body = %q, want the maintenance-release body", got)
	}
}

func TestRelease_TheNextRelease_ReadsTheRecordedAnswersFromTheCommittedConfig(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude, presenter.ChoiceKeep}, nil)
	_, answeredRoot, err := answeredRelease(t, rec, "", excludeThenKeepCalls, designAndFixturesAnswer, aiBody)
	if err != nil {
		t.Fatalf("the answering release returned unexpected error: %v", err)
	}
	committed, err := os.ReadFile(filepath.Join(answeredRoot, config.FileName))
	if err != nil {
		t.Fatalf("reading the committed .mint.toml: %v", err)
	}

	for name, root := range map[string]func(t *testing.T) string{
		"this clone": func(*testing.T) string { return answeredRoot },
		"a fresh clone": func(t *testing.T) string {
			clone := t.TempDir()
			writeConfig(t, clone, string(committed))
			return clone
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// git leaves docs/design/next.md out of the notes diff the recorded exclusion
			// names, so the diff holds the kept fixture and the source file alone.
			f, next := nextReleaseIn(t, root(t), caseJSONDiff+exportGoDiff,
				"A\ttest/fixtures/case.json\nM\tsrc/export.go\n", "1\t0\ttest/fixtures/case.json\n1\t1\tsrc/export.go\n")

			assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", append(slices.Clone(builtinExcludes), ":(exclude)docs/design/")...)
			checks := checkInvocations(f, "claude")
			if len(checks) != 1 {
				t.Fatalf("check calls = %d, want one", len(checks))
			}
			if got, want := checkList(checks[0].Stdin), []string{fmt.Sprintf("- src/export.go (modified, %d B)", len(exportGoDiff))}; !slices.Equal(got, want) {
				t.Errorf("check list = %q, want %q", got, want)
			}
			if got := len(pauseGates(next)); got != 0 {
				t.Errorf("pauses = %d, want none", got)
			}
			notesCalls := notesInvocations(f, "claude")
			if len(notesCalls) != 1 || !strings.Contains(notesCalls[0].Stdin, caseJSONDiff) {
				t.Errorf("the notes call does not carry test/fixtures/case.json")
			}
			assertNoNotesCallCarries(t, f, "docs/design/")
		})
	}
}

// nextReleaseIn runs an attended prior-tag release in root, its notes diff diff, the
// check finding nothing to suggest.
func nextReleaseIn(t *testing.T, root, diff, nameStatus, numstat string) (*runner.FakeRunner, *presentertest.RecordingPresenter) {
	t.Helper()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(diff), ScriptedOut(nameStatus), ScriptedOut(numstat))
	f.SeedSequence("claude", ScriptedOut("none"), ScriptedOut(aiBody))
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}
	if err := engine.Release(t.Context(), attendedDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("the next release returned unexpected error: %v", err)
	}
	return f, rec
}

func indexOfPauseFrom(rec *presentertest.RecordingPresenter, from int) int {
	for i := from; i < len(rec.Events); i++ {
		if rec.Events[i].Kind == presentertest.KindPrompt && isPauseGate(rec.Events[i].Prompt) {
			return i
		}
	}
	return -1
}
