package commit_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/commit"
	"mint/internal/runner"
)

// The editor resolver delegates to `git var GIT_EDITOR`, which returns exactly the
// editor `git commit` would launch — git itself applies the full precedence
// (GIT_EDITOR → core.editor → $VISUAL → $EDITOR → built-in default). These tests
// therefore confirm DELEGATION: each case scripts the `git var GIT_EDITOR` stdout to
// the editor git would pick for that precedence tier and asserts ResolveEditor
// returns it. The precedence ordering itself is git's job, not mint's.

// TestResolveEditor_GitEditorWins scripts `git var GIT_EDITOR` returning the
// GIT_EDITOR value (git's highest-precedence source) and asserts the resolver
// returns it — GIT_EDITOR wins over core.editor, $VISUAL, and $EDITOR.
func TestResolveEditor_GitEditorWins(t *testing.T) {
	t.Parallel()

	const want = "emacs"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: want + "\n"}, nil)

	got, err := commit.ResolveEditor(context.Background(), r)
	if err != nil {
		t.Fatalf("ResolveEditor returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ResolveEditor = %q, want %q (the GIT_EDITOR value git var resolved)", got, want)
	}

	assertGitVarGitEditor(t, r)
}

// TestResolveEditor_CoreEditorWins scripts `git var GIT_EDITOR` returning the
// core.editor value (what git resolves when GIT_EDITOR is unset) and asserts the
// resolver returns it — core.editor wins over $VISUAL and $EDITOR.
func TestResolveEditor_CoreEditorWins(t *testing.T) {
	t.Parallel()

	const want = "nvim"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: want + "\n"}, nil)

	got, err := commit.ResolveEditor(context.Background(), r)
	if err != nil {
		t.Fatalf("ResolveEditor returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ResolveEditor = %q, want %q (the core.editor value git var resolved)", got, want)
	}
}

// TestResolveEditor_VisualWins scripts `git var GIT_EDITOR` returning the $VISUAL
// value (what git resolves when nothing higher is set) and asserts the resolver
// returns it — $VISUAL wins over $EDITOR.
func TestResolveEditor_VisualWins(t *testing.T) {
	t.Parallel()

	const want = "code --wait"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: want + "\n"}, nil)

	got, err := commit.ResolveEditor(context.Background(), r)
	if err != nil {
		t.Fatalf("ResolveEditor returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ResolveEditor = %q, want %q (the $VISUAL value git var resolved, args preserved)", got, want)
	}
}

// TestResolveEditor_UnsetEditorFallsToGitDefault scripts `git var GIT_EDITOR`
// returning git's built-in default (what git supplies when nothing in the chain is
// set, e.g. $EDITOR unset) and asserts the resolver returns it WITHOUT an error.
// An unset $EDITOR is not an error on a TTY — git's default still applies.
func TestResolveEditor_UnsetEditorFallsToGitDefault(t *testing.T) {
	t.Parallel()

	const want = "vi"
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: want + "\n"}, nil)

	got, err := commit.ResolveEditor(context.Background(), r)
	if err != nil {
		t.Fatalf("ResolveEditor returned unexpected error for an unset $EDITOR (git's default applies): %v", err)
	}
	if got != want {
		t.Errorf("ResolveEditor = %q, want git's built-in default %q", got, want)
	}
}

// TestResolveEditor_NoLaunchableEditor scripts `git var GIT_EDITOR` FAILING — the
// case where git itself cannot resolve a usable editor (e.g. a dumb terminal with
// nothing configured) — and asserts the resolver returns the distinguished
// not-launchable sentinel (matched via errors.Is), NOT a launch attempt, NOT a panic.
func TestResolveEditor_NoLaunchableEditor(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "terminal is dumb, but EDITOR unset", ExitCode: 128}, errors.New("exit status 128"))

	got, err := commit.ResolveEditor(context.Background(), r)
	if !errors.Is(err, commit.ErrNoEditor) {
		t.Fatalf("ResolveEditor error = %v, want it to match commit.ErrNoEditor", err)
	}
	if got != "" {
		t.Errorf("ResolveEditor returned %q alongside the not-launchable signal; it must return an empty editor", got)
	}
}

// TestResolveEditor_BlankGitVarYieldsSentinel scripts `git var GIT_EDITOR`
// SUCCEEDING (nil error) but naming no editor — git exited clean yet stdout trims to
// empty. This is the trim-to-empty branch (editor.go ~64-67): a clean exit with a
// blank value is equivalently "no launchable candidate", so the resolver must return
// the distinguished not-launchable sentinel (matched via errors.Is) and an empty
// editor, NOT a launch attempt. The "\n" and "   " variants lock that the value is
// trimmed before the emptiness check, guarding the spec's "fails OR returns blank"
// contract against regression.
func TestResolveEditor_BlankGitVarYieldsSentinel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stdout string
	}{
		{name: "NewlineOnly", stdout: "\n"},
		{name: "WhitespaceOnly", stdout: "   "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{Stdout: tt.stdout}, nil)

			got, err := commit.ResolveEditor(context.Background(), r)
			if !errors.Is(err, commit.ErrNoEditor) {
				t.Fatalf("ResolveEditor error = %v, want it to match commit.ErrNoEditor for a clean exit naming no editor", err)
			}
			if got != "" {
				t.Errorf("ResolveEditor returned %q alongside the not-launchable signal; it must return an empty editor", got)
			}
		})
	}
}

// TestResolveEditor_DoesNotLaunchStageOrCommit proves the resolver is interrogation
// ONLY: it runs the read-only `git var GIT_EDITOR` and nothing else — no interactive
// editor launch (RunInteractive), no `git add` staging, no `git commit`.
func TestResolveEditor_DoesNotLaunchStageOrCommit(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "vi\n"}, nil)

	if _, err := commit.ResolveEditor(context.Background(), r); err != nil {
		t.Fatalf("ResolveEditor returned unexpected error: %v", err)
	}

	for _, inv := range r.Invocations() {
		if len(inv.Args) > 0 && inv.Args[0] == "add" {
			t.Errorf("ResolveEditor ran `git %v`; it must not stage", inv.Args)
		}
		if len(inv.Args) > 0 && inv.Args[0] == "commit" {
			t.Errorf("ResolveEditor ran `git %v`; it must not commit", inv.Args)
		}
	}

	// The only invocation must be the single read-only `git var GIT_EDITOR`; an
	// interactive launch would record an invocation with no further git read.
	if len(r.Invocations()) != 1 {
		t.Errorf("ResolveEditor made %d invocations (%v), want exactly 1 (git var GIT_EDITOR)", len(r.Invocations()), r.Invocations())
	}
}

// assertGitVarGitEditor fails unless the resolver's single recorded call was the
// read-only `git var GIT_EDITOR` interrogation — the delegation the whole design
// rests on.
func assertGitVarGitEditor(t *testing.T, r *runner.FakeRunner) {
	t.Helper()

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("ResolveEditor made %d invocations (%v), want exactly 1", len(invs), invs)
	}
	inv := invs[0]
	if inv.Name != "git" {
		t.Errorf("invocation name = %q, want %q", inv.Name, "git")
	}
	want := []string{"var", "GIT_EDITOR"}
	if len(inv.Args) != len(want) {
		t.Fatalf("args = %v, want %v", inv.Args, want)
	}
	for i := range want {
		if inv.Args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q (full argv %v)", i, inv.Args[i], want[i], inv.Args)
		}
	}
}
