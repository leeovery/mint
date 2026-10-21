package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/notescache"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

func TestRegenerate_ChunkedFreshRunFails_SavesTheFinishedPartsNotesInTheGivenStore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte("max_prompt_bytes = 20000\n"), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
	chunkNotesBase := t.TempDir()
	f := runner.NewFakeRunner()
	seedRegenStartup(f, root, gitOut(githubOrigin))
	f.SeedSequence("git",
		gitOut(twoPartDiff()),                           // diff v1.3.0..v1.4.0
		gitOut("M\tapi/a.go\nM\tcore/b.go\n"),           // --name-status
		gitOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"), // --numstat
	)
	f.Seed("gh", runner.Result{}, nil)
	// Whichever part call arrives first returns its notes and the other fails, its
	// retry with it.
	f.SeedSequence("claude", gitOut("- part notes"), aiFailed(), aiFailed())
	rec := &presentertest.RecordingPresenter{}
	req := parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release", "-y")

	if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(chunkNotesBase)); code != 1 {
		t.Fatalf("exit code = %d, want 1; events %v", code, rec.Kinds())
	}

	scope := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	path := filepath.Join(chunkNotesBase, scope, "v1.4.0.md")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the saved chunk notes: %v", err)
	}
	if !strings.Contains(string(saved), "- part notes") {
		t.Errorf("saved chunk notes = %q, want the finished part's notes", saved)
	}
	failures := stageFailures(rec)
	if len(failures) != 1 || failures[0].SavedNotes != path {
		t.Errorf("StageFailures = %+v, want one naming %q", failures, path)
	}
}

func TestRegenerate_UnloadableConfig_StopsBeforeAnyTagReadOrAICall(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config string
	}{
		{name: "unterminated quote in [release] ai_command", config: "[release]\nai_command = 'claude --system-prompt \"Be terse'\n"},
		{name: "removed max_diff_lines", config: "max_diff_lines = 50000\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte(tt.config), 0o644); err != nil {
				t.Fatalf("writing .mint.toml: %v", err)
			}
			f := runner.NewFakeRunner()
			f.SeedSequence("git", gitOut(root+"\n")) // rev-parse --show-toplevel
			rec := &presentertest.RecordingPresenter{}
			req := parseRegenArgs(t, "1.4.0", "--source", "fresh", "--target", "release")

			if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code == 0 {
				t.Fatalf("exit code = 0, want non-zero; events %+v", rec.Events)
			}

			got := f.Invocations()
			if len(got) != 1 || got[0].Name != "git" || !slices.Equal(got[0].Args, []string{"rev-parse", "--show-toplevel"}) {
				t.Errorf("commands ran = %+v, want only `git rev-parse --show-toplevel` (no tag read, no AI call)", got)
			}
		})
	}
}
