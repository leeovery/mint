package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/publish"
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
		ScriptedOut(""),                      // -C root add CHANGELOG.md
		ScriptedOut(""),                      // -C root commit -m
		ScriptedOut(""),                      // tag -a {tag} -F -
		ScriptedOut(""),                      // push --atomic origin HEAD {tag}
	)
}

// newDeps builds the orchestrator's dependency set around a single FakeRunner so
// every external call (git via the units, gh via the publisher) is scripted and
// recorded on one timeline.
func newDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	return engine.ReleaseDeps{
		Presenter: rec,
		Runner:    f,
		Releaser:  release.NewReleaser(f),
		Publisher: publish.NewGitHubPublisher(f),
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
		"git -C " + root + " add CHANGELOG.md",
		"git -C " + root + " commit -m 🌿 Release v0.0.1",
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

	// The presenter event timeline must emit in spec order: RunStarted, ShowPlan,
	// ShowNotes, Prompt — then end on RunFinished (success).
	wantKinds := []presentertest.EventKind{
		presentertest.KindRunStarted,
		presentertest.KindShowPlan,
		presentertest.KindShowNotes,
		presentertest.KindPrompt,
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

	// RunStarted carries the engine-set Action and Leaf (from commit_prefix).
	start, _ := rec.At(0)
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
	notes, _ := rec.At(2)
	if notes.ShowNotes.Body != "Initial release." {
		t.Errorf("ShowNotes.Body = %q, want %q", notes.ShowNotes.Body, "Initial release.")
	}

	// RunFinished carries the resolved version.
	fin, _ := rec.At(4)
	if fin.RunFinished.Version != "0.0.1" {
		t.Errorf("RunFinished.Version = %q, want %q", fin.RunFinished.Version, "0.0.1")
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

// TestRelease_AlwaysPromptsUnderYes proves the engine ALWAYS calls Prompt at the
// review gate — even under -y the recorder records KindPrompt and the run proceeds
// on the gate's returned default (the -y skip happens inside the presenter, which
// the recorder models by returning the default). The run reaches a successful
// RunFinished without any extra branching around the call.
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

	if !recorded(rec, presentertest.KindPrompt) {
		t.Errorf("engine did not call Prompt; it must always prompt at the gate")
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
				PromptResult: func(presenter.Gate) (presenter.Choice, error) {
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
// the run (non-zero exit) before any mutation, surfacing an Unwound and stopping.
func TestRelease_GateNo_AbortsNonZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want a gate-no abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindUnwound) {
		t.Errorf("gate-no did not surface an Unwound event")
	}
	assertNoMutation(t, f)
}

// TestRelease_GateEditThenYes proves the Phase 1 minimal edit handling: an `e`
// answer re-shows the notes and re-prompts ONCE; a subsequent `y` proceeds the
// spine through to a successful release. No $EDITOR is invoked.
func TestRelease_GateEditThenYes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The notes are shown twice (initial + re-show on edit) and prompted twice.
	if got := countKind(rec, presentertest.KindShowNotes); got != 2 {
		t.Errorf("ShowNotes count = %d, want 2 (initial + edit re-show)", got)
	}
	if got := countKind(rec, presentertest.KindPrompt); got != 2 {
		t.Errorf("Prompt count = %d, want 2 (initial + re-prompt after edit)", got)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("edit-then-yes did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_GateEditThenNo proves the edit re-prompt still honours a `no`: an
// `e` followed by `n` re-shows the notes, then aborts (Unwound, non-zero exit)
// before any mutation.
func TestRelease_GateEditThenNo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceEdit, presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want an edit-then-no abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindUnwound) {
		t.Errorf("edit-then-no did not surface an Unwound event")
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
	// The gate never ran the review prompt or any mutation.
	if recorded(rec, presentertest.KindPrompt) {
		t.Errorf("review gate prompted despite a failing preflight gate")
	}
	assertNoMutation(t, f)
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
	notes, _ := rec.At(2)
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
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(""),            // status --porcelain
		ScriptedOut("main"),        // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),          // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),        // rev-list left-right count
		ScriptedOut(""),            // ls-remote --tags
		ScriptedOut(""),            // tag -a v0.0.1 -F -
		ScriptedOut(""),            // push --atomic origin HEAD v0.0.1
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
