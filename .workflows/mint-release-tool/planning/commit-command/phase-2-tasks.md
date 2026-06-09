---
phase: 2
phase_name: Staging Model — `-a` / `-A` with Deferred Staging
total: 4
---

## commit-command-2-1 | approved

### Task commit-command-2-1: Parse `-a`/`-A` flags with mutual-exclusion fail-loud

**Problem**: Phase 1 wired only the bare staged-only `mint commit`. The staging model needs two faithful flags — `-a`/`--all` (`git commit -a` semantics: tracked modifications + deletions, no untracked) and `-A`/`--add-all` (`git add -A` then commit: everything including untracked) — and these two are **mutually exclusive**. Supplying both (`mint commit -aA`) is contradictory input and must **fail loud before any read or AI work**, never silently picking a winner. Without flag parsing and the mutual-exclusion guard, the later staging-mode tasks (2-2, 2-3, 2-4) have no mode to branch on.

**Solution**: Add the `-a`/`--all` and `-A`/`--add-all` flags to the `mint commit` subcommand (consumed `cmd/mint` surface, reserved in 1-4) and resolve them into a single staging-mode value the orchestrator (`internal/commit` `Run`) branches on — e.g. a `StagingMode` enum: `StagedOnly` (neither flag, the Phase 1 default), `All` (`-a`), `AddAll` (`-A`). When **both** flags are supplied, fail loud immediately at flag resolution, before preflight, diff computation, or any AI call, with the conflicting-flags message. Neither flag preserves the Phase 1 staged-only behaviour unchanged.

**Solution note**: This task owns only flag parsing and the mutual-exclusion guard plus the mode value the rest of Phase 2 consumes. It does NOT compute any diff (2-2), defer/apply staging (2-3), or produce empty-staging messages (2-4). The `-p`, `--no-ai`, and gate `e`/`r` flags belong to later phases — reserve but do not implement them. The mutual-exclusion check must run **before** the consumed preflight (1-6) and before L3 generate (1-3) — it is a pure argument-validation fail-loud with zero side effects.

**Outcome**: `mint commit` with neither flag behaves exactly as Phase 1 (staged-only). `mint commit -a` resolves to the tracked-mods+deletions mode; `mint commit -A` resolves to the everything-including-untracked mode; both long forms (`--all`, `--add-all`) resolve identically. `mint commit -aA` (or any spelling combining the two: `-a -A`, `--all --add-all`) **fails loud before any read/AI/preflight work** with *"`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes"* — no diff is computed, no AI is called, the index is never touched.

**Do**:
- In the `mint commit` subcommand (`cmd/mint`, the surface 1-4 added), register `-a`/`--all` and `-A`/`--add-all` as boolean flags. Both reserved-but-unimplemented in Phase 1 are now activated.
- In the commit orchestrator (`internal/commit` `Run`), resolve the two booleans into a single `StagingMode` value (e.g. `StagedOnly` / `All` / `AddAll`), so 2-2/2-3/2-4 branch on one enum rather than two raw booleans.
- **Mutual-exclusion guard**: if both `-a` and `-A` are set, return a loud failure with the exact intent *"`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes"*, surfaced via the consumed Presenter (and stderr). This check runs **first** — before the 1-6 preflight (repo-present / something-to-commit), before L3 generate, before any `CommandRunner` call.
- Neither flag → `StagedOnly`, the unchanged Phase 1 default path (no `git add`, staged index used directly).
- Do NOT implement the staging diff, the deferred `git add`, or the empty-staging messaging here — those are 2-2/2-3/2-4. Do NOT touch `-p`/`--no-ai`/gate `e`/`r` (later phases).

**Acceptance Criteria**:
- [ ] `-a`/`--all` resolves to the tracked-mods+deletions staging mode.
- [ ] `-A`/`--add-all` resolves to the everything-including-untracked staging mode.
- [ ] Neither flag resolves to the Phase 1 staged-only default (unchanged behaviour).
- [ ] `-a` and `-A` supplied together fail loud with *"`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes"*.
- [ ] The mutual-exclusion failure occurs **before** any preflight, diff computation, or AI call — no `CommandRunner`/`ai_command` invocation is recorded, the index is untouched.
- [ ] Both long forms (`--all`, `--add-all`) parse identically to their short forms.
- [ ] No `-p`/`--no-ai`/gate-`e`/`r` behaviour is implemented (deferred to later phases).

