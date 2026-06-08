# Discussion: Release-Notes Quality

## Context

Can `mint release`'s AI-generated release notes be lifted in quality beyond what a raw textual diff alone allows — especially for large releases where big diffs "summarise to mush"? The research phase converged on a clear strategic picture; this discussion decides the open questions it deliberately left unresolved.

**What research settled:**
- The architectural home is already carved out: anything here lives inside **L1 (the context builder)** — it changes *what content is assembled*, not the AI engine (L2, git-unaware / content-agnostic), the prompt-ownership model, or the sinks.
- The motivating failure ("glosses over the big feature on big releases") is a **salience / narrative** failure, not a *missing-data* failure. Feeding more raw diff can make it worse.
- **The diff is the reliable backbone** — the one signal always true regardless of merge strategy or commit discipline. It stays primary.
- **Commit-intent is opportunistic, best-effort enrichment** — used only when a per-release quality check says the history is good enough. A conditional bonus, not a co-equal partner.
- **Precedence (decided in research):** *the diff always wins; commit-intent only adds framing — it can never assert a change the diff doesn't show.* Structurally prevents hallucinating features from abandoned/reverted commit messages.
- **AST/semantic enrichment (the original hypothesis) is de-prioritised** — least language-agnostic option; industry avoided it for a decade for mint's exact generality reason; near-useless for the markdown dogfood repo. mint must serve *every* repo (markdown, Go, Bash, SDKs).
- **mint's structural advantage:** it owns *both ends of the pipe* (`mint commit` authors Conventional Commits; `mint release` consumes them) — no competitor can. But this is conditional (only pays off where the user actually uses `mint commit` and keeps granular history).
- **Artifact noise** is largely closed by already-decided config (`diff_exclude` + inherent `.gitignore` exclusion).

### References

- [Research: release-notes-quality](../research/release-notes-quality.md) — converged strategic picture, prior art, the 8 open questions seeded below
- [Prior discussion: mint-release-tool](mint-release-tool.md) — first discussion; settled prompt/output side, `max_diff_lines`, diff-exclusion tiers, default format
- [Prior discussion: commit-command](commit-command.md) — `mint commit` → Conventional Commits; the "both ends of the pipe" authoring side
- git-cliff `git cliff --context` JSON schema — reference shape for structured commit-intent L1 might assemble (per-commit id/group/scope/body/footers/breaking/raw_message)

## Discussion Map

### States

- **pending** (`○`) — identified but not yet explored
- **exploring** (`◐`) — actively being discussed
- **converging** (`→`) — narrowing toward a decision
- **decided** (`✓`) — decision reached with rationale documented

### Map

  Discussion Map — Release-Notes Quality (9 subtopics — 4 decided · 5 pending)

  ├─ ✓ Ingest commit data at all? + cooperative weighting [decided]
  ├─ ✓ Which commit signal is highest-value [decided]
  ├─ ✓ Graceful degradation — detection & default posture [decided]
  ├─ ✓ Quality convention anchor (Keep a Changelog) [decided]
  ├─ ○ Salience preamble — diff-derived structural map [pending]
  ├─ ○ Noise deprioritisation (diff_exclude granularity) [pending]
  ├─ ○ Hierarchical summarisation for big diffs / token budget [pending]
  ├─ ○ L1 output shape — the connective tissue [pending]
  └─ ○ Tag-range vs release scope [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Ingest commit data at all? + cooperative weighting

### Context

Research's central convergence was: diff = backbone, commit-intent = opportunistic best-effort enrichment, gated by per-release degradation detection. The whole apparatus (Q2 commit-signal choice, Q3 degradation detection, half of Q7's L1 composite) hung off a "yes, ingest" answer to this gate.

### Decision — do NOT build commit-intent ingestion as a feature

The user rejects the commit-intent direction outright, on **value** grounds (not the correctness grounds research already neutralised):

