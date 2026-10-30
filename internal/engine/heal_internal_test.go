package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/git"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/publish"
	"mint/internal/runner"
)

// savingChunkNotes fails every save with err.
type savingChunkNotes struct{ err error }

func (s savingChunkNotes) Save(string, string, string) (string, error) {
	return "", s.err
}

// failingPublisher fails the release probe, the dispatch's first call, with err.
type failingPublisher struct{ err error }

func (f failingPublisher) ReleaseExists(context.Context, string) (bool, error) {
	return false, f.err
}

func (failingPublisher) ReadReleaseBody(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (failingPublisher) CreateRelease(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (failingPublisher) UpdateRelease(context.Context, string, string, string) (string, error) {
	return "", nil
}

// fixedRegenerator writes the notes again as body.
type fixedRegenerator struct{ body string }

func (r fixedRegenerator) Regenerate(context.Context, string) (string, error) {
	return r.body, nil
}

// fixedEditor saves the notes as body.
type fixedEditor struct{ body string }

func (e fixedEditor) Edit(context.Context, string) (string, error) {
	return e.body, nil
}

// TestRegenerateWrite_AHealWritesTheReleaseFromTheNotesJustPushed pins the heal a
// failed release write past the changelog's push names, one rule whatever the source:
// the notes the changelog was pushed with, as reviewed, saved to the tag's heal file
// and passed with --notes-file. Reading the source again would not give them: fresh
// notes the `r` wrote again, supplied notes edited at the gate, and notes taken at the
// notes floor are each the reviewed body alone.
func TestRegenerateWrite_AHealWritesTheReleaseFromTheNotesJustPushed(t *testing.T) {
	t.Parallel()

	const (
		read     = "Initial release.\n"
		reviewed = "Webhooks retry on their own now.\n"
	)
	for _, tc := range []struct {
		name    string
		source  RegenerateSource
		choices []presenter.Choice
		deps    ReleaseDeps
		pushed  string
	}{
		{
			name:    "fresh notes the r wrote again",
			source:  RegenerateSourceFresh,
			choices: []presenter.Choice{presenter.ChoiceRegen, presenter.ChoiceYes},
			deps:    ReleaseDeps{Regenerator: fixedRegenerator{body: reviewed}},
			pushed:  reviewed,
		},
		{
			name:    "supplied notes edited at the gate",
			source:  regenerateSourceSupplied,
			choices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
			deps:    ReleaseDeps{Editor: fixedEditor{body: reviewed}},
			pushed:  reviewed,
		},
		{
			name:    "notes taken at the floor",
			source:  regenerateSourceFloor,
			choices: []presenter.Choice{presenter.ChoiceYes},
			pushed:  read,
		},
		{
			name:    "the tag's notes, confirmed",
			source:  RegenerateSourceTag,
			choices: []presenter.Choice{presenter.ChoiceYes},
			pushed:  read,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, cache := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			f := runner.NewFakeRunner()
			f.SeedSequence("git",
				runner.ScriptedCall{Result: runner.Result{Stdout: "startHEAD"}},  // rev-parse HEAD
				runner.ScriptedCall{Result: runner.Result{Stdout: "2024-02-15"}}, // the tag's date
				runner.ScriptedCall{}, // add
				runner.ScriptedCall{}, // commit
				runner.ScriptedCall{}, // push: the changelog is public
			)
			rec := &presentertest.RecordingPresenter{NextChoices: tc.choices, NextLines: []string{""}}
			deps := tc.deps
			deps.Presenter, deps.Runner, deps.Mutator = rec, f, git.NewMutator(f)
			deps.ChunkNotes = notescache.NewChunkNotes(cache)
			req := RegenerateWriteRequest{Source: tc.source, Target: RegenerateTargetBoth, Tag: "v1.4.0", VersionKey: "1.4.0", Body: read}

			if err := RegenerateWrite(t.Context(), deps, failingPublisher{err: errors.New("HTTP 502")}, root, req); err != nil {
				t.Fatalf("RegenerateWrite = %v, want nil: a release write past the push only warns", err)
			}

			heal := healMoves(t, rec)
			saved := strings.TrimPrefix(heal.Command, "mint release regenerate v1.4.0 --target release --notes-file ")
			want := presenter.NextMove{Lead: "Re-heal the provider release from the notes just pushed, saved to " + saved + ":", Command: heal.Command}
			if filepath.Base(saved) != "v1.4.0.heal.md" || heal != want {
				t.Errorf("heal = %+v, want the release written from the tag's heal file", heal)
			}
			if warned := publishWarning(t, rec); warned.Message != "changelog is already pushed; re-heal the provider release with "+heal.Command {
				t.Errorf("plain's warning = %q, want it to re-heal with the heal's command, the file named", warned.Message)
			}
			if got, err := os.ReadFile(saved); err != nil || string(got) != tc.pushed {
				t.Errorf("heal notes at %s = %q, %v, want the notes pushed, %q", saved, got, err, tc.pushed)
			}
		})
	}
}

// TestHealMove_WithNowhereToSaveTheNotesTheLeadSaysHowToHeal pins the heal where the
// pushed notes can't be saved: a lead alone, naming the notes the changelog holds and
// the command that writes the release from a file of them.
func TestHealMove_WithNowhereToSaveTheNotesTheLeadSaysHowToHeal(t *testing.T) {
	t.Parallel()

	want := presenter.NextMove{Lead: "Save the notes CHANGELOG.md now holds for v1.4.0 to a file, then re-heal the GitHub release with it (`mint release regenerate v1.4.0 --target release --notes-file <file>`)"}
	for _, tc := range []struct {
		name  string
		notes ChunkNotesSaver
	}{
		{name: "no chunk notes store"},
		{name: "the save fails", notes: savingChunkNotes{err: errors.New("no space left on device")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, with := healMove(ReleaseDeps{ChunkNotes: tc.notes}, "/repo", "v1.4.0", "Initial release.\n", "GitHub release")
			if got != want {
				t.Errorf("heal = %+v, want %+v", got, want)
			}
			if want := "mint release regenerate v1.4.0 --target release --notes-file <file>, the notes CHANGELOG.md now holds for v1.4.0 saved to <file>"; with != want {
				t.Errorf("plain re-heals with %q, want %q", with, want)
			}
		})
	}
}

// publishWarning is the post-push release warning recorded.
func publishWarning(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Warning {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Key == stagePublish {
			return ev.Warn
		}
	}
	t.Fatalf("no publish warning recorded; kinds = %v", rec.Kinds())
	return presenter.Warning{}
}

// healMoves is the one move the post-push release warning recorded.
func healMoves(t *testing.T, rec *presentertest.RecordingPresenter) presenter.NextMove {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNextMoves && ev.NextMoves.Stage == stagePublish && len(ev.NextMoves.Moves) == 1 {
			return ev.NextMoves.Moves[0]
		}
	}
	t.Fatalf("no publish heal recorded; kinds = %v", rec.Kinds())
	return presenter.NextMove{}
}

