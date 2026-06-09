# Plan: Commit Command

## Phases

### Phase 1: Walking Skeleton — Bare Commit, End-to-End
status: approved
approved_at: 2026-06-09

**Goal**: A bare `mint commit` (staged-only) generates a Conventional Commits message from the staged diff and commits it — threading the commit-specific L3 glue through the consumed L1/L2 AI engine, the Presenter `Continue?` gate, git_safe, and the `[commit]` config table.

**Why this order**: This is the walking skeleton — the thinnest end-to-end thread through every primitive commit consumes. It proves the L3 binding (source selection, Conventional Commits prompt/format, commit sink), the gate rendering, and the git_safe commit all work together against the real shared engine before any variant or degradation path is layered on. Establishing this pattern first means every later phase extends a working system.

**Acceptance**:
- [ ] `mint commit` on a repo with a staged diff generates a Conventional Commits message (AI infers `type`, scope off by default) and creates the commit via git_safe
- [ ] Message generation consumes L1 (`git diff --cached`, with `diff_exclude` + `max_diff_lines` applied) and L2 (transport, output validation, one retry) without re-implementing engine internals in commit code
- [ ] The `Continue?` gate renders via the Presenter; `y`/Enter accepts and commits, `n` aborts as a true no-op
- [ ] `[commit].context` and `[commit].prompt` are read, typed, and fail loud on invalid input — context injects into the prompt, prompt overrides it fully
- [ ] Preflight fails loud with no AI call when the index is empty ("nothing to commit, working tree clean")
- [ ] No `commit_prefix` / 🌿 branding appears anywhere in the commit message text

### Phase 2: Staging Model — `-a` / `-A` with Deferred Staging
status: approved
approved_at: 2026-06-09

**Goal**: Add `-a`/`--all` (tracked modifications + deletions) and `-A`/`--add-all` (everything including untracked), computing the would-be-committed diff read-only for message generation and applying `git add` only after gate-accept.

**Why this order**: Builds directly on the skeleton's commit sink and gate. It introduces the mutate-nothing-until-accept invariant and the staging-specific preflight and empty-staging messaging as one cohesive concern, before the editor degradation path (Phase 3) needs to reconcile with deferred staging.

**Acceptance**:
- [ ] `-a` stages tracked modifications + deletions (no untracked); `-A` stages everything including untracked — both computed read-only before the gate, without mutating the index
- [ ] Staging is applied only after gate-accept; aborting an `-a`/`-A` run leaves the index exactly as it was
- [ ] `-a` and `-A` supplied together fail loud before any work with the conflicting-flags message
- [ ] Empty-staging fails loud with the message determined by actual tree state after the requested mode: clean-tree vs. "no changes staged" vs. tracked-only `-a` on untracked-only changes pointing at `-A`
- [ ] `mint commit -A` on a pristine tree reports "nothing to commit, working tree clean"

### Phase 3: $EDITOR Fallback — Unified No-AI Degradation Path
status: approved
approved_at: 2026-06-09

**Goal**: Route all three "no AI message" cases — `--no-ai`, AI-generation failure after retry, and an oversized diff (`max_diff_lines` exceeded) — to `$EDITOR` with save-as-accept, reconciled with deferred staging and the `-y`/non-TTY forbidden-combo posture.

**Why this order**: Depends on the staging-on-accept mechanics (Phase 2) and the base gate/commit sink (Phase 1). It introduces a distinct risk profile — interactive editor launch plus unattended fail-loud — and converges three cases onto one consistent path, which later gate actions (Phase 4) also route into.

**Acceptance**:
- [ ] `--no-ai`, AI failure after the engine's one retry, and a diff over `max_diff_lines` (skipped at L1 before any L2 call, after `diff_exclude`) all drop to the editor; the oversized case notes "diff too large to summarise — opening editor"
- [ ] The editor is resolved via git's own order (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git default) and opened by mint itself; an unset `$EDITOR` is not an error on a TTY
- [ ] A non-empty save is the accept event — it applies `-a`/`-A` staging then commits; an empty/aborted editor is a true no-op (no staging, no commit)
- [ ] Under `-y` or non-TTY stdin, any fallback fires fail-loud ("no AI message and no interactive editor available") — never hangs, never commits an empty message
- [ ] When no editor in the chain is launchable on the fallback path, mint fails loud (there is no message to fall back to)

### Phase 4: Interactive Gate Actions — Edit and Regenerate
status: approved
approved_at: 2026-06-09

**Goal**: Add the `e` (edit, loop back to gate) and `r` (regenerate-with-context) gate actions, including the one-time context line-read via the Presenter and regeneration-failure routing.

**Why this order**: Extends the Phase 1 gate and reuses the Phase 3 editor resolution and fallback routing. This interactive refinement loop is only meaningful once both the base gate and the editor path exist, so it sequences after them.

**Acceptance**:
- [ ] `e` opens the editor pre-filled with the current message and returns to the `Continue?` gate with the edited message used verbatim (no AI reprocessing); an empty save discards the edit and re-renders the gate with the prior message preserved
- [ ] `r` prompts for a single free-text context line via the Presenter's line-read, injects it one-time into the regeneration prompt (not persisted), and re-runs the engine; an empty line regenerates with no injected context
- [ ] Regeneration failure after its one retry routes to the `$EDITOR` fallback (no special re-show-prior-message path)
- [ ] `e` when no editor in the chain is launchable graceful-degrades: warn the editor could not launch and re-render the gate with the unedited message preserved (treat `e` as a no-op)

### Phase 5: Auto-push — `-p` with Warn-Don't-Unwind
status: approved
approved_at: 2026-06-09

**Goal**: Add `-p`/`--push` to push after a completed commit, applying the post-accept never-unwind invariant on push failure and completing the `mint commit -Ap` / `-Apy` ergonomic target.

**Why this order**: Push is strictly post-accept and depends on a completed commit produced by every prior accept path (gate-accept in Phases 1/4 and editor-save-accept in Phase 3). It is the final vertical slice and closes the headline one-liner.

**Acceptance**:
- [ ] `-p` runs a normal `git push` (current branch → its configured upstream) only after a successful commit, including after an editor save-as-accept (e.g. `mint commit -Ap --no-ai`)
- [ ] No push occurs without `-p`; there is no push config default
- [ ] Any push failure emits one generic warn (commit is in place; re-run the push) with git's stderr passed through verbatim; mint does not classify causes and never unstages, resets, or rewrites
- [ ] An empty/aborted run (gate `n` or empty editor save) performs no push even with `-p`
- [ ] No pre-push or remote-sync gate is run; `mint commit -Apy` executes unattended end-to-end
