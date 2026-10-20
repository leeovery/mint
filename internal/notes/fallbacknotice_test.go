package notes_test

import (
	"errors"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// seedFailingNormalPath scripts a normal-AI run's diff assembly, then subjects as the
// commit-subject list's git log.
func seedFailingNormalPath(r *runner.FakeRunner, subjects string) {
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "M\ta.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t1\ta.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: subjects}},
	)
}

func TestSelector_WithFallbackNotice_HandsOverTheFailureTheCommitSubjectListShipsOver(t *testing.T) {
	t.Parallel()

	failure := &ai.GenerationError{Stdout: "Not logged in · Please run /login", ExitCode: 1}
	r := runner.NewFakeRunner()
	seedFailingNormalPath(r, "subject one\n")
	var noticed []error
	sel := newSelector(t, r, &recordingTransport{err: failure}).WithFallbackNotice(func(err error) { noticed = append(noticed, err) })

	body, kind, err := sel.SelectBody(t.Context(), notes.SelectState{LastTag: "v1.0.0"}, config.Config{Release: config.Release{OnNotesFailure: "fallback"}, MaxPromptBytes: config.DefaultMaxPromptBytes})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != "subject one\n" || kind != notes.KindFallback {
		t.Errorf("SelectBody = %q, %v, want the commit-subject list as KindFallback", body, kind)
	}
	if len(noticed) != 1 {
		t.Fatalf("notices = %d, want one", len(noticed))
	}
	var genErr *ai.GenerationError
	if !errors.As(noticed[0], &genErr) || genErr.Output() != failure.Output() {
		t.Errorf("noticed failure = %v, want the AI's failure", noticed[0])
	}
}

func TestSelector_WithFallbackNotice_NoNoticeWithoutAFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		transport      *recordingTransport
		onNotesFailure string
	}{
		{name: "the AI writes the notes", transport: &recordingTransport{body: "## notes\n"}, onNotesFailure: "fallback"},
		{name: "abort", transport: &recordingTransport{err: ai.ErrGenerationFailed}, onNotesFailure: "abort"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			seedFailingNormalPath(r, "subject one\n")
			noticed := 0
			sel := newSelector(t, r, tt.transport).WithFallbackNotice(func(error) { noticed++ })

			_, _, _ = sel.SelectBody(t.Context(), notes.SelectState{LastTag: "v1.0.0"}, config.Config{Release: config.Release{OnNotesFailure: tt.onNotesFailure}, MaxPromptBytes: config.DefaultMaxPromptBytes})

			if noticed != 0 {
				t.Errorf("notices = %d, want none", noticed)
			}
		})
	}
}
