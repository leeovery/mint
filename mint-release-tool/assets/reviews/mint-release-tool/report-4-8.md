TASK: mint-release-tool-4-8 — Real-run cache reuse, miss-regenerate & TTL/gate orthogonality

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. engine/release.go:771-791 realRunReuse closure, :746 wired into resolveBody via SelectBodyWithReuse, :799-825 reuse/miss/unreadable notices; notes/select.go:150-198 SelectBodyWithReuse pre-AI interception; notescache/cache.go:145-163 Lookup (TTL/expiry/corrupt-read). Key consistency: write side writeDryRunNoteCache and read side realRunReuse both compute notescache.Key(diff, versionKey, instructions). Reuse hook nil under --dry-run or no cache → reuse automatic. TTL/corrupt/expiry behind store seam; engine degrades read errors to regeneration (never aborts).

TESTS:
- Status: Adequate. release_realruncache_test.go: key match no-AI (claude unseeded so any invocation errors), miss + spec message, expired TTL, automatic/no-flag, excluded-artifact reuse, non-excluded-change miss, reused note shown at interactive gate, -y skips on real + dry run, corrupt-read degrade. notescache/cache_test.go: TTL boundary/expiry/corrupt. cacheinputs_test.go: selector-level reuse interception.

CODE QUALITY:
- Followed conventions (accept-interfaces NoteCacheReader/Writer, runner seam, warn-only degrade). SOLID good — selector knows nothing of cache/clock; engine closes over them via ReuseFunc. Low complexity, distinct notices.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/notes/select.go:104 — ReuseFunc takes no context.Context, so the engine's Lookup (file I/O) runs uncancellable. Fast local read so impact negligible, but threading ctx through ReuseFunc(ctx, diff, instructions) would align with the codebase's context discipline.
- [quickfix] internal/engine/release.go:746 — the reuse closure is constructed/resolved even when deps.NoteCache==nil returns a nil hook early; fine, but a one-line guard comment at the SelectBodyWithReuse call clarifying nil hook == always-generate would aid the next reader.
- [idea] internal/engine/release.go:802,810 — the reuse/miss notices ride p.Warn with label "notes"; a reused note is a normal outcome so this slightly overloads "warning" semantics (the comment justifies it — Warn sets no failure state). If a neutral "notice" seam ever lands, migrate these and reportNotesCacheUnreadable to it.
