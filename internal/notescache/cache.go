// Package notescache is mint's dry-run note cache: the WRITE side that lets a
// `mint release --dry-run` persist the generated AI note, and the READ side
// (Lookup) that lets the subsequent real run reuse the EXACT bytes that were
// previewed.
//
// The cache KEY is a hash of (the post-diff_exclude diff fed to the AI + the
// computed next version + the resolved prompt/[release].context) — deliberately
// NOT the HEAD sha, because a pre_tag hook can move HEAD between the dry run and
// the real run without changing what would ship. Each entry stores the note body
// alongside a TTL stamp (the write time) so Lookup can enforce the ~1h default TTL
// (see TTL): an entry older than the window is treated as absent so a stale preview
// is never reused. The cache is REPO-SCOPED (entries are namespaced by the resolved
// repo root, so previews never collide across repos) and lives under the user's cache
// directory (os.UserCacheDir) in a per-project sub-directory — so no project is ever
// polluted with an in-repo cache dir.
package notescache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mint/internal/fsutil"
)

// entryFileExt is the on-disk extension for a cache entry's JSON document.
const entryFileExt = ".json"

// TTL is the default freshness window a cached dry-run note may be reused within.
// It is the dry-run→real-run handoff backstop: long enough for an agent to surface
// the preview to a human and then run for real, short enough that a forgotten
// preview cannot resurrect days later. Lookup measures an entry's WrittenAt stamp
// against the store's injected clock and treats anything older than TTL as absent.
const TTL = time.Hour

// Entry is a single cached dry-run note: the generated body plus the TTL stamp
// (the write time). The stamp is recorded so the reuse side (task 4-8) can expire
// a stale preview against the ~1h default TTL; this package only WRITES it.
type Entry struct {
	// Body is the generated AI note body, stored verbatim so the real run reuses
	// the exact bytes that were previewed.
	Body string `json:"body"`
	// WrittenAt is the write time — the TTL stamp 4-8 measures the ~1h default TTL
	// against. It is the store's injected clock, never time.Now() directly, so the
	// stamp is deterministic in tests.
	WrittenAt time.Time `json:"written_at"`
}

// Key computes the cache key from the three canonical inputs: the
// post-diff_exclude diff (the filtered diff fed to the AI, NOT the HEAD sha), the
// computed next version, and the resolved prompt/context (the [release].prompt
// override contents OR the default prompt plus the injected [release].context).
//
// It hashes a LENGTH-PREFIXED concatenation of the three fields so the boundaries
// are unambiguous — shifting bytes across a field boundary (e.g. "ab"+"c" vs
// "a"+"bc") yields a different key, never a collision. The result is the hex
// SHA-256 digest, safe to use as a filename.
func Key(diff, version, instructions string) string {
	h := sha256.New()
	for _, field := range []string{diff, version, instructions} {
		// Length-prefix each field so the concatenation is injective: a change in
		// where one field ends and the next begins always changes the digest.
		h.Write([]byte(strconv.Itoa(len(field))))
		h.Write([]byte{0})
		h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Store reads and writes note cache entries under a base directory, namespaced into a
// per-project sub-directory and stamped with an injected clock. The base is the user
// cache dir in production and a t.TempDir() in tests, so entries never live inside a
// repo.
type Store struct {
	// base is the cache root the entries live under: production passes the user cache
	// dir ({UserCacheDir}/mint/cache); tests pass a t.TempDir(). Entries live under
	// {base}/<repo-scoped sub-dir>, so the cache is never inside any repo.
	base string
	// now supplies the TTL write stamp; injected so tests get a deterministic time.
	now func() time.Time
}

// NewStore builds a Store rooted at base (the user cache dir in production, a
// t.TempDir() in tests): entries are namespaced by a per-project sub-directory so
// caches never collide across repos and never live inside a repo. now supplies the
// TTL write stamp.
func NewStore(base string, now func() time.Time) *Store {
	return &Store{base: base, now: now}
}

// Write persists body under key for repoRoot, stamping the entry with the store's
// clock. It creates the repo-scoped cache directory (and, in repo-path mode, the
// .gitignore guard) as needed, then writes the JSON entry atomically.
func (s *Store) Write(repoRoot, key, body string) error {
	dir := s.cacheDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating note cache dir %q: %w", dir, err)
	}

	entry := Entry{Body: body, WrittenAt: s.now()}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encoding note cache entry: %w", err)
	}
	if err := writeFileAtomic(s.EntryPath(repoRoot, key), data); err != nil {
		return fmt.Errorf("writing note cache entry: %w", err)
	}
	return nil
}

