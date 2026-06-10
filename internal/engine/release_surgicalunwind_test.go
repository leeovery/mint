package engine_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 4-3: the spine's gate-abort and pre-push failure triggers route
// through the SURGICAL pre-PONR unwind (4-2) — driven off the captured StartState +
// tracked MadeState (the count of commits mint made and whether the tag was created)
// rather than the legacy best-effort HEAD-probe reset. The `n` abort and a pre-push
// failure are treated IDENTICALLY: same captured inputs, identical clean-state result,
// identical engine-authored Unwound summary. The post-PONR publish-failure path is
// proven to stay warn-only and NEVER unwind.

// tagCreationFailed models a `git tag -a` failure (NOT a push rejection): the error is
// a plain failure NOT wrapped in release.ErrPushRejected, so the spine's MadeState
// records TagCreated=false — no tag exists to delete.
func tagCreationFailed() runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{ExitCode: 1},
		Err:    errors.New("fatal: tag already exists"),
	}
}

// TestRelease_GateNo_Surgical_ResetsTrackedCommit proves a gate-`n` abort AFTER a
// pre_tag artifact commit routes through the surgical unwind driven by the tracked
// MadeState: exactly the one commit mint made this run is reset to the captured
// starting sha (no HEAD probe), no tag is deleted (the gate sits before the tag), and
// the engine-authored Unwound names the single reset commit with the repo-clean tail.
func TestRelease_GateNo_Surgical_ResetsTrackedCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	// Read stages, preflight, startingHEAD capture, then the pre_tag hook dirties the
	// tree and mint makes ONE artifact commit — and the gate then answers `n`.
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
		ScriptedOut(""),               // unwind: reset --hard startingSHA
	)
	f.Seed("sh", runner.Result{}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	// The surgical unwind resets the one tracked commit to the captured starting sha.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gate-n surgical unwind did not reset the tracked commit; got %v", commandLines(f.Invocations()))
	}
	// It NEVER probes HEAD — the reset is driven by MadeState, not a rev-parse compare.
	if countCmd(f, "git", "rev-parse", "HEAD") != 1 {
		t.Errorf("surgical unwind probed HEAD; the reset must be driven by MadeState, not a probe (rev-parse HEAD count = %d, want the single pre-gate capture)", countCmd(f, "git", "rev-parse", "HEAD"))
	}
	// No tag was created before the gate, so none is deleted.
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("gate-n deleted a tag though none was created")
	}
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoMutation(t, f)
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_TagCreationFailure_Surgical_ResetsCommitNoTagDelete proves a pre-push
// tag-CREATION failure (not a push rejection) routes through the IDENTICAL surgical
// unwind: the bookkeeping commit mint made is reset, but because no tag was created
// (TagCreated=false), NO `git tag -d` is issued. The Unwound names only the reset.
func TestRelease_TagCreationFailure_Surgical_ResetsCommitNoTagDelete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughCommit(f, root, "main", "v0.0.1")
	// gh auth passes; the annotated tag CREATION fails (not a push rejection).
	f.SeedSequence("git",
		tagCreationFailed(), // tag -a v0.0.1 -F - (creation fails)
		ScriptedOut(""),     // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("tag-creation-failure unwind did not reset the tracked commit; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("tag-creation-failure unwind deleted a tag though none was created")
	}
	// No HEAD probe inside the unwind — only the single pre-gate capture.
	if countCmd(f, "git", "rev-parse", "HEAD") != 1 {
		t.Errorf("surgical unwind probed HEAD; reset must be driven by MadeState (rev-parse HEAD count = %d)", countCmd(f, "git", "rev-parse", "HEAD"))
	}
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_PushRejected_Surgical_ResetsCommitAndDeletesTag proves a push REJECTION
// (post-tag, pre-PONR) routes through the surgical unwind: the rejection means the
// local tag WAS created (TagCreated=true via release.ErrPushRejected), so the unwind
// resets the tracked commit AND deletes the tag, naming both with the repo-clean tail.
func TestRelease_PushRejected_Surgical_ResetsCommitAndDeletesTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGitThroughTag(f, root, "main", "v0.0.1")
	f.SeedSequence("git",
		pushRejected(),  // push --atomic origin HEAD v0.0.1 (rejected)
		ScriptedOut(""), // unwind: tag -d v0.0.1
		ScriptedOut(""), // unwind: reset --hard startingSHA
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("push-rejection unwind did not reset the tracked commit; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("push-rejection unwind did not delete the locally-created tag; got %v", commandLines(f.Invocations()))
	}
	if countCmd(f, "git", "rev-parse", "HEAD") != 1 {
		t.Errorf("surgical unwind probed HEAD; reset must be driven by MadeState (rev-parse HEAD count = %d)", countCmd(f, "git", "rev-parse", "HEAD"))
	}
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit and deleted tag v0.0.1; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_GateNoAndPrePushFailure_IdenticalCleanStateAndSummary proves the spec
// invariant at the heart of 4-3: a gate-`n` abort and a pre-push failure with the SAME
// tracked MadeState produce IDENTICAL clean-state results AND identical Unwound
// summaries. Both scenarios make exactly one commit and create no tag, so the surgical
// unwind issues the identical `git reset --hard {startingSHA}`, issues no `git tag -d`,
// and authors the byte-identical summary.
func TestRelease_GateNoAndPrePushFailure_IdenticalCleanStateAndSummary(t *testing.T) {
	t.Parallel()

	// Gate-`n` after a pre_tag artifact commit: MadeState{Commits:1, TagCreated:false}.
	gateRoot := t.TempDir()
	writeConfig(t, gateRoot, "[release.hooks]\npre_tag = \"build.sh\"\n")
	gateRunner := runner.NewFakeRunner()
	gateRunner.SeedSequence("git",
		ScriptedOut(gateRoot),         // rev-parse --show-toplevel
		ScriptedOut("origin/main"),    // symbolic-ref --short origin/HEAD
		ScriptedOut(""),               // tag --list
		ScriptedOut(""),               // fetch --tags
		ScriptedOut(""),               // status --porcelain (clean)
		ScriptedOut("main"),           // rev-parse --abbrev-ref HEAD
		ScriptedNonZero(),             // rev-parse -q --verify refs/tags/v0.0.1
		ScriptedOut("0\t1"),           // rev-list left-right count
		ScriptedOut(""),               // ls-remote --tags
		ScriptedOut(startingSHA),      // rev-parse HEAD (capture clean start)
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: DIRTY)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
		ScriptedOut(""),               // unwind: reset --hard startingSHA
	)
	gateRunner.Seed("sh", runner.Result{}, nil)
	gateRunner.Seed("gh", runner.Result{}, nil)
	gateRec := &presentertest.RecordingPresenter{
		NextChoices: []presenter.Choice{presenter.ChoiceNo},
	}
	gateErr := engine.Release(t.Context(), newDeps(gateRec, gateRunner), patchOptions())

	// Pre-push tag-CREATION failure after one bookkeeping commit: the identical
	// MadeState{Commits:1, TagCreated:false}.
	pushRoot := t.TempDir()
	pushRunner := runner.NewFakeRunner()
	seedHappyGitThroughCommit(pushRunner, pushRoot, "main", "v0.0.1")
	pushRunner.SeedSequence("git",
		tagCreationFailed(), // tag -a v0.0.1 -F - (creation fails → no tag)
		ScriptedOut(""),     // unwind: reset --hard startingSHA
	)
	pushRunner.Seed("gh", runner.Result{}, nil)
	pushRec := &presentertest.RecordingPresenter{}
	pushErr := engine.Release(t.Context(), newDeps(pushRec, pushRunner), patchOptions())

	// Both abort non-zero and both end in an Unwound.
	assertAbortNonZero(t, gateErr)
	assertAbortNonZero(t, pushErr)

	// IDENTICAL clean-state result: both issue the same reset, neither deletes a tag.
	if !invokedWith(gateRunner, "git", "reset", "--hard", startingSHA) {
		t.Errorf("gate-n did not issue the shared reset to the starting sha")
	}
	if !invokedWith(pushRunner, "git", "reset", "--hard", startingSHA) {
		t.Errorf("pre-push failure did not issue the shared reset to the starting sha")
	}
	if invokedWith(gateRunner, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("gate-n deleted a tag; no tag was created")
	}
	if invokedWith(pushRunner, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("tag-creation failure deleted a tag; none was created")
	}

	// IDENTICAL engine-authored Unwound summaries — the assertion at the core of 4-3.
	gateSummary := unwoundSummary(t, gateRec)
	pushSummary := unwoundSummary(t, pushRec)
	if gateSummary != pushSummary {
		t.Errorf("gate-n and pre-push failure produced DIFFERENT Unwound summaries:\n gate=%q\n push=%q", gateSummary, pushSummary)
	}
	if want := "reset 1 commit; repo clean"; gateSummary != want {
		t.Errorf("shared Unwound.Summary = %q, want %q", gateSummary, want)
	}
}

