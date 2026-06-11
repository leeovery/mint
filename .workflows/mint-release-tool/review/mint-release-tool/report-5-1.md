TASK: mint-release-tool-5-1 — regenerate command skeleton & two-axis flag parsing

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. Subcommand wiring cmd/mint/main.go:55-71,262-280 (classifyCommand routes `release regenerate` → commandRegenerate, bare `regenerate` → unknown); parse + rules cmd/mint/regenerate_flags.go:75-213. Single-value --target, source enum default fresh, SourceSet distinguishes defaulted vs explicit --fresh (for 5-10), targetUnset zero value preserves "omitted", exact error strings. Stdlib flag (consistent with project); splitRegeneratePositional works around flag's "stop at first non-flag" so <version> may sit before/between/after flags. Parse is pure — no runner/network/mutation.

TESTS:
- Status: Adequate. regenerate_flags_test.go: all 7 mandated cases + positional-ordering variants (before/mid/after, --target= inline), --target changelog, both -y/--yes, unknown-target exact message, bare error, version+all error, reuse+fresh error, two-positionals error. dispatch_test.go: release regenerate routes correctly; bare regenerate unknown.

CODE QUALITY:
- Followed conventions (stdlib flag consistent, lowercase no-punctuation error strings, doc comments). SOLID good — parse decomposed into focused helpers (splitRegeneratePositional, resolveRegenerateSource/Target, checkVersionPresence). Low complexity, intent-revealing names.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/mint/regenerate_flags.go:142 — --target as the final token with no following value (`regenerate 1.4.0 --target`) falls through to fs.Parse, which errors with flag's generic "flag needs an argument: -target" rather than the curated invalid-target wording. Add a test asserting this path errors (acceptable message), or a small guard returning the spec-style message; behaviour correct, only wording generic.
- [idea] cmd/mint/regenerate_flags.go:160-168 — isFlagToken/isTargetFlag hard-code that --target is the only value-taking flag; if a future regenerate flag takes a value the positional split silently mis-parses. Consider deriving value-taking flags from the FlagSet rather than a hand-maintained predicate. Defer — comment already notes the assumption.
