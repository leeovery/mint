package main

import (
	"context"
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

const (
	regenNotesBody  = "## What changed\n\n- hand-written notes  \n\n"
	githubOrigin    = "https://github.com/acme/widget.git\n"
	nonGithubOrigin = "https://gitlab.com/acme/widget.git\n"
)

func gitOut(stdout string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{Stdout: stdout}}
}

// seedRegenStartup expects root to hold no .mint.toml: with release_branch unset,
// the release branch is read from origin/HEAD.
func seedRegenStartup(f *runner.FakeRunner, root string, remote runner.ScriptedCall) {
	f.SeedSequence("git",
		gitOut(root+"\n"),          // rev-parse --show-toplevel
		gitOut("origin/main\n"),    // symbolic-ref --short refs/remotes/origin/HEAD
		gitOut("v1.3.0\nv1.4.0\n"), // tag --list
		remote,                     // remote get-url origin
	)
}

func writeRegenNotesFile(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func parseRegenArgs(t *testing.T, args ...string) regenerateRequest {
	t.Helper()
	req, err := parseRegenerateFlags(args)
	if err != nil {
		t.Fatalf("parseRegenerateFlags(%q) returned error: %v", args, err)
	}
	return req
}

func readFailure(t *testing.T, path string) string {
	t.Helper()
	_, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("reading %q succeeded; the case needs a path mint cannot read", path)
	}
	return err.Error()
}

func unusableRegenNotesFiles(t *testing.T) []struct{ name, path, wantMsg string } {
	t.Helper()
	dir := t.TempDir()
	empty := writeRegenNotesFile(t, t.TempDir(), "")
	blank := writeRegenNotesFile(t, t.TempDir(), " \n\t\n")
	missing := filepath.Join(dir, "missing.md")
	return []struct{ name, path, wantMsg string }{
		{name: "empty", path: empty, wantMsg: "--notes-file " + empty + " is empty"},
		{name: "whitespace-only", path: blank, wantMsg: "--notes-file " + blank + " is empty"},
		{name: "missing", path: missing, wantMsg: "reading --notes-file " + missing + ": " + readFailure(t, missing)},
		{name: "empty path", path: "", wantMsg: "reading --notes-file : " + readFailure(t, "")},
	}
}

// assertOnlyNotesRefusal fails unless the run exited 1 having run no command and
// shown only the one notes-stage refusal.
func assertOnlyNotesRefusal(t *testing.T, code int, rec *presentertest.RecordingPresenter, f *runner.FakeRunner, wantMsg string) {
	t.Helper()
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if got := f.Invocations(); len(got) != 0 {
		t.Errorf("commands ran = %+v, want none (no repo, branch, tag or remote read)", got)
	}
	if len(rec.Events) != 1 || rec.Events[0].Kind != presentertest.KindStageFailed {
		t.Fatalf("presenter events = %+v, want exactly one StageFailed", rec.Events)
	}
	want := presenter.StageFailure{Name: "notes", Message: wantMsg}
	if got := rec.Events[0].StageFailed; got != want {
		t.Errorf("StageFailed = %+v, want %+v", got, want)
	}
}

func promptSubjects(rec *presentertest.RecordingPresenter) []string {
	var subjects []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			subjects = append(subjects, ev.Prompt.Subject)
		}
	}
	return subjects
}

func warnings(rec *presentertest.RecordingPresenter) []presenter.Warning {
	var warns []presenter.Warning
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			warns = append(warns, ev.Warn)
		}
	}
	return warns
}

func invokedGit(f *runner.FakeRunner, args ...string) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Equal(inv.Args, args) {
			return true
		}
	}
	return false
}

func TestRegenerate_UnusableNotesFileWithUnresolvableProvider_IsTheOnlyThingReported(t *testing.T) {
	t.Parallel()

	origins := []struct {
		name   string
		remote runner.ScriptedCall
	}{
		{name: "no origin remote", remote: runner.ScriptedCall{Result: runner.Result{ExitCode: 2}, Err: errors.New("no such remote 'origin'")}},
		{name: "a non-github origin", remote: runner.ScriptedCall{Result: runner.Result{Stdout: nonGithubOrigin}}},
	}

	for _, origin := range origins {
		for _, file := range unusableRegenNotesFiles(t) {
			t.Run(origin.name+"/"+file.name, func(t *testing.T) {
				t.Parallel()

				f := runner.NewFakeRunner()
				seedRegenStartup(f, t.TempDir(), origin.remote)
				rec := &presentertest.RecordingPresenter{}
				req := parseRegenArgs(t, "1.4.0", "--notes-file", file.path)

				code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir()))

				assertOnlyNotesRefusal(t, code, rec, f, file.wantMsg)
			})
		}
	}
}

