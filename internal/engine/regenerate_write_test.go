package engine_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// This file pins task 5-9: the single-version regenerate WRITE / PUSH / RECOVERY
// orchestration. It wires the 5-3..5-8 pieces into one sequence:
//
//	gate/confirm → (fresh: write changelog commit) → push (PONR) → provider write
//
// The load-bearing facts under test:
//   - the push is PLAIN `git push origin HEAD` (no tag, NOT the forward
//     `--atomic origin HEAD {tag}`);
//   - a gate abort (fresh) or any pre-push failure RESETS the local CHANGELOG commit
//     (a lighter unwind than the forward surgical unwind — just the commit reset, no
//     tag);
//   - a provider failure AFTER the changelog push is WARN ONLY (never a reset);
//   - --target both is NON-ATOMIC across surfaces: changelog (commit + push) FIRST,
//     then the provider release;
//   - fresh runs the notes-review gate, reuse is a simple confirm (no review gate),
//     -y skips the confirm/gate (modelled by the recorder returning the gate default);
//   - the changelog write uses the version's ORIGINAL HISTORICAL date read from the
//     tag (`git for-each-ref --format=%(creatordate:short)`), NOT today.

const (
	regenWriteTag        = "v1.4.0"
	regenWriteVersionKey = "1.4.0"
	regenWriteBody       = "## What's Changed\n\n- Healed the notes\n"
	regenWriteHistorical = "2024-02-15" // the tag's original creatordate (NOT today)
)

// forEachRefDateArgs is the exact git argv the historical-date read must issue: a
// single for-each-ref with the creatordate:short selector against refs/tags/<tag>.
func forEachRefDateArgs(tag string) []string {
	return []string{"for-each-ref", "--format=%(creatordate:short)", "refs/tags/" + tag}
}

// freshWriteReq builds a fresh-source single-version write request for the given
// target with the canned tag/version/body.
func freshWriteReq(target engine.RegenerateTarget) engine.RegenerateWriteRequest {
	return engine.RegenerateWriteRequest{
		Source:     engine.RegenerateSourceFresh,
		Target:     target,
		Tag:        regenWriteTag,
		VersionKey: regenWriteVersionKey,
		Body:       regenWriteBody,
	}
}

// reuseWriteReq builds a reuse-source single-version write request for the given
// target.
func reuseWriteReq(target engine.RegenerateTarget) engine.RegenerateWriteRequest {
	return engine.RegenerateWriteRequest{
		Source:     engine.RegenerateSourceReuse,
		Target:     target,
		Tag:        regenWriteTag,
		VersionKey: regenWriteVersionKey,
		Body:       regenWriteBody,
	}
}

// resetIssued reports whether the FakeRunner recorded a `git reset --hard` (the
// regenerate commit-reset recovery).
func resetIssued(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && len(inv.Args) >= 2 && inv.Args[0] == "reset" && inv.Args[1] == "--hard" {
			return true
		}
	}
	return false
}

// TestRegenerateWrite_Changelog_PushIsPlainNoTag proves a fresh --target changelog
// run pushes with the PLAIN `git push origin HEAD` form — no tag, NOT the forward
// `--atomic origin HEAD {tag}`.
func TestRegenerateWrite_Changelog_PushIsPlainNoTag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD (capture clean start)
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short (historical date)
		ScriptedOut(""),                   // -C dir add CHANGELOG.md
		ScriptedOut(""),                   // -C dir commit -m docs(changelog): ...
		ScriptedOut(""),                   // push origin HEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))
	if err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "push", "origin", "HEAD") {
		t.Errorf("no plain `git push origin HEAD`; got %v", commandLines(f.Invocations()))
	}
	// It must NOT use the forward atomic-with-tag push form.
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, "--atomic") {
			t.Errorf("push used the forward --atomic form %v; regenerate pushes plain (no tag)", inv.Args)
		}
		if inv.Name == "git" && inv.Args[0] == "push" && slices.Contains(inv.Args, regenWriteTag) {
			t.Errorf("push carried the tag %v; regenerate never pushes a tag", inv.Args)
		}
	}
}

// TestRegenerateWrite_Changelog_UsesHistoricalDateNotToday proves the changelog
// write recovers the version's ORIGINAL historical date from the tag via
// `git for-each-ref --format=%(creatordate:short)` and writes the section header
// with THAT date — never today — preserving changelog data integrity.
func TestRegenerateWrite_Changelog_UsesHistoricalDateNotToday(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // add
		ScriptedOut(""),                   // commit
		ScriptedOut(""),                   // push origin HEAD
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	// The for-each-ref creatordate read must have happened.
	if !invokedWith(f, "git", forEachRefDateArgs(regenWriteTag)...) {
		t.Errorf("no `git for-each-ref --format=%%(creatordate:short)` historical-date read; got %v", commandLines(f.Invocations()))
	}

	got := readChangelogFile(t, dir)
	// renderSection appends a trailing newline after the body, so the canned body
	// (which already ends in \n) yields a blank line before EOF.
	want := kacPreamble + "\n## [1.4.0] - 2024-02-15\n\n" + regenWriteBody + "\n"
	if got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant the healed body under the HISTORICAL header date\n%q", got, want)
	}
	if !strings.Contains(got, "## [1.4.0] - 2024-02-15") {
		t.Errorf("header date drifted from the historical 2024-02-15:\n%q", got)
	}
}

