package engine_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

const fallbackConfig = "[release]\non_notes_failure = \"fallback\"\n"

// nextVersion is nextTag's version, as regenerate takes it.
const nextVersion = "1.2.4"

func fallbackWarning(phrase, output string) presenter.Warning {
	return presenter.Warning{Label: "notes", Message: phrase + "; falling back to the commit-subject list", Output: output}
}

func savedFallbackWarning(phrase, output, path string) presenter.Warning {
	return presenter.Warning{
		Label: "notes",
		Message: phrase + "; falling back to the commit-subject list; part notes saved to " + path +
			" — once this release is out, put them on it with mint release regenerate " + nextVersion + " --notes-file " + path +
			", or if the run ends before the push, release them with mint release --notes-file " + path,
		Output: output,
	}
}

// fallbackSinglePassRun is a release under fallback whose one notes call fails with
// failure, scripted through to the push.
func fallbackSinglePassRun(t *testing.T, failure error) (*runner.FakeRunner, *presentertest.RecordingPresenter, engine.ReleaseDeps) {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, fallbackConfig)
	f, rec, deps := singlePassFailingRun(t, root, failure)
	f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	return f, rec, deps
}

// assertWarnedBetweenTheNotesAndTheirShowing fails unless every warning follows the
// notes stage's completion and precedes the notes being shown.
func assertWarnedBetweenTheNotesAndTheirShowing(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	done, shown := -1, indexOfKind(rec, presentertest.KindShowNotes)
	for i, e := range rec.Events {
		if e.Kind == presentertest.KindStageSucceeded && e.StageSucceeded.Name == "notes" {
			done = i
		}
	}
	for i, e := range rec.Events {
		if e.Kind == presentertest.KindWarn && (i < done || i > shown) {
			t.Errorf("event kinds = %v, want every warning after the notes stage ends and before the notes show", rec.Kinds())
			return
		}
	}
}

func TestRelease_Fallback_Unattended_ShipsTheCommitSubjectListWithOneWarningCarryingTheOutput(t *testing.T) {
	t.Parallel()

	f, rec, deps := fallbackSinglePassRun(t, &ai.GenerationError{Stdout: loginOutput + "\n", ExitCode: 1})
	deps.Yes = true

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("the run was offered the choice: %+v", gates)
	}
	if n := indexOfKind(rec, presentertest.KindStageFailed); n != -1 {
		t.Errorf("a ✗ line rendered; kinds = %v", rec.Kinds())
	}
	assertOnlyWarning(t, rec, fallbackWarning("AI call failed", loginOutput))
	assertWarnedBetweenTheNotesAndTheirShowing(t, rec)
	if got := tagAnnotationBody(t, f, nextTag); got != commitSubjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, commitSubjects)
	}
}

func TestRelease_Fallback_Attended_WarnsThenShowsTheListAtTheReviewGateWithoutRegenerate(t *testing.T) {
	t.Parallel()

	f, rec, deps := fallbackSinglePassRun(t, &ai.GenerationError{Stdout: loginOutput + "\n", ExitCode: 1})

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if gates := floorGates(rec); len(gates) != 0 {
		t.Errorf("the run was offered the choice: %+v", gates)
	}
	assertOnlyWarning(t, rec, fallbackWarning("AI call failed", loginOutput))
	assertWarnedBetweenTheNotesAndTheirShowing(t, rec)
	if got := showNotesBody(t, rec); got != commitSubjects {
		t.Errorf("shown notes = %q, want the commit-subject list %q", got, commitSubjects)
	}
	gates := reviewGates(rec)
	if len(gates) != 1 {
		t.Fatalf("review gates = %d, want one", len(gates))
	}
	if got, want := gates[0].Keys(), []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}; !slices.Equal(got, want) {
		t.Errorf("review gate keys = %v, want %v", got, want)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != commitSubjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, commitSubjects)
	}
}

