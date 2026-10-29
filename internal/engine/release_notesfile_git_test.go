package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// excludePathspec is the pathspec that leaves a repo-relative notes file out of a
// git step.
func excludePathspec(rel string) string {
	return ":(top,literal,exclude)" + rel
}

// inRepoNotesFileOptions releases with a notes file named relative to workDir.
func inRepoNotesFileOptions(workDir, notesFile string) engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, NotesFile: notesFile, WorkDir: workDir}
}

// argsOf returns the args of every git invocation whose args start with prefix.
func argsOf(f *runner.FakeRunner, prefix ...string) [][]string {
	var got [][]string
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) >= len(prefix) && slices.Equal(inv.Args[:len(prefix)], prefix) {
			got = append(got, inv.Args)
		}
	}
	return got
}

// assertGitArgs fails unless exactly one git invocation starts with want's first
// prefixLen args, and that invocation carries want exactly.
func assertGitArgs(t *testing.T, f *runner.FakeRunner, prefixLen int, want []string) {
	t.Helper()
	got := argsOf(f, want[:prefixLen]...)
	if len(got) != 1 || !slices.Equal(got[0], want) {
		t.Errorf("git %q invocations = %q, want exactly git %q", want[:prefixLen], got, want)
	}
}

// assertNotesFileOnlyExcluded fails if any git invocation takes the notes file's path,
// repo-relative or absolute, as a bare argument.
func assertNotesFileOnlyExcluded(t *testing.T, f *runner.FakeRunner, rel string) {
	t.Helper()
	for _, inv := range f.Invocations() {
		for _, arg := range inv.Args {
			if arg == rel || strings.HasSuffix(arg, "/"+rel) {
				t.Errorf("git step names the notes file outside its exclusion: %s", commandLine(inv))
			}
		}
	}
}

// stagedChangelog is the staged-path list the bookkeeping commit reads after it
// stages the changelog.
const stagedChangelog = "CHANGELOG.md\x00"

// stagedArtifacts is the staged-path list the artifact commit reads after a pre_tag
// hook rewrote bundle.js.
const stagedArtifacts = "bundle.js\x00"

// committedChangelog and committedArtifacts are the literal pathspecs the commits
// pipe for those staged lists.
const (
	committedChangelog = ":(top,literal)CHANGELOG.md\x00"
	committedArtifacts = ":(top,literal)bundle.js\x00"
)

// keepOutCommit scripts a commit that keeps the notes file out. notesStaged is the
// staged-edit probe's output: empty sends the commit down the as-built route, and
// otherwise the commit reads staged, the other staged paths, and names them.
func keepOutCommit(notesStaged, staged string) []runner.ScriptedCall {
	if notesStaged == "" {
		return []runner.ScriptedCall{
			ScriptedOut(""), // -C root diff --cached -- <notes> (no staged edits)
			ScriptedOut(""), // -C root commit -m
		}
	}
	return []runner.ScriptedCall{
		ScriptedOut(notesStaged), // -C root diff --cached -- <notes> (staged edits)
		ScriptedOut(staged),      // -C root diff --cached --name-only -- :(exclude)<notes>
		ScriptedOut(""),          // commit --pathspec-from-file
	}
}

