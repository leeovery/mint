package engine_test

import (
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// matchIndexStderr is git failing the restore that matches the index to a commit.
const matchIndexStderr = "fatal: Unable to write new index file\n"

// indexBehindWarning is the warning a commit of mint's in the repo at root leaves where
// the index could not be matched to it, labelled label, its command naming the commits
// either side of it and leaving out keepOut, a pathspec as the shell takes it.
func indexBehindWarning(root, label string, keepOut ...string) presenter.Warning {
	git := "git -C " + root
	paths := git + " diff --name-only -z --no-renames --diff-filter=d " + beforeCommitSHA + " " + afterCommitSHA
	for _, spec := range keepOut {
		paths += " -- " + spec
	}
	return presenter.Warning{
		Label: label,
		Message: "committed, but the index still holds what the commit hooks staged as it was before them; " +
			"match it to the commit (`" + paths + " | " +
			git + " --literal-pathspecs restore --source=" + afterCommitSHA + " --staged --pathspec-from-file=- --pathspec-file-nul && " +
			git + " update-index -q --refresh`)",
		Output: matchIndexStderr,
	}
}

// After the bookkeeping commit, every path changed since HEAD before it has its index
// entry matched to HEAD after it, so what a commit hook staged reads as no change once
// the release is out.
func TestRelease_TheBookkeepingCommit_MatchesTheIndexToEveryPathItChanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		unchangedFiles,   // the read of whether the user changed the files mint writes
		ScriptedOut(""),  // -C root add CHANGELOG.md
		headBeforeCommit, // HEAD before the commit
		ScriptedOut(""),  // -C root commit -m
		headAfterCommit,  // HEAD after it
		ScriptedOut("CHANGELOG.md\x00dist/notes.json\x00"), // the paths the commit changed
		ScriptedOut(""), // restore --staged
		ScriptedOut(""), // update-index --refresh
		ScriptedOut(""), // tag -a
		ScriptedOut(""), // push
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock}); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	lines := stepLines(f)
	commit := slices.Index(lines, "git -C "+root+" commit -m 🌿 Release v0.0.1 --pathspec-from-file=- --pathspec-file-nul"+piped("CHANGELOG.md"))
	if commit < 1 || len(lines) < commit+5 {
		t.Fatalf("git steps = %q, want the bookkeeping commit then the index matched to it", lines)
	}
	if got := lines[commit-1]; got != headRead(root) {
		t.Errorf("before the commit = %q, want the read of HEAD %q", got, headRead(root))
	}
	want := []string{
		headRead(root),
		committedPathsRead(root),
		"git -C " + root + " restore --source=" + afterCommitSHA + " --staged --pathspec-from-file=- --pathspec-file-nul" + piped("CHANGELOG.md", "dist/notes.json"),
		"git -C " + root + " update-index -q --refresh",
	}
	if got := lines[commit+1 : commit+5]; !slices.Equal(got, want) {
		t.Errorf("after the commit = %q, want %q", got, want)
	}
	if got := warnings(rec); len(got) != 0 {
		t.Errorf("warnings = %+v, want none", got)
	}
}

// Where the index can't be matched to the commit, the commit stands: the run warns
// with git's words and the command that matches it by hand, counts the commit, and
// goes on to tag and push it.
func TestRelease_TheIndexLeftBehindTheBookkeepingCommit_WarnsAndReleases(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		unchangedFiles,                  // the read of whether the user changed the files mint writes
		ScriptedOut(""),                 // -C root add CHANGELOG.md
		headBeforeCommit,                // HEAD before the commit
		ScriptedOut(""),                 // -C root commit -m
		headAfterCommit,                 // HEAD after it
		ScriptedOut("CHANGELOG.md\x00"), // the paths the commit changed
		gitFailing(matchIndexStderr),    // restore --staged
		ScriptedOut(""),                 // tag -a
		ScriptedOut(""),                 // push
	)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Now: fixedClock}); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	onlyWarning(t, rec, indexBehindWarning(root, "record"))
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("no push after the warning; got %v", commandLines(f.Invocations()))
	}
	if got := countKind(rec, presentertest.KindUnwound); got != 0 {
		t.Errorf("Unwound events = %d, want none: the commit stands", got)
	}
}