func TestRelease_Fallback_Timeout_WarnsAITimedOutWithNoOutput(t *testing.T) {
	t.Parallel()

	f, rec, deps := fallbackSinglePassRun(t, fmt.Errorf("%w: context deadline exceeded", ai.ErrTimeout))
	deps.Yes = true

	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertOnlyWarning(t, rec, fallbackWarning("AI timed out", ""))
	if got := tagAnnotationBody(t, f, nextTag); got != commitSubjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, commitSubjects)
	}
}

func showNotesBody(t *testing.T, rec *presentertest.RecordingPresenter) string {
	t.Helper()
	at := indexOfKind(rec, presentertest.KindShowNotes)
	if at == -1 {
		t.Fatalf("no notes shown; kinds = %v", rec.Kinds())
	}
	return rec.Events[at].ShowNotes.Body
}

const partThreeOutput = "API Error: Overloaded"

var sixPartPaths = []string{"a/1.go", "b/2.go", "c/3.go", "d/4.go", "e/5.go", "f/6.go"}

// sixFileDiff is a range diff of six files of about size bytes each.
func sixFileDiff(size int) string {
	var b strings.Builder
	for _, path := range sixPartPaths {
		b.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		b.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", size/64))
	}
	return b.String()
}

// sixPartTransport answers the exclusion check "none", a six-part chunked run's
// classify calls OTHER and its parts with their notes, failing part 3 with
// partThreeOutput.
func sixPartTransport(_ context.Context, prompt string) (string, error) {
	switch {
	case isCheckPrompt(prompt):
		return "none", nil
	case isClassifyPrompt(prompt):
		return "OTHER", nil
	case strings.Contains(prompt, "What follows is part 3 of 6"):
		return "", &ai.GenerationError{Stdout: partThreeOutput, ExitCode: 1}
	case isChunkCallPrompt(prompt):
		return "- part notes", nil
	default:
		return "merged notes", nil
	}
}

// keptChunkNotes records the chunk notes each save is handed.
type keptChunkNotes struct {
	store engine.ChunkNotesSaver
	mu    sync.Mutex
	notes []string
}

func (k *keptChunkNotes) Save(repoRoot, tag, notes string) (string, error) {
	k.mu.Lock()
	k.notes = append(k.notes, notes)
	k.mu.Unlock()
	return k.store.Save(repoRoot, tag, notes)
}

// newSixPartFallbackRun is a release under fallback whose pre_tag hook leaves an
// artifact commit, over six files of about 10 KB under partCapBytes, one part a
// file, whose part 3 fails for good; kept records what its chunk notes store saves.
// The caller seeds what follows the commit-subject list.
func newSixPartFallbackRun(t *testing.T) (*chunkNotesRun, *keptChunkNotes) {
	t.Helper()
	run := &chunkNotesRun{root: t.TempDir(), userCache: t.TempDir(), f: runner.NewFakeRunner(), rec: &presentertest.RecordingPresenter{}}
	writeConfig(t, run.root, partCapToml+"\n"+fallbackConfig+"\n[release.hooks]\npre_tag = \"build.sh\"\n")
	run.f.Seed("sh", runner.Result{}, nil)
	seedPriorTagReadGates(run.f, run.root, "main")
	var nameStatus, numstat strings.Builder
	for _, path := range sixPartPaths {
		nameStatus.WriteString("M\t" + path + "\n")
		numstat.WriteString("156\t0\t" + path + "\n")
	}
	run.f.SeedSequence("git",
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
		ScriptedOut(sixFileDiff(10_000)),
		ScriptedOut(nameStatus.String()),
		ScriptedOut(numstat.String()),
		ScriptedOut(commitSubjects), // log --format=%s
	)
	kept := &keptChunkNotes{store: notescache.NewChunkNotes(run.chunkNotesBase())}
	run.deps = newDeps(run.rec, run.f)
	run.deps.Transport = transportFunc(sixPartTransport)
	run.deps.ChunkNotes = kept
	return run, kept
}

