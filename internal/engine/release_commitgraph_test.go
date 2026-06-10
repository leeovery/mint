package engine_test

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins the up-to-two-commit graph end to end (task 3-8): the optional
// pre_tag hook-artifact commit (0/1) and the optional release-bookkeeping commit
// (0/1) assemble in order, the annotated tag (which tags HEAD) lands on the LAST
// commit mint made this run — the bookkeeping commit when one exists, else the
// hook-artifact commit, else the pre-existing HEAD — and the single atomic push
// sends every resulting commit + the tag together. The four combinations (BOTH,
// ONLY BOOKKEEPING, ONLY HOOK, NEITHER) are each driven through the real spine on a
// FakeRunner + RecordingPresenter; the constituent commits (3-3 / 3-7) and the
// tag+push PONR (Phase 1) are exercised together here as one graph.

const (
	commitGraphArtifactSubject    = "chore(release): pre-tag artifacts for v0.0.1"
	commitGraphBookkeepingSubject = "🌿 Release v0.0.1"
)

// tagAt returns the index of the annotated `git tag -a {tag} -F -` invocation, or
// -1 if it never ran. The tag tags HEAD, so "the tag points at commit X" is proven
// by X being the LAST commit invocation before this index.
func tagAt(f *runner.FakeRunner, tag string) int {
	return indexOfCmd(f, "git", "tag", "-a", tag, "-F", "-")
}

// lastCommitBeforeTag returns the subject of the LAST `git -C {root} commit -m …`
// invocation that precedes the annotated tag, or "" when no commit precedes it. The
// annotated tag tags HEAD, so whatever this final pre-tag commit is, the tag points
// at it — this is how the tests assert the tag target without a real git HEAD.
func lastCommitBeforeTag(f *runner.FakeRunner, root, tag string) string {
	tagIdx := tagAt(f, tag)
	if tagIdx == -1 {
		return ""
	}
	last := ""
	for i, inv := range f.Invocations() {
		if i >= tagIdx {
			break
		}
		if inv.Name != "git" || len(inv.Args) < 5 {
			continue
		}
		// Shape: git -C {root} commit -m {subject}
		if inv.Args[0] == "-C" && inv.Args[1] == root && inv.Args[2] == "commit" && inv.Args[3] == "-m" {
			last = inv.Args[4]
		}
	}
	return last
}

// commitsBeforeTag returns, in order, the subjects of every `git -C {root} commit
// -m …` invocation that precedes the annotated tag — the commit graph the atomic
// push carries on HEAD.
func commitsBeforeTag(f *runner.FakeRunner, root, tag string) []string {
	tagIdx := tagAt(f, tag)
	var subjects []string
	for i, inv := range f.Invocations() {
		if tagIdx != -1 && i >= tagIdx {
			break
		}
		if inv.Name != "git" || len(inv.Args) < 5 {
			continue
		}
		if inv.Args[0] == "-C" && inv.Args[1] == root && inv.Args[2] == "commit" && inv.Args[3] == "-m" {
			subjects = append(subjects, inv.Args[4])
		}
	}
	return subjects
}

// assertAtomicPushAfterTag fails the test unless the exact atomic push
// `git push --atomic origin HEAD {tag}` ran AFTER the annotated tag — HEAD carries
// every resulting commit and the tag is sent with them in the single PONR.
func assertAtomicPushAfterTag(t *testing.T, f *runner.FakeRunner, tag string) {
	t.Helper()
	tagIdx := tagAt(f, tag)
	if tagIdx == -1 {
		t.Fatalf("annotated tag `git tag -a %s -F -` never ran; got %v", tag, commandLines(f.Invocations()))
	}
	pushIdx := indexOfCmd(f, "git", "push", "--atomic", "origin", "HEAD", tag)
	if pushIdx == -1 {
		t.Fatalf("atomic push `git push --atomic origin HEAD %s` never ran; got %v", tag, commandLines(f.Invocations()))
	}
	if pushIdx < tagIdx {
		t.Errorf("atomic push (at %d) ran before the tag (at %d); the tag must be created first then pushed with HEAD", pushIdx, tagIdx)
	}
}

