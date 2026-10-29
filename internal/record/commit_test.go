package record_test

import (
	"errors"
	"testing"

	"mint/internal/git"
	"mint/internal/record"
	"mint/internal/runner"
)

// mutator wraps a FakeRunner in a *git.Mutator with the production happy-path
// defaults: no lock error seeded, so Mutate runs each command once and returns —
// behaving exactly like the bare runner the record functions used before.
func mutator(f *runner.FakeRunner) *git.Mutator {
	return git.NewMutator(f)
}

func TestCommitBookkeeping_ChangelogOnly_StagesChangelogAndCommitsWithSubject(t *testing.T) {
	t.Parallel()

	// A changelog-only bookkeeping commit (no version file change) stages JUST the
	// changelog and commits with subject `{commit_prefix} Release {tag}` through the
	// CommandRunner seam — no real git. Both invocations target the repo root via
	// `git -C {dir}`, and the commit subject is asserted exactly (the default 🌿
	// prefix gives `🌿 Release v0.0.1`).
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", true, false, ""); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (git add then git commit)", len(invs))
	}

	add := invs[0]
	wantAdd := []string{"-C", dir, "add", "CHANGELOG.md"}
	if add.Name != "git" || !equalArgs(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v (changelog only, version unchanged)", add.Name, add.Args, wantAdd)
	}

	commit := invs[1]
	wantCommit := []string{"-C", dir, "commit", "-m", "🌿 Release v0.0.1"}
	if commit.Name != "git" || !equalArgs(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
	}
}

func TestCommitBookkeeping_BothChanged_FoldsIntoOneCommit(t *testing.T) {
	t.Parallel()

	// When BOTH the changelog and the version file changed, they fold into ONE commit:
	// a single `git -C {dir} add CHANGELOG.md {versionFile}` staging BOTH paths,
	// followed by a single `git -C {dir} commit -m {commit_prefix} Release {tag}`. The
	// version file is NEVER given its own separate commit.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", true, true, ""); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (one folded add then one commit)\n got: %v", len(invs), invs)
	}

	add := invs[0]
	wantAdd := []string{"-C", dir, "add", "CHANGELOG.md", "version.txt"}
	if add.Name != "git" || !equalArgs(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v (both paths in ONE add)", add.Name, add.Args, wantAdd)
	}

	commit := invs[1]
	wantCommit := []string{"-C", dir, "commit", "-m", "🌿 Release v0.0.1"}
	if commit.Name != "git" || !equalArgs(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
	}
}

func TestCommitBookkeeping_VersionOnly_StagesVersionFileAndCommits(t *testing.T) {
	t.Parallel()

	// When ONLY the version file changed (the changelog netted no change, e.g.
	// changelog disabled), the bookkeeping commit stages JUST the version file and
	// still commits with subject `{commit_prefix} Release {tag}`. The changelog is NOT
	// staged.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", false, true, ""); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (version-file add then commit)\n got: %v", len(invs), invs)
	}

	add := invs[0]
	wantAdd := []string{"-C", dir, "add", "version.txt"}
	if add.Name != "git" || !equalArgs(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v (version file only)", add.Name, add.Args, wantAdd)
	}

	commit := invs[1]
	wantCommit := []string{"-C", dir, "commit", "-m", "🌿 Release v0.0.1"}
	if commit.Name != "git" || !equalArgs(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
	}
}

func TestCommitBookkeeping_NeitherChanged_SkipsCommit(t *testing.T) {
	t.Parallel()

	// Combined no-op: when NEITHER the changelog nor the version file changed there is
	// nothing to stage, so mint creates NO commit (no empty commit) and never touches
	// the runner — even though a version file path is configured.
	r := runner.NewFakeRunner()

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", false, false, ""); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	if got := len(r.Invocations()); got != 0 {
		t.Errorf("invocations = %d, want 0 (combined no-op must not stage or commit)", got)
	}
}