// assertCommitAsBuilt fails unless the run probed the notes file for staged edits
// and made the as-built commit under subject.
func assertCommitAsBuilt(t *testing.T, f *runner.FakeRunner, root, rel, subject string) {
	t.Helper()
	probe := []string{"-C", root, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", ":(top,literal)" + rel}
	if !invokedWith(f, "git", probe...) {
		t.Errorf("no staged-edit probe git %q; got %v", probe, commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "-C", root, "commit", "-m", subject) {
		t.Errorf("no as-built commit %q; got %v", subject, commandLines(f.Invocations()))
	}
}

// assertCommitKeepsNotesOut fails unless the run read the staged paths with the notes
// file excluded and then committed under subject with wantStdin as its pathspecs.
func assertCommitKeepsNotesOut(t *testing.T, f *runner.FakeRunner, root, rel, subject, wantStdin string) {
	t.Helper()
	read := []string{"-C", root, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", excludePathspec(rel)}
	if !invokedWith(f, "git", read...) {
		t.Errorf("no staged-path read git %q; got %v", read, commandLines(f.Invocations()))
	}
	commit := []string{"-C", root, "commit", "-m", subject, "--pathspec-from-file=-", "--pathspec-file-nul"}
	if got := stdinOf(t, f, "git", commit...); got != wantStdin {
		t.Errorf("commit %q stdin = %q, want %q", subject, got, wantStdin)
	}
}

// seedInRepoNotesFileGit scripts a prior-tag supplied-body release whose bookkeeping
// commit keeps an in-repo notes file out; notesStaged is as for keepOutCommit.
func seedInRepoNotesFileGit(f *runner.FakeRunner, root, notesStaged string) {
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut(""),            // ls-files --ignored (the notes file is not ignored)
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"), // tag --list
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v1.2.4
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(startingSHA),   // rev-parse HEAD
		ScriptedOut(""),            // -C root add CHANGELOG.md
	)
	f.SeedSequence("git", keepOutCommit(notesStaged, stagedChangelog)...)
	f.SeedSequence("git",
		ScriptedOut(githubRemoteURL), // remote get-url origin
		ScriptedOut(""),              // tag -a v1.2.4 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v1.2.4
	)
	f.Seed("gh", runner.Result{}, nil)
}

// firstReleaseGates scripts a first release's read gates, with an optional stash
// push outcome before the fetch. ignored is the ignored-path probe's output: empty
// for a notes file git does not ignore.
func firstReleaseGates(root, ignored string, stash ...runner.ScriptedCall) []runner.ScriptedCall {
	calls := []runner.ScriptedCall{
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut(ignored),       // ls-files --ignored
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
	}
	calls = append(calls, stash...)
	return append(calls,
		ScriptedOut(""),          // fetch --tags
		ScriptedOut(""),          // status --porcelain
		ScriptedOut("main"),      // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),        // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),      // rev-list left-right count
		ScriptedOut(""),          // ls-remote --tags
		ScriptedOut(startingSHA), // rev-parse HEAD
	)
}

// firstReleaseTail scripts the bookkeeping commit with a notes file that has no
// staged edits kept out, then the tag and push.
func firstReleaseTail() []runner.ScriptedCall {
	return firstReleaseTailFor("")
}

// firstReleaseTailFor is firstReleaseTail with notesStaged as for keepOutCommit.
func firstReleaseTailFor(notesStaged string) []runner.ScriptedCall {
	calls := []runner.ScriptedCall{ScriptedOut("")} // -C root add CHANGELOG.md
	calls = append(calls, keepOutCommit(notesStaged, stagedChangelog)...)
	return append(calls,
		ScriptedOut(githubRemoteURL), // remote get-url origin
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	)
}

// bareFirstReleaseTail scripts the bookkeeping commit with no exclusion, then the
// tag and push.
func bareFirstReleaseTail() []runner.ScriptedCall {
	return []runner.ScriptedCall{
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	}
}

func assertFileUnchanged(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q after the run, want it untouched as %q", path, got, want)
	}
}

func assertRunFinished(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish; last event = %v", fin.Kind)
	}
}

func TestRelease_NotesFileInRepo_KeptOutOfTheCleanTreeCheckAndBookkeepingCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		notesStaged string
	}{
		{name: "an untracked notes.md at the repo root", file: "notes.md"},
		{name: "a tracked NEXT_RELEASE.md with unstaged edits", file: "NEXT_RELEASE.md"},
		{name: "a tracked NEXT_RELEASE.md with staged edits", file: "NEXT_RELEASE.md", notesStaged: "NEXT_RELEASE.md\x00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := writeNotesFile(t, root, tt.file, suppliedBody)
			f := runner.NewFakeRunner()
			seedInRepoNotesFileGit(f, root, tt.notesStaged)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, tt.file)); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertGitArgs(t, f, 2, []string{"status", "--porcelain", "--", excludePathspec(tt.file)})
			assertGitArgs(t, f, 3, []string{"-C", root, "add", "CHANGELOG.md"})
			if tt.notesStaged == "" {
				assertCommitAsBuilt(t, f, root, tt.file, "🌿 Release "+nextTag)
			} else {
				assertCommitKeepsNotesOut(t, f, root, tt.file, "🌿 Release "+nextTag, committedChangelog)
			}
			assertNotesFileOnlyExcluded(t, f, tt.file)
			assertFileUnchanged(t, path, suppliedBody)
			assertRunFinished(t, rec)
		})
	}
}

