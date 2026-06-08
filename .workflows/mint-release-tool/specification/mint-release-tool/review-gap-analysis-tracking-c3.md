---
status: complete
created: 2026-06-08
cycle: 3
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

*(Cycle 3. Prior cycles' 17 findings applied and excluded from scope. Four genuinely new internal gaps surfaced, all in the regenerate **write** path. finding_gate_mode = auto. All four resolved together in a single new "Write path (commit, push & recovery)" subsection of the Regenerate section, derived consistently from the forward path's principles — none were genuine design forks.)*

## Findings

### 1. Regenerate write-path commit graph & push are undefined (commit message, count, atomicity)

**Priority**: Important
**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Stage 5 — Commit graph; Regenerate — Preflight subset; What's mutable

**Details**:
The only commit-graph spec is the forward path (hook commit + `{commit_prefix} Release {tag}` + tag + `--atomic` push). Regenerate cuts no tag, so push form, commit subject, and commit count were all unstated.

**Proposed Addition**:
"Write path" subsection: ≤1 CHANGELOG commit (no hook artifacts); subject `docs(changelog): regenerate notes for {tag}` (single) / `docs(changelog): regenerate release notes` (`--all`); plain `git push origin HEAD` (no tag → not `--atomic`).

**Resolution**: Approved
**Notes**:

---

### 2. Regenerate write-path recovery semantics (pre-push failure / abort) are undefined

**Priority**: Important
**Source**: Specification analysis
**Affects**: Stages 6–7 — Failure model; Interactive Review — `n` abort; Regenerate — fresh review gate

**Details**:
The auto-unwind/PONR model is forward-only ("deletes the tag it made"). Regenerate makes a local changelog commit before pushing and runs the gate, but its abort/pre-push recovery, its PONR, and `--target both` partial-failure were unstated.

**Proposed Addition**:
"Write path" subsection: changelog `git push` is the PONR; abort/pre-push failure → reset the local changelog commit to clean state (no tag involved); provider create/update failure after the push → warn-only (re-heal with `--target release`); `--target both` is not atomic — changelog first (commit+push), then provider; provider failure after changelog push is warn-only, not rollback.

**Resolution**: Approved
**Notes**:

---

### 3. `regenerate --target release`: Create-vs-Update selection (and missing/existing-release edge) undefined

**Priority**: Important
**Source**: Specification analysis
**Affects**: Stages 6–7 — Publisher interface; Regenerate — composition table; What's mutable

**Details**:
`Publisher` exposes both `CreateRelease`/`UpdateRelease`; the composition table needs both (mass-heal missing = create; refresh existing = update) but never says how mint chooses, nor the missing/existing edges.

**Proposed Addition**:
"Write path" subsection: mint probes whether a provider release exists at `{tag}` → `UpdateRelease` if present, `CreateRelease` if absent. Automatic and per-version; user never picks; `--all` transparently mixes creates and updates.

**Resolution**: Approved
**Notes**: Resolves both edges (update-requested-but-missing → create; create-requested-but-exists → update) via a single existence-probe rule.

---

### 4. `regenerate --all` CHANGELOG rebuild strategy & commit/push batching undefined

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Regenerate — Batch `--all` semantics; Stage 5 — Changelog idempotency

**Details**:
"Rebuild `CHANGELOG.md`" (whole-file) vs idempotent in-place section-replace is ambiguous for `--all`; and whether the batch makes one commit/push at the end or N (one per version) was unstated.

**Proposed Addition**:
"Write path" subsection: `--all` → whole-file rebuild (preamble + every section, newest-on-top, repairs drift); single-version → idempotent in-place replace (per Stage 5); batch makes ONE CHANGELOG commit + push at the end (after all per-version notes generated/reviewed), not one per version.

**Resolution**: Approved
**Notes**:

---