func TestCommitBookkeeping_SubjectHonoursPrefixAndTag(t *testing.T) {
	t.Parallel()

	// The subject is `{commit_prefix} Release {tag}` for whatever prefix/tag are
	// supplied — the prefix is configurable and the tag carries its own prefix.
	tests := []struct {
		name         string
		commitPrefix string
		tag          string
		wantSubject  string
	}{
		{name: "default emoji prefix", commitPrefix: "🌿", tag: "v0.0.1", wantSubject: "🌿 Release v0.0.1"},
		{name: "custom text prefix", commitPrefix: "chore:", tag: "v1.2.3", wantSubject: "chore: Release v1.2.3"},
		{name: "prefixless tag", commitPrefix: "🌿", tag: "2.0.0", wantSubject: "🌿 Release 2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.Seed("git", runner.Result{}, nil)

			if err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", tt.commitPrefix, tt.tag, "version.txt", true, false, ""); err != nil {
				t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
			}

			commit := r.Invocations()[1]
			gotSubject := commit.Args[len(commit.Args)-1]
			if gotSubject != tt.wantSubject {
				t.Errorf("commit subject = %q, want %q", gotSubject, tt.wantSubject)
			}
		})
	}
}

func TestCommitBookkeeping_StageFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `git add` exit returns a populated Result alongside an error.
	// CommitBookkeeping must surface it (so the orchestrator can unwind) and must
	// NOT proceed to commit on a failed stage.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: pathspec error\n", ExitCode: 128}, errors.New("exit status 128"))

	err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", "🌿", "v0.0.1", "version.txt", true, false, "")
	if err == nil {
		t.Fatal("CommitBookkeeping returned nil error, want the git add failure to surface")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (commit must not run after a failed stage)", got)
	}
}

// TestCommits_AFailureOnceStaged_IsAnUncommittedError proves a commit that fails once
// its stage ran says so with an *UncommittedError, its message the failure's own, and
// that a failure before the stage never does: what is staged is what a caller names.
func TestCommits_AFailureOnceStaged_IsAnUncommittedError(t *testing.T) {
	t.Parallel()

	failing := runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}
	dirty := runner.ScriptedCall{Result: runner.Result{Stdout: " M file\n"}}
	dirtyTree := func(r *runner.FakeRunner) error {
		_, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "")
		return err
	}
	bookkeeping := func(r *runner.FakeRunner) error {
		return record.CommitBookkeeping(t.Context(), mutator(r), "/repo", "🌿", "v0.0.1", "", true, false, "")
	}
	tests := []struct {
		name   string
		calls  []runner.ScriptedCall
		commit func(*runner.FakeRunner) error
		want   string
		staged bool
	}{
		{name: "the artifact commit failing once staged", calls: []runner.ScriptedCall{dirty, {}, failing}, commit: dirtyTree, want: `committing dirty tree "subject": exit status 1`, staged: true},
		{name: "the artifact stage failing", calls: []runner.ScriptedCall{dirty, failing}, commit: dirtyTree, want: "staging dirty tree in /repo: exit status 1"},
		{name: "the bookkeeping commit failing once staged", calls: []runner.ScriptedCall{{}, failing}, commit: bookkeeping, want: `committing release bookkeeping "🌿 Release v0.0.1": exit status 1`, staged: true},
		{name: "the bookkeeping stage failing", calls: []runner.ScriptedCall{failing}, commit: bookkeeping, want: "staging [CHANGELOG.md]: exit status 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := runner.NewFakeRunner()
			r.SeedSequence("git", tt.calls...)

			err := tt.commit(r)

			var uncommitted *record.UncommittedError
			if got := errors.As(err, &uncommitted); got != tt.staged {
				t.Errorf("an UncommittedError = %v, want %v: %v", got, tt.staged, err)
			}
			if err == nil || err.Error() != tt.want {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCommitDirtyTree_DirtyTree_StagesAllAndCommits(t *testing.T) {
	t.Parallel()

	// When the tree is dirty after a hook, CommitDirtyTree stages everything
	// (`git -C {dir} add -A`) and commits with the supplied subject through the
	// CommandRunner seam — no real git. The porcelain probe is non-empty, so a
	// commit is made and committed=true is reported. All three invocations target
	// the repo root via `git -C {dir}`.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M bundle.js\n?? new.txt\n"}}, // status --porcelain (dirty)
		runner.ScriptedCall{}, // add -A
		runner.ScriptedCall{}, // commit -m
	)

	const dir = "/repo/root"
	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), dir, "chore(release): pre-tag artifacts for v0.0.1", "")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true (dirty tree must produce a commit)")
	}

	invs := r.Invocations()
	if len(invs) != 3 {
		t.Fatalf("invocations = %d, want 3 (status, add, commit)\n got: %v", len(invs), invs)
	}

	wantStatus := []string{"-C", dir, "status", "--porcelain"}
	if status := invs[0]; status.Name != "git" || !equalArgs(status.Args, wantStatus) {
		t.Errorf("status invocation = %s %v, want git %v", status.Name, status.Args, wantStatus)
	}
	wantAdd := []string{"-C", dir, "add", "-A"}
	if add := invs[1]; add.Name != "git" || !equalArgs(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v", add.Name, add.Args, wantAdd)
	}
	wantCommit := []string{"-C", dir, "commit", "-m", "chore(release): pre-tag artifacts for v0.0.1"}
	if commit := invs[2]; commit.Name != "git" || !equalArgs(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
	}
}