**Tests**:
- `"-a alone resolves to the tracked-mods-plus-deletions staging mode"`
- `"-A alone resolves to the everything-including-untracked staging mode"`
- `"neither flag keeps the Phase 1 staged-only default"`
- `"--all and --add-all parse identically to -a and -A"`
- `"-aA fails loud with the conflicting-flags message"`
- `"-a -A (separate) also fails loud with the conflicting-flags message"`
- `"the -aA conflict fails before any git/AI call (no CommandRunner or ai_command invocation recorded)"`

**Edge Cases**:
- `-aA` combined fails loud before any read/AI work.
- `-A` alone → everything-including-untracked mode.
- `-a` alone → tracked-mods+deletions mode.
- Neither given → keeps default staged-only behaviour.

**Context**:
> Staging Model: "Decision — two faithful flags: Default = staged-only … `-a` / `--all` = `git commit -a` — tracked modifications + deletions, no untracked. Muscle-memory faithful. `-A` / `--add-all` = `git add -A` then commit — everything including untracked … `-a` and `-A` are mutually exclusive. Supplying both (`mint commit -aA`) is a conflicting-flags error → fail loud before any work (*'`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes'*). Consistent with the fail-loud posture for contradictory input — mint never silently picks a winner." CLI Surface & Flags: "`-a, --all` stage tracked changes first (git commit -a semantics); `-A, --add-all` stage everything incl. untracked first (git add -A)." The deferred staging and empty-staging messaging are 2-3/2-4; this task is flag parsing + the mutual-exclusion guard only.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Staging Model → Decision — two faithful flags", "CLI Surface & Flags".

## commit-command-2-2 | approved

### Task commit-command-2-2: Compute the would-be-staged diff read-only per mode

**Problem**: The core invariant is **mint mutates nothing until gate-accept** — so under `-a`/`-A` the message must be generated from the *would-be-committed* diff computed **read-only**, without running `git add` and without mutating the index. The two modes capture different content (`-a` = tracked modifications + deletions, no untracked; `-A` = everything including untracked), and both must include deletions. Phase 1's L1 source was hardwired to `git diff --cached` (staged-only); without a per-mode read-only source the staging flags from 2-1 cannot feed L1/L2 to produce a message that reflects what will actually be committed.

**Solution**: Extend commit's L3 source selection (the 1-3 glue) so the L1 source is chosen by the 2-1 `StagingMode`: `StagedOnly` → `git diff --cached` (Phase 1, unchanged); `All` (`-a`) → the would-be-staged diff of **tracked** working-tree changes (modifications + deletions, **excluding** untracked) computed read-only; `AddAll` (`-A`) → the would-be-staged diff **including** untracked files, computed read-only. All three feed the *same* consumed L1 `diff_exclude` + `max_diff_lines` logic and the *same* consumed L2 transport — only the diff source differs. Crucially, computing the `-a`/`-A` diff runs **no** `git add`: it reads the working tree against HEAD/index without mutating the index.

**Solution note**: L1's `diff_exclude` + `max_diff_lines` filtering and L2's transport/validate/one-retry are **consumed** from the shared engine (per 1-3) — do NOT rebuild them. This task only adds the two read-only would-be-staged *sources* and routes the selected source into the existing L1→compose→L2 pipeline. It does NOT run `git add` (that is 2-3, on accept), does NOT produce empty-staging messages (2-4), and does NOT route oversized/`--no-ai` failures (Phase 3). The read-only computation must leave the index **byte-for-byte unchanged** — assert the index (and any pre-existing user staging) is identical before and after.

