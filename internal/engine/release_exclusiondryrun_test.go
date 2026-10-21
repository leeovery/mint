package engine_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// newAttendedDryRun is newSuggestedRun at a terminal without -y, its notes call
// recording how many events had been shown when it was made.
func newAttendedDryRun(t *testing.T, config string) (*suggestedRun, *int) {
	t.Helper()
	shownAtNotes := -1
	var run *suggestedRun
	run = newSuggestedRun(t, config, designSuggestion, transportFunc(func(context.Context, string) (string, error) {
		if shownAtNotes == -1 {
			shownAtNotes = len(run.rec.Events)
		}
		return aiBody, nil
	}))
	run.deps.Yes = false
	run.deps.StdinInteractive = true
	return run, &shownAtNotes
}

func previewedGates(rec *presentertest.RecordingPresenter) []int {
	var at []int
	for i, ev := range rec.Events {
		if ev.Kind == presentertest.KindPreviewGate {
			at = append(at, i)
		}
	}
	return at
}

func TestRelease_AttendedDryRun_ShowsEachSuggestionAsItsPauseWouldBeforeTheNotesAreWritten(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		config   string
		question string
	}{
		{
			name:     "no keep list",
			question: "Exclude docs/design/ from the notes? (design documents; 2 files, 15 KB)",
		},
		{
			name:     "a matched file on the keep list",
			config:   "[release]\ndiff_keep = [\"docs/design/b.md\"]\n",
			question: "Exclude docs/design/ from the notes? (design documents; 2 files, 15 KB; on the keep list: docs/design/b.md)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run, shownAtNotes := newAttendedDryRun(t, tc.config)

			if err := run.releaseWith(t, dryRunOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			at := previewedGates(run.rec)
			if len(at) != 1 {
				t.Fatalf("previewed gates = %d, want one; kinds = %v", len(at), run.rec.Kinds())
			}
			gate := run.rec.Events[at[0]].PreviewGate
			if gate.Question != tc.question {
				t.Errorf("previewed question = %q, want %q", gate.Question, tc.question)
			}
			if !gate.Has(presenter.ChoiceExclude) || !gate.Has(presenter.ChoiceKeep) || gate.Default != presenter.ChoiceKeep {
				t.Errorf("previewed gate = %+v, want the pause's exclude/keep gate", gate)
			}
			if *shownAtNotes == -1 || at[0] >= *shownAtNotes {
				t.Errorf("previewed at event %d, notes call made after %d events; want the preview first", at[0], *shownAtNotes)
			}
			if at[0] == 0 || run.rec.Events[at[0]-1].Kind != presentertest.KindSuspendSpinner ||
				at[0]+1 >= len(run.rec.Events) || run.rec.Events[at[0]+1].Kind != presentertest.KindResumeSpinner {
				t.Errorf("want the preview inside a suspended spinner; kinds = %v", run.rec.Kinds())
			}
		})
	}
}

