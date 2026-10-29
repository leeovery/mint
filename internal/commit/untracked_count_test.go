package commit_test

// This file pins how the preflight counts the untracked files -A will stage for the diff
// node's row: from the disk, from the one listing git gives, as git's numstat counts
// them, never failing the run and never a git call per file.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// untrackedRow runs `mint commit -A --no-ai` in a repo where git lists listing as the
// untracked files and nothing tracked has changed, whatever write puts on the disk under
// the root, its editor saving nothing so the run stops once the diff has settled, and
// returns the diff node's row and git's calls.
func untrackedRow(t *testing.T, listing string, write func(root string)) (string, []runner.Invocation) {
	t.Helper()
	root := t.TempDir()
	write(root)
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		scriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD
		scriptedStdout(""),         // diff HEAD --numstat -- .
		scriptedStdout(listing),    // ls-files --others --exclude-standard -z -- .
		scriptedStdout("myedit\n"), // var GIT_EDITOR
	)
	er := &editorRunner{fake: f, saved: ""}
	rec := &presentertest.RecordingPresenter{}
	err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.AddAll, NoAI: true}))
	if err == nil || len(stageFailures(rec)) != 0 {
		t.Fatalf("Run returned %v with failures %+v, want the empty save's no-op and no failure", err, stageFailures(rec))
	}
	return settles(rec)[0].Detail, gitInvocationsOf(f.Invocations())
}

func putFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_UntrackedFilesCountAsGitsNumstatCountsThem(t *testing.T) {
	t.Parallel()

	const everything = " · everything, staged when you accept"
	for _, tc := range []struct {
		name    string
		content []byte
		// want is the row git's `diff --no-index --numstat -- /dev/null` makes it.
		want string
	}{
		{name: "text, its lines", content: []byte("a\nb\nc\n"), want: "1 file · +3 −0"},
		{name: "no newline at its end, its last line counted", content: []byte("a\nb\nc"), want: "1 file · +3 −0"},
		{name: "empty, no lines", content: nil, want: "1 file · +0 −0"},
		{name: "binary, a NUL in its first 8000 bytes, no lines", content: append([]byte("a\nb\n"), 0, 1, 2), want: "1 file · +0 −0"},
		{name: "a NUL past its first 8000 bytes, text", content: append([]byte(strings.Repeat("x", 7999)+"\n"), 0, '\n'), want: "1 file · +2 −0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			row, _ := untrackedRow(t, "new.dat\x00", func(root string) { putFile(t, filepath.Join(root, "new.dat"), tc.content) })
			if row != tc.want+everything {
				t.Errorf("diff row = %q, want %q", row, tc.want+everything)
			}
		})
	}
}