// TestHealMove_TheNotesAreSavedBesideTheTagsPartNotesNeverOverThem pins that a heal
// keeps the part notes an earlier failed run saved for the tag: both files survive,
// each with its own notes, and the move points at the heal's.
func TestHealMove_TheNotesAreSavedBesideTheTagsPartNotesNeverOverThem(t *testing.T) {
	t.Parallel()

	const (
		partNotes = "## part 1 of 2\n- Retries.\n"
		body      = "Webhooks retry on their own now.\n"
	)
	store := notescache.NewChunkNotes(t.TempDir())
	parts, err := store.Save("/repo", "v1.4.0", partNotes)
	if err != nil {
		t.Fatal(err)
	}
	move, _ := healMove(ReleaseDeps{ChunkNotes: store}, "/repo", "v1.4.0", body, "GitHub release")

	healed := filepath.Join(filepath.Dir(parts), "v1.4.0.heal.md")
	if want := "mint release regenerate v1.4.0 --target release --notes-file " + healed; move.Command != want {
		t.Errorf("heal command = %q, want %q", move.Command, want)
	}
	for path, want := range map[string]string{parts: partNotes, healed: body} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v, want %q", path, got, err, want)
		}
	}
}

// TestPublishFailureOutput_IsGhsStderrElseTheFailuresText pins what a post-push
// publish warning shows beneath its line: gh's stderr verbatim, or, where gh wrote
// nothing, the failure's own text.
func TestPublishFailureOutput_IsGhsStderrElseTheFailuresText(t *testing.T) {
	t.Parallel()

	const stderr = "HTTP 502: Bad Gateway (https://api.github.com/repos/acme/pigeon/releases)\n"
	for _, tc := range []struct {
		name, stderr, want string
	}{
		{name: "gh wrote its diagnostics", stderr: stderr, want: stderr},
		{name: "gh wrote nothing", want: `creating GitHub release for tag "v1.4.0": exit status 1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := runner.NewFakeRunner()
			f.Seed("gh", runner.Result{Stderr: tc.stderr, ExitCode: 1}, errors.New("exit status 1"))
			_, err := publish.NewGitHubPublisher(f).CreateRelease(t.Context(), "v1.4.0", "v1.4.0", "notes")
			if got := publishFailureOutput(err); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}
