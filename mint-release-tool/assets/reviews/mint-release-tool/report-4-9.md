TASK: mint-release-tool-4-9 — Provider auto-detection from remote host

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/publish/resolve.go:79 ResolvePublisher, :101 resolveByHost, :123 parseHost (HTTPS/ssh:// via net/url, SCP-like via scpHost), :89 resolveByProvider override. Wired at engine/release.go:893 resolvePublisher + :905 RemoteURL (`git remote get-url origin` via runner), and cmd/mint/main.go + regenerate_all.go. Override wins over detection; recognised-but-unsupported value does NOT fall through to GitHub (returns sentinel). Only NewGitHubPublisher sites are inside the resolver. Returns behind Publisher interface. Sentinel + *UnresolvedError with Unwrap.

TESTS:
- Status: Adequate. resolve_test.go: HTTPS, SCP-SSH, ssh://, explicit override-on-non-github, behind-interface, unresolved/reason cases. release_provider_test.go: table over all three github URL forms + override, remote read via runner, GitHub driver published (gh release create), gh auth gate ran before tag. remote_url_test.go pins RemoteURL reader.

CODE QUALITY:
- Followed conventions (sentinel+typed-error, errors.Is/As, runner seam, t.Parallel, table tests). SOLID good — resolver depends on Publisher + CommandRunner, open to new drivers without caller change. Low complexity, strings.Cut/url.Hostname, good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/publish/resolve.go:127 — parseHost treats any string containing "://" as parseable via net/url. No defect found for the three documented git forms, but consider whether an explicit scheme allowlist is worth adding (future-proofing).
- [do-now] internal/engine/release.go:893 — resolvePublisher is a one-line wrapper used only via the spine; the cmd regenerate paths call publish.ResolvePublisher(engine.RemoteURL(...), ...) directly and discard the error (, _) with a duplicated rationale comment (noted in analysis-duplication-c2). Consider exporting a single engine-level resolve helper both cmd paths reuse to remove the duplicated resolve-and-discard step + comment.
