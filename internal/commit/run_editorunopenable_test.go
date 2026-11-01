package commit_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"mint/internal/ai"
	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// The ways on from an editor that could not be opened.
const (
	editorMoveLead = "Set an editor git can open, in GIT_EDITOR, core.editor, VISUAL or EDITOR (git falls back to vi), then run it again:"
	ownMessageLead = "Or commit with a message of your own:"
)

// errNotStarted is a launch the editor never started from, as the runner reports one.
var errNotStarted = fmt.Errorf("running %q: %w: %w", "/opt/subl", runner.ErrNotStarted, errors.New("fork/exec /opt/subl: permission denied"))

func nextMoves(rec *presentertest.RecordingPresenter) []presenter.NextMove {
	var moves []presenter.NextMove
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNextMoves {
			moves = append(moves, ev.NextMoves.Moves...)
		}
	}
	return moves
}

// seedAIFailedThenEditor scripts a staged-only run whose AI call fails: the start
// station, the preflight's count, the AI's diff, then git naming the editor, or
// failing to name one where editor is empty.
func seedAIFailedThenEditor(editor string) *runner.FakeRunner {
	named := runner.ScriptedCall{Result: runner.Result{Stdout: editor + "\n"}}
	if editor == "" {
		named = runner.ScriptedCall{Result: runner.Result{Stderr: "terminal is dumb", ExitCode: 128}, Err: errExitOne}
	}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		scriptedHead(),
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t0\tx\n"}},                 // git diff --cached --numstat -- .
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // git diff --cached -- .
		named, // git var GIT_EDITOR
	)
	return f
}

// An attended run whose AI fails and whose editor can't be opened fails on the message
// node: the AI's cause, the editor and why it did not open, and the ways on, with
// nothing staged or committed.
func TestRun_AIFailedAndEditorUnopenable_FailsLoudSayingWhy(t *testing.T) {
	t.Parallel()

	causes := []struct {
		name   string
		err    error
		phrase string
	}{
		{name: "a failed call", err: &ai.GenerationError{Stdout: "API Error: 529 overloaded", ExitCode: 1}, phrase: "its call failed"},
		{name: "an empty message", err: &ai.GenerationError{}, phrase: "it returned an empty message"},
		{name: "a timeout", err: fmt.Errorf("%w: after 1m0s", ai.ErrTimeout), phrase: "it timed out"},
		{name: "a missing command", err: fmt.Errorf("%w: claude", ai.ErrCommandMissing), phrase: "its command is not installed"},
	}
	editors := []struct {
		name     string
		editor   string
		launch   error
		clause   string
		output   string
		launches int
	}{
		{name: "git names none", clause: "git names no editor to open"},
		{name: "not found", editor: "nvim", launch: wrapNotFound("nvim"), clause: `the editor "nvim" could not be opened (not found)`, launches: 1},
		{name: "never started", editor: "/opt/subl -w", launch: errNotStarted, clause: `the editor "/opt/subl" could not be opened (it failed to start)`, output: errNotStarted.Error(), launches: 1},
	}
	for _, cause := range causes {
		for _, ed := range editors {
			t.Run(cause.name+", "+ed.name, func(t *testing.T) {
				t.Parallel()

				rec := &presentertest.RecordingPresenter{}
				er := &editorRunner{fake: seedAIFailedThenEditor(ed.editor), launchErr: ed.launch}
				deps := editorDeps(rec, er, editorDepsOptions{Transport: &recordingTransport{err: fmt.Errorf("generating commit message: %w", cause.err)}, Root: t.TempDir()})

				err := commit.Run(context.Background(), deps)

				if err == nil {
					t.Fatal("Run returned nil, want the run to fail")
				}
				want := presenter.StageFailure{
					Name:    "editor",
					Key:     "message",
					Message: "the AI could not write the message (" + cause.phrase + "), and " + ed.clause,
					Output:  ed.output,
				}
				if got := stageFailures(rec); len(got) != 1 || got[0] != want {
					t.Errorf("failures = %+v, want only %+v", got, want)
				}
				wantMoves := []presenter.NextMove{
					{Lead: editorMoveLead, Command: "mint commit"},
					{Lead: ownMessageLead, Command: `git commit -m "<message>"`},
				}
				if got := nextMoves(rec); !slices.Equal(got, wantMoves) {
					t.Errorf("next moves = %+v, want %+v", got, wantMoves)
				}
				if len(er.launches) != ed.launches {
					t.Errorf("editor launches = %d, want %d", len(er.launches), ed.launches)
				}
				if adds, commits := editorAddInvocations(er), editorCommitInvocations(er); len(adds)+len(commits) != 0 {
					t.Errorf("git add %v, git commit %v; want neither", adds, commits)
				}
			})
		}
	}
}

