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

  Discussion Map — Release-Notes Quality (8 subtopics · 8 pending)

  ├─ ○ Ingest commit data at all? + cooperative weighting [pending]
  ├─ ○ Which commit signal is highest-value [pending]
  ├─ ○ Graceful degradation — detection & default posture [pending]
  ├─ ○ Structural headline hint (thread E) [pending]
  ├─ ○ Token-budget interaction [pending]
  ├─ ○ diff_exclude granularity (minor) [pending]
  ├─ ○ L1 output shape — the connective tissue [pending]
  └─ ○ Tag-range vs release scope [pending]

---

*Subtopics are documented below as they reach `decided` or accumulate enough exploration to capture.*

---

## Summary

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Nothing decided yet — discussion just initialized from research handoff.
