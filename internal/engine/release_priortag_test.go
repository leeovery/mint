package engine_test

// This file holds the Phase 2 end-to-end PRIOR-TAG wiring tests (task 2-16): a
// repo WITH a prior tag is driven through the whole spine with the unified notes
// selector resolving the body from the precedence (no opts.NotesBody override).
// The AI transport is the REAL ai.Transport over the same FakeRunner, so the
// `claude -p` call is scripted on the one git/gh/claude timeline — no real
// process is spawned. The forward-path body-distribution, gate, unwind, and PONR
// invariants are re-asserted here on the prior-tag path.

import (
	"errors"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// priorTag is the prior release tag the prior-tag tests seed into `git tag --list`
// so CurrentVersion resolves 1.2.3, Next (patch) yields v1.2.4, and FirstRelease
// is false — the AI path's diff base is then priorTag..HEAD.
const priorTag = "v1.2.3"

// nextTag is the patch-bumped tag the prior-tag tests expect (v1.2.3 -> v1.2.4).
const nextTag = "v1.2.4"

// aiBody is the distinctive multi-line body the scripted `claude` call returns on
// the prior-tag NORMAL-AI path, so the tests can prove the AI body flows IDENTICALLY
// to the tag annotation, the CHANGELOG section, and the provider release.
const aiBody = "TL;DR: the auth package landed and token refresh is fixed.\n\n" +
	"## ✨ Added\n" +
	"- **New auth package** with login and session handling\n\n" +
	"## 🐛 Fixed\n" +
	"- Token refresh no longer drops the session\n"

// priorTagDiff is the non-degenerate post-exclusion diff the assemble step returns
// for priorTag..HEAD, driving the normal AI path (non-empty, non-whitespace).
const priorTagDiff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"

// seedPriorTagReadGates scripts the read-side git timeline for a prior-tag run up
// to and including the pre-gate `git rev-parse HEAD` capture: the same shape as
// the first-release read gates, but `git tag --list` returns priorTag so the run
// is NOT a first release and the diff base is priorTag..HEAD.
func seedPriorTagReadGates(f *runner.FakeRunner, root, releaseBranch string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"),           // tag --list (a prior tag exists)
		ScriptedOut(""),                      // fetch --tags
		ScriptedOut(""),                      // status --porcelain (clean)
		ScriptedOut(releaseBranch),           // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                    // rev-parse -q --verify refs/tags/v1.2.4 (absent)
		ScriptedOut("0\t1"),                  // rev-list left-right count (ahead only)
		ScriptedOut(""),                      // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),             // rev-parse HEAD (capture the clean start)
	)
}

// seedNormalAINotes scripts the SelectBody normal-AI assembly: the degenerate-check
// diff (non-degenerate), then the Change Map's name-status + numstat. The caller
// seeds the `claude` transport outcome separately. With these the selector reaches
// the AI path and returns its body as KindNormalAI.
func seedNormalAINotes(f *runner.FakeRunner) {
	f.SeedSequence("git",
		ScriptedOut(priorTagDiff),             // diff priorTag..HEAD (degenerate-check assemble)
		ScriptedOut("A\tauth/login.go\n"),     // diff --name-status (change map)
		ScriptedOut("20\t0\tauth/login.go\n"), // diff --numstat (change map)
	)
}

// seedRecordTagPush scripts the mutation tail shared by the prior-tag happy paths:
// the bookkeeping commit, the annotated tag, and the atomic push. The gh auth +
// release create are seeded by the caller on the "gh" timeline.
func seedRecordTagPush(f *runner.FakeRunner, root string) {
	f.SeedSequence("git",
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v1.2.4 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v1.2.4
	)
}

// priorTagNormalAIOptions is the default-bump options with the fixed clock and
// NoAI=false — the prior-tag NORMAL-AI path. No NotesBody override is set, so the
// body is resolved by the selector from the precedence.
func priorTagNormalAIOptions() engine.ReleaseOptions {
	return engine.ReleaseOptions{Bump: version.BumpPatch, Now: fixedClock}
}

