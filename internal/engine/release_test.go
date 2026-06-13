package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/notes"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/release"
	"mint/internal/runner"
	"mint/internal/version"
)

// fixedClock is the deterministic release date the tests inject so the changelog
// header is exactly assertable.
var fixedClock = time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

// ScriptedOut is a stdout-bearing successful scripted call.
func ScriptedOut(stdout string) runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{Stdout: stdout}}
}

// ScriptedNonZero is a clean ran-and-exited-non-zero call (e.g. the tag-free
// local probe when the tag is absent): a populated Result with exit 1 and a
// non-nil error, matching the real runner's contract.
func ScriptedNonZero() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{ExitCode: 1},
		Err:    errors.New("exit status 1"),
	}
}

// seedHappyGit scripts a FakeRunner's "git" timeline for a no-tags first-release
// run through the full spine — every git probe the orchestrator makes resolves
// the way a clean first-release repo would (publish=true variant). The trailing
// gh calls (auth status, release create) are seeded by the caller.
//
//	root          — `git rev-parse --show-toplevel`
//	releaseBranch — `git symbolic-ref --short refs/remotes/origin/HEAD`
//	tag           — the computed tag (e.g. "v0.0.1")
func seedHappyGit(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m
		ScriptedOut(githubRemoteURL),         // remote get-url origin (provider detection)
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// githubRemoteURL is the github.com remote URL the happy-path git timeline returns
// for `git remote get-url origin` so provider auto-detection resolves to the GitHub
// driver (the publish=true default). Tests exercising other hosts/forms script their
// own remote via seedHappyGitRemote.
const githubRemoteURL = "https://github.com/acme/widget.git"

// newDeps builds the orchestrator's dependency set around a single FakeRunner so
// every external call (git via the units, gh via the publisher) is scripted and
// recorded on one timeline.
func newDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	// One Mutator is built from the single FakeRunner and shared by both the engine's
	// mutation calls and the Releaser, mirroring production wiring. With no lock error
	// seeded it behaves exactly like the bare runner — every existing spine test passes
	// unchanged through the wrapper.
	mut := git.NewMutator(f)
	return engine.ReleaseDeps{
		Presenter: rec,
		Runner:    f,
		Mutator:   mut,
		Releaser:  release.NewReleaser(mut),
	}
}

// newDepsWithMutator builds the dependency set around a caller-supplied Mutator so a
// test can drive the lock-resilient mutation path deterministically (a no-op backoff,
// a tuned threshold). The Mutator and Releaser share it, exactly as newDeps and
// production wire them.
func newDepsWithMutator(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, mut *git.Mutator) engine.ReleaseDeps {
	return engine.ReleaseDeps{
		Presenter: rec,
		Runner:    f,
		Mutator:   mut,
		Releaser:  release.NewReleaser(mut),
	}
}

// patchOptions is the default-bump options with the fixed clock.
func patchOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock}
}

// commandLine renders an Invocation as "name arg arg …" for order assertions.
func commandLine(inv runner.Invocation) string {
	return inv.Name + " " + strings.Join(inv.Args, " ")
}

// TestRelease_FirstRelease_FullSpine drives a no-tags repo through the whole
// Phase 1 spine: 0.0.0 → 0.0.1, every gate passes, the user accepts the gate, and
// the run ends at a created GitHub release. It asserts the spine ORDER on both the
// recorded git/gh invocation timeline AND the presenter event timeline.
func TestRelease_FirstRelease_FullSpine(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The external-command timeline must follow the spine in exact order: read
	// stages, fetch, local gates, network gates, bookkeeping commit, gh auth, tag,
	// push, then the provider release create.
	wantCmds := []string{
		"git rev-parse --show-toplevel",
		"git symbolic-ref --short refs/remotes/origin/HEAD",
		"git tag --list",
		"git fetch --tags",
		"git status --porcelain",
		"git rev-parse --abbrev-ref HEAD",
		"git rev-parse -q --verify refs/tags/v0.0.1",
		"git rev-list --left-right --count @{u}...HEAD",
		"git ls-remote --tags origin refs/tags/v0.0.1",
		"git rev-parse HEAD",
		"git -C " + root + " add CHANGELOG.md",
		"git -C " + root + " commit -m 🌿 Release v0.0.1",
		"git remote get-url origin",
		"gh auth status",
		"git tag -a v0.0.1 -F -",
		"git push --atomic origin HEAD v0.0.1",
		"gh release create v0.0.1 --title v0.0.1 --notes-file - --verify-tag",
	}
	invs := f.Invocations()
	if len(invs) != len(wantCmds) {
		t.Fatalf("invocation count = %d, want %d\n got: %v", len(invs), len(wantCmds), commandLines(invs))
	}
	for i, want := range wantCmds {
		if got := commandLine(invs[i]); got != want {
			t.Errorf("invocation[%d] = %q, want %q", i, got, want)
		}
	}

	// The presenter event timeline must emit in spine order. RunStarted OPENS the run;
	// then the read-only gates (version, preflight) narrate their completion; then the
	// blocking notes stage brackets its spinner; then the existing ShowPlan, ShowNotes,
	// Prompt block fires in order; then the blocking push stage brackets its spinner
	// before the run finishes. (No pre_tag stage here — no hook is configured.)
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindPrompt,         // version-confirmation gate (real run, ahead of preflight)
		presentertest.KindStageSucceeded, // preflight (read-only gate completion)
		presentertest.KindStageStarted,   // notes (blocking)
		presentertest.KindStageSucceeded, // notes
		presentertest.KindShowPlan,
		presentertest.KindShowNotes,
		presentertest.KindPrompt,         // notes review gate
		presentertest.KindStageSucceeded, // record (what the bookkeeping commit carried)
		presentertest.KindStageStarted,   // push (blocking)
		presentertest.KindStageSucceeded, // push
		presentertest.KindStageSucceeded, // publish (post-PONR success narration)
		presentertest.KindRunFinished,
	}
	gotKinds := rec.Kinds()
	if len(gotKinds) != len(wantKinds) {
		t.Fatalf("event kinds = %v, want %v", gotKinds, wantKinds)
	}
	for i, want := range wantKinds {
		if gotKinds[i] != want {
			t.Errorf("event[%d] = %v, want %v", i, gotKinds[i], want)
		}
	}

	// RunStarted carries the engine-set Action and Leaf (from commit_prefix). It now
	// OPENS the run, ahead of every stage event.
	start, _ := rec.At(indexOfKind(rec, presentertest.KindRunStarted))
	if start.RunStarted.Action != "releasing" {
		t.Errorf("RunStarted.Action = %q, want %q", start.RunStarted.Action, "releasing")
	}
	if start.RunStarted.Leaf != "🌿" {
		t.Errorf("RunStarted.Leaf = %q, want commit_prefix %q", start.RunStarted.Leaf, "🌿")
	}
	if start.RunStarted.Version != "0.0.1" {
		t.Errorf("RunStarted.Version = %q, want %q", start.RunStarted.Version, "0.0.1")
	}
	if want := filepath.Base(root); start.RunStarted.Project != want {
		t.Errorf("RunStarted.Project = %q, want repo-root basename %q", start.RunStarted.Project, want)
	}

	// ShowNotes carries the fixed first-release body.
	notes, _ := rec.At(indexOfKind(rec, presentertest.KindShowNotes))
	if notes.ShowNotes.Body != "Initial release." {
		t.Errorf("ShowNotes.Body = %q, want %q", notes.ShowNotes.Body, "Initial release.")
	}

	// RunFinished carries the resolved version — the terminal event of the run.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.RunFinished.Version != "0.0.1" {
		t.Errorf("RunFinished.Version = %q, want %q", fin.RunFinished.Version, "0.0.1")
	}
}

// TestRelease_PublishURLThreadsToRunFinished proves the success-footer seam is
// CLOSED end-to-end: the release URL `gh release create` prints to stdout is parsed
// by the publisher and threaded into RunResult.URL, so the success footer renders a
// real, non-empty URL rather than an empty segment.
func TestRelease_PublishURLThreadsToRunFinished(t *testing.T) {
	t.Parallel()

	const releaseURL = "https://github.com/acme/widget/releases/tag/v0.0.1"

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	// `gh release create` prints the created release URL to stdout; the publisher must
	// parse it and the engine must thread it into RunResult.URL.
	f.Seed("gh", runner.Result{Stdout: releaseURL + "\n"}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Fatalf("last event = %v, want RunFinished", fin.Kind)
	}
	if fin.RunFinished.URL != releaseURL {
		t.Errorf("RunFinished.URL = %q, want the publisher's release URL %q", fin.RunFinished.URL, releaseURL)
	}
}

