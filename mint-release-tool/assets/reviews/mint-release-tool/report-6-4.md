TASK: mint-release-tool-6-4 — Route earlier-phase as-needed loaders through the validated schema

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/config/config.go is the single canonical schema + Load (one os.ReadFile, one strict toml.Decode, then validateHooks/validateOnNotesFailure). Every Phase 1-4 key has its field + prior default. Provider is plain string carried verbatim — never rejected by validation. Orchestrator sequencing: engine/release.go:288 calls config.Load at Stage 1 after root resolution; config error returns via surface(p,"config",err) BEFORE branch/version/preflight/notes. Publish resolution reads cfg.Release.Provider at :894 → ResolvePublisher → ErrProviderUnresolved → warnPublishDowngraded. Single decode — .mint.toml TOML decoding exists only in config.go; no separate/minimal loader remains.

TESTS:
- Status: Adequate. release_configconsolidation_test.go: up-front abort (4 bad-config cases; only git rev-parse --show-toplevel runs, abort surfaces as "config" StageFailed); Phase-2 key (ai_command) drives transport through consolidated Config w/ prior default. release_downgrade_test.go: provider="gitlab" loads clean + warns+downgrades on github.com remote. config_test.go: each Phase 1-4 key's prior default + whole-file validation.

CODE QUALITY:
- Followed conventions (single decode seam, clear error wrapping, doc comments). SOLID/DRY good — one schema, resolve helpers per concern, no duplicated loader. Low complexity, errors.As, *bool/*int absent-vs-explicit.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:344-352 — translateTypeError identifies the offending field by substring-matching the decoder's error text against keys like "fileShape.MaxDiffLines". Couples mint's clear messages to go-toml/v2's internal error-text format; a library upgrade reworking that text would silently fall back to the raw decoder description (still safe, less friendly). [Same family as 6-1/6-3 notes.]
- [do-now] internal/config/config.go:133-136 — the Release struct doc comment paragraph order may not match the field order for Fallback (after OnNotesFailure); confirm the doc paragraph order matches the field order. Comment-ordering tidy, zero logic impact.
