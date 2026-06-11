package engine_test

// This file holds the Phase 4 REAL-RUN NOTE-CACHE REUSE tests (task 4-8): the read
// side of the dry-run cache. On the REAL run (not --dry-run), BEFORE invoking the AI,
// mint recomputes the cache key (the SAME post-diff_exclude diff + computed version +
// resolved prompt/context hash the dry run wrote under) and looks it up:
//
//   - a live key MATCH within the TTL reuses the cached body and SKIPS the AI call;
//   - a key MISS regenerates and reports "diff changed since dry-run preview —
//     regenerating notes" (the stale note is never shipped);
//   - an EXPIRED entry (older than ~1h) regenerates regardless of a key match.
//
// Reuse is automatic (no flag). The reused-or-regenerated body flows through the
// notes-review gate identically. Tests pre-seed the cache with a 4-7 Write, drive the
// REAL notes path over the FakeRunner, and record the gate via the RecordingPresenter.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// realRunClock is the deterministic real-run clock the reuse tests inject so the
// TTL window is measured against an exactly-controlled "now". The dry-run write
// stamps cacheWriteClock; this clock advances from it by a test-chosen delta.
var realRunClock = cacheWriteClock.Add(10 * time.Minute)

// cachedReuseBody is the distinctive body a prior --dry-run is presumed to have
// previewed and written to the cache, so a reuse test can prove THIS body — not a
// freshly generated one — flows to the sinks and the gate.
const cachedReuseBody = "TL;DR: PREVIEWED note reused verbatim from the dry-run cache.\n"

// newDepsWithRealRunCache builds the dependency set with a note cache whose clock is
// the real-run clock, so the reuse-side TTL check measures the entry's age against a
// controlled "now" while the entry itself was stamped by the dry-run write clock.
func newDepsWithRealRunCache(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, cacheBase string, now func() time.Time) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.NoteCache = notescache.NewStore(cacheBase, now)
	return deps
}

// seedCacheEntry writes a cache entry exactly as a prior --dry-run (4-7) would: under
// the canonical key (post-exclusion diff + bare version + DEFAULT prompt) stamped with
// the dry-run write clock. It is the PRE-SEED a reuse test relies on.
func seedCacheEntry(t *testing.T, cacheBase, root, diff, version, body string) {
	t.Helper()
	store := notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })
	if err := store.Write(root, cachedKey(diff, version), body); err != nil {
		t.Fatalf("pre-seeding cache entry: %v", err)
	}
}

// seedRealRunReuseGit scripts the REAL-run git timeline for a prior-tag run that
// REUSES a cached note: the read gates, the single AssembleDiff degenerate-check diff
// (the ONLY notes git call on the reuse path — no Change Map name-status/numstat, since
// the AI is skipped), then the record/tag/push mutation tail. `claude` is deliberately
// left UNSEEDED so any AI call would error — proving reuse skipped it.
func seedRealRunReuseGit(f *runner.FakeRunner, root, diff string) {
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut(diff)) // diff priorTag..HEAD (degenerate-check assemble)
	seedRecordTagPush(f, root)
}

// claudeInvoked reports whether the scripted `claude` AI command was ever called —
// the load-bearing reuse assertion (reuse must NOT invoke the AI).
func claudeInvoked(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			return true
		}
	}
	return false
}

// TestRelease_RealRun_KeyMatch_ReusesCachedNote_NoAICall proves the load-bearing
// reuse: with a fresh cache entry under the run's key, the real run uses the CACHED
// body and NEVER calls the AI. `claude` is left unseeded, so any AI call would have
// failed the run; the clean success + the cached body reaching the tag annotation
// proves reuse skipped the AI.
func TestRelease_RealRun_KeyMatch_ReusesCachedNote_NoAICall(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The AI was NEVER invoked — reuse skipped it.
	if claudeInvoked(f) {
		t.Errorf("the AI was invoked on a key match; reuse must skip the AI call")
	}
	// The CACHED body — not a freshly generated one — flowed to the tag annotation.
	if got := tagAnnotationBody(t, f, nextTag); got != cachedReuseBody {
		t.Errorf("tag annotation body = %q, want the reused cached body %q", got, cachedReuseBody)
	}
	if got := changelogSectionBody(t, root, "1.2.4"); got != cachedReuseBody {
		t.Errorf("CHANGELOG body = %q, want the reused cached body %q", got, cachedReuseBody)
	}
	// The reuse is reported quietly.
	warnWithMessage(t, rec, reuseReportMessage)
}

// reuseReportMessage is the quiet notice mint emits when it reuses a previewed note.
const reuseReportMessage = "reusing the previewed notes from the dry-run cache"

