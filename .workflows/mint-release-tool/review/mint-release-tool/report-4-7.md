TASK: mint-release-tool-4-7 — Dry-run note cache write & key computation

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (no drift). Note: key-input plumbing lives as CacheInputs in internal/notes/select.go (not a `cacheinputs` package), tests in cacheinputs_test.go — naming differs, behaviour matches. notescache/cache.go: Key (length-prefixed concat of diff+version+instructions, SHA-256, injective), Store/Write, repo-scope, gitignore guard. versionKey = next.String("") bare. Instructions from ResolveInstructions (prompt-override OR default+injected-context) so prompt/context changes invalidate key. Write failure warn-only; cache write is sole dry-run side effect. Atomic write via fsutil.

TESTS:
- Status: Adequate. cache_test.go: Key determinism, per-input change, field-boundary injectivity, body+TTL-stamp persistence, repo-scoping, gitignore guard. release_dryruncache_test.go: end-to-end write, key-changes-with-diff/version/prompt-context, HEAD-sha invariance, hooks-still-skipped + no-mutation + cache-sole-side-effect. cacheinputs_test.go: Cacheable only on normal-AI path.

CODE QUALITY:
- Followed conventions (accept-interfaces for NoteCache/Writer/Reader, injected clock, warn-only non-fatal, shared fsutil, table tests). SOLID good — segregated reader/writer, single-responsibility Store, key computation isolated. Low complexity, thorough doc comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/notescache/cache.go:211-214 — the entryPerm doc comment ("0o600, the default os.CreateTemp mode the original writer left in place (no chmod)") is stale after the refactor to fsutil.WriteFile, which now explicitly Chmods to perm. Reword to state 0o600 is the deliberate owner-only mode passed to fsutil.WriteFile (drop the "no chmod" framing). Value/behaviour correct.
- [quickfix] internal/engine/release_dryruncache_test.go:302-319 — cacheEntryKeys reconstructs the cache dir by string-trimming EntryPath(root, "") of the .json suffix and hand-rolls suffix matching; brittle if the extension changes. Export a small dir/extension accessor on Store (or a notescache test helper) so listing doesn't depend on EntryPath's filename shape.
- [idea] internal/notescache/cache.go:185-201 — ensureGitignore writes `*` only when .mint/.gitignore is absent and never reconciles an existing-but-non-ignoring file. Acceptable for now (whole-dir ignore is the documented contract); whether to detect/repair a partial existing ignore is a design decision for hardening.