// TestCommitGraph_Both_TwoCommitsThenTagAtBookkeeping pins the BOTH combination: a
// dirtying pre_tag hook AND a changelog change produce TWO commits — the
// hook-artifact commit FIRST, then the bookkeeping commit — both preceding the
// annotated tag, with the bookkeeping commit LAST before the tag (so the tag, at
// HEAD, points at it), and the single atomic push carrying everything.
func TestCommitGraph_Both_TwoCommitsThenTagAtBookkeeping(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedPreTagDirty(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // pre_tag hook exits zero, dirties the tree
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Exactly TWO commits precede the tag, in order: hook-artifact then bookkeeping.
	got := commitsBeforeTag(f, root, "v0.0.1")
	want := []string{commitGraphArtifactSubject, commitGraphBookkeepingSubject}
	if len(got) != len(want) {
		t.Fatalf("commits before the tag = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("commit[%d] before the tag = %q, want %q", i, got[i], want[i])
		}
	}

	// The bookkeeping commit is the LAST commit before the tag → the tag (at HEAD)
	// points at the bookkeeping commit.
	if last := lastCommitBeforeTag(f, root, "v0.0.1"); last != commitGraphBookkeepingSubject {
		t.Errorf("last commit before the tag = %q, want the bookkeeping commit %q (the tag must point at it)", last, commitGraphBookkeepingSubject)
	}

	// The single atomic push sends both commits (on HEAD) + the tag together.
	assertAtomicPushAfterTag(t, f, "v0.0.1")

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("BOTH-commit run did not finish; last event = %v", fin.Kind)
	}
}

// TestCommitGraph_OnlyBookkeeping_OneCommitThenTag pins the ONLY-BOOKKEEPING
// combination: no pre_tag hook (so no hook-artifact commit) plus a changelog change
// produces ONE commit — the bookkeeping commit — which is the tag target, with no
// `chore(release)` artifact commit anywhere, then the atomic push.
func TestCommitGraph_OnlyBookkeeping_OneCommitThenTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .mint.toml → no pre_tag hook; changelog defaults on → a bookkeeping commit.
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Exactly ONE commit precedes the tag: the bookkeeping commit, no artifact commit.
	got := commitsBeforeTag(f, root, "v0.0.1")
	want := []string{commitGraphBookkeepingSubject}
	if len(got) != len(want) {
		t.Fatalf("commits before the tag = %v, want exactly %v", got, want)
	}
	if got[0] != commitGraphBookkeepingSubject {
		t.Errorf("commit before the tag = %q, want the bookkeeping commit %q", got[0], commitGraphBookkeepingSubject)
	}
	assertNoArtifactCommit(t, f, root)

	// The bookkeeping commit is the tag target (last — and only — commit before the tag).
	if last := lastCommitBeforeTag(f, root, "v0.0.1"); last != commitGraphBookkeepingSubject {
		t.Errorf("last commit before the tag = %q, want the bookkeeping commit %q", last, commitGraphBookkeepingSubject)
	}

	assertAtomicPushAfterTag(t, f, "v0.0.1")

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("ONLY-BOOKKEEPING run did not finish; last event = %v", fin.Kind)
	}
}

