TASK: mint-release-tool-1-8 — Publisher interface & GitHub driver (gh gate when publishing)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (and evolved through Phase 4/5 as-built). publish.go: Publisher interface + GitHubPublisher (CreateRelease/UpdateRelease/ReleaseExists). CreateRelease pipes body via `--notes-file -` on stdin, uses `--verify-tag`, returns trimmed stdout URL. preflight.go CheckGhAuth. engine/release.go: conditional resolve + gh gate strictly before TagAndPush. Gate ordering invariant holds. ReleaseExists + URL returns are legitimate Phase 4/5 seam, not 1-8 drift.

TESTS:
- Status: Adequate (well-balanced). Driver: exact argv + stdin body, distinct title/tag, empty-stdout→empty-URL, gh-fail surfaced, update mirror, ReleaseExists true/not-found/fail/missing-gh, publish gating table. Gate: installed/auth pass, not-installed/not-authenticated messages. Engine: gh-auth-before-tag ordering, gh-auth-fail resets commit w/ no tag delete, publish=false → no gh + still tags/pushes, publish-after-push fail warn-only. argv compared element-for-element.

CODE QUALITY:
- Followed conventions (runner seam, typed *GateError, errors.Is discrimination, table tests). SOLID good — tight Publisher seam. Low complexity, strings.Cut + wrapped errors, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/publish/publish.go:118,148 — ReleaseExists classifies absence by substring-matching literal "release not found" in gh stderr; brittle to gh wording/locale changes (a reworded message → misclassified as genuine probe failure, blocking create-or-update dispatch). Decide whether to harden (match on exit code / stable marker, or `gh api` with structured 404). Out of 1-8 scope but track for Phase 5 reuse path.
- [idea] internal/preflight/preflight.go:261 — CheckGhAuth probes `gh auth status` without a host argument, so it passes when gh is authed to ANY host, not necessarily the release remote's host. Fine for github.com-only; once non-github hosts/provider overrides are live, consider scoping to resolved host (`gh auth status --hostname <host>`). Design call.
