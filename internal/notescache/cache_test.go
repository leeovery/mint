package notescache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mint/internal/notescache"
)

// fixedClock is the deterministic write time the store tests inject so the TTL
// stamp recorded with each entry is exactly assertable.
var fixedClock = time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

// keyInputs is a representative set of the three cache-key components: the
// post-diff_exclude diff fed to the AI, the computed next version, and the
// resolved prompt/context instructions.
func keyInputs() (diff, version, instructions string) {
	return "diff --git a/auth.go b/auth.go\n+package auth\n", "1.2.4", "DEFAULT PROMPT + context"
}

// TestKey_Deterministic proves the key is a stable function of its three inputs:
// the same (diff, version, instructions) always hashes to the same key.
func TestKey_Deterministic(t *testing.T) {
	t.Parallel()

	diff, version, instructions := keyInputs()
	first := notescache.Key(diff, version, instructions)
	second := notescache.Key(diff, version, instructions)
	if first != second {
		t.Errorf("Key is not deterministic: %q != %q", first, second)
	}
	if first == "" {
		t.Errorf("Key returned empty string")
	}
}

// TestKey_ChangesWithEachInput proves the key changes when ANY of the three
// inputs changes — the post-diff_exclude diff, the computed version, or the
// resolved prompt/context. Each variant must differ from the baseline AND from
// every other variant (so no two distinct inputs collide on the same key).
func TestKey_ChangesWithEachInput(t *testing.T) {
	t.Parallel()

	diff, version, instructions := keyInputs()
	base := notescache.Key(diff, version, instructions)

	tests := []struct {
		name string
		key  string
	}{
		{name: "diff changed", key: notescache.Key(diff+"\n+more", version, instructions)},
		{name: "version changed", key: notescache.Key(diff, "1.3.0", instructions)},
		{name: "instructions changed", key: notescache.Key(diff, version, instructions+" extra context")},
	}

	seen := map[string]string{base: "baseline"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key == base {
				t.Errorf("key did not change when %s", tt.name)
			}
			if prior, ok := seen[tt.key]; ok {
				t.Errorf("key for %q collides with %q", tt.name, prior)
			}
			seen[tt.key] = tt.name
		})
	}
}

// TestKey_InvariantToFieldBoundaries proves the canonical concatenation is
// unambiguous: shifting bytes across the field boundary produces a DIFFERENT key
// (no naive concatenation collision where "ab"+"c" and "a"+"bc" hash alike).
func TestKey_InvariantToFieldBoundaries(t *testing.T) {
	t.Parallel()

	a := notescache.Key("ab", "c", "d")
	b := notescache.Key("a", "bc", "d")
	if a == b {
		t.Errorf("ambiguous concatenation: distinct field splits hashed to the same key %q", a)
	}
}

// TestStore_Write_PersistsBodyAndTTLStamp proves a write stores BOTH the note
// body and a TTL timestamp (the write time) under the computed key, recoverable
// from the on-disk entry. The write time is the injected clock so 4-8 can later
// enforce the ~1h TTL deterministically.
func TestStore_Write_PersistsBodyAndTTLStamp(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	repoRoot := t.TempDir()
	store := notescache.NewStore(cacheRoot, func() time.Time { return fixedClock })

	diff, version, instructions := keyInputs()
	key := notescache.Key(diff, version, instructions)
	const body = "TL;DR: cached preview body.\n"

	if err := store.Write(repoRoot, key, body); err != nil {
		t.Fatalf("Write returned unexpected error: %v", err)
	}

	entry := readEntry(t, cacheRoot, repoRoot, key)
	if entry.Body != body {
		t.Errorf("entry Body = %q, want %q", entry.Body, body)
	}
	if !entry.WrittenAt.Equal(fixedClock) {
		t.Errorf("entry WrittenAt = %v, want the injected write time %v", entry.WrittenAt, fixedClock)
	}
}

// TestStore_Write_RepoScoped proves the cache is repo-scoped: two repos writing
// the SAME key land in DIFFERENT cache files, so previews never collide across
// repos.
func TestStore_Write_RepoScoped(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	store := notescache.NewStore(cacheRoot, func() time.Time { return fixedClock })

	repoA := t.TempDir()
	repoB := t.TempDir()
	diff, version, instructions := keyInputs()
	key := notescache.Key(diff, version, instructions)

	if err := store.Write(repoA, key, "body A"); err != nil {
		t.Fatalf("Write repoA: %v", err)
	}
	if err := store.Write(repoB, key, "body B"); err != nil {
		t.Fatalf("Write repoB: %v", err)
	}

	pathA := store.EntryPath(repoA, key)
	pathB := store.EntryPath(repoB, key)
	if pathA == pathB {
		t.Fatalf("repo-scoped entry paths collided: %q", pathA)
	}
	if got := readEntry(t, cacheRoot, repoA, key); got.Body != "body A" {
		t.Errorf("repoA entry Body = %q, want %q", got.Body, "body A")
	}
	if got := readEntry(t, cacheRoot, repoB, key); got.Body != "body B" {
		t.Errorf("repoB entry Body = %q, want %q", got.Body, "body B")
	}
}

// TestStore_Write_GitignoresCacheDir proves the cache directory under the repo is
// gitignored so it is NEVER committed: writing an entry ensures a .gitignore in
// the cache parent that ignores the cache contents.
func TestStore_Write_GitignoresCacheDir(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	// A repo-path store (cache lives under the repo, not system temp): the cache dir
	// MUST be gitignored.
	store := notescache.NewRepoStore(func() time.Time { return fixedClock })

	diff, version, instructions := keyInputs()
	key := notescache.Key(diff, version, instructions)
	if err := store.Write(repoRoot, key, "body"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The .mint cache base must carry a .gitignore that ignores everything beneath it.
	ignorePath := filepath.Join(repoRoot, ".mint", ".gitignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("reading cache .gitignore at %q: %v", ignorePath, err)
	}
	if strings.TrimSpace(string(data)) != "*" {
		t.Errorf("cache .gitignore = %q, want %q (ignore the whole cache dir)", string(data), "*")
	}
	// The entry must live UNDER the gitignored .mint dir.
	entryPath := store.EntryPath(repoRoot, key)
	mintDir := filepath.Join(repoRoot, ".mint")
	if !strings.HasPrefix(entryPath, mintDir+string(os.PathSeparator)) {
		t.Errorf("entry path %q is not under the gitignored cache dir %q", entryPath, mintDir)
	}
}

// readEntry decodes the on-disk cache entry for (repoRoot, key) under cacheRoot,
// failing the test if it is missing or malformed.
func readEntry(t *testing.T, cacheRoot, repoRoot, key string) notescache.Entry {
	t.Helper()
	store := notescache.NewStore(cacheRoot, func() time.Time { return fixedClock })
	data, err := os.ReadFile(store.EntryPath(repoRoot, key))
	if err != nil {
		t.Fatalf("reading cache entry: %v", err)
	}
	var entry notescache.Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("decoding cache entry: %v", err)
	}
	return entry
}
