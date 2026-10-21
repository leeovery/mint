package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/notescache"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const regenNotes = "## notes\n"

// regenFreshVersionGit scripts one fresh version's notes diff and Change Map, whose
// path neither an exclusion nor the keep list covers.
func regenFreshVersionGit(f *runner.FakeRunner, path string) string {
	diff := "diff --git a/" + path + " b/" + path + "\n@@ -0,0 +1 @@\n+change\n"
	f.SeedSequence("git", gitOut(diff), gitOut("A\t"+path+"\n"), gitOut("1\t0\t"+path+"\n"))
	return diff
}

// writeKeepConfig gives root a .mint.toml whose [release] lists leave the fresh
// versions' paths uncovered.
func writeKeepConfig(t *testing.T, root string) {
	t.Helper()
	config := "diff_exclude = [\"*.lock\"]\n\n[release]\ndiff_exclude = [\"specs/\"]\ndiff_keep = [\"docs/api/\"]\n"
	if err := os.WriteFile(filepath.Join(root, ".mint.toml"), []byte(config), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

// assertOnlyNotesCalls fails unless claude ran once per diff, each call carrying its
// version's diff in order, and the run showed no exclusion check line.
func assertOnlyNotesCalls(t *testing.T, f *runner.FakeRunner, rec *presentertest.RecordingPresenter, diffs ...string) {
	t.Helper()
	var calls []runner.Invocation
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			calls = append(calls, inv)
		}
	}
	if len(calls) != len(diffs) {
		t.Fatalf("claude calls = %d, want only the %d notes calls", len(calls), len(diffs))
	}
	for i, diff := range diffs {
		if !strings.Contains(calls[i].Stdin, diff) {
			t.Errorf("claude call %d is not the notes call over its version's diff:\n%s", i+1, calls[i].Stdin)
		}
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNotesChecking || ev.Kind == presentertest.KindNotesChecked {
			t.Errorf("the run showed the exclusion check's line: %v", ev.Kind)
		}
	}
}

func TestRegenerate_FreshSingleVersion_MakesNoCheckCall(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "attended", args: []string{"1.4.0", "--source", "fresh", "--target", "release"}},
		{name: "-y", args: []string{"1.4.0", "--source", "fresh", "--target", "release", "-y"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeKeepConfig(t, root)
			f := runner.NewFakeRunner()
			seedRegenStartup(f, root, gitOut(githubOrigin))
			diff := regenFreshVersionGit(f, "src/export.go")
			f.Seed("gh", runner.Result{}, nil)
			f.Seed("claude", runner.Result{Stdout: regenNotes}, nil)
			rec := &presentertest.RecordingPresenter{}
			req := parseRegenArgs(t, tt.args...)
			req.StdinInteractive = true

			if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 0 {
				t.Fatalf("exit code = %d, want 0; events %+v", code, rec.Events)
			}

			assertOnlyNotesCalls(t, f, rec, diff)
		})
	}
}

func TestRegenerate_FreshAll_MakesNoCheckCallForAnyVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "attended", args: []string{"--all", "--source", "fresh", "--target", "release"}},
		{name: "-y", args: []string{"--all", "--source", "fresh", "--target", "release", "-y"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeKeepConfig(t, root)
			f := runner.NewFakeRunner()
			f.SeedSequence("git",
				gitOut(root+"\n"),                  // rev-parse --show-toplevel
				gitOut("origin/main\n"),            // symbolic-ref --short refs/remotes/origin/HEAD
				gitOut("v1.2.0\nv1.3.0\nv1.4.0\n"), // tag --list
				gitOut(githubOrigin),               // remote get-url origin
			)
			// v1.2.0 is the oldest release, whose fixed body needs no diff.
			first := regenFreshVersionGit(f, "src/export.go")
			second := regenFreshVersionGit(f, "src/import.go")
			f.Seed("gh", runner.Result{}, nil)
			f.Seed("claude", runner.Result{Stdout: regenNotes}, nil)
			rec := &presentertest.RecordingPresenter{}
			req := parseRegenArgs(t, tt.args...)
			req.StdinInteractive = true

			if code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir())); code != 0 {
				t.Fatalf("exit code = %d, want 0; events %+v", code, rec.Events)
			}

			assertOnlyNotesCalls(t, f, rec, first, second)
		})
	}
}
