package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const regenFloorSubjects = "feat: add export\nfix: handle empty input\n"

func TestRegenerate_AttendedFreshFailure_PutsTheVersionsCommitSubjectsOnIt(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedRegenStartup(f, t.TempDir(), gitOut(githubOrigin))
	f.SeedSequence("git",
		gitOut("diff --git a/a.go b/a.go\n+change\n"), // diff v1.3.0..v1.4.0
		gitOut("M\ta.go\n"),                           // --name-status
		gitOut("1\t0\ta.go\n"),                        // --numstat
		gitOut(regenFloorSubjects),                    // log --format=%s v1.3.0..v1.4.0
	)
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("claude", runner.Result{Stdout: "API Error: usage limit reached"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceCommitSubjects, presenter.ChoiceYes},
	}
	req := parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release")
	req.StdinInteractive = true

	if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 0 {
		t.Fatalf("exit code = %d, want 0; events %+v", code, rec.Events)
	}

	if !invokedGit(f, "log", "--format=%s", "v1.3.0..v1.4.0") {
		t.Errorf("no `git log --format=%%s v1.3.0..v1.4.0`; got %+v", f.Invocations())
	}
	var writes []runner.Invocation
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" && slices.Contains(inv.Args, "release") && (slices.Contains(inv.Args, "edit") || slices.Contains(inv.Args, "create")) {
			writes = append(writes, inv)
		}
	}
	if len(writes) != 1 || writes[0].Stdin != regenFloorSubjects {
		t.Errorf("provider release writes = %+v, want one carrying %q", writes, regenFloorSubjects)
	}
}

func TestRegenerate_UnattendedFreshFailure_OffersNoChoice(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedRegenStartup(f, t.TempDir(), gitOut(githubOrigin))
	f.SeedSequence("git",
		gitOut("diff --git a/a.go b/a.go\n+change\n"),
		gitOut("M\ta.go\n"),
		gitOut("1\t0\ta.go\n"),
	)
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("claude", runner.Result{Stdout: "API Error: usage limit reached"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}
	req := parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release", "-y")
	req.StdinInteractive = true

	if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if got := promptSubjects(rec); len(got) != 0 {
		t.Errorf("gate subjects = %v, want no gate under -y", got)
	}
}

func TestRegenerate_UnattendedFreshFailure_FitsTheNextMovesToTheLoadedConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte("ai_command = \"mytool --print\"\n"), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
	f := runner.NewFakeRunner()
	seedRegenStartup(f, root, gitOut(githubOrigin))
	f.SeedSequence("git",
		gitOut("diff --git a/a.go b/a.go\n+change\n"),
		gitOut("M\ta.go\n"),
		gitOut("1\t0\ta.go\n"),
	)
	f.Seed("gh", runner.Result{}, nil)
	f.Seed("mytool", runner.Result{Stdout: "API Error: usage limit reached"}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}
	req := parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release", "-y")

	if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	var moves []presenter.NextMoves
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNextMoves {
			moves = append(moves, ev.NextMoves)
		}
	}
	want := []string{
		"write the notes to a file and release with mint release regenerate 1.4.0 --notes-file <file>",
		`if ai_command carries a large overhead of its own, use the lean command documented in the README under "The shipped default"`,
	}
	if len(moves) != 1 || !slices.Equal(moves[0].Moves, want) {
		t.Errorf("NextMoves = %+v, want one carrying %q", moves, want)
	}
}