// TestRelease_RealRun_KeyMiss_RegeneratesAndReports proves a key MISS (no entry for
// the run's key) regenerates via the AI and reports the EXACT spec message; the run
// proceeds with the FRESH body, never a stale cached one.
func TestRelease_RealRun_KeyMiss_RegeneratesAndReports(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	// Seed an entry under a DIFFERENT diff so the run's key misses (a stale preview
	// from a prior tree state that no longer matches what would ship).
	const staleDiff = "diff --git a/old.go b/old.go\n@@ -0,0 +1 @@\n+package old\n"
	const staleBody = "STALE: this preview no longer matches the release.\n"
	seedCacheEntry(t, cacheBase, root, staleDiff, "1.2.4", staleBody)

	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f) // a miss runs the full AI path: assemble + change map
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The AI WAS invoked — the miss regenerated.
	if !claudeInvoked(f) {
		t.Errorf("the AI was not invoked on a key miss; a miss must regenerate")
	}
	// The exact spec miss report fired.
	warnWithMessage(t, rec, missReportMessage)
	// The FRESH body shipped — the stale cached note was NEVER shipped.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the freshly generated body %q", got, aiBody)
	}
	if got := tagAnnotationBody(t, f, nextTag); got == staleBody {
		t.Errorf("the stale cached body was shipped on a key miss")
	}
}

// missReportMessage is the EXACT spec wording for a key miss on the real run.
const missReportMessage = "diff changed since dry-run preview — regenerating notes"

// corruptReadReportMessage is the DISTINCT warn emitted when the cache entry under the
// run's key exists but cannot be read/decoded (a corrupt or partial file). It must be
// separate from missReportMessage — a corrupt read is a different situation from a
// clean key miss, and reusing the diff-changed wording would be misleading.
const corruptReadReportMessage = "could not read cached notes preview; regenerating"

// seedCorruptCacheEntry pre-writes an UNDECODABLE entry at the run key's path, exactly
// where a 4-7 dry-run write would have landed — modelling a partial file from a killed
// process or a glitched write. The directory is created first so Lookup READS the
// corrupt bytes (rather than missing the file) and surfaces a decode error.
func seedCorruptCacheEntry(t *testing.T, cacheBase, root, diff, version string) {
	t.Helper()
	store := notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })
	entryPath := store.EntryPath(root, cachedKey(diff, version))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing corrupt cache entry: %v", err)
	}
}

// TestRelease_RealRun_CorruptCacheEntry_RegeneratesAndDoesNotAbort proves the
// read-error degrade (refinement to 4-8): a CORRUPT cache entry under the run's key
// must NOT abort the release. Like the warn-only WRITE side, a read failure degrades to
// regeneration — the run warns with a DISTINCT message, regenerates via the AI, ships
// the FRESH body, and reaches RunFinished. A corrupt preview is never shipped.
func TestRelease_RealRun_CorruptCacheEntry_RegeneratesAndDoesNotAbort(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	// The entry under the EXACT run key is corrupt (undecodable), as a killed dry-run
	// write might leave it.
	seedCorruptCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4")

	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f) // a corrupt read regenerates: the full AI path runs
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	// (a) the release does NOT abort — it returns nil and reaches RunFinished.
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release aborted on a corrupt cache entry: %v; a read error must degrade to regeneration", err)
	}
	if got := countKind(rec, presentertest.KindRunFinished); got != 1 {
		t.Errorf("RunFinished count = %d, want 1; a corrupt read must not abort the run", got)
	}

	// (b) the AI WAS invoked — the corrupt read regenerated.
	if !claudeInvoked(f) {
		t.Errorf("the AI was not invoked on a corrupt cache entry; a read error must regenerate")
	}

	// (c) the FRESH body shipped — the corrupt/stale entry was never reused.
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the freshly generated body %q (corrupt entry not reused)", got, aiBody)
	}

	// (d) the DISTINCT read-error warning fired — NOT the diff-changed miss message.
	warnWithMessage(t, rec, corruptReadReportMessage)
	if hasWarnMessage(rec, missReportMessage) {
		t.Errorf("the diff-changed miss message fired on a corrupt read; a read error must use its own distinct warning")
	}
}

// hasWarnMessage reports whether any recorded Warn carried the exact message — the
// negative counterpart to warnWithMessage, used to prove a particular notice did NOT
// fire (here: a corrupt read must not reuse the diff-changed miss wording).
func hasWarnMessage(rec *presentertest.RecordingPresenter, message string) bool {
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindWarn && ev.Warn.Message == message {
			return true
		}
	}
	return false
}

