package commit_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// commitExportDiff is the post-exclusion diff git returns once the lockfile and tmp/ are
// excluded: src/export.go alone.
const commitExportDiff = "diff --git a/src/export.go b/src/export.go\n@@ -0,0 +1 @@\n+package src\n"

func writeMintTOML(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

func TestRun_CommitDiffExclude_AddsToTheSharedListOnTheAIDiff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintTOML(t, root, "diff_exclude = [\"*.lock\"]\n\n[commit]\ndiff_exclude = [\"tmp/\"]\n")

	const message = "feat: export"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		scriptedHead(), // git rev-parse HEAD --abbrev-ref HEAD (the start station)
		runner.ScriptedCall{Result: runner.Result{Stdout: "app.lock\ntmp/run.log\nsrc/export.go\n"}}, // preflight probe (unfiltered)
		runner.ScriptedCall{Result: runner.Result{Stdout: commitExportDiff}},                         // L1 staged diff (filtered)
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),   // git rev-parse --short HEAD (the end station)
	)
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := gitInvocationsOf(r.Invocations())
	if len(gits) < 2 {
		t.Fatalf("git invocations = %v, want the preflight probe then the L1 diff", gits)
	}
	assertArgs(t, gits[0].Args, []string{"diff", "--cached", "--numstat", "--", "."})
	assertArgs(t, gits[1].Args, []string{"diff", "--cached", "--", ".", ":(exclude)*.lock", ":(exclude)tmp/"})

	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, commitExportDiff) {
		t.Errorf("commit AI call does not carry the src/export.go diff:\n%s", prompt)
	}
	for _, excluded := range []string{"app.lock", "tmp/run.log"} {
		if strings.Contains(prompt, excluded) {
			t.Errorf("commit AI call names excluded path %q:\n%s", excluded, prompt)
		}
	}

	if commitInv := findCommitInvocation(t, r); commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want %q", commitInv.Stdin, message)
	}
	for _, inv := range gits {
		if len(inv.Args) > 0 && inv.Args[0] == "add" {
			t.Errorf("StagedOnly ran %v; the whole staged index is committed as staged", inv.Args)
		}
	}
}

func TestRun_CommitDiffExclude_AddAllKeepsTheUntrackedExclusionOutOfTheAIDiffButStagesIt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintTOML(t, root, "[commit]\ndiff_exclude = [\"tmp/\"]\n")

	const message = "feat: export"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		scriptedHead(), // git rev-parse HEAD --abbrev-ref HEAD (the start station)
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t0\tsrc/export.go\n"}}, // preflight tracked probe (unfiltered): non-empty
		runner.ScriptedCall{Result: runner.Result{Stdout: "tmp/new.log\x00"}},       // preflight untracked probe (unfiltered): tmp/new.log
		scriptedCount("tmp/new.log"),                                         // preflight count of tmp/new.log
		runner.ScriptedCall{Result: runner.Result{Stdout: commitExportDiff}}, // L1 tracked diff (filtered)
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},               // L1 untracked enumeration (filtered): tmp/new.log excluded
		runner.ScriptedCall{},                                                // git add -A
		runner.ScriptedCall{},                                                // git commit -F -
		scriptedShortHead(),                                                  // git rev-parse --short HEAD (the end station)
	)
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.AddAll

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := gitInvocationsOf(r.Invocations())
	if len(gits) != 7 {
		t.Fatalf("git invocations = %v, want the probes and the untracked file's count, tracked diff, untracked enumeration, add, commit", gits)
	}
	assertArgs(t, gits[0].Args, []string{"diff", "HEAD", "--numstat", "--", "."})
	assertArgs(t, gits[1].Args, []string{"ls-files", "--others", "--exclude-standard", "-z", "--", "."})
	assertArgs(t, gits[2].Args, []string{"diff", "--no-index", "--numstat", "--", "/dev/null", "tmp/new.log"})
	assertArgs(t, gits[3].Args, []string{"diff", "HEAD", "--", ".", ":(exclude)tmp/"})
	assertArgs(t, gits[4].Args, []string{"ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude)tmp/"})
	if args := gits[5].Args; args[len(args)-1] != "-A" || !slices.Contains(args, "add") {
		t.Errorf("staging call = %v, want `git add -A`", args)
	}

	prompt := transport.lastPrompt(t)
	if !strings.Contains(prompt, commitExportDiff) {
		t.Errorf("commit AI call does not carry the src/export.go diff:\n%s", prompt)
	}
	if strings.Contains(prompt, "tmp/new.log") {
		t.Errorf("commit AI call names the excluded untracked tmp/new.log:\n%s", prompt)
	}
	if commitInv := findCommitInvocation(t, r); commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want %q", commitInv.Stdin, message)
	}
}

