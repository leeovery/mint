---
status: in-progress
created: 2026-06-08
cycle: 3
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

*(Cycle 3. Prior cycles' 17 findings applied and excluded from scope. Four genuinely new internal gaps surfaced, all in the regenerate **write** path — the spec specifies the forward path's commit/push/recovery mechanics in detail but leaves the regenerate path's equivalents implicit.)*

## Findings

### 1. Regenerate write-path commit graph & push are undefined (commit message, count, atomicity)

**Priority**: Important
**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Stage 5 — Commit graph (lines 389–394); Regenerate — Preflight subset per verb (line 548); Regenerate — What's mutable (line 493)

**Details**:
`regenerate --fresh --target changelog`/`both` "commits + pushes" (line 548 explicitly lists clean-tree/branch/remote-sync gates *because* it mutates the tree). But the **only** commit-graph specification in the spec (Stage 5, lines 389–394) describes the **forward** path: a hook-artifact commit + a release-bookkeeping commit subject `{commit_prefix} Release {tag}` + an annotated tag + `git push --atomic origin HEAD {tag}`. None of that maps cleanly to regenerate:

- **No tag is cut** on regenerate (tags are immutable, untouched), so the forward path's `--atomic … {tag}` push form doesn't apply — yet the spec never states the regenerate push form (plain `git push origin HEAD`?).
- **The commit subject is unspecified.** Reusing `{commit_prefix} Release {tag}` would be misleading (nothing was released this run); a `docs(changelog): regenerate {tag}`-style message is the natural analogue but is never stated. An implementer must invent the message.
- **Commit count / batching is unstated.** Regenerate has no hook-artifact commit (hooks don't run), so it's at most one CHANGELOG commit — but this isn't said, leaving the implementer to infer it from the forward path.

The forward path got a precise four-step commit graph; the regenerate write path — which genuinely mutates and pushes — got none. An implementer would have to design the commit message, push invocation, and commit count from scratch.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. Regenerate write-path recovery semantics (pre-push failure / abort) are undefined

**Priority**: Important
**Source**: Specification analysis
**Affects**: Stages 6–7 — Failure model (lines 414–420); Interactive Review — `n` abort (line 452); Regenerate — fresh runs the review gate (line 531)

**Details**:
The failure/auto-unwind model (lines 414–420, 452) is written entirely around the forward path: "deletes the tag it made, resets the release commit(s)," PONR = `git push --atomic`. For `regenerate --fresh --target changelog`/`both`, the run makes a **local CHANGELOG commit before pushing** and runs the **notes-review gate** (line 531) — so the same pre-push window exists, but the spec never states:

- What happens on **abort at the regenerate review gate** (the `n` path) after the local changelog commit is made — does mint reset that commit back to the clean starting state, mirroring the forward auto-unwind? (The forward unwind text references "the tag it made," which regenerate never makes, so the rule doesn't transfer verbatim.)
- What the **point of no return** is for regenerate's changelog push (presumably the `git push` of the changelog commit), and whether a **provider-release update failure** after that push is warn-only (consistent with forward Stage 7) or unwinds.
- Whether `--target both` is **ordered/atomic** across its two surfaces (changelog commit+push *and* provider update): if the changelog push succeeds but the provider update fails, is that warn-only (heal later) or a partial-failure the user must reconcile?

Without this, an implementer can't tell whether regenerate is recoverable like the forward path or fire-and-forget, and `--target both`'s partial-failure behaviour is fully open.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. `regenerate --target release`: Create-vs-Update selection (and missing/existing-release edge) undefined

**Priority**: Important
**Source**: Specification analysis
**Affects**: Stages 6–7 — Publisher interface (line 427); Regenerate — composition table (lines 514–519); What's mutable (line 493)

**Details**:
The `Publisher` interface exposes **both** `CreateRelease` and `UpdateRelease` (line 427), and the regenerate composition table needs both behaviours: "Mass-heal missing provider releases" (line 518) implies the provider release **does not exist yet** (→ create), while "Refresh public release text only" (line 516) implies it **already exists** (→ update). But the spec never states **how regenerate chooses** between create and update for a given `--target release` run, nor the edge behaviours:

- For a single `regenerate <ver> --target release`, does mint probe whether a provider release exists at `{tag}` and pick create-or-update accordingly? That existence check is the load-bearing decision and is unspecified.
- **Update requested but no release exists** (the heal case described as "missing") — does mint fall back to create, or error?
- **Create requested but a release already exists** — overwrite/update, or error?
- In `--all` mode, the batch will mix versions that have a release (update) with versions that don't (create); the per-version create/update resolution must be automatic, which reinforces the need for the existence-probe rule.

An implementer can't wire `CreateRelease`/`UpdateRelease` dispatch without this rule, and the missing/existing edges would each be a guessed design decision.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. `regenerate --all` CHANGELOG rebuild strategy & commit/push batching undefined

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Regenerate — Batch `--all` semantics (lines 538–541); Stage 5 — Changelog idempotency (line 379)

**Details**:
`--all` orders oldest→newest "to rebuild `CHANGELOG.md` in natural order" (line 538), and the changelog rule is "idempotent by version key — replaces a section in place" (line 379). For `--all --target changelog`/`both` these two facts leave the rebuild *mechanics* ambiguous:

- **Whole-file rebuild vs incremental section-replace.** "Rebuild `CHANGELOG.md`" (lines 493, 538) suggests regenerating the entire file from scratch (header preamble + all sections newest-on-top); the idempotent-replace rule suggests editing one section per version in place. These produce different results when the existing file has stray/extra sections, ordering drift, or a non-standard preamble. Which strategy governs `--all` is unstated.
- **Commit/push batching across the batch.** With per-version review gates (line 540) and skip-and-continue (line 539), is there **one** CHANGELOG commit+push at the **end** of the batch, or **N** commits/pushes (one per version)? Per-version pushes for a 30-release backfill would be very noisy and interact awkwardly with mid-batch skips; an end-of-batch single commit is the natural reading but is never stated. (This also feeds finding #1's commit-message question — a batch commit subject differs from a single-version one.)

Without a stated rebuild strategy and batch-commit policy, an implementer must choose, and the choice visibly affects the committed CHANGELOG and history.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
