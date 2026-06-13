package engine_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// pretagArtifactSubject is the FIXED subject mint uses for the pre_tag artifact
// commit — a `chore(release):` prefix (NOT the configurable commit_prefix),
// semantically distinct from the bookkeeping `{commit_prefix} Release {tag}` commit.
// It intentionally mirrors the production producer of the same name (release.go) so the
// black-box engine_test package stays decoupled; a subject change must update both.
func pretagArtifactSubject(tag string) string {
	return "chore(release): pre-tag artifacts for " + tag
}

// seedPreTagDirty scripts the full git timeline for a happy first-release run whose
// pre_tag hook DIRTIED the tree: after the startingHEAD capture the artifact-commit
// path probes status (non-empty → dirty), stages everything, and commits its own
// `chore(release): pre-tag artifacts for {tag}` commit — THEN the normal bookkeeping
// add/commit, tag, and push follow. The caller seeds `sh` (the hook) and `gh`.
func seedPreTagDirty(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (preflight clean-tree gate)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
		ScriptedOut(" M bundle.js\n"),        // -C root status --porcelain (post-hook: DIRTY)
		ScriptedOut(""),                      // -C root add -A
		ScriptedOut(""),                      // -C root commit -m chore(release): pre-tag artifacts
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m {commit_prefix} Release {tag}
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// seedPreTagClean scripts the full git timeline for a happy first-release run whose
// pre_tag hook left a CLEAN tree (porcelain empty after the hook): the status probe
// returns nothing, so NO artifact commit is made — the spine proceeds straight to
// the bookkeeping add/commit, tag, and push. The caller seeds `sh` and `gh`.
func seedPreTagClean(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (preflight clean-tree gate)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),                      // -C root status --porcelain (post-hook: CLEAN)
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m {commit_prefix} Release {tag}
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// TestRelease_PreTagHook_DirtyTree_CommitsArtifactsSeparately proves a pre_tag hook
// that dirties the tree yields exactly ONE `chore(release): pre-tag artifacts for
// {tag}` commit — its own commit, staged with `git -C {root} add -A` — and that this
// artifact commit is SEPARATE from the bookkeeping `{commit_prefix} Release {tag}`
// commit (two distinct commits in the timeline). The run completes successfully.
func TestRelease_PreTagHook_DirtyTree_CommitsArtifactsSeparately(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedPreTagDirty(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // pre_tag hook exits zero
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The hook ran exactly once as `sh -c "build.sh"`.
	if got := countSh(f); got != 1 {
		t.Errorf("sh ran %d times, want exactly 1 (the pre_tag hook)", got)
	}
	hookAt := indexOfCmd(f, "sh", "-c", "build.sh")
	if hookAt == -1 {
		t.Fatalf("pre_tag hook `sh -c build.sh` never ran; got %v", commandLines(f.Invocations()))
	}

	// Exactly one artifact commit, staged with `add -A`, with the FIXED chore subject.
	if !invokedWith(f, "git", "-C", root, "add", "-A") {
		t.Errorf("no `git -C %s add -A` (artifact staging); got %v", root, commandLines(f.Invocations()))
	}
	artifactSubject := pretagArtifactSubject("v0.0.1")
	if !invokedWith(f, "git", "-C", root, "commit", "-m", artifactSubject) {
		t.Errorf("no artifact commit %q; got %v", artifactSubject, commandLines(f.Invocations()))
	}
	if got := countCommitsWithSubject(f, root, artifactSubject); got != 1 {
		t.Errorf("artifact commits = %d, want exactly 1", got)
	}

	// The artifact commit is DISTINCT from the bookkeeping commit: both appear, as
	// two separate commits, and the artifact commit precedes the bookkeeping one.
	bookkeepingSubject := "🌿 Release v0.0.1"
	if !invokedWith(f, "git", "-C", root, "commit", "-m", bookkeepingSubject) {
		t.Errorf("no bookkeeping commit %q; got %v", bookkeepingSubject, commandLines(f.Invocations()))
	}
	artifactAt := indexOfCmd(f, "git", "-C", root, "commit", "-m", artifactSubject)
	bookkeepingAt := indexOfCmd(f, "git", "-C", root, "commit", "-m", bookkeepingSubject)
	if artifactAt == -1 || bookkeepingAt == -1 {
		t.Fatalf("expected both artifact and bookkeeping commits; got %v", commandLines(f.Invocations()))
	}
	if artifactAt == bookkeepingAt {
		t.Errorf("artifact and bookkeeping commits are the same invocation; they must not be folded")
	}
	if artifactAt > bookkeepingAt {
		t.Errorf("artifact commit (at %d) must precede the bookkeeping commit (at %d)", artifactAt, bookkeepingAt)
	}

	// The run reached the successful end-of-run line.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish with a dirtying pre_tag hook; last event = %v", fin.Kind)
	}
}

// TestRelease_PreTagHook_RunsAfterStartingHEADBeforeNotes proves the pre_tag hook
// runs at Stage 3 — AFTER the startingHEAD capture (`git rev-parse HEAD`) so its
// artifact commit is covered by the auto-unwind, and BEFORE notes generation (here
// observed via the bookkeeping commit / tag, which follow notes).
func TestRelease_PreTagHook_RunsAfterStartingHEADBeforeNotes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedPreTagDirty(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	hookAt := indexOfCmd(f, "sh", "-c", "build.sh")
	startingHEADAt := indexOfCmd(f, "git", "rev-parse", "HEAD")
	if hookAt == -1 || startingHEADAt == -1 {
		t.Fatalf("hook or startingHEAD capture missing; got %v", commandLines(f.Invocations()))
	}
	// AFTER startingHEAD capture → the artifact commit is unwind-covered.
	if hookAt < startingHEADAt {
		t.Errorf("pre_tag hook ran at %d, before the startingHEAD capture at %d", hookAt, startingHEADAt)
	}
	// BEFORE the bookkeeping commit and tag (which follow notes generation).
	bookkeepingAt := indexOfCmd(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1")
	if bookkeepingAt != -1 && hookAt > bookkeepingAt {
		t.Errorf("pre_tag hook ran at %d, after the bookkeeping commit at %d", hookAt, bookkeepingAt)
	}
	if tagAt := firstIndexWithPrefix(f, "git tag -a"); tagAt != -1 && hookAt > tagAt {
		t.Errorf("pre_tag hook ran at %d, after the annotated tag at %d", hookAt, tagAt)
	}
}

// TestRelease_PreTagHook_CleanTree_NoArtifactCommit proves a pre_tag hook that
// leaves a CLEAN tree (empty porcelain after the hook) produces NO artifact commit:
// no `git -C {root} add -A` and no `chore(release)` commit run, yet the run still
// proceeds to the bookkeeping commit, tag, and a successful finish.
func TestRelease_PreTagHook_CleanTree_NoArtifactCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedPreTagClean(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // hook ran, left a clean tree
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The hook ran, but a clean tree means NO artifact commit.
	if indexOfCmd(f, "sh", "-c", "build.sh") == -1 {
		t.Fatalf("pre_tag hook never ran; got %v", commandLines(f.Invocations()))
	}
	assertNoArtifactCommit(t, f, root)

	// The run still proceeds to the bookkeeping commit and a successful finish.
	if !invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("clean-tree run did not make the bookkeeping commit; got %v", commandLines(f.Invocations()))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("clean-tree pre_tag run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PreTagHook_OwnCommitCleanTree_NoArtifactCommit proves a hook that
// makes its OWN commit and hands back a clean tree leads mint to commit NOTHING:
// the single porcelain probe is empty (the hook already committed), so the
// interplay rule short-circuits — same observable outcome as the clean-tree case.
func TestRelease_PreTagHook_OwnCommitCleanTree_NoArtifactCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build-and-commit.sh\"\n")

	f := runner.NewFakeRunner()
	// The hook made its own commit and left a clean tree → porcelain empty.
	seedPreTagClean(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if indexOfCmd(f, "sh", "-c", "build-and-commit.sh") == -1 {
		t.Fatalf("pre_tag hook never ran; got %v", commandLines(f.Invocations()))
	}
	// mint sees a clean tree (the hook committed) → no mint artifact commit.
	assertNoArtifactCommit(t, f, root)

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("own-commit clean-tree pre_tag run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PreTagHook_GitignoredOutputs_NotDirty proves gitignored-only hook
// outputs do not count as dirty: `git status --porcelain` omits gitignored entries
// (mint never passes --ignored), so the probe is empty and NO artifact commit is
// made. The probe runs WITHOUT the --ignored flag.
func TestRelease_PreTagHook_GitignoredOutputs_NotDirty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	// Gitignored-only outputs → porcelain (no --ignored) reports nothing → clean.
	seedPreTagClean(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The clean-tree convention probes status WITHOUT --ignored, so gitignored
	// outputs are exempt and produce no artifact commit.
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && hasArg(inv.Args, "status") && hasArg(inv.Args, "--ignored") {
			t.Errorf("status probe passed --ignored; gitignored outputs must stay exempt: %v", inv.Args)
		}
	}
	assertNoArtifactCommit(t, f, root)
}

// TestRelease_PreTagHook_NonZeroAbortsBeforeTag proves a non-zero pre_tag hook
// aborts cleanly BEFORE any notes/tag/push: the abort surfaces a StageFailed naming
// "pre_tag", returns a non-zero *AbortError, and performs NO mutation (nothing
// tagged, pushed, or published). With the hook making no commit (HEAD unchanged),
// the unwind finds nothing to reset.
func TestRelease_PreTagHook_NonZeroAbortsBeforeTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	// The unwind drives off the tracked MadeState (it never re-probes HEAD); the
	// hook made no commit, so made is zero and there is nothing to reset.
	f.Seed("sh", runner.Result{ExitCode: 1}, errors.New("exit status 1")) // hook exits non-zero
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "pre_tag" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "pre_tag")
	}
	// No tag/push/publish — the abort precedes all of it.
	assertNoMutation(t, f)
	// The hook made no commit, so there is nothing to reset (no `git reset --hard`).
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("a reset ran though the failing hook moved no HEAD; nothing should be reset")
	}
	// Notes never reached the gate — the abort is before notes generation. The
	// Stage 1 version-confirmation gate still prompts ahead of the pre_tag hook, so
	// this asserts the NOTES gate specifically, not the absence of all prompts.
	if notesGatePrompted(rec) {
		t.Errorf("notes review gate prompted despite a failing pre_tag hook (it runs before notes)")
	}
}