// The ways on run this commit again, and commit what it would carry, as its flags
// stage and push it.
func TestRun_EditorUnopenable_TheWaysOnFollowTheFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		set   func(*commit.Deps)
		again string
		own   string
	}{
		{name: "-a", set: func(d *commit.Deps) { d.Staging = commit.All }, again: "mint commit -a", own: `git commit -a -m "<message>"`},
		{name: "-Ap", set: func(d *commit.Deps) { d.Staging, d.Push = commit.AddAll, true }, again: "mint commit -A -p", own: `git add -A && git commit -m "<message>" && git push`},
		{name: "-p --no-ai", set: func(d *commit.Deps) { d.Push, d.NoAI = true, true }, again: "mint commit -p --no-ai", own: `git commit -m "<message>" && git push`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{}, nil)
			f.SeedSequence("git", scriptedHead(), runner.ScriptedCall{Result: runner.Result{Stdout: "1\t0\tx\n"}})
			f.SeedCall("git", []string{"var", "GIT_EDITOR"}, runner.ScriptedCall{Result: runner.Result{Stdout: "nvim\n"}})
			rec := &presentertest.RecordingPresenter{}
			er := &editorRunner{fake: f, launchErr: wrapNotFound("nvim")}
			deps := editorDeps(rec, er, editorDepsOptions{Transport: &recordingTransport{err: ai.ErrTimeout}, Root: t.TempDir()})
			tt.set(&deps)

			if err := commit.Run(context.Background(), deps); err == nil {
				t.Fatal("Run returned nil, want the run to fail")
			}

			want := []presenter.NextMove{{Lead: editorMoveLead, Command: tt.again}, {Lead: ownMessageLead, Command: tt.own}}
			if got := nextMoves(rec); !slices.Equal(got, want) {
				t.Errorf("next moves = %+v, want %+v", got, want)
			}
		})
	}
}

// A regeneration that fails leaves no message either: an editor that can't be opened
// then fails on the message node with the regeneration's cause.
func TestRun_RegenerateFailedAndEditorUnopenable_FailsLoudSayingWhy(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		scriptedHead(),
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t0\tx\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // the first message's diff
		runner.ScriptedCall{Result: runner.Result{Stdout: "diff --git a/x b/x\n+work"}}, // the r's diff
		runner.ScriptedCall{Result: runner.Result{Stdout: "nvim\n"}},                    // git var GIT_EDITOR
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceRegen}}
	er := &editorRunner{fake: f, launchErr: wrapNotFound("nvim")}
	transport := &sequenceTransport{answers: []aiAnswer{{body: "feat: first\n"}, {err: fmt.Errorf("%w: after 1m0s", ai.ErrTimeout)}}}

	err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Transport: transport, Root: t.TempDir()}))

	if err == nil {
		t.Fatal("Run returned nil, want the run to fail")
	}
	want := `the AI could not write the message (it timed out), and the editor "nvim" could not be opened (not found)`
	if got := stageFailedMessages(rec); len(got) != 1 || got[0] != want {
		t.Errorf("failures = %q, want only %q", got, want)
	}
}

// An editor that never started is one `e` could not open: the review keeps its
// message, as it does for one that is missing.
func TestRun_EditWithAnEditorThatNeverStarted_KeepsTheMessage(t *testing.T) {
	t.Parallel()

	const generated = "feat: the generated message\n"
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes}}
	er := &editorRunner{fake: seedEditMissingBinaryThenAccept("diff --git a/x b/x\n+work", "/opt/subl"), launchErr: errNotStarted}

	if err := commit.Run(context.Background(), editDeps(rec, er, scriptedTransport(generated), t.TempDir())); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	warns := editorWarnings(rec)
	if len(warns) != 1 || warns[0].Message != "could not launch editor, keeping the message" {
		t.Errorf("warnings = %+v, want the editor's one", warns)
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != generated {
		t.Errorf("commits = %+v, want one carrying the unedited message", commits)
	}
}

// sequenceTransport answers each message call with the next of its answers.
type sequenceTransport struct {
	answers []aiAnswer
}

func (s *sequenceTransport) Generate(context.Context, string) (string, error) {
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a.body, a.err
}