func TestRegenerate_UnusableNotesFileForAnUntaggedVersion_IsRefusedNotAUsageError(t *testing.T) {
	t.Parallel()

	for _, file := range unusableRegenNotesFiles(t) {
		t.Run(file.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			seedRegenStartup(f, t.TempDir(), runner.ScriptedCall{Result: runner.Result{ExitCode: 2}, Err: errors.New("no such remote 'origin'")})
			rec := &presentertest.RecordingPresenter{}
			req := parseRegenArgs(t, "9.9.9", "--notes-file", file.path)

			code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir()))

			assertOnlyNotesRefusal(t, code, rec, f, file.wantMsg)
		})
	}
}

// rewritingRemoteRunner rewrites a file when the origin remote is read, after the
// run has started, then delegates to the wrapped FakeRunner.
type rewritingRemoteRunner struct {
	*runner.FakeRunner
	path    string
	rewrite string
}

func (r *rewritingRemoteRunner) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	if name == "git" && slices.Contains(args, "remote") {
		if err := os.WriteFile(r.path, []byte(r.rewrite), 0o644); err != nil {
			return runner.Result{}, err
		}
	}
	return r.FakeRunner.Run(ctx, name, args...)
}

// Not parallel: t.Chdir sets the process working directory.
func TestRegenerate_RelativeNotesFile_ShipsTheBodyReadAtStartupWithNoSourceQuestion(t *testing.T) {
	runDir := t.TempDir()
	path := writeRegenNotesFile(t, runDir, regenNotesBody)
	t.Chdir(runDir)
	f := runner.NewFakeRunner()
	seedRegenStartup(f, t.TempDir(), gitOut(githubOrigin))
	f.Seed("gh", runner.Result{}, nil)
	const rewritten = "rewritten after startup\n"
	r := &rewritingRemoteRunner{FakeRunner: f, path: path, rewrite: rewritten}
	rec := &presentertest.RecordingPresenter{}
	req := parseRegenArgs(t, "1.4.0", "--notes-file", "notes.md", "--target", "release")

	if code := regenerate(t.Context(), rec, r, req, notescache.NewChunkNotes(t.TempDir())); code != 0 {
		t.Fatalf("exit code = %d, want 0; events %+v", code, rec.Events)
	}

	if got, want := promptSubjects(rec), []string{"notes"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v (no source question)", got, want)
	}
	var edits []runner.Invocation
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" && slices.Contains(inv.Args, "edit") {
			edits = append(edits, inv)
		}
	}
	if len(edits) != 1 {
		t.Fatalf("gh release edit ran %d times, want 1; got %+v", len(edits), f.Invocations())
	}
	if edits[0].Stdin != regenNotesBody {
		t.Errorf("provider release body = %q, want the body read at startup %q", edits[0].Stdin, regenNotesBody)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(data) != rewritten {
		t.Errorf("notes file = %q, want it left as the run found it %q", data, rewritten)
	}
}

func TestRegenerate_NoNotesFile_ResolvesThePublisherAndAsksTheSource(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedRegenStartup(f, t.TempDir(), gitOut(nonGithubOrigin))
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(presenter.Gate) (presenter.Choice, error) { return "", presenter.ErrInputClosed },
	}
	req := parseRegenArgs(t, "1.4.0")

	code := regenerate(t.Context(), rec, f, req, notescache.NewChunkNotes(t.TempDir()))

	if code != 1 {
		t.Errorf("exit code = %d, want 1 once the source question's input closes", code)
	}
	if !invokedGit(f, "tag", "--list") || !invokedGit(f, "remote", "get-url", "origin") {
		t.Errorf("commands ran = %+v, want the tag lookup and the remote read", f.Invocations())
	}
	warns := warnings(rec)
	if len(warns) != 1 || warns[0].Label != "publish skipped" {
		t.Errorf("warnings = %+v, want the one publish skipped downgrade", warns)
	}
	if got, want := promptSubjects(rec), []string{"source"}; !slices.Equal(got, want) {
		t.Errorf("gate subjects = %v, want %v", got, want)
	}
}
