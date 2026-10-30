package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// stashingRunner plays git's effects on .mint.toml, whose content at HEAD is
// committed, for the stash, the unwind's reset and the restore, which the fake does
// not touch; a conflicting pop leaves the file and the stash as they were. Each
// answers commit's .mint.toml is kept in answersCommits.
type stashingRunner struct {
	*runner.FakeRunner
	t              *testing.T
	root           string
	committed      string
	stash          []string
	answersCommits []string
}

func (r *stashingRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	if name == "git" && slices.Equal(args, []string{"-C", r.root, "commit", "-m", answersSubject(nextTag)}) {
		r.answersCommits = append(r.answersCommits, r.read())
	}
	res, err := r.FakeRunner.Run(ctx, name, args...)
	if name != "git" || err != nil {
		return res, err
	}
	switch {
	case slices.Equal(args, []string{"stash", "push", "--include-untracked"}):
		r.stash = append(r.stash, r.read())
		r.write(r.committed)
	case slices.Equal(args, []string{"reset", "--hard", startingSHA}):
		r.write(r.committed)
	case slices.Equal(args, []string{"stash", "pop"}):
		edit := r.stash[len(r.stash)-1]
		r.stash = r.stash[:len(r.stash)-1]
		r.write(r.read() + strings.TrimPrefix(edit, r.committed))
	}
	return res, nil
}

func (r *stashingRunner) read() string {
	got, err := os.ReadFile(filepath.Join(r.root, config.FileName))
	if err != nil {
		r.t.Errorf("reading %s: %v", config.FileName, err)
	}
	return string(got)
}

func (r *stashingRunner) write(contents string) {
	if err := os.WriteFile(filepath.Join(r.root, config.FileName), []byte(contents), 0o644); err != nil {
		r.t.Errorf("writing %s: %v", config.FileName, err)
	}
}

// stashedReleaseEdit and stashedCommitEdit are uncommitted edits to an empty
// committed .mint.toml: one adding the [release] list the answers also write, and one
// adding a [commit] list, the one a simulated clean restore can append to the answers
// and still decode.
const (
	stashedReleaseEdit = "[release]\ndiff_exclude = [\"specs/\"]\n"
	stashedCommitEdit  = "\n[commit]\ndiff_exclude = [\"specs/\"]\n"
)

// autostashedRelease is an attended --autostash release of nextTag whose work tree
// carries edit over an empty committed .mint.toml and whose operator excluded
// docs/design/ at the one pause.
type autostashedRelease struct {
	f    *runner.FakeRunner
	git  *stashingRunner
	deps engine.ReleaseDeps
}

// newAutostashedRelease readies the release through the notes and their answers
// commit; the caller scripts what follows, then the stash pop.
func newAutostashedRelease(t *testing.T, rec *presentertest.RecordingPresenter, edit string) *autostashedRelease {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, edit)
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"), // tag --list (a prior tag exists)
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
		stashSaved(),               // stash push --include-untracked
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean once stashed)
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v1.2.4 (absent)
		ScriptedOut("0\t1"),        // rev-list left-right count (ahead only)
		ScriptedOut(""),            // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture the clean start)
	)
	f.SeedSequence("git", withAnswersCommit(append([]runner.ScriptedCall{ScriptedOut(exportGoDiff + designDocDiff), ScriptedOut(exportGoDiff)}, exportOnlyChangeMap...)...)...)
	f.Seed("gh", runner.Result{}, nil)
	stashing := &stashingRunner{FakeRunner: f, t: t, root: root}
	deps := newDepsWithMutator(rec, f, git.NewMutator(stashing))
	deps.StdinInteractive = true
	deps.Transport = &answeredTransport{}
	return &autostashedRelease{f: f, git: stashing, deps: deps}
}

// released scripts the bookkeeping commit, tag and push, then pop as the restore.
func (a *autostashedRelease) released(pop runner.ScriptedCall) *autostashedRelease {
	seedRecordTagPush(a.f, a.git.root)
	a.f.SeedSequence("git", pop)
	return a
}

// unwound scripts the unwind's read of the answers commit's sha, its reset and its
// re-commit of the answers, then pop as the restore.
func (a *autostashedRelease) unwound(pop runner.ScriptedCall) *autostashedRelease {
	a.f.SeedSequence("git", ScriptedOut(answersSHA+"\n"), ScriptedOut(""), ScriptedOut(""), ScriptedOut(""), pop)
	return a
}

func (a *autostashedRelease) release(ctx context.Context) error {
	return engine.Release(ctx, a.deps, autostashOptions())
}

// assertOnlyTheAnswers fails unless contents, an answers commit's .mint.toml, holds
// the docs/design/ answer and nothing of a stashed edit.
func assertOnlyTheAnswers(t *testing.T, contents string) {
	t.Helper()
	cfg := loadConfigText(t, contents)
	if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("answers commit [release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if len(cfg.Commit.DiffExclude) != 0 {
		t.Errorf("answers commit [commit].diff_exclude = %q, want none of the stashed edit", cfg.Commit.DiffExclude)
	}
}

// loadConfigText decodes contents as a .mint.toml.
func loadConfigText(t *testing.T, contents string) config.Config {
	t.Helper()
	root := t.TempDir()
	writeConfig(t, root, contents)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("loading .mint.toml:\n%s\n%v", contents, err)
	}
	return cfg
}

