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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-1-1 | Read [commit] config table | key absent (context and prompt both optional), wrong type fails loud, prompt-override path unreadable/missing |
| commit-command-1-2 | Assemble Conventional Commits prompt (L3) | context present injects vs absent, prompt-override fully replaces default, scope omitted by default, no commit_prefix / 🌿 in output |
| commit-command-1-3 | Bind staged-diff source through L1/L2 (L3 glue) | diff_exclude removes excluded files before generation, max_diff_lines guard applied at L1, L2 one-retry consumed not reimplemented |
| commit-command-1-4 | Wire bare `mint commit` generate-and-commit thread | AI infers type, scope off by default, commit created via git_safe, message text carries no 🌿 branding |
| commit-command-1-5 | Integrate the Continue? review gate | Enter accepts, y accepts, n aborts mutating nothing, -y auto-accept skips gate, non-TTY without -y fail-loud |
| commit-command-1-6 | Minimal preflight: empty-index fail-loud | empty index fails loud with "nothing to commit, working tree clean", no AI call on empty diff, not-a-git-repo fails loud |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-2-1 | Parse `-a`/`-A` flags with mutual-exclusion fail-loud | `-aA` combined fails loud before any read/AI work, `-A` alone, `-a` alone, neither given keeps default staged-only behaviour |
| commit-command-2-2 | Compute the would-be-staged diff read-only per mode | `-a` captures tracked mods + deletions excluding untracked, `-A` includes untracked, deletions captured under both modes, index left unmutated after computation, `diff_exclude` + `max_diff_lines` apply to the would-be-staged diff |
| commit-command-2-3 | Defer staging to gate-accept; abort leaves the index untouched | abort (`n`) leaves index exactly as pre-`mint` (pre-existing user staging untouched), accept applies `git add` for the mode then commits in that order, `-y` auto-accept applies staging, default mode runs no `git add` |
| commit-command-2-4 | Flag-aware empty-staging messaging matrix | `-A` on pristine tree → "nothing to commit, working tree clean", bare commit with unstaged changes → "no changes staged — use `-a`/`-A`/git add", `-a` when only untracked changes exist → point at `-A`/`--add-all`, message keyed on actual post-mode tree state not the flag passed, no AI call on any empty case |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-3-1 | Resolve the editor via git's resolution order | GIT_EDITOR wins over all, core.editor over $VISUAL/$EDITOR, $VISUAL over $EDITOR, unset $EDITOR falls to git default (not an error on TTY), none in chain launchable returns a not-launchable signal |
| commit-command-3-2 | `--no-ai` drops to the editor with save-as-accept | non-empty save = accept applies -a/-A staging then commits in order, empty save = no staging/no commit/no mutation, aborted/quit editor = true no-op, default-mode commits index unchanged on save, editor opened by mint itself not delegated to git commit |
| commit-command-3-3 | Route AI-generation failure to the editor fallback | failure after the engine's one retry routes to editor not abort, distinguished from oversized-skip, save-as-accept semantics reused unchanged, no synthetic stub message inserted |
| commit-command-3-4 | Route oversized diff (max_diff_lines) to the editor fallback with note | detected at L1 after diff_exclude and before any L2 call, diff_exclude applied first so excluded noise can't push over limit, emits "diff too large to summarise — opening editor", treated as generate-skip not generate-failure, at-limit vs over-limit boundary |
| commit-command-3-5 | Fail loud when the fallback has no message source | -y + fallback fails loud, non-TTY stdin + fallback fails loud, no launchable editor on TTY fails loud (no message to fall back to), applies identically across all three converging cases, never hangs, never commits an empty message, no -m escape hatch |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-4-1 | Add the `e` edit action with loop-back to the gate | non-empty save loops back to gate not save-as-accept, edited message used verbatim with no AI reprocessing, multi-line edited body preserved, gate re-renders with edited message and y/n/e/r still available, reuses 3-1 editor resolution (no parallel resolver) |
| commit-command-4-2 | `e` empty-save discards edit, gate re-renders with prior message preserved | empty save discards the edit and preserves the prior message, whitespace-only save treated as empty per editor contract, repeated `e` then empty-save still preserves the original message, `e` is never a message source so can never produce an empty commit |
| commit-command-4-3 | `e` graceful-degrade when no editor is launchable | not-launchable signal from 3-1 triggers warn + re-render not fail-loud, unedited message preserved verbatim, distinct from 3-5 fallback fail-loud because a message already exists, gate remains usable (y/n/e/r) after the warn |
| commit-command-4-4 | Add the `r` regenerate-with-context action (line-read + one-time injection) | non-empty line injected one-time into the regeneration prompt, injected context not persisted to config or subsequent re-rolls, empty line regenerates with no injected context (plain re-roll), Enter submits via the Presenter's line-read, regenerated message returns to the gate, moot under -y/non-TTY (interactive-only action) |
| commit-command-4-5 | Route `r` regeneration-failure to the $EDITOR fallback | failure after the engine's one retry routes to the 3-3 editor fallback, reuses the 3-3 entry point (no parallel failure handler), no special re-show-prior-message path, fallback save-as-accept semantics unchanged, moot under -y/non-TTY (interactive-only action) |

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