// TestRelease_ContendedLockOnBookkeepingCommit_RecoversAndCompletes proves the
// engine's git MUTATIONS flow through the lock-resilient wrapper: a contended .git
// lock on the bookkeeping `git add` (a provably-stale lock file on disk) is cleared
// and the mutation retried, so the spine recovers and the run completes — every
// downstream stage (tag, push, publish) still runs in order. The read-only probes are
// untouched (the lock logic only wraps mutations).
func TestRelease_ContendedLockOnBookkeepingCommit_RecoversAndCompletes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// A provably-stale lock on disk in the repo's .git: an old mtime means the Mutator
	// clears it (no live holder) and retries the add — deterministic, no backoff sleep.
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	lockPath := filepath.Join(gitDir, "index.lock")
	if err := os.WriteFile(lockPath, []byte("pid 1\n"), 0o644); err != nil {
		t.Fatalf("writing lock file: %v", err)
	}
	staleMtime := fixedClock.Add(-1 * time.Hour)
	if err := os.Chtimes(lockPath, staleMtime, staleMtime); err != nil {
		t.Fatalf("setting lock mtime: %v", err)
	}
	lockStderr := "fatal: Unable to create '" + lockPath + "': File exists.\n" +
		"Another git process seems to be running in this repository.\n"

	f := runner.NewFakeRunner()
	// The happy first-release timeline, with ONE lock-error injected on the bookkeeping
	// `git add CHANGELOG.md` (attempt 1 contended → retried → succeeds).
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list (no tags)
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain (clean)
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1 (absent)
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags (free)
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture clean start)
		runner.ScriptedCall{Result: runner.Result{Stderr: lockStderr, ExitCode: 128}, Err: errors.New("exit status 128")}, // -C root add CHANGELOG.md — CONTENDED
		ScriptedOut(""),              // -C root add CHANGELOG.md — retry succeeds
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// A no-op backoff keeps the test fast; a fixed clock makes the stale-vs-live mtime
	// comparison deterministic against the on-disk lock.
	mut := git.NewMutator(f,
		git.WithBackoff(func(int) {}),
		git.WithNow(func() time.Time { return fixedClock }),
	)

	if err := engine.Release(t.Context(), newDepsWithMutator(rec, f, mut), patchOptions()); err != nil {
		t.Fatalf("Release returned unexpected error after a contended-then-cleared lock: %v", err)
	}

	// The stale lock was cleared so the retry could take it.
	if _, err := os.Stat(lockPath); err == nil {
		t.Error("stale lock file still present, want it cleared before the retry")
	}

	// The bookkeeping add ran TWICE (contended then retried), and the run still reached
	// the tag, push and provider release in order.
	addCount := 0
	for _, inv := range f.Invocations() {
		if commandLine(inv) == "git -C "+root+" add CHANGELOG.md" {
			addCount++
		}
	}
	if addCount != 2 {
		t.Errorf("bookkeeping `git add` ran %d times, want 2 (contended then a successful retry)", addCount)
	}

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish after the lock cleared; last event = %v", fin.Kind)
	}
}

// commandLines renders a slice of invocations for failure output.
func commandLines(invs []runner.Invocation) []string {
	out := make([]string, len(invs))
	for i, inv := range invs {
		out[i] = commandLine(inv)
	}
	return out
}

// TestRelease_BumpSelection proves the bump flag selects the next version on a
// no-tags repo: default → 0.0.1, -m → 0.1.0, -M → 1.0.0. Each drives a full spine
// and asserts the computed tag flows through to the tag/push invocations.
func TestRelease_BumpSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bump    version.Bump
		wantTag string
		wantVer string
	}{
		{name: "default patch", bump: version.BumpPatch, wantTag: "v0.0.1", wantVer: "0.0.1"},
		{name: "minor", bump: version.BumpMinor, wantTag: "v0.1.0", wantVer: "0.1.0"},
		{name: "major", bump: version.BumpMajor, wantTag: "v1.0.0", wantVer: "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedHappyGit(f, root, "main", tt.wantTag)
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), engine.ReleaseOptions{Bump: tt.bump, Now: fixedClock})
			if err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			fin, _ := rec.At(len(rec.Events) - 1)
			if fin.RunFinished.Version != tt.wantVer {
				t.Errorf("RunFinished.Version = %q, want %q", fin.RunFinished.Version, tt.wantVer)
			}

			// The annotated tag invocation must carry the bumped tag.
			if !invokedWith(f, "git", "tag", "-a", tt.wantTag, "-F", "-") {
				t.Errorf("no `git tag -a %s` invocation; got %v", tt.wantTag, commandLines(f.Invocations()))
			}
			// The provider release create must carry the bumped tag.
			if !invokedWith(f, "gh", "release", "create", tt.wantTag, "--title", tt.wantTag, "--notes-file", "-", "--verify-tag") {
				t.Errorf("no `gh release create %s` invocation; got %v", tt.wantTag, commandLines(f.Invocations()))
			}
		})
	}
}

// invokedWith reports whether the FakeRunner recorded a call to name with exactly
// the given args.
func invokedWith(f *runner.FakeRunner, name string, args ...string) bool {
	want := name + " " + strings.Join(args, " ")
	for _, inv := range f.Invocations() {
		if commandLine(inv) == want {
			return true
		}
	}
	return false
}

// TestRelease_AlwaysPromptsUnderYes proves the engine ALWAYS calls Prompt at both
// gates — the version-confirmation gate and the notes review gate — even under -y:
// the recorder records a KindPrompt for each and the run proceeds on the gate's
// returned default (the -y skip happens inside the presenter, which the recorder
// models by returning the default). The run reaches a successful RunFinished
// without any extra branching around the calls.
func TestRelease_AlwaysPromptsUnderYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// No NextChoices scripted: the recorder falls back to the gate Default (yes),
	// modelling the -y auto-accept the real presenter performs inside Prompt.
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Prompt fires EXACTLY twice under -y — once for the version gate, once for the
	// notes review gate; the engine never branches around the calls nor prompts
	// again; the auto-accept happens inside Prompt.
	if got := countKind(rec, presentertest.KindPrompt); got != 2 {
		t.Errorf("Prompt count = %d, want exactly 2 under -y (version + notes gates)", got)
	}
	// The notes are shown exactly once: no edit re-render, no engine-printed
	// auto-accept echo. The echo is presenter-rendered inside Prompt, never emitted
	// by the engine as a separate ShowNotes/ShowMessage event.
	if got := countKind(rec, presentertest.KindShowNotes); got != 1 {
		t.Errorf("ShowNotes count = %d, want 1 (no engine-printed auto-accept echo)", got)
	}
	if recorded(rec, presentertest.KindShowMessage) {
		t.Errorf("engine emitted a ShowMessage; the -y auto-accept echo is presenter-rendered, not engine-printed")
	}
	// The run proceeds on the returned default with notes as GENERATED — the
	// original first-release body reaches the sinks unchanged.
	const want = "Initial release."
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != want {
		t.Errorf("tag annotation body = %q, want original (as-generated) body %q", got, want)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish successfully; last event = %v", fin.Kind)
	}
}

// recorded reports whether the recorder logged an event of the given kind.
func recorded(rec *presentertest.RecordingPresenter, kind presentertest.EventKind) bool {
	for _, k := range rec.Kinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// TestRelease_PromptError_AbortsNonZero proves a Prompt error — the forbidden
// non-TTY-without-y combination, or EOF mid-gate — aborts the run with a non-zero
// exit and crosses no mutation. Both sentinels are preserved for errors.Is.
func TestRelease_PromptError_AbortsNonZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		injected error
	}{
		{name: "not a TTY without -y", injected: presenter.ErrNotInteractive},
		{name: "input closed mid-gate", injected: presenter.ErrInputClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedHappyGit(f, root, "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)
			rec := &presentertest.RecordingPresenter{
				PromptResult: func(gate presenter.Gate) (presenter.Choice, error) {
					if gate.Subject == "version" {
						return presenter.ChoiceYes, nil // accept the version gate; the error fires at the notes gate
					}
					return "", tt.injected
				},
			}

			err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
			if err == nil {
				t.Fatalf("Release returned nil error, want an abort")
			}
			if !errors.Is(err, tt.injected) {
				t.Errorf("err does not wrap injected sentinel %v: %v", tt.injected, err)
			}
			var abort *engine.AbortError
			if !errors.As(err, &abort) {
				t.Fatalf("err is not an *engine.AbortError: %v", err)
			}
			if abort.ExitCode == 0 {
				t.Errorf("abort ExitCode = 0, want non-zero")
			}

			// No mutation may have happened: nothing tagged, pushed, or published.
			assertNoMutation(t, f)
		})
	}
}

