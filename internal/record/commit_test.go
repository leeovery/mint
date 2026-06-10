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
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", true, false); err != nil {
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
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", true, true); err != nil {
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
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", false, true); err != nil {
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
	if err := record.CommitBookkeeping(t.Context(), mutator(r), dir, "🌿", "v0.0.1", "version.txt", false, false); err != nil {
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

			if err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", tt.commitPrefix, tt.tag, "version.txt", true, false); err != nil {
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

	err := record.CommitBookkeeping(t.Context(), mutator(r), "/repo", "🌿", "v0.0.1", "version.txt", true, false)
	if err == nil {
		t.Fatal("CommitBookkeeping returned nil error, want the git add failure to surface")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (commit must not run after a failed stage)", got)
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
	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), dir, "chore(release): pre-tag artifacts for v0.0.1")
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

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "chore(release): pre-tag artifacts for v0.0.1")
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

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject")
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

	committed, err := record.CommitDirtyTree(t.Context(), mutator(r), "/repo", "subject")
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