func TestCommitDirtyTree_CleanTree_CommitsNothing(t *testing.T) {
	t.Parallel()

	// A clean tree (empty porcelain output) means there is nothing to commit:
	// CommitDirtyTree probes status, sees no changes, and makes NO add/commit —
	// reporting committed=false. Only the single status probe runs.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil) // status --porcelain (clean)

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "chore(release): pre-tag artifacts for v0.0.1", "")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if committed {
		t.Errorf("committed = true, want false (clean tree must not commit)")
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1 (only the status probe)\n got: %v", len(invs), invs)
	}
	wantStatus := []string{"-C", "/repo", "status", "--porcelain"}
	if status := invs[0]; status.Name != "git" || !equalArgs(status.Args, wantStatus) {
		t.Errorf("status invocation = %s %v, want git %v", status.Name, status.Args, wantStatus)
	}
}

func TestCommitDirtyTree_StatusProbeFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `git status --porcelain` exit is surfaced (so the orchestrator can
	// abort/unwind) and no add/commit follows a failed probe.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: not a git repo\n", ExitCode: 128}, errors.New("exit status 128"))

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "")
	if err == nil {
		t.Fatal("CommitDirtyTree returned nil error, want the status probe failure to surface")
	}
	if committed {
		t.Errorf("committed = true, want false on a failed probe")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (no add/commit after a failed probe)", got)
	}
}

func TestCommitDirtyTree_StageFails_SurfacesErrorBeforeCommit(t *testing.T) {
	t.Parallel()

	// A dirty tree whose `git add -A` exits non-zero surfaces the error and does NOT
	// proceed to commit — a failed stage can never produce a commit.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M file\n"}},                           // status --porcelain (dirty)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}, // add -A fails
	)

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "")
	if err == nil {
		t.Fatal("CommitDirtyTree returned nil error, want the git add failure to surface")
	}
	if committed {
		t.Errorf("committed = true, want false on a failed stage")
	}
	if got := len(r.Invocations()); got != 2 {
		t.Errorf("invocations = %d, want 2 (commit must not run after a failed stage)", got)
	}
}

const notesExclusionPathspec = ":(top,literal,exclude)NEXT_RELEASE.md"

// stagedPathsArgs is the staged-path read a commit that keeps NEXT_RELEASE.md out
// makes in dir.
func stagedPathsArgs(dir string) []string {
	return []string{"-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", notesExclusionPathspec}
}

// notesStagedArgs is the probe for staged edits to NEXT_RELEASE.md in dir.
func notesStagedArgs(dir string) []string {
	return []string{"-C", dir, "diff", "--cached", "--name-only", "-z", "--no-renames", "--", ":(top,literal)NEXT_RELEASE.md"}
}