// TestRelease_GateNo_AbortsNonZero proves answering "no" at the review gate aborts
// the run (non-zero exit) before any mutation. The gate sits before any commit/tag,
// so the surgical unwind has nothing to undo (zero MadeState): it issues no git
// mutation and — per the surgical contract — emits NO Unwound (the repo never left
// the clean start). The run still aborts non-zero with nothing published.
func TestRelease_GateNo_AbortsNonZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceNo declines the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	// Nothing was made before the notes gate, so the surgical unwind no-ops: no Unwound,
	// no reset, no tag delete — the repo was already clean.
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("gate-no before any mutation emitted an Unwound; nothing was made to undo")
	}
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gate-no before any mutation issued a `git reset`; nothing to reset")
	}
	assertNoMutation(t, f)
}

// TestRelease_GateNo_NoMutation_SurgicalNoOp proves the gate-n surgical unwind BEFORE
// any mutation is a clean NO-OP: with zero MadeState (no commits, no tag) the surgical
// unwind issues NO git command — and crucially NO `git rev-parse HEAD` probe, since the
// reset is driven by the tracked MadeState, not a HEAD comparison — and emits NO
// Unwound (the repo never left the clean start). The run still aborts non-zero with
// nothing published and no success end-of-run line.
func TestRelease_GateNo_NoMutation_SurgicalNoOp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughGate(f, root, "main", "v0.0.1")
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate (so preflight + the pre-gate HEAD
		// capture run); ChoiceNo then declines the notes review gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	// The surgical unwind no-ops with nothing made: no Unwound at all.
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("gate-n before mutation emitted an Unwound; the surgical unwind no-ops when nothing was made")
	}
	// No reset, and no HEAD probe beyond the single pre-gate capture — the surgical
	// unwind is driven by MadeState, never a rev-parse compare.
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gate-n before mutation issued a `git reset --hard`; nothing should be reset")
	}
	if got := countCmd(f, "git", "rev-parse", "HEAD"); got != 1 {
		t.Errorf("rev-parse HEAD count = %d, want 1 (the pre-gate capture only; the unwind probes no HEAD)", got)
	}
	// No tag was deleted (the gate sits before the tag).
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("gate-n before mutation deleted a tag; no tag was created")
	}
	assertNoMutation(t, f)
	// No RunFinished success line follows the aborted run.
	for _, k := range rec.Kinds() {
		if k == presentertest.KindRunFinished {
			t.Errorf("a RunFinished followed a gate-n abort; an aborted run emits no success line; kinds = %v", rec.Kinds())
		}
	}
}

// TestRelease_VersionGateNo_AbortsBeforePreflight proves a "no" at the new
// version-confirmation gate (Stage 1, ahead of preflight) aborts the run cleanly
// BEFORE any work: no preflight ran (no preflight StageSucceeded), no `rev-parse
// HEAD` capture, and no mutation. The version gate sits before preflight and before
// the startingHEAD capture, so a decline has nothing to unwind — a plain non-zero
// abort with nothing touched. (Mirrors the gate-no decline tests, which assert via
// the *engine.AbortError + the no-mutation/no-preflight evidence rather than the
// unexported errReleaseDeclined sentinel, since this is an external test package.)
func TestRelease_VersionGateNo_AbortsBeforePreflight(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// Only the Stage-1 reads run before the version gate: root, release branch, and
	// the tag list (a prior tag exists, so this is a normal current→next bump). The
	// decline stops the run before fetch/preflight/HEAD-capture/mutation.
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"), // tag --list (a prior tag exists)
	)
	rec := &presentertest.RecordingPresenter{
		// The single ChoiceNo declines the version-confirmation gate (the first prompt).
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	// A non-zero abort, just like the notes-gate decline.
	assertAbortNonZero(t, err)

	// The version gate fired before preflight: no preflight StageSucceeded was emitted.
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageSucceeded && ev.StageSucceeded.Name == "preflight" {
			t.Errorf("preflight StageSucceeded fired despite a version-gate decline before preflight")
		}
	}
	// No notes review gate ran (the decline stopped the run at the version gate).
	if notesGatePrompted(rec) {
		t.Errorf("notes review gate prompted despite a version-gate decline")
	}
	// No HEAD capture: the decline precedes resolveHEAD entirely.
	if got := countCmd(f, "git", "rev-parse", "HEAD"); got != 0 {
		t.Errorf("rev-parse HEAD count = %d, want 0 (the decline precedes the pre-gate capture)", got)
	}
	// Nothing mutated: no commit, tag, push, or publish.
	if invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v1.2.4") {
		t.Errorf("a bookkeeping commit ran despite a version-gate decline")
	}
	assertNoMutation(t, f)
	// No success line follows the aborted run.
	if recorded(rec, presentertest.KindRunFinished) {
		t.Errorf("a RunFinished followed a version-gate decline; an aborted run emits no success line")
	}
}

// TestRelease_PushRejected_ResetsCommitAndDeletesTag proves a push REJECTION
// (post-Record, pre-PONR) routes through the surgical unwind AFTER a StageFailed: the
// tracked MadeState (one bookkeeping commit, tag created via release.ErrPushRejected)
// drives a `git reset --hard {startingSHA}` and a `git tag -d {tag}` — no HEAD probe.
// The Unwound Summary (engine-authored) names the reset commit AND the deleted tag AND
// the repo-clean tail, a StageFailed precedes the Unwound, the run returns a non-zero
// *AbortError, and no success end-of-run line follows.
func TestRelease_PushRejected_ResetsCommitAndDeletesTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughTag(f, root, "main", "v0.0.1")
	// The atomic push is REJECTED (ran-and-exited-non-zero), then the surgical unwind
	// runs — tag-delete first, then reset, driven by MadeState (no HEAD probe).
	f.SeedSequence("git",
		pushRejected(),  // push --atomic origin HEAD v0.0.1 (rejected)
		ScriptedOut(""), // unwind: tag -d v0.0.1
		ScriptedOut(""), // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{}, nil) // gh auth status (authenticated)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	// The release commit was reset to the clean starting point.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("push-rejection unwind did not reset to the starting SHA; got %v", commandLines(f.Invocations()))
	}
	// The local tag mint created this run was deleted.
	if !invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("push-rejection unwind did not delete the local tag; got %v", commandLines(f.Invocations()))
	}
	// A StageFailed precedes the Unwound (a failed stage, then the unwind narration).
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_GhAuthFails_ResetsCommit proves a post-Record, pre-tag failure (the
// conditional gh-auth gate fails after the bookkeeping commit but before the tag)
// routes through the surgical unwind AFTER a StageFailed: the tracked MadeState (one
// bookkeeping commit, no tag) drives a `git reset --hard {startingSHA}` with NO HEAD
// probe and NO `git tag -d` (no tag was created). The Summary names the reset commit
// and the repo-clean tail, a StageFailed precedes the Unwound, and no success line
// follows.
func TestRelease_GhAuthFails_ResetsCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughCommit(f, root, "main", "v0.0.1")
	// The gh-auth gate fails after the bookkeeping commit; the surgical unwind resets
	// the one tracked commit (no HEAD probe).
	f.SeedSequence("git",
		ScriptedOut(""), // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{ExitCode: 1}, errors.New("gh: not authenticated"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gh-auth-failure unwind did not reset to the starting SHA; got %v", commandLines(f.Invocations()))
	}
	// No tag was created before the gh-auth gate, so none must be deleted.
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("gh-auth-failure unwind deleted a tag; no tag was created yet")
	}
	// No HEAD probe inside the unwind — only the single pre-gate capture.
	if got := countCmd(f, "git", "rev-parse", "HEAD"); got != 1 {
		t.Errorf("rev-parse HEAD count = %d, want 1 (the pre-gate capture only)", got)
	}
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_AbortPathMatchesPrePushFailurePath proves the spec invariant that a
// user-abort (gate n) and a pre-push git failure flow through the SAME surgical unwind:
// each calls Unwind with its captured StartState + tracked MadeState, and the recovery
// is driven by what mint actually made — not a HEAD probe. The gate-n case here ran
// before any mutation (zero MadeState → the surgical unwind no-ops: no Unwound, no
// reset, the repo never left clean); the push-rejection case ran after the commit (one
// tracked commit + a created tag → reset to the captured starting SHA + tag delete).
// Both abort non-zero; the reset, when issued, targets the exact captured starting
// state. The matched-MadeState identity (byte-identical clean-state + summary) is
// proven by TestRelease_GateNoAndPrePushFailure_IdenticalCleanStateAndSummary.
func TestRelease_AbortPathMatchesPrePushFailurePath(t *testing.T) {
	t.Parallel()

	// Gate-n: aborts before mutation, zero MadeState, surgical no-op (no Unwound, no reset).
	gateRoot := t.TempDir()
	gateRunner := runner.NewFakeRunner()
	seedHappyGitThroughGate(gateRunner, gateRoot, "main", "v0.0.1")
	gateRec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceNo declines the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
	}
	gateErr := engine.Release(t.Context(), newDeps(gateRec, gateRunner), patchOptions())

	// Pre-push failure: the push is rejected after the commit (one tracked commit + tag),
	// so the surgical unwind resets to the starting SHA and deletes the tag.
	pushRoot := t.TempDir()
	pushRunner := runner.NewFakeRunner()
	seedHappyGitThroughTag(pushRunner, pushRoot, "main", "v0.0.1")
	pushRunner.SeedSequence("git",
		pushRejected(),  // push rejected
		ScriptedOut(""), // unwind: tag -d
		ScriptedOut(""), // unwind: reset --hard
	)
	pushRunner.Seed("gh", runner.Result{}, nil)
	pushRec := &presentertest.RecordingPresenter{}
	pushErr := engine.Release(t.Context(), newDeps(pushRec, pushRunner), patchOptions())

	// Both abort non-zero through the same surgical path.
	assertAbortNonZero(t, gateErr)
	assertAbortNonZero(t, pushErr)
	// Gate-n made nothing, so the surgical unwind no-ops: no Unwound, no reset.
	if recorded(gateRec, presentertest.KindUnwound) {
		t.Errorf("gate-n abort emitted an Unwound despite making nothing; the surgical unwind no-ops")
	}
	if invokedWith(gateRunner, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gate-n abort reset despite making nothing")
	}
	// The pre-push path made a commit + tag, so it resets to the shared starting SHA and
	// narrates an Unwound.
	if !recorded(pushRec, presentertest.KindUnwound) {
		t.Errorf("pre-push failure did not end in an Unwound")
	}
	if !invokedWith(pushRunner, "git", "reset", "--hard", startingSHA) {
		t.Errorf("pre-push failure did not reset to the shared starting SHA")
	}
}