// TestRelease_NotesFailure_AfterArtifactCommit_SurgicalResets proves a notes failure
// (on_notes_failure=abort) that occurs AFTER a pre_tag artifact commit routes through
// the surgical unwind — NOT a plain surface — so the one tracked commit mint made is
// reset back to the captured starting sha. Notes resolution sits after the pre_tag
// hook, so a pre_tag artifact commit can already exist; the surgical unwind discards it
// exactly like any other pre-push failure. No tag was created, so none is deleted.
func TestRelease_NotesFailure_AfterArtifactCommit_SurgicalResets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")

	f := runner.NewFakeRunner()
	// Prior-tag read gates + startingHEAD capture (so the AI notes path is reached at
	// all — a first release would short-circuit to "Initial release." with no AI).
	seedPriorTagReadGates(f, root, "main")
	// The pre_tag hook dirties the tree and mint makes ONE artifact commit.
	f.SeedSequence("git",
		ScriptedOut(" M bundle.js\n"), // -C root status --porcelain (post-hook: DIRTY)
		ScriptedOut(""),               // -C root add -A
		ScriptedOut(""),               // -C root commit -m chore(release): pre-tag artifacts
	)
	f.Seed("sh", runner.Result{}, nil)
	// The notes selector then runs the AI path; the AI returns empty on both attempts
	// → ai.ErrNotesFailure → default abort mode → notes failure AFTER the artifact commit.
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: ""}, nil)
	// The surgical unwind then resets the one tracked artifact commit.
	f.SeedSequence("git", ScriptedOut("")) // unwind: reset --hard startingSHA
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	// The tracked artifact commit is reset back to the clean start through the surgical unwind.
	if !invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("notes failure after an artifact commit did not reset it; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "tag", "-d", nextTag) {
		t.Errorf("notes failure deleted a tag though none was created")
	}
	// A StageFailed (the notes failure) precedes the Unwound.
	assertStageFailedThenUnwound(t, rec)
	if got, want := unwoundSummary(t, rec), "reset 1 commit; repo clean"; got != want {
		t.Errorf("Unwound.Summary = %q, want %q", got, want)
	}
	assertNoMutation(t, f)
	assertNoFinishAfterUnwound(t, rec)
}