// notesStaged is the probe's answer when NEXT_RELEASE.md has staged edits.
var notesStaged = runner.ScriptedCall{Result: runner.Result{Stdout: "NEXT_RELEASE.md\x00"}}

// pathspecCommitArgs is the commit of exactly the paths named on stdin in dir.
func pathspecCommitArgs(dir, subject string) []string {
	return []string{"-C", dir, "commit", "-m", subject, "--pathspec-from-file=-", "--pathspec-file-nul"}
}

type wantCall struct {
	args  []string
	stdin string
}

func assertCalls(t *testing.T, r *runner.FakeRunner, want []wantCall) {
	t.Helper()
	invs := r.Invocations()
	if len(invs) != len(want) {
		t.Fatalf("invocations = %d, want %d\n got: %v", len(invs), len(want), invs)
	}
	for i, inv := range invs {
		if inv.Name != "git" || !equalArgs(inv.Args, want[i].args) || inv.Stdin != want[i].stdin {
			t.Errorf("invocation %d = %s %q stdin %q, want git %q stdin %q", i, inv.Name, inv.Args, inv.Stdin, want[i].args, want[i].stdin)
		}
	}
}

func TestCommitDirtyTree_Exclusion_CommitsOnlyTheOtherStagedPaths(t *testing.T) {
	t.Parallel()

	const staged = "bundle.js\x00odd [x]*.txt\x00"
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M bundle.js\n"}}, // status --porcelain (dirty)
		runner.ScriptedCall{}, // add -A
		notesStaged,           // diff --cached -- NEXT_RELEASE.md (staged edits)
		runner.ScriptedCall{Result: runner.Result{Stdout: staged}}, // diff --cached --name-only
		runner.ScriptedCall{}, // commit --pathspec-from-file
	)

	const dir = "/repo/root"
	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), dir, "subject", "NEXT_RELEASE.md")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true (another path is staged)")
	}

	assertCalls(t, r, []wantCall{
		{args: []string{"-C", dir, "status", "--porcelain", "--", notesExclusionPathspec}},
		{args: []string{"-C", dir, "add", "-A", "--", notesExclusionPathspec}},
		{args: notesStagedArgs(dir)},
		{args: stagedPathsArgs(dir)},
		{args: pathspecCommitArgs(dir, "subject"), stdin: ":(top,literal)bundle.js\x00:(top,literal)odd [x]*.txt\x00"},
	})
}

func TestCommitDirtyTree_Exclusion_NothingElseStaged_CommitsNothing(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: "?? vendor/\n"}}, // status --porcelain (dirty)
		runner.ScriptedCall{}, // add -A
		notesStaged,           // diff --cached -- NEXT_RELEASE.md (staged edits)
		runner.ScriptedCall{}, // diff --cached --name-only (nothing staged)
	)

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "NEXT_RELEASE.md")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if committed {
		t.Errorf("committed = true, want false (nothing but the excluded path could be committed)")
	}
	if got := len(r.Invocations()); got != 4 {
		t.Errorf("invocations = %d, want 4 (no commit after an empty staged list)", got)
	}
}

func TestCommitDirtyTree_Exclusion_OnlyTheExcludedPathDirty_CommitsNothing(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil) // the excluded path is all that changed

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "NEXT_RELEASE.md")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if committed {
		t.Errorf("committed = true, want false")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (only the status probe)", got)
	}
}

func TestCommitBookkeeping_Exclusion_CommitsOnlyTheOtherStagedPaths(t *testing.T) {
	t.Parallel()

	const staged = "CHANGELOG.md\x00version.txt\x00"
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{}, // add CHANGELOG.md version.txt
		notesStaged,           // diff --cached -- NEXT_RELEASE.md (staged edits)
		runner.ScriptedCall{Result: runner.Result{Stdout: staged}}, // diff --cached --name-only
		runner.ScriptedCall{}, // commit --pathspec-from-file
	)

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", true, true, "NEXT_RELEASE.md"); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	assertCalls(t, r, []wantCall{
		{args: []string{"-C", dir, "add", "CHANGELOG.md", "version.txt"}},
		{args: notesStagedArgs(dir)},
		{args: stagedPathsArgs(dir)},
		{args: pathspecCommitArgs(dir, "🌿 Release v0.0.1"), stdin: ":(top,literal)CHANGELOG.md\x00:(top,literal)version.txt\x00"},
	})
}