// Regenerate's changelog commit is matched the same way, and left behind it warns and
// pushes.
func TestRegenerateRun_TheIndexLeftBehindTheChangelogCommit_WarnsAndPushes(t *testing.T) {
	t.Parallel()

	dir := staleChangelogDir(t, "Stale body.\n")
	rec, err := regenerateChangelogIn(t, dir,
		ScriptedOut(""),                 // -C root add CHANGELOG.md
		headBeforeCommit,                // HEAD before the commit
		ScriptedOut(""),                 // -C root commit -m
		headAfterCommit,                 // HEAD after it
		ScriptedOut("CHANGELOG.md\x00"), // the paths the commit changed
		gitFailing(matchIndexStderr),    // restore --staged
		ScriptedOut(""),                 // push
	)
	if err != nil {
		t.Fatalf("regenerate returned unexpected error: %v", err)
	}
	onlyWarning(t, rec, indexBehindWarning(dir, "record"))
}

// The answers commit is matched the same way, and left behind it warns, counts as
// landed, and the release goes on.
func TestRelease_TheIndexLeftBehindTheAnswersCommit_WarnsAndReleases(t *testing.T) {
	t.Parallel()

	rec := answeringGates([]presenter.Choice{presenter.ChoiceExclude}, nil)
	f, root, err := answeredRelease(t, rec, "",
		[]runner.ScriptedCall{
			ScriptedOut(designDocDiff),    // the notes diff the pause asks about
			unchangedFiles,                // the read of whether the user changed .mint.toml
			configNotIgnored,              // check-ignore .mint.toml
			ScriptedOut(""),               // add .mint.toml
			headBeforeCommit,              // HEAD before the commit
			ScriptedOut(""),               // commit
			headAfterCommit,               // HEAD after it
			ScriptedOut(".mint.toml\x00"), // the paths the commit changed
			gitFailing(matchIndexStderr),  // restore --staged
			ScriptedOut(""),               // the notes diff, the answers applied
		},
		"docs/design/: design documents\n")
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	onlyWarning(t, rec, indexBehindWarning(root, "notes"))
	if got := countCommitsWithSubject(f, root, answersSubject(nextTag)); got != 1 {
		t.Errorf("answers commits = %d, want one", got)
	}
	if got := countKind(rec, presentertest.KindExclusionAnswersRecorded); got != 1 {
		t.Errorf("answers lines = %d, want one: the commit landed", got)
	}
}

// Where a notes file in the repo is kept out of the run, the warning's command leaves it
// out of the match too, quoted for the shell.
func TestRelease_TheIndexLeftBehindWithANotesFileInTheRepo_WarnsLeavingItOut(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNotesFile(t, root, "notes.md", suppliedBody)
	f := runner.NewFakeRunner()
	seedProvider(f)
	f.SeedSequence("git",
		ScriptedOut(root),               // rev-parse --show-toplevel
		ScriptedOut(""),                 // ls-files --ignored (the notes file is not ignored)
		ScriptedOut("origin/main"),      // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"),      // tag --list
		ScriptedHead(),                  // rev-parse HEAD --abbrev-ref HEAD (the start station)
		ScriptedOut(""),                 // fetch --tags
		ScriptedOut(""),                 // status --porcelain
		ScriptedOut("main"),             // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),               // rev-parse -q --verify refs/tags/v1.2.4
		ScriptedOut("0\t1"),             // rev-list left-right count
		ScriptedOut(""),                 // ls-remote --tags
		ScriptedOut(startingSHA),        // rev-parse HEAD
		unchangedFiles,                  // the read of whether the user changed the files mint writes
		ScriptedOut(""),                 // -C root add CHANGELOG.md
		headBeforeCommit,                // HEAD before the commit
		ScriptedOut(""),                 // -C root commit of CHANGELOG.md
		headAfterCommit,                 // HEAD after it
		ScriptedOut("CHANGELOG.md\x00"), // the paths the commit changed, less the notes file
		gitFailing(matchIndexStderr),    // restore --staged
		ScriptedOut(""),                 // tag -a v1.2.4 -F -
		ScriptedOut(""),                 // push --atomic origin HEAD v1.2.4
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), inRepoNotesFileOptions(root, "notes.md")); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	onlyWarning(t, rec, indexBehindWarning(root, "record", "':(top,literal,exclude)notes.md'"))
}