func TestRelease_NotesFileInRepo_AnotherDirtyPath_StillFailsTheCleanTreeCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
	}{
		{name: "another untracked path", status: "?? scratch.txt\n"},
		{name: "another modified path", status: " M file.go\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeNotesFile(t, root, "notes.md", suppliedBody)
			f := runner.NewFakeRunner()
			f.SeedSequence("git",
				ScriptedOut(root),          // rev-parse --show-toplevel
				ScriptedOut(""),            // ls-files --ignored
				ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
				ScriptedOut(priorTag+"\n"), // tag --list
				ScriptedHead(),             // rev-parse HEAD --abbrev-ref HEAD (the start station)
				ScriptedOut(""),            // fetch --tags
				ScriptedOut(tt.status),     // status --porcelain (the other path is dirty)
			)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "notes.md"))
			assertAbortNonZero(t, err)

			if !recorded(rec, presentertest.KindStageFailed) {
				t.Errorf("a dirty path beside the notes file did not fail the clean-tree check")
			}
			assertGitArgs(t, f, 2, []string{"status", "--porcelain", "--", excludePathspec("notes.md")})
			assertNoMutation(t, f)
		})
	}
}

func TestRelease_NotesFileInRepo_Autostash_StashesOnlyTheOtherChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, root, "NEXT_RELEASE.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", firstReleaseGates(root, "", stashSaved())...)
	f.SeedSequence("git", firstReleaseTail()...)
	f.SeedSequence("git", stashPopped()) // stash pop (restore the other changes)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	opts := inRepoNotesFileOptions(root, "NEXT_RELEASE.md")
	opts.AutoStash = true
	if err := engine.Release(t.Context(), newDeps(rec, f), opts); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked", "--", excludePathspec("NEXT_RELEASE.md")})
	assertGitArgs(t, f, 2, []string{"stash", "pop"})
	assertNotesFileOnlyExcluded(t, f, "NEXT_RELEASE.md")
	assertFileUnchanged(t, path, suppliedBody)
	assertRunFinished(t, rec)
}

func TestRelease_NotesFileInRepo_Autostash_NotesFileTheOnlyChange_StashesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, root, "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", firstReleaseGates(root, "", stashNothing())...) // stash push saves nothing: only the notes file changed
	f.SeedSequence("git", firstReleaseTail()...)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	opts := inRepoNotesFileOptions(root, "notes.md")
	opts.AutoStash = true
	if err := engine.Release(t.Context(), newDeps(rec, f), opts); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked", "--", excludePathspec("notes.md")})
	if got := argsOf(f, "stash", "pop"); len(got) != 0 {
		t.Errorf("git stash pop ran though nothing was stashed: %q", got)
	}
	assertFileUnchanged(t, path, suppliedBody)
	assertRunFinished(t, rec)
}