// startingSHA is the fixed SHA the pre-gate `git rev-parse HEAD` capture returns —
// the exact clean starting state the surgical unwind resets back to. The surgical
// unwind drives off the tracked MadeState (not a HEAD probe), so there is no
// "moved HEAD" sha to script inside the unwind anymore.
const startingSHA = "startsha"

// pushRejected models a ran-and-rejected atomic push: a populated non-zero Result
// wrapped in release.ErrPushRejected, so the engine's tagMayExist branch fires (the
// local tag was created before the push was rejected).
func pushRejected() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{ExitCode: 1},
		Err:    release.ErrPushRejected,
	}
}

// seedHappyGitThroughGate scripts the git timeline up to and including the pre-gate
// `git rev-parse HEAD` capture — the read stages, the preflight gates, then the
// starting-SHA capture. The caller seeds whatever the gate/unwind needs next.
func seedHappyGitThroughGate(f *runner.FakeRunner, root, releaseBranch, tag string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),                      // tag --list (no tags)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/{tag} (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
	)
}

// seedHappyGitThroughCommit extends seedHappyGitThroughGate through the bookkeeping
// commit (CHANGELOG add + commit) and the provider-detection remote read (a
// github.com URL so the GitHub driver resolves), leaving the spine positioned at
// the conditional gh-auth gate. The caller seeds whatever the unwind needs next.
func seedHappyGitThroughCommit(f *runner.FakeRunner, root, releaseBranch, tag string) {
	seedHappyGitThroughGate(f, root, releaseBranch, tag)
	f.SeedSequence("git",
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
	)
}

// seedHappyGitThroughTag extends seedHappyGitThroughCommit through the gh-auth gate
// (seeded gh) and the annotated tag, leaving the spine positioned at the atomic
// push. The caller seeds the push outcome and whatever the unwind needs next.
func seedHappyGitThroughTag(f *runner.FakeRunner, root, releaseBranch, tag string) {
	seedHappyGitThroughCommit(f, root, releaseBranch, tag)
	f.SeedSequence("git",
		ScriptedOut(""), // tag -a {tag} -F -
	)
}

// unwoundSummary returns the Summary of the first recorded Unwound event, failing
// the test if none fired.
func unwoundSummary(t *testing.T, rec *presentertest.RecordingPresenter) string {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindUnwound {
			return ev.Unwound.Summary
		}
	}
	t.Fatalf("no Unwound event recorded; kinds = %v", rec.Kinds())
	return ""
}

// assertAbortNonZero fails the test unless err is a non-zero *engine.AbortError.
func assertAbortNonZero(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("Release returned nil error, want an abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
}

// assertStageFailedThenUnwound fails the test unless a KindStageFailed event
// precedes a KindUnwound event — the post-mutation failure ordering (surface the
// failed stage, then narrate the unwind).
func assertStageFailedThenUnwound(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	kinds := rec.Kinds()
	stageFailedAt, unwoundAt := -1, -1
	for i, k := range kinds {
		if k == presentertest.KindStageFailed && stageFailedAt == -1 {
			stageFailedAt = i
		}
		if k == presentertest.KindUnwound && unwoundAt == -1 {
			unwoundAt = i
		}
	}
	if stageFailedAt == -1 {
		t.Errorf("no StageFailed event recorded; kinds = %v", kinds)
	}
	if unwoundAt == -1 {
		t.Errorf("no Unwound event recorded; kinds = %v", kinds)
	}
	if stageFailedAt != -1 && unwoundAt != -1 && stageFailedAt > unwoundAt {
		t.Errorf("StageFailed (at %d) did not precede Unwound (at %d); kinds = %v", stageFailedAt, unwoundAt, kinds)
	}
}

// assertNoFinishAfterUnwound fails the test if any KindRunFinished follows a
// KindUnwound — an unwound run is terminal and emits no success end-of-run line.
func assertNoFinishAfterUnwound(t *testing.T, rec *presentertest.RecordingPresenter) {
	t.Helper()
	kinds := rec.Kinds()
	unwoundAt := -1
	for i, k := range kinds {
		if k == presentertest.KindUnwound {
			unwoundAt = i
			break
		}
	}
	if unwoundAt == -1 {
		t.Fatalf("no Unwound event recorded; kinds = %v", kinds)
	}
	for _, k := range kinds[unwoundAt+1:] {
		if k == presentertest.KindRunFinished {
			t.Errorf("a RunFinished followed the Unwound; an unwound run emits no success line; kinds = %v", kinds)
		}
	}
}

// fakeEditor is a scripted Editor seam: it captures the `current` body it was
// handed and returns a pre-configured edited result (or error). It stands in for
// the real $EDITOR resolution (task 2-13) so the gate-loop tests can drive the
// `e` path without launching a real editor.
type fakeEditor struct {
	// edited is the body the editor returns on success — distinctive so a test can
	// prove the EDITED text (not the original) flows downstream.
	edited string
	// err, when non-nil, is returned instead of edited to drive the edit-failure path.
	err error
	// gotCurrent captures the body the engine passed in as `current`, so a test can
	// assert the editor received the original (pre-edit) body.
	gotCurrent string
	// calls counts how many times Edit was invoked.
	calls int
}

func (e *fakeEditor) Edit(_ context.Context, current string) (string, error) {
	e.calls++
	e.gotCurrent = current
	if e.err != nil {
		return "", e.err
	}
	return e.edited, nil
}

// editedBody is a distinctive multi-line body the fake editor returns so the
// edit-path tests can prove the EDITED text — not the original — reaches every
// sink verbatim, with no re-parse or re-validation.
const editedBody = "Edited by the human: shipped verbatim.\n\n" +
	"## ✨ Hand-written\n" +
	"- This body was typed in $EDITOR and must survive untouched\n"

// newDepsWithEditor builds the orchestrator's dependency set with an injected
// editor seam (the `e` path consults it).
func newDepsWithEditor(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, ed engine.Editor) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.Editor = ed
	return deps
}