func TestRelease_AttendedDryRun_OffersNoChoiceAndReadsNoInput(t *testing.T) {
	t.Parallel()

	run, _ := newAttendedDryRun(t, "")

	if err := run.releaseWith(t, dryRunOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if gates := promptGates(run.rec); len(gates) != 0 {
		t.Errorf("an attended dry run offered a choice: %+v", gates)
	}
	if asks := countKind(run.rec, presentertest.KindAskLine); asks != 0 {
		t.Errorf("an attended dry run read input %d times", asks)
	}
	if got := suggestionEvents(run.rec); len(got) != 0 {
		t.Errorf("an attended dry run printed the unattended suggestions; kinds = %v", run.rec.Kinds())
	}
}

func TestRelease_AttendedDryRun_TheNotesCarryTheSuggestedFiles(t *testing.T) {
	t.Parallel()

	run := newSuggestedRun(t, "", designSuggestion, notesBody(aiBody, nil))
	run.deps.Yes = false
	run.deps.StdinInteractive = true

	if err := run.releaseWith(t, dryRunOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if len(run.tr.prompts) != 1 {
		t.Fatalf("notes calls = %d, want one", len(run.tr.prompts))
	}
	for _, path := range []string{"docs/design/a.md", "docs/design/b.md"} {
		if !strings.Contains(run.tr.prompts[0], "diff --git a/"+path+" b/"+path+"\n") {
			t.Errorf("the notes call does not carry %s's diff", path)
		}
	}
	assertNotesGitCallsExclude(t, run.f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml")
}

// unattendedDryRuns are the two ways a dry run runs unattended.
var unattendedDryRuns = []struct {
	name        string
	yes         bool
	interactive bool
}{
	{name: "-y at a terminal", yes: true, interactive: true},
	{name: "stdin not a terminal", interactive: false},
}

func TestRelease_UnattendedDryRun_PrintsEachSuggestionAfterTheNotesWithTheLineThatExcludesIt(t *testing.T) {
	t.Parallel()

	for _, mode := range unattendedDryRuns {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, "", designSuggestion, notesBody(aiBody, nil))
			run.deps.Yes = mode.yes
			run.deps.StdinInteractive = mode.interactive

			if err := run.releaseWith(t, dryRunOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			at, got := onlySuggestions(t, run.rec)
			assertSuggested(t, got, designSuggested)
			if shown := indexOfKind(run.rec, presentertest.KindShowNotes); shown == -1 || shown+1 != at {
				t.Errorf("want the suggestions straight after the notes; kinds = %v", run.rec.Kinds())
			}
			if previewed := previewedGates(run.rec); len(previewed) != 0 {
				t.Errorf("an unattended dry run previewed a pause; kinds = %v", run.rec.Kinds())
			}
			if gates := promptGates(run.rec); len(gates) != 0 {
				t.Errorf("an unattended dry run offered a choice: %+v", gates)
			}
		})
	}
}

func TestRelease_UnattendedDryRun_Fallback_PrintsTheSuggestionsAfterThePreviewedCommitSubjectList(t *testing.T) {
	t.Parallel()

	for _, mode := range unattendedDryRuns {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, fallbackConfig, designSuggestion, notesBody("", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))
			run.f.SeedSequence("git", ScriptedOut(commitSubjects)) // log --format=%s
			run.deps.Yes = mode.yes
			run.deps.StdinInteractive = mode.interactive

			if err := run.releaseWith(t, dryRunOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			at, got := onlySuggestions(t, run.rec)
			assertSuggested(t, got, designSuggested)
			shown := indexOfKind(run.rec, presentertest.KindShowNotes)
			if shown == -1 || run.rec.Events[shown].ShowNotes.Body != commitSubjects {
				t.Fatalf("want the commit-subject list previewed; kinds = %v", run.rec.Kinds())
			}
			if shown+1 != at {
				t.Errorf("want the suggestions straight after the commit-subject list; kinds = %v", run.rec.Kinds())
			}
		})
	}
}

func TestRelease_UnattendedDryRun_AtTheFloor_PrintsTheSuggestionsWithTheNextMoves(t *testing.T) {
	t.Parallel()

	for _, mode := range unattendedDryRuns {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, "[release]\non_notes_failure = \"abort\"\n", designSuggestion,
				notesBody("", &ai.GenerationError{Stdout: loginOutput, ExitCode: 1}))
			run.deps.Yes = mode.yes
			run.deps.StdinInteractive = mode.interactive

			err := run.releaseWith(t, dryRunOptions())

			var abort *engine.AbortError
			if !errors.As(err, &abort) || abort.ExitCode != 1 {
				t.Fatalf("err = %v, want an abort with exit 1", err)
			}
			at, got := onlySuggestions(t, run.rec)
			assertSuggested(t, got, designSuggested)
			if moves := indexOfKind(run.rec, presentertest.KindNextMoves); moves == -1 || moves+1 != at {
				t.Errorf("want the suggestions straight after the next moves; kinds = %v", run.rec.Kinds())
			}
			if gates := promptGates(run.rec); len(gates) != 0 {
				t.Errorf("an unattended dry run offered a choice: %+v", gates)
			}
		})
	}
}

func TestRelease_DryRun_NeitherModeStagesOrCommitsAnything(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		yes         bool
		interactive bool
	}{
		{name: "at a terminal without -y", interactive: true},
		{name: "-y", yes: true, interactive: true},
		{name: "stdin not a terminal", interactive: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := newSuggestedRun(t, "[release]\ndiff_exclude = [\"specs/\"]\n", designSuggestion, notesBody(aiBody, nil))
			run.deps.Yes = tc.yes
			run.deps.StdinInteractive = tc.interactive

			if err := run.releaseWith(t, dryRunOptions()); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			for _, inv := range invocationsNamed(run.f, "git") {
				if slices.Contains(inv.Args, "add") || slices.Contains(inv.Args, "commit") {
					t.Errorf("the dry run mutated the repo: %s", commandLine(inv))
				}
			}
		})
	}
}
