---
status: in-progress
created: 2026-06-08
cycle: 2
phase: Input Review
topic: cli-presentation
---

# Review Tracking: cli-presentation - Input Review

## Findings

### 1. Notes-body-verbatim rule ties to an engine invariant ("what previews is what ships")

**Source**: discussion/cli-presentation.md, "Decisions locked (plain layer)" — line 281 ("it would contradict the engine's 'use the body whole' rule and break 'what previews is what ships'. The few extra tokens are negligible.")

**Category**: Enhancement to existing topic
**Affects**: "The Plain Layer" section (notes-body-verbatim bullet, spec line 203); secondarily reinforces the same byte-identical claim in the intro paragraph (spec line 195).

**Details**:
The spec states the *what* — "Notes body verbatim … emoji headers shown if present … No stripping/transforming" — but drops the *why* the discussion gave: stripping/transforming would (a) contradict the engine's "use the body whole" rule and (b) break the "what previews is what ships" invariant. This is an integration-point rationale, not mere provenance: it pins the presentation rule to an engine guarantee, explaining why the rule is non-negotiable rather than a stylistic preference. The discussion also explicitly judged the token cost ("the few extra tokens are negligible") — relevant given plain mode's stated token-efficiency goal, since emoji headers superficially cut against it. Capturing this prevents a future reader from "optimising" plain mode by stripping emoji headers and silently breaking preview/ship parity.

**Current**:
> - **Notes body verbatim** — the same bytes as pretty/tag/changelog/release, **emoji headers shown if present** (`✨ Features`, `🐛 Fixes`). No stripping/transforming.

**Proposed Addition**:
_(leave blank until discussed)_

**Resolution**: Pending
**Notes**:

---