1. **The final diff is the source of truth; commit history is the path, not the destination.** A commit may add code that a later commit removes — the final diff correctly shows neither. The path we took is largely noise relative to the outcome.
2. **Commit messages are unreliable and entirely user-controlled.** mint won't always author them (`mint commit` adoption is optional), so they may be hand-written or bare `WIP`. There's no floor on commit-message quality to build on.
3. **The conditional machinery isn't worth it.** Because the signal is unreliable, research had to make degradation-detection "central." That's a lot of complexity for a bonus that fires only on the subset of repos with clean granular history — and shrinks further as merge strategies (squash/rebase) collapse history.

**Residual (low-stakes, open):** commits *could* still be passed into L1 raw with **zero special handling** — no detection, no weighting, no degradation logic — and the diff-always-wins precedence rule already prevents hallucination. The user is neutral on this ("we can take it in; I don't think we need any special handling if we choose to"). Not a load-bearing inclusion; deferred as a trivial L1-shape detail, not a feature.

### Cascade

This collapses the commit-dependent open questions:
- **Q2 (which commit signal is highest-value)** — moot. No signal is being mined.
- **Q3 (graceful degradation / detection)** — moot *for commits*. There's no commit-quality signal to detect or degrade. (Degradation may still matter for diff-side concerns like oversized diffs — tracked under token-budget / mush handling, not here.)

The salience problem research identified is **still real** — it just has to be solved from the diff alone (see pivot to diff-derived enrichment).

---

## Quality convention anchor (Keep a Changelog)

### Context

With commit-ingestion dropped, the lever for quality is the **prompt + a thin diff-derived salience hint**. Rather than invent a house quality bar, anchor to an established convention so the bar is principled and the output looks professional/familiar. The user's instinct: "this must be a defined principle somewhere we could follow." It is — **Keep a Changelog** (keepachangelog.com), paired with **SemVer**.

### Why this convention fits unusually well

Its core principles are the same thesis as this whole epic:
- *"Changelogs are for humans, not machines."*
- *"A changelog is not a commit log"* — the exact line research quoted; it's what justifies mint's AI narrative layer existing, and independently validates dropping commit-ingestion.
- Notable changes grouped by a finite type taxonomy: **Added / Changed / Deprecated / Removed / Fixed / Security**.

### Decision — borrow the principles + taxonomy, keep mint's presentation

**"Their meaning, mint's skin."** The decision separates two things the headings question conflated:
- **Taxonomy (semantics)** — adopt Keep a Changelog's categories as the canonical bucket set. Rationale isn't aesthetic: a *fixed, standard* taxonomy forces the AI to classify every change, and classification is itself prioritization (helps the salience problem). KaC's set is battle-tested and universally recognised.
- **Presentation (skin)** — keep mint's emoji-headed style as the rendering of those categories (`✨ Added`, `🐛 Fixed`, `🔧 Changed`, `🗑️ Removed`, …). This *refines* the first discussion's "emoji-headed sections" decision (pins the taxonomy behind the emoji); it does not override it.

Refinements that fall out:
- **mint's TL;DR one-liner is retained**, sitting *above* the categorized sections. KaC has no equivalent; it's mint's genuine value-add — the cross-release narrative synthesis that's the whole reason an AI layer beats regex tools.
- **Diff-inferability tiers the categories.** `Added / Changed / Fixed / Removed` are readable from a diff. `Deprecated` and `Security` are intent-laden and often invisible in a raw diff → kept in the vocabulary but treated as **opportunistic** (emit only on real signal), never forced. Empty sections omitted entirely (KaC principle).
- **One generated payload, two sinks** (resolves the changelog-vs-release-note question): the per-release entry is identical; `CHANGELOG.md` accumulates entries under SemVer version headers per KaC's file structure, while the tag/GitHub release note surfaces the single entry. Same convention governs both.

### Confidence

Medium-high. Per the user's stance ("take a stance and adjust as we go"), the taxonomy/principles are firm; the exact emoji↔category mapping and prompt wording are explicitly ship-and-refine.

---

## Summary

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Nothing decided yet — discussion just initialized from research handoff.