// TestRelease_GateBareEnterDefault proves a bare Enter (the gate Default,
// modelled by an empty NextChoices queue) accepts and proceeds to Record and
// onward to a successful RunFinished. The recorder records exactly one Prompt.
func TestRelease_GateBareEnterDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// Empty NextChoices: Prompt returns the gate Default (ChoiceYes) — the bare-Enter
	// accept path.
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindPrompt); got != 2 {
		t.Errorf("Prompt count = %d, want 2 (bare Enter accepts the version gate then the notes gate)", got)
	}
	// The run reached Record (the bookkeeping commit) and finished.
	if !invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("bare-Enter accept did not reach Record (no bookkeeping commit)")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("bare-Enter accept did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_GateExplicitYes proves an explicit `y` accepts and proceeds to a
// successful release on the first prompt.
func TestRelease_GateExplicitYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; second accepts the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindPrompt); got != 2 {
		t.Errorf("Prompt count = %d, want 2 (explicit y accepts the version gate then the notes gate)", got)
	}
	if !invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("explicit-y accept did not reach Record (no bookkeeping commit)")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("explicit-y accept did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_GateEditThenYes proves the real edit semantics: an `e` answer
// consults the editor seam, replaces the body with the editor's result VERBATIM
// (no re-parse, no re-validation), re-shows the notes, and re-prompts. A
// subsequent `y` proceeds — and the EDITED body, not the original, reaches every
// sink (tag annotation, CHANGELOG, provider release). The editor receives the
// ORIGINAL body as `current`.
func TestRelease_GateEditThenYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	ed := &fakeEditor{edited: editedBody}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDepsWithEditor(rec, f, ed), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The editor was consulted once and received the ORIGINAL body as `current`.
	if ed.calls != 1 {
		t.Errorf("editor Edit calls = %d, want 1", ed.calls)
	}
	if ed.gotCurrent != phase2Body {
		t.Errorf("editor received current = %q, want the original body %q", ed.gotCurrent, phase2Body)
	}

	// The notes are re-shown after the edit (initial + edit re-show) and prompts
	// fire three times: the version gate, then the notes edit and the accepting yes.
	if got := countKind(rec, presentertest.KindShowNotes); got != 2 {
		t.Errorf("ShowNotes count = %d, want 2 (initial + edit re-show)", got)
	}
	if got := countKind(rec, presentertest.KindPrompt); got != 3 {
		t.Errorf("Prompt count = %d, want 3 (version gate + edit + re-prompt after edit)", got)
	}

	// The EDITED body — not the original — reaches every sink, verbatim.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != editedBody {
		t.Errorf("tag annotation body = %q, want EDITED body %q", got, editedBody)
	}
	if got := changelogSectionBody(t, root, "0.0.1"); got != editedBody {
		t.Errorf("CHANGELOG body = %q, want EDITED body %q", got, editedBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag"); got != editedBody {
		t.Errorf("provider release body = %q, want EDITED body %q", got, editedBody)
	}

	// The edited body re-shown at the gate must be the editor's result.
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowNotes && ev.ShowNotes.Body == editedBody {
			goto found
		}
	}
	t.Errorf("no ShowNotes re-render carried the EDITED body %q", editedBody)
found:

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("edit-then-yes did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_GateEditThenNo proves the loop honours a `no` after an edit: an
// `e` (which consults the editor) followed by `n` re-shows the notes, then
// aborts non-zero before any mutation — the edited body never reaches a sink because
// nothing is mutated. The gate sits before any commit/tag, so the surgical unwind has
// zero MadeState and no-ops: it emits NO Unwound (the repo never left clean).
func TestRelease_GateEditThenNo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	ed := &fakeEditor{edited: editedBody}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDepsWithEditor(rec, f, ed), patchOptions())

	assertAbortNonZero(t, err)
	// Nothing was made before the gate, so the surgical unwind no-ops — no Unwound.
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("edit-then-no before any mutation emitted an Unwound; nothing was made to undo")
	}
	assertNoMutation(t, f)
}

// TestRelease_GateEdit_EditorError_Aborts proves an editor-seam failure on the
// `e` path is surfaced (StageFailed) and aborts non-zero before any mutation —
// the spine never blocks on a broken editor.
func TestRelease_GateEdit_EditorError_Aborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	ed := &fakeEditor{err: errors.New("no editor on PATH")}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceEdit drives the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit},
	}

	err := engine.Release(t.Context(), newDepsWithEditor(rec, f, ed), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want an editor-failure abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("editor failure did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// TestRelease_GateEdit_EditorUnavailable_ReturnsToGate proves the return-to-gate
// branch: an `e` choice whose editor cannot be launched (Edit returns
// ErrEditorReturnToGate — the launcher has already reported the problem via the
// presenter) does NOT abort. The gate is re-presented with the body UNCHANGED,
// and a subsequent `y` proceeds — so the run completes and the ORIGINAL body
// reaches every sink.
func TestRelease_GateEdit_EditorUnavailable_ReturnsToGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// The editor signals return-to-gate (no editor launchable); it leaves `edited`
	// empty so a regression that USED its result would surface a wrong body.
	ed := &fakeEditor{err: engine.ErrEditorReturnToGate}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDepsWithEditor(rec, f, ed), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The editor was consulted once (the `e`); the return-to-gate re-presented the
	// gate, and the subsequent `y` accepted without re-editing.
	if ed.calls != 1 {
		t.Errorf("editor Edit calls = %d, want 1", ed.calls)
	}
	if got := countKind(rec, presentertest.KindPrompt); got != 3 {
		t.Errorf("Prompt count = %d, want 3 (version gate + edit returns to gate + accepting yes)", got)
	}
	// The gate re-presented WITHOUT a re-render (the body did not change): only the
	// initial ShowNotes fired.
	if got := countKind(rec, presentertest.KindShowNotes); got != 1 {
		t.Errorf("ShowNotes count = %d, want 1 (no re-render on return-to-gate)", got)
	}
	// No StageFailed / Unwound: return-to-gate is not an abort.
	if recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("return-to-gate surfaced a StageFailed; it must not abort")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("return-to-gate surfaced an Unwound; it must not abort")
	}

	// The ORIGINAL body — not the (empty) editor result — reached every sink.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != phase2Body {
		t.Errorf("tag annotation body = %q, want ORIGINAL body %q", got, phase2Body)
	}
	if got := changelogSectionBody(t, root, "0.0.1"); got != phase2Body {
		t.Errorf("CHANGELOG body = %q, want ORIGINAL body %q", got, phase2Body)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("return-to-gate run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_GateEdit_NilEditor_Aborts proves the `e` choice with NO editor
// wired (a misconfiguration — production wires it in task 2-13) surfaces a clean
// failure and aborts non-zero before any mutation, rather than panicking.
func TestRelease_GateEdit_NilEditor_Aborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// No editor wired: newDeps leaves ReleaseDeps.Editor nil.
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceEdit drives the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceEdit},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want a nil-editor abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("nil-editor edit did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// TestRelease_Gate_UnexpectedChoice_Aborts proves the gate loop refuses to spin
// on a choice outside the declared y/n/e set (a presenter-contract violation,
// e.g. an r leaking through before task 2-14): it surfaces a failure and aborts
// non-zero before any mutation rather than looping forever.
func TestRelease_Gate_UnexpectedChoice_Aborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		PromptResult: func(gate presenter.Gate) (presenter.Choice, error) {
			if gate.Subject == "version" {
				return presenter.ChoiceYes, nil // accept the version gate
			}
			return presenter.ChoiceRegen, nil // r is not in the first-release gate's set
		},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want an unexpected-choice abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("unexpected choice did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// countKind counts how many recorded events match the given kind.
func countKind(rec *presentertest.RecordingPresenter, kind presentertest.EventKind) int {
	n := 0
	for _, k := range rec.Kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

// assertNoMutation fails the test if any mutating git/gh command was recorded:
// no annotated tag, no push, no provider release create. Read-only probes and the
// changelog write are not mutations to the remote.
func assertNoMutation(t *testing.T, f *runner.FakeRunner) {
	t.Helper()
	for _, inv := range f.Invocations() {
		line := commandLine(inv)
		switch {
		case strings.HasPrefix(line, "git tag -a"):
			t.Errorf("mutation occurred: annotated tag created (%q)", line)
		case strings.HasPrefix(line, "git push"):
			t.Errorf("mutation occurred: push attempted (%q)", line)
		case strings.HasPrefix(line, "gh release create"):
			t.Errorf("mutation occurred: provider release created (%q)", line)
		}
	}
}

// TestRelease_FailingGate_AbortsBeforeMutation proves a failing preflight gate
// (here: the working tree is dirty) surfaces a StageFailed, aborts non-zero, and
// performs NO mutation — nothing tagged, pushed, or published.
func TestRelease_FailingGate_AbortsBeforeMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// Resolve + fetch succeed; the clean-tree gate fails (porcelain non-empty).
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(" M file.go"),  // status --porcelain (DIRTY — gate fails)
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want a gate abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("failing gate did not surface a StageFailed event")
	}
	// The notes review gate never ran, nor any mutation. (The version-confirmation
	// gate fires first, ahead of preflight, and is accepted by default — but the
	// preflight failure stops the run before the notes review gate.)
	if notesGatePrompted(rec) {
		t.Errorf("notes review gate prompted despite a failing preflight gate")
	}
	assertNoMutation(t, f)
}

// notesGatePrompted reports whether the notes review gate (Subject "notes") was
// presented — distinct from the version-confirmation gate (Subject "version") that
// now leads every real run ahead of preflight.
func notesGatePrompted(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt && ev.Prompt.Subject == "notes" {
			return true
		}
	}
	return false
}

// TestRelease_PublishFailsAfterPush_WarnsOnly proves the PONR asymmetry: a publish
// failure AFTER a successful atomic push is WARN-ONLY — the recorder shows a Warn
// (not a StageFailed/abort), the run still finishes successfully (RunFinished),
// and Release returns nil. The tag is already public, so mint does not unwind.
func TestRelease_PublishFailsAfterPush_WarnsOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	// gh auth status succeeds, but `gh release create` fails after the push.
	f.SeedSequence("gh",
		ScriptedOut(""), // gh auth status (authenticated)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("gh: server error")}, // release create fails
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned error %v, want nil (warn-only post-PONR)", err)
	}

	if !recorded(rec, presentertest.KindWarn) {
		t.Errorf("post-PONR publish failure did not surface a Warn event")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("post-PONR publish failure unwound; it must never unwind a public tag")
	}
	if recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("post-PONR publish failure surfaced a StageFailed; it must warn only")
	}
	// The push must have crossed the PONR and the run must still finish.
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("atomic push did not run; PONR was never crossed")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish after warn-only publish failure; last event = %v", fin.Kind)
	}
	// A failed publish has no release URL — the footer must render NO URL (empty), not
	// a bogus one.
	if fin.RunFinished.URL != "" {
		t.Errorf("RunFinished.URL = %q, want empty after a warn-only publish failure", fin.RunFinished.URL)
	}
}