func TestRelease_NotesFileInRepo_PreTagHook_ArtifactCommitLeavesTheNotesFileOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		notesStaged string
		artifact    []runner.ScriptedCall
		wantCommit  bool
	}{
		{
			name: "a hook that changes files gets an artifact commit without the notes file",
			artifact: []runner.ScriptedCall{
				ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
				ScriptedOut(""),               // -C root add -A
				ScriptedOut(""),               // -C root diff --cached -- notes.md (no staged edits)
				ScriptedOut(""),               // -C root commit -m (artifacts)
			},
			wantCommit: true,
		},
		{
			name:        "a hook that changes files, beside staged notes edits, gets an artifact commit of the other paths",
			notesStaged: "notes.md\x00",
			artifact: append([]runner.ScriptedCall{
				ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
				ScriptedOut(""),               // -C root add -A
			}, keepOutCommit("notes.md\x00", stagedArtifacts)...),
			wantCommit: true,
		},
		{
			name:     "a hook that changes nothing gets no artifact commit",
			artifact: []runner.ScriptedCall{ScriptedOut("")}, // -C root status --porcelain (post-hook: clean)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
			path := writeNotesFile(t, root, "notes.md", suppliedBody)
			f := runner.NewFakeRunner()
			f.SeedSequence("git", firstReleaseGates(root, "")...)
			f.SeedSequence("git", tt.artifact...)
			f.SeedSequence("git", firstReleaseTailFor(tt.notesStaged)...)
			f.Seed("sh", runner.Result{}, nil)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "notes.md")); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			exclude := excludePathspec("notes.md")
			assertGitArgs(t, f, 3, []string{"-C", root, "status", "--porcelain", "--", exclude})
			artifactSubject := pretagArtifactSubject("v0.0.1")
			if tt.wantCommit {
				assertGitArgs(t, f, 4, []string{"-C", root, "add", "-A", "--", exclude})
				if tt.notesStaged == "" {
					assertCommitAsBuilt(t, f, root, "notes.md", artifactSubject)
				} else {
					assertCommitKeepsNotesOut(t, f, root, "notes.md", artifactSubject, committedArtifacts)
					assertCommitKeepsNotesOut(t, f, root, "notes.md", "🌿 Release v0.0.1", committedChangelog)
				}
			} else if got := argsOf(f, "-C", root, "commit", "-m", artifactSubject); len(got) != 0 {
				t.Errorf("an artifact commit was made though the hook changed nothing: %q", got)
			}
			assertNotesFileOnlyExcluded(t, f, "notes.md")
			assertFileUnchanged(t, path, suppliedBody)
			assertRunFinished(t, rec)
		})
	}
}

func TestRelease_NotesFileInRepo_RelativePathFromASubdirectory_ExcludesTheFileItNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workDir   string
		notesFile string
		wantRel   string
	}{
		{name: "a path below the subdirectory", workDir: "sub", notesFile: "drafts/notes.md", wantRel: "sub/drafts/notes.md"},
		{name: "a path climbing out of the subdirectory", workDir: "sub", notesFile: "../notes.md", wantRel: "notes.md"},
		{name: "a working directory reached through a symlink", workDir: "link/sub", notesFile: "notes.md", wantRel: "sub/notes.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
				t.Fatalf("creating sub: %v", err)
			}
			links := t.TempDir()
			if err := os.Symlink(root, filepath.Join(links, "link")); err != nil {
				t.Fatalf("creating symlink: %v", err)
			}
			workDir := filepath.Join(root, tt.workDir)
			if strings.HasPrefix(tt.workDir, "link/") {
				workDir = filepath.Join(links, tt.workDir)
			}
			path := writeNotesFile(t, root, tt.wantRel, suppliedBody)
			f := runner.NewFakeRunner()
			seedInRepoNotesFileGit(f, root, "")
			rec := &presentertest.RecordingPresenter{}

			if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(workDir, tt.notesFile)); err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			assertGitArgs(t, f, 2, []string{"status", "--porcelain", "--", excludePathspec(tt.wantRel)})
			assertCommitAsBuilt(t, f, root, tt.wantRel, "🌿 Release "+nextTag)
			assertSuppliedBodyShipped(t, f, root, nextTag, "1.2.4", suppliedBody)
			assertFileUnchanged(t, path, suppliedBody)
		})
	}
}

func TestRelease_NotesFileOutsideTheRepo_GitStepsRunAsBuilt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeNotesFile(t, t.TempDir(), "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedPriorTagNotesFileGit(f, root)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions(path)); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertGitArgs(t, f, 2, []string{"status", "--porcelain"})
	assertGitArgs(t, f, 3, []string{"-C", root, "commit", "-m", "🌿 Release " + nextTag})
}

func TestRelease_NotesFile_EmptyWorkDir_ReadsThePathAsGiven(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), notesFileOptions("./missing-notes.md"))
	if err == nil {
		t.Fatal("Release returned nil error, want the missing notes file to fail the run")
	}

	const wantPrefix = "reading --notes-file ./missing-notes.md: "
	if !strings.Contains(err.Error(), wantPrefix) {
		t.Errorf("error = %q, want it to name the path as given: %q", err, wantPrefix)
	}
	if got := len(f.Invocations()); got != 0 {
		t.Errorf("invocations = %d, want 0 (the read fails before any step)", got)
	}
}

