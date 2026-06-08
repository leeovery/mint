# Discussion Consolidation Analysis

## Recommended Groupings

### Mint Release Tool *(reuses the existing anchored spec name)*
- **mint-release-tool**: the release engine + seven-stage lifecycle spine (version → preflight → hooks → notes → record → tag/push → publish), regenerate/heal, config schema, CLI surface, `mint init`. The backbone of the whole tool.
- **release-notes-quality**: the AI-notes quality lever — it explicitly lives *inside L1 (the context builder)* of the release-notes stage, changing only *what content is assembled* (Keep a Changelog taxonomy, the diff-derived Change Map salience preamble, degenerate-release stub, CHANGELOG accumulation mechanics). It is not independently buildable — it refines the release-notes stage that only exists in the release pipeline.

**Coupling**: Conceptual + data coupling to the release-notes stage. release-notes-quality has no deliverable of its own; it tunes the prompt and the L1 input of `mint release`'s notes generation, and its CHANGELOG accumulation decisions (no `[Unreleased]`, newest-on-top, idempotent-by-version) directly refine the main discussion's Record/sink decisions — the discussion itself flags this overlap as cross-cutting back into the main topic. Grouping it alone would be too thin; it belongs with the engine.

### CLI Presentation *(cross-cutting)*
- **cli-presentation**: the styled-but-restrained UI applied consistently across *every* verb (`release`, `regenerate`, `init`, `version`, future `commit`) — the event-oriented `Presenter` seam, pretty/plain render modes by `isatty(stdout)`/`--plain`, the `Continue?` review-gate rendering, `-y` orthogonality, library selection (lipgloss + standalone spinner).

**Coupling**: This is a pattern/architecture concern, not a feature deliverable — it defines a seam and a policy that *all* verbs render through. Per cross-cutting guidance it should be its own specification rather than folded into any single feature. It owes (and is owed) reconciliations with the other specs: it revises the engine's `[a]/[e]/[r]/[q]` gate to a default-yes `Continue?` (`y`/`n`/`e`/`r`) and introduces the global `--plain` flag, both of which the release spec must adopt.

### Commit Command
- **commit-command**: the `mint commit` verb — a thin standalone verb (does NOT ride the release spine) that consumes the shared, content-agnostic AI engine (the three-layer L1/L2/L3 split), the verb-namespaced TOML config, `git_safe`, and the `Presenter` seam. Staged-diff input with `-a`/`-A` staging deferred to gate-accept; Conventional Commits output with `$EDITOR` fallback; minimal preflight; opt-in `-p` push; mutate-nothing-until-accept / never-unwind-after.

**Coupling**: Behavioural reuse of shared primitives but a distinct, independently-plannable verb with its own lifecycle, prompt, staging model, and safety posture (the inverse of release's). Separate spec.

## Independent Discussions
- None — all four discussions slot into one of the three groupings above.

## Analysis Notes

**User context (decisive):** the in-progress `mint-release-tool` *specification* was begun when this work was a single **feature** (release only), before `cli-presentation`, `commit-command`, and `release-notes-quality` existed. It has since been **pivoted to an epic** mid-spec. The existing spec is therefore **stale** in specific, known ways and the three later discussions explicitly record the "spec hand-offs / reconciliation owed":
- **Config layout** — the spec documents the *flat* schema (`notes_context`, `notes_prompt`, top-level `[hooks]`). `commit-command` superseded this with a **verb-namespaced** shape: shared engine keys at top (`ai_command`, `diff_exclude`, `max_diff_lines`) + `[release]` / `[commit]` tables, hooks nested as `[release.hooks]`. The spec's current config section is now wrong.
- **Review-gate rendering** — the spec documents `[a] accept / [e] edit / [r] regenerate / [q] abort`. `cli-presentation` superseded the *rendering* with a default-yes `Continue?` (`y`/`n`/`e`/`r`, Enter ⇒ accept). Same four semantic choices, new rendering — the spec must adopt it and drop the stale `[a]`/`[q]` keys.
- **AI engine shape** — `commit-command` introduced the **three-layer split** (L1 context builder / L2 content-agnostic engine / L3 per-verb glue) so release and commit literally share L1 + L2. The release spec should express notes generation through this layering. `release-notes-quality` is built on the same boundary (its enrichment is an L1 change with zero L2 impact).
- **Presentation seam + `--plain`** — the spec has no `Presenter` seam and no `--plain` global flag; both come from `cli-presentation`.
- **Notes format/quality** — the spec's "emoji-headed sections" should be refined by `release-notes-quality` (Keep a Changelog taxonomy behind the emoji skin, Change Map salience preamble, degenerate-release stub).

**Implication for routing:** the existing "Mint Release Tool" spec should not be naively *continued* from its single stale source. Its grouping now spans `mint-release-tool` + `release-notes-quality`, and it must absorb the cross-discussion reconciliations above (some of which *invalidate* current spec content). Treat this as a regeneration/supersede candidate rather than an incremental top-up — the user anticipated exactly this.

**Dependency order (foundational → dependent):** `CLI Presentation` (Presenter seam, render modes — consumed by every verb) and the shared AI engine + config model (established by the release spec) are foundational; `Commit Command` reuses all of them. Suggested build order: CLI Presentation alongside/before Release → Release (establishes engine, config, consumes Presenter) → Commit.

**Naming:** "Mint Release Tool" reuses the existing anchored spec name (its sole prior source, `mint-release-tool`, is the majority of the new grouping). No naming conflict — the anchored discussion is not scattered across multiple new groupings.
