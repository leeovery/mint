TASK: mint-release-tool-1-6 — Network preflight gates (fetch --tags, remote sync, tag-free remote)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented and wired. preflight.go Fetch (`git fetch --tags`), CheckRemoteSync (`@{u}...HEAD` left-right count; behind>0 aborts with count + origin/{branch}; ahead/up-to-date pass), parseLeftRightCount, CheckTagFreeRemote, ErrNoUpstream sentinel. Orchestrated in engine/release.go: Fetch → RunLocalGates → CheckRemoteSync → CheckTagFreeRemote. Read-only; no pull anywhere. No-upstream → ErrNoUpstream (distinct from GateError and ErrCommandNotFound).

TESTS:
- Status: Adequate. Covers fetch issues exactly `git fetch --tags` + never pulls, fetch-precedes-remote ordering, behind aborts w/ count, diverged aborts, ahead/up-to-date pass, no-upstream → ErrNoUpstream not GateError, command-not-found hard error, tag-free-remote absent/exists/not-found. argRunner justified (FakeRunner matches name only).

CODE QUALITY:
- Followed conventions (runner seam, typed *GateError vs sentinels). SOLID good, low complexity, errors.Is/As + %w, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release.go:953 — runPreflight returns ErrNoUpstream as a plain bubbled error with no errors.Is(err, preflight.ErrNoUpstream) branch, so the user sees the wrapped "no upstream configured…" text rather than tailored "set an upstream / push -u" guidance. Acceptable for Phase 1; consider special-casing it into dedicated guidance (design decision).