// TestCommitGraph_OnlyHook_OneArtifactCommitThenTagAtHEAD pins the ONLY-HOOK
// combination: a dirtying pre_tag hook plus NO bookkeeping change (changelog
// disabled and no version_file) produces ONE commit — the hook-artifact commit —
// with NO `{commit_prefix} Release {tag}` bookkeeping commit, the tag (at HEAD)
// sitting on the hook-artifact commit, then the atomic push.
func TestCommitGraph_OnlyHook_OneArtifactCommitThenTagAtHEAD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A dirtying pre_tag hook, but changelog=false and no version_file → no bookkeeping.
	writeConfig(t, root, "[release]\nchangelog = false\n\n[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	// After startingHEAD: status (dirty) → add -A → artifact commit → [no bookkeeping
	// staging/commit: changelog disabled, no version file] → gh auth → tag → push.
	f.SeedSequence("git",
		ScriptedOut(root),             // rev-parse --show-toplevel
		ScriptedOut("origin/main"),    // symbolic-ref --short origin/HEAD
		ScriptedOut(""),               // tag --list (no tags)
		ScriptedOut(""),               // fetch --tags
		ScriptedOut(""),               // status --porcelain (preflight clean-tree gate)
		ScriptedOut("main"),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),             // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),           // rev-list left-right count (ahead only)
		ScriptedOut(""),               // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),      // rev-parse HEAD (capture the clean start)
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: DIRTY)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
		ScriptedOut(githubRemoteURL),  // remote get-url origin (provider detection)
		ScriptedOut(""),               // tag -a v0.0.1 -F - (no bookkeeping commit precedes it)
		ScriptedOut(""),               // push --atomic origin HEAD v0.0.1
	)
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Exactly ONE commit precedes the tag: the hook-artifact commit, no bookkeeping.
	got := commitsBeforeTag(f, root, "v0.0.1")
	want := []string{commitGraphArtifactSubject}
	if len(got) != len(want) {
		t.Fatalf("commits before the tag = %v, want exactly %v", got, want)
	}
	if got[0] != commitGraphArtifactSubject {
		t.Errorf("commit before the tag = %q, want the hook-artifact commit %q", got[0], commitGraphArtifactSubject)
	}
	// No bookkeeping commit and no bookkeeping staging when nothing net-changed.
	if invokedWith(f, "git", "-C", root, "commit", "-m", commitGraphBookkeepingSubject) {
		t.Errorf("a bookkeeping commit %q ran though nothing net-changed; no empty commit allowed", commitGraphBookkeepingSubject)
	}
	if invokedWith(f, "git", "-C", root, "add", "CHANGELOG.md") {
		t.Errorf("CHANGELOG.md was staged though changelog is disabled")
	}

	// The hook-artifact commit is the tag target — the tag (at HEAD) sits on it.
	if last := lastCommitBeforeTag(f, root, "v0.0.1"); last != commitGraphArtifactSubject {
		t.Errorf("last commit before the tag = %q, want the hook-artifact commit %q (tag at HEAD points at it)", last, commitGraphArtifactSubject)
	}

	assertAtomicPushAfterTag(t, f, "v0.0.1")

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("ONLY-HOOK run did not finish; last event = %v", fin.Kind)
	}
}

// TestCommitGraph_Neither_ZeroCommitsThenTagAtExistingHEAD pins the NEITHER
// combination: no hook dirt (no pre_tag hook) AND no bookkeeping change (changelog
// disabled, no version_file) produce ZERO new commits — no `chore(release)` and no
// `{commit_prefix} Release {tag}` — yet the tag (at the existing HEAD) and the
// atomic push still happen.
func TestCommitGraph_Neither_ZeroCommitsThenTagAtExistingHEAD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No pre_tag hook, changelog disabled, no version_file → no commit of any kind.
	writeConfig(t, root, "[release]\nchangelog = false\n")

	f := runner.NewFakeRunner()
	// The spine jumps from the startingHEAD capture straight to the tag + push.
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut(""),              // tag --list (no tags)
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (clean)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),          // rev-list left-right count (ahead only)
		ScriptedOut(""),              // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v0.0.1 -F - (no commit precedes it)
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// ZERO commits precede the tag — no hook-artifact and no bookkeeping commit.
	if got := commitsBeforeTag(f, root, "v0.0.1"); len(got) != 0 {
		t.Fatalf("commits before the tag = %v, want none (zero new commits)", got)
	}
	assertNoArtifactCommit(t, f, root)
	if invokedWith(f, "git", "-C", root, "commit", "-m", commitGraphBookkeepingSubject) {
		t.Errorf("a bookkeeping commit ran though nothing changed; no empty commit allowed")
	}
	// No hook ran at all (none configured), so no post-hook status probe either.
	if shInvoked(f) {
		t.Errorf("an absent pre_tag hook ran `sh`; got %v", commandLines(f.Invocations()))
	}

	// The tag (at HEAD) sits at the pre-existing HEAD: no commit precedes it.
	if last := lastCommitBeforeTag(f, root, "v0.0.1"); last != "" {
		t.Errorf("a commit (%q) preceded the tag though none was expected; the tag must sit at the existing HEAD", last)
	}

	assertAtomicPushAfterTag(t, f, "v0.0.1")

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("NEITHER run did not finish; last event = %v", fin.Kind)
	}
}