**Outcome**: Under `-a`, the diff fed to L1/L2 contains tracked modifications and tracked deletions and **excludes** untracked files — matching `git commit -a` semantics. Under `-A`, the diff additionally includes untracked (new) files — matching `git add -A` then diff. Deletions are captured under **both** modes. The index is **unmutated** after computation (no `git add` ran; any pre-existing user-staged content is exactly as it was). `diff_exclude` globs and the `max_diff_lines` guard apply to the would-be-staged diff exactly as they apply to the staged-only diff (consumed L1 logic, identical for all sources).

**Do**:
- In commit's L3 glue (`internal/commit`, extending 1-3's `Generate`), branch the **L1 source** on the 2-1 `StagingMode`:
  - `StagedOnly` → `git diff --cached` (the Phase 1 source — unchanged).
  - `All` (`-a`) → a **read-only** diff of tracked working-tree changes vs the committed/index state, capturing modifications + deletions and **excluding** untracked files (the content `git commit -a` would stage: e.g. `git diff HEAD` restricted to tracked paths, or the equivalent that yields tracked mods+deletions without untracked — choose the read-only formulation that matches `git add -u`'s set without staging it).
  - `AddAll` (`-A`) → a **read-only** diff that additionally includes untracked files (the content `git add -A` would stage: tracked mods + deletions **plus** new/untracked files), still without running `git add`. (mint runs from the repo root, so this is the `git add .` ≡ `git add -A` set.)
- Feed the selected source through the **consumed** L1 `diff_exclude` + `max_diff_lines` logic and the **consumed** L2 transport — identical to 1-3, only the source differs. Do NOT duplicate the filter/guard/transport.
- The would-be-staged computation must be **read-only**: run **no** `git add`, leave the index unmutated. All git reads go through the consumed `CommandRunner` seam.
- Capture **deletions** under both `-a` and `-A` (a deleted tracked file is part of what `git commit -a` / `git add -A` would stage and must appear in the diff).
- Do NOT apply the staging (no `git add`) here — that is deferred to accept (2-3). Do NOT emit empty-staging messages (2-4) or route failures (Phase 3).

**Acceptance Criteria**:
- [ ] `-a` produces a diff of tracked modifications + deletions, **excluding** untracked files.
- [ ] `-A` produces a diff that includes untracked files (plus tracked mods + deletions).
- [ ] Deletions are captured in the diff under **both** `-a` and `-A`.
- [ ] The index is **unmutated** after the would-be-staged computation (no `git add` ran; pre-existing user staging untouched).
- [ ] `diff_exclude` globs are applied to the would-be-staged diff (excluded files never reach the prompt) — consumed L1 logic.
- [ ] `max_diff_lines` guard is applied to the would-be-staged diff at L1 (consumed logic).
- [ ] `StagedOnly` continues to use `git diff --cached` unchanged (Phase 1 path intact).
- [ ] All diff computation is read-only via the consumed `CommandRunner`/fake; no `git add` is invoked.

**Tests**:
- `"-a captures tracked modifications and deletions and excludes untracked files"`
- `"-A captures tracked mods, deletions, and untracked files"`
- `"a deleted tracked file appears in the diff under -a"`
- `"a deleted tracked file appears in the diff under -A"`
- `"the index is unchanged after the -a would-be-staged computation (no git add ran)"`
- `"pre-existing user-staged content is unchanged after the -A would-be-staged computation"`
- `"diff_exclude removes excluded files from the would-be-staged diff before generation"`
- `"the max_diff_lines guard is applied to the would-be-staged diff at L1"`
- `"StagedOnly still uses git diff --cached unchanged"`

**Edge Cases**:
- `-a` captures tracked mods + deletions, excluding untracked.
- `-A` includes untracked.
- Deletions captured under both modes.
- Index left unmutated after computation.
- `diff_exclude` + `max_diff_lines` apply to the would-be-staged diff.

**Context**:
> Commit Flow / Lifecycle: "Build context (L1) — filtered diff of what *would* be committed (default: `git diff --cached`; with `-a`/`-A`: the would-be-staged working-tree diff, computed **without** mutating the index), with `diff_exclude` + `max_diff_lines` applied." Staging Model table: "`git commit -a` / `git add -u`: Modified tracked ✅, Deleted tracked ✅, New/untracked ❌; `git add .` (from root) / `git add -A`: Modified tracked ✅, Deleted tracked ✅, New/untracked ✅. (mint runs from the repo root, so `git add .` ≡ `git add -A` for its purposes.)" "Staging is deferred to gate-accept. With `-a`/`-A`, mint computes the would-be-committed diff *read-only* for message generation, and only runs `git add` after the user accepts." AI Engine: "Layer 1 — Context builder … Parameterised by *source* … Applies `diff_exclude` globs and the `max_diff_lines` guard — identical logic for both verbs." Commit's binding: "Layer 1 source: the staged diff (`git diff --cached`), or the would-be-staged diff under `-a`/`-A` computed read-only." The `git add` itself is deferred to accept (2-3); empty-staging messaging is 2-4.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Flow / Lifecycle", "Staging Model", "AI Engine — Three-Layer Split → Commit's binding to the engine".

## commit-command-2-3 | approved

### Task commit-command-2-3: Defer staging to gate-accept; abort leaves the index untouched

**Problem**: The cross-cutting invariant **mint mutates nothing until accept** is what makes abort a true no-op: under `-a`/`-A` the `git add` must run **only after** the user accepts the gate (or `-y` auto-accepts), and an abort (`n`) must leave the index **exactly** as it was before `mint` ran — including any pre-existing user staging. 2-2 computes the would-be-staged diff read-only; this task owns the deferred `git add` on the accept path and the strict no-mutation-on-abort guarantee. Without it, an `-a`/`-A` abort would either leave a half-staged worktree or stage at the wrong time.

**Solution**: Place the mode-appropriate `git add` **after** the gate-accept branch in the commit orchestrator (extending the 1-5 gate integration), and **before** the `git_safe` commit, in that order: on accept, stage for the mode, then commit. `-a` applies `git add -u` (tracked mods + deletions, no untracked); `-A` applies `git add -A` (everything including untracked). `StagedOnly` (default) applies **no** `git add` (Phase 1 path). On abort (`n`), do nothing — run no `git add`, no commit — so the index is exactly its pre-`mint` state. `-y` auto-accept follows the accept path (stage then commit).

**Solution note**: The gate rendering / accept-abort branch / `-y` skip are **consumed** from the 1-5 Presenter integration — do NOT rebuild the gate. This task only adds the deferred `git add` on the accept side and proves the abort side mutates nothing. The `git add` mutation and the `git commit` both run via the consumed `git_safe` lock-resilient wrapper (not the raw runner), since they are mutating git operations. Do NOT implement the editor save-as-accept path (Phase 3) or `-p` push (Phase 5) here — the accept path here is gate-accept → stage → commit. The empty-staging cases (where staging would stage nothing) are 2-4 — this task assumes a non-empty would-be-staged set reaching the gate.

**Outcome**: On gate-accept under `-a`, mint runs `git add -u` then commits (in that order). On accept under `-A`, mint runs `git add -A` then commits. On accept under the default (no flag), mint runs **no** `git add` and commits the existing index (Phase 1, unchanged). `-y` follows the accept path (stages then commits) without rendering the gate. On abort (`n`) under any mode, mint runs **no** `git add` and **no** commit — the index is **exactly** as it was before `mint` ran, including any content the user had staged beforehand (mint never leaves a half-staged worktree behind and never touches pre-existing staging on abort).

**Do**:
- In the commit orchestrator (`internal/commit` `Run`, extending the 1-5 gate branch), on the **accept** branch (gate `y`/Enter, or `-y` auto-accept), before the `git_safe` commit:
  - `All` (`-a`) → run `git add -u` (tracked modifications + deletions, no untracked) via the consumed **`git_safe`** wrapper, then proceed to the commit.
  - `AddAll` (`-A`) → run `git add -A` (everything including untracked) via the consumed **`git_safe`** wrapper, then proceed to the commit.
  - `StagedOnly` → run **no** `git add`; proceed straight to committing the existing index (Phase 1 path).
  - Ordering is strict: **stage → then commit**. The commit (1-4's `git_safe` commit) runs after staging completes.
- On the **abort** branch (gate `n`): do nothing — no `git add`, no commit. Return a clean no-op. Assert the index (porcelain / `git diff --cached`) is byte-for-byte identical to the pre-`mint` state, including any pre-existing user-staged content.
- `-y` auto-accept consumes the 1-5 skip and follows the accept path (stage then commit).
- Use the consumed **`git_safe`** for both the `git add` and the `git commit` mutations (lock-resilient git is a consumed dependency); do not use the raw runner for these mutating operations.
- Do NOT implement the editor save-as-accept path (Phase 3) or `-p` push (Phase 5). Do NOT handle the stage-nothing empty cases here (2-4).
- Tests use the recording presenter (scripted choice) + fake runner: assert accept runs the mode's `git add` **then** the commit (ordered), abort runs neither, `-y` stages and commits, default mode runs no `git add`.

**Acceptance Criteria**:
- [ ] On accept under `-a`, `git add -u` runs **then** the commit, in that order.
- [ ] On accept under `-A`, `git add -A` runs **then** the commit, in that order.
- [ ] On accept under the default mode, **no** `git add` runs (existing index committed — Phase 1 path).
- [ ] On abort (`n`) under any mode, **no** `git add` and **no** commit run.
- [ ] After an abort, the index is **exactly** the pre-`mint` state, including pre-existing user staging (untouched).
- [ ] `-y` auto-accept follows the accept path (stages for the mode, then commits).
- [ ] The `git add` and `git commit` both run via the consumed `git_safe` wrapper, not the raw runner.
- [ ] No editor save-as-accept (Phase 3) or `-p` push (Phase 5) behaviour is implemented.

**Tests**:
- `"accept under -a runs git add -u then commits, in that order"`
- `"accept under -A runs git add -A then commits, in that order"`
- `"accept under the default mode runs no git add and commits the existing index"`
- `"abort (n) under -a runs no git add and no commit"`
- `"abort (n) under -A leaves the index exactly as the pre-mint state"`
- `"abort leaves pre-existing user-staged content untouched"`
- `"-y auto-accept under -A stages then commits without rendering the gate"`
- `"the staging git add runs via git_safe, not the raw runner"`

**Edge Cases**:
- Abort (`n`) leaves index exactly as pre-`mint` (pre-existing user staging untouched).
- Accept applies `git add` for the mode then commits in that order.
- `-y` auto-accept applies staging.
- Default mode runs no `git add`.

**Context**:
> Commit Flow / Lifecycle: "The core invariant that shapes the whole flow: mint mutates nothing until the user accepts the gate. Everything before accept is read-only — including the `-a`/`-A` staging, which is deferred to the accept path. This is what makes abort a true no-op … On accept — apply `-a`/`-A` staging now (if given), then `git commit` (via `git_safe`)." Staging Model: "Staging is deferred to gate-accept. With `-a`/`-A`, mint computes the would-be-committed diff *read-only* for message generation, and only runs `git add` after the user accepts. Aborting an `-a`/`-A` run leaves the index exactly as it was — mint never leaves a half-staged worktree behind." Interactive Review Gate: "`y` / accept → stage (if `-a`/`-A`) then commit … `n` / abort → do nothing. No auto-unwind needed — nothing has been mutated yet (staging deferred to accept), so abort is a true no-op back to the pre-`mint` state." Invariant: "Before gate-accept, mint mutates nothing — staging (`-a`/`-A`) is deferred to accept, so abort returns the user to their exact pre-`mint` state (their own prior staging untouched)." The gate-abort refinement: "Originally the flow staged `-a`/`-A` *before* the gate, which meant aborting would leave a mint-altered worktree — wrong … The fix — mint mutates nothing until accept (staging deferred)." Editor save-as-accept is Phase 3; `-p` push is Phase 5.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Commit Flow / Lifecycle", "Staging Model", "Interactive Review Gate", "Invariant — mutate nothing until accept; never unwind after".

## commit-command-2-4 | approved

### Task commit-command-2-4: Flag-aware empty-staging messaging matrix

**Problem**: Phase 1's preflight (1-6) produced a single bare empty-index message (*"nothing to commit, working tree clean"*). With `-a`/`-A`, the empty-staging case fans out: a chosen mode may stage nothing because the tree is genuinely clean, or because changes exist that the chosen mode could not stage. The spec requires mirroring git's two-message distinction, and — critically — **which message fires is determined by the *actual* tree state after the requested staging mode, not by the flag passed**. The AI must **never** be invoked on an empty diff in any of these cases. Without this matrix, `mint commit -a` on untracked-only changes would mislead the user, or mint would call the AI on nothing.

**Solution**: Extend the 1-6 preflight "something to commit" check to be staging-mode aware. After computing the would-be-staged set for the requested mode (read-only, from 2-2), if that set is empty, fail loud **before any AI call** with the message keyed on the **actual post-mode tree state**: (a) genuinely clean tree (nothing the mode could ever stage and the index is empty) → *"nothing to commit, working tree clean"*; (b) changes exist but the chosen mode staged none → mint's flavour of git's `no changes added to commit`, naming the modes that *would* help — bare `mint commit` with unstaged changes → *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*; `mint commit -a` when the only changes are **untracked** → *"no tracked changes to stage — use `-A`/`--add-all` to include untracked files"*. This replaces/extends 1-6's single bare message.

**Solution note**: This task extends the consumed 1-6 preflight; it does NOT re-implement repo-present detection or the dropped gates (clean-working-tree, on-release-branch, remote-in-sync — deliberately dropped). It consumes the 2-2 would-be-staged computation to learn the post-mode tree state and the 2-1 mode value. The message must be keyed on **tree state**, not the flag: e.g. `mint commit -A` on a pristine tree yields the clean-tree message (not a flag-specific one) because the actual post-`-A` state is empty-and-clean. No `git add` runs in any empty case (staging is still deferred / never reached), and **no AI call** fires. Do NOT implement the `$EDITOR` fallback (Phase 3) — these are hard fail-loud cases, not editor drops.

**Outcome**: The empty-staging failure message is selected by the actual post-mode tree state:
- `mint commit -A` (or `-a`, or bare) on a **pristine tree** (nothing to stage, index empty) → *"nothing to commit, working tree clean"*.
- Bare `mint commit` when **unstaged changes exist** but nothing is staged → *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*.
- `mint commit -a` when the **only** changes are untracked (so tracked-only `-a` staged nothing) → *"no tracked changes to stage — use `-A`/`--add-all` to include untracked files"*.
In every empty case the run fails loud, **no AI is invoked**, **no `git add` runs**, and **no commit** is created. The message is keyed on the tree state after the requested mode, never on the flag alone.

**Do**:
- In the commit preflight (extending 1-6), after determining the would-be-staged set for the 2-1 `StagingMode` (read-only, via 2-2), if that set is **empty**, classify the post-mode tree state and fail loud with the matching message:
  - **Genuinely clean tree** — the chosen mode had nothing it could ever stage **and** the index is empty (e.g. `mint commit -A` on a pristine tree) → *"nothing to commit, working tree clean"* (mirror git; reuse the 1-6 message).
  - **Changes exist, chosen mode staged none**:
    - Bare `mint commit` (`StagedOnly`) with unstaged changes present and nothing staged → *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*.
    - `mint commit -a` (`All`) when the only changes are **untracked** (tracked-only `-a` staged nothing) → *"no tracked changes to stage — use `-A`/`--add-all` to include untracked files"*.
- **Key the message on the actual post-mode tree state, not the flag passed**: compute what the requested mode would have staged plus whether other changes exist (untracked-only vs unstaged-tracked vs nothing), and pick the message from that — `-A` on a pristine tree must yield the clean-tree message, not an `-A`-specific one.
- **No AI on any empty case**: the empty classification must short-circuit before L3 generate (1-3) — assert no `ai_command`/`claude` invocation is recorded in any empty-staging case.
- **No mutation on any empty case**: no `git add`, no commit runs — these are pre-accept fail-loud paths.
- This **replaces/extends** the 1-6 single bare message: the bare-`StagedOnly` empty case now distinguishes genuinely-clean (clean-tree message) from unstaged-changes-present (no-changes-staged message) where 1-6 produced one message. Preserve 1-6's not-a-git-repo fail-loud and read-only posture.
- All state checks are read-only via the consumed `CommandRunner`; tests script the tree states (pristine / unstaged-tracked-only / untracked-only / staged) per mode. Do NOT route any of these to the `$EDITOR` fallback (Phase 3) — they are hard fail-loud.

**Acceptance Criteria**:
- [ ] `mint commit -A` on a pristine tree → *"nothing to commit, working tree clean"* (keyed on clean tree state, not the `-A` flag).
- [ ] Bare `mint commit` with unstaged changes but nothing staged → *"no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`"*.
- [ ] `mint commit -a` when the only changes are untracked → *"no tracked changes to stage — use `-A`/`--add-all` to include untracked files"*.
- [ ] The message is selected by the **actual post-mode tree state**, not by the flag passed.
- [ ] **No AI/`claude` is invoked** in any empty-staging case (short-circuits before generate).
- [ ] **No `git add` and no commit** runs in any empty-staging case.
- [ ] The 1-6 not-a-git-repo fail-loud and read-only posture are preserved; the dropped gates remain unimplemented.
- [ ] No empty case is routed to the `$EDITOR` fallback (that is Phase 3) — all are hard fail-loud.

**Tests**:
- `"mint commit -A on a pristine tree reports 'nothing to commit, working tree clean'"`
- `"mint commit -a on a pristine tree reports 'nothing to commit, working tree clean'"`
- `"bare mint commit with unstaged changes reports 'no changes staged — use -a/--all, -A/--add-all, or git add'"`
- `"mint commit -a with only untracked changes points at -A/--add-all"`
- `"the empty-staging message is keyed on tree state, not the flag (-A on clean tree gives the clean-tree message)"`
- `"no AI is invoked in any empty-staging case"`
- `"no git add and no commit run in any empty-staging case"`
- `"not-a-git-repo still fails loud (1-6 behaviour preserved)"`

**Edge Cases**:
- `-A` on pristine tree → "nothing to commit, working tree clean".
- Bare commit with unstaged changes → "no changes staged — use `-a`/`-A`/git add".
- `-a` when only untracked changes exist → point at `-A`/`--add-all`.
- Message keyed on actual post-mode tree state, not the flag passed.
- No AI call on any empty case.

**Context**:
> Staging Model — Empty-staging handling: "Empty staging (nothing to commit after staging) → fail loud; never invoke the AI on an empty diff. `-A`/`-a` that stage nothing land here too. Distinguish the two empty cases exactly as git does. Which message fires is determined by the *actual* tree state after the requested staging mode, not by the flag passed: Genuinely clean tree (the chosen mode had nothing it could ever stage, and the index is empty) → *'nothing to commit, working tree clean'*. (E.g. `mint commit -A` on a pristine tree.) Changes exist but the chosen mode staged none → guide the user — mint's flavour of git's `no changes added to commit` — naming the modes that *would* help: Bare `mint commit` with unstaged changes → *'no changes staged — use `-a`/`--all`, `-A`/`--add-all`, or `git add`'*. `mint commit -a` when the only changes are **untracked** (so tracked-only `-a` staged nothing) → point specifically at `-A`/`--add-all` … e.g. *'no tracked changes to stage — use `-A`/`--add-all` to include untracked files'*." Commit Flow: "Preflight (minimal) — git repo present; *something to commit* (for `-a`/`-A`, the would-be-staged changes; otherwise the existing index). Computed read-only. Empty → fail loud." This extends 1-6's single bare message into the flag-aware matrix; the `$EDITOR` fallback (Phase 3) does not apply to these empty cases — they are hard fail-loud.

**Spec Reference**: `.workflows/mint-release-tool/specification/commit-command/specification.md` — "Staging Model → Empty-staging handling", "Preflight & Safety", "Commit Flow / Lifecycle".
