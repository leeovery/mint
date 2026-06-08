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

  Discussion Map — Release-Notes Quality (9 subtopics — 5 decided · 4 pending)

  ├─ ✓ Ingest commit data at all? + cooperative weighting [decided]
  ├─ ✓ Which commit signal is highest-value [decided — moot via cascade]
  ├─ ✓ Graceful degradation — detection & default posture [decided — moot via cascade]
  ├─ ✓ Quality convention anchor (Keep a Changelog) [decided]
  ├─ ✓ Salience preamble — diff-derived structural map [decided]
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

**Residual — now closed.** Earlier the user was neutral on passing commits into L1 raw ("we can take it in… if we choose to"). On reflection the user closed it: **don't use commits at all — it was a false path.** No commit text enters L1 in any form. This removes the residual GIGO/precedence concern (review F6): with zero commit text in the AI's context, there's no unlabelled signal that could compete with the diff, so no precedence-framing prose is needed.

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

**Unit of entry = the notable item, not the file/hunk/change/commit (resolves review F1).** The AI reads the whole diff, extracts *notable items*, and files each under its category. A diff that adds a feature *and* fixes a bug yields two items in two sections — no "force one change into one bucket" problem, because that was never the unit. Multi-category coverage falls out naturally. The only residual ambiguity is when the AI can't tell *what* a change does — a description problem, not a classification problem, and inherent to any approach.

Refinements that fall out:
- **mint's TL;DR one-liner is retained**, sitting *above* the categorized sections. KaC has no equivalent; it's mint's genuine value-add — **cross-*change* narrative synthesis from the whole diff**. (Clarified per review F8: the value is the AI reading the *entire release as one picture* and writing a unified story — independent of how it was committed. This is what beats regex tools, which render one-line-per-commit and structurally can't see the whole. It needs the complete diff, which it has; it never needed commit history. "Cross-commit" was loose wording for "cross-change.")
- **Diff-inferability tiers the categories.** `Added / Changed / Fixed / Removed` are readable from a diff. `Deprecated` and `Security` are intent-laden and often invisible in a raw diff → kept in the vocabulary but treated as **opportunistic** (emit only on real signal), never forced. Empty sections omitted entirely (KaC principle).
- **One generated payload, two sinks** (resolves the changelog-vs-release-note question): the per-release entry is identical; `CHANGELOG.md` accumulates entries under SemVer version headers per KaC's file structure, while the tag/GitHub release note surfaces the single entry. Same convention governs both.

**Version number — out of scope, settled upstream (review F2).** The SemVer bump is *not* AI-decided. `mint release` defaults to patch; the user passes a flag (patch/minor/major) to override. AI-managed SemVer is explicitly out of scope. Consequence for this topic: the version number is always known *before* notes generation, so the `CHANGELOG.md` version header is a given input, not something the notes pipeline computes. Dropping commit-intent therefore costs nothing on the versioning axis — that signal was never going to drive the bump here.

### Confidence

Medium-high. Per the user's stance ("take a stance and adjust as we go"), the taxonomy/principles are firm; the exact emoji↔category mapping and prompt wording are explicitly ship-and-refine.

---

## Salience preamble — diff-derived structural map

### Context

The "make the diff legible" lever (avenue #1). Since commit-intent is gone, salience must come from the diff itself. A raw unified diff is anti-salient — a 3-line tweak and a 400-line subsystem both render as "a hunk." The fix is a computed **Change Map** prepended to the AI's input, telling it what to prioritize.

### Decision — adopt a directory-rollup "Change Map", structural novelty weighted above magnitude

L1 computes a Change Map (cheap git commands) and prepends it to the AI input. Components:

- **Structural novelty (primary signal):** new / removed / renamed paths — *especially new directories or packages appearing*. "A whole new `auth/` package showed up" is the strongest language-agnostic headline signal there is, and it's qualitatively different from churn: a new subsystem is a headline even at modest line count, whereas a large refactor of existing code may not be. This is weighted **above** raw magnitude in both ordering and how the prompt is told to read it.
- **Magnitude (secondary signal):** per-area churn ranking, as supporting context ("400 lines here, 3 there").

Design choices:
- **Granularity — directory/area rollup by default**, with individually-notable files called out (new top-level entries, the single largest file). A flat list of every changed file *is* mush on big releases — the exact case this targets — so it just relocates the noise. Rollup is the salience-preserving form.
- **Computed after `diff_exclude`.** The user confirms bulk noise (large planning docs, generated artifacts) is already handled by mint's existing exclude config, so post-exclude magnitude is largely trustworthy. The B-depends-on-A ordering from research holds: the Change Map runs *after* exclusion, never before.
- **Prompt discipline (carries over diff-always-wins):** the prompt says **rank** importance using the Change Map, but **describe** changes from the diff. The map is salience *metadata*, not content — the AI must never narrate a file as a feature merely because it's large or new.

### Why structural novelty still leads even with `diff_exclude` doing the heavy lifting

The user noted `diff_exclude` removes most noise, which makes magnitude more trustworthy — but novelty still leads, for a reason independent of noise: a refactor and a new feature can have identical (post-exclude, real-source) churn, yet only one is the headline. "New structure appeared" captures the headline axis that line-count fundamentally cannot.

### Confidence

Medium-high; ship-and-refine. Exact Change Map formatting and prompt wording are tuning knobs.

---

## Summary

### Key Insights

1. **The reliable enrichment signals are the ones derived from the diff itself.** Dropping commits didn't shrink the option space so much as clarify it — structural novelty and magnitude inherit the diff's trustworthiness; commit-intent never could.
2. **Salience, not data volume, is the lever.** The motivating failure is misallocated attention, not missing data; feeding *more* raw diff makes it worse. Every decision here adds *structure/signal*, not *more text*.
3. **Anchoring to Keep a Changelog gives a principled quality bar for free** — and its "a changelog is not a commit log / for humans" thesis independently validates both the AI-narrative layer and the commit-rejection.

### Open Threads

- Five-then-four pending subtopics on the map: noise deprioritisation, hierarchical summarisation / token budget, L1 output shape, tag-range vs release scope.
- Background review (set 001) raised 7 gaps + 2 questions — being worked through one at a time.

### Current State

- **Decided:** no commit-intent ingestion; quality anchored on Keep a Changelog (their taxonomy, mint's skin); diff-derived Change Map salience preamble (novelty > magnitude, directory rollup, post-`diff_exclude`).
- **Open:** noise deprioritisation tier, big-diff handling / token budget, the L1 composite output shape, tag-range scoping, and the review findings.

### Key Insights

*(captured as the discussion progresses)*

### Open Threads

*(captured as the discussion progresses)*

### Current State

- Nothing decided yet — discussion just initialized from research handoff.