func TestRun_AnUntrackedNestedRepoIsOneFileOfNoLines(t *testing.T) {
	t.Parallel()

	row, _ := untrackedRow(t, "vendor/lib/\x00", func(root string) {
		putFile(t, filepath.Join(root, "vendor", "lib", ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	})
	if want := "1 file · +0 −0 · everything, staged when you accept"; row != want {
		t.Errorf("diff row = %q, want %q: a gitlink, which `git add -A` stages", row, want)
	}
}

func TestRun_AnUntrackedFileGoneSinceItWasListedIsLeftOutOfTheCount(t *testing.T) {
	t.Parallel()

	row, _ := untrackedRow(t, "gone.go\x00kept.go\x00", func(root string) {
		putFile(t, filepath.Join(root, "kept.go"), []byte("package kept\n"))
	})
	if want := "1 file · +1 −0 · everything, staged when you accept"; row != want {
		t.Errorf("diff row = %q, want %q, the run going on", row, want)
	}
	// What git listed decides whether there is anything to commit, not what could be
	// read to count it: a run whose one untracked file has gone still goes on.
	if row, _ := untrackedRow(t, "gone.go\x00", func(string) {}); row != "0 files · +0 −0 · everything, staged when you accept" {
		t.Errorf("diff row = %q, want nothing counted and the run going on", row)
	}
}

func TestRun_UntrackedFilesAreCountedWithOneGitCall(t *testing.T) {
	t.Parallel()

	_, gits := untrackedRow(t, "a.go\x00b.go\x00c.go\x00", func(root string) {
		for _, name := range []string{"a.go", "b.go", "c.go"} {
			putFile(t, filepath.Join(root, name), []byte("package x\n"))
		}
	})
	var verbs [][]string
	for _, inv := range gits {
		verbs = append(verbs, inv.Args)
	}
	want := [][]string{
		{"diff", "HEAD", "--numstat", "--", "."},
		{"ls-files", "--others", "--exclude-standard", "-z", "--", "."},
		{"var", "GIT_EDITOR"},
	}
	if !slices.EqualFunc(verbs, want, slices.Equal) {
		t.Errorf("git calls = %q, want the preflight's two probes and no call per untracked file, then the editor's %q", verbs, want)
	}
}

// TestRun_AnUntrackedNestedRepoCommits pins the runs a nested repo used to fail at the
// preflight once it counted each untracked file with git: they commit, as they did
// before the count.
func TestRun_AnUntrackedNestedRepoCommits(t *testing.T) {
	t.Parallel()

	nested := func(t *testing.T) string {
		root := t.TempDir()
		putFile(t, filepath.Join(root, "vendor", "lib", ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
		return root
	}

	t.Run("excluded from the AI under -A -y", func(t *testing.T) {
		t.Parallel()

		root := nested(t)
		writeMintTOML(t, root, "diff_exclude = [\"vendor/**\"]\n")
		f := runner.NewFakeRunner()
		f.SeedSequence("git",
			scriptedHead(),                          // rev-parse HEAD --abbrev-ref HEAD
			scriptedStdout("1\t0\tsrc/export.go\n"), // diff HEAD --numstat -- .
			scriptedStdout("vendor/lib/\x00"),       // ls-files --others (the preflight's, unfiltered)
			scriptedStdout(commitExportDiff),        // diff HEAD -- . :(exclude)vendor/**
			scriptedStdout(""),                      // ls-files --others :(exclude)vendor/**
			runner.ScriptedCall{},                   // add -A
			runner.ScriptedCall{},                   // commit -F -
			scriptedShortHead(),                     // rev-parse --short HEAD
		)
		rec := &presentertest.RecordingPresenter{} // unscripted → the default, as -y takes it
		deps := newCommitDeps(rec, f, scriptedTransport("feat: export"), root)
		deps.Staging, deps.Yes = commit.AddAll, true

		if err := commit.Run(context.Background(), deps); err != nil {
			t.Fatalf("Run returned %v, want the commit made", err)
		}
		if commits := gitVerbInvocations(f.Invocations(), "commit"); len(commits) != 1 {
			t.Errorf("commits = %v, want one", commits)
		}
		if row := settles(rec)[0].Detail; row != "2 files · +1 −0 · everything, staged when you accept" {
			t.Errorf("diff row = %q, want the nested repo counted as one file", row)
		}
	})

	t.Run("under --no-ai -A", func(t *testing.T) {
		t.Parallel()

		f := runner.NewFakeRunner()
		f.SeedSequence("git",
			scriptedHead(),                    // rev-parse HEAD --abbrev-ref HEAD
			scriptedStdout(""),                // diff HEAD --numstat -- .
			scriptedStdout("vendor/lib/\x00"), // ls-files --others
			scriptedStdout("myedit\n"),        // var GIT_EDITOR
			runner.ScriptedCall{},             // add -A
			runner.ScriptedCall{},             // commit -F -
			scriptedShortHead(),               // rev-parse --short HEAD
		)
		er := &editorRunner{fake: f, saved: "chore: vendor lib\n"}
		rec := &presentertest.RecordingPresenter{}
		err := commit.Run(context.Background(), editorDeps(rec, er, editorDepsOptions{Root: nested(t), Staging: commit.AddAll, NoAI: true}))
		if err != nil {
			t.Fatalf("Run returned %v, want the commit made", err)
		}
		if commits := editorCommitInvocations(er); len(commits) != 1 || commits[0].Stdin != "chore: vendor lib\n" {
			t.Errorf("commits = %v, want the one written in the editor", commits)
		}
	})
}