// TestRegenerateWrite_Fresh_GateAbort_ResetsCommit proves a fresh run whose review
// gate is declined routes to the commit-reset recovery and never pushes. With the
// gate sitting BEFORE the changelog commit, no commit exists to reset — but the run
// still aborts non-zero and issues no push.
func TestRegenerateWrite_Fresh_GateAbort_ResetsCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	// Nothing is seeded for git: the gate aborts before any git mutation.
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	if invokedWith(f, "git", "push", "origin", "HEAD") {
		t.Errorf("a gate abort pushed; the gate sits before the PONR")
	}
	// The changelog must be untouched (no write happened).
	if got := readChangelogFile(t, dir); got != kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n" {
		t.Errorf("CHANGELOG.md changed on a gate abort:\n%q", got)
	}
}

// TestRegenerateWrite_Fresh_GateAbortAfterCommit_ResetsCommit proves that when a
// commit DID land before an abort-routing failure, the recovery resets the local
// CHANGELOG commit to the captured starting HEAD (a plain commit reset, no tag).
// This exercises the reset path via a push rejection (any pre-push failure resets).
func TestRegenerateWrite_PrePushFailure_ResetsCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD (captured start)
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // add
		ScriptedOut(""),                   // commit (the changelog commit lands)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("remote rejected")}, // push fails
		ScriptedOut(""), // reset --hard startHEAD (recovery)
	)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), nil, dir, freshWriteReq(engine.RegenerateTargetChangelog))

	assertAbortNonZero(t, err)
	if !invokedWith(f, "git", "reset", "--hard", "startHEAD") {
		t.Errorf("pre-push failure did not reset the CHANGELOG commit to the captured start; got %v", commandLines(f.Invocations()))
	}
	// No tag is ever involved in the regenerate recovery.
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && inv.Args[0] == "tag" {
			t.Errorf("recovery touched a tag %v; regenerate recovery is commit-reset only", inv.Args)
		}
	}
}

// TestRegenerateWrite_ProviderFailureAfterPush_WarnOnly proves that for --target
// both, a provider create/update failure AFTER the changelog push is WARN ONLY: the
// changelog is already public, so the run does not reset and exits successfully.
func TestRegenerateWrite_ProviderFailureAfterPush_WarnOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // add
		ScriptedOut(""),                   // commit
		ScriptedOut(""),                   // push origin HEAD (PONR crossed)
	)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, true, errors.New("HTTP 500 from provider"))
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, freshWriteReq(engine.RegenerateTargetBoth))
	if err != nil {
		t.Fatalf("RegenerateWrite returned error %v, want nil (provider failure post-push is warn-only)", err)
	}

	// A warn must have been emitted, and NO reset followed the push.
	if !hasWarn(rec) {
		t.Errorf("no Warn emitted for the post-push provider failure; kinds = %v", rec.Kinds())
	}
	if resetIssued(f) {
		t.Errorf("a `git reset` followed a post-push provider failure; post-PONR must never unwind")
	}
}

// TestRegenerateWrite_Both_ChangelogPushBeforeProvider proves --target both is
// non-atomic across surfaces and ordered: the changelog commit + push happen FIRST,
// then the provider release. The fake Publisher captures the git invocation count at
// dispatch time, which must be AFTER the push.
func TestRegenerateWrite_Both_ChangelogPushBeforeProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2024-02-15\n\nStale body.\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut("startHEAD"),          // rev-parse HEAD
		ScriptedOut(regenWriteHistorical), // for-each-ref creatordate:short
		ScriptedOut(""),                   // add
		ScriptedOut(""),                   // commit
		ScriptedOut(""),                   // push origin HEAD
	)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, true, nil)
	pushedBeforeDispatch := false
	pub.beforeDispatch = func() {
		pushedBeforeDispatch = invokedWith(f, "git", "push", "origin", "HEAD")
	}
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, freshWriteReq(engine.RegenerateTargetBoth)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	if !pushedBeforeDispatch {
		t.Errorf("the provider release was dispatched before the changelog push; --target both must write changelog (commit + push) FIRST")
	}
	if len(pub.dispatched) != 1 {
		t.Fatalf("provider dispatched %d times, want exactly 1", len(pub.dispatched))
	}
	if got := pub.dispatched[0]; got.method != "update" || got.tag != regenWriteTag || got.body != regenWriteBody {
		t.Errorf("provider dispatch = %+v, want update for %s with the healed body", got, regenWriteTag)
	}
}

