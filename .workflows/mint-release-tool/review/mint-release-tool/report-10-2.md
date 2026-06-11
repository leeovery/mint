TASK: Fix the Nil-Publisher Crash on the Regenerate Paths (mint-release-tool-10-2, type: bug)

ACCEPTANCE CRITERIA:
- `mint release regenerate <ver> --reuse -y` against a non-github / no-remote origin does not panic.
- The unresolvable-provider case either aborts with a clear message or downgrades (provider write skipped) consistent with `engine.Release`.
- No regenerate code path dereferences a nil `Publisher`.

STATUS: Complete

SPEC CONTEXT:
specification.md:430 — "Auto-detection with no matching driver" (non-github.com remote, unmatchable SSH host, or no remote at all) with publish=true and no explicit provider is treated the same as an unsupported provider value: mint warns loudly and downgrades to tag + push only, never silently assuming GitHub. The forward `engine.Release` Stage-6 (release.go:556-577) already implements this carve-out. This task brings the regenerate paths into line, removing the `publisher, _ := …` discards that passed nil down into `DispatchRelease` → `ReleaseExists` and panicked.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Shared helper: internal/engine/release.go:928-939 `engine.ResolvePublisher(ctx, deps, cfg)` — wraps `resolvePublisher` (→ `publish.ResolvePublisher(RemoteURL(...), cfg.Release.Provider, r)`), branches on `publish.ErrProviderUnresolved` (warn-and-downgrade, returns nil,nil), surfaces any other error as a preflight abort, else returns the driver. Mirrors the forward Stage-6 switch (release.go:559-566) precisely.
  - cmd helper: cmd/mint/main.go:221-227 `resolveRegeneratePublisher` collapses the helper into (publisher, exitCode, proceed).
  - Call site 1 (single): cmd/mint/main.go:183 — replaces the former `publisher, _ := …` discard.
  - Call site 2 (--all batch): cmd/mint/regenerate_all.go:48 — same replacement.
  - Nil-guard (single): internal/engine/regenerate_write.go:170-174 — `if publisher == nil { warnRegenerateProviderSkipped(p); return nil }` before `DispatchRelease`.
  - Nil-guard (batch): internal/engine/regenerate_batch.go:296-298 — matching guard in `processOneVersion`; the version is still collected (downgrade, not a per-version failure).
- Notes: All three "Do" steps land. The shared helper genuinely consolidates the two discard sites AND mirrors the forward path, satisfying the folded-in Idea #17 in the same change as specified. RemoteURL (release.go:949) is the single shared "empty == unresolved" remote reader, so forward and regenerate cannot drift. The supersession instructions were honoured (no separate Idea #17 / quick-fix #13 work surfaced).

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/regenerate_nilpublisher_test.go:26 — single-version --target release, nil publisher → no panic, no abort, warn emitted.
  - :51 — single-version --target both, nil publisher → changelog still committed+pushed (asserts `git push origin HEAD`), provider skipped with warn, no nil deref post-push.
  - :86 — `--all` batch (RegenerateAll), nil publisher, reuse → no panic, all 3 versions still collected (downgrade not a drop), warn emitted. Satisfies the required batch test.
  - :113 — shared engine.ResolvePublisher downgrade path: warns, returns nil,nil, no error; asserts the "publish skipped" warn label.
  - cmd/mint/regenerate_publisher_test.go:30 — cmd helper unresolved → proceed=true, nil publisher, exit 0 (the former quick-fix #13 cmd-level unresolved test, folded in).
  - :53 — cmd helper resolved github origin → real driver, proceed=true, exit 0 (normal path unregressed).
- Notes: Tests directly target the reproduced crash (nil interface → ReleaseExists) and assert the no-panic + warn/downgrade outcome rather than just "no error". Both required tests (single + batch) present, plus the engine-helper and both cmd-helper branches. Focused, no redundant bloat. Would fail if the guard or the discard regressed (the nil-publisher tests would panic). The acceptance criteria's explicit "aborts with a clear message" branch (a non-ErrProviderUnresolved resolution error → surface+abort) is exercised at the helper level only indirectly; the forward path's abort branch is well covered elsewhere, and the regenerate helper reuses the same `resolvePublisher`, so this is a minor, non-blocking gap (see below).

CODE QUALITY:
- Project conventions: Followed. Nil safety (golang-safety) is the heart of the fix — explicit nil-guard at both dispatch sites before the interface call. Error handling uses errors.Is against the wrapped sentinel (ErrProviderUnresolved via UnresolvedError.Unwrap, resolve.go:43-45), matching golang-error-handling. Testify not required; table-free focused tests match golang-testing.
- SOLID principles: Good. DRY win — the two discard sites collapse to one shared helper that also unifies with the forward path's intent; RemoteURL and resolvePublisher are single-owned readers. Single responsibility preserved (resolve vs. dispatch vs. warn each separate).
- Complexity: Low. Helper is a 3-arm switch; guards are single ifs.
- Modern idioms: Yes — context.WithoutCancel reset resilience untouched; errors.Is/Unwrap sentinel matching.
- Readability: Good. Comments at every changed site explicitly name the reproduced crash and the forward-path mirror, so the intent (why nil is now safe) is self-documenting.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/engine/regenerate_nilpublisher_test.go — add a test for engine.ResolvePublisher's abort arm (a non-ErrProviderUnresolved resolver error → returns surfaced error, StageFailed emitted): the AC's "aborts with a clear message" branch is asserted only on the forward path, not on the shared regenerate entry. Concrete edit at a known location, mechanical (seed a non-unresolved error, assert err != nil + a "preflight" StageFailed). Low priority — the helper reuses the forward-covered resolvePublisher.
