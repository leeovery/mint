---
status: in-progress
created: 2026-06-13
cycle: 1
phase: Input Review
topic: ai-model-selection
---

# Review Tracking: AI Model Selection - Input Review

## Findings

### 1. Haiku rejection — the rationale that collapsed the model space

**Source**: discussion `## Per-Verb Model Differentiation > ### Journey` (lines 72) and `### Summary > Key Insights` (line 177)
**Category**: Enhancement to existing topic
**Affects**: "Pinned default model" section (specifically the "Default model is Sonnet" bullet)

**Details**:
The spec states the default is Sonnet ("strong enough for the salience-heavy notes task … Opus is reserved for explicit per-verb opt-in") but omits *why the model space collapsed to Sonnet in the first place*. The discussion's "first collapse" records that Haiku was ruled out by user preference for both verbs, eliminating the "Sonnet release / Haiku commit" pairing and reducing the 2×2 to a single fork (releases on Sonnet vs Opus). It also captures the honest technical read: Haiku is "probably fine for the *bounded* commit task, wrong for the *salience-synthesis* notes task — but moot once the user rejected it outright."

This matters because the Sonnet-everywhere choice looks arbitrary without it. During planning someone may reasonably ask "why not Haiku for the cheap, bounded commit task?" — a question the discussion already answered (user preference, decided), and the spec's hard rule against re-litigating settled decisions means this rationale should be on record. It is decision-foundation context, not padding.

**Current**:
- **Default model is Sonnet.** Sonnet is strong enough for the salience-heavy notes task and comfortably inside the per-attempt deadline. Opus is reserved for explicit per-verb opt-in — never the shipped default.

**Proposed Addition**:
(leave blank until discussed)

**Resolution**: Pending
**Notes**:

---

### 2. "Experimentation is not the justification" — what actually warrants per-verb config

**Source**: discussion `## Per-Verb Model Differentiation > ### Journey` (line 73) and `### Summary > Key Insights` item 2 (line 178)
**Category**: Enhancement to existing topic
**Affects**: "Config schema: per-verb `ai_command` override" section (the "Why top-level shared baseline + optional per-verb override" rationale block) — or the Overview's justification for promoting per-verb override

**Details**:
The spec explains *why a shared baseline + optional override* over verb-only defaults, but it does not capture the prior, more fundamental point: *why per-verb config is warranted at all*. The discussion is explicit that "try them and see" experimentation is **already possible today** with the single shared `ai_command` (edit one key, cut a few commits), so wanting to experiment does NOT by itself justify per-verb config. The real justification is wanting *different* models pinned *simultaneously* (a different AI / command per verb). This is flagged as a top-level Key Insight in the discussion summary.

This matters because without it the spec's case for the whole feature is incomplete — a reviewer could argue the override is unnecessary ("you can already swap the one key"). The discussion pre-empts exactly that objection, and the answer should survive into the spec.

**Current**:
**Why top-level shared baseline + optional per-verb override (not verb-only defaults):**

- "Set once for all verbs" is the common case — one model for both; per-verb is the exception. Verb-only would force editing each verb's key.
- `ai_command` is already a top-level shared engine key; per-verb override is purely additive — no churn to the "shared keys at top" principle.
- Single source of truth: verb-only defaults would bake the shipped default once per verb (more duplication); a single top-level shipped default keeps it canonical.

**Proposed Addition**:
(leave blank until discussed)

**Resolution**: Pending
**Notes**:

---

### 3. Driver-pattern revisit condition (dropped, not deferred — but with a documented re-entry trigger)

**Source**: discussion `## Driver-Based AI Config — Dropped > ### Decision` (line 124)
**Category**: Enhancement to existing topic
**Affects**: "Scope boundaries (non-goals)" section — the "No driver / provider-registry pattern" bullet

**Details**:
The spec correctly records the driver pattern as "explicitly dropped (not deferred)" and that the raw command string delivers the generality. But the discussion qualifies the drop with a precise, recorded revisit condition that the spec omits: the driver's only residual value is *ergonomics* (typing `haiku` vs the full command), which doesn't justify the cost today, and it should be revisited "only as sugar over the command string if a future user juggles several AIs frequently."

This matters because "dropped" without the narrow re-entry trigger reads as a harder no than the discussion reached. Capturing the residual-value-is-ergonomics framing and the specific revisit condition prevents a future reader from either reopening the driver question from scratch or treating it as permanently forbidden. It bounds the non-goal precisely.

**Current**:
- **No driver / provider-registry pattern.** Configuring "which AI" plus a model alias, with mint knowing how to invoke each AI, is explicitly dropped (not deferred) — a raw per-verb command string already delivers the multi-AI/multi-command generality with less machinery.

**Proposed Addition**:
(leave blank until discussed)

**Resolution**: Pending
**Notes**:

---