// TestRelease_PublishFailsAfterPush_NeverUnwinds_Surgical proves the post-PONR
// asymmetry holds under the rewired triggers: a publish-create failure AFTER a
// successful atomic push does NOT route through the surgical unwind. No `git reset`
// and no `git tag -d` are issued, no Unwound fires, the run warns (pointing to the
// regenerate --reuse heal path) and still finishes successfully, returning nil.
func TestRelease_PublishFailsAfterPush_NeverUnwinds_Surgical(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.SeedSequence("gh",
		ScriptedOut(""), // gh auth status (authenticated)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("gh: server error")}, // release create fails
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned %v, want nil (warn-only post-PONR, never unwinds)", err)
	}

	// The push crossed the PONR; the tag is public, so the surgical unwind must NOT run.
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("post-PONR publish failure issued a `git reset`; it must never unwind a public tag")
	}
	if invokedWith(f, "git", "tag", "-d", "v0.0.1") {
		t.Errorf("post-PONR publish failure deleted the tag; it must never unwind a public tag")
	}
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("post-PONR publish failure emitted an Unwound; it must warn only")
	}
	// It warns and points to the heal path, then finishes successfully.
	if !recorded(rec, presentertest.KindWarn) {
		t.Errorf("post-PONR publish failure did not surface a Warn")
	}
	if !publishHealWarnRecorded(rec) {
		t.Errorf("publish-failure warn did not point to the regenerate --reuse heal path; warns = %v", warnMessages(rec))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish after warn-only publish failure; last event = %v", fin.Kind)
	}
}

// countCmd counts how many recorded invocations match name+args exactly.
func countCmd(f *runner.FakeRunner, name string, args ...string) int {
	want := name + " " + strings.Join(args, " ")
	n := 0
	for _, inv := range f.Invocations() {
		if commandLine(inv) == want {
			n++
		}
	}
	return n
}

// publishHealWarnRecorded reports whether any recorded Warn points to the
// regenerate --reuse heal path — the spec-fixed post-PONR publish-failure guidance.
func publishHealWarnRecorded(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && strings.Contains(ev.Warn.Message, "regenerate --reuse") {
			return true
		}
	}
	return false
}

// warnMessages collects every recorded Warn message for failure output.
func warnMessages(rec *presentertest.RecordingPresenter) []string {
	var msgs []string
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			msgs = append(msgs, ev.Warn.Message)
		}
	}
	return msgs
}
