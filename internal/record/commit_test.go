package record_test

import (
	"errors"
	"testing"

	"mint/internal/record"
	"mint/internal/runner"
)

func TestCommitBookkeeping_StagesChangelogAndCommitsWithSubject(t *testing.T) {
	t.Parallel()

	// The release-bookkeeping commit stages the changelog change and commits with
	// subject `{commit_prefix} Release {tag}` through the CommandRunner seam — no
	// real git. Both invocations target the repo root via `git -C {dir}`, and the
	// commit subject is asserted exactly (the default 🌿 prefix gives `🌿 Release
	// v0.0.1`).
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	const dir = "/repo/root"
	if err := record.CommitBookkeeping(t.Context(), r, dir, "🌿", "v0.0.1", true); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (git add then git commit)", len(invs))
	}

	add := invs[0]
	wantAdd := []string{"-C", dir, "add", "CHANGELOG.md"}
	if add.Name != "git" || !equalArgs(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v", add.Name, add.Args, wantAdd)
	}

	commit := invs[1]
	wantCommit := []string{"-C", dir, "commit", "-m", "🌿 Release v0.0.1"}
	if commit.Name != "git" || !equalArgs(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
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

			if err := record.CommitBookkeeping(t.Context(), r, "/repo", tt.commitPrefix, tt.tag, true); err != nil {
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

func TestCommitBookkeeping_NoChange_SkipsCommit(t *testing.T) {
	t.Parallel()

	// No-op safety: when the changelog write produced no net change there is
	// nothing to stage, so mint creates NO commit (no empty commits). The runner is
	// never touched at all.
	r := runner.NewFakeRunner()

	if err := record.CommitBookkeeping(t.Context(), r, "/repo", "🌿", "v0.0.1", false); err != nil {
		t.Fatalf("CommitBookkeeping returned unexpected error: %v", err)
	}

	if got := len(r.Invocations()); got != 0 {
		t.Errorf("invocations = %d, want 0 (no-op changelog must not stage or commit)", got)
	}
}

func TestCommitBookkeeping_StageFails_SurfacesError(t *testing.T) {
	t.Parallel()

	// A non-zero `git add` exit returns a populated Result alongside an error.
	// CommitBookkeeping must surface it (so the orchestrator can unwind) and must
	// NOT proceed to commit on a failed stage.
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: pathspec error\n", ExitCode: 128}, errors.New("exit status 128"))

	err := record.CommitBookkeeping(t.Context(), r, "/repo", "🌿", "v0.0.1", true)
	if err == nil {
		t.Fatal("CommitBookkeeping returned nil error, want the git add failure to surface")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (commit must not run after a failed stage)", got)
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
