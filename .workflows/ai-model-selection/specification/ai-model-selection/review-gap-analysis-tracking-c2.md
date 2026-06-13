---
status: complete
created: 2026-06-13
cycle: 2
phase: Gap Analysis
topic: ai-model-selection
---

# Review Tracking: ai-model-selection - Gap Analysis

## Findings

### 1. `ai.Config.Timeout` cannot carry the "no deadline" vs "wiring forgot to set it" distinction — boundary representation unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Resolution value semantics (the `timeout` bullets, esp. the conditional-deadline mechanics), Single source of truth for config defaults ("The transport carries no defaults"), Migration & mechanical carry-overs (Transport wiring sites)

**Details**:
The spec settles the transport's *runtime* branch — `== 0` ⇒ skip `context.WithTimeout`; positive ⇒ `WithTimeout` with that value — and asserts "Config guarantees the transport receives only a positive value or an explicit `0` ... so no negative reaches the transport." But it never specifies *how the value crosses the config→transport boundary*, and the as-built boundary type makes this load-bearing.

Today `ai.Config.Timeout` is a plain `time.Duration` (`internal/ai/transport.go`), and all three wiring sites construct `ai.Config{AICommand: cfg.AICommand}` with `Timeout` left at its zero value — relying precisely on the transport's `timeout <= 0 → 60s` self-default that this work *deletes*. After deletion, a `time.Duration` zero is ambiguous: it is both the operator's explicit "no deadline" (which the spec says must be honored) AND the value a wiring site produces if it forgets to thread the resolved timeout. Once the self-default is gone, a forgotten field silently becomes "run unbounded" — a direct inversion of "fail loud, never hang," reached by omission rather than operator choice.

An implementer must therefore decide, with no spec guidance:
- whether `ai.Config.Timeout` changes type (e.g. `*time.Duration`, or a small wrapper) so absent is distinguishable from explicit-zero at the boundary, OR
- whether the contract is "all three sites MUST populate Timeout from `cfg.TimeoutFor(verb)` and zero is trusted as no-deadline," with that always-populate requirement made explicit and test-pinned.

This is a different question from cycle 1's transport conditional-deadline mechanics (which assumed a resolved value already in hand) and from the config-side absent-vs-zero pointer decoding (which lives entirely inside `config`). The unaddressed seam is specifically the `config.Config → ai.Config` mapping the spec itself calls out ("consumers map `config.Config` → `ai.Config`") — for `timeout` that mapping's safety is undefined.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Approved
**Notes**: Applied to specification (auto-approved, cycle 2).

---

### 2. Transport doc comments encode the deleted `Timeout <= 0 → default` contract — same-change update is in scope but unstated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Single source of truth for config defaults ("The transport carries no defaults"; `defaultAICommand` deletion), Resolution value semantics (transport learns `timeout = 0` ⇒ no deadline)

**Details**:
The spec enumerates the *code* deletions in the transport (`defaultAICommand` const removed; the empty→re-default path removed; `Timeout <= 0` split into `== 0` no-deadline vs positive) but does not flag that the transport's own WHY-comments hard-encode the about-to-be-false contract — and CLAUDE.md requires comments stay true to as-built *in the same change*. This is the exact class of obligation the spec already makes explicit for the `Commit` struct doc comment in the Cross-spec reconciliation section; the symmetric transport-side comments are not enumerated.

Concretely, `internal/ai/transport.go` currently states, verbatim:
- on `Config.AICommand`: "(default `claude -p` when empty)";
- on `Config.Timeout`: "A zero or negative Timeout falls back to the production default.";
- on `NewTransport`: "An empty AICommand resolves to `claude -p` and a non-positive Timeout resolves to the ~60s production default, so the zero Config yields a fully working production transport.";
- on `Generate`/`attempt`: "Each attempt gets its own deadline via context.WithTimeout(ctx, t.timeout)" — which becomes conditional once `timeout = 0` skips `WithTimeout`.

After this work each of these contradicts shipped behavior. Without enumerating them, an implementer scoping the transport edit could change the code and leave stale, now-false contract comments — the precise CLAUDE.md violation the spec is careful to forbid elsewhere. Listing them (as the Migration section lists the test-pin breakages) keeps the comment update from being rediscovered or missed.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Approved
**Notes**: Applied to specification (auto-approved, cycle 2).

---