// TestRelease_PublishDisabled_NoGhNoPublish proves that with publish=false the
// run runs no gh gate and no provider release: it ends at a successful tag +
// atomic push. The config is written into the repo-root .mint.toml so Load picks
// it up.
func TestRelease_PublishDisabled_NoGhNoPublish(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\npublish = false\n")

	f := runner.NewFakeRunner()
	// No gh calls at all in this variant — the sequence ends at the push.
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),            // -C root add CHANGELOG.md
		ScriptedOut(""),            // -C root commit -m
		ScriptedOut(""),            // tag -a v0.0.1 -F -
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// No gh invocation may have happened (no gate, no publish).
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" {
			t.Errorf("gh was invoked with publish=false: %q", commandLine(inv))
		}
	}
	// The run still tags and pushes, then finishes.
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("publish=false run did not reach the atomic push")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("publish=false run did not finish; last event = %v", fin.Kind)
	}
	// The plan must NOT include a publish step under publish=false.
	plan, _ := rec.At(1)
	for _, step := range plan.ShowPlan.Steps {
		if step.Verb == "publish" {
			t.Errorf("plan includes a publish step despite publish=false: %+v", step)
		}
	}
}

// writeConfig writes a .mint.toml at the repo root so config.Load picks it up.
func writeConfig(t *testing.T, root, contents string) {
	t.Helper()
	path := root + "/.mint.toml"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// writeFile seeds an arbitrary repo-relative file under root — used to stage a real
// source file (e.g. an embedded-mode version_file) the pipeline reads or rewrites.
func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := root + "/" + name
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// phase2Body is a distinctive multi-line Phase-2-style notes body: a TL;DR plus
// emoji-headed sections. It exercises body distribution end to end — the exact
// same bytes must reach the tag annotation, the CHANGELOG section, and the
// provider release, surviving verbatim with no parsing, splitting, or per-sink
// reassembly.
const phase2Body = "TL;DR: ship the body whole to every sink.\n\n" +
	"## ✨ Features\n" +
	"- Single body distributed to tag, changelog, and provider release\n\n" +
	"## 🐛 Fixes\n" +
	"- No per-sink reassembly; identical bytes everywhere\n"

// patchOptionsWithBody returns the default-bump options with the fixed clock and
// an injected NotesBody — the Phase-2 seam SelectBody (task 2-16) will later fill.
func patchOptionsWithBody(body string) engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock, NotesBody: body}
}

// stdinOf returns the recorded stdin for the first invocation whose command line
// matches name+args exactly, failing the test if no such invocation was recorded.
// It is how body-distribution tests read back what was piped to `git tag -a … -F -`
// and `gh release create … --notes-file -`.
func stdinOf(t *testing.T, f *runner.FakeRunner, name string, args ...string) string {
	t.Helper()
	want := name + " " + strings.Join(args, " ")
	for _, inv := range f.Invocations() {
		if commandLine(inv) == want {
			return inv.Stdin
		}
	}
	t.Fatalf("no invocation %q recorded; got %v", want, commandLines(f.Invocations()))
	return ""
}

// tagAnnotationBody extracts the notes body from a piped `git tag -a … -F -`
// stdin: the composed message is the `{commit_prefix} Release {tag}` subject, a
// blank line, then the full body verbatim. Splitting on the first blank line
// returns the body untouched so tests assert it byte-for-byte.
func tagAnnotationBody(t *testing.T, f *runner.FakeRunner, tag string) string {
	t.Helper()
	message := stdinOf(t, f, "git", "tag", "-a", tag, "-F", "-")
	_, body, found := strings.Cut(message, "\n\n")
	if !found {
		t.Fatalf("tag annotation message has no subject/body separator: %q", message)
	}
	return body
}

// changelogSectionBody reads {root}/CHANGELOG.md and returns the body under the
// `## [version] - date` header exactly as projected. The section is rendered as
// `header\n\nbody\n` (renderSection appends a single trailing newline so the next
// section starts on its own line), so the body is recovered by stripping the
// header's "\n\n" separator and exactly that one rendering newline — not all
// trailing newlines, which would corrupt a body that itself ends in "\n".
func changelogSectionBody(t *testing.T, root, version string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	header := "## [" + version + "] - " + fixedClock.Format("2006-01-02")
	_, afterHeader, found := strings.Cut(string(data), header)
	if !found {
		t.Fatalf("CHANGELOG.md has no section header %q; got:\n%s", header, data)
	}
	// afterHeader begins with the header's "\n\n" separator then the body, then the
	// single trailing newline renderSection appends.
	body := strings.TrimPrefix(afterHeader, "\n\n")
	return strings.TrimSuffix(body, "\n")
}

// TestRelease_SingleBodyToAllSinks proves the same notes body reaches all three
// sinks verbatim: the annotated tag message, the CHANGELOG section, and the
// provider release — identical bytes, used whole, no parsing or per-sink
// reassembly. The body is injected via opts.NotesBody (the SelectBody seam).
func TestRelease_SingleBodyToAllSinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// (a) The annotated tag carries the full body verbatim, after the subject.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != phase2Body {
		t.Errorf("tag annotation body = %q, want %q", got, phase2Body)
	}
	// (b) The CHANGELOG section carries the same body verbatim.
	if got := changelogSectionBody(t, root, "0.0.1"); got != phase2Body {
		t.Errorf("CHANGELOG body = %q, want %q", got, phase2Body)
	}
	// (c) The provider release create carries the same body verbatim on stdin.
	if got := stdinOf(t, f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag"); got != phase2Body {
		t.Errorf("provider release body = %q, want %q", got, phase2Body)
	}

	// The body survived whole: every sink got the exact same multi-line bytes,
	// proving no parsing/splitting/reassembly diverged any one of them.
	tagBody := tagAnnotationBody(t, f, "v0.0.1")
	changelogBody := changelogSectionBody(t, root, "0.0.1")
	providerBody := stdinOf(t, f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag")
	if tagBody != changelogBody || changelogBody != providerBody {
		t.Errorf("bodies diverged across sinks:\n tag=%q\n changelog=%q\n provider=%q", tagBody, changelogBody, providerBody)
	}
}

// TestRelease_EmptyNotesBody_FallsBackToFirstReleaseDefault proves the Phase-2
// seam preserves current behaviour: an empty opts.NotesBody falls back to the
// Phase-1 first-release default body, which then flows to every sink. This is the
// override-absent path the existing 1-x tests rely on.
func TestRelease_EmptyNotesBody_FallsBackToFirstReleaseDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// NotesBody is the zero value (""), so the spine must use the first-release body.
	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	const want = "Initial release."
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != want {
		t.Errorf("tag annotation body = %q, want first-release default %q", got, want)
	}
	if got := changelogSectionBody(t, root, "0.0.1"); got != want {
		t.Errorf("CHANGELOG body = %q, want first-release default %q", got, want)
	}
	notes, _ := rec.At(indexOfKind(rec, presentertest.KindShowNotes))
	if notes.ShowNotes.Body != want {
		t.Errorf("ShowNotes.Body = %q, want first-release default %q", notes.ShowNotes.Body, want)
	}
}