func TestRun_CommitDiffExclude_AllExcludedOpensTheEditorAndCommits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintTOML(t, root, "[commit]\ndiff_exclude = [\"tmp/\"]\n")

	const saved = "chore: keep the run log\n"
	rec := &presentertest.RecordingPresenter{}
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		scriptedHead(), // git rev-parse HEAD --abbrev-ref HEAD (the start station)
		runner.ScriptedCall{Result: runner.Result{Stdout: "tmp/run.log\n"}}, // preflight probe (unfiltered): dirty
		runner.ScriptedCall{Result: runner.Result{Stdout: ""}},              // L1 staged diff (filtered): all excluded
		runner.ScriptedCall{Result: runner.Result{Stdout: "myedit\n"}},      // git var GIT_EDITOR
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),   // git rev-parse --short HEAD (the end station)
	)
	er := &editorRunner{fake: f, saved: saved}
	transport := scriptedTransport("must never be returned (all excluded)")
	deps := editorDeps(rec, er, editorDepsOptions{Root: root, Staging: commit.StagedOnly, Transport: transport})

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	gits := editorGitInvocations(er)
	if len(gits) < 2 {
		t.Fatalf("git invocations = %v, want the preflight probe then the L1 diff", gits)
	}
	assertArgs(t, gits[0].Args, []string{"diff", "--cached", "--numstat", "--", "."})
	assertArgs(t, gits[1].Args, []string{"diff", "--cached", "--", ".", ":(exclude)tmp/"})

	if transport.calls() != 0 {
		t.Errorf("transport called %d times; an all-excluded diff makes no AI call", transport.calls())
	}
	assertWarnings(t, warnEvents(rec), []presenter.Warning{{Label: "diff", Message: "all changes are excluded from the AI context — opening editor", Key: "message"}})
	if len(er.launches) != 1 {
		t.Fatalf("editor launches = %d, want 1", len(er.launches))
	}
	commits := editorCommitInvocations(er)
	if len(commits) != 1 || commits[0].Stdin != saved {
		t.Fatalf("commit invocations = %v, want exactly one carrying %q", commits, saved)
	}
}

func TestRun_MakesOneAICallForTheMessageAndNoExclusionCheck(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMintTOML(t, root, "[release]\ndiff_keep = [\"docs/api/\"]\n")

	const message = "feat: export"
	rec := &presentertest.RecordingPresenter{}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		scriptedHead(), // git rev-parse HEAD --abbrev-ref HEAD (the start station)
		runner.ScriptedCall{Result: runner.Result{Stdout: "src/export.go\n"}}, // preflight probe
		runner.ScriptedCall{Result: runner.Result{Stdout: commitExportDiff}},  // L1 staged diff
		runner.ScriptedCall{}, // git commit -F -
		scriptedShortHead(),   // git rev-parse --short HEAD (the end station)
	)
	transport := scriptedTransport(message)
	deps := newCommitDeps(rec, r, transport, root)
	deps.Staging = commit.StagedOnly

	if err := commit.Run(context.Background(), deps); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := transport.calls(); got != 1 {
		t.Fatalf("AI calls = %d, want only the commit message's", got)
	}
	if prompt := transport.lastPrompt(t); !strings.Contains(prompt, commitExportDiff) {
		t.Errorf("the one AI call is not the commit message call over the staged diff:\n%s", prompt)
	}
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNotesChecking || ev.Kind == presentertest.KindNotesChecked {
			t.Errorf("the commit showed the exclusion check's line: %v", ev.Kind)
		}
	}
	if commitInv := findCommitInvocation(t, r); commitInv.Stdin != message {
		t.Errorf("commit stdin = %q, want %q", commitInv.Stdin, message)
	}
}