// TestRelease_PreTagHook_NonZero_SurgicalNoOpsOnZeroMadeState proves the surgical
// model's boundary when a pre_tag hook exits non-zero: because the hook failed BEFORE
// mint's own artifact commit (record.CommitDirtyTree never runs), mint made ZERO
// tracked commits this run. The surgical unwind is driven by that tracked MadeState —
// not a HEAD probe — so with nothing mint made to undo it NO-OPS: it issues no `git
// reset`, no HEAD probe, and emits NO Unwound. The run still surfaces the pre_tag
// StageFailed and aborts non-zero with nothing tagged or pushed. (Per the approved
// surgical design, mint counts only the commits IT made — a hook's own internal commit
// is outside MadeState; the hook is expected to be idempotent and re-run next time.)
func TestRelease_PreTagHook_NonZero_SurgicalNoOpsOnZeroMadeState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	// The hook exits non-zero; mint never reaches its own artifact commit, so MadeState
	// is zero and the surgical unwind issues no further git commands.
	f.Seed("sh", runner.Result{ExitCode: 1}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "pre_tag" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "pre_tag")
	}
	// Zero MadeState → surgical no-op: no reset, no Unwound, no HEAD probe beyond capture.
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("a `git reset` ran though mint made no tracked commit; the surgical unwind no-ops on zero MadeState")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("an Unwound fired though mint made nothing to undo; the surgical unwind no-ops on zero MadeState")
	}
	if got := countCmd(f, "git", "rev-parse", "HEAD"); got != 1 {
		t.Errorf("rev-parse HEAD count = %d, want 1 (the pre-gate capture only; the unwind probes no HEAD)", got)
	}
	assertNoMutation(t, f)
}