func TestRelease_Fallback_ChunkFailsForGood_WarningNamesThePartTheOutputThePathAndBothCommands(t *testing.T) {
	t.Parallel()

	run, _ := newSixPartFallbackRun(t)
	run.deps.Yes = true
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)

	if err := run.release(t, t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	path := run.savedPath(nextTag)
	assertOnlyWarning(t, run.rec, savedFallbackWarning("AI call failed (part 3 of 6)", partThreeOutput, path))
	if got := tagAnnotationBody(t, run.f, nextTag); got != commitSubjects {
		t.Errorf("tag annotation body = %q, want the commit-subject list %q", got, commitSubjects)
	}
}

func TestRelease_Fallback_MergePromptOverTheCap_WarningNamesThePathAndBothCommands(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "fallback", func(_ context.Context, prompt string) (string, error) {
		switch {
		case isCheckPrompt(prompt):
			return "none", nil
		case isChunkCallPrompt(prompt):
			return bigPartNotes, nil
		}
		return "merged notes", nil
	})
	run.deps.Yes = true
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)

	if err := run.release(t, t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertOnlyWarning(t, run.rec, savedFallbackWarning(mergeTooLargePhrase, "", run.savedPath(nextTag)))
}

// TestRelease_Fallback_TheWarningsReleaseCommandCarriesTheRunsOwnFlags pins that the
// release the warning names is the run's own, its -m kept, with only --notes-file
// added; the regenerate command takes no release flag.
func TestRelease_Fallback_TheWarningsReleaseCommandCarriesTheRunsOwnFlags(t *testing.T) {
	t.Parallel()

	run := newChunkNotesRun(t, "fallback", func(_ context.Context, prompt string) (string, error) {
		switch {
		case isCheckPrompt(prompt):
			return "none", nil
		case isChunkCallPrompt(prompt):
			return bigPartNotes, nil
		}
		return "merged notes", nil
	})
	run.deps.Yes = true
	run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
	seedRecordTagPush(run.f, run.root)
	run.f.Seed("gh", runner.Result{}, nil)
	opts := priorTagNormalAIOptions()
	opts.Bump = version.BumpMinor

	if err := engine.Release(t.Context(), run.deps, opts); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	path := run.savedPath("v1.3.0")
	assertOnlyWarning(t, run.rec, presenter.Warning{
		Label: "notes",
		Message: mergeTooLargePhrase + "; falling back to the commit-subject list; part notes saved to " + path +
			" — once this release is out, put them on it with mint release regenerate 1.3.0 --notes-file " + path +
			", or if the run ends before the push, release them with mint release -m --notes-file " + path,
	})
}

func TestRelease_Fallback_AttendedAbortAtTheReviewGate_UnwindsAndKeepsTheSavedPartNotes(t *testing.T) {
	t.Parallel()

	run, kept := newSixPartFallbackRun(t)
	run.f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startingSHA
	run.deps.StdinInteractive = true
	run.rec.PromptResult = func(g presenter.Gate) (presenter.Choice, error) {
		if g.Subject == "notes" && g.Has(presenter.ChoiceYes) {
			return presenter.ChoiceNo, nil
		}
		return g.Default, nil
	}

	err := run.release(t, t.Context())

	assertAbortNonZero(t, err)
	assertNoTag(t, run.f)
	if !invokedWith(run.f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("the pre_tag artifact commit was not unwound; got %v", commandLines(run.f.Invocations()))
	}
	if len(kept.notes) != 1 {
		t.Fatalf("chunk notes saves = %d, want one", len(kept.notes))
	}
	if got := readSaved(t, run.savedPath(nextTag)); got != kept.notes[0] {
		t.Errorf("file at the saved path = %q, want the saved part notes %q", got, kept.notes[0])
	}
}
