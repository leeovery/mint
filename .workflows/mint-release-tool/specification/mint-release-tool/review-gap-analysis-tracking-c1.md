---
status: in-progress
created: 2026-06-08
cycle: 1
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

## Findings

### 1. Abort key inconsistency: `[n]` vs `q`

**Priority**: Minor
**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Confirmation & Notes Review (gate menu line 433); Stages 6–7 — Failure model (line 406)

**Details**:
The interactive gate menu defines the abort choice as `[n] abort` (line 433) and the prose at line 437 consistently calls it "`n` abort". But the Stages 6–7 failure-model section (line 406) says "The auto-unwind is the same path a user `q`/abort at the review gate takes". The key `q` appears nowhere else and is not in the menu. An implementer reading the failure-model section could wire a `q` key that the gate section never defines. The two sections disagree on the abort key letter.

**Proposed Addition**:
Reworded the Stages 6–7 line to "the same path taken when the user answers **no** at the `Continue?` review gate" — frames abort as the "no" answer rather than inventing an abort key.

**Resolution**: Approved
**Notes**: Per user — the gate is a Continue? question answered yes/no (+ edit/regenerate); abort is "no", not a `q`/`n`-abort compound.

---

### 2. `MINT_BUMP` value undefined for `--set-version` releases

**Priority**: Important
**Source**: Specification analysis
**Affects**: Hooks — Invocation & context env-var table (lines 196, 198); Stage 1 — `--set-version` rules

**Details**:
The injected `MINT_BUMP` env var is documented with values `patch`/`minor`/`major` only (line 196). But `--set-version X.Y.Z` is a first-class path that produces a release without any bump flag. The spec never states what `MINT_BUMP` is set to in that case (e.g. `explicit`, `set`, empty, or the closest-equivalent computed bump). A hook author keying behaviour off `MINT_BUMP` would have to guess, and an implementer must invent a value. The same gap touches the dry-run env injection. Needs an explicit value (or "unset") for the `--set-version` case.

**Proposed Addition**:
Env-var table: `MINT_BUMP` values become `patch` / `minor` / `major` / `explicit`, with `explicit` set when `--set-version` was used.

**Resolution**: Approved
**Notes**:

---

### 3. `provider` config: unknown value — fail-loud vs silent downgrade contradiction

**Priority**: Important
**Source**: Specification analysis
**Affects**: Stages 6–7 — Publishing: provider driver abstraction (line 415); Config Format & Schema — Typed validation (line 577)

**Details**:
Two rules collide. The publishing section says "An unknown/unsupported provider → tag + push only" (line 415) — i.e. silently downgrade to no-publish. The config section says config has "Typed validation, fail-loud on unknown keys / bad types, with clear messages" (line 577). An unknown `provider` *value* (e.g. `provider = "gitlab"` when only GitHub is implemented) sits between these: is it a fail-loud validation error, or a silent downgrade to tag+push-only? The two readings produce opposite behaviour (release aborts vs release ships without a GitHub release). This is especially consequential because a typo in `provider` would silently skip publishing rather than erroring. Implementers need one rule.

**Proposed Addition**:
Recognised `provider` key with an unsupported value → warn loudly + downgrade to tag+push-only (publish skipped), never silent. Fail-loud config validation remains for unknown keys / bad types.

**Resolution**: Approved
**Notes**: Reconciles the two source rules; user accepted the warn+downgrade over a hard abort.

---

### 4. Regenerate `<version>` argument format and base-diff resolution undefined

**Priority**: Important
**Source**: Specification analysis
**Affects**: Regenerate / Backfill Notes — Two-axis contract, fresh path (line 486); CLI Surface — regenerate command

**Details**:
The fresh regenerate path "re-diffs `vX-1..vX`" (lines 477, 486) but the spec never defines (a) whether the `<version>` CLI argument is given with or without the `tag_prefix` (e.g. `regenerate v1.4.0` vs `regenerate 1.4.0`), (b) how mint resolves `vX-1` — the previous tag — when the target is the **oldest** release (no `vX-1` exists), and (c) what happens for a `<version>` that has no matching tag at all. The forward path solves the no-prior-tag case explicitly ("Initial release.", line 233), but the regenerate fresh path does not state the analogous rule for the oldest version in a `--all` backfill or a single regenerate of the first release. An implementer would have to guess the base for the earliest version and the argument's prefix handling.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. Cache TTL value unspecified

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Dry-Run — Dry-run note caching, Location (line 467)

