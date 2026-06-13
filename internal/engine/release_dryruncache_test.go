package engine_test

// This file holds the DRY-RUN NOTE CACHE WRITE tests: under --dry-run, after the AI
// notes preview is generated, mint writes the generated body to the note cache (a
// user-level, per-project store — a t.TempDir() here) keyed by a hash of the
// post-diff_exclude diff + the computed version + the resolved prompt/context (NOT
// the HEAD sha), stamped with a TTL write time. The dry run reuses a live entry
// SILENTLY (it asks for nothing); the real run prompts use/regenerate.
//
// Tests drive the REAL notes path over the FakeRunner (scripting the diff + the
// `claude` AI command), recording the preview via the RecordingPresenter, and
// point the cache at a TEMP dir so nothing is written into the real repo.

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/notescache"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// cacheWriteClock is the deterministic write time the cache-write tests inject so
// the TTL stamp recorded with each entry is exactly assertable.
var cacheWriteClock = time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)

// newDepsWithCache builds the dependency set with an injected temp-dir note cache
// store so the dry-run cache write lands in cacheBase, never the real repo.
func newDepsWithCache(rec *presentertest.RecordingPresenter, f *runner.FakeRunner, cacheBase string) engine.ReleaseDeps {
	deps := newDeps(rec, f)
	deps.NoteCache = notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })
	return deps
}

// runDryRunNormalAI drives a prior-tag NORMAL-AI dry run with the given AI body
// and diff over a fresh FakeRunner+temp cache, returning the recorder, runner, and
// the cache base for assertions. It scripts the read gates, the notes assembly
// (with the supplied diff), the read-only provider detection, and the `claude` AI
// command. The caller supplies the repo root so the cache is repo-scoped to it.
func runDryRunNormalAI(t *testing.T, root, cacheBase, diff, aiBody string, opts engine.ReleaseOptions) (*presentertest.RecordingPresenter, *runner.FakeRunner) {
	t.Helper()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(diff),                     // diff priorTag..HEAD (degenerate-check assemble)
		ScriptedOut("A\tauth/login.go\n"),     // diff --name-status (change map)
		ScriptedOut("20\t0\tauth/login.go\n"), // diff --numstat (change map)
	)
	f.SeedSequence("git", ScriptedOut(githubRemoteURL)) // remote get-url origin (provider detection)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDepsWithCache(rec, f, cacheBase), opts); err != nil {
		t.Fatalf("dry-run Release returned unexpected error: %v", err)
	}
	return rec, f
}

// cachedKey computes the expected cache key for a normal-AI dry run with the given
// post-exclusion diff and bare version, using the DEFAULT prompt (no override /
// context configured in these tests).
func cachedKey(diff, version string) string {
	return notescache.Key(diff, version, notes.DefaultPrompt)
}

// readCacheEntry decodes the on-disk cache entry at the store's path for
// (root, key), failing the test if it is missing or malformed.
func readCacheEntry(t *testing.T, cacheBase, root, key string) notescache.Entry {
	t.Helper()
	store := notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })
	data, err := os.ReadFile(store.EntryPath(root, key))
	if err != nil {
		t.Fatalf("reading cache entry for key %q: %v", key, err)
	}
	var entry notescache.Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("decoding cache entry: %v", err)
	}
	return entry
}

// TestRelease_DryRun_WritesNoteCacheEntry proves a --dry-run writes the generated
// AI note body to a cache entry under the computed key, stamped with the TTL write
// time. The cache lives in the injected temp dir, never the repo.
func TestRelease_DryRun_WritesNoteCacheEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cacheBase := t.TempDir()
	rec, _ := runDryRunNormalAI(t, root, cacheBase, priorTagDiff, aiBody, dryRunOptions())

	// The preview was generated (sanity: this is the body we expect cached).
	if got := lastNotesBody(t, rec); got != aiBody {
		t.Fatalf("dry-run notes preview = %q, want AI body %q", got, aiBody)
	}

	key := cachedKey(priorTagDiff, "1.2.4")
	entry := readCacheEntry(t, cacheBase, root, key)
	if entry.Body != aiBody {
		t.Errorf("cached body = %q, want the generated AI body %q", entry.Body, aiBody)
	}
	// A TTL stamp (write time) is recorded with the entry.
	if !entry.WrittenAt.Equal(cacheWriteClock) {
		t.Errorf("cached WrittenAt = %v, want the injected write time %v", entry.WrittenAt, cacheWriteClock)
	}
}

