---
status: complete
created: 2026-06-09
cycle: 1
phase: Gap Analysis
topic: commit-command
---

# Review Tracking: commit-command - Gap Analysis

## Findings

### 1. `e` (edit) at the AI-path gate: save-as-accept vs. loop-back is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Review Gate; `$EDITOR` Fallback — Path Semantics

**Details**:
Two different `$EDITOR`-save semantics coexisted without reconciliation: the fallback path defines "save = accept," while the gate's `e` choice left post-editor behaviour unstated. The cli-presentation seam's contract is loop-back (re-render the gate after `e`).

**Proposed Addition**:
Clarified `e` in Interactive Review Gate: `e` opens `$EDITOR` pre-filled, on save returns to the `Continue?` gate with the edited message shown verbatim (seam loop-back — NOT save-as-accept; only the fallback editor is save-as-accept). Empty save under `e` discards the edit and preserves the prior message (`e` is a refinement step, never a message source — can never produce an empty commit).

**Resolution**: Approved
**Notes**: Auto-resolved. Grounded in the cli-presentation seam's loop-back contract; reconciles with the fallback path's distinct save-as-accept rule.

---

### 2. `r` (regenerate-with-context): how the one-time context line is collected is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Review Gate (`r` choice)

**Details**:
`r` was defined as "re-run the AI with a one-time context line" but the context-entry UX was undefined. Since `--context` was consciously dropped, interactive `r` is the only surviving one-time-context path.

**Proposed Addition**:
Added to `r`: after `r`, mint prompts for a single free-text context line via the Presenter's line-read (same input model as the gate); Enter submits. The line is injected as one-time context (not persisted). Empty line = plain re-roll (no injected context). Regeneration runs the engine's one retry; failure → `$EDITOR` fallback.

**Resolution**: Approved
**Notes**: Auto-resolved. Uses the existing Presenter line-read model; empty-line = plain regenerate keeps it forgiving.

---

### 3. Oversized-diff (`max_diff_lines`) detection ordering relative to the AI call is implied but not stated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Commit Message Format (`$EDITOR` fallback case 3); Commit Flow / Lifecycle (stages 2–3)

**Details**:
Case 3 (oversized → fallback) must be detected before the L2 call, and is a generate-skip not a generate-fail. The ordering and its interaction with the `-y`/non-TTY fail-loud rule were implicit.

**Proposed Addition**:
Added "Detection ordering for the oversized case" to Commit Message Format: `max_diff_lines` is evaluated at L1 (after `diff_exclude`, before any L2 call); over-limit short-circuits L2 (generate-skip, like `--no-ai`) → `$EDITOR` fallback; the `-y`/non-TTY forbidden-combo check then applies as for `--no-ai`.

**Resolution**: Approved
**Notes**: Auto-resolved. Consistent with the three-layer engine boundary and the fail-loud forbidden-combo rule.

---

### 4. `e` / `r` / `$EDITOR` references but no statement of behaviour when `$EDITOR` is unset

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: `$EDITOR` Fallback — Path Semantics; Interactive Review Gate (`e` choice)

**Details**:
The no-TTY case was handled but the no-`$EDITOR`-on-a-TTY case (fresh machines/CI shells) was not. Git has a defined editor-resolution chain.

**Proposed Addition**:
Added "Editor resolution" to Fallback Semantics: mint resolves the editor using git's own order (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git's built-in default), so unset `$EDITOR` is not itself an error on a TTY. Fail loud only when no TTY/`-y`, or when no editor resolves to a launchable program. mint opens the editor itself (not via `git commit`) because staging is deferred to save-as-accept.

**Resolution**: Approved
**Notes**: Auto-resolved. "Behave like plain git commit" → reuse git's resolution chain; reconciled with deferred-staging (mint can't delegate to `git commit`).

---

### 5. `-a` and `-A` passed together: precedence/conflict behaviour undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Staging Model; CLI Surface & Flags

**Details**:
`-a` and `-A` are distinct staging modes; the both-supplied case (`mint commit -aA`) had no defined resolution.

**Proposed Addition**:
Added to Staging Model: `-a` and `-A` are mutually exclusive; supplying both is a conflicting-flags error → fail loud before any work (*"`-a` and `-A` cannot be combined; `-A` already includes `-a`'s changes"*).

**Resolution**: Approved
**Notes**: Auto-resolved. Fail-loud on contradictory input matches mint's posture; chosen over silent last-flag/superset-wins.

---

### 6. Push-failure message classification ambiguous (no-upstream UX overlaps the generic warn-clearly rule)

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Auto-push Behaviour

**Details**:
Only the no-upstream case had a bespoke message example; whether mint classifies push-failure causes or emits one generic warn with git's output attached was unspecified.

**Proposed Addition**:
Added to Auto-push: mint does not classify push-failure causes — one generic warn for all (commit in place; re-run the push), with git's stderr passed through verbatim beneath it. The "set an upstream" line is illustrative of git's pass-through, not a mint-authored per-cause message.

**Resolution**: Approved
**Notes**: Auto-resolved. Single rule (keep commit, surface git output, push is repeatable) matches the no-special-logic stance.

---

### 7. `-A`/`-a` empty-staging diagnostic: which git-flavoured message fires is underspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Staging Model (Empty-staging handling)

**Details**:
"`-A`/`-a` that stage nothing land here too" didn't say which of the two messages they get. The {which flag, what's present} → {which message} mapping was left to interpretation.

**Proposed Addition**:
Rewrote the empty-staging distinction: which message fires is determined by the actual tree state after the requested staging mode, not the flag. Genuinely clean → "working tree clean". Changes exist but mode staged none → guidance naming the modes that would help; specifically, `-a` that staged nothing because only untracked changes exist points at `-A`/`--add-all`.

**Resolution**: Approved
**Notes**: Auto-resolved. Makes "mirror git's messaging" precise for the staging-flag cases.

---
