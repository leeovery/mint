TASK: mint-release-tool-1-2 — Minimal config load (tag_prefix, commit_prefix, release_branch, publish)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (config.go is now the FULL canonical schema; later phases — notably Phase 6 — extended this package; the four Phase-1 keys remain present and correct)
- Location: internal/config/config.go
- Notes: Four keys Release.{TagPrefix,CommitPrefix,ReleaseBranch,Publish}. Defaults seeded; present-file decode into pre-seeded fileShape so absent keys keep defaults. publish bool-trap solved via *bool + boolOrDefault. Explicit tag_prefix="" preserved. Absent/blank/comments-only → defaults. Single TOML decoder (pelletier/go-toml/v2). Downstream wiring confirmed in engine/release.go and gitrepo.go (empty release_branch → auto-derive).

TESTS:
- Status: Adequate
- Coverage: All six Phase-1 criteria mapped to focused tests via config.Load(t.TempDir()). Behaviour-focused, standalone.

CODE QUALITY:
- Followed conventions; accurate doc comments; errors.As for typed decoder errors; *T absent-vs-zero idiom. SOLID good, low complexity, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:344-352 — translateTypeError identifies the offending field by substring-matching go-toml/v2's rendered error text against Go struct-field paths; this couples the user-facing type message to the library's internal wording (an upgrade could silently drop to the raw fallback). Consider deriving the message from the DecodeError's exposed position/keys rather than its rendered text. (Phase-6 surface, not 1-2.)
