# gh-output classification brittleness (substring-matching English stderr)

The provider/auth layer classifies `gh` results by substring-matching English stderr text, which is fragile to gh wording changes and locale. Decide whether to harden against this (e.g. exit code, `gh api` 404, or another structured signal):

- `internal/publish/publish.go:118,148` — `ReleaseExists` keys on a "release not found" marker in stderr (Reports 1-8, 5-7)
- `internal/preflight/preflight.go:261` — `CheckGhAuth` probes `gh auth status` with no `--hostname`, so auth against *any* host passes (Report 1-8)

Non-blocking. A decision/design item: choose between hardening to a structured signal vs accepting the test-guarded coupling.

Source: review of mint-release-tool/mint-release-tool (Recommendation #18)