// TestRelease_PriorTag_NormalAI_EndToEnd drives a repo WITH a prior tag through the
// whole spine on the NORMAL-AI path: the selector assembles the priorTag..HEAD
// diff, runs the AI, and the resulting body flows IDENTICALLY to the tag
// annotation, the CHANGELOG, and the provider release. The diff base is
// priorTag..HEAD, the gate is accepted, and the run reaches RunFinished.
func TestRelease_PriorTag_NormalAI_EndToEnd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil) // the AI returns the distinctive body
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The AI body flows IDENTICALLY to all three sinks.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want AI body %q", got, aiBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != aiBody {
		t.Errorf("CHANGELOG body = %q, want AI body %q", got, aiBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != aiBody {
		t.Errorf("provider release body = %q, want AI body %q", got, aiBody)
	}

	// The diff base is priorTag..HEAD — the assemble step ranged from the prior tag.
	if !invokedWith(f, "git", "diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("diff base is not %s..HEAD; got %v", priorTag, commandLines(f.Invocations()))
	}
	// The claude transport was driven over the FakeRunner (no real process), receiving
	// the assembled diff in its prompt on stdin.
	if got := stdinOf(t, f, "claude", "-p"); got == "" {
		t.Errorf("claude was not invoked with a prompt on stdin")
	}

	// The run finished successfully on the prior-tag path.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("prior-tag normal-AI run did not finish; last event = %v", fin.Kind)
	}
	if fin.RunFinished.Version != "1.2.4" {
		t.Errorf("RunFinished.Version = %q, want %q", fin.RunFinished.Version, "1.2.4")
	}
}

