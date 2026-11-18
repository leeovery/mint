TASK: mint-release-tool-6-1 — Typed config schema structs (full verb-namespaced shape)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (with minor superset — file is final consolidated state carrying 6-2/6-3/6-4). internal/config/config.go: Config (80-103), Release (145-159), HookValue/Hooks (171-179), defaults() (182-201), fileShape/releaseShape/hooksShape (212-243), Load (249-302), resolvers (378-426). Defaults match spec block exactly. Bool-trap via *bool + boolOrDefault; max_diff_lines/ai_command use *int/*string absent-vs-explicit idiom. tag_prefix decoded onto default-pre-seeded releaseShape so explicit "" overwrites "v" while absence preserves default. HookValue is any, supports string + []any. Single TOML dep (pelletier/go-toml/v2).

TESTS:
- Status: Adequate. config_test.go: absent, empty/comment-only, partial [release], only-top-level, publish=false/changelog=false, explicit empty tag_prefix, hook string vs array. Behaviour-focused (observable config values).

CODE QUALITY:
- Followed conventions (package doc, exported-symbol docs, t.Parallel, table test, stdlib assertions). SOLID/DRY good — single decode+resolve pass; boolOrDefault/resolveMaxDiffLines/resolveAICommand factor the idiom. Low complexity, errors.As on typed decoder errors.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/config.go:155 — Release.Fallback (TOML key `fallback`) is part of the as-built canonical schema and genuinely consumed (notes/resolve.go:67,82 via rel.Fallback), but it is not in the 6-1 Do list or the spec's schema block (lines 622-645). Decide whether `fallback` should be added to the spec's documented schema block so the consolidated schema and spec stay in sync, or whether it is intentionally internal-only.
- [do-now] internal/config/config.go:344-352 — translateTypeError iterates typeErrorMessages (a map); iteration order is non-deterministic. Safe today because at most one field path appears per DecodeError text; add a one-line comment stating that invariant so a future reader doesn't assume map order is load-bearing.
- [quickfix] internal/config/config.go:330-335,347 — bad-type detection matches on the Go struct-field path embedded in the decoder's error text (field+" "), a fragile coupling to pelletier's message format that a library upgrade could break (falling through to the opaque description). Add a test asserting each mapped path still produces the mint message after the decoder version in go.mod. [Same family as 6-3 note.]
