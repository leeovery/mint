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
// repo root, so previews never collide across repos) and lives in a gitignored path
// so it is NEVER committed.
package notescache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"mint/internal/fsutil"
)

// cacheDirName is the repo-relative base for the on-repo cache (e.g. .mint/cache).
// The whole .mint dir is gitignored (a single "*" .gitignore at .mint), so a
// repo-path cache is never committed.
const cacheDirName = ".mint"

// cacheSubdir is the cache namespace under cacheDirName, keeping the gitignored
// .mint base free for other future mint state.
const cacheSubdir = "cache"

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

// Store writes dry-run note cache entries under a base directory, repo-scoped and
// stamped with an injected clock. The base may be the system temp dir or a path
// under the repo; NewRepoStore selects the repo-path form (which also ensures the
// gitignore guard).
type Store struct {
	// base is the cache root the entries live under. When empty the store is in
	// REPO-PATH mode: entries live under {repoRoot}/.mint/cache and the .mint dir is
	// gitignored. When set (e.g. an injected temp dir or os.TempDir) entries live
	// under {base}/<repo-hash>/cache and no repo .gitignore is needed.
	base string
	// now supplies the TTL write stamp; injected so tests get a deterministic time.
	now func() time.Time
}

// NewStore builds a Store rooted at base (a system-temp-style location OUTSIDE the
// repo): entries are namespaced by a hash of the repo root so caches never collide
// across repos, and no repo .gitignore is needed because the cache lives outside
// the repo entirely. now supplies the TTL write stamp.
func NewStore(base string, now func() time.Time) *Store {
	return &Store{base: base, now: now}
}

// NewRepoStore builds a Store whose cache lives UNDER the repo at
// {repoRoot}/.mint/cache. Because the cache is inside the repo, Write ensures a
// .gitignore at {repoRoot}/.mint that ignores the whole dir, so the cache is never
// committed. now supplies the TTL write stamp.
func NewRepoStore(now func() time.Time) *Store {
	return &Store{base: "", now: now}
}

// Write persists body under key for repoRoot, stamping the entry with the store's
// clock. It creates the repo-scoped cache directory (and, in repo-path mode, the
// .gitignore guard) as needed, then writes the JSON entry atomically.
func (s *Store) Write(repoRoot, key, body string) error {
	dir := s.cacheDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating note cache dir %q: %w", dir, err)
	}
	if err := s.ensureGitignore(repoRoot); err != nil {
		return err
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

// HasEntries reports whether ANY cache entry file exists for repoRoot — fresh or
// expired. The real-run reuse path consults it to stay SILENT on a clean miss when
// no dry-run preview was ever written: warning "diff changed since dry-run preview"
// without a preview in existence is noise that misleads (observed live on a plain
// release). Read errors (no dir yet, unreadable dir) report false — absence of
// evidence of a preview is treated as absence of a preview.
func (s *Store) HasEntries(repoRoot string) bool {
	entries, err := os.ReadDir(s.cacheDir(repoRoot))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

// cacheDir resolves the repo-scoped cache directory. In repo-path mode it is
// {repoRoot}/.mint/cache (gitignored); otherwise it is {base}/<repo-hash>/cache,
// namespaced by a hash of the repo root so distinct repos never share a dir.
func (s *Store) cacheDir(repoRoot string) string {
	if s.base == "" {
		return filepath.Join(repoRoot, cacheDirName, cacheSubdir)
	}
	return filepath.Join(s.base, repoScope(repoRoot), cacheSubdir)
}

// ensureGitignore writes the gitignore guard for a repo-path cache so the .mint
// dir is never committed. In external-base mode the cache lives outside the repo,
// so there is nothing to ignore and it is a no-op.
func (s *Store) ensureGitignore(repoRoot string) error {
	if s.base != "" {
		return nil
	}
	mintDir := filepath.Join(repoRoot, cacheDirName)
	if err := os.MkdirAll(mintDir, 0o755); err != nil {
		return fmt.Errorf("creating cache base %q: %w", mintDir, err)
	}
	ignorePath := filepath.Join(mintDir, ".gitignore")
	if _, err := os.Stat(ignorePath); err == nil {
		return nil // already guarded; do not clobber an existing entry.
	}
	if err := os.WriteFile(ignorePath, []byte("*\n"), 0o644); err != nil {
		return fmt.Errorf("writing cache .gitignore %q: %w", ignorePath, err)
	}
	return nil
}

// repoScope derives a stable, filesystem-safe namespace for repoRoot so an
// external-base cache keeps each repo's entries apart. It is the hex SHA-256 of
// the repo root path — collision-free in practice and a valid directory name.
func repoScope(repoRoot string) string {
	sum := sha256.Sum256([]byte(repoRoot))
	return hex.EncodeToString(sum[:])
}

// entryPerm is the on-disk mode for a cache entry: 0o600, the default os.CreateTemp
// mode the original writer left in place (no chmod), kept so a gitignored cache entry
// stays owner-only.
const entryPerm = 0o600

// writeFileAtomic writes data to path crash-safely (temp file + rename) so a crash
// mid-write never leaves a partial entry that a later reuse might read as valid. It
// delegates the shared idiom to fsutil.WriteFile; the caller (Write) wraps the
// returned error with the note-cache context, so it is returned as-is here.
func writeFileAtomic(path string, data []byte) error {
	return fsutil.WriteFile(path, data, entryPerm)
}