// TestRelease_PriorTag_DiffExcludeGlobsReachDiffAndChangeMapGitCalls wires the
// shared top-level diff_exclude config through the whole spine: a .mint.toml at the
// repo root carries two globs, and resolveBody must thread them into the Assembler so
// the AssembleDiff git call AND both Change Map git calls carry each :(exclude)<glob>
// ON TOP OF the built-in :(exclude)CHANGELOG.md, in config order. git — not Go —
// performs the exclusion, so the assertion is on the exact git argv.
func TestRelease_PriorTag_DiffExcludeGlobsReachDiffAndChangeMapGitCalls(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "diff_exclude = [\"skills/**/knowledge.cjs\", \"*.min.js\"]\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The configured globs ride on top of CHANGELOG.md on the assemble diff and both
	// Change Map git calls, in config order — git does the exclusion.
	const g1, g2 = ":(exclude)skills/**/knowledge.cjs", ":(exclude)*.min.js"
	if !invokedWith(f, "git", "diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", g1, g2) {
		t.Errorf("assemble diff did not carry the configured diff_exclude globs; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "diff", "--name-status", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", g1, g2) {
		t.Errorf("change map name-status did not carry the configured diff_exclude globs; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "diff", "--numstat", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", g1, g2) {
		t.Errorf("change map numstat did not carry the configured diff_exclude globs; got %v", commandLines(f.Invocations()))
	}
}

// TestRelease_PriorTag_PlainVersionFile_ExcludeReachesDiffAndChangeMapButIsInert wires
// a PLAIN-mode version_file (version_file set, NO version_pattern) through the spine: the
// strategy-aware decision must thread the :(exclude)<version_file> entry into the
// Assembler so the AssembleDiff git call AND both Change Map git calls carry it ON TOP OF
// :(exclude)CHANGELOG.md — the decision is COMPUTED here. It is nonetheless INERT on the
// forward path: notes generate (Stage 4) precedes the version write (Stage 5), so the
// version file is unchanged at notes time and the produced body is the SAME AI body
// regardless. The test proves both: the entry rides the argv, and the run finishes with
// the unchanged body reaching the sinks (no behavioural difference). The rule exists so
// the regenerate path (Phase 5) inherits a correct decision — not exercised here.
func TestRelease_PriorTag_PlainVersionFile_ExcludeReachesDiffAndChangeMapButIsInert(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"release.txt\"\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The plain-mode version_file decision is COMPUTED: the :(exclude)release.txt entry
	// rides the assemble diff and BOTH Change Map git calls, after :(exclude)CHANGELOG.md.
	const vf = ":(exclude)release.txt"
	if !invokedWith(f, "git", "diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", vf) {
		t.Errorf("assemble diff did not carry the plain-mode version_file exclude; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "diff", "--name-status", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", vf) {
		t.Errorf("change map name-status did not carry the plain-mode version_file exclude; got %v", commandLines(f.Invocations()))
	}
	if !invokedWith(f, "git", "diff", "--numstat", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", vf) {
		t.Errorf("change map numstat did not carry the plain-mode version_file exclude; got %v", commandLines(f.Invocations()))
	}

	// INERT: the decision changes nothing on the forward path — the unchanged AI body
	// still reaches the tag annotation and the run finishes.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want unchanged AI body %q (version_file exclude is inert here)", got, aiBody)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("plain-version_file run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_EmbeddedVersionFile_NotExcludedFromDiff wires an EMBEDDED-mode
// version_file (version_file + version_pattern) through the spine: the strategy does NOT
// exclude it — it is real source we want in the notes. The argv must carry the built-in
// :(exclude)CHANGELOG.md but NO :(exclude)<version_file> entry on the assemble diff or
// the Change Map calls (the lone version-line bump is neutralised by the prompt rule, not
// by hiding source).
func TestRelease_PriorTag_EmbeddedVersionFile_NotExcludedFromDiff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\nversion_file = \"main.go\"\nversion_pattern = \"RELEASE_VERSION = {version}\"\n")
	// Embedded mode operates on a REAL source file: seed main.go with a line matching
	// version_pattern so the Stage-5 version write (downstream of the diff under test)
	// succeeds and the run completes — keeping the test focused on the diff argv.
	writeFile(t, root, "main.go", "package main\n\nconst RELEASE_VERSION = 1.2.3\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The assemble diff carries the built-in CHANGELOG.md exclude but NOT the version_file
	// — embedded mode keeps the source file in the notes.
	if !invokedWith(f, "git", "diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("assemble diff did not carry the built-in CHANGELOG.md exclude; got %v", commandLines(f.Invocations()))
	}
	if invokedWith(f, "git", "diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)main.go") {
		t.Errorf("assemble diff excluded the embedded-mode version_file; embedded source must stay in the notes")
	}
}

// TestRelease_PriorTag_NormalAI_EventProtocol asserts the as-built event protocol on
// the prior-tag NORMAL-AI success path: RunStarted -> ShowPlan -> ShowNotes ->
// Prompt -> RunFinished. The gate is the four-choice y/n/e/r variant (KindNormalAI).
func TestRelease_PriorTag_NormalAI_EventProtocol(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The event timeline OPENS with RunStarted, then the version-confirmation Prompt
	// (Stage 1, ahead of preflight), then narrates the read-only preflight gate, then
	// the blocking notes stage, then the existing ShowPlan/ShowNotes/Prompt block (the
	// notes review gate), then the blocking push stage, then RunFinished. No pre_tag
	// stage — the prior-tag normal-AI path configures no hook.
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

	// The notes shown are the AI body (resolved by the selector, not a fixed default).
	notesEv, _ := rec.At(indexOfKind(rec, presentertest.KindShowNotes))
	if notesEv.ShowNotes.Body != aiBody {
		t.Errorf("ShowNotes.Body = %q, want AI body %q", notesEv.ShowNotes.Body, aiBody)
	}
	// The gate is the four-choice variant: KindNormalAI offers r.
	gate := promptGate(t, rec)
	if !gate.Has(presenter.ChoiceRegen) {
		t.Errorf("prior-tag normal-AI gate omitted r; KindNormalAI must offer regenerate")
	}
}

// TestRelease_PriorTag_GateAccept_ProceedsToRecord proves the gate ACCEPT on the
// prior-tag path proceeds to Record/tag/push/publish.
func TestRelease_PriorTag_GateAccept_ProceedsToRecord(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; second accepts the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceYes},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "-C", root, "commit", "-m", "🌿 Release "+nextTag) {
		t.Errorf("gate accept did not reach Record (no bookkeeping commit)")
	}
	if !invokedWith(f, "git", "tag", "-a", nextTag, "-F", "-") {
		t.Errorf("gate accept did not tag")
	}
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", nextTag) {
		t.Errorf("gate accept did not push")
	}
	if !invokedWith(f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag") {
		t.Errorf("gate accept did not publish")
	}
}

// TestRelease_PriorTag_GateAbort_LeavesRepoClean proves the gate-n abort on the
// prior-tag path leaves the repo clean (no mutation) and aborts non-zero — the same
// surgical-unwind path as first-release. The gate sits before any commit/tag, so the
// surgical unwind has zero MadeState and no-ops: no Unwound, no reset, no HEAD probe.
func TestRelease_PriorTag_GateAbort_LeavesRepoClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; ChoiceNo declines the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceNo},
	}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())

	assertAbortNonZero(t, err)
	// Nothing was made before the notes gate, so the surgical unwind no-ops — no Unwound.
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("prior-tag gate-no before any mutation emitted an Unwound; nothing was made to undo")
	}
	if invokedWith(f, "git", "reset", "--hard", startingSHA) {
		t.Errorf("prior-tag gate-no issued a `git reset`; nothing to reset")
	}
	assertNoMutation(t, f)
	// No RunFinished success line follows the aborted run.
	for _, k := range rec.Kinds() {
		if k == presentertest.KindRunFinished {
			t.Errorf("a RunFinished followed the gate-n abort; kinds = %v", rec.Kinds())
		}
	}
}

