---
status: in-progress
created: 2026-06-16
cycle: 2
phase: Gap Analysis
topic: interactive-mint-init-setup
---

# Review Tracking: interactive-mint-init-setup - Gap Analysis

## Findings

### 1. Drift-test bijection contract is "every toml-tagged field" — but three toml-tagged fields are sub-table containers (`release`/`commit`/`hooks`), not leaf keys, and have no SoT row

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Drift test (the anti-drift enforcement)" → "What counts as one 'key' (the bijection contract)"

**Details**:
Cycle 1's finding #4 settled the *per-(level, key)* matching rule and the four-row model for the dual-level keys, and it pins the mechanism to "derive mechanically from the decode-shape structs' `toml` tags (`fileShape`, `releaseShape`, `commitShape`, `hooksShape`)." The resolution text states the bijection is **total**: "every `toml`-tagged schema field has exactly one matching SoT row at its level, and every SoT row maps back to a schema field."

That literal contract does not hold against the actual schema. Three `toml`-tagged fields in those exact structs are **sub-table container fields, not config keys**:
- `fileShape.Release` → `` `toml:"release"` `` (a `releaseShape`)
- `fileShape.Commit` → `` `toml:"commit"` `` (a `commitShape`)
- `releaseShape.Hooks` → `` `toml:"hooks"` `` (a `hooksShape`)

These satisfy the literal "every `toml`-tagged schema field" predicate, but they are the **level/table containers themselves** — there is no `release` / `commit` / `hooks` config key with a `key · level · default · description` SoT row (the SoT models the `[release]` / `[commit]` / `[release.hooks]` levels, not a row named "release"). A mechanical reflection that walks `toml` tags and demands "exactly one matching SoT row per tagged field" will, taken at face value, see three tagged fields (`release`, `commit`, `hooks`) with **no** matching SoT row and either (a) fail the bijection / break the build, or (b) force the implementer to invent an unstated carve-out ("skip struct-typed fields whose tag is a sub-table name / recurse into them instead of treating them as a key").

Which carve-out is correct is not obvious: the reflection helper must **recurse** into `Release`/`Commit`/`Hooks` to reach their leaf keys (with the nested struct's tag prepended as the `level`), AND must **not** emit `release`/`commit`/`hooks` as keys in their own right. That "recurse-don't-count" rule for sub-shape fields is exactly the kind of traversal decision the bijection contract is supposed to nail down (the contract is explicitly called "decided, not a planning detail"), yet it is the one structural case the current wording contradicts. Without stating it, the drift test — a core DoD deliverable that the whole SoT exists to enable — cannot be written to a passing total bijection without the implementer guessing the traversal.

Note this is the inverse direction from cycle 1 #4's concern: #4 handled keys that appear at *multiple* levels (one field → multiple conceptual rows / dual-level); this is fields that are *containers and not keys at all* (tagged field → **zero** rows, plus recurse).

**Current**:
> - The authoritative key set is **derived mechanically from the `config` decode-shape structs' `toml` tags** (`fileShape`, `releaseShape`, `commitShape`, `hooksShape`) — **not** a hand-maintained list — so the test catches a genuine schema↔SoT divergence rather than comparing two copies of the same hand-list. The bijection is total: every `toml`-tagged schema field has exactly one matching SoT row at its level, and every SoT row maps back to a schema field. (The exact reflection helper / package layout is a planning detail; the *contract* — derive from the schema, match per (level, key), bijective — is decided.)

**Proposed Addition**:
Qualify the "every `toml`-tagged schema field" statement so the bijection is over **leaf keys**, not table-container fields: the sub-shape container fields (`fileShape.Release`/`Commit` → `release`/`commit`, `releaseShape.Hooks` → `hooks`) are **traversed into (recursed), not counted as keys** — the nested struct's tag supplies the `level` (`[release]`, `[commit]`, `[release.hooks]`) for the leaf rows it contains, and no SoT row is expected for `release`/`commit`/`hooks` themselves. The total bijection then holds between **leaf `toml`-tagged scalar/collection fields** and SoT rows. (Exact reflection helper still a planning detail; the recurse-don't-count rule for sub-shape fields is part of the decided contract.)

**Resolution**: Pending
**Notes**: Grounded against internal/config/config.go: fileShape (lines 326-333) carries `Release`/`Commit` as `toml:"release"`/`toml:"commit"`; releaseShape (lines 353-369) carries `Hooks` as `toml:"hooks"`. These are the only three sub-shape container fields; all other tagged fields are leaf keys, so the carve-out is bounded and stable.

---