// TestRelease_DryRun_CacheKeyChangesWithDiff proves the cache key changes when the
// post-diff_exclude diff changes: two dry runs whose ONLY difference is the
// filtered diff write to DIFFERENT keys.
func TestRelease_DryRun_CacheKeyChangesWithDiff(t *testing.T) {
	t.Parallel()

	const diffA = "diff --git a/a.go b/a.go\n@@ -0,0 +1 @@\n+package a\n"
	const diffB = "diff --git a/b.go b/b.go\n@@ -0,0 +1 @@\n+package b\n"

	rootA := t.TempDir()
	cacheA := t.TempDir()
	runDryRunNormalAI(t, rootA, cacheA, diffA, aiBody, dryRunOptions())

	rootB := t.TempDir()
	cacheB := t.TempDir()
	runDryRunNormalAI(t, rootB, cacheB, diffB, aiBody, dryRunOptions())

	keyA := cachedKey(diffA, "1.2.4")
	keyB := cachedKey(diffB, "1.2.4")
	if keyA == keyB {
		t.Fatalf("differing diffs produced the same cache key %q", keyA)
	}
	// Each run wrote under its own diff-derived key (and not the other's).
	readCacheEntry(t, cacheA, rootA, keyA)
	readCacheEntry(t, cacheB, rootB, keyB)
	if _, err := os.Stat(notescache.NewStore(cacheA, func() time.Time { return cacheWriteClock }).EntryPath(rootA, keyB)); !os.IsNotExist(err) {
		t.Errorf("run A wrote under run B's diff key; the diff did not change the key")
	}
}

// TestRelease_DryRun_CacheKeyChangesWithVersion proves the cache key changes when
// the computed version changes: a patch bump and a minor bump over the SAME diff
// write to DIFFERENT keys.
func TestRelease_DryRun_CacheKeyChangesWithVersion(t *testing.T) {
	t.Parallel()

	patchOpts := dryRunOptions() // v1.2.3 -> 1.2.4
	minorOpts := engine.ReleaseOptions{Bump: version.BumpMinor, Now: fixedClock, DryRun: true}

	rootA := t.TempDir()
	cacheA := t.TempDir()
	runDryRunNormalAI(t, rootA, cacheA, priorTagDiff, aiBody, patchOpts)

	rootB := t.TempDir()
	cacheB := t.TempDir()
	runDryRunNormalAI(t, rootB, cacheB, priorTagDiff, aiBody, minorOpts)

	keyPatch := cachedKey(priorTagDiff, "1.2.4")
	keyMinor := cachedKey(priorTagDiff, "1.3.0")
	if keyPatch == keyMinor {
		t.Fatalf("differing versions produced the same cache key %q", keyPatch)
	}
	readCacheEntry(t, cacheA, rootA, keyPatch)
	readCacheEntry(t, cacheB, rootB, keyMinor)
}

// TestRelease_DryRun_CacheKeyChangesWithPromptContext proves the cache key changes
// when the resolved prompt/context changes: a run with an injected
// [release].context writes to a DIFFERENT key than the default-prompt run over the
// same diff and version.
func TestRelease_DryRun_CacheKeyChangesWithPromptContext(t *testing.T) {
	t.Parallel()

	// Default-prompt run.
	rootA := t.TempDir()
	cacheA := t.TempDir()
	runDryRunNormalAI(t, rootA, cacheA, priorTagDiff, aiBody, dryRunOptions())
	keyDefault := cachedKey(priorTagDiff, "1.2.4")
	readCacheEntry(t, cacheA, rootA, keyDefault)

	// Context-injected run: the resolved instructions differ, so the key must differ.
	rootB := t.TempDir()
	const contextLine = "Lead with the auth package."
	writeConfig(t, rootB, "[release]\ncontext = \""+contextLine+"\"\n")
	cacheB := t.TempDir()
	runDryRunNormalAI(t, rootB, cacheB, priorTagDiff, aiBody, dryRunOptions())

	// The default-prompt key must NOT exist in run B's cache — the context changed it.
	storeB := notescache.NewStore(cacheB, func() time.Time { return cacheWriteClock })
	if _, err := os.Stat(storeB.EntryPath(rootB, keyDefault)); !os.IsNotExist(err) {
		t.Errorf("context-injected run wrote under the default-prompt key; the context did not change the key")
	}
	// Exactly one entry was written in run B, under a context-derived (non-default) key.
	assertExactlyOneCacheEntryNotKey(t, storeB, rootB, keyDefault)
}

// TestRelease_DryRun_CacheKeyInvariantToHEADSha proves the cache key is INVARIANT
// to a HEAD sha change that does NOT change the filtered diff: two dry runs with
// DIFFERENT starting HEAD shas but the SAME post-exclusion diff (and same version
// and prompt) write to the SAME key — the key is the diff, not the sha.
func TestRelease_DryRun_CacheKeyInvariantToHEADSha(t *testing.T) {
	t.Parallel()

	// Run A and run B share a repo root (so the repo-scope namespace matches) and a
	// single cache base, but their pre-gate `git rev-parse HEAD` returns different
	// shas. Because the key omits the sha, both must land on the same entry.
	root := t.TempDir()
	cacheBase := t.TempDir()

	runDryRunNormalAIWithSha(t, root, cacheBase, priorTagDiff, aiBody, "sha-one")
	runDryRunNormalAIWithSha(t, root, cacheBase, priorTagDiff, aiBody, "sha-two")

	key := cachedKey(priorTagDiff, "1.2.4")
	// Exactly one entry exists (both runs collapsed onto the sha-invariant key).
	store := notescache.NewStore(cacheBase, func() time.Time { return cacheWriteClock })
	assertExactlyOneCacheEntryWithKey(t, store, root, key)
}