**Details**:
The dry-run note cache has "a **short TTL** backstop so a stale preview can't resurrect" (line 467) but no concrete value or order of magnitude is given. Unlike `max_diff_lines` (50000) and the AI timeout (~60s), which carry defaults, the TTL is left fully open. An implementer must invent a duration, and too-short defeats the motivating workflow (dry-run then real run) while too-long defeats the staleness backstop. A default (even an approximate one, consistent with "~60s"-style guidance) is needed.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. AI timeout vs "retry once" interaction unspecified

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 4 — AI Release Notes: Engine layering / transport (line 226), Engine timeout (line 238), Failure behaviour (line 282), Validation (line 289)

**Details**:
The transport "retries once" on a bad/empty/error/refusal generation (lines 226, 289). Separately, a hung call is bounded by a "timeout (~60s)" (line 238). The `on_notes_failure` list includes "timeout" as a failure cause (line 282). It is not stated whether a **timeout** is one of the conditions that triggers the single automatic retry, or whether a timeout bypasses the retry and goes straight to `on_notes_failure`. This affects worst-case release latency (one 60s timeout vs two = 120s) and the retry implementation. Needs a one-line clarification of whether timeout is retryable.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. Forward-path changelog idempotency is unreachable given the tag-free preflight gate

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 5 — Changelog mechanics: idempotency (line 366); Stage 2 — Target tag is free (line 126)

**Details**:
The changelog rule states it is "Idempotent by version key — a re-run or amended release for an *existing* version replaces that version's section in place rather than appending a duplicate" (line 366). But on the forward `mint release` path, Stage 2's tag-free gate (line 126) aborts before Record if the target tag already exists, so the forward path can never reach Record with an "existing version". The idempotency rule therefore only applies to the regenerate path. As written it reads as a forward-path Record behaviour, which is contradicted by the preflight gate. Either clarify that idempotent-replace is a regenerate-path behaviour, or note the specific forward-path scenario (if any) where Record sees an existing version section. Without this an implementer may build forward-path replace logic that is dead code, or be confused about which path owns it.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. `--autostash` restore ordering relative to auto-unwind not specified

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 2 — Clean working tree / `--autostash` (line 125); Stages 6–7 — Failure model auto-unwind (line 402); Interactive Review — abort (line 437)

**Details**:
`--autostash` "stashes before the run and restores after, including on abort/failure" (line 125). The auto-unwind "resets the release commit(s)" and returns the repo "to the exact clean starting state" (line 402, 437). The interaction order is undefined: on an abort/failure with `--autostash` active, does mint first unwind its own commits/tag, then pop the stash (restoring the user's WIP on top of the original clean state)? The order matters — popping before the reset, or resetting past the stash point, could either conflict or lose the WIP. Given the spec's strong "exact clean starting state" guarantee, the precise unwind-then-restore sequence (and behaviour if the stash pop conflicts) should be stated so an implementer doesn't guess and risk losing user work.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 9. First-release fixed body vs `on_notes_failure` / `--no-ai` / fallback interaction

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 4 — Diff base, first release (line 233); Failure behaviour (lines 282–284); Degenerate release (line 258)

**Details**:
The first release (no prior tag) "skips the AI and uses a fixed body, 'Initial release.'" (line 233). It is not stated how this fixed-body path interacts with the surrounding knobs: does `--no-ai` on a first release also yield "Initial release." (rather than the commit-subject fallback)? Does `on_notes_failure` ever apply to a first release (it shouldn't, since the AI isn't called)? And does the degenerate-diff stub (line 258) or the first-release fixed body win when a first release also has an empty/all-excluded diff? These are independent guards that can co-occur on the same run; their precedence is unstated. An implementer would have to pick an ordering.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 10. `regenerate --all both` with `changelog = false` — batch error vs documented skip-and-continue

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Regenerate — target/changelog-disabled error (line 491); Batch `--all` partial-failure semantics (line 515)

**Details**:
Two regenerate rules can interact ambiguously. A `--target changelog`/`both` when `changelog = false` is a fail-loud error (line 491). Batch `--all` uses "skip-and-continue, summarise at the end" rather than abort-the-batch (line 515). For `regenerate --all --target both` with `changelog = false`, it is unclear whether the changelog-disabled condition is a single up-front validation error that aborts the whole batch before it starts (config-level, not per-version), or whether it is treated as a per-version skip. Since `changelog = false` is a static config fact (not a per-version condition like a too-large diff), the natural reading is an up-front abort — but the spec's two rules don't say which wins. Worth one line to disambiguate config-level pre-validation from per-version skip-and-continue.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