func TestRelease_NotesFileInRepo_ProbesWhetherGitIgnoresIt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNotesFile(t, root, "tmp/notes [x].md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git", firstReleaseGates(root, "")...)
	f.SeedSequence("git", firstReleaseTail()...)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "tmp/notes [x].md")); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	want := []string{"-C", root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--", ":(top,literal)tmp/notes [x].md"}
	if got := argsOf(f, "-C", root, "ls-files"); len(got) != 1 || !slices.Equal(got[0], want) {
		t.Errorf("ignored-path probe = %q, want exactly git %q", got, want)
	}
}

func TestRelease_NotesFileInRepo_IgnoredByGit_GitStepsRunAsBuilt(t *testing.T) {
	t.Parallel()

	const ignored = "notes.md\x00"

	t.Run("a pre_tag hook that changes files", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
		path := writeNotesFile(t, root, "notes.md", suppliedBody)
		f := runner.NewFakeRunner()
		f.SeedSequence("git", firstReleaseGates(root, ignored)...)
		f.SeedSequence("git",
			ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: dirty)
			ScriptedOut(""),               // -C root add -A
			ScriptedOut(""),               // -C root commit -m (artifacts)
		)
		f.SeedSequence("git", bareFirstReleaseTail()...)
		f.Seed("sh", runner.Result{}, nil)
		f.Seed("gh", runner.Result{}, nil)
		rec := &presentertest.RecordingPresenter{}

		if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "notes.md")); err != nil {
			t.Fatalf("Release returned unexpected error: %v", err)
		}

		assertGitArgs(t, f, 2, []string{"status", "--porcelain"})
		assertGitArgs(t, f, 3, []string{"-C", root, "status", "--porcelain"})
		assertGitArgs(t, f, 4, []string{"-C", root, "add", "-A"})
		assertGitArgs(t, f, 5, []string{"-C", root, "commit", "-m", pretagArtifactSubject("v0.0.1")})
		assertGitArgs(t, f, 5, []string{"-C", root, "commit", "-m", "🌿 Release v0.0.1"})
		assertFileUnchanged(t, path, suppliedBody)
		assertRunFinished(t, rec)
	})

	t.Run("--autostash with other changes", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		path := writeNotesFile(t, root, "notes.md", suppliedBody)
		f := runner.NewFakeRunner()
		f.SeedSequence("git", firstReleaseGates(root, ignored, stashSaved())...)
		f.SeedSequence("git", bareFirstReleaseTail()...)
		f.SeedSequence("git", stashPopped())
		f.Seed("gh", runner.Result{}, nil)
		rec := &presentertest.RecordingPresenter{}

		opts := inRepoNotesFileOptions(root, "notes.md")
		opts.AutoStash = true
		if err := engine.Release(t.Context(), newDeps(rec, f), opts); err != nil {
			t.Fatalf("Release returned unexpected error: %v", err)
		}

		assertGitArgs(t, f, 2, []string{"stash", "push", "--include-untracked"})
		assertGitArgs(t, f, 2, []string{"stash", "pop"})
		assertFileUnchanged(t, path, suppliedBody)
		assertRunFinished(t, rec)
	})
}

func TestRelease_NotesFileInRepo_IgnoredProbeFails_AbortsAtPreflight(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNotesFile(t, root, "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root), // rev-parse --show-toplevel
		runner.ScriptedCall{Result: runner.Result{ExitCode: 128, Stderr: "fatal: bad\n"}, Err: errors.New("exit status 128")}, // ls-files --ignored
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "notes.md"))
	assertAbortNonZero(t, err)

	var failed []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			failed = append(failed, ev.StageFailed.Name)
		}
	}
	if !slices.Equal(failed, []string{"preflight"}) {
		t.Errorf("failed stages = %q, want exactly [preflight]", failed)
	}
	assertNoMutation(t, f)
}