// TestRelease_RealRun_ExpiredTTL_Regenerates proves an entry under the run's key but
// older than the ~1h TTL is treated as ABSENT: the real run regenerates via the AI
// even though the key matches, and reports the miss.
func TestRelease_RealRun_ExpiredTTL_Regenerates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	// The entry is under the EXACT run key but stamped by the dry-run clock.
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f) // an expired entry regenerates: full AI path
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// The real-run clock is PAST the TTL window after the write, so the entry expired.
	expiredNow := func() time.Time { return cacheWriteClock.Add(notescache.TTL + time.Minute) }
	deps := newDepsWithRealRunCache(rec, f, cacheBase, expiredNow)
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !claudeInvoked(f) {
		t.Errorf("the AI was not invoked despite an EXPIRED entry; an expired TTL must regenerate")
	}
	warnWithMessage(t, rec, missReportMessage)
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the regenerated body %q (expired entry not reused)", got, aiBody)
	}
}

// TestRelease_RealRun_ReuseIsAutomatic_NoFlag proves reuse activates with NO flag:
// the plain default-bump options (no special flag) reuse the cached preview.
func TestRelease_RealRun_ReuseIsAutomatic_NoFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	// priorTagNormalAIOptions carries NO reuse flag — only the bump + clock.
	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if claudeInvoked(f) {
		t.Errorf("the AI was invoked; reuse must activate automatically with no flag")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != cachedReuseBody {
		t.Errorf("tag annotation body = %q, want the reused cached body %q", got, cachedReuseBody)
	}
}

// TestRelease_RealRun_ExcludedHookArtifact_StillReuses proves the hook-interaction
// correctness that FALLS OUT of the post-diff_exclude key: a pre_tag hook that changes
// ONLY an excluded artifact does NOT change the post-exclusion diff, so the dry-run
// (hook-skipped) preview and the real (post-hook) run share the SAME key → reuse holds.
// The cache is keyed by the post-exclusion diff, so the hook running (and committing an
// artifact) leaves the key unchanged and the cached note is reused.
func TestRelease_RealRun_ExcludedHookArtifact_StillReuses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A pre_tag hook is configured AND an excluded glob covers its artifact, so the
	// hook's output never enters the post-exclusion diff.
	writeConfig(t, root, "diff_exclude = [\"dist/**\"]\n[release.hooks]\npre_tag = \"build.sh\"\n")
	cacheBase := t.TempDir()
	// The post-exclusion diff the dry run previewed and the real run computes are the
	// SAME (the hook touched only excluded dist/**), so the cache key matches.
	excludedDiff := priorTagDiff
	seedCacheEntry(t, cacheBase, root, excludedDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	// The pre_tag hook runs (real run) and dirties an excluded path; mint commits it.
	f.Seed("sh", runner.Result{}, nil)                        // pre_tag hook
	f.SeedSequence("git", ScriptedOut(" M dist/bundle.js\n")) // status --porcelain (dirty)
	f.SeedSequence("git", ScriptedOut(""), ScriptedOut(""))   // add -A + commit (artifact)
	f.SeedSequence("git", ScriptedOut(excludedDiff))          // AssembleDiff (post-exclusion, unchanged)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if claudeInvoked(f) {
		t.Errorf("the AI was invoked despite an excluded-only hook artifact; the post-exclusion key should match and reuse")
	}
	if got := tagAnnotationBody(t, f, nextTag); got != cachedReuseBody {
		t.Errorf("tag annotation body = %q, want the reused cached body %q", got, cachedReuseBody)
	}
}

