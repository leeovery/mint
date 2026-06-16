AGENT: duplication
FINDINGS:
- FINDING: The full 25-pair (level, key) expected list is hand-authored verbatim in two test functions
  SEVERITY: medium
  FILES: internal/config/metadata_test.go:43-73, internal/config/metadata_drift_test.go:125-155
  DESCRIPTION: TestMetadataRows_OneRowPerLevelKeyPair (metadata_test.go) and
    TestSchemaLeafKeys_DerivesAllLeafPairs (metadata_drift_test.go) each carry the
    SAME ~30-line literal enumeration of all 25 (level, key) pairs — identical
    ordering, identical comment structure (grouped "Shared / [release] /
    [release.hooks] / [commit]"), differing only in the element type name (config.rowKey
    `{config.LevelShared, "ai_command"}` vs the internal `leafKey{LevelShared,
    "ai_command"}`). This is the classic copy-paste-drift surface: a schema key added,
    renamed, or removed must be edited in BOTH lists by hand, and the two will silently
    diverge if an editor touches only one. NOTE: this is distinct from — and does NOT
    undermine — the deliberate independence of the two DRIFT SIDES (SoT rows vs
    reflection-derived schema tags); that independence is the design's anti-drift
    value. The duplication here is the third, hand-maintained EXPECTED literal in each
    naming test, which is neither the SoT nor the reflected schema — it is a manual copy
    of the same census kept in two files.
  RECOMMENDATION: Extract the single ordered expected-pair census to one shared
    internal slice in the config package (e.g. an unexported `expectedLeafKeys` var, or
    a same-package helper returning []leafKey, reusable since both tests already live in
    or alongside package config — metadata_drift_test.go is `package config`). Have both
    naming tests assert against that one list (metadata_test.go's config_test rows can be
    projected from it). Keep the SoT-side and schema-side DERIVATIONS independent (do not
    merge MetadataRows() with schemaLeafKeys()); only the hand-written expected census is
    consolidated. This removes the dual hand-edit without weakening the bijection.

- FINDING: rowKey struct + (level,key)->row index map re-authored in two test packages
  SEVERITY: low
  FILES: internal/config/metadata_test.go:14-33 (rowKey + rowSet), internal/setupguide/setupguide_test.go:477-492 (rowKey + rowByLevelKey)
  DESCRIPTION: Both test files define an identical `rowKey` struct
    (`{level config.MetadataLevel; key string}`) and a near-identical builder that folds
    config.MetadataRows() into a map[rowKey]config.MetadataRow keyed on that pair
    (rowSet in config_test, rowByLevelKey in setupguide_test). The struct is byte-for-byte
    the same; the two builders differ only in rowSet's extra duplicate-collision Fatalf.
    Both were independently written to solve the same "look up the SoT row for a (level,
    key) pair" need across the task boundary (the SoT test vs the renderer test).
  RECOMMENDATION: Lower priority because the two live in different packages (config_test
    vs setupguide_test) so sharing requires a small exported test-support seam, and the
    duplication is modest (~12 lines each). If consolidated, the natural home is a tiny
    exported helper in config (e.g. config.MetadataByLevelKey() returning the indexed map,
    or a config/configtest support file), which both test packages could consume — keeping
    the (level, key) indexing single-sourced alongside the SoT it indexes. Acceptable to
    leave as-is given the package boundary, but flag for awareness: a change to the SoT
    row identity model would need touching both builders.

- FINDING: tomlTagsOf duplicates the toml-tag-walk already inside schemaLeafKeysInto
  SEVERITY: low
  FILES: internal/config/metadata_drift_test.go:261-269 (tomlTagsOf), internal/config/metadata_drift_test.go:72-100 (schemaLeafKeysInto)
  DESCRIPTION: tomlTagsOf re-implements the same per-field `field.Tag.Get("toml")` read
    with the identical `tag == "" || tag == "-"` skip guard that schemaLeafKeysInto
    already performs, only flattened into a single-level name set for the independence
    test. Two functions in the same file now own the toml-tag-extraction rule; a change
    to the skip-token convention (or a future tag option suffix like `,omitempty`) must be
    updated in both.
  RECOMMENDATION: Have tomlTagsOf reuse a single tag-reading primitive (e.g. a small
    `tomlTag(field) (name string, ok bool)` helper) that schemaLeafKeysInto also calls,
    so the skip-guard and tag-parse rule lives in one place. Low severity — both are
    test-only, in one file, and small — but it is a genuine same-concept reimplementation
    worth a one-helper consolidation.

SUMMARY: One medium-impact duplication — the 25-pair (level, key) expected census is
  hand-copied across metadata_test.go and metadata_drift_test.go, a dual hand-edit /
  drift surface that can be collapsed to one shared census without weakening the
  deliberately-independent bijection sides. Two low-impact items: the rowKey/index-map
  helper re-authored across the config and setupguide test packages, and tomlTagsOf
  re-implementing the toml-tag walk already in schemaLeafKeysInto. Production code
  (setupguide.go, metadata.go, initgen.go, setup.go) is cleanly single-sourced — the
  SoT is the sole metadata carrier and the cmd-layer help/parse idiom intentionally
  mirrors the pre-existing verb pattern.