// Lookup reads the cache entry for (repoRoot, key) and reports whether a FRESH one
// exists — the reuse side of the dry-run cache (task 4-8). found is true ONLY when an
// entry exists for the key AND its WrittenAt stamp is within TTL of the store's
// injected clock; an absent entry and an EXPIRED entry both yield (Entry{}, false,
// nil) so the real run regenerates rather than ever shipping a stale preview. The
// TTL check lives here (not at the caller) because the store owns the clock seam, so
// tests control expiry by injecting the reading store's now. A genuine read/decode
// error (a corrupt entry, a permissions failure) propagates so the caller can decide
// — it is NOT swallowed as a miss.
func (s *Store) Lookup(repoRoot, key string) (Entry, bool, error) {
	data, err := os.ReadFile(s.EntryPath(repoRoot, key))
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, false, nil // a clean miss: regenerate.
		}
		return Entry{}, false, fmt.Errorf("reading note cache entry: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Entry{}, false, fmt.Errorf("decoding note cache entry: %w", err)
	}

	if s.now().Sub(entry.WrittenAt) > TTL {
		return Entry{}, false, nil // expired: treat as absent, regenerate.
	}
	return entry, true, nil
}

// EntryPath returns the absolute path of the cache entry file for (repoRoot, key).
// It is exported so the reuse side (task 4-8) and tests can locate an entry by its
// computed key.
func (s *Store) EntryPath(repoRoot, key string) string {
	return filepath.Join(s.cacheDir(repoRoot), key+entryFileExt)
}

// cacheDir resolves the per-project cache directory: {base}/<repoScope>, where
// repoScope is the repo path munged to a readable, filesystem-safe name so distinct
// repos never share a dir.
func (s *Store) cacheDir(repoRoot string) string {
	return filepath.Join(s.base, repoScope(repoRoot))
}

// Prune deletes every cache entry for repoRoot EXCEPT keepKey's, bounding the
// per-project cache to the current diff's note so stale entries (older diffs and
// versions) never accumulate. It is best-effort HOUSEKEEPING: a directory it cannot
// read is a no-op, and a file it cannot delete is left in place (a leftover entry is
// harmless — the cache is only an optimization). It removes ONLY this package's own
// ".json" entries.
func (s *Store) Prune(repoRoot, keepKey string) {
	dir := s.cacheDir(repoRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	keep := keepKey + entryFileExt
	for _, e := range entries {
		if e.IsDir() || e.Name() == keep || !strings.HasSuffix(e.Name(), entryFileExt) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// repoScope derives the per-project sub-directory name under the cache base from the
// repo's absolute path — a readable munge (path separators → dashes, like Claude's
// session-transcript directories) so a human browsing the cache can tell which project
// a directory belongs to. e.g. /Users/lee/Code/mint → -Users-lee-Code-mint. (Two paths
// that differ only by a literal dash vs a separator would collide; the cost is at most
// a cache regenerate, never data loss, so a readable name is preferred to an opaque
// hash.)
func repoScope(repoRoot string) string {
	return strings.ReplaceAll(filepath.Clean(repoRoot), string(filepath.Separator), "-")
}

// entryPerm is the on-disk mode for a cache entry: 0o600, owner-only.
const entryPerm = 0o600

// writeFileAtomic writes data to path crash-safely (temp file + rename) so a crash
// mid-write never leaves a partial entry that a later reuse might read as valid. It
// delegates the shared idiom to fsutil.WriteFile; the caller (Write) wraps the
// returned error with the note-cache context, so it is returned as-is here.
func writeFileAtomic(path string, data []byte) error {
	return fsutil.WriteFile(path, data, entryPerm)
}