// TestRelease_RealRun_NonExcludedHookChange_Misses proves the inverse hook
// correctness: a pre_tag hook that changes a NON-excluded (real source) path makes the
// real post-hook diff DIFFER from the dry-run (hook-skipped) preview, so the key
// correctly MISSES and the real run regenerates. The dry-run cached note (under the
// pre-hook diff) is never reused.
func TestRelease_RealRun_NonExcludedHookChange_Misses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\n")
	cacheBase := t.TempDir()
	// The dry run previewed (and cached) under the PRE-hook diff.
	preHookDiff := priorTagDiff
	seedCacheEntry(t, cacheBase, root, preHookDiff, "1.2.4", cachedReuseBody)

	// The real run's hook changed a non-excluded source path, so the post-hook diff
	// differs — the run's key is computed over THIS diff and misses the cached entry.
	const postHookDiff = "diff --git a/src/gen.go b/src/gen.go\n@@ -0,0 +1 @@\n+package gen\n"

	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.Seed("sh", runner.Result{}, nil)                      // pre_tag hook
	f.SeedSequence("git", ScriptedOut(" M src/gen.go\n"))   // status --porcelain (dirty)
	f.SeedSequence("git", ScriptedOut(""), ScriptedOut("")) // add -A + commit (artifact)
	f.SeedSequence("git",
		ScriptedOut(postHookDiff),         // AssembleDiff (post-hook, CHANGED)
		ScriptedOut("A\tsrc/gen.go\n"),    // diff --name-status (change map, on the miss)
		ScriptedOut("5\t0\tsrc/gen.go\n"), // diff --numstat (change map, on the miss)
	)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !claudeInvoked(f) {
		t.Errorf("the AI was not invoked despite a non-excluded hook change; the key should miss and regenerate")
	}
	warnWithMessage(t, rec, missReportMessage)
	if got := tagAnnotationBody(t, f, nextTag); got != aiBody {
		t.Errorf("tag annotation body = %q, want the regenerated body %q (cached note not reused on a miss)", got, aiBody)
	}
}

// TestRelease_RealRun_ReusedNote_ShownAtGate proves gate orthogonality: an INTERACTIVE
// real run (no -y, modelled by scripting an explicit accept) still SHOWS the reused
// note at the notes-review gate — the cached body flows through ShowNotes + Prompt
// exactly as a freshly generated one would.
func TestRelease_RealRun_ReusedNote_ShownAtGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	// An interactive run that explicitly accepts at the gate.
	rec := &presentertest.RecordingPresenter{NextChoices: []presenter.Choice{presenter.ChoiceYes}}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The gate fired (Prompt recorded) over the REUSED note.
	if got := countKind(rec, presentertest.KindPrompt); got != 1 {
		t.Errorf("Prompt count = %d, want exactly 1 (the gate still runs over a reused note)", got)
	}
	// The note shown at the gate is the REUSED cached body.
	if got := lastNotesBody(t, rec); got != cachedReuseBody {
		t.Errorf("ShowNotes body = %q, want the reused cached body %q", got, cachedReuseBody)
	}
	// It is the four-choice KindNormalAI gate — reuse preserves the AI Kind.
	gate := promptGate(t, rec)
	if !gate.Has(presenter.ChoiceRegen) {
		t.Errorf("the reused-note gate omitted r; a reused AI note keeps KindNormalAI's four-choice gate")
	}
}

// TestRelease_RealRun_ReuseUnderYes_SkipsGate proves -y still skips the gate on the
// REAL run with a reused note: with no scripted choices (the recorder returns the gate
// Default, modelling -y), the gate auto-accepts and the run proceeds to the sinks with
// the reused body. The gate stays orthogonal to reuse.
func TestRelease_RealRun_ReuseUnderYes_SkipsGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	seedCacheEntry(t, cacheBase, root, priorTagDiff, "1.2.4", cachedReuseBody)

	f := runner.NewFakeRunner()
	seedRealRunReuseGit(f, root, priorTagDiff)
	f.Seed("gh", runner.Result{}, nil)
	// No NextChoices: the recorder returns the gate Default (yes), modelling -y.
	rec := &presentertest.RecordingPresenter{}

	deps := newDepsWithRealRunCache(rec, f, cacheBase, func() time.Time { return realRunClock })
	if err := engine.Release(t.Context(), deps, priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := countKind(rec, presentertest.KindPrompt); got != 1 {
		t.Errorf("Prompt count = %d, want exactly 1 under -y (the engine always calls Prompt; the presenter auto-accepts)", got)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != cachedReuseBody {
		t.Errorf("tag annotation body = %q, want the reused cached body %q under -y", got, cachedReuseBody)
	}
}

// TestRelease_DryRunUnderYes_SkipsGate proves -y skips the gate on the DRY run too:
// the dry run still calls Prompt once (auto-accepted by the presenter default) and
// finishes, writing the preview to the cache. This pins the "-y skips on BOTH runs"
// acceptance with the matching dry-run half.
func TestRelease_DryRunUnderYes_SkipsGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	// No NextChoices: the recorder returns the gate Default (yes), modelling -y.
	rec, f := runDryRunNormalAI(t, root, cacheBase, priorTagDiff, aiBody, dryRunOptions())
	_ = f

	if got := countKind(rec, presentertest.KindPrompt); got != 1 {
		t.Errorf("dry-run Prompt count = %d, want exactly 1 under -y", got)
	}
	// The dry run finished and wrote the preview to the cache (the handoff to a real run).
	readCacheEntry(t, cacheBase, root, cachedKey(priorTagDiff, "1.2.4"))
}
