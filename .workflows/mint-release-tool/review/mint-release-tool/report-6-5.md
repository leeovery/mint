TASK: mint-release-tool-6-5 — .mint.toml commented-template generation

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/initgen/initgen.go:31-65 MintTOML(); pure string generator, no IO. Common keys active at defaults (39-49), optional keys commented w/ inline one-line explanations (42, 51-56), [release.hooks] block commented w/ preflight/pre_tag(string)/post_release + array-form prose example (58-63), prompt-override only mentioned (56), no top-level [hooks], no auto-detection (stated in docs). Every uncommented config line is a known schema key w/ correct type + valid enum (on_notes_failure='abort') — full uncommented document loads cleanly.

TESTS:
- Status: Adequate. initgen_test.go: common keys at defaults, optional keys present-but-commented each w/ explanation, no top-level [hooks] + [release.hooks] present, pre_tag string+array forms, prompt mentioned in comment only, validity guarantee (programmatically uncomment then feed through REAL config.Load + survived-defaults check). "No auto-detection" asserted via static-generator design/doc (no IO path to exercise).

CODE QUALITY:
- Followed conventions (pure generator, package/func docs, raw string literal). SOLID good — single responsibility (string generation only; IO deferred to 6-6/6-7). Low complexity, self-documenting template.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/initgen/initgen.go:42 — the diff_exclude commented example uses '*.min.js', a basename glob; git pathspecs are repo-root-relative by default so *.min.js matches only top-level files, not nested ones (the config.go DiffExclude doc uses skills/**/knowledge.cjs). Consider whether the example should use a recursive form ('**/*.min.js') so an operator copying it gets the likely-intended behaviour. Still schema-valid.
- [do-now] internal/initgen/initgen.go:62 — the pre_tag array-form is documented in a prose comment rather than a togglable config line; consider appending a brief inline note that it replaces the string form above (only one pre_tag may be set) to pre-empt a user uncommenting both. Comment-only.