// assertRestored fails unless the stash is empty and the work tree's .mint.toml
// carries both the answer and the stashed [commit] edit.
func assertRestored(t *testing.T, a *autostashedRelease) {
	t.Helper()
	if len(a.git.stash) != 0 {
		t.Errorf("stash entries = %d, want the clean restore to have dropped it", len(a.git.stash))
	}
	cfg := loadConfigText(t, a.git.read())
	if want := []string{"docs/design/"}; !slices.Equal(cfg.Release.DiffExclude, want) {
		t.Errorf("work tree [release].diff_exclude = %q, want %q", cfg.Release.DiffExclude, want)
	}
	if want := []string{"specs/"}; !slices.Equal(cfg.Commit.DiffExclude, want) {
		t.Errorf("work tree [commit].diff_exclude = %q, want the stashed edit's %q", cfg.Commit.DiffExclude, want)
	}
}

// assertKeptInStash fails unless the stashed edit is still the one stash entry, mint
// dropped nothing, and the run warned the restore did not apply.
func assertKeptInStash(t *testing.T, rec *presentertest.RecordingPresenter, a *autostashedRelease, edit string) {
	t.Helper()
	if !slices.Equal(a.git.stash, []string{edit}) {
		t.Errorf("stash = %q, want the stashed edit %q kept", a.git.stash, edit)
	}
	if n := countCmd(a.f, "git", "stash", "drop"); n != 0 {
		t.Errorf("git stash drop ran %d times, want none", n)
	}
	want := presenter.Warning{
		Label:   "autostash",
		Message: "could not restore stashed changes cleanly — your work is preserved in git stash; resolve manually",
	}
	if got := warnings(rec); !slices.Contains(got, want) {
		t.Errorf("warnings = %+v, want %+v", got, want)
	}
}

func TestRelease_Autostash_TheAnswersCommitHoldsTheAnswersAndNoneOfTheStashedEdit(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAutostashedRelease(t, rec, stashedReleaseEdit).released(stashPopped())

	if err := a.release(t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if len(a.git.answersCommits) != 1 {
		t.Fatalf("answers commits = %d, want one", len(a.git.answersCommits))
	}
	assertOnlyTheAnswers(t, a.git.answersCommits[0])
}

func TestRelease_Autostash_ACleanRestoreLeavesTheEditOverTheAnswersAndDropsTheStash(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAutostashedRelease(t, rec, stashedCommitEdit).released(stashPopped())

	if err := a.release(t.Context()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	commitAt := indexOfCmd(a.f, "git", "-C", a.git.root, "commit", "-m", answersSubject(nextTag))
	popAt := indexOfCmd(a.f, "git", "stash", "pop")
	if commitAt == -1 || popAt < commitAt {
		t.Errorf("stash pop (at %d) does not follow the answers commit (at %d)", popAt, commitAt)
	}
	assertOnlyTheAnswers(t, a.git.answersCommits[0])
	assertRestored(t, a)
	if got := warnings(rec); len(got) != 0 {
		t.Errorf("warnings = %+v, want none for a clean restore", got)
	}
}

func TestRelease_Autostash_AConflictingRestoreKeepsTheStashAndWarns(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	a := newAutostashedRelease(t, rec, stashedReleaseEdit).released(stashPopConflict())

	if err := a.release(t.Context()); err != nil {
		t.Fatalf("Release returned %v, want the conflicting restore to warn only", err)
	}

	assertKeptInStash(t, rec, a, stashedReleaseEdit)
	if len(a.git.answersCommits) != 1 {
		t.Fatalf("answers commits = %d, want one", len(a.git.answersCommits))
	}
	assertOnlyTheAnswers(t, a.git.answersCommits[0])
}

func TestRelease_Autostash_AbortAfterAnswering_RecommitsOnlyTheAnswersThenRestoresTheEdit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		edit   string
		pop    runner.ScriptedCall
		assert func(*testing.T, *presentertest.RecordingPresenter, *autostashedRelease)
	}{
		{
			name: "a clean restore",
			edit: stashedCommitEdit,
			pop:  stashPopped(),
			assert: func(t *testing.T, _ *presentertest.RecordingPresenter, a *autostashedRelease) {
				t.Helper()
				assertRestored(t, a)
			},
		},
		{
			name: "a conflicting restore",
			edit: stashedReleaseEdit,
			pop:  stashPopConflict(),
			assert: func(t *testing.T, rec *presentertest.RecordingPresenter, a *autostashedRelease) {
				t.Helper()
				assertKeptInStash(t, rec, a, stashedReleaseEdit)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, []presenter.Choice{presenter.ChoiceNo})
			a := newAutostashedRelease(t, rec, tc.edit).unwound(tc.pop)

			var abort *engine.AbortError
			if err := a.release(t.Context()); !errors.As(err, &abort) || abort.ExitCode != 1 {
				t.Fatalf("Release returned %v, want an abort exiting 1", err)
			}

			reset := indexOfCmd(a.f, "git", "reset", "--hard", startingSHA)
			if reset == -1 {
				t.Fatalf("no reset to the starting commit; got %v", commandLines(a.f.Invocations()))
			}
			want := []string{
				"git -C " + a.git.root + " add " + config.FileName,
				"git -C " + a.git.root + " commit -m " + answersSubject(nextTag),
				"git stash pop",
			}
			if got := commandLines(a.f.Invocations()[reset+1:]); !slices.Equal(got, want) {
				t.Errorf("after the reset = %q, want the answers re-committed then the stash restored %q", got, want)
			}
			if len(a.git.answersCommits) != 2 {
				t.Fatalf("answers commits = %d, want the first and its re-commit", len(a.git.answersCommits))
			}
			assertOnlyTheAnswers(t, a.git.answersCommits[1])
			tc.assert(t, rec, a)
		})
	}
}