func TestCommitBookkeeping_Exclusion_NothingElseStaged_FailsWithoutCommitting(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{}, // add CHANGELOG.md
		notesStaged,           // diff --cached -- NEXT_RELEASE.md (staged edits)
		runner.ScriptedCall{}, // diff --cached --name-only (nothing staged)
	)

	err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", "🌿", "v0.0.1", "", true, false, "NEXT_RELEASE.md")
	const want = `committing release bookkeeping "🌿 Release v0.0.1": nothing staged to commit`
	if err == nil || err.Error() != want {
		t.Fatalf("CommitBookkeeping error = %v, want %q", err, want)
	}
	if got := len(r.Invocations()); got != 3 {
		t.Errorf("invocations = %d, want 3 (no commit after an empty staged list)", got)
	}
}

func TestCommitDirtyTree_Exclusion_StagedPathReadFails_SurfacesErrorWithoutCommitting(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M bundle.js\n"}}, // status --porcelain (dirty)
		runner.ScriptedCall{}, // add -A
		notesStaged,           // diff --cached -- NEXT_RELEASE.md (staged edits)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 128}, Err: errors.New("exit status 128")}, // diff --cached fails
	)

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject", "NEXT_RELEASE.md")
	if err == nil {
		t.Fatal("CommitDirtyTree returned nil error, want the staged-path read failure to surface")
	}
	if committed {
		t.Errorf("committed = true, want false on a failed read")
	}
	if got := len(r.Invocations()); got != 4 {
		t.Errorf("invocations = %d, want 4 (no commit after a failed read)", got)
	}
}

func TestCommitDirtyTree_Exclusion_NoStagedEditsToKeepOut_CommitsAsBuilt(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: " M bundle.js\n"}}, // status --porcelain (dirty)
		runner.ScriptedCall{}, // add -A
		runner.ScriptedCall{}, // diff --cached -- NEXT_RELEASE.md (no staged edits)
		runner.ScriptedCall{}, // commit -m
	)

	const dir = "/repo/root"
	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), dir, "subject", "NEXT_RELEASE.md")
	if err != nil {
		t.Fatalf("CommitDirtyTree returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true")
	}

	assertCalls(t, r, []wantCall{
		{args: []string{"-C", dir, "status", "--porcelain", "--", notesExclusionPathspec}},
		{args: []string{"-C", dir, "add", "-A", "--", notesExclusionPathspec}},
		{args: notesStagedArgs(dir)},
		{args: []string{"-C", dir, "commit", "-m", "subject"}},
	})
}

func TestCommitBookkeeping_Exclusion_NoStagedEditsToKeepOut_CommitsAsBuilt(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil) // add, the empty staged-edit probe, commit

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "", true, false, "NEXT_RELEASE.md"); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	assertCalls(t, r, []wantCall{
		{args: []string{"-C", dir, "add", "CHANGELOG.md"}},
		{args: notesStagedArgs(dir)},
		{args: []string{"-C", dir, "commit", "-m", "🌿 Release v0.0.1"}},
	})
}

func TestCommitStaged_Exclusion_StagedEditProbeFails_SurfacesErrorWithoutCommitting(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{}, // add CHANGELOG.md
		runner.ScriptedCall{Result: runner.Result{ExitCode: 128}, Err: errors.New("exit status 128")}, // probe fails
	)

	if err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", "🌿", "v0.0.1", "", true, false, "NEXT_RELEASE.md"); err == nil {
		t.Fatal("CommitBookkeeping returned nil error, want the staged-edit probe failure to surface")
	}
	if got := len(r.Invocations()); got != 2 {
		t.Errorf("invocations = %d, want 2 (no commit after a failed probe)", got)
	}
}

// equalArgs reports whether two argument slices are element-for-element equal, so
// command-line assertions check the exact argv rather than a substring.
func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