// runDryRunNormalAIWithSha is runDryRunNormalAI with a custom pre-gate HEAD sha so
// the HEAD-invariance test can vary the sha while holding the diff fixed.
func runDryRunNormalAIWithSha(t *testing.T, root, cacheBase, diff, aiBody, headSha string) {
	t.Helper()
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),                     // rev-parse --show-toplevel
		ScriptedOut("origin/main"),            // symbolic-ref --short origin/HEAD
		ScriptedOut(priorTag+"\n"),            // tag --list (a prior tag exists)
		ScriptedOut(""),                       // fetch --tags
		ScriptedOut(""),                       // status --porcelain (clean)
		ScriptedOut("main"),                   // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),                     // rev-parse -q --verify refs/tags/v1.2.4 (absent)
		ScriptedOut("0\t1"),                   // rev-list left-right count (ahead only)
		ScriptedOut(""),                       // ls-remote --tags (tag free remote)
		ScriptedOut(headSha),                  // rev-parse HEAD (capture clean start — VARIES)
		ScriptedOut(diff),                     // diff priorTag..HEAD (degenerate-check assemble)
		ScriptedOut("A\tauth/login.go\n"),     // diff --name-status (change map)
		ScriptedOut("20\t0\tauth/login.go\n"), // diff --numstat (change map)
		ScriptedOut(githubRemoteURL),          // remote get-url origin (provider detection)
	)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDepsWithCache(rec, f, cacheBase), dryRunOptions()); err != nil {
		t.Fatalf("dry-run Release (sha %q) returned unexpected error: %v", headSha, err)
	}
}

// TestRelease_DryRun_CacheStillSkipsHooks re-confirms the 4-7a/3-11 behaviour STILL
// holds with caching wired: a configured hook is skipped (no `sh` runs) and the
// skip is reported, AND the cache write is the SOLE filesystem side effect (no
// mutation reaches the wrapper).
func TestRelease_DryRun_CacheStillSkipsHooks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npre_tag = \"build.sh\"\npost_release = \"notify.sh\"\n")
	cacheBase := t.TempDir()

	rec, f := runDryRunNormalAI(t, root, cacheBase, priorTagDiff, aiBody, dryRunOptions())

	if shInvoked(f) {
		t.Errorf("a hook ran under the dry-run cache write; no `sh -c …` must reach the runner: %v", commandLines(f.Invocations()))
	}
	warnWithMessage(t, rec, "skipping pre_tag hook")
	warnWithMessage(t, rec, "skipping post_release hook")
	assertNoMutation(t, f)
	// The cache entry was still written (the sole side effect).
	readCacheEntry(t, cacheBase, root, cachedKey(priorTagDiff, "1.2.4"))
}

// assertExactlyOneCacheEntryNotKey fails the test unless exactly one .json cache
// entry exists for root AND its key is NOT notKey (proving a context/prompt change
// produced a single, different key).
func assertExactlyOneCacheEntryNotKey(t *testing.T, store *notescache.Store, root, notKey string) {
	t.Helper()
	keys := cacheEntryKeys(t, store, root)
	if len(keys) != 1 {
		t.Fatalf("cache entries = %d, want exactly 1; keys = %v", len(keys), keys)
	}
	if keys[0] == notKey {
		t.Errorf("the single cache entry key %q matches the excluded key", notKey)
	}
}

// assertExactlyOneCacheEntryWithKey fails the test unless exactly one .json cache
// entry exists for root AND its key equals wantKey.
func assertExactlyOneCacheEntryWithKey(t *testing.T, store *notescache.Store, root, wantKey string) {
	t.Helper()
	keys := cacheEntryKeys(t, store, root)
	if len(keys) != 1 {
		t.Fatalf("cache entries = %d, want exactly 1 (HEAD-sha invariant); keys = %v", len(keys), keys)
	}
	if keys[0] != wantKey {
		t.Errorf("cache entry key = %q, want the sha-invariant key %q", keys[0], wantKey)
	}
}

// cacheEntryKeys lists the cache entry keys (filenames minus .json) under the
// store's cache dir for root.
func cacheEntryKeys(t *testing.T, store *notescache.Store, root string) []string {
	t.Helper()
	// EntryPath for an empty key resolves the cache dir + "/.json"; trim to the dir.
	probe := store.EntryPath(root, "")
	dir := probe[:len(probe)-len(".json")]
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading cache dir %q: %v", dir, err)
	}
	var keys []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(".json") && name[len(name)-len(".json"):] == ".json" {
			keys = append(keys, name[:len(name)-len(".json")])
		}
	}
	return keys
}