// TestRelease_PreTagHook_AbsentSkipped proves an absent pre_tag hook leaves the
// existing happy path unaffected: no `sh`, no post-hook status probe, and no
// `chore(release)` artifact commit — the spine runs exactly as before.
func TestRelease_PreTagHook_AbsentSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .mint.toml — no [release.hooks].pre_tag.
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("an absent pre_tag hook ran `sh`; got %v", commandLines(f.Invocations()))
	}
	// No post-hook status probe and no artifact commit when the hook is absent.
	if invokedWith(f, "git", "-C", root, "status", "--porcelain") {
		t.Errorf("an absent pre_tag hook still ran the post-hook status probe")
	}
	assertNoArtifactCommit(t, f, root)
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("absent pre_tag run did not finish; last event = %v", fin.Kind)
	}
}

// assertNoArtifactCommit fails the test if any `git -C {root} add -A` staging or any
// `chore(release): pre-tag artifacts for …` commit was recorded — the no-commit
// outcome of the clean-tree, own-commit, gitignored, and absent-hook cases.
func assertNoArtifactCommit(t *testing.T, f *runner.FakeRunner, root string) {
	t.Helper()
	if invokedWith(f, "git", "-C", root, "add", "-A") {
		t.Errorf("an artifact-staging `git -C %s add -A` ran though no commit was expected", root)
	}
	for _, inv := range f.Invocations() {
		if inv.Name != "git" {
			continue
		}
		line := commandLine(inv)
		if strings.Contains(line, "commit -m chore(release): pre-tag artifacts") {
			t.Errorf("an unexpected pre-tag artifact commit ran: %q", line)
		}
	}
}

// countCommitsWithSubject counts how many `git -C {root} commit -m {subject}`
// invocations were recorded, so a test can prove EXACTLY one artifact commit was made.
func countCommitsWithSubject(f *runner.FakeRunner, root, subject string) int {
	want := commandLine(runner.Invocation{Name: "git", Args: []string{"-C", root, "commit", "-m", subject}})
	n := 0
	for _, inv := range f.Invocations() {
		if commandLine(inv) == want {
			n++
		}
	}
	return n
}

// hasArg reports whether args contains the exact token.
func hasArg(args []string, token string) bool {
	for _, a := range args {
		if a == token {
			return true
		}
	}
	return false
}