#### Tasks
status: approved
approved_at: 2026-06-09

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-5-1 | Parse the `-p`/`--push` flag (flag-only, no config default) | `-p` absent → no push, `-p` present arms push, no push config key exists or is read, composes in `-Ap` and `-Apy` bundles, push never armed by default |
| commit-command-5-2 | Push after a successful gate-accept commit | push runs only after the commit succeeds, normal `git push` (current branch → configured upstream) with no special upstream logic, `-p` without a successful commit performs no push, push runs after `-y` auto-accept commit too |
| commit-command-5-3 | Push after an editor save-as-accept commit | non-empty editor save commits then pushes, `mint commit -Ap --no-ai` end-to-end (stage, commit, push), reuses the single push step (no parallel push call), push runs only after the staging+commit ordering completes |
| commit-command-5-4 | Warn-don't-unwind on push failure | one generic warn for all causes (rejected, remote moved, no upstream, network), git's stderr passed through verbatim beneath the warn, no cause classification, never unstages/resets/rewrites the commit, commit stays forward-only and the push is repeatable, no-upstream surfaces git's own hint via pass-through |
| commit-command-5-5 | Suppress push on empty/aborted runs; confirm no pre-push/remote-sync gate | gate `n` with `-p` performs no push (nothing committed), empty/aborted editor save with `-p` performs no push, no pre-push or remote-sync gate runs, `mint commit -Apy` runs unattended end-to-end, no remote-sync precheck blocks the push attempt |

### Phase 6: Analysis (Cycle 1)

**Goal**: Address findings from Analysis (Cycle 1).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-6-1 | Single-source the emptiness verdict from the exclusion-filtered diff so the AI is never invoked on an empty post-exclusion diff | all-excluded staged/changed set fails loud with the exact spec empty-staging message and mutates nothing, `transport.Generate` never called on an empty post-`diff_exclude` diff, preflight probes and the AI's L1 diff read the same exclusion-filtered source, all-excluded worktree/deferred-staging path also fails loud without an AI call, genuinely non-empty staged set still reaches `Generate`, normally-empty staging set still fails loud as before |
| commit-command-6-2 | Rename/redocument the Committer seam to reflect it is the lock-resilient sink for ALL commit mutations (stage, commit, push) | interface and Deps-field docs both state stage/commit/push (not commit-only), rename to `Mutator` mirrors `engine.ReleaseDeps.Mutator` if chosen, all references (interface, Deps field, cmd-layer wiring, tests) compile if renamed, no behavioural change (same three mutations route through the lock wrapper), go vet/golangci-lint/`go test ./internal/commit/...` clean |
| commit-command-6-3 | Do not emit the "opening editor" note on the unattended oversized path that fails loud | `-y`/non-TTY oversized path records no "opening editor" Warn and fails loud with the exact spec message mutating nothing, attended (TTY, non-`-y`) oversized path still emits the note and opens the editor, AI-failure trigger still emits no note, no-message-source guard runs before the oversized note |
| commit-command-6-4 | Consolidate the duplicated commit test-suite scaffolding (invocation-filter helpers and per-file Deps builders) | two shared invocation-filter helpers replace the nine wrappers (no re-rolled Name==git or Args[0]==verb loops), single editor-path Deps builder centralises the `git.NewMutator(...WithBackoff...)` wiring and `StdinInteractive` default, previously-identical/subset builders become thin wrappers or inlined with each scenario's distinct fields preserved, `newCommitDeps` (bare path) stays separate, both raw-runner and `editorRunner.fake` call sites route through the shared helpers, suite passes unchanged with no assertion meaning changed |
| commit-command-6-5 | Extract a single commitAccept helper to single-source the stage→commit→push accept tail | one `commitAccept` helper holds the stage→commit→push→RunFinished→return-pushErr sequence, neither `Run` nor `runEditorFallback` inlines the tail, gate-accept passes `finalBody` and save-as-accept passes `saved`, identical observable behaviour (same mutations/ordering/error-surfacing, RunFinished always fires, pushErr returned), existing accept-path and push tests pass unchanged |

### Phase 7: Analysis (Cycle 2)

**Goal**: Address findings from Analysis (Cycle 2).

#### Tasks

| Internal ID | Name | Edge Cases |
|-------------|------|------------|
| commit-command-7-1 | Structurally single-source the per-mode git source selection shared by the preflight probes and the L1 diff sources | each per-mode argv prefix and the `-- .` selector spelled once and shared by preflight probe and L1 source, preflight probe argv = shared prefix + `--name-only` (diff cases) / shared `ls-files` prefix (untracked), StagingMode→sources mapping (incl. AddAll tracked-then-untracked short-circuit) defined in one place for both emptiness and diff paths, empty-staging preflight cluster + shared selector moved to `internal/commit/preflight.go` with `run.go` no longer containing them, AddAll probe still uses `--name-only` (no untracked file bodies), verbatim empty-staging messages and per-mode emptiness verdicts unchanged, go vet/golangci-lint/commit tests clean |
