TASK: interactive-mint-init-setup-6-2 — Reuse a single toml-tag-reading primitive in tomlTagsOf and schemaLeafKeysInto

ACCEPTANCE CRITERIA:
- The `Tag.Get("toml")` read and the ""/"-" skip guard appear in exactly one helper.
- Both `tomlTagsOf` and `schemaLeafKeysInto` consume that helper; neither carries its own inline tag read.
(Plus task-derived checks: bijection/independence tests pass unchanged; a future tag-option suffix like `,omitempty` is handled in one place; no premature abstraction / clear seam.)

STATUS: Complete

SPEC CONTEXT:
This is an analysis-cycle-2 remediation task (severity low, source: duplication) from analysis-tasks-c2.md Task 2. The broader feature (interactive mint init setup) establishes a config-metadata Source of Truth (MetadataRows() in metadata.go) plus a reflection-derived schema leaf-key set (schemaLeafKeys in metadata_drift_test.go); a bijection drift test proves the two cannot diverge. The two derivation sides are deliberately INDEPENDENT (one hand-written, one schema-reflected) so divergence is caught. The finding: two helpers in metadata_drift_test.go each re-implemented the same `field.Tag.Get("toml")` read + `tag == "" || tag == "-"` skip guard, so a change to the skip-token convention (or a future tag-option suffix) would need editing in both. Remediation: extract one tag-reading primitive both callers consume. The constraint per Task 1 / the file header is that the two DERIVATIONS (MetadataRows / schemaLeafKeys) stay untouched and independent — only the shared parse-and-skip rule is single-sourced.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - New primitive: internal/config/metadata_drift_test.go:250-256 (`tomlTag(field reflect.StructField) (name string, ok bool)`) — reads `field.Tag.Get("toml")` once, returns `ok=false` for "" / "-".
  - Consumer 1: internal/config/metadata_drift_test.go:77 (`schemaLeafKeysInto` now calls `tag, ok := tomlTag(field)` and skips on `!ok`).
  - Consumer 2: internal/config/metadata_drift_test.go:238 (`tomlTagsOf` now calls `if tag, ok := tomlTag(st.Field(i)); ok`).
- Notes:
  - Verified by grep that `Tag.Get("toml")` now appears in EXACTLY one location across internal/config (metadata_drift_test.go:251 inside `tomlTag`). The literal `"" / "-"` skip guard likewise lives only in `tomlTag` (the remaining "-" mention at line 78 is a comment).
  - The 6-2 commit (2a319b9) diff is a pure mechanical extraction: +19/-6 lines, touching only metadata_drift_test.go (plus bookkeeping). Both call-site rewrites are behaviour-preserving — the old `tag == "" || tag == "-"` → `continue`/skip path maps exactly onto the new `!ok` / `!ok`-false path.
  - `MetadataRows()` (metadata.go) and `schemaLeafKeys()`/`schemaLeafKeysInto` derivation logic are NOT touched — the two independent bijection sides are preserved. Confirmed against the prior commit (be01618) which carried 2 `Tag.Get` reads; the 6-2 commit reduces that to 1.
  - The (d) criterion — a future `,omitempty`-style tag-option suffix handled in one place — is structurally satisfied: any such handling would be added inside `tomlTag` once and both callers inherit it. (The current code does not split on a comma yet, which is correct — there are no tag options in the schema today and adding one pre-emptively would be premature; the single seam means it can be added in one place when needed. Noted non-blocking below.)

TESTS:
- Status: Adequate (existing tests cover the refactor; no new test needed per the task's own Tests section, which only requires the existing bijection/independence tests to pass unchanged).
- Coverage:
  - `tomlTag` is exercised transitively by every test routing through `schemaLeafKeys` (TestSchemaLeafKeys_* in metadata_drift_test.go) and `tomlTagsOf` (TestSchemaLeafKeys_IndependentOfSoT, metadata_drift_test.go:213-230).
  - The two-sided bijection (metadata_bijection_test.go) matches `schemaLeafKeys()` (now via `tomlTag`) against `MetadataRows()` — the authoritative drift guard, unchanged.
  - TestSchemaLeafKeys_DerivesAllLeafPairs and TestMetadataRows_OneRowPerLevelKeyPair both assert against the shared census (ExpectedLeafKeys, from 6-1) — unaffected.
- Notes:
  - The task explicitly does not ask for a direct unit test of `tomlTag`, and one is not warranted: the helper is a 3-line same-package test primitive fully exercised through its two callers, and its skip behaviour is asserted by the derivation tests (a leaf is emitted iff its tag is non-empty/non-"-"). Adding a dedicated `tomlTag` table test would be over-testing for a trivially-correct extraction.
  - Tests were assessed by reading; the suite was not run (per role constraints). The diff's behaviour-preserving nature makes a green run the expected outcome.

CODE QUALITY:
- Project conventions: Followed. External-vs-internal test package split is respected (metadata_drift_test.go stays `package config` to reach unexported shapes; the new primitive is unexported test-only support). Heavy WHY-comment convention is honoured — the `tomlTag` doc comment (lines 245-249) states the single-source contract and the future-change rationale, true to as-built. The `schemaLeafKeysInto` "Defensive" comment (lines 78-80) remains accurate after the rewrite.
- SOLID principles: Good. Single responsibility — `tomlTag` owns exactly the parse-and-skip rule; callers own their own shaping (set vs recursive walk). The `(name, ok)` signature is the idiomatic Go seam for "lookup that may be absent", correctly decoupling the skip decision from each caller's loop.
- Complexity: Low. The primitive is a single read + one guard. No added branching at call sites (each replaced a 2-condition inline guard with one boolean).
- Modern idioms: Yes. `(value, ok bool)` comma-ok return is the conventional Go shape; named returns document intent without obscuring control flow.
- Readability: Good. Intent is self-evident; both call sites read more cleanly than the inline form they replaced.
- Issues: None. No premature abstraction — the primitive is the minimum that removes the duplication; it does not over-generalise (e.g. no parameterised tag-name, no speculative option parsing).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/config/metadata_drift_test.go:250-256 — `tomlTag` does not yet split a `name,opt1,opt2` tag value on the first comma, so a future field tagged e.g. `toml:"foo,omitempty"` would yield the literal name "foo,omitempty" rather than "foo". This is correct for today's schema (no tag options exist) and pre-emptively adding comma-splitting would be premature abstraction; the value of this task is precisely that such handling now has ONE home. Flagged only so that whoever first introduces a tag option knows to add the split here (the single seam) — a decision to defer until a real option appears.