// TestRelease_PriorTag_UnderYes_PromptFiresOnceAndProceeds proves the engine ALWAYS
// calls Prompt on the prior-tag path — even under -y (modelled by an empty
// NextChoices, so Prompt returns the gate Default). Prompt fires for both gates (the
// version-confirmation gate and the notes review gate) and the run proceeds to a
// successful RunFinished on the returned defaults.
func TestRelease_PriorTag_UnderYes_PromptFiresOnceAndProceeds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	// No NextChoices: the recorder returns the gate Default (yes), modelling -y.
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindPrompt); got != 2 {
		t.Errorf("Prompt count = %d, want exactly 2 under -y (version + notes gates)", got)
	}
	// The run proceeds on the default with the AI body reaching the sinks.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want AI body %q", got, aiBody)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("under -y the prior-tag run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_RegenViaRealGenerator_EndToEnd proves the PER-RUN
// regenerator closure: with NO deps.Regenerator injected, an `r` at the gate runs
// the REAL Generator (GenerateWithContext) bound to this run's lastTag + cfg over
// the FakeRunner. It re-assembles the priorTag..HEAD diff, re-runs `claude` with
// the one-time context, and the REGENERATED body — not the first AI body — reaches
// every sink.
func TestRelease_PriorTag_RegenViaRealGenerator_EndToEnd(t *testing.T) {
	t.Parallel()

	const regenBody = "Regenerated via the real generator: lead with auth.\n\n" +
		"## ✨ Added\n- Auth package, foregrounded\n"

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// Initial normal-AI assembly + the regenerate round's re-assembly, on one git
	// timeline: SelectBody assembles + change map, then GenerateWithContext (the `r`
	// closure) assembles + change map again.
	f.SeedSequence("git",
		ScriptedOut(priorTagDiff),             // SelectBody: diff (degenerate-check assemble)
		ScriptedOut("A\tauth/login.go\n"),     // SelectBody: name-status
		ScriptedOut("20\t0\tauth/login.go\n"), // SelectBody: numstat
		ScriptedOut(priorTagDiff),             // regen: GenerateWithContext assemble
		ScriptedOut("A\tauth/login.go\n"),     // regen: name-status
		ScriptedOut("20\t0\tauth/login.go\n"), // regen: numstat
	)
	// The claude transport returns the first AI body, then the regenerated body.
	f.SeedSequence("claude",
		ScriptedOut(aiBody),
		ScriptedOut(regenBody),
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{
		// First ChoiceYes accepts the version gate; the rest drive the notes gate.
		NextChoices: []presenter.Choice{presenter.ChoiceYes, presenter.ChoiceRegen, presenter.ChoiceYes},
		NextLines:   []string{"lead with auth"},
	}

	// No deps.Regenerator: the per-run closure over the real Generator must drive `r`.
	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The one-time context reached the regenerate prompt's instructions.
	if got := stdinOf(t, f, "claude", "-p"); got == "" {
		t.Errorf("claude was not invoked with a prompt on stdin")
	}
	// The REGENERATED body — not the first AI body — reached every sink.
	if got := tagAnnotationBody(t, f, nextTag); got != regenBody {
		t.Errorf("tag annotation body = %q, want REGENERATED body %q", got, regenBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != regenBody {
		t.Errorf("CHANGELOG body = %q, want REGENERATED body %q", got, regenBody)
	}
	if got := stdinOf(t, f, "gh", "release", "create", nextTag, "--title", nextTag, "--notes-file", "-", "--verify-tag"); got != regenBody {
		t.Errorf("provider release body = %q, want REGENERATED body %q", got, regenBody)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("regen-via-real-generator run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_NoAI_EndToEnd proves the --no-ai path end-to-end on a
// prior-tag repo: the selector takes KindNoAI, builds the fallback commit-subject
// body via `git log` (NO claude call), and the body flows to all sinks. The run
// completes.
func TestRelease_PriorTag_NoAI_EndToEnd(t *testing.T) {
	t.Parallel()

	const subjects = "Add login flow\nFix token refresh\n"

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// --no-ai: the selector assembles the diff (non-degenerate), then routes to
	// NoAIBody's commit-subject `git log` — NO change-map, NO claude.
	f.SeedSequence("git",
		ScriptedOut(priorTagDiff), // diff priorTag..HEAD (degenerate-check assemble)
		ScriptedOut(subjects),     // git log --format=%s priorTag..HEAD (no-ai fallback)
	)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	opts := priorTagNormalAIOptions()
	opts.NoAI = true

	err := engine.Release(t.Context(), newDeps(rec, f), opts)
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// No claude call: --no-ai never invokes the AI.
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("claude was invoked under --no-ai: %q", commandLine(inv))
		}
	}
	// The commit-subject fallback body reached every sink.
	if got := tagAnnotationBody(t, f, nextTag); got != subjects {
		t.Errorf("tag annotation body = %q, want --no-ai fallback %q", got, subjects)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != subjects {
		t.Errorf("CHANGELOG body = %q, want --no-ai fallback %q", got, subjects)
	}
	// The git log ranged from priorTag..HEAD.
	if !invokedWith(f, "git", "log", "--format=%s", priorTag+"..HEAD") {
		t.Errorf("no-ai fallback did not range from %s..HEAD; got %v", priorTag, commandLines(f.Invocations()))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("--no-ai run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_Degenerate_EndToEnd proves the degenerate path end-to-end on
// a prior-tag repo: the post-exclusion diff is empty, so the selector takes
// KindDegenerate, writes the StubBody (NO claude call), and the run completes.
func TestRelease_PriorTag_Degenerate_EndToEnd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// The post-exclusion diff is empty -> degenerate; no change map, no claude.
	f.SeedSequence("git", ScriptedOut("")) // diff priorTag..HEAD (empty -> degenerate)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("claude was invoked on the degenerate path: %q", commandLine(inv))
		}
	}
	want := "Maintenance release — no notable source changes"
	if got := tagAnnotationBody(t, f, nextTag); got != want {
		t.Errorf("tag annotation body = %q, want degenerate StubBody %q", got, want)
	}
	// ShowNotes now sits at index 6 (after the version/preflight gate completions, the
	// blocking notes stage pair, RunStarted, and ShowPlan).
	notesEv, _ := rec.At(6)
	if notesEv.ShowNotes.Body != want {
		t.Errorf("ShowNotes.Body = %q, want degenerate StubBody %q", notesEv.ShowNotes.Body, want)
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("degenerate run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_PublishFailsAfterPush_WarnsOnly proves the PONR asymmetry on
// the prior-tag path: a publish failure AFTER a successful push is warn-only — the
// run finishes successfully and returns nil.
func TestRelease_PriorTag_PublishFailsAfterPush_WarnsOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	// gh auth status succeeds, but `gh release create` fails after the push.
	f.SeedSequence("gh",
		ScriptedOut(""), // gh auth status (authenticated)
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("gh: server error")},
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
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
	if !invokedWith(f, "git", "push", "--atomic", "origin", "HEAD", nextTag) {
		t.Errorf("atomic push did not run; PONR was never crossed")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish after warn-only publish failure; last event = %v", fin.Kind)
	}
}

// TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation proves an
// on_notes_failure=abort notes failure (the AI returns empty/invalid after retry)
// surfaces and aborts. With no pre_tag hook nothing was committed, so the surgical
// unwind it routes through has zero MadeState and no-ops (no reset, no Unwound): the
// run surfaces a StageFailed and exits non-zero with nothing tagged. (The case where a
// pre_tag artifact commit precedes the notes failure — so the surgical unwind resets it
// — is proven by TestRelease_NotesFailure_AfterArtifactCommit_SurgicalResets.)
func TestRelease_PriorTag_NotesFailureAbort_AbortsBeforeMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	// The AI returns empty on BOTH attempts -> ai.ErrGenerationFailed; default abort mode
	// then propagates the abort out of SelectBody.
	f.Seed("claude", runner.Result{Stdout: ""}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions())
	if err == nil {
		t.Fatalf("Release returned nil error, want a notes-failure abort")
	}
	var abort *engine.AbortError
	if !errors.As(err, &abort) {
		t.Fatalf("err is not an *engine.AbortError: %v", err)
	}
	if abort.ExitCode == 0 {
		t.Errorf("abort ExitCode = 0, want non-zero")
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("notes failure did not surface a StageFailed event")
	}
	// The notes review gate was never reached and nothing mutated. (The
	// version-confirmation gate fires first, ahead of preflight/notes, and is accepted
	// by default — but the notes failure stops the run before the notes review gate.)
	if notesGatePrompted(rec) {
		t.Errorf("notes review gate prompted despite a notes-failure abort")
	}
	assertNoMutation(t, f)
}
