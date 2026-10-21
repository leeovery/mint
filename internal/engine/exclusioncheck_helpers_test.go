package engine_test

import (
	"strings"
	"testing"

	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// checkPromptOpening opens every exclusion check prompt and no other prompt.
const checkPromptOpening = "A software release's notes are about to be written from its diff."

func isCheckPrompt(prompt string) bool {
	return strings.HasPrefix(prompt, checkPromptOpening)
}

// checkInvocations returns name's invocations that carried an exclusion check prompt.
func checkInvocations(f *runner.FakeRunner, name string) []runner.Invocation {
	var out []runner.Invocation
	for _, inv := range invocationsNamed(f, name) {
		if isCheckPrompt(inv.Stdin) {
			out = append(out, inv)
		}
	}
	return out
}

// notesInvocations returns name's invocations other than the exclusion check's.
func notesInvocations(f *runner.FakeRunner, name string) []runner.Invocation {
	var out []runner.Invocation
	for _, inv := range invocationsNamed(f, name) {
		if !isCheckPrompt(inv.Stdin) {
			out = append(out, inv)
		}
	}
	return out
}

// notesStdinOf is stdinOf over the invocations other than the exclusion check's.
func notesStdinOf(t *testing.T, f *runner.FakeRunner, name string, args ...string) string {
	t.Helper()
	want := name + " " + strings.Join(args, " ")
	for _, inv := range notesInvocations(f, name) {
		if commandLine(inv) == want {
			return inv.Stdin
		}
	}
	t.Fatalf("no notes invocation %q recorded; got %v", want, commandLines(f.Invocations()))
	return ""
}

// seedClaudeAfterCheck scripts claude to answer the exclusion check "none", then each
// of calls in turn.
func seedClaudeAfterCheck(f *runner.FakeRunner, calls ...runner.ScriptedCall) {
	f.SeedSequence("claude", append([]runner.ScriptedCall{ScriptedOut("none")}, calls...)...)
}

// plainCheckLine is the line plain mode prints for the exclusion check's call.
const plainCheckLine = "notes: checking for files that don't belong in the notes"

// afterCheck fails unless stage opens with the exclusion check's call, and returns the
// rest of the stage.
func afterCheck(t *testing.T, stage []presentertest.Event) []presentertest.Event {
	t.Helper()
	if len(stage) < 2 || stage[0].Kind != presentertest.KindNotesChecking || stage[1].Kind != presentertest.KindNotesChecked {
		t.Fatalf("notes stage events = %v, want the exclusion check's call first", kindsOf(stage))
	}
	return stage[2:]
}

// releaseNotesStage is the run's one notes stage after the exclusion check's call.
func releaseNotesStage(t *testing.T, rec *presentertest.RecordingPresenter) []presentertest.Event {
	t.Helper()
	return afterCheck(t, onlyNotesStage(t, rec))
}

// withoutPlainCheckLine fails unless the exclusion check's line follows the notes
// stage's start in lines, and returns lines without it.
func withoutPlainCheckLine(t *testing.T, lines []string) []string {
	t.Helper()
	if len(lines) < 2 || lines[0] != "notes: running..." || lines[1] != plainCheckLine {
		t.Fatalf("notes lines =\n%s\nwant the start, then the exclusion check's line", strings.Join(lines, "\n"))
	}
	return append([]string{lines[0]}, lines[2:]...)
}
