package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// chunkedMergeBody is what the merge call returns on an over-cap release.
const chunkedMergeBody = "✨ Added\n- The api and core areas landed together   \n"

// seedOverCapNotes scripts the notes assembly for a two-file diff of about 10 KB a
// file, then the AI's answers: two chunk calls' notes and the merge output last.
func seedOverCapNotes(f *runner.FakeRunner) {
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
	)
	f.SeedSequence("claude",
		ScriptedOut("- api notes"),
		ScriptedOut("- core notes"),
		ScriptedOut(chunkedMergeBody),
	)
}

func TestRelease_OverCap_ChunkedNotesShipByteIdenticalToEverySink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "max_prompt_bytes = 20000\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedOverCapNotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	var aiCalls []runner.Invocation
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			aiCalls = append(aiCalls, inv)
		}
	}
	if len(aiCalls) != 3 {
		t.Fatalf("AI calls = %d, want two chunk calls and one merge", len(aiCalls))
	}
	for i, inv := range aiCalls {
		if len(inv.Stdin) > 20_000 {
			t.Errorf("AI call %d is %d bytes, over the 20000-byte cap", i, len(inv.Stdin))
		}
	}
	if merge := aiCalls[2].Stdin; !strings.Contains(merge, "- api notes") || !strings.Contains(merge, "- core notes") {
		t.Errorf("the last call is not the merge over both parts' notes")
	}

	if got := tagAnnotationBody(t, f, nextTag); got != chunkedMergeBody {
		t.Errorf("tag annotation body = %q, want the merge output %q", got, chunkedMergeBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != chunkedMergeBody {
		t.Errorf("CHANGELOG body = %q, want the merge output %q", got, chunkedMergeBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != chunkedMergeBody {
		t.Errorf("provider release body = %q, want the merge output %q", got, chunkedMergeBody)
	}
}

func TestRelease_OverCap_ChunkedNotesReachTheReviewGateAsAINotesWithNoPromptBeforeThem(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "max_prompt_bytes = 20000\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedOverCapNotes(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	gate := promptGate(t, rec)
	want := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen}
	if got := keysOf(gate); !equalChoices(got, want) {
		t.Errorf("chunked notes gate keys = %v, want the AI-notes y/n/e/r gate %v", got, want)
	}

	// Between the version gate and the notes review gate nothing is asked: chunking
	// needs no consent.
	var subjects []string
	for _, g := range promptGates(rec) {
		subjects = append(subjects, g.Subject)
	}
	if len(subjects) != 2 || subjects[1] != gate.Subject {
		t.Errorf("gates asked = %v, want only the version gate then the notes review gate", subjects)
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindAskLine {
			t.Errorf("an over-cap release read a line before its notes; chunking asks nothing")
		}
	}
}