// TestRegenerateWrite_Reuse_SimpleConfirmNoReviewGate proves a reuse run uses the
// simple two-choice confirm gate (ReuseConfirmGate, y/n) — NOT the four-choice
// notes-review gate — because reuse generates no new notes to edit/regenerate.
func TestRegenerateWrite_Reuse_SimpleConfirmNoReviewGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, false, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	// reuse implies --target release (release-only, no changelog write).
	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, reuseWriteReq(engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	gate := singlePromptGate(t, rec)
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo}
	if !slices.Equal(gate.Keys(), wantKeys) {
		t.Errorf("reuse gate keys = %v, want the simple confirm %v (no e/r review gate)", gate.Keys(), wantKeys)
	}
}

// TestRegenerateWrite_Fresh_RunsNotesReviewGate proves a fresh run uses the
// four-choice notes-review gate (y/n/e/r) — the freshly-generated notes are
// reviewable before they overwrite the live release.
func TestRegenerateWrite_Fresh_RunsNotesReviewGate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, true, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, freshWriteReq(engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	gate := singlePromptGate(t, rec)
	wantKeys := []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo, presenter.ChoiceEdit, presenter.ChoiceRegen}
	if !slices.Equal(gate.Keys(), wantKeys) {
		t.Errorf("fresh gate keys = %v, want the four-choice review gate %v", gate.Keys(), wantKeys)
	}
}

// TestRegenerateWrite_Release_NoChangelogCommitOrPush proves --target release does
// the provider write only: no changelog write, no commit, no push.
func TestRegenerateWrite_Release_NoChangelogCommitOrPush(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)
	pub := newFakePublisher()
	pub.seedExists(regenWriteTag, false, nil)
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	if err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, reuseWriteReq(engine.RegenerateTargetRelease)); err != nil {
		t.Fatalf("RegenerateWrite returned unexpected error: %v", err)
	}

	for _, inv := range f.Invocations() {
		if inv.Name == "git" && (slices.Contains(inv.Args, "commit") || slices.Contains(inv.Args, "push") || slices.Contains(inv.Args, "add")) {
			t.Errorf("--target release issued a changelog mutation %v; it must do provider write only", inv.Args)
		}
	}
	if len(pub.dispatched) != 1 || pub.dispatched[0].method != "create" {
		t.Errorf("provider dispatch = %+v, want exactly one create (absent release)", pub.dispatched)
	}
}

// TestRegenerateWrite_Reuse_GateDecline_Aborts proves a declined reuse confirm
// aborts non-zero with no provider write and no git mutation.
func TestRegenerateWrite_Reuse_GateDecline_Aborts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	f := runner.NewFakeRunner()
	pub := newFakePublisher()
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceNo}}

	err := engine.RegenerateWrite(t.Context(), regenWriteDeps(rec, f), pub, dir, reuseWriteReq(engine.RegenerateTargetRelease))

	assertAbortNonZero(t, err)
	if len(pub.dispatched) != 0 {
		t.Errorf("a declined reuse confirm dispatched a provider write %+v", pub.dispatched)
	}
	if len(f.Invocations()) != 0 {
		t.Errorf("a declined reuse confirm issued git %v", commandLines(f.Invocations()))
	}
}

// regenWriteDeps builds the ReleaseDeps for the write-path tests: a recording
// presenter, the single FakeRunner, and a Mutator over it (shared with no Releaser —
// regenerate cuts no tag and uses a plain push, so the Releaser is unused here).
func regenWriteDeps(rec *presentertest.RecordingPresenter, f *runner.FakeRunner) engine.ReleaseDeps {
	mut := git.NewMutator(f)
	return engine.ReleaseDeps{
		Presenter: rec,
		Runner:    f,
		Mutator:   mut,
	}
}

// singlePromptGate returns the single Prompt gate the recorder captured, failing if
// none (or more than one) fired — the write path issues exactly one gate/confirm.
func singlePromptGate(t *testing.T, rec *presentertest.RecordingPresenter) presenter.Gate {
	t.Helper()
	var gates []presenter.Gate
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			gates = append(gates, ev.Prompt)
		}
	}
	if len(gates) != 1 {
		t.Fatalf("recorded %d Prompt gates, want exactly 1; kinds = %v", len(gates), rec.Kinds())
	}
	return gates[0]
}

// hasWarn reports whether the recorder captured any Warn event.
func hasWarn(rec *presentertest.RecordingPresenter) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn {
			return true
		}
	}
	return false
}