// TestRelease_ChangelogDisabled_SkipsChangelogTagStillCarriesBody proves
// changelog=false skips the CHANGELOG projection (no file written, no bookkeeping
// commit) while the annotated tag STILL carries the full body — nothing durable is
// lost. The tag points at the existing HEAD (no empty bookkeeping commit) and the
// run finishes successfully.
func TestRelease_ChangelogDisabled_SkipsChangelogTagStillCarriesBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nchangelog = false\n")

	f := runner.NewFakeRunner()
	// changelog=false: no `add CHANGELOG.md` / `commit` calls — the sequence runs
	// the read gates, then tag + push directly. publish defaults true, so gh runs.
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut(""),              // tag --list
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),          // rev-list left-right count
		ScriptedOut(""),              // ls-remote --tags
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v0.0.1 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v0.0.1
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// No CHANGELOG.md may have been written.
	if _, statErr := os.Stat(filepath.Join(root, "CHANGELOG.md")); !os.IsNotExist(statErr) {
		t.Errorf("CHANGELOG.md was written despite changelog=false (stat err: %v)", statErr)
	}
	// No bookkeeping commit (no stage, no commit) may have happened.
	for _, inv := range f.Invocations() {
		line := commandLine(inv)
		if strings.Contains(line, "add CHANGELOG.md") || strings.Contains(line, "commit -m") {
			t.Errorf("bookkeeping commit ran despite changelog=false: %q", line)
		}
	}
	// The tag STILL carries the full body — the mandatory floor / sole read source.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != phase2Body {
		t.Errorf("tag annotation body = %q, want full body %q even with changelog=false", got, phase2Body)
	}
	// The run still tags and pushes, then finishes.
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("changelog=false run did not reach the atomic push")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("changelog=false run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_ChangelogDefaultTrue_WritesChangelog pins the toggle default: with
// no config file, changelog defaults true, so the CHANGELOG projection is written
// (file created, bookkeeping commit recorded) carrying the body.
func TestRelease_ChangelogDefaultTrue_WritesChangelog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The default-true changelog must produce a CHANGELOG.md carrying the body.
	if got := changelogSectionBody(t, root, "0.0.1"); got != phase2Body {
		t.Errorf("CHANGELOG body = %q, want %q (changelog defaults true)", got, phase2Body)
	}
	// The bookkeeping commit must have run (the changelog changed).
	if !invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release v0.0.1") {
		t.Errorf("default-true changelog did not record the bookkeeping commit")
	}
}

// TestRelease_PublishDisabled_TagStillCarriesBody proves publish=false skips the
// provider release while the annotated tag still carries the full injected body
// (and the changelog projection, default-true, also carries it). The run ends at a
// successful tag + push.
func TestRelease_PublishDisabled_TagStillCarriesBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\npublish = false\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(startingSHA),   // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),            // -C root add CHANGELOG.md
		ScriptedOut(""),            // -C root commit -m
		ScriptedOut(""),            // tag -a v0.0.1 -F -
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptionsWithBody(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// No provider release may have been created.
	if invokedWith(f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag") {
		t.Errorf("provider release created despite publish=false")
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "gh" {
			t.Errorf("gh was invoked with publish=false: %q", commandLine(inv))
		}
	}
	// The tag still carries the full body, and the push still ran.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != phase2Body {
		t.Errorf("tag annotation body = %q, want full body %q with publish=false", got, phase2Body)
	}
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1") {
		t.Errorf("publish=false run did not reach the atomic push")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("publish=false run did not finish; last event = %v", fin.Kind)
	}
}

// fakeRegenerator is a scripted Regenerator seam: it captures every one-time
// context line it was handed and returns successive scripted bodies (one per
// Regenerate call). It stands in for the real AI-path regeneration (task 2-16) so
// the gate-loop tests can drive the `r` path without the notes engine.
type fakeRegenerator struct {
	// bodies is the FIFO of bodies returned, one per Regenerate call (call i returns
	// bodies[i]); a multi-r test scripts body1, body2, … to prove the FINAL one wins.
	bodies []string
	// err, when non-nil, is returned instead of the next body to drive the
	// regenerate-failure abort path.
	err error
	// gotContexts records the one-time context line each Regenerate call received,
	// in order — so a test can assert the scripted nudge reached the AI.
	gotContexts []string
}

func (r *fakeRegenerator) Regenerate(_ context.Context, oneTimeContext string) (string, error) {
	r.gotContexts = append(r.gotContexts, oneTimeContext)
	if r.err != nil {
		return "", r.err
	}
	body := r.bodies[len(r.gotContexts)-1]
	return body, nil
}

// regen1Body and regen2Body are distinctive bodies the fake Regenerator returns so
// the regenerate-path tests can prove the REGENERATED text (not the original, and
// for the multi-r case the FINAL regeneration) reaches every sink verbatim.
const regen1Body = "Regenerated #1: lead with the auth package.\n\n" +
	"## ✨ Added\n- New auth package\n"
const regen2Body = "Regenerated #2: now emphasise the security fix.\n\n" +
	"## 🔒 Security\n- Hardened token validation\n"

// normalAIOptions returns the default-bump options with the fixed clock, an
// injected NotesBody, and NotesKind=KindNormalAI — the seam that selects the
// four-choice y/n/e/r review gate (the only Kind that offers `r`).
func normalAIOptions(body string) engine.ReleaseOptions {
	opts := patchOptionsWithBody(body)
	opts.NotesKind = notes.KindNormalAI
	return opts
}

// newDepsWithRegenerator builds the orchestrator's dependency set with an injected
// Regenerator seam (the `r` path consults it).
func newDepsWithRegenerator(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, regen engine.Regenerator) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.Regenerator = regen
	return deps
}

// TestRelease_GateRegenThenYes proves the normal-AI `r` path: an `r` answer reads
// a one-time context line via AskLine, hands it to the Regenerator, replaces the
// body with the regenerated result VERBATIM, re-shows the notes, and re-presents
// the gate. A subsequent `y` proceeds — and the REGENERATED body, not the
// original, reaches every sink.
func TestRelease_GateRegenThenYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{bodies: []string{regen1Body}}
	const contextLine = "Lead with the new auth package."
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{contextLine},
	}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// AskLine fired (the one-time context was read through the presenter's input
	// seam, never stdin directly), and the Regenerator received the scripted line.
	if got := countKind(rec, presentertest.KindAskLine); got != 1 {
		t.Errorf("AskLine count = %d, want 1 (the one-time context read)", got)
	}
	if len(regen.gotContexts) != 1 {
		t.Fatalf("Regenerate calls = %d, want 1", len(regen.gotContexts))
	}
	if regen.gotContexts[0] != contextLine {
		t.Errorf("Regenerator received context = %q, want the scripted line %q", regen.gotContexts[0], contextLine)
	}

	// The notes are re-shown after regeneration (initial + regen re-show) and the
	// gate is prompted twice (regen, then the accepting yes).
	if got := countKind(rec, presentertest.KindShowNotes); got != 2 {
		t.Errorf("ShowNotes count = %d, want 2 (initial + regen re-show)", got)
	}
	if got := countKind(rec, presentertest.KindPrompt); got != 3 {
		t.Errorf("Prompt count = %d, want 3 (version gate + regen + re-prompt after regen)", got)
	}

	// The REGENERATED body — not the original — reaches every sink, verbatim.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != regen1Body {
		t.Errorf("tag annotation body = %q, want REGENERATED body %q", got, regen1Body)
	}
	if got := changelogSectionBody(t, root, "0.0.1"); got != regen1Body {
		t.Errorf("CHANGELOG body = %q, want REGENERATED body %q", got, regen1Body)
	}
	if got := stdinOf(t, f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag"); got != regen1Body {
		t.Errorf("provider release body = %q, want REGENERATED body %q", got, regen1Body)
	}

	// The regenerated body was re-shown at the gate.
	if !showedNotesBody(rec, regen1Body) {
		t.Errorf("no ShowNotes re-render carried the REGENERATED body %q", regen1Body)
	}

	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("regen-then-yes did not finish; last event = %v", fin.Kind)
	}
}

// showedNotesBody reports whether any recorded ShowNotes carried exactly body.
func showedNotesBody(rec *presentertest.RecordingPresenter, body string) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindShowNotes && ev.ShowNotes.Body == body {
			return true
		}
	}
	return false
}

