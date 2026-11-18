TASK: mint-release-tool-4-10 — Safe downgrade to tag + push on unresolved provider

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented, no drift. internal/publish/resolve.go: ErrProviderUnresolved sentinel + UnresolvedError with Reason(); host parse across HTTPS/ssh://`/SCP. engine/release.go: Stage 6/7 downgrade switch (L556-577), gh gate gated on publisher != nil, publish gated on publisher != nil, warnPublishDowngraded + downgradeReason, dry-run mirror dryRunPublishTarget. Downgrade branches on errors.Is(err, ErrProviderUnresolved) → warn; any other resolve error stays a pre-PONR surgical-unwind abort. publish=false skips entire block (no remote read) — genuinely silent. Never falls through to GitHub.

TESTS:
- Status: Adequate. resolve_test.go: all four unresolved causes return sentinel + nil + correct Reason() (unknown value / non-github host / no remote / unparseable SSH) + resolved/override happy paths. release_downgrade_test.go: unknown provider value (remote IS github.com proving value governs), GHE/GitLab/Gitea hosts table, unparseable SSH, no remote, publish=false silent (no warn + remote never read), never-assumes-GitHub (no gh release create). Each downgrade asserts no gh invocation + annotated tag + atomic push ran.

CODE QUALITY:
- Followed conventions (accept-interfaces, runner seam, sentinel+errors.Is/As, table tests, t.Parallel). SOLID good — resolver owns selection, engine owns downgrade policy. Low complexity, errors.As + strings.Cut, heavily documented.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release.go:562-563 — a non-unresolved resolve error is routed to a "preflight" surgical-unwind abort, but resolvePublisher currently can only ever return ErrProviderUnresolved (GitHub driver construction does no IO), so this branch is presently unreachable. Decide whether to keep it as defensive forward-compat scaffolding (fine) or drop it; if kept, a one-line test asserting a non-sentinel error aborts rather than downgrades would lock the policy.
- [do-now] internal/engine/release_downgrade_test.go:30-33 — the seedDowngradeGit doc comment ("It is seedHappyGitRemote minus nothing on the git side") is garbled; reword to state plainly it scripts the full happy git timeline including the remote read but seeds no gh.