// TestRelease_GateRegen_EmptyContext_RegeneratesWithNoExtraContext proves an empty
// AskLine answer is LEGAL: `r` with a bare-Enter context line regenerates with NO
// extra context — the Regenerator receives the empty string.
func TestRelease_GateRegen_EmptyContext_RegeneratesWithNoExtraContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{bodies: []string{regen1Body}}
	// Empty NextLines: AskLine falls back to "" — the legal "no extra context" answer.
	// First ChoiceYes accepts the version gate; the rest drive the notes gate.
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindAskLine); got != 1 {
		t.Errorf("AskLine count = %d, want 1 (still asked, empty answer is legal)", got)
	}
	if len(regen.gotContexts) != 1 {
		t.Fatalf("Regenerate calls = %d, want 1", len(regen.gotContexts))
	}
	if regen.gotContexts[0] != "" {
		t.Errorf("Regenerator received context = %q, want empty (no extra context)", regen.gotContexts[0])
	}
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != regen1Body {
		t.Errorf("tag annotation body = %q, want REGENERATED body %q", got, regen1Body)
	}
}

// TestRelease_GateRegen_InputClosed_AbortsFailLoud proves ErrInputClosed from the
// AskLine one-time-context read aborts the run fail-loud (non-zero exit, sentinel
// preserved) before any mutation — the engine never blocks on a closed input.
func TestRelease_GateRegen_InputClosed_AbortsFailLoud(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{bodies: []string{regen1Body}}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceRegen drives the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		AskLineResult: func(string) (string, error) {
			return "", presenter.ErrInputClosed
		},
	}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err == nil {
		t.Fatalf("Release returned nil error, want an input-closed abort")
	}
	if !errors.Is(err, presenter.ErrInputClosed) {
		t.Errorf("err does not wrap presenter.ErrInputClosed: %v", err)
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	// The Regenerator was never consulted (the read failed first), and nothing mutated.
	if len(regen.gotContexts) != 0 {
		t.Errorf("Regenerate was called %d times despite a closed input read", len(regen.gotContexts))
	}
	assertNoMutation(t, f)
}

// TestRelease_GateMultipleRegen_FinalBodyReachesSinks proves multiple `r` loops:
// r, r, y regenerates twice (body1 then body2) and the FINAL (body2) reaches every
// sink — each `r` regenerates and re-shows.
func TestRelease_GateMultipleRegen_FinalBodyReachesSinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{bodies: []string{regen1Body, regen2Body}}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"first nudge", "second nudge"},
	}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// Two regenerations happened, each reading its own one-time context line.
	if len(regen.gotContexts) != 2 {
		t.Fatalf("Regenerate calls = %d, want 2", len(regen.gotContexts))
	}
	if regen.gotContexts[0] != "first nudge" || regen.gotContexts[1] != "second nudge" {
		t.Errorf("Regenerator contexts = %v, want [first nudge, second nudge]", regen.gotContexts)
	}
	if got := countKind(rec, presentertest.KindAskLine); got != 2 {
		t.Errorf("AskLine count = %d, want 2 (one per regenerate)", got)
	}
	// Each `r` re-shows: initial + two regen re-shows = 3 ShowNotes; three prompts.
	if got := countKind(rec, presentertest.KindShowNotes); got != 3 {
		t.Errorf("ShowNotes count = %d, want 3 (initial + 2 regen re-shows)", got)
	}
	if got := countKind(rec, presentertest.KindPrompt); got != 4 {
		t.Errorf("Prompt count = %d, want 4 (version gate, then r, r, y)", got)
	}

	// The FINAL regeneration (body2) — not body1 — reaches every sink.
	if got := tagAnnotationBody(t, f, "v0.0.1"); got != regen2Body {
		t.Errorf("tag annotation body = %q, want FINAL regenerated body %q", got, regen2Body)
	}
	if got := changelogSectionBody(t, root, "0.0.1"); got != regen2Body {
		t.Errorf("CHANGELOG body = %q, want FINAL regenerated body %q", got, regen2Body)
	}
	if got := stdinOf(t, f, "gh", "release", "create", "v0.0.1", "--title", "v0.0.1", "--notes-file", "-", "--verify-tag"); got != regen2Body {
		t.Errorf("provider release body = %q, want FINAL regenerated body %q", got, regen2Body)
	}
}

// TestRelease_GateRegen_RegeneratorError_Aborts proves a Regenerator failure on
// the `r` path is surfaced (StageFailed) and aborts non-zero before any mutation —
// a regenerate failure is fail-loud.
func TestRelease_GateRegen_RegeneratorError_Aborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{err: errors.New("claude timed out")}
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceRegen drives the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		NextLines:   []string{"a nudge"},
	}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err == nil {
		t.Fatalf("Release returned nil error, want a regenerate-failure abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("regenerate failure did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// TestRelease_GateRegen_NilRegenerator_Aborts proves the `r` choice with NO
// Regenerator wired (a misconfiguration — production wires it in task 2-16)
// surfaces a clean failure and aborts non-zero before any mutation, rather than
// panicking.
func TestRelease_GateRegen_NilRegenerator_Aborts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	// No Regenerator wired: newDeps leaves ReleaseDeps.Regenerator nil.
	// First ChoiceYes accepts the version gate; ChoiceRegen drives the notes gate.
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen},
		NextLines:   []string{"a nudge"},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), normalAIOptions(phase2Body))
	if err == nil {
		t.Fatalf("Release returned nil error, want a nil-regenerator abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("nil-regenerator regen did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// TestRelease_NoAIPaths_GateOmitsRegenerate proves the gate variant: on every
// no-AI Kind (first-release, degenerate, --no-ai) the review gate offered is the
// y/n/e variant — its declared keys are exactly [y n e], with NO `r` (there is no
// AI to nudge on those paths). Asserted via the recorded gate's Keys().
func TestRelease_NoAIPaths_GateOmitsRegenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind notes.Kind
	}{
		{name: "first release", kind: notes.KindFirstRelease},
		{name: "degenerate", kind: notes.KindDegenerate},
		{name: "no-ai", kind: notes.KindNoAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedHappyGit(f, root, "main", "v0.0.1")
			f.Seed("gh", runner.Result{}, nil)
			// A Regenerator IS wired, to prove `r` is omitted by the GATE, not because
			// the seam is absent — the no-AI gate never even offers the choice.
			regen := &fakeRegenerator{bodies: []string{regen1Body}}
			rec := &presentertest.RecordingPresenter{}

			opts := patchOptionsWithBody(phase2Body)
			opts.NotesKind = tt.kind

			err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), opts)
			if err != nil {
				t.Fatalf("Release returned unexpected error: %v", err)
			}

			gate := promptGate(t, rec)
			if got, want := keysOf(gate), []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit}; !equalChoices(got, want) {
				t.Errorf("%s gate keys = %v, want the y/n/e variant %v (no r)", tt.name, got, want)
			}
			if gate.Has(presenter.ChoiceRegen) {
				t.Errorf("%s gate offered r; the no-AI gate must omit regenerate", tt.name)
			}
		})
	}
}

// TestRelease_NormalAI_GateOffersRegenerate proves the complementary case: on the
// normal-AI Kind the review gate offered is the four-choice y/n/e/r variant — its
// declared keys are exactly [y n e r].
func TestRelease_NormalAI_GateOffersRegenerate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	regen := &fakeRegenerator{bodies: []string{regen1Body}}
	// Accept immediately (bare-Enter default), so the gate is recorded exactly once.
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDepsWithRegenerator(rec, f, regen), normalAIOptions(phase2Body))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	gate := promptGate(t, rec)
	want := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen}
	if got := keysOf(gate); !equalChoices(got, want) {
		t.Errorf("normal-AI gate keys = %v, want the y/n/e/r variant %v", got, want)
	}
}

// promptGate returns the gate carried by the NOTES review Prompt event (Subject
// "notes"), skipping the earlier version-confirmation gate (Subject "version"),
// failing the test if no notes gate fired. The version gate always leads a real
// run now, so gate-content assertions target the notes gate explicitly.
func promptGate(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Gate {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt && ev.Prompt.Subject != "version" {
			return ev.Prompt
		}
	}
	t.Fatalf("no notes Prompt event recorded")
	return presenter.Gate{}
}

// keysOf is a thin alias for gate.Keys() reading nicely at the assertion site.
func keysOf(gate presenter.Gate) []presenter.Choice {
	return gate.Keys()
}

// equalChoices reports whether two choice slices are equal in order and contents.
func equalChoices(a, b []presenter.Choice) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
